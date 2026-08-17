package service

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/server"

	"go.uber.org/zap"
)

// smtpSendMailFunc — сигнатура функции отправки письма по SMTP, совпадает с [net/smtp.SendMail].
// Вынесена полем структуры, чтобы в тестах подменять реальную сетевую отправку.
type smtpSendMailFunc func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

type EmailService struct {
	cfg          config.EmailConfig
	localMailBox chan string
	serverMode   server.Mode
	// sendMail — функция реальной отправки письма по SMTP. В проде — [net/smtp.SendMail],
	// в тестах подменяется опцией WithSendMail.
	sendMail smtpSendMailFunc
}

// EmailServiceOption настраивает EmailService сверх обязательных зависимостей конструктора.
type EmailServiceOption func(*EmailService)

// WithSendMail подменяет функцию реальной отправки письма по SMTP. Предназначена для тестов.
func WithSendMail(sendMail smtpSendMailFunc) EmailServiceOption {
	return func(s *EmailService) {
		s.sendMail = sendMail
	}
}

func NewEmailService(
	cfg config.EmailConfig,
	serverMode server.Mode,
	localMailBox chan string,
	opts ...EmailServiceOption,
) *EmailService {
	s := &EmailService{cfg: cfg, localMailBox: localMailBox, serverMode: serverMode, sendMail: smtp.SendMail}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *EmailService) SendRegisteredMail(ctx context.Context, email, password string) error {
	// Формирование темы письма
	subject := "Welcome to ViLib!"
	return s.send(ctx, []string{email}, subject, password)
}

func (s *EmailService) SendCreateUserEmail(ctx context.Context, email, password string) error {
	// Формирование темы письма
	subject := "Welcome to ViLib!"
	return s.send(ctx, []string{email}, subject, password)
}

// resetMailSubject — тема письма сброса пароля (§6 дизайна эпика Э2).
const resetMailSubject = "Сброс пароля ViLib"

// SendPasswordResetMail отправляет письмо со ссылкой сброса пароля (§6 дизайна эпика Э2,
// поправка О-1). Одна ссылка (links из одного элемента) — единый текст со ссылкой; несколько —
// список организаций, у каждой своя ссылка (свой токен), так пользователь не должен знать
// идентификатор организации.
func (s *EmailService) SendPasswordResetMail(
	ctx context.Context,
	email string,
	links []domain.PasswordResetLink,
	ttl time.Duration,
) error {
	return s.send(ctx, []string{email}, resetMailSubject, buildPasswordResetBody(links, ttl))
}

// buildPasswordResetBody собирает текст письма сброса пароля: одна ссылка — короткий текст,
// несколько — список «Организация «X»: <ссылка>» построчно (§6 дизайна эпика Э2, поправка О-1).
func buildPasswordResetBody(links []domain.PasswordResetLink, ttl time.Duration) string {
	var b strings.Builder

	if len(links) == 1 {
		b.WriteString("Для установки нового пароля перейдите по ссылке: ")
		b.WriteString(links[0].URL)
		b.WriteString(".")
	} else {
		b.WriteString("Сброс пароля запрошен для нескольких организаций. Перейдите по ссылке своей организации:\n")
		for _, link := range links {
			fmt.Fprintf(&b, "Организация «%s»: %s\n", link.AccountName, link.URL)
		}
	}

	fmt.Fprintf(&b, " Ссылка действует %s.", ttl)
	b.WriteString(" Если вы не запрашивали сброс — проигнорируйте письмо.")

	return b.String()
}

func (s *EmailService) send(ctx context.Context, to []string, subject string, body string) error {
	// Отправка в зависимости от режима работы сервера
	switch s.serverMode {
	case server.HybridMode:
		s.sendLocalMail(body)
		return s.sendRealMail(ctx, to, subject, body)
	case server.ProductionMode:
		return s.sendRealMail(ctx, to, subject, body)
	case server.DevelopmentMode:
		s.sendLocalMail(body)
		return nil
	}

	return server.ErrInvalidServerMode
}

// sendLocalMail неблокирующе кладёт письмо в локальный почтовый ящик — если канал не задан
// или уже заполнен, письмо отбрасывается с предупреждением в лог, а не подвешивает вызывающую
// ручку навсегда.
func (s *EmailService) sendLocalMail(body string) {
	if s.localMailBox == nil {
		zap.L().Warn("local mailbox is not configured, message dropped")
		return
	}

	select {
	case s.localMailBox <- body:
	default:
		zap.L().Warn("local mailbox is full, message dropped")
	}
}

func (s *EmailService) sendRealMail(ctx context.Context, to []string, subject string, body string) error {
	// Формирование SMTP-сообщения
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", s.cfg.From, to[0], subject, body)

	// Настройка аутентификации — пустой EMAIL_USERNAME означает SMTP без аутентификации
	// (например, локальный перехватчик Mailpit, не поддерживающий SMTP AUTH без TLS).
	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)

	// Отправка письма
	if err := s.sendMail(addr, auth, s.cfg.From, to, []byte(msg)); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
