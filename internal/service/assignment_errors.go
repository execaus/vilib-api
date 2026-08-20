package service

var (
	// ErrVideoNotAssignable — видео в статусе failed или uploading нельзя назначить (§4
	// дизайна эпика Э3, шаг 2).
	ErrVideoNotAssignable = NewConflictErrorCode("conflict.video_not_assignable", "video is not assignable")
	// ErrAssignmentCancelled — операция над уже отменённым назначением (В-52).
	ErrAssignmentCancelled = NewConflictErrorCode("conflict.assignment_cancelled", "assignment already cancelled")
	// ErrParticipantCompleted — попытка снять с назначения уже завершившего обучение
	// участника (Э3-Н1, В-52).
	ErrParticipantCompleted = NewConflictErrorCode("conflict.participant_completed", "participant already completed")
	// ErrDueAtInvalid — срок в режиме "date" не задан или не в будущем (В-6 решение владельца).
	ErrDueAtInvalid = NewValidationErrorCode("validation.due_at", "due date must be in the future")
	// ErrDueDaysInvalid — число дней в режиме "days" вне диапазона [1;3650].
	ErrDueDaysInvalid = NewValidationErrorCode("validation.due_days", "due days must be between 1 and 3650")
	// ErrTargetsEmpty — ни пользователи, ни группы не переданы (§4 дизайна эпика Э3, шаг 4).
	ErrTargetsEmpty = NewValidationErrorCode("validation.targets", "at least one target is required")
	// ErrTargetGroupInvalid — цель-группа отличается от группы видео (решение О-1: в этом
	// эпике цель-группа — только группа видео).
	ErrTargetGroupInvalid = NewValidationErrorCode("validation.target_group", "target group must be the video group")
)
