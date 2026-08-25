package service

var (
	// ErrVideoNotReadyForChapters — попытка создать главу или сдвинуть её начало у видео вне
	// статуса ready (§3, §4 дизайна эпика Э4): до готовности длительность видео неизвестна,
	// границу главы проверить нечем.
	ErrVideoNotReadyForChapters = NewConflictErrorCode("conflict.video_not_ready", "video is not ready for chapters")
	// ErrChapterStartInvalid — начало главы вне диапазона [0, duration_ms) видео (Э4-Т3).
	ErrChapterStartInvalid = NewValidationErrorCode(
		"validation.chapter_start", "chapter start_ms must be within [0, duration_ms) of the video",
	)
	// ErrChaptersLimit — у видео уже 100 глав, лимит на видео (Э4-Т3).
	ErrChaptersLimit = NewConflictErrorCode(
		"conflict.chapters_limit", "video already has the maximum number of chapters",
	)
	// ErrChapterStartTaken — у видео уже есть глава с таким start_ms (UNIQUE video_id, start_ms).
	ErrChapterStartTaken = NewConflictErrorCode(
		"conflict.chapter_start_taken", "chapter with this start_ms already exists for the video",
	)
)
