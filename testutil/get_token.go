package testutil

import (
	"vilib-api/config"
	"vilib-api/internal/service"
)

func GetToken(
	userID string,
	accounts []string,
	currentAccountID string,
) (string, error) {
	srv := service.NewAuthService(config.AuthConfig{}, nil)

	token, err := srv.GenerateToken(userID, accounts, currentAccountID)
	if err != nil {
		return "", err
	}

	return token, err
}
