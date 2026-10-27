package delivery

// DefectInput es el cuerpo del alta de un defecto. No declara ningún campo de autoría: un createdBy
// o createdById que llegue en el cuerpo se ignora (FR-056).
type DefectInput struct {
	// Texto libre de 1 a 2000 caracteres tras recortar los espacios extremos (FR-002).
	Description string `json:"description" validate:"required" minLength:"1" maxLength:"2000"`
	// Severidad: critical, high, medium o low (FR-003).
	Severity string `json:"severity" validate:"required" enums:"critical,high,medium,low"`
	// Historia del mismo proyecto, en cualquier estado (FR-004).
	StoryID string `json:"storyId" validate:"required" format:"uuid"`
	// Sprint de detección del mismo proyecto, activo o cerrado (FR-006).
	DetectedSprintID string `json:"detectedSprintId" validate:"required" format:"uuid"`
}
