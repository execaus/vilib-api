package testutil

import (
	"vilib-api/config"
	"vilib-api/internal/service"

	"github.com/google/uuid"
)

func GetToken(
	userID uuid.UUID,
	accounts []uuid.UUID,
	currentAccountID uuid.UUID,
) (string, error) {
	srv := service.NewAuthService(config.AuthConfig{}, nil)

	token, err := srv.GenerateToken(userID, accounts, currentAccountID)
	if err != nil {
		return "", err
	}

	return token, err
}
