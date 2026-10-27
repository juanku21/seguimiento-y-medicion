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

// defectAccessRow es la fila plana de la consulta de acceso: el defecto, el proyecto de su historia y
// el estado de ese proyecto.
type defectAccessRow struct {
	DefectID      uuid.UUID
	ProjectID     uuid.UUID
	ProjectStatus projectsdomain.ProjectStatus
}

// FindAccessible resuelve el acceso a un defecto para RequireDefectAccess con una sola consulta
// (research.md sección 3): une defects con su historia, con la membresía vigente del usuario en
// project_members y con projects. Si no hay fila, el defecto no existe o el usuario no es integrante
// vigente del proyecto, y ambos casos se reportan igual con ErrDefectNotFound.
func (r *DefectRepository) FindAccessible(ctx context.Context, defectID, userID uuid.UUID) (*domain.DefectAccess, error) {
	var row defectAccessRow
	err := r.db.WithContext(ctx).
		Table("defects").
		Select(`defects.id AS defect_id, user_stories.project_id AS project_id,
			projects.status AS project_status`).
		Joins("JOIN user_stories ON user_stories.id = defects.story_id").
		Joins(`JOIN project_members ON project_members.project_id = user_stories.project_id
			AND project_members.user_id = ?`, userID).
		Joins("JOIN projects ON projects.id = user_stories.project_id").
		Where("defects.id = ?", defectID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrDefectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolver el acceso al defecto: %w", err)
	}
	return &domain.DefectAccess{
		DefectID:        row.DefectID,
		ProjectID:       row.ProjectID,
		ProjectFinished: row.ProjectStatus == projectsdomain.ProjectStatusFinished,
	}, nil
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

// ChangeStatus aplica un cambio de estado en una transacción, con el orden de bloqueos de research.md
// sección 10: proyecto (SHARE OF projects), defecto (UPDATE) y su sprint de detección, sprint de
// resolución (KEY SHARE) si viene, PlanTransition con el estado releído y, por último, la escritura.
// Devuelve el detalle leído dentro de la misma transacción.
func (r *DefectRepository) ChangeStatus(ctx context.Context, defectID, actorID uuid.UUID, change domain.StatusChange, now time.Time) (*domain.DefectView, error) {
	var view *domain.DefectView
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Paso 1: el proyecto del defecto se lee con SHARE para que no pase a finalizado durante el cambio.
		var project projectsdomain.Project
		err := tx.
			Table("defects").
			Select("projects.id", "projects.status").
			Joins("JOIN user_stories ON user_stories.id = defects.story_id").
			Joins("JOIN projects ON projects.id = user_stories.project_id").
			Where("defects.id = ?", defectID).
			Clauses(clause.Locking{Strength: "SHARE", Table: clause.Table{Name: "projects"}}).
			Take(&project).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrDefectNotFound
		}
		if err != nil {
			return fmt.Errorf("bloquear el proyecto del defecto: %w", err)
		}
		if project.Status == projectsdomain.ProjectStatusFinished {
			return domain.ErrDefectsReadOnly
		}

		// Paso 2: el defecto se relee con FOR UPDATE, junto con su sprint de detección, para decidir
		// la transición sobre el estado vigente y serializar los cambios concurrentes (FR-018).
		var defect domain.Defect
		err = tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "status", "detected_sprint_id").
			Where("id = ?", defectID).
			Take(&defect).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrDefectNotFound
		}
		if err != nil {
			return fmt.Errorf("bloquear el defecto: %w", err)
		}
		detected, err := readSprintAnchor(tx.Where("id = ?", defect.DetectedSprintID))
		if err != nil {
			return fmt.Errorf("leer el sprint de detección: %w", err)
		}

		// Paso 3: el sprint de resolución, si viene, debe ser del proyecto; KEY SHARE impide borrarlo.
		var resolution *domain.SprintAnchor
		if change.ResolvedSprintID != nil {
			anchor, err := readSprintAnchor(tx.
				Clauses(clause.Locking{Strength: "KEY SHARE"}).
				Where("id = ? AND project_id = ?", *change.ResolvedSprintID, project.ID))
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return sprintsdomain.ErrSprintNotFound
			}
			if err != nil {
				return fmt.Errorf("bloquear el sprint de resolución: %w", err)
			}
			resolution = &anchor
		}

		// Paso 4: la regla de dominio decide sobre el estado releído y fija el sprint de resolución.
		resolvedSprintID, err := domain.PlanTransition(defect.Status, change.To, detected, resolution)
		if err != nil {
			return err
		}

		// Paso 5: se escriben el estado, el sprint de resolución que devolvió la regla y el autor y el
		// instante del cambio; updated_at lo completa GORM.
		err = tx.Model(&domain.Defect{}).
			Where("id = ?", defectID).
			Updates(map[string]any{
				"status":               change.To,
				"resolved_sprint_id":   resolvedSprintID,
				"status_changed_at":    now,
				"status_changed_by_id": actorID,
				"updated_by_id":        actorID,
			}).Error
		if err != nil {
			return fmt.Errorf("actualizar el estado del defecto: %w", err)
		}

		// El detalle se lee con la transacción, para devolver lo escrito sin una lectura posterior.
		view, err = NewDefectRepository(tx).FindView(ctx, defectID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// readSprintAnchor lee el identificador, el estado y la fecha de inicio del sprint que selecciona la
// consulta recibida; si no hay fila devuelve gorm.ErrRecordNotFound para que quien llama lo traduzca.
func readSprintAnchor(query *gorm.DB) (domain.SprintAnchor, error) {
	var sprint sprintsdomain.Sprint
	if err := query.Select("id", "status", "start_date").Take(&sprint).Error; err != nil {
		return domain.SprintAnchor{}, err
	}
	return domain.SprintAnchor{ID: sprint.ID, Status: sprint.Status, StartDate: sprint.StartDate}, nil
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

// viewQuery arma la consulta base de la vista de un defecto: las columnas de defects con la historia, los
// sprints y los usuarios resueltos por JOIN; el sprint de resolución es LEFT JOIN porque puede ser nulo.
func (r *DefectRepository) viewQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
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
		Joins("JOIN users su ON su.id = d.status_changed_by_id")
}

// toView arma la vista con las referencias mínimas de cada relación a partir de la fila plana.
func (row defectViewRow) toView() domain.DefectView {
	view := domain.DefectView{
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
	return view
}

// FindView devuelve el detalle de un defecto con historia, sprints y usuarios resueltos por JOIN;
// si no existe devuelve ErrDefectNotFound.
func (r *DefectRepository) FindView(ctx context.Context, defectID uuid.UUID) (*domain.DefectView, error) {
	var row defectViewRow
	res := r.viewQuery(ctx).
		Where("d.id = ?", defectID).
		Limit(1).
		Scan(&row)
	if res.Error != nil {
		return nil, fmt.Errorf("leer el defecto: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrDefectNotFound
	}
	view := row.toView()
	return &view, nil
}

// severityOrder ordena por gravedad descendente: crítica, alta, media y baja.
const severityOrder = "CASE d.severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END"

// ListForProject lista los defectos del proyecto (los de sus historias) con los filtros combinados
// de forma conjuntiva, de mayor a menor gravedad y, a igual gravedad, del más reciente al más antiguo.
// No pagina; devuelve una lista vacía y no nil cuando nada coincide.
func (r *DefectRepository) ListForProject(ctx context.Context, projectID uuid.UUID, filter domain.DefectFilter) ([]domain.DefectView, error) {
	query := r.viewQuery(ctx).Where("us.project_id = ?", projectID)
	if len(filter.Statuses) > 0 {
		query = query.Where("d.status IN ?", filter.Statuses)
	}
	if len(filter.Severities) > 0 {
		query = query.Where("d.severity IN ?", filter.Severities)
	}
	if filter.StoryID != nil {
		query = query.Where("d.story_id = ?", *filter.StoryID)
	}
	if filter.SprintID != nil {
		query = query.Where("(d.detected_sprint_id = ? OR d.resolved_sprint_id = ?)", *filter.SprintID, *filter.SprintID)
	}

	var rows []defectViewRow
	if err := query.Order(severityOrder + ", d.created_at DESC, d.id DESC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("listar los defectos del proyecto: %w", err)
	}
	views := make([]domain.DefectView, 0, len(rows))
	for _, row := range rows {
		views = append(views, row.toView())
	}
	return views, nil
}

// ValidateFilter verifica que la historia y el sprint del filtro pertenezcan al proyecto. Un
// identificador inexistente y uno de otro proyecto se rechazan igual, con InvalidFilterError.
func (r *DefectRepository) ValidateFilter(ctx context.Context, projectID uuid.UUID, filter domain.DefectFilter) error {
	if filter.StoryID != nil {
		var count int64
		err := r.db.WithContext(ctx).Model(&backlogdomain.UserStory{}).
			Where("id = ? AND project_id = ?", *filter.StoryID, projectID).
			Count(&count).Error
		if err != nil {
			return fmt.Errorf("validar la historia del filtro: %w", err)
		}
		if count == 0 {
			return &domain.InvalidFilterError{Field: "storyId"}
		}
	}
	if filter.SprintID != nil {
		var count int64
		err := r.db.WithContext(ctx).Model(&sprintsdomain.Sprint{}).
			Where("id = ? AND project_id = ?", *filter.SprintID, projectID).
			Count(&count).Error
		if err != nil {
			return fmt.Errorf("validar el sprint del filtro: %w", err)
		}
		if count == 0 {
			return &domain.InvalidFilterError{Field: "sprintId"}
		}
	}
	return nil
}

// HasProjectDefects indica si alguna historia del proyecto tiene al menos un defecto.
func (r *DefectRepository) HasProjectDefects(ctx context.Context, projectID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.WithContext(ctx).
		Table("defects AS d").
		Select("COUNT(*) > 0").
		Joins("JOIN user_stories us ON us.id = d.story_id").
		Where("us.project_id = ?", projectID).
		Scan(&exists).Error
	if err != nil {
		return false, fmt.Errorf("buscar defectos del proyecto: %w", err)
	}
	return exists, nil
}
