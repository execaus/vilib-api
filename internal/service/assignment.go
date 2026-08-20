package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// assignmentMinDueDays/assignmentMaxDueDays — допустимый диапазон срока в режиме "days"
	// (§4 дизайна эпика Э3, шаг 3).
	assignmentMinDueDays = 1
	assignmentMaxDueDays = 3650
	// maxEventsPerEnrolledParticipant — сколько событий журнала может дать один зачисленный
	// участник: participant_enrolled и, если он уже досмотрел видео, participant_completed.
	maxEventsPerEnrolledParticipant = 2
)

// assignmentCandidate — кандидат в участники назначения на этапе раскрытия целей (§4 дизайна
// эпика Э3, шаг 5): пользователь вместе с тем, откуда он получен (лично или через группу).
type assignmentCandidate struct {
	User   domain.User
	Source domain.AssignmentParticipantSource
}

// assignmentCreatedEventPayload — детали события "created" журнала назначения (Э3-Т32).
type assignmentCreatedEventPayload struct {
	Users   []uuid.UUID `json:"users"`
	Groups  []uuid.UUID `json:"groups"`
	DueMode string      `json:"due_mode"`
	DueAt   *time.Time  `json:"due_at,omitempty"`
	DueDays *int        `json:"due_days,omitempty"`
}

// AssignmentService реализует сервис назначений обязательного обучения — создание, чтение
// карточки и «мои назначения» (§4 дизайна эпика Э3).
type AssignmentService struct {
	repo         repository.Assignment
	targets      repository.AssignmentTarget
	participants repository.AssignmentParticipant
	events       repository.AssignmentEvent
	progress     repository.WatchProgress
	video        repository.Video
	groupMembers repository.GroupMember
	srv          *Service
	cfg          config.VideoConfig
	// now — источник текущего времени; в проде time.Now, в тестах подменяется опцией
	// WithAssignmentNow для детерминированных проверок срока и зачисления.
	now func() time.Time
}

// AssignmentServiceOption настраивает AssignmentService сверх обязательных зависимостей
// конструктора.
type AssignmentServiceOption func(*AssignmentService)

// WithAssignmentNow подменяет источник текущего времени. Предназначена для тестов.
func WithAssignmentNow(now func() time.Time) AssignmentServiceOption {
	return func(s *AssignmentService) {
		s.now = now
	}
}

func NewAssignmentService(
	repo repository.Assignment,
	targets repository.AssignmentTarget,
	participants repository.AssignmentParticipant,
	events repository.AssignmentEvent,
	progress repository.WatchProgress,
	video repository.Video,
	groupMembers repository.GroupMember,
	srv *Service,
	cfg config.VideoConfig,
	opts ...AssignmentServiceOption,
) *AssignmentService {
	s := &AssignmentService{
		repo: repo, targets: targets, participants: participants, events: events,
		progress: progress, video: video, groupMembers: groupMembers, srv: srv, cfg: cfg,
		now: time.Now,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Create создаёт назначение видео пользователям и/или группе (§4 дизайна эпика Э3, шаги 1–9):
// проверяет права и статус видео, валидирует срок и цели, раскрывает цели в персональные
// записи (В-4: без доступа — rejected, не ошибка; В-11: уже просмотревшие — сразу completed),
// фиксирует журнал и возвращает карточку назначения тем же способом, что и Get.
func (s *AssignmentService) Create(
	ctx context.Context, accountID, initiatorID uuid.UUID, in domain.CreateAssignment,
) (domain.AssignmentDetails, []domain.RejectedTarget, error) {
	now := s.now()

	video, err := s.resolveAssignableVideo(ctx, accountID, initiatorID, in.VideoID)
	if err != nil {
		return domain.AssignmentDetails{}, nil, err
	}

	if err = validateAssignmentDue(in, now); err != nil {
		return domain.AssignmentDetails{}, nil, err
	}
	if err = validateAssignmentTargets(in, video.GroupID); err != nil {
		return domain.AssignmentDetails{}, nil, err
	}

	group, err := s.groupByID(ctx, video.GroupID)
	if err != nil {
		return domain.AssignmentDetails{}, nil, err
	}

	accepted, rejected, err := s.resolveTargets(ctx, accountID, video.GroupID, in)
	if err != nil {
		return domain.AssignmentDetails{}, nil, err
	}

	assignment, err := s.repo.Insert(
		ctx, accountID, video.ID, video.Name, video.GroupID, group.Name, initiatorID,
		in.DueMode, in.DueAt, in.DueDays, in.Comment,
	)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AssignmentDetails{}, nil, err
	}

	if err = s.insertTargets(ctx, assignment.ID, in); err != nil {
		return domain.AssignmentDetails{}, nil, err
	}

	participants, err := s.enrollParticipants(ctx, assignment, video, accepted, in, now)
	if err != nil {
		return domain.AssignmentDetails{}, nil, err
	}

	if err = s.recordCreateEvents(ctx, assignment, in, participants, rejected, initiatorID, now); err != nil {
		return domain.AssignmentDetails{}, nil, err
	}

	details, err := s.Get(ctx, accountID, initiatorID, assignment.ID)
	if err != nil {
		return domain.AssignmentDetails{}, nil, err
	}

	return details, rejected, nil
}

// resolveAssignableVideo проверяет право инициатора назначать обучение в области группы
// видео (§4 дизайна эпика Э3, шаг 1) и то, что видео можно назначать (шаг 2: не failed/
// uploading).
func (s *AssignmentService) resolveAssignableVideo(
	ctx context.Context, accountID, initiatorID, videoID uuid.UUID,
) (domain.Video, error) {
	video, err := s.video.Select(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	if err = s.srv.Access.CanManageAssignments(ctx, accountID, initiatorID, video.GroupID); err != nil {
		return domain.Video{}, err
	}

	if video.Status == domain.VideoStatusFailed || video.Status == domain.VideoStatusUploading {
		return domain.Video{}, ErrVideoNotAssignable
	}

	return *video, nil
}

// validateAssignmentDue проверяет обязательность и корректность срока (В-6 решение владельца,
// §4 дизайна эпика Э3, шаг 3): "date" — обязателен и строго в будущем; "days" — целое число
// от 1 до 3650.
func validateAssignmentDue(in domain.CreateAssignment, now time.Time) error {
	return validateDue(in.DueMode, in.DueAt, in.DueDays, now)
}

// validateDue — общая проверка срока для создания и изменения назначения: "date" — дата
// задана и строго в будущем; "days" — целое число от 1 до 3650.
func validateDue(mode domain.AssignmentDueMode, dueAt *time.Time, dueDays *int, now time.Time) error {
	switch mode {
	case domain.AssignmentDueModeDate:
		if dueAt == nil || !dueAt.After(now) {
			return ErrDueAtInvalid
		}
	case domain.AssignmentDueModeDays:
		if dueDays == nil || *dueDays < assignmentMinDueDays || *dueDays > assignmentMaxDueDays {
			return ErrDueDaysInvalid
		}
	default:
		return ErrDueAtInvalid
	}

	return nil
}

// validateAssignmentTargets проверяет, что цели непусты и что цель-группа — только группа
// видео (§4 дизайна эпика Э3, шаг 4; решение О-1).
func validateAssignmentTargets(in domain.CreateAssignment, videoGroupID uuid.UUID) error {
	if len(in.Users) == 0 && len(in.Groups) == 0 {
		return ErrTargetsEmpty
	}

	for _, groupID := range in.Groups {
		if groupID != videoGroupID {
			return ErrTargetGroupInvalid
		}
	}

	return nil
}

// groupByID выбирает группу по идентификатору без проверки прав — вызывается после того, как
// право уже подтверждено resolveAssignableVideo.
func (s *AssignmentService) groupByID(ctx context.Context, groupID uuid.UUID) (domain.UserGroup, error) {
	groups, err := s.srv.UserGroup.GetByID(ctx, groupID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.UserGroup{}, err
	}
	if len(groups) == 0 {
		return domain.UserGroup{}, ErrNotFound
	}

	return groups[0], nil
}

// resolveTargets раскрывает цели назначения (пользователей и, при наличии цели-группы, её
// участников) в кандидатов и делит их на принятых и отклонённых (§4 дизайна эпика Э3, шаг 5;
// В-4). Явный личный список приоритетнее группового — источник "personal" побеждает.
func (s *AssignmentService) resolveTargets(
	ctx context.Context, accountID, groupID uuid.UUID, in domain.CreateAssignment,
) ([]assignmentCandidate, []domain.RejectedTarget, error) {
	candidateIDs, sourceByID, err := s.collectCandidateIDs(ctx, groupID, in)
	if err != nil {
		return nil, nil, err
	}
	if len(candidateIDs) == 0 {
		return nil, nil, nil
	}

	users, err := s.srv.User.GetByIDs(ctx, candidateIDs)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, nil, err
	}
	userByID := make(map[uuid.UUID]domain.User, len(users))
	roleIDs := make([]uuid.UUID, 0, len(users))
	for _, u := range users {
		userByID[u.ID] = u
		roleIDs = append(roleIDs, u.RoleID)
	}

	accountByRole, err := s.accountIDsByRole(ctx, roleIDs)
	if err != nil {
		return nil, nil, err
	}

	accepted := make([]assignmentCandidate, 0, len(candidateIDs))
	rejected := make([]domain.RejectedTarget, 0)
	for _, id := range candidateIDs {
		user, ok := userByID[id]
		switch {
		case !ok:
			rejected = append(rejected, domain.RejectedTarget{UserID: id, Reason: domain.RejectedReasonNotInAccount})
		case accountByRole[user.RoleID] != accountID:
			rejected = append(rejected, rejectedFromUser(user, domain.RejectedReasonNotInAccount))
		case !user.IsActive():
			rejected = append(rejected, rejectedFromUser(user, domain.RejectedReasonInactive))
		case !s.srv.Access.CanWatchVideo(ctx, accountID, id, groupID):
			rejected = append(rejected, rejectedFromUser(user, domain.RejectedReasonNoAccess))
		default:
			accepted = append(accepted, assignmentCandidate{User: user, Source: sourceByID[id]})
		}
	}

	return accepted, rejected, nil
}

// collectCandidateIDs собирает уникальных кандидатов из личного списка и, при наличии
// цели-группы, её текущих участников — личный список приоритетнее (source=personal).
func (s *AssignmentService) collectCandidateIDs(
	ctx context.Context, groupID uuid.UUID, in domain.CreateAssignment,
) ([]uuid.UUID, map[uuid.UUID]domain.AssignmentParticipantSource, error) {
	sourceByID := make(map[uuid.UUID]domain.AssignmentParticipantSource, len(in.Users))
	ids := make([]uuid.UUID, 0, len(in.Users))

	for _, id := range in.Users {
		if _, exists := sourceByID[id]; exists {
			continue
		}
		sourceByID[id] = domain.AssignmentParticipantSourcePersonal
		ids = append(ids, id)
	}

	if len(in.Groups) == 0 {
		return ids, sourceByID, nil
	}

	members, err := s.groupMembers.SelectByGroupID(ctx, groupID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, nil, err
	}
	for _, member := range members {
		if _, exists := sourceByID[member.UserID]; exists {
			continue
		}
		sourceByID[member.UserID] = domain.AssignmentParticipantSourceGroup
		ids = append(ids, member.UserID)
	}

	return ids, sourceByID, nil
}

// accountIDsByRole батчем резолвит принадлежность ролей аккаунту — по одному запросу вместо
// N (§4 дизайна эпика Э3, «батчи вместо JOIN»).
func (s *AssignmentService) accountIDsByRole(
	ctx context.Context,
	roleIDs []uuid.UUID,
) (map[uuid.UUID]uuid.UUID, error) {
	unique := dedupeUUIDs(roleIDs)
	if len(unique) == 0 {
		return map[uuid.UUID]uuid.UUID{}, nil
	}

	roles, err := s.srv.AccountRole.GetByID(ctx, unique...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	result := make(map[uuid.UUID]uuid.UUID, len(roles))
	for _, role := range roles {
		result[role.ID] = role.AccountID
	}

	return result, nil
}

// rejectedFromUser собирает RejectedTarget с ФИО/email из уже загруженного пользователя —
// без повторного запроса на уровне DTO.
func rejectedFromUser(user domain.User, reason domain.RejectedReason) domain.RejectedTarget {
	return domain.RejectedTarget{
		UserID: user.ID, Name: user.Name, Surname: user.Surname, Email: user.Email, Reason: reason,
	}
}

// insertTargets сохраняет цели назначения — пользователей и группы из запроса (§4 дизайна
// эпика Э3, шаг 6).
func (s *AssignmentService) insertTargets(
	ctx context.Context,
	assignmentID uuid.UUID,
	in domain.CreateAssignment,
) error {
	targets := make([]domain.AssignmentTarget, 0, len(in.Users)+len(in.Groups))
	for _, id := range dedupeUUIDs(in.Users) {
		targets = append(targets, domain.AssignmentTarget{
			AssignmentID: assignmentID, TargetType: domain.AssignmentTargetTypeUser, TargetID: id,
		})
	}
	for _, id := range dedupeUUIDs(in.Groups) {
		targets = append(targets, domain.AssignmentTarget{
			AssignmentID: assignmentID, TargetType: domain.AssignmentTargetTypeGroup, TargetID: id,
		})
	}

	if _, err := s.targets.InsertBatch(ctx, targets); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// enrollParticipants создаёт персональные записи принятых кандидатов (§4 дизайна эпика Э3,
// шаг 7): персональный срок — из режима назначения; начальный статус — по уже накопленному
// прогрессу просмотра (В-11: покрытие порога до назначения даёт сразу completed).
func (s *AssignmentService) enrollParticipants(
	ctx context.Context,
	assignment domain.Assignment, video domain.Video,
	accepted []assignmentCandidate, in domain.CreateAssignment, now time.Time,
) ([]domain.AssignmentParticipant, error) {
	if len(accepted) == 0 {
		return nil, nil
	}

	progressByUser, err := s.progressByUser(ctx, video.ID)
	if err != nil {
		return nil, err
	}

	thresholdPct := assignmentThresholdPercent(s.cfg.WatchCompletionThreshold)

	rows := make([]domain.AssignmentParticipant, len(accepted))
	for i, candidate := range accepted {
		participant := domain.AssignmentParticipant{
			AssignmentID: assignment.ID, UserID: candidate.User.ID,
			Source: candidate.Source, EnrolledAt: now,
			DueAt: assignmentDueAt(in, now),
		}
		if candidate.Source == domain.AssignmentParticipantSourceGroup {
			groupID := video.GroupID
			participant.SourceGroupID = &groupID
		}

		applyInitialParticipantProgress(&participant, progressByUser[candidate.User.ID], video.DurationMs, thresholdPct)

		rows[i] = participant
	}

	inserted, err := s.participants.InsertBatch(ctx, rows)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return inserted, nil
}

// progressByUser выбирает прогресс просмотра всех пользователей по одному видео одним
// запросом (WatchProgress.SelectByVideoIDs с единственным элементом).
func (s *AssignmentService) progressByUser(
	ctx context.Context,
	videoID uuid.UUID,
) (map[uuid.UUID]domain.WatchProgress, error) {
	progresses, err := s.progress.SelectByVideoIDs(ctx, []uuid.UUID{videoID})
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	byUser := make(map[uuid.UUID]domain.WatchProgress, len(progresses))
	for _, p := range progresses {
		byUser[p.UserID] = p
	}

	return byUser, nil
}

// assignmentDueAt вычисляет персональный срок участника по режиму назначения: "date" — общий
// due_at; "days" — enrolled_at (здесь — момент зачисления) плюс due_days.
func assignmentDueAt(in domain.CreateAssignment, enrolledAt time.Time) time.Time {
	return participantDueAt(in.DueMode, in.DueAt, in.DueDays, enrolledAt)
}

// participantDueAt вычисляет персональный срок по режиму назначения — общая формула для
// создания назначения и зачисления новичков группы (каскад OnMembersAdded).
func participantDueAt(
	mode domain.AssignmentDueMode, dueAt *time.Time, dueDays *int, enrolledAt time.Time,
) time.Time {
	if mode == domain.AssignmentDueModeDays && dueDays != nil {
		return enrolledAt.AddDate(0, 0, *dueDays)
	}
	if dueAt != nil {
		return *dueAt
	}

	return enrolledAt
}

// applyInitialParticipantProgress выставляет начальный статус персональной записи по уже
// накопленному прогрессу просмотра (§3.3 Э3-Т18, В-11 решение владельца): порог достигнут до
// назначения — сразу completed с completed_at = момент достижения порога; есть покрытие, но
// порог не достигнут — in_progress; иначе — assigned.
func applyInitialParticipantProgress(
	p *domain.AssignmentParticipant, progress domain.WatchProgress, durationMs *int64, thresholdPct int,
) {
	if progress.ThresholdReachedAt != nil {
		p.Status = domain.AssignmentParticipantStatusCompleted
		completedAt := *progress.ThresholdReachedAt
		p.CompletedAt = &completedAt
		coverage := coveragePercent(progress.CoveredMs, durationMs)
		p.CompletedCoveragePct = &coverage
		threshold := thresholdPct
		p.CompletedThresholdPct = &threshold

		return
	}

	if progress.CoveredMs > 0 {
		p.Status = domain.AssignmentParticipantStatusInProgress
		return
	}

	p.Status = domain.AssignmentParticipantStatusAssigned
}

// assignmentThresholdPercent переводит долю порога зачёта в целый процент — версия правила,
// сохраняемая в completed_threshold_pct при В-11 (Э3-Н1).
func assignmentThresholdPercent(threshold float64) int {
	return int(math.Round(threshold * watchPercentMultiplier))
}

// recordCreateEvents фиксирует журнал создания назначения (§4 дизайна эпика Э3, шаг 8):
// created, participant_enrolled на каждого принятого, participant_completed на зачтённых
// сразу по В-11, participant_rejected на каждого отклонённого.
func (s *AssignmentService) recordCreateEvents(
	ctx context.Context,
	assignment domain.Assignment, in domain.CreateAssignment,
	participants []domain.AssignmentParticipant, rejected []domain.RejectedTarget,
	initiatorID uuid.UUID, now time.Time,
) error {
	events, err := assignmentCreateEvents(assignment, in, participants, rejected, initiatorID, now)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if _, err = s.events.InsertBatch(ctx, events); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// assignmentCreateEvents собирает срез событий журнала для recordCreateEvents — вынесено в
// свободную функцию, чтобы держать сериализацию payload отдельно от работы с репозиторием.
func assignmentCreateEvents(
	assignment domain.Assignment, in domain.CreateAssignment,
	participants []domain.AssignmentParticipant, rejected []domain.RejectedTarget,
	initiatorID uuid.UUID, now time.Time,
) ([]domain.AssignmentEvent, error) {
	events := make([]domain.AssignmentEvent, 0, 1+2*len(participants)+len(rejected))

	createdPayload, err := json.Marshal(assignmentCreatedEventPayload{
		Users: in.Users, Groups: in.Groups, DueMode: string(in.DueMode), DueAt: in.DueAt, DueDays: in.DueDays,
	})
	if err != nil {
		return nil, err
	}
	events = append(events, domain.AssignmentEvent{
		AssignmentID: assignment.ID, Type: domain.AssignmentEventTypeCreated,
		ActorID: &initiatorID, Payload: createdPayload, CreatedAt: now,
	})

	for _, p := range participants {
		enrolled, enrollErr := participantEnrolledEvent(assignment.ID, p, initiatorID, now)
		if enrollErr != nil {
			return nil, enrollErr
		}
		events = append(events, enrolled)

		if p.Status != domain.AssignmentParticipantStatusCompleted {
			continue
		}
		completed, completeErr := participantCompletedEvent(assignment.ID, p, now)
		if completeErr != nil {
			return nil, completeErr
		}
		events = append(events, completed)
	}

	for _, r := range rejected {
		event, rejectErr := participantRejectedEvent(assignment.ID, r, initiatorID, now)
		if rejectErr != nil {
			return nil, rejectErr
		}
		events = append(events, event)
	}

	return events, nil
}

func participantEnrolledEvent(
	assignmentID uuid.UUID, p domain.AssignmentParticipant, actorID uuid.UUID, now time.Time,
) (domain.AssignmentEvent, error) {
	userID := p.UserID
	payload, err := json.Marshal(map[string]any{"source": p.Source})
	if err != nil {
		return domain.AssignmentEvent{}, err
	}

	return domain.AssignmentEvent{
		AssignmentID: assignmentID, UserID: &userID, Type: domain.AssignmentEventTypeParticipantEnrolled,
		ActorID: &actorID, Payload: payload, CreatedAt: now,
	}, nil
}

func participantCompletedEvent(
	assignmentID uuid.UUID, p domain.AssignmentParticipant, now time.Time,
) (domain.AssignmentEvent, error) {
	userID := p.UserID
	payload, err := json.Marshal(map[string]any{"coverage_pct": p.CompletedCoveragePct})
	if err != nil {
		return domain.AssignmentEvent{}, err
	}

	return domain.AssignmentEvent{
		AssignmentID: assignmentID, UserID: &userID, Type: domain.AssignmentEventTypeParticipantCompleted,
		Payload: payload, CreatedAt: now,
	}, nil
}

func participantRejectedEvent(
	assignmentID uuid.UUID, r domain.RejectedTarget, actorID uuid.UUID, now time.Time,
) (domain.AssignmentEvent, error) {
	userID := r.UserID
	payload, err := json.Marshal(map[string]any{"reason": r.Reason})
	if err != nil {
		return domain.AssignmentEvent{}, err
	}

	return domain.AssignmentEvent{
		AssignmentID: assignmentID, UserID: &userID, Type: domain.AssignmentEventTypeParticipantRejected,
		ActorID: &actorID, Payload: payload, CreatedAt: now,
	}, nil
}

// Get собирает карточку назначения целиком: цели, счётчики, участников с покрытием и
// признаком доступа, журнал (§4 дизайна эпика Э3). Право чтения — В-8: назначивший видит
// своё назначение всегда, иначе — область ManagedAssignmentGroups.
func (s *AssignmentService) Get(
	ctx context.Context, accountID, initiatorID, id uuid.UUID,
) (domain.AssignmentDetails, error) {
	assignment, err := s.repo.SelectByID(ctx, id)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AssignmentDetails{}, err
	}
	if assignment.AccountID != accountID {
		return domain.AssignmentDetails{}, ErrNotFound
	}

	if err = s.checkReadAccess(ctx, accountID, initiatorID, assignment); err != nil {
		return domain.AssignmentDetails{}, err
	}

	targets, err := s.targets.SelectByAssignmentIDs(ctx, []uuid.UUID{id})
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AssignmentDetails{}, err
	}
	targets, err = s.buildTargetDetails(ctx, assignment, targets)
	if err != nil {
		return domain.AssignmentDetails{}, err
	}

	participants, err := s.participants.SelectByAssignmentIDs(ctx, []uuid.UUID{id})
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AssignmentDetails{}, err
	}

	participantDetails, err := s.buildParticipantDetails(ctx, accountID, assignment, participants)
	if err != nil {
		return domain.AssignmentDetails{}, err
	}

	counters, err := s.participants.CountByAssignmentIDs(ctx, []uuid.UUID{id})
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AssignmentDetails{}, err
	}

	events, err := s.buildEventDetails(ctx, id)
	if err != nil {
		return domain.AssignmentDetails{}, err
	}

	createdByUsers, err := s.usersByID(ctx, []uuid.UUID{assignment.CreatedBy})
	if err != nil {
		return domain.AssignmentDetails{}, err
	}

	return domain.AssignmentDetails{
		Assignment: assignment, CreatedByUser: createdByUsers[assignment.CreatedBy],
		Targets: targets, Counters: counters[id],
		Participants: participantDetails, Events: events,
	}, nil
}

// checkReadAccess реализует правило чтения В-8 (§2 дизайна эпика Э3): назначивший видит своё
// назначение всегда; иначе — обладатель права назначения в области (аккаунт — всё, группа —
// своя, по текущему assignment.GroupID).
func (s *AssignmentService) checkReadAccess(
	ctx context.Context, accountID, initiatorID uuid.UUID, assignment domain.Assignment,
) error {
	if assignment.CreatedBy == initiatorID {
		return nil
	}

	all, groups, err := s.srv.Access.ManagedAssignmentGroups(ctx, accountID, initiatorID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}
	if all {
		return nil
	}
	if assignment.GroupID != nil && slices.Contains(groups, *assignment.GroupID) {
		return nil
	}

	return ErrForbidden
}

// buildTargetDetails резолвит отображаемые имена целей назначения (§5 контракта Э3): цель-
// группа — снимок имени группы видео (единственная допустимая цель-группа, решение О-1),
// пользователи — батч User.GetByIDs.
func (s *AssignmentService) buildTargetDetails(
	ctx context.Context, assignment domain.Assignment, targets []domain.AssignmentTarget,
) ([]domain.AssignmentTarget, error) {
	userIDs := make([]uuid.UUID, 0, len(targets))
	for _, t := range targets {
		if t.TargetType == domain.AssignmentTargetTypeUser {
			userIDs = append(userIDs, t.TargetID)
		}
	}

	userByID, err := s.usersByID(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	resolved := make([]domain.AssignmentTarget, len(targets))
	for i, t := range targets {
		t.Name = assignmentTargetName(t, assignment, userByID)
		resolved[i] = t
	}

	return resolved, nil
}

// assignmentTargetName вычисляет отображаемое имя одной цели.
func assignmentTargetName(
	t domain.AssignmentTarget, assignment domain.Assignment, userByID map[uuid.UUID]domain.User,
) string {
	if t.TargetType == domain.AssignmentTargetTypeGroup {
		return assignment.GroupName
	}
	if user, ok := userByID[t.TargetID]; ok {
		return strings.TrimSpace(user.Name + " " + user.Surname)
	}

	return ""
}

// buildParticipantDetails собирает участников карточки с данными пользователя, текущим
// покрытием и признаком доступа к видео (§4 дизайна эпика Э3, Get).
func (s *AssignmentService) buildParticipantDetails(
	ctx context.Context, accountID uuid.UUID, assignment domain.Assignment, participants []domain.AssignmentParticipant,
) ([]domain.ParticipantDetails, error) {
	if len(participants) == 0 {
		return nil, nil
	}

	userByID, err := s.usersByID(ctx, participantUserIDs(participants))
	if err != nil {
		return nil, err
	}

	progressByUser, durationMs := s.videoProgressForAssignment(ctx, assignment)

	details := make([]domain.ParticipantDetails, len(participants))
	for i, p := range participants {
		details[i] = domain.ParticipantDetails{
			Participant: p,
			User:        userByID[p.UserID],
			CoveragePct: participantCoveragePct(p, progressByUser, durationMs),
			HasAccess:   s.participantHasAccess(ctx, accountID, assignment, p),
		}
	}

	return details, nil
}

// participantUserIDs собирает идентификаторы пользователей участников назначения.
func participantUserIDs(participants []domain.AssignmentParticipant) []uuid.UUID {
	ids := make([]uuid.UUID, len(participants))
	for i, p := range participants {
		ids[i] = p.UserID
	}

	return ids
}

// usersByID батчем резолвит пользователей в карту по идентификатору.
func (s *AssignmentService) usersByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.User, error) {
	users, err := s.srv.User.GetByIDs(ctx, dedupeUUIDs(ids))
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	byID := make(map[uuid.UUID]domain.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}

	return byID, nil
}

// videoProgressForAssignment выбирает прогресс всех участников по видео назначения и его
// длительность — nil-карта и nil-длительность, если видео уже удалено (assignment.VideoID
// == nil) или запрос не удался (лог, но не фатально для карточки).
func (s *AssignmentService) videoProgressForAssignment(
	ctx context.Context, assignment domain.Assignment,
) (map[uuid.UUID]domain.WatchProgress, *int64) {
	if assignment.VideoID == nil {
		return nil, nil
	}

	var durationMs *int64
	if video, err := s.video.Select(ctx, *assignment.VideoID); err == nil {
		durationMs = video.DurationMs
	} else {
		zap.L().Warn(err.Error())
	}

	progresses, err := s.progress.SelectByVideoIDs(ctx, []uuid.UUID{*assignment.VideoID})
	if err != nil {
		zap.L().Error(err.Error())
		return nil, durationMs
	}

	byUser := make(map[uuid.UUID]domain.WatchProgress, len(progresses))
	for _, p := range progresses {
		byUser[p.UserID] = p
	}

	return byUser, durationMs
}

// participantCoveragePct — покрытие для отображения: у завершивших — зафиксированный процент
// на момент подтверждения (неизменяем, Э3-Н1), иначе — текущий прогресс просмотра.
func participantCoveragePct(
	p domain.AssignmentParticipant, progressByUser map[uuid.UUID]domain.WatchProgress, durationMs *int64,
) int {
	if p.Status == domain.AssignmentParticipantStatusCompleted {
		if p.CompletedCoveragePct != nil {
			return *p.CompletedCoveragePct
		}
		return 0
	}

	if progress, ok := progressByUser[p.UserID]; ok {
		return coveragePercent(progress.CoveredMs, durationMs)
	}

	return 0
}

// participantHasAccess вычисляет признак «нет доступа» только для незавершённых активных
// участников (§4 дизайна эпика Э3, Get) — доступ выполнивших/отменённых не имеет значения.
func (s *AssignmentService) participantHasAccess(
	ctx context.Context, accountID uuid.UUID, assignment domain.Assignment, p domain.AssignmentParticipant,
) bool {
	active := p.Status == domain.AssignmentParticipantStatusAssigned ||
		p.Status == domain.AssignmentParticipantStatusInProgress
	if !active || assignment.GroupID == nil {
		return false
	}

	return s.srv.Access.CanWatchVideo(ctx, accountID, p.UserID, *assignment.GroupID)
}

// buildEventDetails резолвит инициаторов событий журнала батчем (§4 дизайна эпика Э3, Get).
func (s *AssignmentService) buildEventDetails(ctx context.Context, id uuid.UUID) ([]domain.EventDetails, error) {
	events, err := s.events.SelectByAssignmentID(ctx, id)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}

	actorIDs := make([]uuid.UUID, 0, len(events))
	for _, e := range events {
		if e.ActorID != nil {
			actorIDs = append(actorIDs, *e.ActorID)
		}
	}
	actorByID, err := s.usersByID(ctx, actorIDs)
	if err != nil {
		return nil, err
	}

	details := make([]domain.EventDetails, len(events))
	for i, e := range events {
		detail := domain.EventDetails{Event: e}
		if e.ActorID != nil {
			if actor, ok := actorByID[*e.ActorID]; ok {
				detail.Actor = &actor
			}
		}
		details[i] = detail
	}

	return details, nil
}

// ListMine собирает «мои назначения» пользователя — все участия во всех статусах вместе с
// назначением, текущим состоянием видео (снимок, если видео удалено) и автором (§4 дизайна
// эпика Э3).
func (s *AssignmentService) ListMine(ctx context.Context, userID uuid.UUID) ([]domain.MyAssignment, error) {
	participants, err := s.participants.SelectByUserID(ctx, userID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}
	if len(participants) == 0 {
		return nil, nil
	}

	assignmentByID, err := s.assignmentsByIDs(ctx, participants)
	if err != nil {
		return nil, err
	}

	videoIDs, authorIDs := assignmentVideoAndAuthorIDs(assignmentByID)

	videoByID, err := s.videosByIDs(ctx, videoIDs)
	if err != nil {
		return nil, err
	}

	authorByID, err := s.usersByID(ctx, authorIDs)
	if err != nil {
		return nil, err
	}

	progressByVideo, err := s.progressByVideoForUser(ctx, userID, videoIDs)
	if err != nil {
		return nil, err
	}

	result := make([]domain.MyAssignment, len(participants))
	for i, p := range participants {
		result[i] = buildMyAssignment(p, assignmentByID[p.AssignmentID], videoByID, authorByID, progressByVideo)
	}

	return result, nil
}

// assignmentsByIDs батчем выбирает назначения участий в карту по идентификатору.
func (s *AssignmentService) assignmentsByIDs(
	ctx context.Context, participants []domain.AssignmentParticipant,
) (map[uuid.UUID]domain.Assignment, error) {
	ids := make([]uuid.UUID, len(participants))
	for i, p := range participants {
		ids[i] = p.AssignmentID
	}

	assignments, err := s.repo.SelectByIDs(ctx, dedupeUUIDs(ids))
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	byID := make(map[uuid.UUID]domain.Assignment, len(assignments))
	for _, a := range assignments {
		byID[a.ID] = a
	}

	return byID, nil
}

// assignmentVideoAndAuthorIDs собирает уникальные идентификаторы видео (без удалённых) и
// авторов назначений.
func assignmentVideoAndAuthorIDs(assignmentByID map[uuid.UUID]domain.Assignment) ([]uuid.UUID, []uuid.UUID) {
	videoIDs := make([]uuid.UUID, 0, len(assignmentByID))
	authorIDs := make([]uuid.UUID, 0, len(assignmentByID))
	for _, a := range assignmentByID {
		if a.VideoID != nil {
			videoIDs = append(videoIDs, *a.VideoID)
		}
		authorIDs = append(authorIDs, a.CreatedBy)
	}

	return videoIDs, authorIDs
}

// videosByIDs батчем выбирает текущее состояние видео назначений в карту по идентификатору.
func (s *AssignmentService) videosByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.Video, error) {
	videos, err := s.video.SelectByIDs(ctx, dedupeUUIDs(ids))
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	byID := make(map[uuid.UUID]domain.Video, len(videos))
	for _, v := range videos {
		byID[v.ID] = v
	}

	return byID, nil
}

// progressByVideoForUser выбирает прогресс пользователя по всем видео его назначений одним
// запросом.
func (s *AssignmentService) progressByVideoForUser(
	ctx context.Context, userID uuid.UUID, videoIDs []uuid.UUID,
) (map[uuid.UUID]domain.WatchProgress, error) {
	progresses, err := s.progress.SelectByUserAndVideoIDs(ctx, userID, dedupeUUIDs(videoIDs))
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	byVideo := make(map[uuid.UUID]domain.WatchProgress, len(progresses))
	for _, p := range progresses {
		byVideo[p.VideoID] = p
	}

	return byVideo, nil
}

// buildMyAssignment собирает один элемент «моих назначений» — вынесено в свободную функцию
// для читаемости ListMine.
func buildMyAssignment(
	p domain.AssignmentParticipant, assignment domain.Assignment,
	videoByID map[uuid.UUID]domain.Video, authorByID map[uuid.UUID]domain.User,
	progressByVideo map[uuid.UUID]domain.WatchProgress,
) domain.MyAssignment {
	var (
		video      *domain.Video
		durationMs *int64
	)
	if assignment.VideoID != nil {
		if v, ok := videoByID[*assignment.VideoID]; ok {
			vv := v
			video = &vv
			durationMs = v.DurationMs
		}
	}

	coverage := 0
	switch {
	case p.Status == domain.AssignmentParticipantStatusCompleted && p.CompletedCoveragePct != nil:
		coverage = *p.CompletedCoveragePct
	case assignment.VideoID != nil:
		if progress, ok := progressByVideo[*assignment.VideoID]; ok {
			coverage = coveragePercent(progress.CoveredMs, durationMs)
		}
	}

	return domain.MyAssignment{
		Participant: p, Assignment: assignment, Video: video,
		AssignedBy: authorByID[assignment.CreatedBy], CoveragePct: coverage,
	}
}

// dedupeUUIDs возвращает уникальные идентификаторы в порядке первого появления.
func dedupeUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}

// assignmentDueChangedEventPayload — детали события "due_changed" журнала назначения:
// прежний и новый срок в терминах режима (Э3-Т32).
type assignmentDueChangedEventPayload struct {
	OldDueMode string     `json:"old_due_mode"`
	OldDueAt   *time.Time `json:"old_due_at,omitempty"`
	OldDueDays *int       `json:"old_due_days,omitempty"`
	NewDueMode string     `json:"new_due_mode"`
	NewDueAt   *time.Time `json:"new_due_at,omitempty"`
	NewDueDays *int       `json:"new_due_days,omitempty"`
}

// UpdateDue меняет срок и/или комментарий назначения (§4 дизайна эпика Э3): отменённое
// назначение не редактируется (409), новый срок валидируется как при создании и
// пересчитывается всем незавершённым участникам — завершённые записи неприкосновенны
// (Э3-Н1, КП-9: completed_at не меняется ни одним методом).
func (s *AssignmentService) UpdateDue(
	ctx context.Context, accountID, initiatorID, id uuid.UUID, patch domain.UpdateAssignment,
) (domain.AssignmentDetails, error) {
	now := s.now()

	assignment, err := s.assignmentForManage(ctx, accountID, initiatorID, id)
	if err != nil {
		return domain.AssignmentDetails{}, err
	}
	if assignment.Status == domain.AssignmentStatusCancelled {
		return domain.AssignmentDetails{}, ErrAssignmentCancelled
	}

	if patch.HasDue() {
		if err = s.applyDuePatch(ctx, assignment, patch, initiatorID, now); err != nil {
			return domain.AssignmentDetails{}, err
		}
	}

	if patch.Comment != nil {
		if _, err = s.repo.UpdateComment(ctx, id, *patch.Comment); err != nil {
			zap.L().Error(err.Error())
			return domain.AssignmentDetails{}, err
		}
	}

	return s.Get(ctx, accountID, initiatorID, id)
}

// applyDuePatch валидирует новый срок, сохраняет его в назначении, пересчитывает персональные
// сроки незавершённых участников и пишет событие due_changed.
func (s *AssignmentService) applyDuePatch(
	ctx context.Context,
	assignment domain.Assignment, patch domain.UpdateAssignment,
	initiatorID uuid.UUID, now time.Time,
) error {
	if err := validateDue(*patch.DueMode, patch.DueAt, patch.DueDays, now); err != nil {
		return err
	}

	if _, err := s.repo.UpdateDue(ctx, assignment.ID, *patch.DueMode, patch.DueAt, patch.DueDays); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if _, err := s.participants.UpdateDueByAssignment(
		ctx, assignment.ID, *patch.DueMode, patch.DueAt, patch.DueDays,
	); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	payload, err := json.Marshal(assignmentDueChangedEventPayload{
		OldDueMode: string(assignment.DueMode), OldDueAt: assignment.DueAt, OldDueDays: assignment.DueDays,
		NewDueMode: string(*patch.DueMode), NewDueAt: patch.DueAt, NewDueDays: patch.DueDays,
	})
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if _, err = s.events.Insert(
		ctx, assignment.ID, nil, domain.AssignmentEventTypeDueChanged, &initiatorID, payload, now,
	); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// Cancel отменяет назначение целиком (§4 дизайна эпика Э3): переводит в cancelled само
// назначение и всех незавершённых участников, пишет журнал. Повторная отмена — 409.
func (s *AssignmentService) Cancel(ctx context.Context, accountID, initiatorID, id uuid.UUID) error {
	now := s.now()

	assignment, err := s.assignmentForManage(ctx, accountID, initiatorID, id)
	if err != nil {
		return err
	}
	if assignment.Status == domain.AssignmentStatusCancelled {
		return ErrAssignmentCancelled
	}

	cancelled, err := s.repo.Cancel(ctx, id, &initiatorID, domain.AssignmentCancelReasonManual, now)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}
	if !cancelled {
		// Гонка: назначение отменили между чтением и записью — для клиента это тот же 409.
		return ErrAssignmentCancelled
	}

	return s.cancelParticipantsWithEvents(
		ctx, id, domain.AssignmentParticipantCancelReasonAssignmentCancelled, &initiatorID, now,
	)
}

// RemoveParticipant снимает одного участника с назначения (§4 дизайна эпика Э3): завершившего
// обучение снять нельзя (409 — Э3-Н1), отсутствующего — 404, у отменённого назначения — 409.
func (s *AssignmentService) RemoveParticipant(
	ctx context.Context, accountID, initiatorID, id, userID uuid.UUID,
) error {
	now := s.now()

	assignment, err := s.assignmentForManage(ctx, accountID, initiatorID, id)
	if err != nil {
		return err
	}
	if assignment.Status == domain.AssignmentStatusCancelled {
		return ErrAssignmentCancelled
	}

	participant, err := s.participants.SelectByAssignmentIDAndUserID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		zap.L().Error(err.Error())

		return err
	}
	if participant.Status == domain.AssignmentParticipantStatusCompleted {
		return ErrParticipantCompleted
	}

	removed, err := s.participants.CancelOne(
		ctx, id, userID, domain.AssignmentParticipantCancelReasonRemovedByManager, now,
	)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}
	if !removed {
		// Участник уже снят или завершил обучение в параллельной транзакции.
		return ErrParticipantCompleted
	}

	return s.recordParticipantCancelledEvents(
		ctx, id, []uuid.UUID{userID},
		domain.AssignmentParticipantCancelReasonRemovedByManager, &initiatorID, now,
	)
}

// assignmentForManage читает назначение и проверяет право управления им (§2 дизайна эпика Э3,
// В-8): область определяется группой видео на момент создания; если группа уже удалена,
// управлять назначением может только обладатель аккаунтного права.
func (s *AssignmentService) assignmentForManage(
	ctx context.Context, accountID, initiatorID, id uuid.UUID,
) (domain.Assignment, error) {
	assignment, err := s.repo.SelectByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.Assignment{}, ErrNotFound
		}
		zap.L().Error(err.Error())

		return domain.Assignment{}, err
	}
	if assignment.AccountID != accountID {
		return domain.Assignment{}, ErrNotFound
	}

	if assignment.GroupID != nil {
		if err = s.srv.Access.CanManageAssignments(ctx, accountID, initiatorID, *assignment.GroupID); err != nil {
			return domain.Assignment{}, err
		}

		return assignment, nil
	}

	all, _, err := s.srv.Access.ManagedAssignmentGroups(ctx, accountID, initiatorID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Assignment{}, err
	}
	if !all {
		return domain.Assignment{}, ErrForbidden
	}

	return assignment, nil
}

// cancelParticipantsWithEvents отменяет незавершённых участников назначения и пишет по
// событию на каждого — общий хвост ручной отмены и системных каскадов.
func (s *AssignmentService) cancelParticipantsWithEvents(
	ctx context.Context,
	assignmentID uuid.UUID,
	reason domain.AssignmentParticipantCancelReason,
	actorID *uuid.UUID, now time.Time,
) error {
	cancelledUsers, err := s.participants.CancelByAssignment(ctx, assignmentID, reason, now)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return s.recordParticipantCancelledEvents(ctx, assignmentID, cancelledUsers, reason, actorID, now)
}

// recordParticipantCancelledEvents пишет событие participant_cancelled на каждого отменённого
// участника (Э3-Т32). Событие отмены назначения целиком пишется отдельно — вызывающим кодом.
func (s *AssignmentService) recordParticipantCancelledEvents(
	ctx context.Context,
	assignmentID uuid.UUID, userIDs []uuid.UUID,
	reason domain.AssignmentParticipantCancelReason,
	actorID *uuid.UUID, now time.Time,
) error {
	if len(userIDs) == 0 {
		return nil
	}

	payload, err := json.Marshal(map[string]any{"reason": reason})
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	events := make([]domain.AssignmentEvent, len(userIDs))
	for i := range userIDs {
		events[i] = domain.AssignmentEvent{
			AssignmentID: assignmentID, UserID: &userIDs[i],
			Type:    domain.AssignmentEventTypeParticipantCancelled,
			ActorID: actorID, Payload: payload, CreatedAt: now,
		}
	}

	if _, err = s.events.InsertBatch(ctx, events); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// OnMembersAdded зачисляет новых участников группы в действующие назначения, адресованные
// этой группе (§4 «Каскады» дизайна эпика Э3, Э3-Т3): срок считается по правилу В-5 —
// в режиме «дата» истёкшее назначение новичку не выдаётся, в режиме «N дней» срок отсчитывается
// от момента зачисления. Метод системный: вызывается из саги добавления участников уже после
// проверки прав на управление группой, поэтому прав не проверяет.
func (s *AssignmentService) OnMembersAdded(ctx context.Context, groupID uuid.UUID, userIDs []uuid.UUID) error {
	if len(userIDs) == 0 {
		return nil
	}

	now := s.now()

	assignments, err := s.repo.SelectActiveByTargetGroup(ctx, groupID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	for _, assignment := range assignments {
		if !assignmentEnrollsNewcomers(assignment, now) {
			continue
		}
		if err = s.enrollGroupMembers(ctx, assignment, groupID, userIDs, now); err != nil {
			return err
		}
	}

	return nil
}

// assignmentEnrollsNewcomers сообщает, зачисляются ли в назначение новые участники группы:
// назначение с фиксированной датой после её наступления новичкам не выдаётся (В-5), у
// назначения без видео (видео удалено) зачислять нечего.
func assignmentEnrollsNewcomers(assignment domain.Assignment, now time.Time) bool {
	if assignment.VideoID == nil {
		return false
	}

	if assignment.DueMode == domain.AssignmentDueModeDate {
		return assignment.DueAt != nil && assignment.DueAt.After(now)
	}

	return true
}

// enrollGroupMembers создаёт персональные записи новых участников группы в одном назначении
// и пишет журнал. Повторно добавленный в группу сотрудник с отменённой записью реактивируется,
// а завершивший обучение не дублируется — это обеспечивает ON CONFLICT репозитория (Э3-Т30).
func (s *AssignmentService) enrollGroupMembers(
	ctx context.Context,
	assignment domain.Assignment, groupID uuid.UUID, userIDs []uuid.UUID, now time.Time,
) error {
	video, err := s.video.Select(ctx, *assignment.VideoID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	progressByUser, err := s.progressByUser(ctx, video.ID)
	if err != nil {
		return err
	}

	thresholdPct := assignmentThresholdPercent(s.cfg.WatchCompletionThreshold)
	sourceGroupID := groupID

	rows := make([]domain.AssignmentParticipant, len(userIDs))
	for i, userID := range userIDs {
		participant := domain.AssignmentParticipant{
			AssignmentID: assignment.ID, UserID: userID,
			Source: domain.AssignmentParticipantSourceGroup, SourceGroupID: &sourceGroupID,
			EnrolledAt: now,
			DueAt:      participantDueAt(assignment.DueMode, assignment.DueAt, assignment.DueDays, now),
		}
		applyInitialParticipantProgress(&participant, progressByUser[userID], video.DurationMs, thresholdPct)
		rows[i] = participant
	}

	enrolled, err := s.participants.InsertBatch(ctx, rows)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return s.recordGroupEnrolledEvents(ctx, assignment.ID, enrolled, now)
}

// recordGroupEnrolledEvents пишет журнал зачисления новичков группы: participant_enrolled с
// признаком joined (сотрудник добавлен в группу после создания назначения) и
// participant_completed для тех, кто досмотрел видео заранее (В-11).
func (s *AssignmentService) recordGroupEnrolledEvents(
	ctx context.Context, assignmentID uuid.UUID, participants []domain.AssignmentParticipant, now time.Time,
) error {
	if len(participants) == 0 {
		return nil
	}

	enrolledPayload, err := json.Marshal(map[string]any{
		"source": domain.AssignmentParticipantSourceGroup, "joined": true,
	})
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// На каждого участника приходится до двух событий: зачисление и, для В-11, подтверждение.
	events := make([]domain.AssignmentEvent, 0, maxEventsPerEnrolledParticipant*len(participants))
	for i := range participants {
		events = append(events, domain.AssignmentEvent{
			AssignmentID: assignmentID, UserID: &participants[i].UserID,
			Type: domain.AssignmentEventTypeParticipantEnrolled, Payload: enrolledPayload, CreatedAt: now,
		})

		if participants[i].Status != domain.AssignmentParticipantStatusCompleted {
			continue
		}

		completed, completeErr := participantCompletedEvent(assignmentID, participants[i], now)
		if completeErr != nil {
			zap.L().Error(completeErr.Error())
			return completeErr
		}
		events = append(events, completed)
	}

	if _, err = s.events.InsertBatch(ctx, events); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// OnMemberRemoved отменяет участия исключённого из группы сотрудника, полученные через эту
// группу (§4 «Каскады» дизайна эпика Э3, Э3-Т30): личные назначения не затрагиваются,
// завершённые записи остаются как есть. Метод системный — вызывается из саги удаления
// участника после проверки прав.
func (s *AssignmentService) OnMemberRemoved(ctx context.Context, groupID, userID uuid.UUID) error {
	now := s.now()

	assignmentIDs, err := s.participants.CancelBySourceGroupAndUser(
		ctx, groupID, userID, domain.AssignmentParticipantCancelReasonLeftGroup, now,
	)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	for _, assignmentID := range assignmentIDs {
		if err = s.recordParticipantCancelledEvents(
			ctx, assignmentID, []uuid.UUID{userID},
			domain.AssignmentParticipantCancelReasonLeftGroup, nil, now,
		); err != nil {
			return err
		}
	}

	return nil
}

// OnVideoDeleted отменяет действующие назначения удаляемого видео (§4 «Каскады» дизайна
// эпика Э3, Э3-Т28): незавершённые участия переходят в cancelled, завершённые остаются со
// снимком названия видео. Вызывается из саги удаления видео до удаления строки.
func (s *AssignmentService) OnVideoDeleted(ctx context.Context, videoID uuid.UUID) error {
	assignments, err := s.repo.SelectActiveByVideoIDs(ctx, []uuid.UUID{videoID})
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return s.cancelAssignments(
		ctx, assignments,
		domain.AssignmentCancelReasonVideoDeleted, domain.AssignmentParticipantCancelReasonVideoDeleted,
	)
}

// OnGroupDeleted отменяет действующие назначения видео удаляемой группы (§4 «Каскады» дизайна
// эпика Э3, Э3-Т31). Вызывается из саги удаления группы до каскадного удаления её данных.
func (s *AssignmentService) OnGroupDeleted(ctx context.Context, groupID uuid.UUID) error {
	assignments, err := s.repo.SelectActiveByGroupID(ctx, groupID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return s.cancelAssignments(
		ctx, assignments,
		domain.AssignmentCancelReasonGroupDeleted, domain.AssignmentParticipantCancelReasonGroupDeleted,
	)
}

// cancelAssignments отменяет перечисленные назначения системно (без инициатора) и пишет
// журнал: cancelled на назначение и participant_cancelled на каждого незавершённого участника.
func (s *AssignmentService) cancelAssignments(
	ctx context.Context,
	assignments []domain.Assignment,
	reason domain.AssignmentCancelReason,
	participantReason domain.AssignmentParticipantCancelReason,
) error {
	if len(assignments) == 0 {
		return nil
	}

	now := s.now()

	payload, err := json.Marshal(map[string]any{"reason": reason})
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	for _, assignment := range assignments {
		cancelled, cancelErr := s.repo.Cancel(ctx, assignment.ID, nil, reason, now)
		if cancelErr != nil {
			zap.L().Error(cancelErr.Error())
			return cancelErr
		}
		if !cancelled {
			continue
		}

		if _, err = s.events.Insert(
			ctx, assignment.ID, nil, domain.AssignmentEventTypeCancelled, nil, payload, now,
		); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		if err = s.cancelParticipantsWithEvents(ctx, assignment.ID, participantReason, nil, now); err != nil {
			return err
		}
	}

	return nil
}
