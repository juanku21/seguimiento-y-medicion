package domain

import (
	"context"

	"github.com/google/uuid"
)

// UserRef es la vista mínima de una cuenta que el módulo de proyectos puede ver (data-model.md
// sección 5; research.md sección 8): solo identificador, nombre completo y correo. No tiene ningún
// campo de contraseña, de modo que el hash no entra al módulo ni como campo en memoria. No es un
// modelo persistido.
type UserRef struct {
	ID       uuid.UUID
	FullName string
	Email    string
}

// UserDirectory es el puerto de solo lectura sobre la tabla users para el alta de integrantes por
// correo (FR-019, FR-020). FindByEmail recibe el correo ya normalizado por el servicio y, si no
// existe una cuenta con ese correo, devuelve ErrUserNotRegistered (FR-021), nunca
// gorm.ErrRecordNotFound ni (nil, nil). No se declara ninguna operación de escritura sobre users.
type UserDirectory interface {
	FindByEmail(ctx context.Context, normalizedEmail string) (*UserRef, error)
}

// BacklogSummary es el resumen del backlog que muestra la vista de estado (data-model.md sección 4;
// FR-042). CountsByState es un mapa de estado a cantidad y no un enumerado, porque el conjunto de
// estados del backlog lo define la spec 003. EstimatedStoryPoints suma solo las historias estimadas:
// las que están sin estimar se cuentan por estado pero no suman, y una estimada en 0 suma 0 (FR-045).
type BacklogSummary struct {
	CountsByState        map[string]int
	EstimatedStoryPoints int
}

// BacklogSummaryProvider es el puerto hacia el backlog de la spec 003 (research.md sección 9).
// Summarize devuelve el resumen del backlog del proyecto; un backlog sin historias no es un error y
// se informa con CountsByState vacío y EstimatedStoryPoints en 0 (FR-044).
type BacklogSummaryProvider interface {
	Summarize(ctx context.Context, projectID uuid.UUID) (BacklogSummary, error)
}

// ActiveSprint identifica el sprint activo de un proyecto, con su identificador y su nombre y nada
// más (data-model.md sección 4; FR-042).
type ActiveSprint struct {
	ID   uuid.UUID
	Name string
}

// SprintDirectory es el puerto hacia los sprints de la spec 004 (research.md sección 9), que usan la
// vista de estado y la regla de finalización (FR-041, RN-16). FindActiveSprint devuelve (nil, nil)
// cuando el proyecto no tiene sprint activo, porque esa ausencia no es un error (FR-043).
type SprintDirectory interface {
	FindActiveSprint(ctx context.Context, projectID uuid.UUID) (*ActiveSprint, error)
}
