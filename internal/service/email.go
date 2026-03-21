package service

import (
	"context"
	"fmt"
	"net/smtp"
	"vilib-api/config"
	"vilib-api/server"

	"go.uber.org/zap"
)

type EmailService struct {
	cfg          config.EmailConfig
	localMailBox chan string
	serverMode   server.Mode
}

func NewEmailService(cfg config.EmailConfig, serverMode server.Mode, localMailBox chan string) *EmailService {
	return &EmailService{cfg: cfg, localMailBox: localMailBox, serverMode: serverMode}
}

func (s *EmailService) SendRegisteredMail(ctx context.Context, email, password string) error {
	subject := "Welcome to ViLib!"
	return s.send(ctx, []string{email}, subject, password)
}

func (s *EmailService) SendCreateUserEmail(ctx context.Context, email, password string) error {
	subject := "Welcome to ViLib!"
	return s.send(ctx, []string{email}, subject, password)
}

func (s *EmailService) send(ctx context.Context, to []string, subject string, body string) error {
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

func (s *EmailService) sendLocalMail(body string) {
	s.localMailBox <- body
}

func (s *EmailService) sendRealMail(ctx context.Context, to []string, subject string, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", s.cfg.From, to[0], subject, body)

	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)

	if err := smtp.SendMail(addr, auth, s.cfg.From, to, []byte(msg)); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
