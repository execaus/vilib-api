package dto

// ConfigResponse — публичный конфиг API для фронтенда (§5.2 контракта Э2, П-8): лимиты и
// допустимые типы загрузки видео, время жизни ссылок и токенов, набор профилей качества —
// значения, которые иначе пришлось бы дублировать константой на стороне фронта.
type ConfigResponse struct {
	// MaxUploadSizeBytes — максимальный размер файла видео для загрузки в байтах.
	MaxUploadSizeBytes int64 `json:"max_upload_size_bytes"`
	// AllowedContentTypes — допустимые MIME-типы загружаемого видео.
	AllowedContentTypes []string `json:"allowed_content_types"`
	// UploadURLTTLSeconds — время жизни преподписанного URL на загрузку оригинала, секунды.
	UploadURLTTLSeconds int64 `json:"upload_url_ttl_seconds"`
	// HLSURLTTLSeconds — время жизни HLS-токена доступа к мастер-плейлисту, секунды.
	HLSURLTTLSeconds int64 `json:"hls_url_ttl_seconds"`
	// Profiles — имена профилей качества HLS, в которые обрабатывается видео.
	Profiles []string `json:"profiles"`
	// TokenTTLSeconds — время жизни токена авторизации, секунды.
	TokenTTLSeconds int64 `json:"token_ttl_seconds"`
	// PasswordMinLength — минимальная длина пароля пользователя.
	PasswordMinLength int `json:"password_min_length"`
}
