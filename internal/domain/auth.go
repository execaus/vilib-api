package domain

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AuthClaims struct {
	jwt.RegisteredClaims

	UserID           uuid.UUID   `json:"user_id"`
	CurrentAccountID uuid.UUID   `json:"current_account_id"`
	Accounts         []uuid.UUID `json:"accounts"`
}

// HLSTokenPurpose — значение claim'а Purpose для HLS-токена, отличающее его от обычного
// токена авторизации (§4.2 дизайна эпика).
const HLSTokenPurpose = "hls"

// HLSClaims — claims короткоживущего токена доступа к HLS-плейлистам видео. Токен передаётся
// в query-параметре запросов мастер- и медиаплейлистов, минующих RequireAuthMiddleware, потому
// что hls.js не умеет добавлять заголовок Authorization к запросам сегментов (§4.2 эпика).
type HLSClaims struct {
	jwt.RegisteredClaims

	// Purpose всегда равен HLSTokenPurpose — отличает HLS-токен от AuthClaims той же подписи.
	Purpose string    `json:"purpose"`
	VideoID uuid.UUID `json:"video_id"`
}
