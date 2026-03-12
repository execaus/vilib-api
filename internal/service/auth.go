package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"time"
	"vilib-api/config"
	"vilib-api/internal/models"

	"github.com/golang-jwt/jwt/v5"
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
}

func NewAuthService(cfg config.AuthConfig) *AuthService {
	return &AuthService{secretKey: cfg.Key}
}

func (s *AuthService) GenerateToken(ctx context.Context, userID, accountID string) (string, error) {
	claims := models.AuthClaims{
		AccountID: accountID,
		UserID:    userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(DefaultJWTExpireDuration)),
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

func (s *AuthService) GetClaimsFromToken(ctx context.Context, tokenString string) (*models.AuthClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.AuthClaims{}, func(token *jwt.Token) (interface{}, error) {
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

	if claims, ok := token.Claims.(*models.AuthClaims); ok && token.Valid {
		return claims, nil
	}

	zap.L().Error(ErrTokenInvalid.Error())
	return nil, ErrTokenInvalid
}

func (s *AuthService) ComparePassword(ctx context.Context, hashedPassword string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		zap.L().Error(err.Error())
		return false
	}

	return true
}

func (s *AuthService) HashPassword(ctx context.Context, password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}
	return string(hashedBytes), nil
}

func (s *AuthService) GeneratePassword() (string, error) {
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
