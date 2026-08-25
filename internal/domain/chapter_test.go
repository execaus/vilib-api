package domain_test

import (
	"testing"
	"vilib-api/internal/domain"

	"github.com/stretchr/testify/require"
)

// TestChapterProgress_CoveragePct проверяет процент покрытия главы (§3 дизайна эпика Э4):
// floor-деление, ограничение сверху 100 и защитный случай вырожденной по длине главы (нечего
// смотреть — считается пройденной по определению).
func TestChapterProgress_CoveragePct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		startMs   int64
		endMs     int64
		coveredMs int64
		want      int
	}{
		{
			name:      "half covered floors down",
			startMs:   0,
			endMs:     10000,
			coveredMs: 4999,
			want:      49,
		},
		{
			name:      "fully covered is 100",
			startMs:   0,
			endMs:     10000,
			coveredMs: 10000,
			want:      100,
		},
		{
			name:      "over covered is capped at 100",
			startMs:   0,
			endMs:     10000,
			coveredMs: 15000,
			want:      100,
		},
		{
			name:      "not covered is 0",
			startMs:   0,
			endMs:     10000,
			coveredMs: 0,
			want:      0,
		},
		{
			name:      "degenerate zero-length chapter counts as fully covered",
			startMs:   5000,
			endMs:     5000,
			coveredMs: 0,
			want:      100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			progress := domain.ChapterProgress{
				ChapterBound: domain.ChapterBound{
					Chapter: domain.Chapter{StartMs: tt.startMs},
					EndMs:   tt.endMs,
				},
				CoveredMs: tt.coveredMs,
			}

			require.Equal(t, tt.want, progress.CoveragePct())
		})
	}
}

// TestChapterProgress_Status проверяет вычисляемый статус пройденности главы при пороге
// (§3 дизайна эпика Э4, решение В-1): нулевое покрытие — "не просмотрена", покрытие не ниже
// порога — "пройдена", иначе — "частично".
func TestChapterProgress_Status(t *testing.T) {
	t.Parallel()

	const chapterLengthMs = 10000

	tests := []struct {
		name      string
		coveredMs int64
		threshold float64
		want      domain.ChapterStatus
	}{
		{
			name:      "zero coverage is not started",
			coveredMs: 0,
			threshold: 0.95,
			want:      domain.ChapterStatusNotStarted,
		},
		{
			name:      "coverage below threshold is partial",
			coveredMs: 9000,
			threshold: 0.95,
			want:      domain.ChapterStatusPartial,
		},
		{
			name:      "coverage at threshold is done",
			coveredMs: 9500,
			threshold: 0.95,
			want:      domain.ChapterStatusDone,
		},
		{
			name:      "full coverage is done",
			coveredMs: chapterLengthMs,
			threshold: 0.95,
			want:      domain.ChapterStatusDone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			progress := domain.ChapterProgress{
				ChapterBound: domain.ChapterBound{
					Chapter: domain.Chapter{StartMs: 0},
					EndMs:   chapterLengthMs,
				},
				CoveredMs: tt.coveredMs,
			}

			require.Equal(t, tt.want, progress.Status(tt.threshold))
		})
	}
}

// TestChapterBound_LengthMs проверяет длину главы: обычный случай и защитный случай
// отрицательной разницы (не должен возникать по построению модели, но не должен паниковать).
func TestChapterBound_LengthMs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		startMs int64
		endMs   int64
		want    int64
	}{
		{name: "normal length", startMs: 1000, endMs: 5000, want: 4000},
		{name: "zero length", startMs: 5000, endMs: 5000, want: 0},
		{name: "defensive negative length clamps to zero", startMs: 5000, endMs: 1000, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bound := domain.ChapterBound{
				Chapter: domain.Chapter{StartMs: tt.startMs},
				EndMs:   tt.endMs,
			}

			require.Equal(t, tt.want, bound.LengthMs())
		})
	}
}
