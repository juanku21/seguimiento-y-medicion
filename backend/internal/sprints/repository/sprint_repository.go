package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// errNotImplemented es el error de las firmas mínimas que crearon las pruebas de integración de T013;
// cada operación se implementa en la tarea de su historia (T017 en adelante).
var errNotImplemented = errors.New("not implemented")

// SprintRepository implementa domain.SprintRepository con GORM sobre las tablas sprints y
// sprint_stories. FindAccessible, Create, Start, AssignStories, RemoveStories y FindDetail están
// implementadas; las demás operaciones conservan la firma mínima de T013 y se implementan en la tarea
// de su historia.
type SprintRepository struct {
	db *gorm.DB
}

// NewSprintRepository crea el repositorio de sprints sobre la conexión recibida.
func NewSprintRepository(db *gorm.DB) *SprintRepository {
	return &SprintRepository{db: db}
}

// sprintAccessRow es la fila que lee FindAccessible: los identificadores del sprint y de su proyecto,
// y el estado del proyecto que llega por el JOIN a projects con el alias project_status.
type sprintAccessRow struct {
	SprintID      uuid.UUID
	ProjectID     uuid.UUID
	ProjectStatus projectsdomain.ProjectStatus
}

// FindAccessible resuelve el acceso a un sprint para RequireSprintAccess con una sola consulta
// (research.md sección 3): une sprints con la membresía vigente del usuario en project_members y con
// projects. Si no hay fila, el sprint no existe o el usuario no es integrante de su proyecto, y ambos
// casos se reportan igual con ErrSprintNotFound, nunca gorm.ErrRecordNotFound ni (nil, nil). El
// estado del proyecto se compara con ProjectStatusFinished para la regla de solo lectura (FR-059).
func (r *SprintRepository) FindAccessible(ctx context.Context, sprintID, userID uuid.UUID) (*domain.SprintAccess, error) {
	var row sprintAccessRow
	err := r.db.WithContext(ctx).
		Table("sprints").
		Select(`sprints.id AS sprint_id, sprints.project_id AS project_id,
			projects.status AS project_status`).
		Joins(`JOIN project_members ON project_members.project_id = sprints.project_id
			AND project_members.user_id = ?`, userID).
		Joins("JOIN projects ON projects.id = sprints.project_id").
		Where("sprints.id = ?", sprintID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSprintNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolver el acceso al sprint: %w", err)
	}
	return &domain.SprintAccess{
		SprintID:        row.SprintID,
		ProjectID:       row.ProjectID,
		ProjectFinished: row.ProjectStatus == projectsdomain.ProjectStatusFinished,
	}, nil
}

// Create registra el sprint en una transacción (research.md sección 4). Primero toma un bloqueo
// FOR UPDATE sobre la fila del proyecto y lee su estado: así las escrituras de calendario de un mismo
// proyecto quedan serializadas y un proyecto Finalizado se rechaza con ErrSprintsReadOnly (FR-059).
// Después busca, entre todos los sprints del proyecto y cualquiera sea su estado, uno cuyo período se
// superponga con el nuevo, con la traducción de PeriodsOverlap a desigualdades estrictas; si lo hay,
// devuelve SprintOverlapError con el de inicio más temprano (y luego menor id), para que el mensaje
// sea determinista (FR-005); se lee con Find y LIMIT 1 para que la ausencia de conflicto, que es el
// caso normal, no pase por gorm.ErrRecordNotFound. Si no, inserta el sprint; CreatedAt y UpdatedAt
// los completa GORM.
func (r *SprintRepository) Create(ctx context.Context, sprint *domain.Sprint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockWritableProject(tx, sprint.ProjectID); err != nil {
			return err
		}

		var conflicts []domain.ConflictingSprint
		err := tx.
			Model(&domain.Sprint{}).
			Select("id, name").
			Where("project_id = ?", sprint.ProjectID).
			Where("start_date < ? AND ? < end_date", sprint.EndDate, sprint.StartDate).
			Order("start_date ASC, id ASC").
			Limit(1).
			Find(&conflicts).Error
		if err != nil {
			return fmt.Errorf("buscar sprints superpuestos: %w", err)
		}
		if len(conflicts) > 0 {
			return &domain.SprintOverlapError{Sprint: conflicts[0]}
		}

		if err := tx.Create(sprint).Error; err != nil {
			return fmt.Errorf("insertar sprint: %w", err)
		}
		return nil
	})
}

// Update modifica un sprint planificado. Firma mínima.
func (r *SprintRepository) Update(ctx context.Context, sprintID uuid.UUID, input domain.SprintInput) error {
	return errNotImplemented
}

// Delete elimina un sprint planificado. Firma mínima.
func (r *SprintRepository) Delete(ctx context.Context, sprintID uuid.UUID) error {
	return errNotImplemented
}

// Start inicia el sprint en una transacción (research.md sección 5). Primero lee el proyecto del
// sprint, que nunca cambia, y bloquea con FOR UPDATE la fila del proyecto, igual que Create, para que
// los inicios de un mismo proyecto queden serializados; después bloquea el sprint y relee su estado,
// en ese orden (proyecto y luego sprint) para no interbloquearse con las demás escrituras. Con los
// datos releídos rechaza el proyecto Finalizado con ErrSprintsReadOnly (FR-059), el sprint Cerrado
// con ErrSprintClosed y cualquier otra transición que no sea planned → active con
// ValidateTransition. Si el proyecto ya tiene otro sprint activo devuelve ActiveSprintExistsError con
// ese sprint (FR-027). Si no, fija status = active, started_at = now y committed_story_points con la
// suma de los Story Points de sus compromisos, cero si no tiene (FR-028). El índice único parcial de
// un activo por proyecto es el respaldo: si la escritura lo viola, se deshace hasta el punto de
// guardado previo y se informa el mismo ActiveSprintExistsError con el sprint activo releído.
func (r *SprintRepository) Start(ctx context.Context, sprintID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sprint, err := lockProjectAndSprint(tx, sprintID)
		if err != nil {
			return err
		}
		if sprint.Status == domain.SprintStatusClosed {
			return domain.ErrSprintClosed
		}
		if err := sprint.Status.ValidateTransition(domain.SprintStatusActive); err != nil {
			return err
		}

		active, err := otherActiveSprint(tx, sprint.ProjectID, sprintID)
		if err != nil {
			return err
		}
		if active != nil {
			return &domain.ActiveSprintExistsError{Sprint: *active}
		}

		// Los Story Points comprometidos se congelan con la suma de las historias vivas del sprint.
		var committedPoints int
		err = tx.
			Table("sprint_stories").
			Select("COALESCE(SUM(user_stories.story_points), 0)").
			Joins("JOIN user_stories ON user_stories.id = sprint_stories.story_id").
			Where("sprint_stories.sprint_id = ?", sprintID).
			Scan(&committedPoints).Error
		if err != nil {
			return fmt.Errorf("sumar los story points comprometidos: %w", err)
		}

		// La actualización va en una transacción anidada (punto de guardado) para que, si el índice
		// parcial la rechaza, la transacción externa siga utilizable y pueda releer el sprint activo.
		err = tx.Transaction(func(inner *gorm.DB) error {
			return inner.
				Model(&domain.Sprint{}).
				Where("id = ?", sprintID).
				Updates(map[string]any{
					"status":                 domain.SprintStatusActive,
					"started_at":             now,
					"committed_story_points": committedPoints,
				}).Error
		})
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return activeSprintExistsError(tx, sprint.ProjectID, sprintID, err)
		}
		if err != nil {
			return fmt.Errorf("iniciar el sprint: %w", err)
		}
		return nil
	})
}

// lockWritableProject bloquea con FOR UPDATE, dentro de la transacción recibida, la fila del proyecto
// para serializar las escrituras de calendario de ese proyecto, y con el estado releído rechaza el
// proyecto Finalizado con ErrSprintsReadOnly (FR-059). La comparten Create y Start.
func lockWritableProject(tx *gorm.DB, projectID uuid.UUID) error {
	var project projectsdomain.Project
	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("status").
		Where("id = ?", projectID).
		Take(&project).Error
	if err != nil {
		return fmt.Errorf("bloquear el proyecto del sprint: %w", err)
	}
	if project.Status == projectsdomain.ProjectStatusFinished {
		return domain.ErrSprintsReadOnly
	}
	return nil
}

// lockProjectAndSprint toma, dentro de la transacción recibida, los bloqueos del inicio: lee el
// proyecto del sprint sin bloquear (si no existe devuelve ErrSprintNotFound), bloquea el proyecto con
// lockWritableProject y por último bloquea la fila del sprint y relee su estado ya serializado.
func lockProjectAndSprint(tx *gorm.DB, sprintID uuid.UUID) (*domain.Sprint, error) {
	var owner domain.Sprint
	err := tx.Select("project_id").Where("id = ?", sprintID).Take(&owner).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSprintNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("leer el proyecto del sprint: %w", err)
	}

	if err := lockWritableProject(tx, owner.ProjectID); err != nil {
		return nil, err
	}

	var sprint domain.Sprint
	err = tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id, project_id, status").
		Where("id = ?", sprintID).
		Take(&sprint).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSprintNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("bloquear el sprint: %w", err)
	}
	return &sprint, nil
}

// otherActiveSprint devuelve el sprint activo del proyecto distinto del indicado, o nil si no hay
// ninguno; se lee con Find y LIMIT 1 para que la ausencia, que es el caso normal, no pase por
// gorm.ErrRecordNotFound.
func otherActiveSprint(tx *gorm.DB, projectID, sprintID uuid.UUID) (*domain.ConflictingSprint, error) {
	var active []domain.ConflictingSprint
	err := tx.
		Model(&domain.Sprint{}).
		Select("id, name").
		Where("project_id = ? AND status = ? AND id <> ?", projectID, domain.SprintStatusActive, sprintID).
		Limit(1).
		Find(&active).Error
	if err != nil {
		return nil, fmt.Errorf("buscar otro sprint activo del proyecto: %w", err)
	}
	if len(active) == 0 {
		return nil, nil
	}
	return &active[0], nil
}

// activeSprintExistsError traduce la violación del índice parcial de un activo por proyecto a
// ActiveSprintExistsError con el sprint activo releído. Si la relectura no encuentra ninguno,
// devuelve envuelto el error original de la actualización.
func activeSprintExistsError(tx *gorm.DB, projectID, sprintID uuid.UUID, updateErr error) error {
	active, err := otherActiveSprint(tx, projectID, sprintID)
	if err != nil {
		return err
	}
	if active == nil {
		return fmt.Errorf("iniciar el sprint: %w", updateErr)
	}
	return &domain.ActiveSprintExistsError{Sprint: *active}
}

// Close cierra un sprint. Firma mínima.
func (r *SprintRepository) Close(ctx context.Context, sprintID, closedBy uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// AssignStories compromete un lote de historias en el sprint, todo o nada, en una transacción
// (research.md sección 8). Bloquea el sprint y las historias con lockSprintAndStories, que además
// rechaza el proyecto Finalizado y el sprint Cerrado, y relee de cada historia sus criterios de
// aceptación y su compromiso abierto, sea en otro sprint o en este mismo; con esos datos evalúa
// Admission y, si alguna tiene problemas, devuelve StoriesNotAdmittedError con todos sin escribir
// nada (FR-023). Si no, inserta una fila de sprint_stories por historia con committed_at = now y
// marca ever_committed. El índice parcial de un compromiso abierto por historia es el respaldo de la
// regla: si la inserción lo viola, se deshace hasta el punto de guardado previo y la violación se
// informa como committed_elsewhere con los compromisos releídos. ever_committed se escribe con UpdateColumn para no tocar updated_at, porque
// esta feature solo escribe status y ever_committed de user_stories (data-model.md sección 3).
func (r *SprintRepository) AssignStories(ctx context.Context, sprintID uuid.UUID, storyIDs []uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		stories, err := lockSprintAndStories(tx, sprintID, storyIDs)
		if err != nil {
			return err
		}

		criteria, err := criteriaCounts(tx, storyIDs)
		if err != nil {
			return err
		}
		commitments, err := openCommitments(tx, storyIDs)
		if err != nil {
			return err
		}

		// Cada historia se evalúa con Admission y se acumulan los problemas de todas, para que el
		// rechazo del lote las liste completas.
		var problems []domain.AdmissionProblem
		for _, story := range stories {
			candidate := domain.AdmissionCandidate{
				ID:            story.ID,
				Status:        story.Status,
				StoryPoints:   story.StoryPoints,
				CriteriaCount: criteria[story.ID],
			}
			problems = append(problems, domain.Admission(candidate, commitments[story.ID])...)
		}
		if len(problems) > 0 {
			return &domain.StoriesNotAdmittedError{Problems: problems}
		}

		// La inserción va en una transacción anidada (punto de guardado) para que, si el índice
		// parcial la rechaza, la transacción externa siga utilizable y pueda releer los compromisos.
		rows := make([]domain.SprintStory, 0, len(stories))
		for _, story := range stories {
			rows = append(rows, domain.SprintStory{
				ID:          uuid.New(),
				SprintID:    sprintID,
				StoryID:     story.ID,
				CommittedAt: now,
			})
		}
		err = tx.Transaction(func(inner *gorm.DB) error {
			return inner.Create(&rows).Error
		})
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return committedElsewhereError(tx, storyIDs, err)
		}
		if err != nil {
			return fmt.Errorf("insertar los compromisos del sprint: %w", err)
		}

		err = tx.
			Model(&backlogdomain.UserStory{}).
			Where("id IN ?", storyIDs).
			UpdateColumn("ever_committed", true).Error
		if err != nil {
			return fmt.Errorf("marcar las historias como comprometidas alguna vez: %w", err)
		}
		return nil
	})
}

// RemoveStories quita un lote de historias del sprint, todo o nada, en una transacción (FR-021 a
// FR-023). Bloquea el sprint y las historias con lockSprintAndStories, que además rechaza el
// proyecto Finalizado y el sprint Cerrado; rechaza el lote con ErrCompletedStoryRemoval si alguna
// está Completada (RN-21) y con ErrStoryNotFound si alguna no está comprometida en este sprint. Si
// no, borra sus filas de sprint_stories y deja las historias en pending (FR-022); ever_committed no se toca y sigue en true (RC-06). status se escribe con
// UpdateColumn, igual que ever_committed en AssignStories, para no tocar updated_at.
func (r *SprintRepository) RemoveStories(ctx context.Context, sprintID uuid.UUID, storyIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		stories, err := lockSprintAndStories(tx, sprintID, storyIDs)
		if err != nil {
			return err
		}
		for _, story := range stories {
			if story.Status == backlogdomain.StoryStatusCompleted {
				return domain.ErrCompletedStoryRemoval
			}
		}

		// Todas las historias del lote tienen que estar comprometidas en este sprint; si falta
		// alguna, el lote se rechaza sin borrar nada.
		var committed int64
		err = tx.
			Model(&domain.SprintStory{}).
			Where("sprint_id = ? AND story_id IN ?", sprintID, storyIDs).
			Count(&committed).Error
		if err != nil {
			return fmt.Errorf("contar los compromisos del sprint: %w", err)
		}
		if committed != int64(len(storyIDs)) {
			return backlogdomain.ErrStoryNotFound
		}

		err = tx.
			Where("sprint_id = ? AND story_id IN ?", sprintID, storyIDs).
			Delete(&domain.SprintStory{}).Error
		if err != nil {
			return fmt.Errorf("borrar los compromisos del sprint: %w", err)
		}
		err = tx.
			Model(&backlogdomain.UserStory{}).
			Where("id IN ?", storyIDs).
			UpdateColumn("status", backlogdomain.StoryStatusPending).Error
		if err != nil {
			return fmt.Errorf("devolver las historias a pendiente: %w", err)
		}
		return nil
	})
}

// lockedSprintRow es la fila que lee lockSprintAndStories al bloquear el sprint: su proyecto, su
// estado y el estado del proyecto, que llega por el JOIN a projects con el alias project_status.
type lockedSprintRow struct {
	ProjectID     uuid.UUID
	Status        domain.SprintStatus
	ProjectStatus projectsdomain.ProjectStatus
}

// lockSprintAndStories toma, dentro de la transacción recibida, los bloqueos comunes de
// AssignStories y RemoveStories y verifica con los datos releídos que el sprint admita cambios
// (research.md sección 8, paso 1). Primero bloquea solo la fila del sprint (FOR UPDATE OF sprints) y,
// en la misma consulta, lee su proyecto, su estado y el estado del proyecto; si no existe devuelve
// ErrSprintNotFound. Después, en el mismo orden que el servicio, rechaza el proyecto Finalizado con
// ErrSprintsReadOnly (FR-059) y el sprint Cerrado con ErrSprintClosed (RN-09, RN-18). Por último
// bloquea con FOR UPDATE las historias pedidas de ese mismo proyecto, ordenadas por id para que dos
// lotes que comparten historias no se interbloqueen. Las historias de otro proyecto quedan fuera del
// filtro, así que si no se encuentran todas, porque alguna no existe o es ajena, devuelve
// ErrStoryNotFound.
func lockSprintAndStories(tx *gorm.DB, sprintID uuid.UUID, storyIDs []uuid.UUID) ([]backlogdomain.UserStory, error) {
	var sprint lockedSprintRow
	err := tx.
		Table("sprints").
		Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "sprints"}}).
		Select("sprints.project_id, sprints.status, projects.status AS project_status").
		Joins("JOIN projects ON projects.id = sprints.project_id").
		Where("sprints.id = ?", sprintID).
		Take(&sprint).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSprintNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("bloquear el sprint: %w", err)
	}
	if sprint.ProjectStatus == projectsdomain.ProjectStatusFinished {
		return nil, domain.ErrSprintsReadOnly
	}
	if sprint.Status == domain.SprintStatusClosed {
		return nil, domain.ErrSprintClosed
	}

	var stories []backlogdomain.UserStory
	err = tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id, status, story_points").
		Where("id IN ? AND project_id = ?", storyIDs, sprint.ProjectID).
		Order("id ASC").
		Find(&stories).Error
	if err != nil {
		return nil, fmt.Errorf("bloquear las historias del lote: %w", err)
	}
	if len(stories) != len(storyIDs) {
		return nil, backlogdomain.ErrStoryNotFound
	}
	return stories, nil
}

// storyCriteriaCount es la fila que lee criteriaCounts: una historia y su cantidad de criterios.
type storyCriteriaCount struct {
	StoryID uuid.UUID
	Count   int
}

// criteriaCounts devuelve la cantidad de criterios de aceptación de cada historia pedida en una sola
// consulta agrupada; una historia sin criterios no aparece en el mapa y vale cero al leerla.
func criteriaCounts(tx *gorm.DB, storyIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []storyCriteriaCount
	err := tx.
		Model(&backlogdomain.AcceptanceCriterion{}).
		Select("story_id, COUNT(*) AS count").
		Where("story_id IN ?", storyIDs).
		Group("story_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("contar los criterios de las historias: %w", err)
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		counts[row.StoryID] = row.Count
	}
	return counts, nil
}

// openCommitmentRow es la fila que lee openCommitments: una historia y el sprint de su compromiso
// abierto.
type openCommitmentRow struct {
	StoryID    uuid.UUID
	SprintID   uuid.UUID
	SprintName string
}

// openCommitments devuelve, para cada historia pedida que tenga un compromiso abierto (outcome IS
// NULL) en cualquier sprint, incluido el de la asignación en curso, el sprint de ese compromiso; las
// historias sin compromiso abierto no aparecen en el mapa y su valor es nil, como espera Admission.
func openCommitments(tx *gorm.DB, storyIDs []uuid.UUID) (map[uuid.UUID]*domain.ConflictingSprint, error) {
	var rows []openCommitmentRow
	err := tx.
		Model(&domain.SprintStory{}).
		Select("sprint_stories.story_id, sprints.id AS sprint_id, sprints.name AS sprint_name").
		Joins("JOIN sprints ON sprints.id = sprint_stories.sprint_id").
		Where("sprint_stories.story_id IN ? AND sprint_stories.outcome IS NULL", storyIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("leer los compromisos abiertos de las historias: %w", err)
	}
	commitments := make(map[uuid.UUID]*domain.ConflictingSprint, len(rows))
	for _, row := range rows {
		commitments[row.StoryID] = &domain.ConflictingSprint{ID: row.SprintID, Name: row.SprintName}
	}
	return commitments, nil
}

// committedElsewhereError traduce la violación del índice parcial de un compromiso abierto por
// historia a StoriesNotAdmittedError: relee los compromisos abiertos del lote, que ahora incluyen el
// de la asignación concurrente que ganó, y arma un problema committed_elsewhere por cada historia que
// lo tenga. Si la relectura no encuentra ninguno, devuelve envuelto el error original de la
// inserción.
func committedElsewhereError(tx *gorm.DB, storyIDs []uuid.UUID, insertErr error) error {
	commitments, err := openCommitments(tx, storyIDs)
	if err != nil {
		return err
	}
	var problems []domain.AdmissionProblem
	for _, id := range storyIDs {
		if sprint, ok := commitments[id]; ok {
			problems = append(problems, domain.AdmissionProblem{
				StoryID: id,
				Reason:  domain.AdmissionReasonCommittedElsewhere,
				Sprint:  sprint,
			})
		}
	}
	if len(problems) == 0 {
		return fmt.Errorf("insertar los compromisos del sprint: %w", insertErr)
	}
	return &domain.StoriesNotAdmittedError{Problems: problems}
}

// ChangeStoryProgress cambia el estado de una historia del sprint activo. Firma mínima.
func (r *SprintRepository) ChangeStoryProgress(ctx context.Context, storyID uuid.UUID, to backlogdomain.StoryStatus, now time.Time) error {
	return errNotImplemented
}

// List devuelve los sprints de un proyecto. Firma mínima.
func (r *SprintRepository) List(ctx context.Context, projectID uuid.UUID, statuses []domain.SprintStatus) ([]domain.SprintListItem, error) {
	return nil, errNotImplemented
}

// HasSprints indica si el proyecto tiene sprints. Firma mínima.
func (r *SprintRepository) HasSprints(ctx context.Context, projectID uuid.UUID) (bool, error) {
	return false, errNotImplemented
}

// sprintDetailRow es la fila que lee FindDetail: las columnas del sprint más el identificador y el
// nombre completo del autor, que llegan por el JOIN a users con los alias author_id y
// author_full_name.
type sprintDetailRow struct {
	domain.Sprint  `gorm:"embedded"`
	AuthorID       uuid.UUID
	AuthorFullName string
}

// FindDetail lee el detalle del sprint en dos consultas (research.md sección 19). La primera une
// sprints con users y de users selecciona solo id y full_name, de modo que password_hash nunca entra
// al módulo; si no hay fila devuelve ErrSprintNotFound, nunca gorm.ErrRecordNotFound ni (nil, nil).
// La segunda lee las historias vivas del sprint, uniendo sprint_stories con user_stories para tomar
// los datos vigentes de cada historia, ordenadas por prioridad Alta, Media, Baja, luego por momento
// de compromiso y por último por id; sin historias devuelve una lista vacía, nunca nil. ClosedBy y
// Results quedan en nil: el detalle de un sprint Cerrado se completa en la historia de cierre.
func (r *SprintRepository) FindDetail(ctx context.Context, sprintID uuid.UUID) (*domain.SprintDetail, error) {
	db := r.db.WithContext(ctx)

	var row sprintDetailRow
	err := db.
		Table("sprints").
		Select("sprints.*, users.id AS author_id, users.full_name AS author_full_name").
		Joins("JOIN users ON users.id = sprints.created_by_id").
		Where("sprints.id = ?", sprintID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSprintNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar el detalle del sprint: %w", err)
	}

	stories := []domain.SprintStoryView{}
	err = db.
		Table("sprint_stories").
		Select("user_stories.id AS story_id, user_stories.title, user_stories.priority, "+
			"user_stories.story_points, user_stories.status, sprint_stories.completed_at").
		Joins("JOIN user_stories ON user_stories.id = sprint_stories.story_id").
		Where("sprint_stories.sprint_id = ?", sprintID).
		Order("CASE user_stories.priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, " +
			"sprint_stories.committed_at ASC, user_stories.id ASC").
		Scan(&stories).Error
	if err != nil {
		return nil, fmt.Errorf("leer las historias del sprint: %w", err)
	}

	return &domain.SprintDetail{
		Sprint: row.Sprint,
		CreatedBy: domain.SprintUserRef{
			ID:       row.AuthorID,
			FullName: row.AuthorFullName,
		},
		Stories: stories,
	}, nil
}

// ListForStory devuelve el historial de sprints de una historia. Firma mínima.
func (r *SprintRepository) ListForStory(ctx context.Context, storyID uuid.UUID) ([]domain.StorySprintHistoryItem, error) {
	return nil, errNotImplemented
}
