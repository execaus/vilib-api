package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
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
	// PasswordMinLength — минимальная длина пароля пользователя, отдаётся публичным конфигом
	// (§5.2 контракта Э2, П-8) для валидации формы на фронте до запроса к API.
	PasswordMinLength = 8
	passwordLength    = 16
	chars             = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// bearerPrefix — префикс значения заголовка Authorization; GetClaimsFromToken принимает
	// значение как с ним, так и без (§1 дизайна эпика).
	bearerPrefix = "Bearer "
	// resetTokenBytesLength — длина сырого токена сброса пароля в байтах до base64url-
	// кодирования (§6 дизайна эпика Э2).
	resetTokenBytesLength = 32
)

type AuthService struct {
	secretKey        string
	passwordResetTTL time.Duration
	frontendOrigin   string
	repo             repository.PasswordResetToken
	srv              *Service
}

func NewAuthService(
	cfg config.AuthConfig,
	frontendCfg config.FrontendConfig,
	repo repository.PasswordResetToken,
	srv *Service,
) *AuthService {
	return &AuthService{
		secretKey:        cfg.Key,
		passwordResetTTL: cfg.PasswordResetTTL,
		frontendOrigin:   frontendCfg.Origin,
		repo:             repo,
		srv:              srv,
	}
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

// ChangePassword меняет пароль текущей строки пользователя userID (§6 дизайна эпика Э2,
// поправка О-1: пароль — свойство организации, а не человека, а не всех строк email).
func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	users, err := s.srv.User.GetByID(ctx, userID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}
	user := users[0]

	// Старый пароль должен совпасть с текущим хешем строки.
	if !s.srv.Auth.ComparePassword(user.PasswordHash, oldPassword) {
		zap.L().Warn(ErrOldPasswordInvalid.Error())
		return ErrOldPasswordInvalid
	}

	// Новый пароль не должен совпадать со старым и быть короче минимальной длины.
	if len(newPassword) < PasswordMinLength || s.srv.Auth.ComparePassword(user.PasswordHash, newPassword) {
		zap.L().Warn(ErrPasswordInvalid.Error())
		return ErrPasswordInvalid
	}

	newHash, err := s.srv.Auth.HashPassword(newPassword)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if _, err = s.srv.User.UpdatePasswordHash(ctx, userID, newHash); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// passwordResetTarget — строка пользователя, которой будет выдан токен сброса пароля, вместе
// с названием её организации (для письма со списком ссылок, §6 дизайна эпика Э2).
type passwordResetTarget struct {
	user        domain.User
	accountName string
}

// RequestPasswordReset запрашивает сброс пароля по email (§6 дизайна эпика Э2, поправка О-1).
// Email не найден, активных строк нет или ни одна не подошла под accountID — тихо ничего не
// делает (лог Warn), ответ вызывающей стороне всегда успешен. Иначе удаляет прежние токены
// email, выдаёт по одному токену на каждую подошедшую строку и отправляет письмо: одна
// строка — одна ссылка, несколько — список организаций со ссылкой на каждую.
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string, accountID *uuid.UUID) error {
	users, err := s.srv.User.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			zap.L().Warn("password reset requested for unknown email")
			return nil
		}
		zap.L().Error(err.Error())
		return err
	}

	targets, err := s.resolvePasswordResetTargets(ctx, users, accountID)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		zap.L().Warn("password reset requested but no matching active account row found")
		return nil
	}

	if err = s.repo.DeleteByEmail(ctx, email); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	links := make([]domain.PasswordResetLink, 0, len(targets))
	for _, target := range targets {
		rawToken, tokenErr := s.generateResetToken()
		if tokenErr != nil {
			zap.L().Error(tokenErr.Error())
			return tokenErr
		}

		if _, err = s.repo.Insert(
			ctx, target.user.ID, email, hashResetToken(rawToken), time.Now().Add(s.passwordResetTTL),
		); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		links = append(links, domain.PasswordResetLink{
			AccountName: target.accountName,
			URL:         s.buildResetLink(rawToken),
		})
	}

	if err = s.srv.Email.SendPasswordResetMail(ctx, email, links, s.passwordResetTTL); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// resolvePasswordResetTargets отбирает активные строки email, подходящие под accountID (§6
// дизайна эпика Э2, поправка О-1): accountID задан — только строка этой организации; не
// задан — все активные строки (одна или несколько).
func (s *AuthService) resolvePasswordResetTargets(
	ctx context.Context,
	users []domain.User,
	accountID *uuid.UUID,
) ([]passwordResetTarget, error) {
	targets := make([]passwordResetTarget, 0, len(users))

	for _, user := range users {
		if !user.IsActive() {
			continue
		}

		roles, err := s.srv.AccountRole.GetByID(ctx, user.RoleID)
		if err != nil {
			zap.L().Error(err.Error())
			return nil, err
		}

		if accountID != nil && roles[0].AccountID != *accountID {
			continue
		}

		accounts, err := s.srv.Account.GetByID(ctx, roles[0].AccountID)
		if err != nil {
			zap.L().Error(err.Error())
			return nil, err
		}

		targets = append(targets, passwordResetTarget{user: user, accountName: accounts[0].Name})
	}

	return targets, nil
}

// ResetPassword обновляет пароль строки пользователя, которой принадлежит токен (§6 дизайна
// эпика Э2, поправка О-1). Токен не найден, использован или просрочен — ErrResetTokenInvalid;
// после успешного сброса помечает токен использованным и удаляет остальные токены email.
func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < PasswordMinLength {
		zap.L().Warn(ErrPasswordInvalid.Error())
		return ErrPasswordInvalid
	}

	resetToken, err := s.repo.SelectByHash(ctx, hashResetToken(token))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			zap.L().Warn(ErrResetTokenInvalid.Error())
			return ErrResetTokenInvalid
		}
		zap.L().Error(err.Error())
		return err
	}

	if !resetToken.IsUsable(time.Now()) {
		zap.L().Warn(ErrResetTokenInvalid.Error())
		return ErrResetTokenInvalid
	}

	newHash, err := s.srv.Auth.HashPassword(newPassword)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if _, err = s.srv.User.UpdatePasswordHash(ctx, resetToken.UserID, newHash); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if err = s.repo.MarkUsed(ctx, resetToken.ID); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if err = s.repo.DeleteByEmail(ctx, resetToken.Email); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// generateResetToken генерирует сырой токен сброса пароля — resetTokenBytesLength случайных
// байт, закодированных в base64url без паддинга (§6 дизайна эпика Э2).
func (s *AuthService) generateResetToken() (string, error) {
	buf := make([]byte, resetTokenBytesLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashResetToken возвращает SHA-256 хеш сырого токена — только он хранится в базе (§6 дизайна
// эпика Э2).
func hashResetToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// buildResetLink собирает ссылку сброса пароля для фронтенда из FRONTEND_ORIGIN и сырого
// токена (§6 дизайна эпика Э2).
func (s *AuthService) buildResetLink(rawToken string) string {
	return fmt.Sprintf("%s/reset-password?token=%s", s.frontendOrigin, rawToken)
}
