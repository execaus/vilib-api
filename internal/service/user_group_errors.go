package service

// ErrGroupNameExists — дубль имени группы в пределах аккаунта (HTTP 409 conflict.group_name).
var ErrGroupNameExists = NewConflictErrorCode("conflict.group_name", "group name already exists")
