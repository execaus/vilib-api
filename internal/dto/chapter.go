package dto

import (
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

// Chapter — глава видео с точки зрения запрашивающего: границы и его собственная пройденность
// (§5 дизайна эпика Э4).
type Chapter struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	StartMs int64     `json:"start_ms"`
	// EndMs — начало следующей главы либо конец ролика; вычисляется сервером.
	EndMs int64 `json:"end_ms"`
	// DurationMs — длина главы (EndMs - StartMs).
	DurationMs int64 `json:"duration_ms"`
	// CoveragePct — покрытие главы просмотром запрашивающего, проценты.
	CoveragePct int `json:"coverage_pct"`
	// Status — not_started | partial | done.
	Status string `json:"status"`
}

// FromDomainProgress заполняет DTO из ChapterProgress — покрытие и статус вычисляются по
// порогу threshold (та же доля, что и порог зачёта видео, В-1 решение владельца эпика Э4).
func (c *Chapter) FromDomainProgress(p domain.ChapterProgress, threshold float64) {
	c.ID = p.ID
	c.Name = p.Name
	c.StartMs = p.StartMs
	c.EndMs = p.EndMs
	c.DurationMs = p.LengthMs()
	c.CoveragePct = p.CoveragePct()
	c.Status = string(p.Status(threshold))
}

// FromDomainBound заполняет DTO из ChapterBound без сведений о просмотре (ответ создания/
// редактирования главы, §4 дизайна эпика Э4) — CoveragePct/Status остаются нулевыми
// («не просмотрена»): поля присутствуют в контракте ради единой формы Chapter, но не несут
// содержательной нагрузки в редакторе.
func (c *Chapter) FromDomainBound(b domain.ChapterBound) {
	c.ID = b.ID
	c.Name = b.Name
	c.StartMs = b.StartMs
	c.EndMs = b.EndMs
	c.DurationMs = b.LengthMs()
	c.CoveragePct = 0
	c.Status = string(domain.ChapterStatusNotStarted)
}

// ListChaptersResponse — ответ на получение списка глав видео.
type ListChaptersResponse struct {
	Chapters []Chapter `json:"chapters"`
}

// CreateChapterRequest — тело запроса на создание главы.
type CreateChapterRequest struct {
	StartMs int64  `json:"start_ms" binding:"min=0"`
	Name    string `json:"name"     binding:"required,min=1,max=200"`
}

// UpdateChapterRequest — тело запроса на редактирование главы: оба поля необязательны,
// заполняется только то, что нужно изменить.
type UpdateChapterRequest struct {
	StartMs *int64  `json:"start_ms" binding:"omitempty,min=0"`
	Name    *string `json:"name"     binding:"omitempty,min=1,max=200"`
}

// ToDomain конвертирует тело запроса в domain.ChapterPatch.
func (r *UpdateChapterRequest) ToDomain() domain.ChapterPatch {
	return domain.ChapterPatch{StartMs: r.StartMs, Name: r.Name}
}

// ChapterResponse — ответ на создание/редактирование главы.
type ChapterResponse struct {
	Chapter Chapter `json:"chapter"`
}
