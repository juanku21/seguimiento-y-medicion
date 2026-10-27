// Package repository implementa la persistencia del módulo de defectos con GORM (Principio III).
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
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// DefectRepository implementa domain.DefectRepository sobre la tabla defects. Firma mínima: las
// operaciones las implementan las tareas de cada historia.
type DefectRepository struct {
	db *gorm.DB
}

// NewDefectRepository crea el repositorio de defectos sobre la conexión recibida.
func NewDefectRepository(db *gorm.DB) *DefectRepository {
	return &DefectRepository{db: db}
}

// FindAccessible resuelve el acceso del usuario a un defecto.
func (r *DefectRepository) FindAccessible(_ context.Context, _, _ uuid.UUID) (*domain.DefectAccess, error) {
	return nil, errors.New("not implemented")
}

// Create da de alta un defecto en el proyecto, en una transacción y con el orden de bloqueos de
// research.md sección 10: proyecto (SHARE), existencia de un sprint de detección posible, sprint de
// detección (KEY SHARE), marca de la historia y, por último, la inserción. El defecto nace abierto,
// sin sprint de resolución y con el actor como autor, titular de la última escritura y del estado.
func (r *DefectRepository) Create(ctx context.Context, projectID, actorID uuid.UUID, input domain.DefectInput, now time.Time) (*domain.DefectView, error) {
	var defectID uuid.UUID
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Paso 1: el proyecto se lee con SHARE para que no pase a finalizado durante el alta.
		var project projectsdomain.Project
		err := tx.
			Clauses(clause.Locking{Strength: "SHARE"}).
			Select("status").
			Where("id = ?", projectID).
			Take(&project).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return projectsdomain.ErrProjectNotFound
		}
		if err != nil {
			return fmt.Errorf("bloquear el proyecto del defecto: %w", err)
		}
		if project.Status == projectsdomain.ProjectStatusFinished {
			return domain.ErrDefectsReadOnly
		}

		// Paso 2: debe existir al menos un sprint activo o cerrado que pueda ser de detección.
		var usable bool
		err = tx.Model(&sprintsdomain.Sprint{}).
			Select("COUNT(*) > 0").
			Where("project_id = ? AND status IN ?", projectID,
				[]sprintsdomain.SprintStatus{sprintsdomain.SprintStatusActive, sprintsdomain.SprintStatusClosed}).
			Scan(&usable).Error
		if err != nil {
			return fmt.Errorf("buscar sprints de detección del proyecto: %w", err)
		}
		if !usable {
			return domain.ErrNoDetectionSprint
		}

		// Paso 3: el sprint de detección debe ser del proyecto; KEY SHARE impide borrarlo en paralelo.
		var sprint sprintsdomain.Sprint
		err = tx.
			Clauses(clause.Locking{Strength: "KEY SHARE"}).
			Select("id", "status").
			Where("id = ? AND project_id = ?", input.DetectedSprintID, projectID).
			Take(&sprint).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sprintsdomain.ErrSprintNotFound
		}
		if err != nil {
			return fmt.Errorf("bloquear el sprint de detección: %w", err)
		}
		if err := domain.EnsureSprintUsableForDetection(sprint.Status); err != nil {
			return err
		}

		// Paso 4: la marca valida la pertenencia de la historia y la bloquea; no toca updated_at.
		res := tx.Model(&backlogdomain.UserStory{}).
			Where("id = ? AND project_id = ?", input.StoryID, projectID).
			UpdateColumn("ever_had_defects", true)
		if res.Error != nil {
			return fmt.Errorf("marcar la historia con defectos: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return backlogdomain.ErrStoryNotFound
		}

		// Paso 5: se inserta el defecto abierto. CreatedAt, UpdatedAt y StatusChangedAt llevan el mismo
		// instante del reloj del servicio, para que el estado de un defecto nuevo date de su creación (FR-022).
		defect := domain.Defect{
			ID:                uuid.New(),
			StoryID:           input.StoryID,
			Description:       input.Description,
			Severity:          input.Severity,
			Status:            domain.DefectStatusOpen,
			DetectedSprintID:  input.DetectedSprintID,
			CreatedByID:       actorID,
			UpdatedByID:       actorID,
			StatusChangedByID: actorID,
			StatusChangedAt:   now,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := tx.Create(&defect).Error; err != nil {
			return fmt.Errorf("insertar defecto: %w", err)
		}
		defectID = defect.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindView(ctx, defectID)
}

// ChangeStatus aplica un cambio de estado.
func (r *DefectRepository) ChangeStatus(_ context.Context, _, _ uuid.UUID, _ domain.StatusChange, _ time.Time) (*domain.DefectView, error) {
	return nil, errors.New("not implemented")
}

// Update aplica una modificación parcial de campos.
func (r *DefectRepository) Update(_ context.Context, _, _ uuid.UUID, _ domain.DefectPatch) (*domain.DefectView, error) {
	return nil, errors.New("not implemented")
}

// defectViewRow es la fila plana de la consulta de detalle: las columnas de defects más los alias
// de las relaciones. Nunca incluye password_hash.
type defectViewRow struct {
	domain.Defect
	StoryTitle           string
	DetectedSprintName   string
	DetectedSprintStatus sprintsdomain.SprintStatus
	DetectedSprintStart  time.Time
	ResolvedSprintName   *string
	ResolvedSprintStatus *sprintsdomain.SprintStatus
	ResolvedSprintStart  *time.Time
	CreatedByName        string
	UpdatedByName        string
	StatusChangedByName  string
}

// FindView devuelve el detalle de un defecto con historia, sprints y usuarios resueltos por JOIN;
// si no existe devuelve ErrDefectNotFound.
func (r *DefectRepository) FindView(ctx context.Context, defectID uuid.UUID) (*domain.DefectView, error) {
	// Una sola consulta con los JOIN; el sprint de resolución es LEFT JOIN porque puede ser nulo.
	var row defectViewRow
	res := r.db.WithContext(ctx).
		Table("defects AS d").
		Select(`d.*,
			us.title AS story_title,
			ds.name AS detected_sprint_name, ds.status AS detected_sprint_status, ds.start_date AS detected_sprint_start,
			rs.name AS resolved_sprint_name, rs.status AS resolved_sprint_status, rs.start_date AS resolved_sprint_start,
			cu.full_name AS created_by_name, uu.full_name AS updated_by_name, su.full_name AS status_changed_by_name`).
		Joins("JOIN user_stories us ON us.id = d.story_id").
		Joins("JOIN sprints ds ON ds.id = d.detected_sprint_id").
		Joins("LEFT JOIN sprints rs ON rs.id = d.resolved_sprint_id").
		Joins("JOIN users cu ON cu.id = d.created_by_id").
		Joins("JOIN users uu ON uu.id = d.updated_by_id").
		Joins("JOIN users su ON su.id = d.status_changed_by_id").
		Where("d.id = ?", defectID).
		Limit(1).
		Scan(&row)
	if res.Error != nil {
		return nil, fmt.Errorf("leer el defecto: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrDefectNotFound
	}

	// Se arma la vista con las referencias mínimas de cada relación.
	view := &domain.DefectView{
		Defect: row.Defect,
		Story:  domain.StoryRef{ID: row.StoryID, Title: row.StoryTitle},
		DetectedSprint: domain.SprintRef{
			ID:        row.DetectedSprintID,
			Name:      row.DetectedSprintName,
			Status:    row.DetectedSprintStatus,
			StartDate: row.DetectedSprintStart,
		},
		CreatedBy:       domain.UserRef{ID: row.CreatedByID, FullName: row.CreatedByName},
		UpdatedBy:       domain.UserRef{ID: row.UpdatedByID, FullName: row.UpdatedByName},
		StatusChangedBy: domain.UserRef{ID: row.StatusChangedByID, FullName: row.StatusChangedByName},
	}

	// El sprint de resolución solo existe si la fila trajo sus datos.
	if row.ResolvedSprintID != nil && row.ResolvedSprintName != nil &&
		row.ResolvedSprintStatus != nil && row.ResolvedSprintStart != nil {
		view.ResolvedSprint = &domain.SprintRef{
			ID:        *row.ResolvedSprintID,
			Name:      *row.ResolvedSprintName,
			Status:    *row.ResolvedSprintStatus,
			StartDate: *row.ResolvedSprintStart,
		}
	}
	return view, nil
}

// ListForProject lista los defectos de un proyecto según el filtro.
func (r *DefectRepository) ListForProject(_ context.Context, _ uuid.UUID, _ domain.DefectFilter) ([]domain.DefectView, error) {
	return nil, errors.New("not implemented")
}

// ValidateFilter verifica que los identificadores del filtro pertenezcan al proyecto.
func (r *DefectRepository) ValidateFilter(_ context.Context, _ uuid.UUID, _ domain.DefectFilter) error {
	return errors.New("not implemented")
}

// HasProjectDefects indica si el proyecto tiene al menos un defecto.
func (r *DefectRepository) HasProjectDefects(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, errors.New("not implemented")
}
