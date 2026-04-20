package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	DefaultJWTExpireDuration = time.Hour * 24
	passwordLength           = 16
	chars                    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
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
	for _, user := range users {
		if ok := s.srv.Auth.ComparePassword(user.PasswordHash, password); ok {
			isValid = true
			userID = user.ID
			break
		}
	}
	if !isValid {
		zap.L().Warn(ErrNotFound.Error())
		return "", ErrNotFound
	}

	// Получение всех организаций пользователя
	accounts, err := s.srv.Account.GetByUserEmail(ctx, email)
	if err != nil {
		zap.L().Error(err.Error())
		return "", nil
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
	token, err := s.srv.Auth.GenerateToken(userID, accountsID, accountsID[0])
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	return token, nil
}

func (s *AuthService) GetClaimsFromToken(tokenString string) (*domain.AuthClaims, error) {
	// Парсинг токена и извлечение claims
	token, err := jwt.ParseWithClaims(tokenString, &domain.AuthClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			zap.L().Error(jwt.ErrTokenSignatureInvalid.Error())
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(s.secretKey), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			zap.L().Warn(err.Error())
			return nil, nil
		}
		zap.L().Error(err.Error())
		return nil, err
	}

	// Проверка валидности токена
	if claims, ok := token.Claims.(*domain.AuthClaims); ok && token.Valid {
		return claims, nil
	}

	zap.L().Error(ErrTokenInvalid.Error())
	return nil, ErrTokenInvalid
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
