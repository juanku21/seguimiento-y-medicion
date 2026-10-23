package domain

import (
	"time"

	"github.com/google/uuid"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// EstimationParticipant es la presencia de un integrante en una sesión; se persiste en la tabla
// estimation_participants (data-model.md sección 3). Hay una sola fila por persona y sesión, sin
// importar cuántas conexiones abra (RN-34), y nunca se borra: quien se reconecta reutiliza la suya
// (FR-015). Connected es verdadero mientras la persona tenga una conexión viva o esté dentro de la
// gracia de 30 s; no lleva valor por defecto, así que GORM siempre incluye la columna en el INSERT,
// también cuando vale false. DisconnectedAt es el momento en que pasó a desconectado y CreatedAt es
// el momento de incorporación. El índice único (session_id, user_id) impide dos filas para la misma
// persona en una sesión. El facilitador no se guarda acá: es estimation_sessions.facilitator_id. Los
// campos Session y User existen solo para que AutoMigrate cree las claves foráneas hacia
// estimation_sessions.id y users.id, sin ON DELETE: no son columnas y nunca se precargan. CreatedAt
// y UpdatedAt los completa GORM.
type EstimationParticipant struct {
	ID             uuid.UUID          `gorm:"type:uuid;primaryKey"`
	SessionID      uuid.UUID          `gorm:"type:uuid;not null;uniqueIndex:idx_estimation_participants_session_user,priority:1"`
	Session        *EstimationSession `gorm:"foreignKey:SessionID;references:ID"`
	UserID         uuid.UUID          `gorm:"type:uuid;not null;uniqueIndex:idx_estimation_participants_session_user,priority:2"`
	User           *authdomain.User   `gorm:"foreignKey:UserID;references:ID"`
	Connected      bool               `gorm:"type:boolean;not null"`
	DisconnectedAt *time.Time         `gorm:"type:timestamptz"`
	CreatedAt      time.Time          `gorm:"type:timestamptz;not null"`
	UpdatedAt      time.Time          `gorm:"type:timestamptz;not null"`
}
