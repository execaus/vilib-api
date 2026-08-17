package repository

import (
	"context"
	"errors"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"go.uber.org/zap"
)

type PasswordResetTokenRepository struct {
	provider *ExecutorProvider
}

func NewPasswordResetTokenRepository(provider *ExecutorProvider) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{provider: provider}
}

// Insert создаёт токен сброса пароля для строки пользователя userID (§6, §7 дизайна эпика Э2,
// поправка О-1).
func (r *PasswordResetTokenRepository) Insert(
	ctx context.Context,
	userID uuid.UUID, email, tokenHash string, expiresAt time.Time,
) (domain.PasswordResetToken, error) {
	exec := r.provider.GetExecutor(ctx)

	var token domain.PasswordResetToken

	tokenDB, err := schema.PasswordResetTokens.Insert(&schema.PasswordResetTokenSetter{
		UserID:    omit.From(userID),
		Email:     omit.From(email),
		TokenHash: omit.From(tokenHash),
		ExpiresAt: omit.From(expiresAt),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return token, err
	}

	token.FromDB(tokenDB)

	return token, nil
}

// SelectByHash выбирает токен по хешу. Строки не существует → ErrNotFound.
func (r *PasswordResetTokenRepository) SelectByHash(
	ctx context.Context,
	tokenHash string,
) (domain.PasswordResetToken, error) {
	exec := r.provider.GetExecutor(ctx)

	tokenDB, err := schema.PasswordResetTokens.Query(
		sm.Where(schema.PasswordResetTokens.Columns.TokenHash.EQ(psql.S(tokenHash))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.PasswordResetToken{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.PasswordResetToken{}, err
	}

	var token domain.PasswordResetToken
	token.FromDB(tokenDB)

	return token, nil
}

// MarkUsed помечает токен использованным (used_at = now).
func (r *PasswordResetTokenRepository) MarkUsed(ctx context.Context, tokenID uuid.UUID) error {
	exec := r.provider.GetExecutor(ctx)

	_, err := schema.PasswordResetTokens.Update(
		(&schema.PasswordResetTokenSetter{UsedAt: omitnull.From(time.Now())}).UpdateMod(),
		um.Where(schema.PasswordResetTokens.Columns.TokenID.EQ(psql.Arg(tokenID))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// DeleteByEmail удаляет все токены сброса пароля указанного email.
func (r *PasswordResetTokenRepository) DeleteByEmail(ctx context.Context, email string) error {
	exec := r.provider.GetExecutor(ctx)

	_, err := schema.PasswordResetTokens.Delete(
		dm.Where(schema.PasswordResetTokens.Columns.Email.EQ(psql.S(email))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
