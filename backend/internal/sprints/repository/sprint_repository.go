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

// SprintRepository implementa domain.SprintRepository con GORM sobre las tablas sprints y
// sprint_stories; todas las operaciones de la interfaz están implementadas.
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
// Después rechaza con checkOverlap la superposición con cualquier sprint del proyecto (FR-005). Si
// no la hay, inserta el sprint; CreatedAt y UpdatedAt los completa GORM.
func (r *SprintRepository) Create(ctx context.Context, sprint *domain.Sprint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockWritableProject(tx, sprint.ProjectID); err != nil {
			return err
		}
		if err := checkOverlap(tx, sprint.ProjectID, uuid.Nil, sprint.StartDate, sprint.EndDate); err != nil {
			return err
		}

		if err := tx.Create(sprint).Error; err != nil {
			return fmt.Errorf("insertar sprint: %w", err)
		}
		return nil
	})
}

// Update modifica un sprint planificado en una transacción (FR-012 a FR-016). Bloquea el proyecto y
// luego el sprint con lockProjectAndSprint, que rechaza el proyecto Finalizado con ErrSprintsReadOnly
// (FR-059), y con el estado releído rechaza el sprint Cerrado con ErrSprintClosed (RN-18) y cualquier
// otro que no esté planned con ErrSprintNotPlanned (FR-016). Después rechaza con checkOverlap la
// superposición, igual que Create pero excluyendo al propio sprint (FR-013). Si no la hay, escribe
// solo name, goal, start_date y end_date; updated_at lo completa GORM. El estado, las historias y la
// instantánea no se tocan (FR-015).
func (r *SprintRepository) Update(ctx context.Context, sprintID uuid.UUID, input domain.SprintInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sprint, err := lockProjectAndSprint(tx, sprintID)
		if err != nil {
			return err
		}
		if sprint.Status == domain.SprintStatusClosed {
			return domain.ErrSprintClosed
		}
		if sprint.Status != domain.SprintStatusPlanned {
			return domain.ErrSprintNotPlanned
		}

		if err := checkOverlap(tx, sprint.ProjectID, sprintID, input.StartDate, input.EndDate); err != nil {
			return err
		}

		err = tx.
			Model(&domain.Sprint{}).
			Where("id = ?", sprintID).
			Updates(map[string]any{
				"name":       input.Name,
				"goal":       input.Goal,
				"start_date": input.StartDate,
				"end_date":   input.EndDate,
			}).Error
		if err != nil {
			return fmt.Errorf("modificar el sprint: %w", err)
		}
		return nil
	})
}

// Delete elimina un sprint planificado en una transacción (research.md sección 16). Bloquea el
// proyecto y luego el sprint con lockProjectAndSprint, que rechaza el proyecto Finalizado con
// ErrSprintsReadOnly (FR-059), y con el estado releído rechaza el sprint que no está planned con
// ErrSprintAlreadyStarted (RN-20). Si no, deja en pending las historias comprometidas, aunque ya lo
// estén, para que la regla sea explícita; borra sus filas de sprint_stories y por último el sprint.
// status se escribe con UpdateColumn, igual que RemoveStories, para no tocar updated_at, y
// ever_committed no se toca y sigue en true (US7-7). El esfuerzo registrado lo verifica el servicio
// con el puerto SprintEffort antes de llamar a esta operación.
func (r *SprintRepository) Delete(ctx context.Context, sprintID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sprint, err := lockProjectAndSprint(tx, sprintID)
		if err != nil {
			return err
		}
		if sprint.Status != domain.SprintStatusPlanned {
			return domain.ErrSprintAlreadyStarted
		}

		err = tx.
			Model(&backlogdomain.UserStory{}).
			Where("id IN (?)", tx.Model(&domain.SprintStory{}).Select("story_id").Where("sprint_id = ?", sprintID)).
			UpdateColumn("status", backlogdomain.StoryStatusPending).Error
		if err != nil {
			return fmt.Errorf("devolver las historias del sprint a pendiente: %w", err)
		}
		err = tx.
			Where("sprint_id = ?", sprintID).
			Delete(&domain.SprintStory{}).Error
		if err != nil {
			return fmt.Errorf("borrar los compromisos del sprint: %w", err)
		}
		err = tx.
			Where("id = ?", sprintID).
			Delete(&domain.Sprint{}).Error
		if err != nil {
			return fmt.Errorf("borrar el sprint: %w", err)
		}
		return nil
	})
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
// proyecto Finalizado con ErrSprintsReadOnly (FR-059). La comparten Create, Update, Delete y Start.
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

// checkOverlap busca, dentro de la transacción recibida y entre todos los sprints del proyecto
// cualquiera sea su estado, uno cuyo período se superponga con [startDate, endDate), con la traducción
// de PeriodsOverlap a desigualdades estrictas. excludeID es el sprint que se está modificando, que no
// cuenta como conflicto; uuid.Nil indica que no se excluye ninguno (alta). Si hay superposición,
// devuelve SprintOverlapError con el de inicio más temprano (y luego menor id), para que el mensaje
// sea determinista (FR-005, FR-013); se lee con Find y LIMIT 1 para que la ausencia de conflicto, que
// es el caso normal, no pase por gorm.ErrRecordNotFound. La comparten Create y Update.
func checkOverlap(tx *gorm.DB, projectID, excludeID uuid.UUID, startDate, endDate time.Time) error {
	query := tx.
		Model(&domain.Sprint{}).
		Select("id, name").
		Where("project_id = ?", projectID)
	if excludeID != uuid.Nil {
		query = query.Where("id <> ?", excludeID)
	}
	var conflicts []domain.ConflictingSprint
	err := query.
		Where("start_date < ? AND ? < end_date", endDate, startDate).
		Order("start_date ASC, id ASC").
		Limit(1).
		Find(&conflicts).Error
	if err != nil {
		return fmt.Errorf("buscar sprints superpuestos: %w", err)
	}
	if len(conflicts) > 0 {
		return &domain.SprintOverlapError{Sprint: conflicts[0]}
	}
	return nil
}

// lockProjectAndSprint toma, dentro de la transacción recibida, los bloqueos de Start, Update y
// Delete: lee el proyecto del sprint sin bloquear (si no existe devuelve ErrSprintNotFound), bloquea
// el proyecto con lockWritableProject y por último bloquea la fila del sprint y relee su estado ya
// serializado.
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

// sprintResultsSelect es el único fragmento de consulta que deriva los totales de la instantánea de
// cierre a partir de las filas congeladas de sprint_stories (research.md sección 11; data-model.md
// sección 7): historias planificadas y completadas, y Story Points planificados y completados, con
// COALESCE para que un sprint vacío dé 0 y no NULL (FR-045). Los alias coinciden con los campos de
// SprintResults. Lo usan FindDetail para un sprint Cerrado y List como subconsulta agregada por
// sprint; no hay una segunda implementación en Go.
const sprintResultsSelect = "COUNT(*) AS planned_story_count, " +
	"COUNT(*) FILTER (WHERE outcome = 'completed') AS completed_story_count, " +
	"COALESCE(SUM(story_points_at_close), 0) AS planned_story_points, " +
	"COALESCE(SUM(story_points_at_close) FILTER (WHERE outcome = 'completed'), 0) AS completed_story_points"

// Close cierra el sprint en una sola transacción (research.md sección 14): si algo falla, el sprint
// sigue Activo y ninguna historia se movió. Bloquea el sprint con lockOpenSprint, que lee en la misma
// consulta el estado del proyecto y rechaza el proyecto Finalizado con ErrSprintsReadOnly (FR-059) y
// el sprint Cerrado con ErrSprintClosed (RN-18); con el estado releído valida la transición hacia
// closed y, si no corresponde, devuelve ErrSprintNotActive (FR-047). Después bloquea las historias
// comprometidas, ordenadas por id para no interbloquearse con las demás escrituras, y congela cada
// fila de sprint_stories con el título, la prioridad, los Story Points y el resultado según Outcome
// (FR-042, FR-043); las no completadas vuelven a pending (FR-041) con UpdateColumn, igual que
// RemoveStories, para no tocar updated_at. Por último copia el factor del proyecto y fija
// closed_at = now, closed_by_id y status = closed (FR-040, FR-044). No toca el estado del proyecto
// (RN-07) ni compara now con la fecha de fin, de modo que el cierre anticipado o tardío es igual.
func (r *SprintRepository) Close(ctx context.Context, sprintID, closedBy uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sprint, err := lockOpenSprint(tx, sprintID)
		if err != nil {
			return err
		}
		if err := sprint.Status.ValidateTransition(domain.SprintStatusClosed); err != nil {
			return domain.ErrSprintNotActive
		}

		var stories []backlogdomain.UserStory
		err = tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id, title, priority, story_points, status").
			Where("id IN (?)", tx.Model(&domain.SprintStory{}).Select("story_id").Where("sprint_id = ?", sprintID)).
			Order("id ASC").
			Find(&stories).Error
		if err != nil {
			return fmt.Errorf("bloquear las historias comprometidas: %w", err)
		}

		// Cada fila del sprint se congela con los datos vigentes de su historia; las no completadas
		// se juntan para devolverlas al backlog en una sola escritura.
		var notCompleted []uuid.UUID
		for _, story := range stories {
			outcome := domain.Outcome(story.Status)
			err = tx.
				Model(&domain.SprintStory{}).
				Where("sprint_id = ? AND story_id = ?", sprintID, story.ID).
				Updates(map[string]any{
					"title_at_close":        story.Title,
					"priority_at_close":     story.Priority,
					"story_points_at_close": story.StoryPoints,
					"outcome":               outcome,
				}).Error
			if err != nil {
				return fmt.Errorf("congelar la historia del sprint: %w", err)
			}
			if outcome == domain.StoryOutcomeNotCompleted {
				notCompleted = append(notCompleted, story.ID)
			}
		}
		if len(notCompleted) > 0 {
			err = tx.
				Model(&backlogdomain.UserStory{}).
				Where("id IN ?", notCompleted).
				UpdateColumn("status", backlogdomain.StoryStatusPending).Error
			if err != nil {
				return fmt.Errorf("devolver las historias no completadas a pendiente: %w", err)
			}
		}

		// El factor vigente del proyecto se copia en el sprint junto con los datos de cierre.
		var project projectsdomain.Project
		err = tx.
			Select("hours_per_story_point_hundredths").
			Where("id = ?", sprint.ProjectID).
			Take(&project).Error
		if err != nil {
			return fmt.Errorf("leer el factor del proyecto: %w", err)
		}
		err = tx.
			Model(&domain.Sprint{}).
			Where("id = ?", sprintID).
			Updates(map[string]any{
				"status":                           domain.SprintStatusClosed,
				"closed_at":                        now,
				"closed_by_id":                     closedBy,
				"hours_per_story_point_hundredths": project.HoursPerStoryPointHundredths,
			}).Error
		if err != nil {
			return fmt.Errorf("cerrar el sprint: %w", err)
		}
		return nil
	})
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

// lockOpenSprint bloquea, dentro de la transacción recibida, solo la fila del sprint (FOR UPDATE OF
// sprints) y, en la misma consulta, lee su proyecto, su estado y el estado del proyecto; si no existe
// devuelve ErrSprintNotFound. Con los datos releídos y en el mismo orden que el servicio, rechaza el
// proyecto Finalizado con ErrSprintsReadOnly (FR-059) y el sprint Cerrado con ErrSprintClosed
// (RN-09, RN-18). La comparten lockSprintAndStories y Close.
func lockOpenSprint(tx *gorm.DB, sprintID uuid.UUID) (*lockedSprintRow, error) {
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
	return &sprint, nil
}

// lockSprintAndStories toma, dentro de la transacción recibida, los bloqueos comunes de
// AssignStories y RemoveStories y verifica con los datos releídos que el sprint admita cambios
// (research.md sección 8, paso 1). Primero bloquea el sprint con lockOpenSprint, que rechaza el
// proyecto Finalizado y el sprint Cerrado. Después bloquea con FOR UPDATE las historias pedidas de
// ese mismo proyecto, ordenadas por id para que dos lotes que comparten historias no se
// interbloqueen. Las historias de otro proyecto quedan fuera del filtro, así que si no se encuentran
// todas, porque alguna no existe o es ajena, devuelve ErrStoryNotFound.
func lockSprintAndStories(tx *gorm.DB, sprintID uuid.UUID, storyIDs []uuid.UUID) ([]backlogdomain.UserStory, error) {
	sprint, err := lockOpenSprint(tx, sprintID)
	if err != nil {
		return nil, err
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

// lockedStoryRow es la fila que lee ChangeStoryProgress al bloquear la historia: su estado y el estado
// de su proyecto, que llega por el JOIN a projects con el alias project_status.
type lockedStoryRow struct {
	Status        backlogdomain.StoryStatus
	ProjectStatus projectsdomain.ProjectStatus
}

// storyCommitmentRow es la fila de un compromiso de la historia junto con su sprint y el estado de ese
// sprint, que llega por el JOIN a sprints con el alias sprint_status.
type storyCommitmentRow struct {
	ID           uuid.UUID
	SprintID     uuid.UUID
	SprintStatus domain.SprintStatus
}

// ChangeStoryProgress aplica el avance de una historia dentro del sprint Activo en una transacción
// (research.md sección 13). Toma los bloqueos en el mismo orden que Close, primero el sprint y después
// la historia, para no interbloquearse con un cierre simultáneo: ubica sin bloquear el sprint de su
// compromiso abierto y toma sobre esa fila un bloqueo compartido (FOR SHARE), que deja avanzar a
// otros integrantes y solo se serializa contra el cierre. Después bloquea la historia (FOR UPDATE OF
// user_stories) y, en la misma consulta, lee el estado de su proyecto; si no existe devuelve
// ErrStoryNotFound y si el proyecto está Finalizado, ErrSprintsReadOnly (FR-059). Con los bloqueos
// tomados relee el compromiso abierto, porque un cierre o una reasignación pudieron confirmarse antes.
// El compromiso y el sprint se verifican antes que la transición: sin compromiso abierto, la historia
// Completada cuyo último compromiso es de un sprint Cerrado se rechaza con ErrSprintClosed (RN-18) y
// cualquier otra con ErrProgressOutsideActiveSprint (FR-037); con compromiso en un sprint distinto del
// bloqueado o que no está Activo, también ErrProgressOutsideActiveSprint (FR-036). Recién entonces
// valida la transición con ValidateStoryProgress (FR-038). Si se admite, escribe user_stories.status
// con UpdateColumn, igual que RemoveStories, para no tocar updated_at, y sprint_stories.completed_at
// con now al completar y NULL al volver a in_progress (FR-034, FR-035).
func (r *SprintRepository) ChangeStoryProgress(ctx context.Context, storyID uuid.UUID, to backlogdomain.StoryStatus, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		located, err := openCommitmentForProgress(tx, storyID)
		if err != nil {
			return err
		}
		if located != nil {
			if err := shareLockSprint(tx, located.SprintID); err != nil {
				return err
			}
		}

		var story lockedStoryRow
		err = tx.
			Table("user_stories").
			Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "user_stories"}}).
			Select("user_stories.status, projects.status AS project_status").
			Joins("JOIN projects ON projects.id = user_stories.project_id").
			Where("user_stories.id = ?", storyID).
			Take(&story).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return backlogdomain.ErrStoryNotFound
		}
		if err != nil {
			return fmt.Errorf("bloquear la historia: %w", err)
		}
		if story.ProjectStatus == projectsdomain.ProjectStatusFinished {
			return domain.ErrSprintsReadOnly
		}

		commitment, err := openCommitmentForProgress(tx, storyID)
		if err != nil {
			return err
		}
		if commitment == nil {
			return progressWithoutCommitmentError(tx, storyID, story.Status)
		}
		lockedOther := located == nil || located.SprintID != commitment.SprintID
		if lockedOther || commitment.SprintStatus != domain.SprintStatusActive {
			return domain.ErrProgressOutsideActiveSprint
		}
		if err := domain.ValidateStoryProgress(story.Status, to); err != nil {
			return err
		}

		err = tx.
			Model(&backlogdomain.UserStory{}).
			Where("id = ?", storyID).
			UpdateColumn("status", to).Error
		if err != nil {
			return fmt.Errorf("cambiar el estado de la historia: %w", err)
		}

		// Al completar se registra el instante; al volver a En progreso la fecha se borra.
		var completedAt *time.Time
		if to == backlogdomain.StoryStatusCompleted {
			completedAt = &now
		}
		err = tx.
			Model(&domain.SprintStory{}).
			Where("id = ?", commitment.ID).
			Update("completed_at", completedAt).Error
		if err != nil {
			return fmt.Errorf("registrar la fecha de completado: %w", err)
		}
		return nil
	})
}

// openCommitmentForProgress devuelve, sin tomar bloqueos, el compromiso abierto (outcome IS NULL) de la
// historia con su sprint y el estado de ese sprint, o nil si no tiene ninguno; se lee con Find y
// LIMIT 1 para que la ausencia no pase por gorm.ErrRecordNotFound.
func openCommitmentForProgress(tx *gorm.DB, storyID uuid.UUID) (*storyCommitmentRow, error) {
	var rows []storyCommitmentRow
	err := tx.
		Table("sprint_stories").
		Select("sprint_stories.id, sprint_stories.sprint_id, sprints.status AS sprint_status").
		Joins("JOIN sprints ON sprints.id = sprint_stories.sprint_id").
		Where("sprint_stories.story_id = ? AND sprint_stories.outcome IS NULL", storyID).
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("leer el compromiso abierto de la historia: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// shareLockSprint toma, dentro de la transacción recibida, un bloqueo compartido (FOR SHARE) sobre la
// fila del sprint, que espera a un cierre en curso y le impide empezar hasta confirmar la transacción.
// Se lee con Find para que un sprint inexistente no sea un error: la relectura del compromiso decide.
func shareLockSprint(tx *gorm.DB, sprintID uuid.UUID) error {
	var ids []uuid.UUID
	err := tx.
		Table("sprints").
		Clauses(clause.Locking{Strength: "SHARE"}).
		Where("id = ?", sprintID).
		Pluck("id", &ids).Error
	if err != nil {
		return fmt.Errorf("bloquear el sprint del compromiso abierto: %w", err)
	}
	return nil
}

// progressWithoutCommitmentError decide el rechazo del avance de una historia sin compromiso abierto:
// si está Completada y su último compromiso, el de committed_at más reciente, es de un sprint Cerrado,
// devuelve ErrSprintClosed (RN-18); en cualquier otro caso, ErrProgressOutsideActiveSprint (FR-037).
func progressWithoutCommitmentError(tx *gorm.DB, storyID uuid.UUID, status backlogdomain.StoryStatus) error {
	if status != backlogdomain.StoryStatusCompleted {
		return domain.ErrProgressOutsideActiveSprint
	}
	var rows []storyCommitmentRow
	err := tx.
		Table("sprint_stories").
		Select("sprint_stories.id, sprints.status AS sprint_status").
		Joins("JOIN sprints ON sprints.id = sprint_stories.sprint_id").
		Where("sprint_stories.story_id = ?", storyID).
		Order("sprint_stories.committed_at DESC, sprint_stories.id DESC").
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return fmt.Errorf("leer el último compromiso de la historia: %w", err)
	}
	if len(rows) > 0 && rows[0].SprintStatus == domain.SprintStatusClosed {
		return domain.ErrSprintClosed
	}
	return domain.ErrProgressOutsideActiveSprint
}

// FindOpenCommitment devuelve el compromiso abierto (outcome IS NULL) de la historia, o nil sin error
// si no tiene ninguno (data-model.md sección 9). Es una lectura simple, sin bloqueos, que arma la
// respuesta del avance después de ChangeStoryProgress; se lee con Find y LIMIT 1 para que la ausencia
// no pase por gorm.ErrRecordNotFound. El índice único parcial garantiza a lo sumo una fila.
func (r *SprintRepository) FindOpenCommitment(ctx context.Context, storyID uuid.UUID) (*domain.SprintStory, error) {
	var rows []domain.SprintStory
	err := r.db.WithContext(ctx).
		Where("story_id = ? AND outcome IS NULL", storyID).
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("leer el compromiso abierto de la historia: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// sprintListRow es la fila que lee List: las columnas del sprint más los totales que calcula
// sprintResultsSelect sobre sus filas de sprint_stories, con los mismos alias que SprintResults.
type sprintListRow struct {
	domain.Sprint        `gorm:"embedded"`
	PlannedStoryCount    int
	CompletedStoryCount  int
	PlannedStoryPoints   int
	CompletedStoryPoints int
}

// List devuelve los sprints del proyecto en una sola consulta (research.md sección 10): une cada
// sprint con una subconsulta LATERAL que agrega sus filas de sprint_stories con sprintResultsSelect,
// de modo que no hay N+1 y un sprint sin historias da todos los totales en 0. Si se pide algún
// estado, filtra con IN; sin filtro devuelve todos. El orden es start_date DESC y luego id ASC
// (FR-052). StoryCount es la cantidad de filas del sprint: los compromisos vigentes en uno abierto y
// las planificadas al cierre en uno Cerrado. Results solo se completa en un sprint Cerrado, con los
// Story Points comprometidos al inicio; en otro caso queda en nil. Sin sprints devuelve una lista
// vacía, nunca nil.
func (r *SprintRepository) List(ctx context.Context, projectID uuid.UUID, statuses []domain.SprintStatus) ([]domain.SprintListItem, error) {
	query := r.db.WithContext(ctx).
		Table("sprints").
		Select("sprints.*, results.planned_story_count, results.completed_story_count, "+
			"results.planned_story_points, results.completed_story_points").
		Joins("CROSS JOIN LATERAL (SELECT "+sprintResultsSelect+
			" FROM sprint_stories WHERE sprint_stories.sprint_id = sprints.id) AS results").
		Where("sprints.project_id = ?", projectID)
	if len(statuses) > 0 {
		query = query.Where("sprints.status IN ?", statuses)
	}
	var rows []sprintListRow
	err := query.
		Order("sprints.start_date DESC, sprints.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listar los sprints del proyecto: %w", err)
	}

	// Cada fila se traduce al elemento del listado; los totales solo se exponen si está Cerrado.
	items := make([]domain.SprintListItem, 0, len(rows))
	for _, row := range rows {
		item := domain.SprintListItem{
			Sprint:     row.Sprint,
			StoryCount: row.PlannedStoryCount,
		}
		if row.Status == domain.SprintStatusClosed {
			results := domain.SprintResults{
				PlannedStoryPoints:   row.PlannedStoryPoints,
				CompletedStoryPoints: row.CompletedStoryPoints,
				PlannedStoryCount:    row.PlannedStoryCount,
				CompletedStoryCount:  row.CompletedStoryCount,
			}
			if row.CommittedStoryPoints != nil {
				results.CommittedStoryPoints = *row.CommittedStoryPoints
			}
			item.Results = &results
		}
		items = append(items, item)
	}
	return items, nil
}

// HasSprints indica si el proyecto tiene al menos un sprint, cualquiera sea su estado, con un
// SELECT 1 ... LIMIT 1; se lee con Find para que la ausencia no pase por gorm.ErrRecordNotFound. El
// servicio la usa solo cuando el listado filtrado viene vacío, para distinguir no_sprints de
// no_matches (FR-054).
func (r *SprintRepository) HasSprints(ctx context.Context, projectID uuid.UUID) (bool, error) {
	var found []int
	err := r.db.WithContext(ctx).
		Table("sprints").
		Select("1").
		Where("project_id = ?", projectID).
		Limit(1).
		Find(&found).Error
	if err != nil {
		return false, fmt.Errorf("verificar si el proyecto tiene sprints: %w", err)
	}
	return len(found) > 0, nil
}

// sprintDetailRow es la fila que lee FindDetail: las columnas del sprint más el identificador y el
// nombre completo del autor, que llegan por el JOIN a users con los alias author_id y
// author_full_name, y los de quien lo cerró, que llegan por el LEFT JOIN a users con los alias
// closer_id y closer_full_name y valen NULL mientras el sprint no está Cerrado.
type sprintDetailRow struct {
	domain.Sprint  `gorm:"embedded"`
	AuthorID       uuid.UUID
	AuthorFullName string
	CloserID       *uuid.UUID
	CloserFullName *string
}

// liveStoriesSelect y liveStoriesOrder leen las historias de un sprint abierto con los datos vigentes
// de user_stories, ordenadas por prioridad Alta, Media, Baja, luego por momento de compromiso y por
// último por id.
const (
	liveStoriesSelect = "user_stories.id AS story_id, user_stories.title, user_stories.priority, " +
		"user_stories.story_points, user_stories.status, sprint_stories.completed_at"
	liveStoriesOrder = "CASE user_stories.priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, " +
		"sprint_stories.committed_at ASC, user_stories.id ASC"
)

// frozenStoriesSelect y frozenStoriesOrder leen las historias de un sprint Cerrado con los datos
// congelados de sprint_stories (US6-8), con el mismo orden que un sprint abierto pero sobre la
// prioridad al cierre; el estado es el vigente de la historia en el backlog, como fija el contrato.
const (
	frozenStoriesSelect = "sprint_stories.story_id, sprint_stories.title_at_close AS title, " +
		"sprint_stories.priority_at_close AS priority, sprint_stories.story_points_at_close AS story_points, " +
		"user_stories.status, sprint_stories.completed_at, sprint_stories.outcome"
	frozenStoriesOrder = "CASE sprint_stories.priority_at_close WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, " +
		"sprint_stories.committed_at ASC, sprint_stories.story_id ASC"
)

// FindDetail lee el detalle del sprint (research.md secciones 11 y 19). La primera consulta une
// sprints con users para el autor y, con LEFT JOIN, para quien lo cerró, seleccionando de users solo
// id y full_name, de modo que password_hash nunca entra al módulo; si no hay fila devuelve
// ErrSprintNotFound, nunca gorm.ErrRecordNotFound ni (nil, nil). La segunda lee las historias del
// sprint: en uno abierto, los datos vigentes de user_stories; en uno Cerrado, los congelados de
// sprint_stories; sin historias devuelve una lista vacía, nunca nil. Solo en un sprint Cerrado
// completa ClosedBy y Results, estos con sprintResultsSelect sobre las filas congeladas y los Story
// Points comprometidos al inicio; en otro caso ambos quedan en nil.
func (r *SprintRepository) FindDetail(ctx context.Context, sprintID uuid.UUID) (*domain.SprintDetail, error) {
	db := r.db.WithContext(ctx)

	var row sprintDetailRow
	err := db.
		Table("sprints").
		Select("sprints.*, users.id AS author_id, users.full_name AS author_full_name, "+
			"closers.id AS closer_id, closers.full_name AS closer_full_name").
		Joins("JOIN users ON users.id = sprints.created_by_id").
		Joins("LEFT JOIN users AS closers ON closers.id = sprints.closed_by_id").
		Where("sprints.id = ?", sprintID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSprintNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar el detalle del sprint: %w", err)
	}

	closed := row.Status == domain.SprintStatusClosed
	selectStories, orderStories := liveStoriesSelect, liveStoriesOrder
	if closed {
		selectStories, orderStories = frozenStoriesSelect, frozenStoriesOrder
	}
	stories := []domain.SprintStoryView{}
	err = db.
		Table("sprint_stories").
		Select(selectStories).
		Joins("JOIN user_stories ON user_stories.id = sprint_stories.story_id").
		Where("sprint_stories.sprint_id = ?", sprintID).
		Order(orderStories).
		Scan(&stories).Error
	if err != nil {
		return nil, fmt.Errorf("leer las historias del sprint: %w", err)
	}

	detail := &domain.SprintDetail{
		Sprint: row.Sprint,
		CreatedBy: domain.SprintUserRef{
			ID:       row.AuthorID,
			FullName: row.AuthorFullName,
		},
		Stories: stories,
	}
	if !closed {
		return detail, nil
	}

	// En un sprint Cerrado se agregan quién lo cerró y los totales derivados de la instantánea.
	if row.CloserID != nil && row.CloserFullName != nil {
		detail.ClosedBy = &domain.SprintUserRef{ID: *row.CloserID, FullName: *row.CloserFullName}
	}
	var results domain.SprintResults
	err = db.
		Model(&domain.SprintStory{}).
		Select(sprintResultsSelect).
		Where("sprint_id = ?", sprintID).
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("calcular los totales de la instantánea: %w", err)
	}
	if row.CommittedStoryPoints != nil {
		results.CommittedStoryPoints = *row.CommittedStoryPoints
	}
	detail.Results = &results
	return detail, nil
}

// ListForStory devuelve el historial de sprints de una historia (FR-055): une sus filas de
// sprint_stories con sprints y las ordena por start_date ASC y luego por el id del sprint. Como al
// quitar una historia de un sprint abierto se borra su fila, solo figuran los sprints donde sigue
// comprometida y los Cerrados donde estuvo al cierre. Outcome es el congelado al cierre, o nil en el
// sprint abierto. Sin sprints devuelve una lista vacía, nunca nil.
func (r *SprintRepository) ListForStory(ctx context.Context, storyID uuid.UUID) ([]domain.StorySprintHistoryItem, error) {
	items := []domain.StorySprintHistoryItem{}
	err := r.db.WithContext(ctx).
		Table("sprint_stories").
		Select("sprints.id AS sprint_id, sprints.name AS sprint_name, sprints.status AS sprint_status, "+
			"sprints.start_date, sprints.end_date, sprint_stories.outcome, sprint_stories.completed_at").
		Joins("JOIN sprints ON sprints.id = sprint_stories.sprint_id").
		Where("sprint_stories.story_id = ?", storyID).
		Order("sprints.start_date ASC, sprints.id ASC").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("leer el historial de sprints de la historia: %w", err)
	}
	return items, nil
}
