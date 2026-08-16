package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// hlsPlaylistContentType — MIME-тип ответа ручек мастер- и медиаплейлиста HLS.
const hlsPlaylistContentType = "application/vnd.apple.mpegurl"

// ErrMissingHLSToken — в запросе плейлиста HLS отсутствует обязательный query-параметр token.
var ErrMissingHLSToken = errors.New("token is required")

// hlsTokenFromQuery извлекает HLS-токен из query-параметра token (§4.2 дизайна эпика).
func hlsTokenFromQuery(c *gin.Context) (string, error) {
	token := c.Query("token")
	if token == "" {
		return "", ErrMissingHLSToken
	}

	return token, nil
}

// sendHLSPlaylist отдаёт тело HLS-плейлиста. Cache-Control запрещает кеширование — плейлист
// авторизован токеном в URL, и его нельзя переиспользовать после истечения TTL (§4.2 эпика).
func sendHLSPlaylist(c *gin.Context, body []byte) {
	c.Header("Cache-Control", "private, no-store")
	c.Data(http.StatusOK, hlsPlaylistContentType, body)
}
