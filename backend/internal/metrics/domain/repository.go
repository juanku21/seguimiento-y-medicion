package domain

import (
	"context"

	"github.com/google/uuid"
)

// MetricsRepository entrega los datos crudos que el servicio necesita para calcular métricas.
// Todas las lecturas de una operación ocurren en una transacción REPEATABLE READ de solo lectura,
// de modo que el resultado sea una instantánea consistente. Ninguna operación devuelve
// gorm.ErrRecordNotFound: la ausencia se expresa con ErrSprintNotFound o con nil, según el caso.
type MetricsRepository interface {
	// LoadProject carga todos los datos del proyecto, incluidos sus sprints.
	LoadProject(ctx context.Context, projectID uuid.UUID) (*ProjectData, error)

	// LoadSprint carga un sprint y devuelve además el factor vigente del proyecto.
	// Devuelve ErrSprintNotFound si el sprint no existe o pertenece a otro proyecto.
	LoadSprint(ctx context.Context, projectID, sprintID uuid.UUID) (*SprintData, int, error)

	// LoadActiveSprint carga el sprint Activo y devuelve además el factor vigente del proyecto.
	// Devuelve nil sin error si el proyecto no tiene un sprint Activo.
	LoadActiveSprint(ctx context.Context, projectID uuid.UUID) (*SprintData, int, error)
}
