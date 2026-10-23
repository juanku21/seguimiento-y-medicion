package domain

import (
	"time"

	"github.com/google/uuid"
)

// EstimationRound es un intento de estimación dentro de una sesión; se persiste en la tabla
// estimation_rounds (data-model.md sección 2). Number numera las rondas de la sesión desde 1 y sin
// límite, y la ronda vigente es la de mayor número. Status es un RoundStatus (voting, revealed)
// guardado como texto y no como enum de PostgreSQL; la ronda nace voting. RevealedAt solo se completa
// al revelar y CreatedAt es el momento de apertura de la ronda. El índice único (session_id, number)
// impide dos rondas con el mismo número en una sesión y sirve para leer la ronda vigente. El
// resultado de la ronda no se guarda: se deriva de los votos (research.md sección 5). El campo
// Session existe solo para que AutoMigrate cree la clave foránea hacia estimation_sessions.id, sin
// ON DELETE: no es columna y nunca se precarga. CreatedAt y UpdatedAt los completa GORM.
type EstimationRound struct {
	ID         uuid.UUID          `gorm:"type:uuid;primaryKey"`
	SessionID  uuid.UUID          `gorm:"type:uuid;not null;uniqueIndex:idx_estimation_rounds_session_number,priority:1"`
	Session    *EstimationSession `gorm:"foreignKey:SessionID;references:ID"`
	Number     int                `gorm:"type:integer;not null;uniqueIndex:idx_estimation_rounds_session_number,priority:2"`
	Status     RoundStatus        `gorm:"type:varchar(16);not null"`
	RevealedAt *time.Time         `gorm:"type:timestamptz"`
	CreatedAt  time.Time          `gorm:"type:timestamptz;not null"`
	UpdatedAt  time.Time          `gorm:"type:timestamptz;not null"`
}
