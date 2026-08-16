package service_test

import (
	"errors"
	"net/smtp"
	"testing"
	"time"
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

// TestEmailService_SendRealMail_AuthDependsOnUsername проверяет, что при пустом EMAIL_USERNAME
// письмо уходит без SMTP-аутентификации (auth == nil), а при заданном — с [smtp.PlainAuth].
func TestEmailService_SendRealMail_AuthDependsOnUsername(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		username string
		wantAuth bool
	}{
		{
			name:     "empty username sends without authentication",
			username: "",
			wantAuth: false,
		},
		{
			name:     "non-empty username sends with plain authentication",
			username: "mailpit-user",
			wantAuth: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.EmailConfig{
				Host:     "mailpit",
				Port:     "1025",
				Username: tt.username,
				Password: "secret",
				From:     "noreply@vilib.local",
			}

			var gotAddr, gotFrom string
			var gotAuth smtp.Auth
			var gotTo []string
			var gotMsg []byte

			sendMail := func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
				gotAddr, gotAuth, gotFrom, gotTo, gotMsg = addr, a, from, to, msg
				return nil
			}

			srv := service.NewEmailService(cfg, server.ProductionMode, nil, service.WithSendMail(sendMail))

			err := srv.SendRegisteredMail(t.Context(), "user@example.com", "pass123")
			require.NoError(t, err)

			require.Equal(t, "mailpit:1025", gotAddr)
			require.Equal(t, cfg.From, gotFrom)
			require.Equal(t, []string{"user@example.com"}, gotTo)
			require.Contains(t, string(gotMsg), "From: noreply@vilib.local")
			require.Contains(t, string(gotMsg), "To: user@example.com")
			require.Contains(t, string(gotMsg), "Subject: Welcome to ViLib!")
			require.Contains(t, string(gotMsg), "pass123")

			if tt.wantAuth {
				require.NotNil(t, gotAuth)
			} else {
				require.Nil(t, gotAuth)
			}
		})
	}
}

// TestEmailService_SendLocalMail_NonBlockingWhenChannelFull проверяет, что запись в заполненный
// localMailBox не блокирует отправку письма — лишнее сообщение отбрасывается с предупреждением.
func TestEmailService_SendLocalMail_NonBlockingWhenChannelFull(t *testing.T) {
	t.Parallel()

	localMailBox := make(chan string, 1)
	cfg := config.EmailConfig{}
	srv := service.NewEmailService(cfg, server.DevelopmentMode, localMailBox)

	require.NoError(t, srv.SendRegisteredMail(t.Context(), "first@example.com", "pass-1"))

	done := make(chan error, 1)
	go func() {
		done <- srv.SendRegisteredMail(t.Context(), "second@example.com", "pass-2")
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("SendRegisteredMail заблокировался при заполненном localMailBox")
	}
}

// TestEmailService_SendLocalMail_NoPanicWhenMailboxNil проверяет, что при отсутствующем
// localMailBox (nil-канал) отправка локальной почты не паникует и не блокируется.
func TestEmailService_SendLocalMail_NoPanicWhenMailboxNil(t *testing.T) {
	t.Parallel()

	cfg := config.EmailConfig{}
	srv := service.NewEmailService(cfg, server.DevelopmentMode, nil)

	err := srv.SendRegisteredMail(t.Context(), "user@example.com", "pass123")
	require.NoError(t, err)
}
