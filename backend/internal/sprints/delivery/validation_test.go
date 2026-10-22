package delivery

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas unitarias obligatorias de la validación y la normalización del cuerpo de un sprint (T011;
// RN-03, RN-04; data-model.md sección 1, reglas de validación). No llevan la etiqueta de un
// escenario de la spec: plan.md asigna los nombres TestUS1_* a las pruebas de integración de
// handler_test.go (T013), igual que T010 con PeriodsOverlap. Los ayudantes llevan el prefijo
// "validation" para no chocar con los de las pruebas de integración del mismo paquete.

// Datos del caso normal de la spec (US1, escenario 1).
const (
	validationName      = "Sprint 1"
	validationGoal      = "Cerrar el alta de clientes"
	validationStartDate = "2026-10-01"
	validationEndDate   = "2026-10-14"
)

// validationDay construye una fecha de octubre de 2026 a medianoche UTC, como se representan las
// fechas de los sprints en el dominio.
func validationDay(d int) time.Time {
	return time.Date(2026, time.October, d, 0, 0, 0, 0, time.UTC)
}

// validationInput arma una entrada válida completa sobre la que cada prueba modifica solo el campo
// que le interesa.
func validationInput() SprintInput {
	return SprintInput{
		Name:      validationName,
		Goal:      validationGoal,
		StartDate: validationStartDate,
		EndDate:   validationEndDate,
	}
}

// validationFieldNames extrae los nombres de los campos rechazados, para comparar sin depender del
// orden en que se informan.
func validationFieldNames(fields []authdelivery.FieldError) []string {
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, f.Field)
	}
	return names
}

// validationRequireOnlyField exige que la validación rechace exactamente un campo, el indicado, y
// que su mensaje no esté vacío y mencione cada una de las piezas recibidas sin distinguir
// mayúsculas.
func validationRequireOnlyField(t *testing.T, fields []authdelivery.FieldError, field string, parts ...string) {
	t.Helper()
	require.Equal(t, []string{field}, validationFieldNames(fields),
		"se esperaba un único campo inválido %q", field)
	msg := strings.ToLower(fields[0].Message)
	require.NotEmpty(t, msg, "el mensaje del campo %q no puede estar vacío", field)
	for _, part := range parts {
		assert.Contains(t, msg, strings.ToLower(part), "el mensaje del campo %q debe mencionar %q", field, part)
	}
}

// validationRequireValid exige que la entrada no tenga campos inválidos y que los datos
// normalizados sean exactamente los esperados, para que un resultado vacío nunca pase por válido.
func validationRequireValid(t *testing.T, got domain.SprintInput, fields []authdelivery.FieldError, want domain.SprintInput) {
	t.Helper()
	require.Empty(t, fields, "la entrada debería ser válida")
	assert.Equal(t, want, got)
}

// Caso normal: nombre, Sprint Goal y fechas válidos pasan la validación; el nombre y el Sprint Goal
// quedan recortados y las fechas se convierten a medianoche UTC (RN-03, RN-04).
func TestValidateSprintInput_EntradaCompleta(t *testing.T) {
	req := validationInput()
	req.Name = "  " + validationName + "\t"
	req.Goal = "\n " + validationGoal + "  "

	got, fields := ValidateSprintInput(req)

	validationRequireValid(t, got, fields, domain.SprintInput{
		Name:      validationName,
		Goal:      validationGoal,
		StartDate: validationDay(1),
		EndDate:   validationDay(14),
	})
}

// El nombre mide entre 1 y 100 runas contadas tras recortar los espacios extremos y se devuelve
// recortado; 100 es válido y 101 se rechaza indicando el límite (RN-03, FR-002).
func TestValidateSprintInput_LimitesDelNombre(t *testing.T) {
	valid := []struct {
		name  string
		input string
		want  string
	}{
		{"1 carácter", "S", "S"},
		{"100 caracteres", strings.Repeat("a", 100), strings.Repeat("a", 100)},
		{"100 caracteres con espacios extremos", "   " + strings.Repeat("a", 100) + "   ", strings.Repeat("a", 100)},
		{"100 caracteres de varios bytes", strings.Repeat("ñ", 100), strings.Repeat("ñ", 100)},
	}
	for _, tc := range valid {
		t.Run("válido: "+tc.name, func(t *testing.T) {
			req := validationInput()
			req.Name = tc.input

			got, fields := ValidateSprintInput(req)

			validationRequireValid(t, got, fields, domain.SprintInput{
				Name:      tc.want,
				Goal:      validationGoal,
				StartDate: validationDay(1),
				EndDate:   validationDay(14),
			})
		})
	}

	invalid := []struct {
		name  string
		input string
	}{
		{"101 caracteres", strings.Repeat("a", 101)},
		{"101 caracteres de varios bytes", strings.Repeat("ñ", 101)},
	}
	for _, tc := range invalid {
		t.Run("inválido: "+tc.name, func(t *testing.T) {
			req := validationInput()
			req.Name = tc.input

			_, fields := ValidateSprintInput(req)

			validationRequireOnlyField(t, fields, "name", "100")
		})
	}
}

// El Sprint Goal mide entre 1 y 500 runas contadas tras recortar los espacios extremos y se
// devuelve recortado; 500 es válido y 501 se rechaza indicando el límite (RN-03, FR-003).
func TestValidateSprintInput_LimitesDelSprintGoal(t *testing.T) {
	valid := []struct {
		name  string
		input string
		want  string
	}{
		{"1 carácter", "G", "G"},
		{"500 caracteres", strings.Repeat("a", 500), strings.Repeat("a", 500)},
		{"500 caracteres con espacios extremos", "  " + strings.Repeat("a", 500) + "\n", strings.Repeat("a", 500)},
		{"500 caracteres de varios bytes", strings.Repeat("ñ", 500), strings.Repeat("ñ", 500)},
	}
	for _, tc := range valid {
		t.Run("válido: "+tc.name, func(t *testing.T) {
			req := validationInput()
			req.Goal = tc.input

			got, fields := ValidateSprintInput(req)

			validationRequireValid(t, got, fields, domain.SprintInput{
				Name:      validationName,
				Goal:      tc.want,
				StartDate: validationDay(1),
				EndDate:   validationDay(14),
			})
		})
	}

	invalid := []struct {
		name  string
		input string
	}{
		{"501 caracteres", strings.Repeat("a", 501)},
		{"501 caracteres de varios bytes", strings.Repeat("ñ", 501)},
	}
	for _, tc := range invalid {
		t.Run("inválido: "+tc.name, func(t *testing.T) {
			req := validationInput()
			req.Goal = tc.input

			_, fields := ValidateSprintInput(req)

			validationRequireOnlyField(t, fields, "goal", "500")
		})
	}
}

// El nombre y el Sprint Goal son obligatorios: ausentes o solo de espacios se rechazan informando
// el campo a corregir (RN-03).
func TestValidateSprintInput_NombreYSprintGoalObligatorios(t *testing.T) {
	cases := []struct {
		name  string
		field string
		input string
	}{
		{"sin nombre", "name", ""},
		{"nombre solo de espacios", "name", "   \t\n "},
		{"sin Sprint Goal", "goal", ""},
		{"Sprint Goal solo de espacios", "goal", " \t  \n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validationInput()
			if tc.field == "name" {
				req.Name = tc.input
			} else {
				req.Goal = tc.input
			}

			_, fields := ValidateSprintInput(req)

			validationRequireOnlyField(t, fields, tc.field, "obligatori")
		})
	}
}

// Las fechas de inicio y de fin son obligatorias: una fecha ausente se rechaza en su propio campo
// (RN-04, FR-004).
func TestValidateSprintInput_FechasObligatorias(t *testing.T) {
	t.Run("sin fecha de inicio", func(t *testing.T) {
		req := validationInput()
		req.StartDate = ""

		_, fields := ValidateSprintInput(req)

		validationRequireOnlyField(t, fields, "startDate", "obligatoria")
	})

	t.Run("sin fecha de fin", func(t *testing.T) {
		req := validationInput()
		req.EndDate = ""

		_, fields := ValidateSprintInput(req)

		validationRequireOnlyField(t, fields, "endDate", "obligatoria")
	})
}

// Las fechas deben tener el formato YYYY-MM-DD de un día que existe; cualquier otra forma, incluida
// una fecha con hora, se rechaza en su propio campo indicando el formato esperado (RN-04,
// research.md sección 4: time.DateOnly). La fecha inexistente solo verifica el campo, porque el
// mensaje para ese caso no está fijado en data-model.md.
func TestValidateSprintInput_FechasMalFormadas(t *testing.T) {
	cases := []struct {
		name  string
		input string
		parts []string
	}{
		{"separador barra", "2026/10/01", []string{"AAAA-MM-DD"}},
		{"orden día-mes-año", "01-10-2026", []string{"AAAA-MM-DD"}},
		{"día sin cero a la izquierda", "2026-10-1", []string{"AAAA-MM-DD"}},
		{"fecha con hora", "2026-10-01T00:00:00Z", []string{"AAAA-MM-DD"}},
		{"texto", "mañana", []string{"AAAA-MM-DD"}},
		{"fecha inexistente", "2026-02-30", nil},
	}
	for _, tc := range cases {
		t.Run("inicio: "+tc.name, func(t *testing.T) {
			req := validationInput()
			req.StartDate = tc.input

			_, fields := ValidateSprintInput(req)

			validationRequireOnlyField(t, fields, "startDate", tc.parts...)
		})
		t.Run("fin: "+tc.name, func(t *testing.T) {
			req := validationInput()
			req.EndDate = tc.input

			_, fields := ValidateSprintInput(req)

			validationRequireOnlyField(t, fields, "endDate", tc.parts...)
		})
	}
}

// La fecha de fin debe ser estrictamente posterior a la de inicio: igual o anterior se rechaza en
// el campo endDate indicando que debe ser posterior (RN-04, FR-004).
func TestValidateSprintInput_FinNoPosteriorAlInicio(t *testing.T) {
	cases := []struct {
		name string
		end  string
	}{
		{"fin igual al inicio", "2026-10-01"},
		{"fin un día anterior al inicio", "2026-09-30"},
		{"fin muy anterior al inicio", "2025-10-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validationInput()
			req.StartDate = "2026-10-01"
			req.EndDate = tc.end

			_, fields := ValidateSprintInput(req)

			validationRequireOnlyField(t, fields, "endDate", "posterior")
		})
	}
}

// La duración mínima admitida es de un día: la fecha de fin al día siguiente de la de inicio se
// acepta (RN-04, FR-004).
func TestValidateSprintInput_DuracionMinima(t *testing.T) {
	req := validationInput()
	req.StartDate = "2026-10-01"
	req.EndDate = "2026-10-02"

	got, fields := ValidateSprintInput(req)

	validationRequireValid(t, got, fields, domain.SprintInput{
		Name:      validationName,
		Goal:      validationGoal,
		StartDate: validationDay(1),
		EndDate:   validationDay(2),
	})
}

// La validación devuelve todos los campos inválidos a la vez, para que la persona corrija todo en
// un solo intento, y en ese caso no devuelve datos normalizados (FR-011).
func TestValidateSprintInput_TodosLosCamposInvalidos(t *testing.T) {
	cases := []struct {
		name string
		req  SprintInput
	}{
		{"todos ausentes", SprintInput{}},
		{"todos inválidos", SprintInput{
			Name:      strings.Repeat("a", 101),
			Goal:      "   ",
			StartDate: "2026/10/01",
			EndDate:   "14-10-2026",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, fields := ValidateSprintInput(tc.req)

			assert.ElementsMatch(t, []string{"name", "goal", "startDate", "endDate"}, validationFieldNames(fields))
			for _, f := range fields {
				assert.NotEmpty(t, f.Message, "el mensaje del campo %q no puede estar vacío", f.Field)
			}
			assert.Equal(t, domain.SprintInput{}, got)
		})
	}
}

// Pruebas unitarias de la validación del cuerpo del avance de una historia (T053; FR-033,
// contracts/openapi.yaml, StoryStatusInput). El cuerpo solo admite los tres valores de StoryStatus;
// si una transición está admitida o no lo decide después ValidateStoryProgress con un 409, de modo
// que pending es un valor válido para el cuerpo. Igual que las de ValidateSprintInput, no llevan la
// etiqueta de un escenario: los nombres TestUS4_* son de las pruebas de integración (T055).

// Los tres estados de la historia pasan la validación y se devuelven como StoryStatus.
func TestValidateStoryStatusInput_EstadosAdmitidos(t *testing.T) {
	cases := []struct {
		raw  string
		want backlogdomain.StoryStatus
	}{
		{"pending", backlogdomain.StoryStatusPending},
		{"in_progress", backlogdomain.StoryStatusInProgress},
		{"completed", backlogdomain.StoryStatusCompleted},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, fields := ValidateStoryStatusInput(StoryStatusInput{Status: tc.raw})

			require.Empty(t, fields, "el estado %q debería ser válido", tc.raw)
			assert.Equal(t, tc.want, got)
		})
	}
}

// Un estado ausente o vacío se rechaza en el campo status y no devuelve ningún estado.
func TestValidateStoryStatusInput_EstadoObligatorio(t *testing.T) {
	got, fields := ValidateStoryStatusInput(StoryStatusInput{})

	validationRequireOnlyField(t, fields, "status")
	assert.Empty(t, got)
}

// Un estado fuera de pending, in_progress y completed se rechaza en el campo status; la comparación
// es exacta, sin recortar espacios ni ignorar mayúsculas, como el enum del contrato.
func TestValidateStoryStatusInput_EstadoDesconocido(t *testing.T) {
	for _, raw := range []string{"done", "planned", "COMPLETED", "In_Progress", " completed", "pending ", "in-progress"} {
		t.Run(raw, func(t *testing.T) {
			got, fields := ValidateStoryStatusInput(StoryStatusInput{Status: raw})

			validationRequireOnlyField(t, fields, "status")
			assert.Empty(t, got)
		})
	}
}
