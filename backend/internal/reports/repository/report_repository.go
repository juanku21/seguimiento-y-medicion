// Package repository implementa la persistencia del módulo de reportes sobre GORM (Principio III).
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	defectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
	pokerdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// ReportRepository lee de la base los datos con los que se arma el reporte.
type ReportRepository struct {
	db *gorm.DB
}

// NewReportRepository crea el repositorio de lecturas del reporte sobre la conexión dada.
func NewReportRepository(db *gorm.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

// sprintColumns son las columnas de un sprint que el reporte necesita.
const sprintColumns = "id, name, status, start_date, end_date, closed_at"

// currentStoryColumns son los datos vigentes de una historia (alias us de user_stories) que el
// reporte muestra cuando no usa la instantánea congelada de un sprint.
const currentStoryColumns = `us.title AS title, us.priority AS priority,
		us.story_points AS story_points, us.status AS status`

// LoadHeader lee el proyecto y el titular de la sesión para el encabezado. Del usuario se selecciona
// solo full_name: nunca el hash de la contraseña. La pertenencia del usuario al proyecto la valida
// el middleware de la ruta antes de llegar acá.
func (r *ReportRepository) LoadHeader(ctx context.Context, projectID, userID uuid.UUID) (domain.ProjectInfo, domain.UserInfo, error) {
	db := r.db.WithContext(ctx)

	// Datos del proyecto.
	var project domain.ProjectInfo
	result := db.Raw("SELECT id, name, status FROM projects WHERE id = @project", sql.Named("project", projectID)).
		Scan(&project)
	if result.Error != nil {
		return domain.ProjectInfo{}, domain.UserInfo{}, fmt.Errorf("leer el proyecto: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ProjectInfo{}, domain.UserInfo{}, fmt.Errorf("leer el proyecto: %w", gorm.ErrRecordNotFound)
	}

	// Nombre del titular de la sesión.
	user := domain.UserInfo{ID: userID}
	result = db.Raw("SELECT full_name FROM users WHERE id = @user", sql.Named("user", userID)).Scan(&user.FullName)
	if result.Error != nil {
		return domain.ProjectInfo{}, domain.UserInfo{}, fmt.Errorf("leer el usuario: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ProjectInfo{}, domain.UserInfo{}, fmt.Errorf("leer el usuario: %w", gorm.ErrRecordNotFound)
	}
	return project, user, nil
}

// LoadSprint lee un sprint acotado al proyecto: uno de otro proyecto equivale a uno inexistente.
func (r *ReportRepository) LoadSprint(ctx context.Context, projectID, sprintID uuid.UUID) (domain.SprintRef, error) {
	var sprint domain.SprintRef
	result := r.db.WithContext(ctx).
		Raw("SELECT "+sprintColumns+" FROM sprints WHERE id = @sprint AND project_id = @project",
			sql.Named("sprint", sprintID), sql.Named("project", projectID)).
		Scan(&sprint)
	if result.Error != nil {
		return domain.SprintRef{}, fmt.Errorf("leer el sprint: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.SprintRef{}, domain.ErrSprintNotFound
	}
	return sprint, nil
}

// ListSprints lista todos los sprints del proyecto, por fecha de inicio y desempatados por id.
func (r *ReportRepository) ListSprints(ctx context.Context, projectID uuid.UUID) ([]domain.SprintRef, error) {
	sprints := []domain.SprintRef{}
	if err := r.db.WithContext(ctx).
		Raw("SELECT "+sprintColumns+" FROM sprints WHERE project_id = @project ORDER BY start_date ASC, id ASC",
			sql.Named("project", projectID)).
		Scan(&sprints).Error; err != nil {
		return nil, fmt.Errorf("listar los sprints: %w", err)
	}
	return sprints, nil
}

// LoadSprintStories lee las historias de un sprint ya validado contra el proyecto. En un sprint
// Cerrado devuelve la instantánea congelada y su resultado; en el Activo, los compromisos abiertos
// con los datos vigentes de la historia (research.md sección 4).
func (r *ReportRepository) LoadSprintStories(ctx context.Context, sprint domain.SprintRef) ([]domain.StoryLine, error) {
	db := r.db.WithContext(ctx)
	lines := []domain.StoryLine{}
	var err error
	if sprint.Status == sprintsdomain.SprintStatusClosed {
		err = db.Raw(`SELECT ss.story_id AS story_id, COALESCE(ss.title_at_close, '') AS title,
				ss.priority_at_close AS priority, ss.story_points_at_close AS story_points,
				ss.outcome AS status
			FROM sprint_stories ss WHERE ss.sprint_id = @sprint`,
			sql.Named("sprint", sprint.ID)).Scan(&lines).Error
	} else {
		err = db.Raw(`SELECT ss.story_id AS story_id, `+currentStoryColumns+`
			FROM sprint_stories ss JOIN user_stories us ON us.id = ss.story_id
			WHERE ss.sprint_id = @sprint AND ss.outcome IS NULL`,
			sql.Named("sprint", sprint.ID)).Scan(&lines).Error
	}
	if err != nil {
		return nil, fmt.Errorf("leer las historias del sprint: %w", err)
	}
	return lines, nil
}

// LoadUnplannedStories lee, con sus datos vigentes, las historias del proyecto que no pertenecen a
// ningún sprint Activo o Cerrado (research.md sección 4).
func (r *ReportRepository) LoadUnplannedStories(ctx context.Context, projectID uuid.UUID) ([]domain.StoryLine, error) {
	lines := []domain.StoryLine{}
	if err := r.db.WithContext(ctx).Raw(`SELECT us.id AS story_id, `+currentStoryColumns+`
		FROM user_stories us
		WHERE us.project_id = @project
			AND NOT EXISTS (
				SELECT 1 FROM sprint_stories ss JOIN sprints s ON s.id = ss.sprint_id
				WHERE ss.story_id = us.id AND s.status IN (@active, @closed))`,
		sql.Named("project", projectID),
		sql.Named("active", sprintsdomain.SprintStatusActive),
		sql.Named("closed", sprintsdomain.SprintStatusClosed)).Scan(&lines).Error; err != nil {
		return nil, fmt.Errorf("leer las historias no planificadas: %w", err)
	}
	return lines, nil
}

// sessionRow es una sesión de Planning Poker con la historia a la que pertenece.
type sessionRow struct {
	StoryID           uuid.UUID
	ID                uuid.UUID
	Status            pokerdomain.SessionStatus
	CreatedAt         time.Time
	ClosedAt          *time.Time
	AgreedStoryPoints *int
	Rounds            int
}

// LoadEstimationSessions lee las sesiones de Planning Poker de las historias dadas, con la cantidad
// de rondas de cada una. Acota al proyecto con un JOIN con user_stories: las historias de otro
// proyecto se ignoran aunque sus ids vengan en la lista.
func (r *ReportRepository) LoadEstimationSessions(ctx context.Context, projectID uuid.UUID, storyIDs []uuid.UUID) (map[uuid.UUID][]domain.SessionRecord, error) {
	sessions := make(map[uuid.UUID][]domain.SessionRecord)
	if len(storyIDs) == 0 {
		return sessions, nil
	}

	var rows []sessionRow
	if err := r.db.WithContext(ctx).Raw(`SELECT es.story_id AS story_id, es.id AS id, es.status AS status,
			es.created_at AS created_at, es.closed_at AS closed_at,
			es.agreed_story_points AS agreed_story_points,
			(SELECT COUNT(*) FROM estimation_rounds er WHERE er.session_id = es.id) AS rounds
		FROM estimation_sessions es JOIN user_stories us ON us.id = es.story_id
		WHERE es.story_id IN @stories AND us.project_id = @project
		ORDER BY es.story_id, es.created_at, es.id`,
		sql.Named("stories", storyIDs), sql.Named("project", projectID)).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("leer las sesiones de estimación: %w", err)
	}

	// Agrupación de las sesiones por historia.
	for _, row := range rows {
		sessions[row.StoryID] = append(sessions[row.StoryID], domain.SessionRecord{
			ID:                row.ID,
			Status:            row.Status,
			CreatedAt:         row.CreatedAt,
			ClosedAt:          row.ClosedAt,
			AgreedStoryPoints: row.AgreedStoryPoints,
			Rounds:            row.Rounds,
		})
	}
	return sessions, nil
}

// LoadEffort lee el esfuerzo agregado por sprint, historia y usuario de los sprints dados.
// Acota al proyecto con JOIN con sprints y user_stories: el esfuerzo de otro proyecto se ignora
// aunque sus ids vengan en la lista.
func (r *ReportRepository) LoadEffort(ctx context.Context, projectID uuid.UUID, sprintIDs []uuid.UUID) ([]domain.EffortRow, error) {
	rows := []domain.EffortRow{}
	if len(sprintIDs) == 0 {
		return rows, nil
	}
	if err := r.db.WithContext(ctx).Raw(`SELECT e.sprint_id AS sprint_id, e.story_id AS story_id,
			us.title AS current_title, e.user_id AS user_id, u.full_name AS full_name,
			SUM(e.hours_hundredths) AS hundredths
		FROM effort_entries e
			JOIN sprints s ON s.id = e.sprint_id
			JOIN user_stories us ON us.id = e.story_id
			JOIN users u ON u.id = e.user_id
		WHERE e.sprint_id IN @sprints AND s.project_id = @project AND us.project_id = @project
		GROUP BY e.sprint_id, e.story_id, us.title, e.user_id, u.full_name`,
		sql.Named("sprints", sprintIDs), sql.Named("project", projectID)).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("leer el esfuerzo de los sprints: %w", err)
	}
	return rows, nil
}

// defectRow es un defecto con los nombres de sus sprints, tal como sale de la consulta.
type defectRow struct {
	DefectID         uuid.UUID
	Description      string
	Severity         defectsdomain.DefectSeverity
	Status           defectsdomain.DefectStatus
	StoryID          uuid.UUID
	StoryTitle       string
	DetectedSprintID uuid.UUID
	DetectedName     string
	ResolvedSprintID *uuid.UUID
	ResolvedName     *string
}

// LoadDefects lee los defectos no descartados del proyecto, o los de un sprint si se indica
// (research.md sección 7). Siempre filtra por proyecto. Con sprint, el título de la historia es el de
// la instantánea del sprint si la historia está en ella y, si no, el vigente.
func (r *ReportRepository) LoadDefects(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID) ([]domain.DefectLine, error) {
	const columns = `SELECT d.id AS defect_id, d.description AS description, d.severity AS severity,
			d.status AS status, d.story_id AS story_id,
			ds.id AS detected_sprint_id, ds.name AS detected_name,
			d.resolved_sprint_id AS resolved_sprint_id, rs.name AS resolved_name`
	const joins = `FROM defects d
			JOIN user_stories us ON us.id = d.story_id
			JOIN sprints ds ON ds.id = d.detected_sprint_id
			LEFT JOIN sprints rs ON rs.id = d.resolved_sprint_id`
	const order = `ORDER BY ds.start_date ASC, d.created_at ASC, d.id ASC`

	var rows []defectRow
	var err error
	if sprintID == nil {
		err = r.db.WithContext(ctx).Raw(columns+`, us.title AS story_title `+joins+`
			WHERE us.project_id = @project AND d.status <> @discarded `+order,
			sql.Named("project", projectID), sql.Named("discarded", defectsdomain.DefectStatusDiscarded)).
			Scan(&rows).Error
	} else {
		err = r.db.WithContext(ctx).Raw(columns+`, COALESCE(ss.title_at_close, us.title) AS story_title `+joins+`
			LEFT JOIN sprint_stories ss ON ss.sprint_id = @sprint AND ss.story_id = d.story_id
			WHERE us.project_id = @project AND d.status <> @discarded
				AND (d.detected_sprint_id = @sprint OR d.resolved_sprint_id = @sprint) `+order,
			sql.Named("project", projectID), sql.Named("sprint", *sprintID),
			sql.Named("discarded", defectsdomain.DefectStatusDiscarded)).Scan(&rows).Error
	}
	if err != nil {
		return nil, fmt.Errorf("leer los defectos: %w", err)
	}

	// Conversión de las filas a las líneas del reporte.
	defects := make([]domain.DefectLine, 0, len(rows))
	for _, row := range rows {
		line := domain.DefectLine{
			DefectID:       row.DefectID,
			Description:    row.Description,
			Severity:       row.Severity,
			Status:         row.Status,
			Story:          domain.StoryRef{ID: row.StoryID, Title: row.StoryTitle},
			DetectedSprint: domain.SprintName{ID: row.DetectedSprintID, Name: row.DetectedName},
		}
		if row.ResolvedSprintID != nil && row.ResolvedName != nil {
			line.ResolvedSprint = &domain.SprintName{ID: *row.ResolvedSprintID, Name: *row.ResolvedName}
		}
		defects = append(defects, line)
	}
	return defects, nil
}
