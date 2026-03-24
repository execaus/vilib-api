package domain

type (
	PermissionFlag = uint8
	PermissionMask = int64
)

const (
	DefaultPermissionMask PermissionMask = 0
)

// HasBit проверяет, установлен ли бит в числе.
// mask - число, в котором проверяются биты.
// bitPosition - позиция бита для проверки (0-based).
func HasBit(mask PermissionMask, bitPosition PermissionFlag) bool {
	return mask&(1<<bitPosition) != 0
}

// SetBits устанавливает указанные биты в числе.
// mask - число, в котором устанавливаются биты.
// bitPositions - список позиций битов для установки.
func SetBits(mask PermissionMask, bitPositions ...PermissionFlag) PermissionMask {
	for _, bitPosition := range bitPositions {
		mask = mask | (1 << bitPosition)
	}
	return mask
}

// ClearBits очищает указанные биты в числе.
// mask - число, в котором очищаются биты.
// bitPositions - список позиций битов для очистки.
func ClearBits(mask PermissionMask, bitPositions ...PermissionFlag) PermissionMask {
	for _, bitPosition := range bitPositions {
		mask = mask &^ (1 << bitPosition)
	}
	return mask
}
