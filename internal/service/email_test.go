package service_test

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"net/mail"
	"net/smtp"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
	"vilib-api/config"
	"vilib-api/internal/domain"
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
			subject, body := decodeMail(t, gotMsg)
			require.Contains(t, string(gotMsg), "From: noreply@vilib.local")
			require.Contains(t, string(gotMsg), "To: user@example.com")
			require.Equal(t, "Регистрация в Vilib", subject)
			require.Contains(t, body, "pass123")

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

// TestService_Email_SendPasswordResetMail проверяет текст письма сброса пароля (§6 дизайна
// эпика Э2, поправка О-1): одна организация — единая ссылка в тексте, несколько — список
// организаций со ссылкой на каждую.
func TestService_Email_SendPasswordResetMail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		links          []domain.PasswordResetLink
		ttl            time.Duration
		wantContains   []string
		wantNotContain string
	}{
		{
			name: "single organization",
			links: []domain.PasswordResetLink{
				{AccountName: "Организация Один", URL: "http://localhost:5173/reset-password?token=abc"},
			},
			ttl: time.Hour,
			wantContains: []string{
				"Для установки нового пароля перейдите по ссылке:\n\nhttp://localhost:5173/reset-password?token=abc\n",
				"Ссылка действует 1 час.",
			},
			wantNotContain: "Организация",
		},
		{
			name: "multiple organizations",
			links: []domain.PasswordResetLink{
				{AccountName: "Организация Один", URL: "http://localhost:5173/reset-password?token=abc"},
				{AccountName: "Организация Два", URL: "http://localhost:5173/reset-password?token=def"},
			},
			ttl: time.Hour,
			wantContains: []string{
				"Организация «Организация Один»:\nhttp://localhost:5173/reset-password?token=abc\n",
				"Организация «Организация Два»:\nhttp://localhost:5173/reset-password?token=def\n",
				"Ссылки действуют 1 час.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotMsg []byte
			sendMail := func(_ string, _ smtp.Auth, _ string, _ []string, msg []byte) error {
				gotMsg = msg
				return nil
			}

			cfg := config.EmailConfig{Host: "mailpit", Port: "1025", From: "noreply@vilib.local"}
			srv := service.NewEmailService(cfg, server.ProductionMode, nil, service.WithSendMail(sendMail))

			err := srv.SendPasswordResetMail(t.Context(), "user@example.com", tt.links, tt.ttl)
			require.NoError(t, err)

			subject, body := decodeMail(t, gotMsg)
			require.Equal(t, "Сброс пароля Vilib", subject)
			for _, want := range tt.wantContains {
				require.Contains(t, body, want)
			}
			if tt.wantNotContain != "" {
				require.NotContains(t, body, tt.wantNotContain)
			}
			// Ссылка не слипается с пунктуацией, срок — словами, а не time.Duration.String.
			require.NotContains(t, body, "token=abc.")
			require.NotContains(t, body, "h0m0s")
		})
	}
}

// decodeMail разбирает MIME-письмо, собранное EmailService: декодирует тему (RFC 2047) и base64-тело,
// переводы строк тела приводит к \n.
func decodeMail(t *testing.T, raw []byte) (string, string) {
	t.Helper()

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	require.NoError(t, err)

	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	require.NoError(t, err)

	encoded, err := io.ReadAll(msg.Body)
	require.NoError(t, err)
	compact := strings.NewReplacer("\r", "", "\n", "").Replace(string(encoded))
	decoded, err := base64.StdEncoding.DecodeString(compact)
	require.NoError(t, err)

	return subject, strings.ReplaceAll(string(decoded), "\r\n", "\n")
}

// TestEmailService_SendRealMail_MIMEUTF8 проверяет, что письма с паролем уходят в MIME с UTF-8:
// сообщение целиком 7-битное, тема и текст на русском, а пароль — первое совпадение 16 латинских
// букв и цифр подряд, по которому его достают e2e-сценарии стенда (В-97).
func TestEmailService_SendRealMail_MIMEUTF8(t *testing.T) {
	t.Parallel()

	const password = "Ab12Cd34Ef56Gh78"

	tests := []struct {
		name        string
		send        func(ctx context.Context, srv *service.EmailService) error
		wantSubject string
		wantIntro   string
	}{
		{
			name: "registered owner",
			send: func(ctx context.Context, srv *service.EmailService) error {
				return srv.SendRegisteredMail(ctx, "user@example.com", password)
			},
			wantSubject: "Регистрация в Vilib",
			wantIntro:   "Организация зарегистрирована в Vilib, вы — её владелец.",
		},
		{
			name: "created employee",
			send: func(ctx context.Context, srv *service.EmailService) error {
				return srv.SendCreateUserEmail(ctx, "user@example.com", password)
			},
			wantSubject: "Доступ к Vilib",
			wantIntro:   "Вам открыт доступ к корпоративной видеобиблиотеке Vilib.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotMsg []byte
			sendMail := func(_ string, _ smtp.Auth, _ string, _ []string, msg []byte) error {
				gotMsg = msg
				return nil
			}

			cfg := config.EmailConfig{Host: "mailpit", Port: "1025", From: "noreply@vilib.local"}
			srv := service.NewEmailService(cfg, server.ProductionMode, nil, service.WithSendMail(sendMail))

			require.NoError(t, tt.send(t.Context(), srv))

			for _, r := range string(gotMsg) {
				require.Less(t, r, rune(utf8.RuneSelf), "в сыром письме не должно быть не-ASCII символов")
			}

			msg, err := mail.ReadMessage(strings.NewReader(string(gotMsg)))
			require.NoError(t, err)
			require.Equal(t, "1.0", msg.Header.Get("MIME-Version"))
			require.Equal(t, "text/plain; charset=UTF-8", msg.Header.Get("Content-Type"))
			require.Equal(t, "base64", msg.Header.Get("Content-Transfer-Encoding"))

			subject, body := decodeMail(t, gotMsg)
			require.Equal(t, tt.wantSubject, subject)
			require.Contains(t, body, tt.wantIntro)
			require.Equal(t, password, regexp.MustCompile(`[A-Za-z0-9]{16}`).FindString(body))
		})
	}
}

// TestService_Email_PasswordResetTTLWording проверяет срок ссылки словами с русским склонением.
func TestService_Email_PasswordResetTTLWording(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ttl  time.Duration
		want string
	}{
		{name: "one hour", ttl: time.Hour, want: "Ссылка действует 1 час."},
		{name: "two hours", ttl: 2 * time.Hour, want: "Ссылка действует 2 часа."},
		{name: "five hours", ttl: 5 * time.Hour, want: "Ссылка действует 5 часов."},
		{name: "eleven hours", ttl: 11 * time.Hour, want: "Ссылка действует 11 часов."},
		{name: "twenty one hours", ttl: 21 * time.Hour, want: "Ссылка действует 21 час."},
		{name: "one minute", ttl: time.Minute, want: "Ссылка действует 1 минуту."},
		{name: "thirty minutes", ttl: 30 * time.Minute, want: "Ссылка действует 30 минут."},
		{name: "hour and a half", ttl: 90 * time.Minute, want: "Ссылка действует 1 час 30 минут."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotMsg []byte
			sendMail := func(_ string, _ smtp.Auth, _ string, _ []string, msg []byte) error {
				gotMsg = msg
				return nil
			}

			cfg := config.EmailConfig{Host: "mailpit", Port: "1025", From: "noreply@vilib.local"}
			srv := service.NewEmailService(cfg, server.ProductionMode, nil, service.WithSendMail(sendMail))
			links := []domain.PasswordResetLink{
				{AccountName: "Организация", URL: "http://localhost/reset-password?token=abc"},
			}

			require.NoError(t, srv.SendPasswordResetMail(t.Context(), "user@example.com", links, tt.ttl))

			_, body := decodeMail(t, gotMsg)
			require.Contains(t, body, tt.want)
		})
	}
}
