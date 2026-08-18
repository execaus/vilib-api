package domain_test

import (
	"testing"
	"vilib-api/internal/domain"

	"github.com/stretchr/testify/require"
)

// TestHasBit_AssignmentPermissions проверяет позиции новых битов права назначения обучения
// (решение В-3 эпика Э3): AccountPermissionManageAssignments — бит 6, GroupPermissionManageAssignments —
// бит 4; HasBit видит бит только в маске, где он установлен.
func TestHasBit_AssignmentPermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mask domain.PermissionMask
		flag domain.PermissionFlag
		want bool
	}{
		{
			name: "account permission manage assignments is bit 6",
			mask: 1 << 6,
			flag: domain.AccountPermissionManageAssignments,
			want: true,
		},
		{
			name: "account permission manage assignments absent in other mask",
			mask: 1 << 5,
			flag: domain.AccountPermissionManageAssignments,
			want: false,
		},
		{
			name: "group permission manage assignments is bit 4",
			mask: 1 << 4,
			flag: domain.GroupPermissionManageAssignments,
			want: true,
		},
		{
			name: "group permission manage assignments absent in other mask",
			mask: 1 << 3,
			flag: domain.GroupPermissionManageAssignments,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, domain.HasBit(tt.mask, tt.flag))
		})
	}
}

// TestAssignmentPermissions_BitPositions фиксирует конкретные позиции битов (решение В-3):
// изменение значения — ломающая перемена контракта ролей.
func TestAssignmentPermissions_BitPositions(t *testing.T) {
	t.Parallel()

	require.Equal(t, domain.AccountPermissionManageAssignments, domain.PermissionFlag(6))
	require.Equal(t, domain.GroupPermissionManageAssignments, domain.PermissionFlag(4))
}

// TestSetBits_AssignmentPermissions проверяет, что SetBits/ClearBits корректно работают с
// новыми битами наравне с существующими (владелец аккаунта/группы проходит по своему биту
// независимо от AccountPermissionManageAssignments/GroupPermissionManageAssignments).
func TestSetBits_AssignmentPermissions(t *testing.T) {
	t.Parallel()

	mask := domain.SetBits(domain.DefaultPermissionMask, domain.AccountPermissionManageAssignments)
	require.True(t, domain.HasBit(mask, domain.AccountPermissionManageAssignments))

	mask = domain.ClearBits(mask, domain.AccountPermissionManageAssignments)
	require.False(t, domain.HasBit(mask, domain.AccountPermissionManageAssignments))
}
