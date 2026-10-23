package domain

import (
	"time"

	"github.com/google/uuid"
)

// EstimationVote es la carta que un participante jugó en una ronda; se persiste en la tabla
// estimation_votes (data-model.md sección 4). Card es una Card guardada como texto de hasta tres
// caracteres. El índice único (round_id, participant_id) permite un solo voto por participante y
// ronda (RN-07): cambiar de carta actualiza la fila (FR-022) y, como retirarla no existe (RN-36), la
// fila nunca se borra. UpdatedAt es el momento en que quedó emitida la carta vigente. Los campos
// Round y Participant existen solo para que AutoMigrate cree las claves foráneas hacia
// estimation_rounds.id y estimation_participants.id, sin ON DELETE: no son columnas y nunca se
// precargan. CreatedAt y UpdatedAt los completa GORM.
type EstimationVote struct {
	ID            uuid.UUID              `gorm:"type:uuid;primaryKey"`
	RoundID       uuid.UUID              `gorm:"type:uuid;not null;uniqueIndex:idx_estimation_votes_round_participant,priority:1"`
	Round         *EstimationRound       `gorm:"foreignKey:RoundID;references:ID"`
	ParticipantID uuid.UUID              `gorm:"type:uuid;not null;uniqueIndex:idx_estimation_votes_round_participant,priority:2"`
	Participant   *EstimationParticipant `gorm:"foreignKey:ParticipantID;references:ID"`
	Card          Card                   `gorm:"type:varchar(3);not null"`
	CreatedAt     time.Time              `gorm:"type:timestamptz;not null"`
	UpdatedAt     time.Time              `gorm:"type:timestamptz;not null"`
}
