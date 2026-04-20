package service_test

import (
	"errors"
	"testing"
	"vilib-api/config"
	"vilib-api/internal/service"
	"vilib-api/server"

	"github.com/stretchr/testify/require"
)

func TestService_Email_SendRegisteredMail(t *testing.T) {
	t.Parallel()

	testEmail := "test@example.com"
	testPassword := "test-password"

	localMailBox := make(chan string, 1)

	cfg := config.EmailConfig{
		Host:     "localhost",
		Port:     "25",
		Username: "test",
		Password: "test",
		From:     "test@test.com",
	}

	tests := []struct {
		name       string
		serverMode server.Mode
		wantErr    error
	}{
		{
			name:       "development mode success",
			serverMode: server.DevelopmentMode,
			wantErr:    nil,
		},
		{
			name:       "production mode error",
			serverMode: server.ProductionMode,
			wantErr:    errors.New("dial tcp: missing address"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := service.NewEmailService(cfg, tt.serverMode, localMailBox)

			err := srv.SendRegisteredMail(t.Context(), testEmail, testPassword)

			if tt.wantErr != nil {
				require.Error(t, err)
			} else {
				require.Equal(t, tt.wantErr, err)
			}
		})
	}
}

func TestService_Email_SendCreateUserEmail(t *testing.T) {
	t.Parallel()

	testEmail := "test@example.com"
	testPassword := "test-password"

	localMailBox := make(chan string, 1)

	cfg := config.EmailConfig{
		Host:     "localhost",
		Port:     "25",
		Username: "test",
		Password: "test",
		From:     "test@test.com",
	}

	tests := []struct {
		name       string
		serverMode server.Mode
		wantErr    error
	}{
		{
			name:       "development mode success",
			serverMode: server.DevelopmentMode,
			wantErr:    nil,
		},
		{
			name:       "production mode error",
			serverMode: server.ProductionMode,
			wantErr:    errors.New("dial tcp: missing address"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := service.NewEmailService(cfg, tt.serverMode, localMailBox)

			err := srv.SendCreateUserEmail(t.Context(), testEmail, testPassword)

			if tt.wantErr != nil {
				require.Error(t, err)
			} else {
				require.Equal(t, tt.wantErr, err)
			}
		})
	}
}
