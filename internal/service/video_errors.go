package service

var (
	// ErrVideoNotAvailable — точка доступа к видео недоступна (загрузка не завершена или сбой
	// без загруженного оригинала).
	ErrVideoNotAvailable = NewConflictErrorCode("conflict.video_not_available", "video is not available")
	// ErrVideoNameExists — дубль имени видео в пределах группы.
	ErrVideoNameExists = NewConflictErrorCode("conflict.video_name", "video name already exists")
	// ErrObjectNotFoundInStorage — подтверждение загрузки, а оригинал не найден в хранилище.
	ErrObjectNotFoundInStorage = NewConflictErrorCode("conflict.object_not_found", "object not found in storage")
	// ErrObjectEmpty — подтверждение загрузки, а оригинал в хранилище пустой.
	ErrObjectEmpty = NewConflictErrorCode("conflict.object_empty", "object is empty")
)

const (
	codeConflictUploadFailed  = "conflict.upload_failed"
	codeValidationContentType = "validation.content_type"
	codeValidationSizeBytes   = "validation.size_bytes"
)
