package domain

import (
	"strings"
	"time"
	"unicode/utf8"
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

// Допустимая длина названия организации в символах после обрезки пробелов по краям (A-01 ТЗ).
const (
	AccountNameMinLength = 2
	AccountNameMaxLength = 128
)

type Account struct {
	ID        uuid.UUID
	Name      string
	Email     string
	CreatedAt time.Time
}

func (a *Account) FromDB(db *schema.Account) {
	a.ID = db.AccountID
	a.Name = db.Name
	a.Email = db.Email
	a.CreatedAt = db.CreatedAt
}

// NormalizeAccountName обрезает пробелы по краям названия организации, указанного при
// регистрации, и проверяет его длину в символах. Второе значение false означает, что название
// вне допустимой длины. Уникальность названия не требуется: организация идентифицируется по id.
func NormalizeAccountName(name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	length := utf8.RuneCountInString(trimmed)

	return trimmed, length >= AccountNameMinLength && length <= AccountNameMaxLength
}
