package delivery

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// Pruebas unitarias obligatorias de la validación y la normalización de la creación de historias
// (T013, RN-03 a RN-08). Los nombres llevan el prefijo "Validacion" para no chocar con las pruebas
// de integración de handler_test.go (T015), que cubren los mismos escenarios de punta a punta. Los
// ayudantes llevan el prefijo "validation" por la misma razón.

// validationScale es el texto con la escala completa que debe enumerar el mensaje de rechazo de los
// Story Points (FR-019, contracts/openapi.yaml).
const validationScale = "0, 1, 2, 3, 5, 8, 13, 20, 40 y 100"

// validationTitle es el título del caso normal de la spec (US1, escenario 1).
const validationTitle = "Exportar el reporte de velocidad a PDF"

// validationStrPtr devuelve un puntero a la cadena recibida, para armar descripciones opcionales.
func validationStrPtr(s string) *string {
	return &s
}

// validationPoints devuelve un puntero al json.Number con el texto literal recibido, tal como lo
// deja el decodificador de JSON.
func validationPoints(literal string) *json.Number {
	n := json.Number(literal)
	return &n
}

// validationInput arma una entrada válida mínima (título y prioridad) sobre la que cada prueba
// modifica solo el campo que le interesa.
func validationInput() StoryInput {
	return StoryInput{Title: validationTitle, Priority: domain.StoryPriorityHigh}
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
// que su mensaje mencione cada una de las piezas recibidas sin distinguir mayúsculas.
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

// validationRequireValid exige que la entrada no tenga campos inválidos y que el título y la
// prioridad normalizados sean los esperados, para que un resultado vacío nunca pase por válido.
func validationRequireValid(t *testing.T, got ValidatedStory, fields []authdelivery.FieldError, title string, priority domain.StoryPriority) {
	t.Helper()
	require.Empty(t, fields, "la entrada debería ser válida")
	require.Equal(t, title, got.Title)
	require.Equal(t, priority, got.Priority)
}

// Escenario: Creación completa (003/US1)
// Caso normal: título, descripción, prioridad Alta, 5 Story Points y dos criterios pasan la
// validación; título, descripción y criterios quedan recortados y los criterios conservan el orden
// en que se enviaron (RN-03, RN-04, RN-08).
func TestUS1_ValidacionCreacionCompleta(t *testing.T) {
	req := StoryInput{
		Title:       "  " + validationTitle + "  ",
		Description: validationStrPtr("  El equipo necesita compartir la velocidad con la gerencia.  "),
		Priority:    domain.StoryPriorityHigh,
		StoryPoints: validationPoints("5"),
		AcceptanceCriteria: []string{
			"  El PDF incluye la velocidad de los últimos cinco sprints.  ",
			"\tEl archivo se descarga con la fecha en el nombre.\n",
		},
	}

	got, fields := ValidateStoryInput(req)

	validationRequireValid(t, got, fields, validationTitle, domain.StoryPriorityHigh)
	require.NotNil(t, got.Description)
	assert.Equal(t, "El equipo necesita compartir la velocidad con la gerencia.", *got.Description)
	require.NotNil(t, got.StoryPoints)
	assert.Equal(t, 5, *got.StoryPoints)
	assert.Equal(t, []string{
		"El PDF incluye la velocidad de los últimos cinco sprints.",
		"El archivo se descarga con la fecha en el nombre.",
	}, got.AcceptanceCriteria)
}

// Escenario: Creación completa (003/US1)
// Cada valor de la escala, enviado como texto literal entero, se acepta y se convierte a su entero
// (RN-06).
func TestUS1_ValidacionStoryPointsDeLaEscala(t *testing.T) {
	for _, value := range []int{0, 1, 2, 3, 5, 8, 13, 20, 40, 100} {
		t.Run(fmt.Sprintf("%d", value), func(t *testing.T) {
			req := validationInput()
			req.StoryPoints = validationPoints(fmt.Sprintf("%d", value))

			got, fields := ValidateStoryInput(req)

			validationRequireValid(t, got, fields, validationTitle, domain.StoryPriorityHigh)
			require.NotNil(t, got.StoryPoints, "el valor %d es una estimación, no sin estimar", value)
			assert.Equal(t, value, *got.StoryPoints)
		})
	}
}

// Escenario: Solo título y prioridad (003/US1)
// Con solo título y prioridad la historia es válida: queda sin estimar, sin descripción y sin
// criterios. La descripción ausente, nula, vacía o solo de espacios se guarda como nula (RN-04) y
// los Story Points ausentes o null significan "sin estimar" (RN-07). Las entradas se decodifican
// desde JSON para cubrir la diferencia entre campo ausente y null.
func TestUS1_ValidacionSoloTituloYPrioridad(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		priority domain.StoryPriority
	}{
		{"campos opcionales ausentes", `{"title":"` + validationTitle + `","priority":"high"}`, domain.StoryPriorityHigh},
		{"campos opcionales nulos", `{"title":"` + validationTitle + `","priority":"medium","description":null,"storyPoints":null,"acceptanceCriteria":null}`, domain.StoryPriorityMedium},
		{"descripción vacía y lista de criterios vacía", `{"title":"` + validationTitle + `","priority":"low","description":"","acceptanceCriteria":[]}`, domain.StoryPriorityLow},
		{"descripción solo de espacios", `{"title":"` + validationTitle + `","priority":"low","description":"   \t  "}`, domain.StoryPriorityLow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req StoryInput
			require.NoError(t, json.Unmarshal([]byte(tc.body), &req))

			got, fields := ValidateStoryInput(req)

			validationRequireValid(t, got, fields, validationTitle, tc.priority)
			assert.Nil(t, got.Description, "la descripción debería quedar nula")
			assert.Nil(t, got.StoryPoints, "la historia debería quedar sin estimar")
			assert.Empty(t, got.AcceptanceCriteria, "la historia no debería tener criterios")
		})
	}
}

// Escenario: Solo título y prioridad (003/US1)
// La descripción opcional admite hasta 2000 runas contadas tras recortar los espacios extremos;
// 2001 se rechaza indicando el límite (RN-04). Se usan caracteres de varios bytes para comprobar que
// se cuentan runas y no bytes.
func TestUS1_ValidacionLimitesDeDescripcion(t *testing.T) {
	valid := []struct {
		name  string
		input string
		want  string
	}{
		{"2000 caracteres", strings.Repeat("a", 2000), strings.Repeat("a", 2000)},
		{"2000 caracteres con espacios extremos", "  " + strings.Repeat("a", 2000) + "  ", strings.Repeat("a", 2000)},
		{"2000 caracteres de varios bytes", strings.Repeat("ñ", 2000), strings.Repeat("ñ", 2000)},
	}
	for _, tc := range valid {
		t.Run("válida: "+tc.name, func(t *testing.T) {
			req := validationInput()
			req.Description = validationStrPtr(tc.input)

			got, fields := ValidateStoryInput(req)

			validationRequireValid(t, got, fields, validationTitle, domain.StoryPriorityHigh)
			require.NotNil(t, got.Description)
			assert.Equal(t, tc.want, *got.Description)
		})
	}

	invalid := []struct {
		name  string
		input string
	}{
		{"2001 caracteres", strings.Repeat("a", 2001)},
		{"2001 caracteres de varios bytes", strings.Repeat("ñ", 2001)},
	}
	for _, tc := range invalid {
		t.Run("inválida: "+tc.name, func(t *testing.T) {
			req := validationInput()
			req.Description = validationStrPtr(tc.input)

			_, fields := ValidateStoryInput(req)

			validationRequireOnlyField(t, fields, "description", "2000")
		})
	}
}

// Escenario: Cero Story Points (003/US1)
// El texto literal 0 es una estimación válida y queda como el entero 0, distinto de "sin estimar"
// (nil) (RN-07).
func TestUS1_ValidacionCeroStoryPoints(t *testing.T) {
	req := validationInput()
	req.StoryPoints = validationPoints("0")

	got, fields := ValidateStoryInput(req)

	validationRequireValid(t, got, fields, validationTitle, domain.StoryPriorityHigh)
	require.NotNil(t, got.StoryPoints, "0 es una estimación y no debe confundirse con sin estimar")
	assert.Equal(t, 0, *got.StoryPoints)
}

// Escenario: Límites del título (003/US1)
// El título mide entre 1 y 150 runas contadas tras recortar los espacios extremos y se guarda
// recortado; 150 es válido y 151 se rechaza indicando el límite (RN-03).
func TestUS1_ValidacionLimitesDelTitulo(t *testing.T) {
	valid := []struct {
		name  string
		input string
		want  string
	}{
		{"1 carácter", "E", "E"},
		{"150 caracteres", strings.Repeat("a", 150), strings.Repeat("a", 150)},
		{"150 caracteres con espacios extremos", "   " + strings.Repeat("a", 150) + "   ", strings.Repeat("a", 150)},
		{"150 caracteres de varios bytes", strings.Repeat("ñ", 150), strings.Repeat("ñ", 150)},
	}
	for _, tc := range valid {
		t.Run("válido: "+tc.name, func(t *testing.T) {
			req := validationInput()
			req.Title = tc.input

			got, fields := ValidateStoryInput(req)

			validationRequireValid(t, got, fields, tc.want, domain.StoryPriorityHigh)
		})
	}

	invalid := []struct {
		name  string
		input string
	}{
		{"151 caracteres", strings.Repeat("a", 151)},
		{"151 caracteres de varios bytes", strings.Repeat("ñ", 151)},
	}
	for _, tc := range invalid {
		t.Run("inválido: "+tc.name, func(t *testing.T) {
			req := validationInput()
			req.Title = tc.input

			_, fields := ValidateStoryInput(req)

			validationRequireOnlyField(t, fields, "title", "150")
		})
	}
}

// Escenario: Límites del criterio (003/US1)
// Cada criterio mide entre 1 y 500 runas tras recortar los espacios extremos: 500 y 1 se aceptan y
// conservan su orden; 501 se rechaza informando el campo con el índice del criterio (RN-08).
func TestUS1_ValidacionLimitesDelCriterio(t *testing.T) {
	t.Run("válidos: 500 y 1 caracteres en orden", func(t *testing.T) {
		req := validationInput()
		req.AcceptanceCriteria = []string{
			"  " + strings.Repeat("ñ", 500) + "  ",
			"X",
		}

		got, fields := ValidateStoryInput(req)

		validationRequireValid(t, got, fields, validationTitle, domain.StoryPriorityHigh)
		assert.Equal(t, []string{strings.Repeat("ñ", 500), "X"}, got.AcceptanceCriteria)
	})

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
			req.AcceptanceCriteria = []string{"El PDF incluye la velocidad.", tc.input}

			_, fields := ValidateStoryInput(req)

			validationRequireOnlyField(t, fields, "acceptanceCriteria[1]", "500")
		})
	}
}

// Escenario: Campos obligatorios (003/US1)
// Un título ausente o solo de espacios se rechaza (RN-03), y una prioridad ausente o distinta de
// high, medium y low también (RN-05); en cada caso se informa el campo a corregir.
func TestUS1_ValidacionCamposObligatorios(t *testing.T) {
	titles := []struct {
		name  string
		input string
	}{
		{"sin título", ""},
		{"título solo de espacios", "   \t\n "},
	}
	for _, tc := range titles {
		t.Run(tc.name, func(t *testing.T) {
			req := validationInput()
			req.Title = tc.input

			_, fields := ValidateStoryInput(req)

			validationRequireOnlyField(t, fields, "title")
		})
	}

	priorities := []struct {
		name  string
		input domain.StoryPriority
	}{
		{"sin prioridad", ""},
		{"prioridad en español", "alta"},
		{"prioridad en mayúsculas", "HIGH"},
		{"prioridad inexistente", "urgent"},
	}
	for _, tc := range priorities {
		t.Run(tc.name, func(t *testing.T) {
			req := validationInput()
			req.Priority = tc.input

			_, fields := ValidateStoryInput(req)

			validationRequireOnlyField(t, fields, "priority")
		})
	}
}

// Escenario: Campos obligatorios (003/US1)
// La validación devuelve todos los campos inválidos a la vez, cada criterio con su índice, para que
// la persona corrija todo en un solo intento (FR-013).
func TestUS1_ValidacionTodosLosCamposInvalidos(t *testing.T) {
	req := StoryInput{
		Title:       "   ",
		Description: validationStrPtr(strings.Repeat("a", 2001)),
		Priority:    "",
		StoryPoints: validationPoints("4"),
		AcceptanceCriteria: []string{
			"El PDF incluye la velocidad.",
			"  ",
			strings.Repeat("a", 501),
		},
	}

	_, fields := ValidateStoryInput(req)

	assert.ElementsMatch(t, []string{
		"title",
		"description",
		"priority",
		"storyPoints",
		"acceptanceCriteria[1]",
		"acceptanceCriteria[2]",
	}, validationFieldNames(fields))
}

// Escenario: Story Points fuera de la escala (003/US1)
// Los Story Points se validan sobre el texto literal: los enteros fuera de la escala, los negativos,
// los decimales (aunque su parte decimal sea cero) y la notación exponencial se rechazan con el
// mensaje que enumera la escala completa (RN-06, FR-019).
func TestUS1_ValidacionStoryPointsFueraDeEscala(t *testing.T) {
	for _, literal := range []string{"4", "6", "7", "50", "101", "-1", "6.5", "5.0", "1e1"} {
		t.Run(literal, func(t *testing.T) {
			req := validationInput()
			req.StoryPoints = validationPoints(literal)

			_, fields := ValidateStoryInput(req)

			validationRequireOnlyField(t, fields, "storyPoints", validationScale)
		})
	}
}

// Escenario: Criterio vacío (003/US1)
// Un criterio vacío o solo de espacios se rechaza indicando que debe tener contenido; el campo se
// informa con el índice que el criterio ocupa en la lista enviada (RN-08).
func TestUS1_ValidacionCriterioVacio(t *testing.T) {
	cases := []struct {
		name     string
		criteria []string
		field    string
	}{
		{"primer criterio vacío", []string{"", "El PDF incluye la velocidad."}, "acceptanceCriteria[0]"},
		{"segundo criterio solo de espacios", []string{"El PDF incluye la velocidad.", "   \t "}, "acceptanceCriteria[1]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validationInput()
			req.AcceptanceCriteria = tc.criteria

			_, fields := ValidateStoryInput(req)

			validationRequireOnlyField(t, fields, tc.field, "criterio")
		})
	}
}
