package service

import (
	"context"
	"errors"
	"strings"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/gen/schema"
	"vilib-api/internal/models"
	"vilib-api/internal/repository"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AccountService struct {
	repo *repository.TransactionalRepository
}

func NewAccountService(repo *repository.TransactionalRepository) *AccountService {
	return &AccountService{repo: repo}
}

func (s *AccountService) Create(ctx context.Context, ownerID, email string) (models.Account, error) {
	exec := s.repo.GetExecutor(ctx)

	var account models.Account

	var accountName string
	if i := strings.Index(email, "@"); i != -1 {
		accountName = email[:i]
	} else {
		return account, ErrEmailInvalid
	}

	accountDB, err := schema.Accounts.Insert(&schema.AccountSetter{
		Name:    omit.From(accountName),
		OwnerID: omit.From(uuid.MustParse(ownerID)),
		Email:   omit.From(email),
	}).One(ctx, exec)
	if err != nil {
		if errors.Is(err, dberrors.AccountErrors.ErrUniqueAccountsNameKey) {
			zap.L().Warn(err.Error())
			return account, ErrAccountNameExists
		}
		zap.L().Error(err.Error())
		return account, err
	}

	account.FromDB(accountDB)

	return account, nil
}
