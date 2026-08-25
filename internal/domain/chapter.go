package domain

import (
	"math"
	"time"
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

const (
	// chapterPercentMultiplier — перевод доли покрытия/порога главы в проценты (тот же приём,
	// что у watchPercentMultiplier видео, эпик Э3).
	chapterPercentMultiplier = 100
	// chapterMaxPercent — верхняя граница процента покрытия главы.
	chapterMaxPercent = 100
)

// ChapterStatus — вычисляемый статус пройденности главы пользователем (§3 дизайна эпика Э4):
// главы нигде не хранят статус, он всегда считается заново из текущего покрытия.
type ChapterStatus string

const (
	// ChapterStatusNotStarted — глава не просмотрена (нулевое покрытие).
	ChapterStatusNotStarted ChapterStatus = "not_started"
	// ChapterStatusPartial — глава просмотрена частично (покрытие больше нуля, но ниже порога).
	ChapterStatusPartial ChapterStatus = "partial"
	// ChapterStatusDone — глава пройдена (покрытие не ниже порога).
	ChapterStatusDone ChapterStatus = "done"
)

// Chapter — глава видео (§1 дизайна эпика Э4): принадлежит ровно одному видео, хранит только
// момент начала — конец главы не хранится ни в одной колонке, а вычисляется (см. ChapterBound)
// как начало следующей главы по порядку либо длительность видео.
type Chapter struct {
	ID        uuid.UUID
	VideoID   uuid.UUID
	Name      string
	StartMs   int64
	CreatedAt time.Time
}

// FromDB заполняет Chapter строкой сгенерированной модели schema.VideoChapter.
func (c *Chapter) FromDB(db *schema.VideoChapter) {
	c.ID = db.ChapterID
	c.VideoID = db.VideoID
	c.Name = db.Name
	c.StartMs = db.StartMS
	c.CreatedAt = db.CreatedAt
}

// ChapterBound — глава с вычисленной границей конца (§1 дизайна эпика Э4): EndMs — начало
// следующей главы того же видео по порядку start_ms либо длительность видео для последней
// главы. Вычисляется в SQL (LEAD/COALESCE), в Go не пересчитывается.
type ChapterBound struct {
	Chapter

	EndMs int64
}

// LengthMs — длина главы в миллисекундах, не меньше нуля (защитный случай для вырожденной по
// построению модели главы, §3 дизайна эпика Э4).
func (b *ChapterBound) LengthMs() int64 {
	length := b.EndMs - b.StartMs
	if length < 0 {
		return 0
	}

	return length
}

// ChapterProgress — покрытие главы просмотром одного пользователя (§3 дизайна эпика Э4):
// CoveredMs — сумма пересечений watch_progress.intervals пользователя с границами главы,
// вычисленная в SQL.
type ChapterProgress struct {
	ChapterBound

	CoveredMs int64
}

// CoveragePct — процент покрытия главы (floor, ограничен сверху 100). Вырожденная по длине
// глава (LengthMs() == 0) считается пройденной по определению — смотреть в ней нечего (§3
// дизайна эпика Э4).
func (p *ChapterProgress) CoveragePct() int {
	length := p.LengthMs()
	if length <= 0 {
		return chapterMaxPercent
	}

	pct := max(min(p.CoveredMs*chapterPercentMultiplier/length, chapterMaxPercent), 0)

	return int(pct)
}

// Status — статус пройденности главы при пороге threshold (доля 0..1, тот же параметр
// конфигурации, что и порог зачёта видео, §3 дизайна эпика Э4, решение В-1): "не просмотрена"
// при нулевом покрытии, "пройдена" при покрытии не ниже порога, иначе "частично".
func (p *ChapterProgress) Status(threshold float64) ChapterStatus {
	if p.CoveredMs <= 0 {
		return ChapterStatusNotStarted
	}

	thresholdPct := int(math.Round(threshold * chapterPercentMultiplier))
	if p.CoveragePct() >= thresholdPct {
		return ChapterStatusDone
	}

	return ChapterStatusPartial
}

// ChapterUserProgress — покрытие главы конкретным пользователем в выборке по многим
// пользователям сразу (отчёты по назначению/сотруднику, §3 дизайна эпика Э4): один SQL-запрос
// на всю карточку вместо запроса на участника (Н1).
type ChapterUserProgress struct {
	ChapterProgress

	UserID uuid.UUID
}

// CreateChapter — параметры создания главы (§4 дизайна эпика Э4): StartMs проверяется на
// вхождение в [0, duration_ms) видео сервисом, Name — на длину 1–200 символов биндингом DTO.
type CreateChapter struct {
	StartMs int64
	Name    string
}

// ChapterPatch — частичное изменение главы (§4 дизайна эпика Э4, ChapterService.Update): nil
// в поле — оно не меняется. Переименование без изменения StartMs не требует повторной проверки
// диапазона и уникальности.
type ChapterPatch struct {
	StartMs *int64
	Name    *string
}
