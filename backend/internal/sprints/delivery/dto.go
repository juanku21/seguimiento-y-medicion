package delivery

// SprintInput es el cuerpo de la creación y de la modificación de un sprint
// (contracts/openapi.yaml, FR-001 a FR-004, FR-012). Las fechas llegan como texto para que la
// validación distinga una fecha ausente de una mal formada y responda por campo (FR-011).
type SprintInput struct {
	Name      string `json:"name"`
	Goal      string `json:"goal"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}
