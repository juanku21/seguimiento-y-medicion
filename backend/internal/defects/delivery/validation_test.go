package delivery

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// Pruebas unitarias obligatorias de la validación del alta de un defecto (T010; RN-03, RN-04, RN-06,
// FR-016). Los escenarios de spec.md no tienen nombre, solo número y tipo; cada comentario cita el
// texto del escenario que corresponde (007/US1) o la regla cuando no hay escenario propio.

// Mensajes que la validación debe devolver, tal como los fija tasks.md (T010).
const (
	validationMsgDescriptionRequired = "La descripción es obligatoria."
	validationMsgDescriptionTooLong  = "La descripción admite como máximo 2000 caracteres."
	validationMsgSeverityInvalid     = "La severidad debe ser Crítica, Alta, Media o Baja."
	validationMsgStoryRequired       = "La historia relacionada es obligatoria."
	validationMsgSprintRequired      = "El sprint de detección es obligatorio."
	validationMsgIDInvalid           = "no es un identificador válido."
)

// validationInput arma un alta válida completa sobre la que cada prueba modifica un solo campo.
func validationInput() DefectInput {
	return DefectInput{
		Description:      "El total del reporte no suma los descuentos",
		Severity:         "high",
		StoryID:          uuid.NewString(),
		DetectedSprintID: uuid.NewString(),
	}
}

// validationFieldNames extrae los nombres de los campos rechazados, ordenados.
func validationFieldNames(fields []authdelivery.FieldError) []string {
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, f.Field)
	}
	sort.Strings(names)
	return names
}

// validationRequireOnlyField exige que se rechace exactamente un campo, con el mensaje indicado.
func validationRequireOnlyField(t *testing.T, fields []authdelivery.FieldError, field, message string) {
	t.Helper()
	if len(fields) != 1 {
		t.Fatalf("se esperaba un único campo inválido (%s), se obtuvieron %v", field, fields)
	}
	if fields[0].Field != field {
		t.Errorf("el campo rechazado debería ser %q, fue %q", field, fields[0].Field)
	}
	if !strings.Contains(fields[0].Message, message) {
		t.Errorf("el mensaje %q debería contener %q", fields[0].Message, message)
	}
}

// Un alta completa y válida se acepta y se normaliza a los tipos del dominio (RN-03, RN-04).
func TestValidateDefectInput_AltaValida(t *testing.T) {
	req := validationInput()

	got, fields := ValidateDefectInput(req)

	if len(fields) != 0 {
		t.Fatalf("no se esperaban campos inválidos, se obtuvieron %v", fields)
	}
	if got.Description != req.Description {
		t.Errorf("descripción = %q, se esperaba %q", got.Description, req.Description)
	}
	if got.Severity != domain.DefectSeverityHigh {
		t.Errorf("severidad = %q, se esperaba %q", got.Severity, domain.DefectSeverityHigh)
	}
	if got.StoryID.String() != req.StoryID || got.DetectedSprintID.String() != req.DetectedSprintID {
		t.Errorf("los identificadores no se convirtieron: %+v", got)
	}
}

// Escenario: Dado un integrante, cuando intenta registrar un defecto sin descripción o con una
// descripción compuesta solo por espacios, entonces el registro se rechaza indicando que la
// descripción es obligatoria (007/US1).
func TestUS1_DescripcionObligatoria(t *testing.T) {
	cases := []struct {
		name  string
		build func() DefectInput
	}{
		{"ausente", func() DefectInput { r := validationInput(); r.Description = ""; return r }},
		{"vacía", func() DefectInput {
			var r DefectInput
			if err := json.Unmarshal([]byte(`{"description":"","severity":"high"}`), &r); err != nil {
				t.Fatal(err)
			}
			r.StoryID, r.DetectedSprintID = uuid.NewString(), uuid.NewString()
			return r
		}},
		{"solo espacios", func() DefectInput { r := validationInput(); r.Description = "   "; return r }},
		{"solo tabulaciones y saltos", func() DefectInput { r := validationInput(); r.Description = " \t\n "; return r }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, fields := ValidateDefectInput(tc.build())
			validationRequireOnlyField(t, fields, "description", validationMsgDescriptionRequired)
		})
	}
}

// Escenario: Dado un integrante, cuando registra un defecto con una descripción de exactamente 2000
// caracteres, entonces el defecto se crea; y cuando tiene 2001, se rechaza (007/US1).
func TestUS1_LimiteDescripcion(t *testing.T) {
	t.Run("2000 runas con acentos se aceptan y se recortan", func(t *testing.T) {
		// Runas multibyte: contar bytes en lugar de runas rechazaría esta descripción (RN-06).
		body := strings.Repeat("á", 2000)
		req := validationInput()
		req.Description = "  " + body + "  "

		got, fields := ValidateDefectInput(req)

		if len(fields) != 0 {
			t.Fatalf("2000 runas deberían aceptarse, se obtuvieron %v", fields)
		}
		if got.Description != body {
			t.Errorf("la descripción debería devolverse recortada (%d runas)", len([]rune(got.Description)))
		}
	})

	t.Run("2001 runas se rechazan", func(t *testing.T) {
		req := validationInput()
		req.Description = strings.Repeat("ñ", 2001)

		_, fields := ValidateDefectInput(req)

		validationRequireOnlyField(t, fields, "description", validationMsgDescriptionTooLong)
	})

	t.Run("los espacios extremos no cuentan para el límite", func(t *testing.T) {
		req := validationInput()
		req.Description = " " + strings.Repeat("é", 2000) + " "

		_, fields := ValidateDefectInput(req)

		if len(fields) != 0 {
			t.Fatalf("los espacios extremos no deberían contar, se obtuvieron %v", fields)
		}
	})
}

// Escenario: Dado un integrante, cuando registra un defecto con cada una de las cuatro severidades
// (Crítica, Alta, Media y Baja), entonces las cuatro se aceptan (007/US1).
func TestUS1_CuatroSeveridades(t *testing.T) {
	cases := map[string]domain.DefectSeverity{
		"critical": domain.DefectSeverityCritical,
		"high":     domain.DefectSeverityHigh,
		"medium":   domain.DefectSeverityMedium,
		"low":      domain.DefectSeverityLow,
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			req := validationInput()
			req.Severity = raw

			got, fields := ValidateDefectInput(req)

			if len(fields) != 0 {
				t.Fatalf("la severidad %q debería aceptarse, se obtuvieron %v", raw, fields)
			}
			if got.Severity != want {
				t.Errorf("severidad = %q, se esperaba %q", got.Severity, want)
			}
		})
	}
}

// Escenario: Dado un integrante, cuando intenta registrar un defecto con una severidad que no es
// ninguna de las cuatro admitidas, entonces el registro se rechaza indicando las severidades
// válidas (007/US1).
func TestUS1_SeveridadInvalida(t *testing.T) {
	for _, raw := range []string{"", "urgent", "Critical"} {
		t.Run("severidad "+raw, func(t *testing.T) {
			req := validationInput()
			req.Severity = raw

			_, fields := ValidateDefectInput(req)

			validationRequireOnlyField(t, fields, "severity", validationMsgSeverityInvalid)
		})
	}
}

// Escenario: Dado un integrante, cuando intenta registrar un defecto sin historia relacionada o sin
// sprint de detección, entonces el registro se rechaza indicando el campo faltante (007/US1).
func TestUS1_HistoriaYSprintObligatorios(t *testing.T) {
	t.Run("sin historia", func(t *testing.T) {
		req := validationInput()
		req.StoryID = ""

		_, fields := ValidateDefectInput(req)

		validationRequireOnlyField(t, fields, "storyId", validationMsgStoryRequired)
	})

	t.Run("sin sprint de detección", func(t *testing.T) {
		req := validationInput()
		req.DetectedSprintID = ""

		_, fields := ValidateDefectInput(req)

		validationRequireOnlyField(t, fields, "detectedSprintId", validationMsgSprintRequired)
	})
}

// Identificadores mal formados (FR-016, Principio VII): se rechazan con un mensaje por campo y no
// llegan a la capa de servicio.
func TestValidateDefectInput_IdentificadoresMalFormados(t *testing.T) {
	t.Run("storyId mal formado", func(t *testing.T) {
		req := validationInput()
		req.StoryID = "no-es-un-uuid"

		_, fields := ValidateDefectInput(req)

		validationRequireOnlyField(t, fields, "storyId", validationMsgIDInvalid)
	})

	t.Run("detectedSprintId mal formado", func(t *testing.T) {
		req := validationInput()
		req.DetectedSprintID = "123"

		_, fields := ValidateDefectInput(req)

		validationRequireOnlyField(t, fields, "detectedSprintId", validationMsgIDInvalid)
	})
}

// Todos los campos inválidos se devuelven a la vez, sin detenerse en el primero (Principio VII).
func TestValidateDefectInput_VariosCamposInvalidos(t *testing.T) {
	req := DefectInput{Description: "   ", Severity: "urgent", StoryID: "x", DetectedSprintID: ""}

	got, fields := ValidateDefectInput(req)

	want := []string{"description", "detectedSprintId", "severity", "storyId"}
	names := validationFieldNames(fields)
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("campos rechazados = %v, se esperaba %v", names, want)
	}
	if got != (domain.DefectInput{}) {
		t.Errorf("con campos inválidos no se debe devolver ningún dato, se obtuvo %+v", got)
	}
}

// Un createdBy o createdById en el cuerpo se ignora: el DTO no tiene campo de autor y el alta se
// acepta igual, atribuida a la sesión (FR-056).
func TestValidateDefectInput_IgnoraAutoria(t *testing.T) {
	body := `{"description":"Falla el total","severity":"low","storyId":"` + uuid.NewString() +
		`","detectedSprintId":"` + uuid.NewString() +
		`","createdBy":"alguien","createdById":"` + uuid.NewString() + `"}`
	var req DefectInput
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("el cuerpo con campos de autoría debería decodificarse: %v", err)
	}

	got, fields := ValidateDefectInput(req)

	if len(fields) != 0 {
		t.Fatalf("los campos de autoría deberían ignorarse, se obtuvieron %v", fields)
	}
	if got.Description != "Falla el total" || got.Severity != domain.DefectSeverityLow {
		t.Errorf("el alta debería normalizarse sin cambios, se obtuvo %+v", got)
	}
}

// Pruebas unitarias obligatorias de la validación del cambio de estado (T026; FR-023, RN-11). La
// regla de transiciones y la del sprint de resolución contra el de detección son del dominio
// (status_test.go); aquí solo se valida la forma del cuerpo antes de llegar al servicio.

// Mensajes del cambio de estado, tal como los fija tasks.md (T026) y data-model.md sección 8.
const (
	validationMsgStatusInvalid            = "El estado debe ser Abierto, En progreso, Resuelto o Descartado."
	validationMsgResolutionSprintRequired = "El sprint de resolución es obligatorio para resolver."
	validationMsgResolutionSprintOnlyOn   = "El sprint de resolución solo se indica al resolver."
)

// Los destinos sin sprint de resolución se aceptan y se normalizan al estado del dominio, sin sprint.
func TestValidateStatusChange_DestinosSinSprint(t *testing.T) {
	cases := map[string]domain.DefectStatus{
		"open":        domain.DefectStatusOpen,
		"in_progress": domain.DefectStatusInProgress,
		"discarded":   domain.DefectStatusDiscarded,
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			got, fields := ValidateStatusChange(DefectStatusChange{Status: raw})

			if len(fields) != 0 {
				t.Fatalf("el destino %q sin sprint debería aceptarse, se obtuvieron %v", raw, fields)
			}
			if got.To != want {
				t.Errorf("destino = %q, se esperaba %q", got.To, want)
			}
			if got.ResolvedSprintID != nil {
				t.Errorf("no se esperaba sprint de resolución, se obtuvo %v", *got.ResolvedSprintID)
			}
		})
	}
}

// Resolver con un sprint bien formado se acepta y devuelve ese sprint convertido (FR-023).
func TestValidateStatusChange_ResolverConSprint(t *testing.T) {
	sprintID := uuid.New()

	got, fields := ValidateStatusChange(DefectStatusChange{Status: "resolved", ResolvedSprintID: sprintID.String()})

	if len(fields) != 0 {
		t.Fatalf("resolver con sprint debería aceptarse, se obtuvieron %v", fields)
	}
	if got.To != domain.DefectStatusResolved {
		t.Errorf("destino = %q, se esperaba %q", got.To, domain.DefectStatusResolved)
	}
	if got.ResolvedSprintID == nil || *got.ResolvedSprintID != sprintID {
		t.Errorf("sprint de resolución = %v, se esperaba %v", got.ResolvedSprintID, sprintID)
	}
}

// Un estado ausente o fuera de los cuatro admitidos se rechaza en el campo status (RN-11). closed es
// un estado de sprint, no de defecto.
func TestValidateStatusChange_EstadoInvalido(t *testing.T) {
	for _, raw := range []string{"", "closed", "Resolved"} {
		t.Run("estado "+raw, func(t *testing.T) {
			got, fields := ValidateStatusChange(DefectStatusChange{Status: raw})

			validationRequireOnlyField(t, fields, "status", validationMsgStatusInvalid)
			if got != (domain.StatusChange{}) {
				t.Errorf("con campos inválidos no se debe devolver ningún dato, se obtuvo %+v", got)
			}
		})
	}
}

// Resolver sin sprint de resolución se rechaza en resolvedSprintId (FR-023). Corresponde al
// escenario 7 de la historia 2 (007/US2), cuya prueba de punta a punta es TestUS2_ResolucionSinSprint.
func TestValidateStatusChange_ResolverSinSprint(t *testing.T) {
	got, fields := ValidateStatusChange(DefectStatusChange{Status: "resolved"})

	validationRequireOnlyField(t, fields, "resolvedSprintId", validationMsgResolutionSprintRequired)
	if got != (domain.StatusChange{}) {
		t.Errorf("con campos inválidos no se debe devolver ningún dato, se obtuvo %+v", got)
	}
}

// Un sprint de resolución en cualquier destino que no sea resolved se rechaza en resolvedSprintId
// (FR-023), aunque el identificador esté bien formado.
func TestValidateStatusChange_SprintFueraDeResolucion(t *testing.T) {
	for _, raw := range []string{"in_progress", "open", "discarded"} {
		t.Run(raw, func(t *testing.T) {
			got, fields := ValidateStatusChange(DefectStatusChange{Status: raw, ResolvedSprintID: uuid.NewString()})

			validationRequireOnlyField(t, fields, "resolvedSprintId", validationMsgResolutionSprintOnlyOn)
			if got != (domain.StatusChange{}) {
				t.Errorf("con campos inválidos no se debe devolver ningún dato, se obtuvo %+v", got)
			}
		})
	}
}

// Un sprint de resolución mal formado se rechaza en resolvedSprintId y no llega al servicio
// (Principio VII).
func TestValidateStatusChange_SprintMalFormado(t *testing.T) {
	for _, raw := range []string{"no-es-un-uuid", "123"} {
		t.Run(raw, func(t *testing.T) {
			got, fields := ValidateStatusChange(DefectStatusChange{Status: "resolved", ResolvedSprintID: raw})

			validationRequireOnlyField(t, fields, "resolvedSprintId", validationMsgIDInvalid)
			if got != (domain.StatusChange{}) {
				t.Errorf("con campos inválidos no se debe devolver ningún dato, se obtuvo %+v", got)
			}
		})
	}
}

// Pruebas unitarias obligatorias de la validación de los filtros del listado (T038; FR-046, FR-047).
// Corresponden a los escenarios de filtrado de la historia 3 (007/US3).

// Mensaje de un filtro de historia o sprint mal formado, tal como lo fija data-model.md.
const validationMsgFilterOutsideProject = "El filtro no corresponde al proyecto."

// Escenario: Dado un integrante, cuando consulta los defectos sin ningún filtro, entonces el filtro
// resultante no acota nada (007/US3).
func TestUS3_FiltroVacio(t *testing.T) {
	got, fields := ParseDefectFilter(nil, nil, "", "")

	if len(fields) != 0 {
		t.Fatalf("sin parámetros no se esperaban errores, se obtuvieron %v", fields)
	}
	if len(got.Statuses) != 0 || len(got.Severities) != 0 || got.StoryID != nil || got.SprintID != nil {
		t.Errorf("el filtro debería estar vacío, se obtuvo %+v", got)
	}
}

// Escenario: Dado un integrante, cuando filtra por varios estados y severidades repetidos, entonces
// el filtro conserva las listas en el orden recibido y convierte historia y sprint (007/US3).
func TestUS3_FiltroValido(t *testing.T) {
	storyID, sprintID := uuid.New(), uuid.New()

	got, fields := ParseDefectFilter(
		[]string{"in_progress", "open"},
		[]string{"critical", "low", "high"},
		storyID.String(), sprintID.String(),
	)

	if len(fields) != 0 {
		t.Fatalf("no se esperaban errores, se obtuvieron %v", fields)
	}
	wantStatuses := []domain.DefectStatus{domain.DefectStatusInProgress, domain.DefectStatusOpen}
	if len(got.Statuses) != 2 || got.Statuses[0] != wantStatuses[0] || got.Statuses[1] != wantStatuses[1] {
		t.Errorf("estados = %v, se esperaba %v", got.Statuses, wantStatuses)
	}
	wantSeverities := []domain.DefectSeverity{domain.DefectSeverityCritical, domain.DefectSeverityLow, domain.DefectSeverityHigh}
	if len(got.Severities) != 3 || got.Severities[0] != wantSeverities[0] ||
		got.Severities[1] != wantSeverities[1] || got.Severities[2] != wantSeverities[2] {
		t.Errorf("severidades = %v, se esperaba %v", got.Severities, wantSeverities)
	}
	if got.StoryID == nil || *got.StoryID != storyID {
		t.Errorf("storyId = %v, se esperaba %v", got.StoryID, storyID)
	}
	if got.SprintID == nil || *got.SprintID != sprintID {
		t.Errorf("sprintId = %v, se esperaba %v", got.SprintID, sprintID)
	}
}

// Escenario: Dado un integrante, cuando filtra por un estado o una severidad inexistentes, entonces
// se rechaza indicando los valores admitidos (007/US3).
func TestUS3_ValorDeFiltroInvalido(t *testing.T) {
	t.Run("estado inexistente", func(t *testing.T) {
		_, fields := ParseDefectFilter([]string{"open", "closed"}, nil, "", "")

		validationRequireOnlyField(t, fields, "status", validationMsgStatusInvalid)
	})

	t.Run("severidad inexistente", func(t *testing.T) {
		_, fields := ParseDefectFilter(nil, []string{"urgent"}, "", "")

		validationRequireOnlyField(t, fields, "severity", validationMsgSeverityInvalid)
	})
}

// Escenario: Dado un integrante, cuando filtra por una historia o un sprint con identificador mal
// formado, entonces se rechaza con el mensaje de filtro ajeno al proyecto (007/US3).
func TestUS3_FiltroMalFormado(t *testing.T) {
	t.Run("storyId mal formado", func(t *testing.T) {
		_, fields := ParseDefectFilter(nil, nil, "no-es-un-uuid", "")

		validationRequireOnlyField(t, fields, "storyId", validationMsgFilterOutsideProject)
	})

	t.Run("sprintId mal formado", func(t *testing.T) {
		_, fields := ParseDefectFilter(nil, nil, "", "123")

		validationRequireOnlyField(t, fields, "sprintId", validationMsgFilterOutsideProject)
	})
}

// Todos los filtros inválidos se informan a la vez, sin detenerse en el primero (Principio VII).
func TestUS3_VariosFiltrosInvalidos(t *testing.T) {
	got, fields := ParseDefectFilter([]string{"closed"}, []string{"urgent"}, "x", "y")

	want := []string{"severity", "sprintId", "status", "storyId"}
	names := validationFieldNames(fields)
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("campos rechazados = %v, se esperaba %v", names, want)
	}
	if len(got.Statuses) != 0 || len(got.Severities) != 0 || got.StoryID != nil || got.SprintID != nil {
		t.Errorf("con campos inválidos no se debe devolver ningún dato, se obtuvo %+v", got)
	}
}

// Pruebas unitarias obligatorias de la validación de la modificación parcial (T047; FR-034, FR-035,
// FR-037). La ventana de corrección de los vínculos y el defecto Descartado son reglas del dominio
// (rules_test.go); aquí solo se valida la forma del cuerpo antes de llegar al servicio.

// Mensajes de la modificación, tal como los fijan tasks.md (T047) y research.md sección 6.
const (
	validationMsgPatchWithoutFields = "Indicá al menos un dato para modificar."
	validationMsgStatusNotEditable  = "El estado y el sprint de resolución se cambian con las acciones de estado del defecto."
	validationMsgOwnerNotEditable   = "El proyecto y el autor de un defecto no se modifican."
)

// validationDecodeDefectPatch decodifica el cuerpo JSON en DefectPatch y falla la prueba si el JSON
// no es sintácticamente válido.
func validationDecodeDefectPatch(t *testing.T, body string) DefectPatch {
	t.Helper()
	var req DefectPatch
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("el cuerpo de prueba debe ser JSON válido: %v", err)
	}
	return req
}

// validationPatchBody serializa un mapa como cuerpo JSON, para armar textos largos sin escaparlos a
// mano.
func validationPatchBody(t *testing.T, body map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// validationRequireFieldPresent exige que entre los rechazos haya uno en el campo indicado, con el
// mensaje indicado.
func validationRequireFieldPresent(t *testing.T, fields []authdelivery.FieldError, field, message string) {
	t.Helper()
	for _, f := range fields {
		if f.Field == field {
			if !strings.Contains(f.Message, message) {
				t.Errorf("el mensaje de %q es %q, debería contener %q", field, f.Message, message)
			}
			return
		}
	}
	t.Errorf("se esperaba un rechazo en %q, se obtuvieron %v", field, fields)
}

// Un cuerpo sin ningún campo se rechaza: la modificación debe indicar al menos un dato (FR-033).
func TestValidateDefectPatch_CuerpoVacio(t *testing.T) {
	got, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, `{}`))

	if len(fields) != 1 {
		t.Fatalf("se esperaba un único rechazo, se obtuvieron %v", fields)
	}
	if fields[0].Message != validationMsgPatchWithoutFields {
		t.Errorf("mensaje = %q, se esperaba %q", fields[0].Message, validationMsgPatchWithoutFields)
	}
	if got != (domain.DefectPatch{}) {
		t.Errorf("un cuerpo vacío no debe devolver cambios, se obtuvo %+v", got)
	}
}

// Escenario: Dado un defecto En progreso, cuando un integrante le cambia la severidad, entonces el
// cambio se acepta y el defecto sigue En progreso (007/US4). Aquí, la parte de forma: un cuerpo con
// solo la severidad se acepta y deja los demás campos sin cambio.
func TestUS4_ValidacionSoloSeveridad(t *testing.T) {
	got, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, `{"severity":"critical"}`))

	if len(fields) != 0 {
		t.Fatalf("solo la severidad debería aceptarse, se obtuvieron %v", fields)
	}
	if got.Severity == nil || *got.Severity != domain.DefectSeverityCritical {
		t.Errorf("severidad = %v, se esperaba %q", got.Severity, domain.DefectSeverityCritical)
	}
	if got.Description != nil || got.StoryID != nil || got.DetectedSprintID != nil {
		t.Errorf("los campos ausentes deben quedar nil, se obtuvo %+v", got)
	}
}

// Escenario: Dado un defecto Abierto, cuando un integrante le corrige la historia relacionada y el
// sprint de detección en la misma operación, entonces ambos cambios se aceptan (007/US4). Aquí, la
// parte de forma: los cuatro campos editables se aceptan juntos y se normalizan al dominio.
func TestUS4_ValidacionTodosLosCampos(t *testing.T) {
	storyID, sprintID := uuid.New(), uuid.New()
	body := validationPatchBody(t, map[string]any{
		"description":      "  El total no suma los descuentos  ",
		"severity":         "low",
		"storyId":          storyID.String(),
		"detectedSprintId": sprintID.String(),
	})

	got, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, body))

	if len(fields) != 0 {
		t.Fatalf("no se esperaban rechazos, se obtuvieron %v", fields)
	}
	if got.Description == nil || *got.Description != "El total no suma los descuentos" {
		t.Errorf("la descripción debería devolverse recortada, se obtuvo %v", got.Description)
	}
	if got.Severity == nil || *got.Severity != domain.DefectSeverityLow {
		t.Errorf("severidad = %v, se esperaba %q", got.Severity, domain.DefectSeverityLow)
	}
	if got.StoryID == nil || *got.StoryID != storyID {
		t.Errorf("storyId = %v, se esperaba %v", got.StoryID, storyID)
	}
	if got.DetectedSprintID == nil || *got.DetectedSprintID != sprintID {
		t.Errorf("detectedSprintId = %v, se esperaba %v", got.DetectedSprintID, sprintID)
	}
}

// Escenario: Dado un defecto, cuando un integrante lleva su descripción a exactamente 2000
// caracteres, entonces el cambio se acepta (007/US4). Con 2001 se rechaza, como en el alta (FR-034).
func TestUS4_ValidacionLimiteDescripcion(t *testing.T) {
	t.Run("2000 runas con acentos se aceptan", func(t *testing.T) {
		text := strings.Repeat("á", 2000)
		body := validationPatchBody(t, map[string]any{"description": " " + text + " "})

		got, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, body))

		if len(fields) != 0 {
			t.Fatalf("2000 runas deberían aceptarse, se obtuvieron %v", fields)
		}
		if got.Description == nil || *got.Description != text {
			t.Errorf("la descripción debería devolverse recortada con 2000 runas")
		}
	})

	t.Run("2001 runas se rechazan", func(t *testing.T) {
		body := validationPatchBody(t, map[string]any{"description": strings.Repeat("ñ", 2001)})

		_, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, body))

		validationRequireOnlyField(t, fields, "description", validationMsgDescriptionTooLong)
	})
}

// Escenario: Dado un defecto, cuando un integrante intenta dejar la descripción vacía o poner una
// severidad inválida, entonces el cambio se rechaza con el motivo y el defecto queda exactamente como
// estaba (007/US4). Las reglas son las del alta (FR-034); null cuenta como vacío.
func TestUS4_ValidacionDatosInvalidos(t *testing.T) {
	cases := []struct {
		name, body, field, message string
	}{
		{"descripción vacía", `{"description":""}`, "description", validationMsgDescriptionRequired},
		{"descripción solo espacios", `{"description":" \t\n "}`, "description", validationMsgDescriptionRequired},
		{"descripción null", `{"description":null}`, "description", validationMsgDescriptionRequired},
		{"severidad inexistente", `{"severity":"urgent"}`, "severity", validationMsgSeverityInvalid},
		{"severidad con mayúscula", `{"severity":"Critical"}`, "severity", validationMsgSeverityInvalid},
		{"severidad vacía", `{"severity":""}`, "severity", validationMsgSeverityInvalid},
		{"severidad null", `{"severity":null}`, "severity", validationMsgSeverityInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, tc.body))

			validationRequireOnlyField(t, fields, tc.field, tc.message)
			if got != (domain.DefectPatch{}) {
				t.Errorf("con campos inválidos no se debe devolver ningún cambio, se obtuvo %+v", got)
			}
		})
	}
}

// Los identificadores de los vínculos siguen las reglas del alta (FR-034, FR-016): vacíos o null
// informan que faltan, y mal formados, que no son un identificador válido.
func TestValidateDefectPatch_Identificadores(t *testing.T) {
	cases := []struct {
		name, body, field, message string
	}{
		{"storyId vacío", `{"storyId":""}`, "storyId", validationMsgStoryRequired},
		{"storyId null", `{"storyId":null}`, "storyId", validationMsgStoryRequired},
		{"storyId mal formado", `{"storyId":"no-es-un-uuid"}`, "storyId", validationMsgIDInvalid},
		{"detectedSprintId vacío", `{"detectedSprintId":""}`, "detectedSprintId", validationMsgSprintRequired},
		{"detectedSprintId null", `{"detectedSprintId":null}`, "detectedSprintId", validationMsgSprintRequired},
		{"detectedSprintId mal formado", `{"detectedSprintId":"123"}`, "detectedSprintId", validationMsgIDInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, tc.body))

			validationRequireOnlyField(t, fields, tc.field, tc.message)
			if got != (domain.DefectPatch{}) {
				t.Errorf("con campos inválidos no se debe devolver ningún cambio, se obtuvo %+v", got)
			}
		})
	}
}

// Escenario: Dado un defecto, cuando un integrante intenta cambiar su estado o su sprint de
// resolución desde la modificación de datos, entonces la acción se rechaza indicando que el estado se
// cambia con las transiciones de US2, US5 y US6 (007/US4). La sola presencia del campo basta, aunque
// valga null (FR-035).
func TestUS4_ValidacionEstadoDesdeModificacion(t *testing.T) {
	for _, field := range []string{"status", "resolvedSprintId"} {
		for _, literal := range []string{`null`, `""`, `"resolved"`, `"` + uuid.NewString() + `"`} {
			t.Run(field+" = "+literal, func(t *testing.T) {
				body := `{"severity":"low","` + field + `":` + literal + `}`

				got, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, body))

				validationRequireOnlyField(t, fields, field, validationMsgStatusNotEditable)
				if got != (domain.DefectPatch{}) {
					t.Errorf("con campos prohibidos no se debe devolver ningún cambio, se obtuvo %+v", got)
				}
			})
		}

		t.Run(field+" como único campo", func(t *testing.T) {
			_, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, `{"`+field+`":null}`))

			validationRequireFieldPresent(t, fields, field, validationMsgStatusNotEditable)
		})
	}
}

// El proyecto y el autor no se modifican (FR-035): projectId, createdBy y createdById se rechazan
// en su campo con la sola presencia, aunque valgan null.
func TestValidateDefectPatch_ProyectoYAutor(t *testing.T) {
	for _, field := range []string{"projectId", "createdBy", "createdById"} {
		for _, literal := range []string{`null`, `"alguien"`, `"` + uuid.NewString() + `"`} {
			t.Run(field+" = "+literal, func(t *testing.T) {
				body := `{"description":"Falla el total","` + field + `":` + literal + `}`

				got, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, body))

				validationRequireOnlyField(t, fields, field, validationMsgOwnerNotEditable)
				if got != (domain.DefectPatch{}) {
					t.Errorf("con campos prohibidos no se debe devolver ningún cambio, se obtuvo %+v", got)
				}
			})
		}

		t.Run(field+" como único campo", func(t *testing.T) {
			_, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, `{"`+field+`":null}`))

			validationRequireFieldPresent(t, fields, field, validationMsgOwnerNotEditable)
		})
	}
}

// La modificación es atómica (FR-037): si un campo es inválido no se devuelve ningún cambio, ni
// siquiera el de los campos válidos.
func TestValidateDefectPatch_Atomicidad(t *testing.T) {
	body := `{"description":"Texto corregido","severity":"urgent","storyId":"` + uuid.NewString() + `"}`

	got, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, body))

	validationRequireOnlyField(t, fields, "severity", validationMsgSeverityInvalid)
	if got != (domain.DefectPatch{}) {
		t.Errorf("con un campo inválido no se debe devolver ningún cambio, se obtuvo %+v", got)
	}
}

// Todos los campos inválidos y prohibidos se informan a la vez, sin detenerse en el primero
// (Principio VII).
func TestValidateDefectPatch_VariosCamposInvalidos(t *testing.T) {
	body := `{"description":"  ","severity":"urgent","storyId":"x","detectedSprintId":"",` +
		`"status":null,"resolvedSprintId":null,"projectId":null,"createdBy":null,"createdById":null}`

	got, fields := ValidateDefectPatch(validationDecodeDefectPatch(t, body))

	want := []string{"createdBy", "createdById", "description", "detectedSprintId", "projectId",
		"resolvedSprintId", "severity", "status", "storyId"}
	names := validationFieldNames(fields)
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("campos rechazados = %v, se esperaba %v", names, want)
	}
	if got != (domain.DefectPatch{}) {
		t.Errorf("con campos inválidos no se debe devolver ningún cambio, se obtuvo %+v", got)
	}
}
