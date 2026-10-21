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

// Pruebas unitarias obligatorias de la validación de los filtros del listado (T029, RN-05, FR-034,
// FR-037). Reciben los valores tal como los entrega c.QueryArray, es decir, una lista por filtro con
// cada repetición del parámetro.

// validationFilterAdmittedStatuses y validationFilterAdmittedPriorities son los valores que el
// mensaje de rechazo de cada filtro debe enumerar (contracts/openapi.yaml, respuesta 400).
var (
	validationFilterAdmittedStatuses   = []string{"pending", "in_progress", "completed"}
	validationFilterAdmittedPriorities = []string{"high", "medium", "low"}
)

// validationRequireFilterRejected exige que el filtro se rechace solo por el campo indicado y que
// cada mensaje informado enumere todos los valores admitidos para ese filtro.
func validationRequireFilterRejected(t *testing.T, fields []authdelivery.FieldError, field string, admitted []string) {
	t.Helper()
	require.NotEmpty(t, fields, "el filtro %q con un valor no admitido debe rechazarse", field)
	for _, f := range fields {
		require.Equal(t, field, f.Field, "solo debe informarse el filtro %q", field)
		for _, value := range admitted {
			assert.Contains(t, f.Message, value, "el mensaje del filtro %q debe indicar el valor admitido %q", field, value)
		}
	}
}

// Escenario: Orden por prioridad y fecha (003/US2)
// Sin filtros la consulta se acepta y el filtro resultante no acota ni por estado ni por prioridad,
// tanto si los parámetros no llegan (nil) como si llegan como listas vacías.
func TestUS2_ValidacionSinFiltros(t *testing.T) {
	cases := []struct {
		name       string
		statuses   []string
		priorities []string
	}{
		{"parámetros ausentes", nil, nil},
		{"listas vacías", []string{}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filter, fields := ParseStoryFilter(tc.statuses, tc.priorities)

			require.Empty(t, fields, "la ausencia de filtros debe aceptarse")
			assert.Empty(t, filter.Statuses)
			assert.Empty(t, filter.Priorities)
		})
	}
}

// Escenario: Filtro por estado (003/US2)
// Cada estado admitido se acepta solo y el parámetro status es repetible: varios valores llegan
// juntos al filtro, sin acotar la prioridad.
func TestUS2_ValidacionFiltroPorEstado(t *testing.T) {
	cases := []struct {
		name     string
		statuses []string
		want     []domain.StoryStatus
	}{
		{"pending", []string{"pending"}, []domain.StoryStatus{domain.StoryStatusPending}},
		{"in_progress", []string{"in_progress"}, []domain.StoryStatus{domain.StoryStatusInProgress}},
		{"completed", []string{"completed"}, []domain.StoryStatus{domain.StoryStatusCompleted}},
		{"pending y in_progress repetidos", []string{"pending", "in_progress"},
			[]domain.StoryStatus{domain.StoryStatusPending, domain.StoryStatusInProgress}},
		{"los tres estados", []string{"pending", "in_progress", "completed"},
			[]domain.StoryStatus{domain.StoryStatusPending, domain.StoryStatusInProgress, domain.StoryStatusCompleted}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filter, fields := ParseStoryFilter(tc.statuses, nil)

			require.Empty(t, fields, "los estados admitidos deben aceptarse")
			assert.ElementsMatch(t, tc.want, filter.Statuses)
			assert.Empty(t, filter.Priorities)
		})
	}
}

// Escenario: Filtros combinados (003/US2)
// El parámetro priority también es repetible, y los dos filtros llegan juntos al StoryFilter para
// combinarse con "y" en el listado.
func TestUS2_ValidacionFiltrosCombinados(t *testing.T) {
	t.Run("prioridad Alta y estado Pendiente", func(t *testing.T) {
		filter, fields := ParseStoryFilter([]string{"pending"}, []string{"high"})

		require.Empty(t, fields, "los filtros admitidos deben aceptarse")
		assert.ElementsMatch(t, []domain.StoryStatus{domain.StoryStatusPending}, filter.Statuses)
		assert.ElementsMatch(t, []domain.StoryPriority{domain.StoryPriorityHigh}, filter.Priorities)
	})

	t.Run("prioridad high y low repetidas", func(t *testing.T) {
		filter, fields := ParseStoryFilter(nil, []string{"high", "low"})

		require.Empty(t, fields, "las prioridades admitidas deben aceptarse")
		assert.Empty(t, filter.Statuses)
		assert.ElementsMatch(t, []domain.StoryPriority{domain.StoryPriorityHigh, domain.StoryPriorityLow}, filter.Priorities)
	})

	t.Run("las tres prioridades", func(t *testing.T) {
		filter, fields := ParseStoryFilter(nil, []string{"high", "medium", "low"})

		require.Empty(t, fields, "las prioridades admitidas deben aceptarse")
		assert.ElementsMatch(t, []domain.StoryPriority{
			domain.StoryPriorityHigh, domain.StoryPriorityMedium, domain.StoryPriorityLow,
		}, filter.Priorities)
	})
}

// Escenario: Filtro inválido (003/US2)
// Un estado o una prioridad que no existe se rechaza indicando los valores admitidos de ese filtro,
// también cuando llega junto a valores válidos, con otra grafía o vacío. Si los dos filtros tienen
// valores no admitidos, se informan ambos a la vez.
func TestUS2_ValidacionFiltroInvalido(t *testing.T) {
	statusCases := []struct {
		name     string
		statuses []string
	}{
		{"estado inexistente", []string{"done"}},
		{"estado en español", []string{"pendiente"}},
		{"estado con otra grafía", []string{"Pending"}},
		{"estado vacío", []string{""}},
		{"estado inválido junto a uno válido", []string{"pending", "archived"}},
	}
	for _, tc := range statusCases {
		t.Run(tc.name, func(t *testing.T) {
			_, fields := ParseStoryFilter(tc.statuses, nil)

			validationRequireFilterRejected(t, fields, "status", validationFilterAdmittedStatuses)
		})
	}

	priorityCases := []struct {
		name       string
		priorities []string
	}{
		{"prioridad inexistente", []string{"urgent"}},
		{"prioridad en español", []string{"alta"}},
		{"prioridad con otra grafía", []string{"HIGH"}},
		{"prioridad vacía", []string{""}},
		{"prioridad inválida junto a una válida", []string{"high", "critical"}},
	}
	for _, tc := range priorityCases {
		t.Run(tc.name, func(t *testing.T) {
			_, fields := ParseStoryFilter(nil, tc.priorities)

			validationRequireFilterRejected(t, fields, "priority", validationFilterAdmittedPriorities)
		})
	}

	t.Run("estado y prioridad inválidos a la vez", func(t *testing.T) {
		_, fields := ParseStoryFilter([]string{"done"}, []string{"urgent"})

		names := validationFieldNames(fields)
		assert.Contains(t, names, "status", "debe informarse el filtro de estado")
		assert.Contains(t, names, "priority", "debe informarse el filtro de prioridad")
	})
}

// TestStoryFilter_ValoresRepetidosSinDuplicados verifica que miles de repeticiones de status y de
// priority, mezclando los valores admitidos, producen un filtro con cada valor una sola vez, de modo
// que la cláusula IN del listado nunca reciba más de tres parámetros por filtro (T085, hallazgo de
// seguridad: filtros repetibles sin tope).
func TestStoryFilter_ValoresRepetidosSinDuplicados(t *testing.T) {
	const repetitions = 5000
	statuses := make([]string, 0, repetitions)
	priorities := make([]string, 0, repetitions)
	for i := 0; i < repetitions; i++ {
		statuses = append(statuses, validationFilterAdmittedStatuses[i%3])
		priorities = append(priorities, validationFilterAdmittedPriorities[i%3])
	}

	filter, fields := ParseStoryFilter(statuses, priorities)

	require.Empty(t, fields, "los valores admitidos repetidos deben aceptarse")
	statusCounts := map[domain.StoryStatus]int{}
	for _, s := range filter.Statuses {
		statusCounts[s]++
	}
	priorityCounts := map[domain.StoryPriority]int{}
	for _, p := range filter.Priorities {
		priorityCounts[p]++
	}
	assert.Equal(t, map[domain.StoryStatus]int{
		domain.StoryStatusPending: 1, domain.StoryStatusInProgress: 1, domain.StoryStatusCompleted: 1,
	}, statusCounts, "cada estado admitido debe figurar una sola vez")
	assert.Equal(t, map[domain.StoryPriority]int{
		domain.StoryPriorityHigh: 1, domain.StoryPriorityMedium: 1, domain.StoryPriorityLow: 1,
	}, priorityCounts, "cada prioridad admitida debe figurar una sola vez")
}

// Pruebas unitarias de la validación del cuerpo de la estimación (T048, FR-016, FR-018, FR-019). El
// cuerpo se decodifica desde el JSON literal, igual que en el manejador, para que la prueba
// distinga el campo ausente del null sin depender de cómo se declara el campo en StoryPointsInput.

// validationDecodeEstimate decodifica el cuerpo JSON de la estimación en StoryPointsInput y falla
// la prueba si el JSON no es sintácticamente válido.
func validationDecodeEstimate(t *testing.T, body string) StoryPointsInput {
	t.Helper()
	var req StoryPointsInput
	require.NoError(t, json.Unmarshal([]byte(body), &req), "el cuerpo de prueba debe ser JSON válido")
	return req
}

// Escenario: Estimación inicial (003/US4)
// Cada valor de la escala, incluido el 13 del caso normal, se acepta y se devuelve como el entero
// recibido (FR-016).
func TestUS4_ValidacionEstimacionInicial(t *testing.T) {
	for _, points := range domain.StoryPointsScale {
		t.Run(fmt.Sprintf("%d", points), func(t *testing.T) {
			req := validationDecodeEstimate(t, fmt.Sprintf(`{"storyPoints": %d}`, points))

			got, fields := ValidateStoryPointsInput(req)

			require.Empty(t, fields, "el valor %d de la escala debería aceptarse", points)
			require.NotNil(t, got, "los Story Points aceptados no pueden quedar sin estimar")
			assert.Equal(t, points, *got)
		})
	}
}

// Escenario: Extremos de la escala (003/US4)
// Los extremos 0 y 100 se aceptan, y el 0 queda como estimación y no como "sin estimar" (FR-015).
func TestUS4_ValidacionExtremosDeLaEscala(t *testing.T) {
	for _, points := range []int{0, 100} {
		t.Run(fmt.Sprintf("%d", points), func(t *testing.T) {
			req := validationDecodeEstimate(t, fmt.Sprintf(`{"storyPoints": %d}`, points))

			got, fields := ValidateStoryPointsInput(req)

			require.Empty(t, fields, "el extremo %d debería aceptarse", points)
			require.NotNil(t, got, "el extremo %d no puede confundirse con sin estimar", points)
			assert.Equal(t, points, *got)
		})
	}
}

// Escenario: Volver a sin estimar (003/US4)
// Un storyPoints null se acepta como "sin estimar": no hay campos inválidos y el valor devuelto es
// nil (FR-018).
func TestUS4_ValidacionVolverASinEstimar(t *testing.T) {
	req := validationDecodeEstimate(t, `{"storyPoints": null}`)

	got, fields := ValidateStoryPointsInput(req)

	assert.Empty(t, fields, "null debería aceptarse como sin estimar")
	assert.Nil(t, got, "null debe devolver la historia a sin estimar")
}

// Escenario: Valor fuera de escala (003/US4)
// El campo storyPoints es obligatorio: un cuerpo sin él se rechaza informando el campo, para no
// confundirlo con "sin estimar" (FR-016, research.md sección 8).
func TestUS4_ValidacionStoryPointsObligatorio(t *testing.T) {
	for _, body := range []string{`{}`, `{"points": 13}`} {
		t.Run(body, func(t *testing.T) {
			req := validationDecodeEstimate(t, body)

			got, fields := ValidateStoryPointsInput(req)

			validationRequireOnlyField(t, fields, "storyPoints")
			assert.Nil(t, got, "un cuerpo rechazado no devuelve Story Points")
		})
	}
}

// Escenario: Valor fuera de escala (003/US4)
// Los enteros intermedios o fuera de la escala (50, 4), los negativos, los decimales (6,5 y 5.0),
// la notación exponencial y los valores de otro tipo se rechazan con el mensaje que enumera la
// escala completa (FR-019).
func TestUS4_ValidacionValorFueraDeEscala(t *testing.T) {
	for _, literal := range []string{"50", "4", "6.5", "5.0", "-1", "101", "1e1", `"13"`, "true", "[13]", "{}"} {
		t.Run(literal, func(t *testing.T) {
			req := validationDecodeEstimate(t, `{"storyPoints": `+literal+`}`)

			got, fields := ValidateStoryPointsInput(req)

			validationRequireOnlyField(t, fields, "storyPoints", validationScale)
			assert.Nil(t, got, "un valor rechazado no devuelve Story Points")
		})
	}
}

// Pruebas unitarias de la decodificación y validación del cuerpo de la modificación parcial (T056,
// research.md sección 6, RN-03 a RN-05, RN-08, RN-10). El cuerpo se decodifica desde el JSON
// literal, igual que en el manejador, para distinguir el campo ausente del null.

// validationDecodePatch decodifica el cuerpo JSON del PATCH en StoryPatch y falla la prueba si el
// JSON no es sintácticamente válido.
func validationDecodePatch(t *testing.T, body string) StoryPatch {
	t.Helper()
	var req StoryPatch
	require.NoError(t, json.Unmarshal([]byte(body), &req), "el cuerpo de prueba debe ser JSON válido")
	return req
}

// validationPatchJSON serializa un mapa como cuerpo JSON, para armar textos largos sin escaparlos a
// mano.
func validationPatchJSON(t *testing.T, body map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	return string(raw)
}

// validationRequirePatchRejected exige que el PATCH se rechace con exactamente el campo indicado,
// con un mensaje que mencione las piezas recibidas, y que no devuelva ningún cambio (FR-023).
func validationRequirePatchRejected(t *testing.T, body, field string, parts ...string) {
	t.Helper()
	got, fields := ValidateStoryPatch(validationDecodePatch(t, body))
	validationRequireOnlyField(t, fields, field, parts...)
	assert.Equal(t, domain.StoryChanges{}, got, "un cuerpo rechazado no devuelve cambios")
}

// Escenario: Refinamiento completo (003/US5)
// Título, descripción, prioridad y criterios válidos se aceptan juntos, recortados, y los criterios
// conservan el orden recibido (FR-020, FR-021).
func TestUS5_ValidacionRefinamientoCompleto(t *testing.T) {
	body := `{"title": "  Exportar a PDF el reporte de velocidad del equipo  ",
		"description": "  Incluye los sprints cerrados del último trimestre.  ",
		"priority": "medium",
		"acceptanceCriteria": ["  El PDF incluye la velocidad.  ", "El archivo lleva el nombre del proyecto."]}`

	got, fields := ValidateStoryPatch(validationDecodePatch(t, body))

	require.Empty(t, fields, "el refinamiento completo debería aceptarse")
	require.NotNil(t, got.Title)
	assert.Equal(t, "Exportar a PDF el reporte de velocidad del equipo", *got.Title)
	assert.True(t, got.DescriptionSet, "la descripción enviada debe marcarse como presente")
	require.NotNil(t, got.Description)
	assert.Equal(t, "Incluye los sprints cerrados del último trimestre.", *got.Description)
	require.NotNil(t, got.Priority)
	assert.Equal(t, domain.StoryPriorityMedium, *got.Priority)
	require.NotNil(t, got.AcceptanceCriteria)
	assert.Equal(t, []string{"El PDF incluye la velocidad.", "El archivo lleva el nombre del proyecto."},
		*got.AcceptanceCriteria)
}

// Escenario: Refinamiento completo (003/US5)
// Los límites del título (150) y de la descripción (2000) se aceptan como en la creación (RN-03,
// RN-04).
func TestUS5_ValidacionLimitesAceptados(t *testing.T) {
	title := strings.Repeat("a", 150)
	description := strings.Repeat("d", 2000)
	body := validationPatchJSON(t, map[string]any{"title": title, "description": description})

	got, fields := ValidateStoryPatch(validationDecodePatch(t, body))

	require.Empty(t, fields, "los límites deberían aceptarse")
	require.NotNil(t, got.Title)
	assert.Equal(t, title, *got.Title)
	require.NotNil(t, got.Description)
	assert.Equal(t, description, *got.Description)
}

// Escenario: Refinamiento completo (003/US5)
// La descripción enviada como null, vacía o solo de espacios se borra: queda presente y sin valor
// (FR-020, RN-04).
func TestUS5_ValidacionBorrarDescripcion(t *testing.T) {
	for _, literal := range []string{`null`, `""`, `"   "`} {
		t.Run(literal, func(t *testing.T) {
			got, fields := ValidateStoryPatch(validationDecodePatch(t, `{"description": `+literal+`}`))

			require.Empty(t, fields, "la descripción %s debería aceptarse como borrado", literal)
			assert.True(t, got.DescriptionSet, "la descripción %s debe marcarse como presente", literal)
			assert.Nil(t, got.Description, "la descripción %s debe borrarse", literal)
			assert.Nil(t, got.Title, "el título ausente no cambia")
			assert.Nil(t, got.Priority, "la prioridad ausente no cambia")
			assert.Nil(t, got.AcceptanceCriteria, "los criterios ausentes no cambian")
		})
	}
}

// Escenario: Historia en progreso (003/US5)
// Un cuerpo que solo corrige la descripción deja ausentes los demás campos, que no cambian (FR-020).
func TestUS5_ValidacionHistoriaEnProgreso(t *testing.T) {
	got, fields := ValidateStoryPatch(validationDecodePatch(t, `{"description": "Aclara el alcance."}`))

	require.Empty(t, fields)
	assert.True(t, got.DescriptionSet)
	require.NotNil(t, got.Description)
	assert.Equal(t, "Aclara el alcance.", *got.Description)
	assert.Nil(t, got.Title, "el título ausente no cambia")
	assert.Nil(t, got.Priority, "la prioridad ausente no cambia")
	assert.Nil(t, got.AcceptanceCriteria, "los criterios ausentes no cambian")
}

// Escenario: Edición simultánea de campos distintos (003/US5)
// Un cuerpo con solo la prioridad lleva únicamente ese cambio: la descripción y el resto quedan
// ausentes, que es lo que permite que dos ediciones de campos distintos no se pisen (FR-020, FR-025).
func TestUS5_ValidacionSoloLosCamposEnviados(t *testing.T) {
	got, fields := ValidateStoryPatch(validationDecodePatch(t, `{"priority": "low"}`))

	require.Empty(t, fields)
	require.NotNil(t, got.Priority)
	assert.Equal(t, domain.StoryPriorityLow, *got.Priority)
	assert.Nil(t, got.Title, "el título ausente no cambia")
	assert.False(t, got.DescriptionSet, "la descripción ausente no cambia")
	assert.Nil(t, got.Description)
	assert.Nil(t, got.AcceptanceCriteria, "los criterios ausentes no cambian")
}

// Escenario: Reorden de criterios (003/US5)
// La lista enviada reemplaza a la anterior en el orden recibido (FR-022).
func TestUS5_ValidacionReordenDeCriterios(t *testing.T) {
	got, fields := ValidateStoryPatch(validationDecodePatch(t,
		`{"acceptanceCriteria": ["Tercero", "Primero", "Segundo"]}`))

	require.Empty(t, fields)
	require.NotNil(t, got.AcceptanceCriteria)
	assert.Equal(t, []string{"Tercero", "Primero", "Segundo"}, *got.AcceptanceCriteria)
}

// Escenario: Vaciar criterios (003/US5)
// Una lista vacía queda presente y vacía, distinta de ausente (FR-020, FR-022).
func TestUS5_ValidacionVaciarCriterios(t *testing.T) {
	got, fields := ValidateStoryPatch(validationDecodePatch(t, `{"acceptanceCriteria": []}`))

	require.Empty(t, fields, "la lista vacía debería aceptarse")
	require.NotNil(t, got.AcceptanceCriteria, "la lista vacía debe quedar presente")
	assert.Empty(t, *got.AcceptanceCriteria, "la lista vacía deja la historia sin criterios")
}

// Escenario: Entrada inválida sin cambio parcial (003/US5)
// El título presente se valida como en la creación, y null, vacío o solo espacios se rechazan
// (RN-03, research.md sección 6).
func TestUS5_ValidacionTituloInvalido(t *testing.T) {
	t.Run("vacíos y null", func(t *testing.T) {
		for _, literal := range []string{`null`, `""`, `"   "`} {
			t.Run(literal, func(t *testing.T) {
				validationRequirePatchRejected(t, `{"title": `+literal+`}`, "title", "obligatorio")
			})
		}
	})
	t.Run("151 caracteres", func(t *testing.T) {
		body := validationPatchJSON(t, map[string]any{"title": strings.Repeat("a", 151)})
		validationRequirePatchRejected(t, body, "title", "150")
	})
	t.Run("tipo no texto", func(t *testing.T) {
		validationRequirePatchRejected(t, `{"title": 5}`, "title")
	})
}

// Escenario: Entrada inválida sin cambio parcial (003/US5)
// Una descripción de más de 2000 caracteres o que no es texto se rechaza (RN-04).
func TestUS5_ValidacionDescripcionInvalida(t *testing.T) {
	t.Run("2001 caracteres", func(t *testing.T) {
		body := validationPatchJSON(t, map[string]any{"description": strings.Repeat("d", 2001)})
		validationRequirePatchRejected(t, body, "description", "2000")
	})
	t.Run("tipo no texto", func(t *testing.T) {
		validationRequirePatchRejected(t, `{"description": 5}`, "description")
	})
}

// Escenario: Entrada inválida sin cambio parcial (003/US5)
// La prioridad presente se valida como en la creación, y null o vacía se rechaza (RN-05).
func TestUS5_ValidacionPrioridadInvalida(t *testing.T) {
	for _, literal := range []string{`null`, `""`, `"urgent"`, `"High"`, `"Alta"`, `1`} {
		t.Run(literal, func(t *testing.T) {
			validationRequirePatchRejected(t, `{"priority": `+literal+`}`, "priority", "Alta", "Media", "Baja")
		})
	}
}

// Escenario: Entrada inválida sin cambio parcial (003/US5)
// Los criterios en null se rechazan (para vaciar se envía []), y cada criterio vacío o de más de
// 500 caracteres se informa con su índice (RN-08, research.md sección 6).
func TestUS5_ValidacionCriteriosInvalidos(t *testing.T) {
	t.Run("null", func(t *testing.T) {
		validationRequirePatchRejected(t, `{"acceptanceCriteria": null}`, "acceptanceCriteria")
	})
	t.Run("criterio vacío", func(t *testing.T) {
		validationRequirePatchRejected(t, `{"acceptanceCriteria": ["Válido", "   "]}`,
			"acceptanceCriteria[1]", "contenido")
	})
	t.Run("criterio de 501", func(t *testing.T) {
		body := validationPatchJSON(t, map[string]any{
			"acceptanceCriteria": []string{strings.Repeat("c", 501), "Válido"},
		})
		validationRequirePatchRejected(t, body, "acceptanceCriteria[0]", "500")
	})
	t.Run("tipo no lista", func(t *testing.T) {
		validationRequirePatchRejected(t, `{"acceptanceCriteria": "Un criterio"}`, "acceptanceCriteria")
	})
}

// Escenario: Entrada inválida sin cambio parcial (003/US5)
// Un campo inválido junto a otros válidos rechaza el cuerpo entero: se informa solo el inválido y
// no se devuelve ninguno de los cambios válidos (FR-023).
func TestUS5_ValidacionEntradaInvalidaSinCambioParcial(t *testing.T) {
	long := func(n int) string { return strings.Repeat("x", n) }
	cases := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"título vacío", map[string]any{"title": "", "priority": "low"}, "title"},
		{"título de 151", map[string]any{"title": long(151), "description": "Nueva"}, "title"},
		{"descripción de 2001", map[string]any{"description": long(2001), "title": "Nuevo"}, "description"},
		{"prioridad inexistente", map[string]any{"priority": "urgent", "title": "Nuevo"}, "priority"},
		{"criterio de 501", map[string]any{"acceptanceCriteria": []string{long(501)}, "priority": "low"},
			"acceptanceCriteria[0]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validationRequirePatchRejected(t, validationPatchJSON(t, tc.body), tc.field)
		})
	}

	// Todos los campos inválidos se informan a la vez (FR-013).
	t.Run("todos los campos inválidos", func(t *testing.T) {
		body := validationPatchJSON(t, map[string]any{
			"title": "", "description": long(2001), "priority": "urgent",
			"acceptanceCriteria": []string{"", long(501)},
		})
		got, fields := ValidateStoryPatch(validationDecodePatch(t, body))
		assert.ElementsMatch(t,
			[]string{"title", "description", "priority", "acceptanceCriteria[0]", "acceptanceCriteria[1]"},
			validationFieldNames(fields))
		assert.Equal(t, domain.StoryChanges{}, got)
	})
}

// Escenario: Entrada inválida sin cambio parcial (003/US5)
// Un cuerpo sin ningún campo editable se rechaza con field body, para que una petición vacía no
// actualice la historia (research.md sección 6).
func TestUS5_ValidacionCuerpoSinCampos(t *testing.T) {
	for _, body := range []string{`{}`, `{"id": "7c4e2a19-3b8d-4f60-a1c5-9e2d0b6f8a73"}`, `{"createdBy": "x"}`} {
		t.Run(body, func(t *testing.T) {
			validationRequirePatchRejected(t, body, "body", "ningún campo a modificar")
		})
	}
}

// Escenario: Cambio de estado no disponible (003/US5)
// El campo status se rechaza con su campo aunque llegue junto a cambios válidos, que tampoco se
// devuelven: el backlog nunca cambia el estado (FR-027, RN-10).
func TestUS5_ValidacionCambioDeEstadoNoDisponible(t *testing.T) {
	for _, status := range []string{`"in_progress"`, `"completed"`, `"pending"`, `null`} {
		t.Run(status, func(t *testing.T) {
			validationRequirePatchRejected(t, `{"priority": "high", "status": `+status+`}`,
				"status", "no se cambia desde el backlog")
		})
	}
	t.Run("solo status", func(t *testing.T) {
		got, fields := ValidateStoryPatch(validationDecodePatch(t, `{"status": "completed"}`))
		assert.Contains(t, validationFieldNames(fields), "status")
		assert.Equal(t, domain.StoryChanges{}, got)
	})
}

// Escenario: Entrada inválida sin cambio parcial (003/US5)
// El campo storyPoints se rechaza con su campo aunque llegue junto a cambios válidos, porque la
// estimación tiene su propio endpoint (research.md secciones 6 y 8).
func TestUS5_ValidacionStoryPointsNoDisponible(t *testing.T) {
	for _, points := range []string{`5`, `null`, `0`} {
		t.Run(points, func(t *testing.T) {
			validationRequirePatchRejected(t, `{"title": "Nuevo", "storyPoints": `+points+`}`, "storyPoints")
		})
	}
	t.Run("solo storyPoints", func(t *testing.T) {
		got, fields := ValidateStoryPatch(validationDecodePatch(t, `{"storyPoints": 8}`))
		assert.Contains(t, validationFieldNames(fields), "storyPoints")
		assert.Equal(t, domain.StoryChanges{}, got)
	})
}
