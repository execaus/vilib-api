package domain

type (
	BitPosition = uint
	BitmapValue = int32
)

const (
	DefaultBitmap BitmapValue = 0
)

const (
	minBitPosition = 0
	maxBitPosition = 31
)

// HasBit проверяет, установлен ли бит в числе.
// mask - число, в котором проверяются биты.
// bitPosition - позиция бита для проверки (0-based).
func HasBit(mask BitmapValue, bitPosition BitPosition) bool {
	return mask&(1<<bitPosition) != 0
}

// SetBits устанавливает указанные биты в числе.
// mask - число, в котором устанавливаются биты.
// bitPositions - список позиций битов для установки.
func SetBits(mask BitmapValue, bitPositions ...BitPosition) BitmapValue {
	for _, bitPosition := range bitPositions {
		mask = mask | (1 << bitPosition)
	}
	return mask
}

// ClearBits очищает указанные биты в числе.
// mask - число, в котором очищаются биты.
// bitPositions - список позиций битов для очистки.
func ClearBits(mask BitmapValue, bitPositions ...BitPosition) BitmapValue {
	for _, bitPosition := range bitPositions {
		mask = mask &^ (1 << bitPosition)
	}
	return mask
}

// SetBitsUpTo устанавливает все биты от 0 до указанной позиции включительно.
// mask - число, в котором устанавливаются биты.
// bitPosition - позиция бита, до которой устанавливаются все биты включительно.
func SetBitsUpTo(mask BitmapValue, bitPosition BitPosition) BitmapValue {
	for i := BitPosition(minBitPosition); i <= bitPosition; i++ {
		mask |= 1 << i
	}
	return mask
}

// ClearBitsFrom очищает все биты от указанной позиции до старшего включительно.
// mask - число, в котором очищаются биты.
// bitPosition - позиция бита, начиная с которой очищаются все старшие биты включительно.
func ClearBitsFrom(mask BitmapValue, bitPosition BitPosition) BitmapValue {
	for i := bitPosition; i <= BitPosition(maxBitPosition); i++ {
		mask &^= 1 << i
	}
	return mask
}
