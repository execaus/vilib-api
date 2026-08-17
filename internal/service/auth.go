package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	DefaultJWTExpireDuration = time.Hour * 24
	passwordLength           = 16
	chars                    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// bearerPrefix — префикс значения заголовка Authorization; GetClaimsFromToken принимает
	// значение как с ним, так и без (§1 дизайна эпика).
	bearerPrefix = "Bearer "
)

type AuthService struct {
	secretKey string
	srv       *Service
}

func NewAuthService(cfg config.AuthConfig, srv *Service) *AuthService {
	return &AuthService{secretKey: cfg.Key, srv: srv}
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	// Получение всех пользователей с таким email
	users, err := s.srv.User.GetByEmail(ctx, email)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	// Поиск совпадений пароля хотя бы в одном
	var userID uuid.UUID
	isValid := false
	var matchedUser domain.User
	for _, user := range users {
		if ok := s.srv.Auth.ComparePassword(user.PasswordHash, password); ok {
			isValid = true
			userID = user.ID
			matchedUser = user
			break
		}
	}
	if !isValid {
		zap.L().Warn(ErrInvalidCredentials.Error())
		return "", ErrInvalidCredentials
	}

	// Проверка, что пользователь активен
	if !matchedUser.IsActive() {
		return "", ErrUserDeactivated
	}

	// Текущая организация токена — организация роли совпавшей строки, а не первая по email
	// (§2.4 дизайна эпика: пользователь в двух организациях мог войти паролем одной, но
	// получить current_account_id другой).
	roles, err := s.srv.AccountRole.GetByID(ctx, matchedUser.RoleID)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}
	currentAccountID := roles[0].AccountID

	// Получение всех организаций пользователя с активной строкой (§2.4 дизайна эпика, A-03 ТЗ)
	accounts, err := s.srv.Account.GetByUserEmail(ctx, email)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	if len(accounts) == 0 {
		zap.L().Error(ErrAccountsNotFound.Error())
		return "", ErrAccountsNotFound
	}

	// Сбор всех идентификаторов организаций
	accountsID := make([]uuid.UUID, len(accounts))
	for i := 0; i < len(accounts); i++ {
		accountsID[i] = accounts[i].ID
	}

	// Генерация токена для авторизации пользователя
	token, err := s.srv.Auth.GenerateToken(userID, accountsID, currentAccountID)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	return token, nil
}

// SwitchAccount выпускает новый токен с current_account_id, переключённым на accountID —
// организацию, в которой у пользователя (по email текущей строки userID) есть строка
// (§2.4 дизайна эпика Э2). Нет строки в организации → ErrNotAccountMember (403 forbidden);
// строка деактивирована → ErrForbiddenUserDeactivated (403 forbidden.user_deactivated).
func (s *AuthService) SwitchAccount(ctx context.Context, userID, accountID uuid.UUID) (string, error) {
	// Текущая строка пользователя — источник email.
	users, err := s.srv.User.GetByID(ctx, userID)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}
	email := users[0].Email

	// Строка пользователя в целевой организации.
	targetUser, err := s.srv.User.GetByEmailAndAccountID(ctx, email, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			zap.L().Warn(err.Error())
			return "", ErrNotAccountMember
		}
		zap.L().Error(err.Error())
		return "", err
	}

	if !targetUser.IsActive() {
		return "", ErrForbiddenUserDeactivated
	}

	// Все организации пользователя с активной строкой — новый список accounts[] токена.
	accounts, err := s.srv.Account.GetByUserEmail(ctx, email)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	accountsID := make([]uuid.UUID, len(accounts))
	for i, account := range accounts {
		accountsID[i] = account.ID
	}

	token, err := s.srv.Auth.GenerateToken(targetUser.ID, accountsID, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	return token, nil
}

// GetClaimsFromToken разбирает и проверяет JWT. Параметр token — значение заголовка
// Authorization целиком: принимается как "Bearer <jwt>", так и голый "<jwt>" (обратная
// совместимость с e2e-скриптами, §1 дизайна эпика). Никогда не возвращает nil, nil:
// просроченный токен — ErrTokenExpired, любая другая проблема (битая подпись, неверный
// формат, token.Valid == false) — ErrTokenInvalid.
func (s *AuthService) GetClaimsFromToken(token string) (*domain.AuthClaims, error) {
	tokenString := strings.TrimSpace(strings.TrimPrefix(token, bearerPrefix))

	// Парсинг токена и извлечение claims
	parsedToken, err := jwt.ParseWithClaims(tokenString, &domain.AuthClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			zap.L().Error(jwt.ErrTokenSignatureInvalid.Error())
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(s.secretKey), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			zap.L().Warn(err.Error())
			return nil, ErrTokenExpired
		}
		zap.L().Warn(err.Error())
		return nil, ErrTokenInvalid
	}

	// Проверка валидности токена
	claims, ok := parsedToken.Claims.(*domain.AuthClaims)
	if !ok || !parsedToken.Valid {
		zap.L().Error(ErrTokenInvalid.Error())
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

func (s *AuthService) GenerateToken(
	userID uuid.UUID,
	accounts []uuid.UUID,
	currentAccountID uuid.UUID,
) (string, error) {
	// Создание структуры claims с данными пользователя
	claims := domain.AuthClaims{
		UserID:           userID,
		Accounts:         accounts,
		CurrentAccountID: currentAccountID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(DefaultJWTExpireDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// Создание и подпись токена
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.secretKey))
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	return signedToken, nil
}

// IssueHLSToken выпускает короткоживущий JWT-токен доступа к HLS-плейлистам видео с claims
// {purpose: "hls", video_id, exp} (§4.2 дизайна эпика). Подписывается тем же ключом, что и
// обычные токены авторизации, но проверяется отдельным ParseHLSToken.
func (s *AuthService) IssueHLSToken(videoID uuid.UUID, ttl time.Duration) (string, error) {
	claims := domain.HLSClaims{
		Purpose: domain.HLSTokenPurpose,
		VideoID: videoID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.secretKey))
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	return signedToken, nil
}

// ParseHLSToken проверяет подпись, срок действия и purpose HLS-токена. Любая проблема с
// токеном (просрочен, битая подпись, неверный purpose) возвращается как ErrUnauthorized —
// принадлежность токена конкретному видео проверяет вызывающий сервис отдельно.
func (s *AuthService) ParseHLSToken(tokenString string) (domain.HLSClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &domain.HLSClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			zap.L().Error(jwt.ErrTokenSignatureInvalid.Error())
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(s.secretKey), nil
	})
	if err != nil {
		zap.L().Warn(err.Error())
		return domain.HLSClaims{}, ErrUnauthorized
	}

	claims, ok := token.Claims.(*domain.HLSClaims)
	if !ok || !token.Valid || claims.Purpose != domain.HLSTokenPurpose {
		zap.L().Warn("invalid hls token claims")
		return domain.HLSClaims{}, ErrUnauthorized
	}

	return *claims, nil
}

func (s *AuthService) ComparePassword(hashedPassword string, password string) bool {
	// Проверка соответствия пароля хешу
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		zap.L().Error(err.Error())
		return false
	}

	return true
}

func (s *AuthService) HashPassword(password string) (string, error) {
	// Хеширование пароля
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}
	return string(hashedBytes), nil
}

func (s *AuthService) GeneratePassword() (string, error) {
	// Генерация случайного пароля заданной длины
	password := make([]byte, passwordLength)
	for i := 0; i < passwordLength; i++ {
		indexBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			zap.L().Error(err.Error())
			return "", err
		}
		password[i] = chars[indexBig.Int64()]
	}

	return string(password), nil
}
