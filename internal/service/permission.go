package service

type (
	BitPosition      = uint
	BitmapPermission = int32
)

const (
	defaultPermission BitmapPermission = 0
)

// HasPermission проверяет, установлен ли бит разрешения в маске прав.
func HasPermission(mask BitmapPermission, bitPosition BitPosition) bool {
	return mask&(1<<bitPosition) != 0
}

// AddPermission устанавливает бит разрешения.
func AddPermission(mask BitmapPermission, bitPositions ...BitPosition) BitmapPermission {
	for _, bitPosition := range bitPositions {
		mask = mask | (1 << bitPosition)
	}
	return mask
}

// RemovePermission снимает бит разрешения.
func RemovePermission(mask BitmapPermission, bitPositions ...BitPosition) BitmapPermission {
	for _, bitPosition := range bitPositions {
		mask = mask &^ (1 << bitPosition)
	}
	return mask
}
