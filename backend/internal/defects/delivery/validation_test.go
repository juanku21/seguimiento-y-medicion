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
