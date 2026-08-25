package domain_test

import (
	"testing"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/null"
	"github.com/stretchr/testify/require"
)

// TestVideo_FromDB_QueueMetrics проверяет заполнение признака срочности и отметок времени
// конвейера (queued_at/compressing_started_at/ready_at) при конвертации из БД (эпик Э5, В-72):
// заполненные значения переносятся указателями, отсутствующие остаются nil.
func TestVideo_FromDB_QueueMetrics(t *testing.T) {
	t.Parallel()

	queuedAt := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	compressingStartedAt := time.Date(2026, 8, 25, 10, 5, 0, 0, time.UTC)
	readyAt := time.Date(2026, 8, 25, 10, 20, 0, 0, time.UTC)

	tests := []struct {
		name string
		db   *schema.UserGroupVideo
		want domain.Video
	}{
		{
			name: "urgent video with all queue timestamps filled",
			db: &schema.UserGroupVideo{
				IsUrgent:             true,
				QueuedAt:             null.From(queuedAt),
				CompressingStartedAt: null.From(compressingStartedAt),
				ReadyAt:              null.From(readyAt),
			},
			want: domain.Video{
				IsUrgent:             true,
				QueuedAt:             &queuedAt,
				CompressingStartedAt: &compressingStartedAt,
				ReadyAt:              &readyAt,
			},
		},
		{
			name: "archive video without queue timestamps",
			db: &schema.UserGroupVideo{
				IsUrgent: false,
			},
			want: domain.Video{
				IsUrgent:             false,
				QueuedAt:             nil,
				CompressingStartedAt: nil,
				ReadyAt:              nil,
			},
		},
		{
			name: "queued video has only queued_at filled",
			db: &schema.UserGroupVideo{
				IsUrgent: false,
				QueuedAt: null.From(queuedAt),
			},
			want: domain.Video{
				IsUrgent:             false,
				QueuedAt:             &queuedAt,
				CompressingStartedAt: nil,
				ReadyAt:              nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var v domain.Video
			v.FromDB(tt.db)

			require.Equal(t, tt.want.IsUrgent, v.IsUrgent)
			require.Equal(t, tt.want.QueuedAt, v.QueuedAt)
			require.Equal(t, tt.want.CompressingStartedAt, v.CompressingStartedAt)
			require.Equal(t, tt.want.ReadyAt, v.ReadyAt)
		})
	}
}

// TestPipelineProgress_FromDB проверяет конвертацию индикатора живости конвейера из БД:
// полоса (архивная/срочная) и момент последнего успешного взятия видео в обработку.
func TestPipelineProgress_FromDB(t *testing.T) {
	t.Parallel()

	lastDequeuedAt := time.Date(2026, 8, 25, 11, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		db   *schema.PipelineProgress
		want domain.PipelineProgress
	}{
		{
			name: "archive lane progress",
			db: &schema.PipelineProgress{
				IsUrgent:       false,
				LastDequeuedAt: lastDequeuedAt,
			},
			want: domain.PipelineProgress{
				IsUrgent:       false,
				LastDequeuedAt: lastDequeuedAt,
			},
		},
		{
			name: "urgent lane progress",
			db: &schema.PipelineProgress{
				IsUrgent:       true,
				LastDequeuedAt: lastDequeuedAt,
			},
			want: domain.PipelineProgress{
				IsUrgent:       true,
				LastDequeuedAt: lastDequeuedAt,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var p domain.PipelineProgress
			p.FromDB(tt.db)

			require.Equal(t, tt.want, p)
		})
	}
}
