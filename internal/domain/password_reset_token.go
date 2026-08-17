package domain

import (
	"time"
	"vilib-api/internal/dbconv"
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

// PasswordResetToken — одноразовый токен сброса пароля (§6 дизайна эпика Э2, поправка О-1:
// токен привязан к конкретной строке пользователя UserID, а не ко всем строкам email). В базе
// хранится только TokenHash (SHA-256 сырого токена) — сам токен уходит пользователю только
// в письме.
type PasswordResetToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Email     string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (t *PasswordResetToken) FromDB(db *schema.PasswordResetToken) {
	t.ID = db.TokenID
	t.UserID = db.UserID
	t.Email = db.Email
	t.TokenHash = db.TokenHash
	t.ExpiresAt = db.ExpiresAt
	t.UsedAt = dbconv.NullValToPtr(db.UsedAt)
	t.CreatedAt = db.CreatedAt
}

// IsUsable возвращает true, если токен ещё не использован и не просрочен на момент now.
func (t *PasswordResetToken) IsUsable(now time.Time) bool {
	return t.UsedAt == nil && t.ExpiresAt.After(now)
}

// PasswordResetLink — организация и готовая ссылка сброса пароля для одного письма (§6 дизайна
// эпика Э2, поправка О-1): при нескольких активных организациях у email письмо содержит список
// таких пар — по одной ссылке (и токену) на организацию.
type PasswordResetLink struct {
	AccountName string
	URL         string
}
