package service

const (
	// Администратор аккаунта. Имеет следующие возможности:
	// - добавлять новых пользователей в аккаунт.
	accountAdminBitPosition = iota

	// Модератор аккаунта. Имеет следующие возможности:
	// - добавлять новых пользователей в аккаунт.
	accountModeratorBitPosition

	// Обычный пользователь системы.
	accountUserBitPosition
)
