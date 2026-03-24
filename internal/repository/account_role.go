package repository

import (
	"context"
	"vilib-api/internal/dbconv"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"go.uber.org/zap"
)

type AccountRoleRepository struct {
	provider *ExecutorProvider
}

func NewAccountRoleRepository(provider *ExecutorProvider) *AccountRoleRepository {
	return &AccountRoleRepository{provider: provider}
}

func (r *AccountRoleRepository) Insert(
	ctx context.Context,
	accountID, name string,
	parentID *string,
	permission domain.PermissionMask,
	isDefault bool,
) (domain.AccountRole, error) {
	exec := r.provider.GetExecutor(ctx)

	roleDB, err := schema.AccountRoles.Insert(&schema.AccountRoleSetter{
		Name:           omit.From(name),
		PermissionMask: omit.From(permission),
		AccountID:      omit.From(uuid.MustParse(accountID)),
		ParentRoleID:   dbconv.StrPtrToNullUUID(parentID),
		IsDefault:      omit.From(isDefault),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	role := domain.AccountRole{}
	role.FromDB(roleDB)

	return role, nil
}

func (r *AccountRoleRepository) SelectByAccountID(ctx context.Context, accountID string) ([]domain.AccountRole, error) {
	exec := r.provider.GetExecutor(ctx)

	accountRolesDB, err := schema.AccountRoles.Query(
		sm.Where(schema.AccountRoles.Columns.AccountID.EQ(psql.S(accountID))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	accountRoles := make([]domain.AccountRole, len(accountRolesDB))
	for i, role := range accountRolesDB {
		accountRoles[i] = domain.AccountRole{}
		accountRoles[i].FromDB(role)
	}

	return accountRoles, nil
}
