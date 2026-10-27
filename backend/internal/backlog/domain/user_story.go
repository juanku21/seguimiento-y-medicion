// Package domain declara las entidades, los errores y las interfaces del módulo del backlog; no
// depende de ninguna otra capa (Principio III).
package domain

import (
	"time"

	"github.com/google/uuid"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// UserStory es la unidad de trabajo que el equipo quiere construir y medir; se persiste en la tabla
// user_stories (data-model.md sección 1). El ID lo genera la aplicación con google/uuid. Title no
// tiene índice único porque dos historias del mismo proyecto pueden llamarse igual, y Description es
// anulable porque la vacía se guarda como NULL. StoryPoints es anulable: NULL significa "sin
// estimar" y 0 es una estimación válida. Priority y Status usan los tipos StoryPriority y
// StoryStatus, que se guardan como texto con los valores de la API (high, medium, low y pending,
// in_progress, completed) y no como enum de PostgreSQL. El índice
// compuesto (project_id, priority, created_at, id) sirve al filtro por proyecto del listado y, en
// parte, a su orden. EverCommitted (columna ever_committed, NOT NULL DEFAULT false) registra si la
// historia estuvo alguna vez comprometida en un sprint: solo la feature de sprints la escribe, pasa a
// true al primer compromiso y nunca vuelve a false, aunque se quite la historia o se elimine el
// sprint (004, research.md sección 7). EverHadEffort (columna ever_had_effort, NOT NULL DEFAULT
// false) registra si la historia tuvo alguna vez esfuerzo cargado: pasa a true con el primer registro
// de esfuerzo y nunca vuelve a false, aunque se eliminen todos los registros (RC-05, RN-22); solo la
// escribe la feature de esfuerzo (006). EverHadDefects (columna ever_had_defects, NOT NULL DEFAULT
// false) pasa a true cuando la historia recibe su primer defecto —por alta o por revinculación— y
// nunca vuelve a false, aunque el defecto se revincule a otra historia o se descarte (RC-05, RN-19);
// solo la escribe la feature de defectos (007). Los campos Project y CreatedBy existen solo para que
// AutoMigrate cree las claves foráneas hacia projects.id y users.id: no son columnas y nunca se
// precargan. No hay columnas de lista para planificar, rango de prioridad, borrado lógico ni último
// modificador (plan.md, reevaluación del Principio I), y CreatedAt y UpdatedAt los completa GORM.
type UserStory struct {
	ID             uuid.UUID               `gorm:"type:uuid;primaryKey;index:idx_user_stories_project_order,priority:4"`
	ProjectID      uuid.UUID               `gorm:"type:uuid;not null;index:idx_user_stories_project_order,priority:1"`
	Project        *projectsdomain.Project `gorm:"foreignKey:ProjectID;references:ID"`
	Title          string                  `gorm:"type:varchar(150);not null"`
	Description    *string                 `gorm:"type:varchar(2000)"`
	Priority       StoryPriority           `gorm:"type:varchar(8);not null;index:idx_user_stories_project_order,priority:2"`
	Status         StoryStatus             `gorm:"type:varchar(16);not null"`
	StoryPoints    *int                    `gorm:"type:smallint"`
	EverCommitted  bool                    `gorm:"type:boolean;not null;default:false"`
	EverHadEffort  bool                    `gorm:"type:boolean;not null;default:false"`
	EverHadDefects bool                    `gorm:"type:boolean;not null;default:false"`
	CreatedByID    uuid.UUID               `gorm:"type:uuid;not null"`
	CreatedBy      *authdomain.User        `gorm:"foreignKey:CreatedByID;references:ID"`
	CreatedAt      time.Time               `gorm:"type:timestamptz;not null;index:idx_user_stories_project_order,priority:3"`
	UpdatedAt      time.Time               `gorm:"type:timestamptz;not null"`
}
