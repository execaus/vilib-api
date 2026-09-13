package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
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

// Тексты писем с паролем (В-97). Пароль стоит отдельной строкой и раньше любых длинных латинских
// последовательностей: e2e-сценарии стенда и Playwright достают его из письма первым совпадением
// 16 латинских букв и цифр подряд.
const (
	registeredMailSubject = "Регистрация в Vilib"
	registeredMailIntro   = "Организация зарегистрирована в Vilib, вы — её владелец."
	createUserMailSubject = "Доступ к Vilib"
	createUserMailIntro   = "Вам открыт доступ к корпоративной видеобиблиотеке Vilib."
)

func (s *EmailService) SendRegisteredMail(ctx context.Context, email, password string) error {
	return s.send(ctx, []string{email}, registeredMailSubject, buildPasswordMailBody(registeredMailIntro, password))
}

func (s *EmailService) SendCreateUserEmail(ctx context.Context, email, password string) error {
	return s.send(ctx, []string{email}, createUserMailSubject, buildPasswordMailBody(createUserMailIntro, password))
}

// buildPasswordMailBody собирает текст письма с паролем для входа: вступление, пароль отдельной
// строкой и подсказка о смене пароля.
func buildPasswordMailBody(intro, password string) string {
	return intro + "\n\n" +
		"Для входа используйте адрес, на который пришло это письмо, и пароль:\n\n" +
		password + "\n\n" +
		"Пароль можно сменить в профиле после входа."
}

// resetMailSubject — тема письма сброса пароля (§6 дизайна эпика Э2).
const resetMailSubject = "Сброс пароля Vilib"

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
// несколько — по абзацу на организацию, название и ссылка отдельными строками (§6 дизайна
// эпика Э2, поправка О-1).
func buildPasswordResetBody(links []domain.PasswordResetLink, ttl time.Duration) string {
	var b strings.Builder

	// Ссылка — отдельной строкой без знаков препинания рядом: при автоссылке почтовые клиенты
	// иначе захватывают соседнюю точку в адрес, и ссылка перестаёт работать (В-97).
	if len(links) == 1 {
		b.WriteString("Для установки нового пароля перейдите по ссылке:\n\n")
		b.WriteString(links[0].URL)
		b.WriteString("\n\n")
		fmt.Fprintf(&b, "Ссылка действует %s.", humanizeDuration(ttl))
	} else {
		b.WriteString("Сброс пароля запрошен для нескольких организаций. ")
		b.WriteString("Перейдите по ссылке своей организации.\n\n")
		for _, link := range links {
			fmt.Fprintf(&b, "Организация «%s»:\n%s\n\n", link.AccountName, link.URL)
		}
		fmt.Fprintf(&b, "Ссылки действуют %s.", humanizeDuration(ttl))
	}

	b.WriteString(" Если вы не запрашивали сброс пароля, просто проигнорируйте это письмо.")

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

func (s *EmailService) sendRealMail(_ context.Context, to []string, subject string, body string) error {
	// Формирование SMTP-сообщения в MIME: без этого кириллица в теме и тексте у получателя нечитаема
	msg := buildMessage(s.cfg.From, to[0], subject, body)

	// Настройка аутентификации — пустой EMAIL_USERNAME означает SMTP без аутентификации
	// (например, локальный перехватчик Mailpit, не поддерживающий SMTP AUTH без TLS).
	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)

	// Отправка письма
	if err := s.sendMail(addr, auth, s.cfg.From, to, msg); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// mimeLineLength — предельная длина строки base64-тела письма по RFC 2045.
const mimeLineLength = 76

// buildMessage собирает письмо в формате MIME: тема кодируется по RFC 2047, тело — UTF-8 в base64
// с переводами строк CRLF. Всё сообщение получается 7-битным и одинаково читается любым почтовым
// сервером и клиентом (В-97).
func buildMessage(from, to, subject, body string) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.BEncoding.Encode("utf-8", subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")

	encoded := base64.StdEncoding.EncodeToString([]byte(strings.ReplaceAll(body, "\n", "\r\n")))
	for len(encoded) > mimeLineLength {
		b.WriteString(encoded[:mimeLineLength])
		b.WriteString("\r\n")
		encoded = encoded[mimeLineLength:]
	}
	b.WriteString(encoded)
	b.WriteString("\r\n")

	return []byte(b.String())
}

// Границы форм русского числительного: 1, 21 — «час»; 2–4, 22–24 — «часа»; остальные,
// включая 11–14, — «часов».
const (
	pluralDecimalBase = 10
	pluralHundredBase = 100
	pluralFewMin      = 2
	pluralFewMax      = 4
	pluralTeenMin     = 11
	pluralTeenMax     = 14
)

// humanizeDuration переводит срок в текст для письма («1 час», «30 минут», «1 час 30 минут»)
// вместо машинного «1h0m0s» из [time.Duration.String] (В-97). Секунды не выводятся: сроки ссылок
// задаются часами и минутами.
func humanizeDuration(d time.Duration) string {
	hours := int(d / time.Hour)
	minutes := int((d % time.Hour) / time.Minute)

	var parts []string
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", hours, pluralRu(hours, "час", "часа", "часов")))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", minutes, pluralRu(minutes, "минуту", "минуты", "минут")))
	}
	if len(parts) == 0 {
		return "меньше минуты"
	}

	return strings.Join(parts, " ")
}

// pluralRu выбирает форму слова для числа n по правилам русского языка.
func pluralRu(n int, one, few, many string) string {
	lastTwo := n % pluralHundredBase
	last := n % pluralDecimalBase

	switch {
	case lastTwo >= pluralTeenMin && lastTwo <= pluralTeenMax:
		return many
	case last == 1:
		return one
	case last >= pluralFewMin && last <= pluralFewMax:
		return few
	default:
		return many
	}
}
