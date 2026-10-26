package delivery

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
)

// Pruebas unitarias obligatorias de la validación del alta de un registro de esfuerzo (T009; RN-02,
// RN-03, SC-006; research.md sección 6). No llevan la etiqueta de un escenario de la spec: los
// nombres TestUS1_* son de las pruebas de integración de handler_test.go (T015), y la trazabilidad
// queda en el comentario de cada prueba. Los ayudantes llevan el prefijo "validation" para no chocar
// con los de las pruebas de integración del mismo paquete.

// Mensajes que la validación debe devolver, tal como los fija tasks.md (T009).
const (
	validationMsgHoursRange    = "Las horas deben ser mayores que 0 y como máximo 24."
	validationMsgHoursMultiple = "Las horas se cargan en múltiplos de 0,25 (un cuarto de hora)."
	validationMsgHoursNumber   = "Las horas son obligatorias y deben ser un número."
	validationMsgActivityEmpty = "La actividad es obligatoria."
	validationMsgActivityLong  = "La actividad admite como máximo 300 caracteres."
)

// validationInput arma un alta válida completa (6,5 h del 2026-10-24) sobre la que cada prueba
// modifica solo el campo que le interesa.
func validationInput() EffortEntryInput {
	return EffortEntryInput{
		EntryDate: "2026-10-24",
		Activity:  "Implementación del formulario de alta",
		Hours:     json.RawMessage(`6.5`),
	}
}

// validationFieldNames extrae los nombres de los campos rechazados, ordenados, para comparar sin
// depender del orden en que se informan.
func validationFieldNames(fields []authdelivery.FieldError) []string {
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, f.Field)
	}
	sort.Strings(names)
	return names
}

// validationRequireOnlyField exige que la validación rechace exactamente un campo, el indicado, y
// que su mensaje contenga el texto esperado.
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

// Las horas válidas se convierten a centésimas de hora exactas, sin pasar por float64: el mínimo
// 0,25, un valor con uno y con dos decimales y el máximo 24 (006/US1, RN-03, SC-006).
func TestParseHours_Validas(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"0.25", 25},
		{"6.5", 650},
		{"6.50", 650},
		{"24", 2400},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := ParseHours(json.RawMessage(tc.raw))
			if err != nil {
				t.Fatalf("%s debería ser válido: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("%s debería quedar en %d centésimas, quedó en %d", tc.raw, tc.want, got)
			}
		})
	}
}

// Las horas en 0, negativas o por encima de 24 se rechazan indicando el rango admitido (006/US1,
// RN-03, SC-006).
func TestParseHours_FueraDeRango(t *testing.T) {
	for _, raw := range []string{"0", "-1", "24.25", "25"} {
		t.Run(raw, func(t *testing.T) {
			_, err := ParseHours(json.RawMessage(raw))
			if err == nil {
				t.Fatalf("%s debería rechazarse", raw)
			}
			if err.Error() != validationMsgHoursRange {
				t.Errorf("mensaje %q, se esperaba %q", err.Error(), validationMsgHoursRange)
			}
		})
	}
}

// Las horas dentro del rango que no son múltiplo de 0,25 se rechazan indicando el múltiplo, incluido
// 0.10, que tiene dos decimales pero no es un cuarto de hora (006/US1, RN-03, SC-006).
func TestParseHours_NoMultiploDeCuarto(t *testing.T) {
	for _, raw := range []string{"1.3", "1.1", "0.10"} {
		t.Run(raw, func(t *testing.T) {
			_, err := ParseHours(json.RawMessage(raw))
			if err == nil {
				t.Fatalf("%s debería rechazarse", raw)
			}
			if err.Error() != validationMsgHoursMultiple {
				t.Errorf("mensaje %q, se esperaba %q", err.Error(), validationMsgHoursMultiple)
			}
		})
	}
}

// Las horas que no son un número JSON con a lo sumo dos decimales y sin exponente se rechazan como
// obligatorias y numéricas: tres decimales, notación exponencial, número entre comillas, texto,
// null, booleano y ausencia (006/US1, RN-03, SC-006; research.md sección 5).
func TestParseHours_NoNumericas(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
	}{
		{"tres decimales", json.RawMessage(`6.555`)},
		{"exponente", json.RawMessage(`1e1`)},
		{"número entre comillas", json.RawMessage(`"6.5"`)},
		{"texto", json.RawMessage(`"abc"`)},
		{"null", json.RawMessage(`null`)},
		{"booleano", json.RawMessage(`true`)},
		{"ausente", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseHours(tc.raw)
			if err == nil {
				t.Fatalf("%q debería rechazarse", string(tc.raw))
			}
			if err.Error() != validationMsgHoursNumber {
				t.Errorf("mensaje %q, se esperaba %q", err.Error(), validationMsgHoursNumber)
			}
		})
	}
}

// Caso normal: un alta completa pasa la validación y queda normalizada, con la actividad recortada,
// la fecha a medianoche UTC y las horas en centésimas (006/US1, RN-02, RN-03).
func TestValidateEntryInput_EntradaCompleta(t *testing.T) {
	req := validationInput()
	req.Activity = "  Implementación del formulario de alta  "

	got, fields := ValidateEntryInput(req)
	if len(fields) != 0 {
		t.Fatalf("la entrada debería ser válida, se obtuvieron %v", fields)
	}
	want := domain.EntryInput{
		EntryDate:       time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC),
		Activity:        "Implementación del formulario de alta",
		HoursHundredths: 650,
	}
	if !got.EntryDate.Equal(want.EntryDate) || got.EntryDate.Location() != time.UTC {
		t.Errorf("la fecha debería ser %v, fue %v", want.EntryDate, got.EntryDate)
	}
	if got.Activity != want.Activity {
		t.Errorf("la actividad debería ser %q, fue %q", want.Activity, got.Activity)
	}
	if got.HoursHundredths != want.HoursHundredths {
		t.Errorf("las horas deberían ser %d, fueron %d", want.HoursHundredths, got.HoursHundredths)
	}
}

// La actividad es obligatoria: ausente o compuesta solo por espacios se rechaza en su campo
// (006/US1, RN-02).
func TestValidateEntryInput_ActividadObligatoria(t *testing.T) {
	for name, activity := range map[string]string{"vacía": "", "solo espacios": "   "} {
		t.Run(name, func(t *testing.T) {
			req := validationInput()
			req.Activity = activity
			_, fields := ValidateEntryInput(req)
			validationRequireOnlyField(t, fields, "activity", validationMsgActivityEmpty)
		})
	}
}

// La actividad mide hasta 300 runas tras recortar los espacios extremos: 300 caracteres acentuados
// (de dos bytes cada uno) se aceptan y se devuelven recortados, y 301 se rechazan indicando el
// máximo (006/US1, RN-02, FR-003).
func TestValidateEntryInput_LimitesDeLaActividad(t *testing.T) {
	t.Run("300 caracteres con acentos", func(t *testing.T) {
		activity := strings.Repeat("á", 300)
		req := validationInput()
		req.Activity = "  " + activity + "  "
		got, fields := ValidateEntryInput(req)
		if len(fields) != 0 {
			t.Fatalf("la actividad de 300 caracteres debería ser válida, se obtuvieron %v", fields)
		}
		if got.Activity != activity {
			t.Errorf("la actividad debería devolverse recortada, con %d bytes, tuvo %d",
				len(activity), len(got.Activity))
		}
	})
	t.Run("301 caracteres con acentos", func(t *testing.T) {
		req := validationInput()
		req.Activity = strings.Repeat("á", 301)
		_, fields := ValidateEntryInput(req)
		validationRequireOnlyField(t, fields, "activity", validationMsgActivityLong)
	})
}

// Las horas inválidas se informan en el campo hours con el mensaje de ParseHours, incluida su
// ausencia en el cuerpo (006/US1, RN-03, SC-006).
func TestValidateEntryInput_HorasInvalidas(t *testing.T) {
	cases := []struct {
		name    string
		raw     json.RawMessage
		message string
	}{
		{"fuera de rango", json.RawMessage(`25`), validationMsgHoursRange},
		{"no múltiplo", json.RawMessage(`1.3`), validationMsgHoursMultiple},
		{"entre comillas", json.RawMessage(`"6.5"`), validationMsgHoursNumber},
		{"ausente", nil, validationMsgHoursNumber},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validationInput()
			req.Hours = tc.raw
			_, fields := ValidateEntryInput(req)
			validationRequireOnlyField(t, fields, "hours", tc.message)
		})
	}
}

// La fecha es obligatoria y debe ser un día real con el formato AAAA-MM-DD: ausente, inexistente
// (2026-02-30) o con otro formato (24/10/2026) se rechaza en entryDate indicando el formato
// (006/US1, FR-006; research.md sección 6).
func TestValidateEntryInput_FechaInvalida(t *testing.T) {
	for name, date := range map[string]string{
		"ausente":      "",
		"inexistente":  "2026-02-30",
		"otro formato": "24/10/2026",
	} {
		t.Run(name, func(t *testing.T) {
			req := validationInput()
			req.EntryDate = date
			_, fields := ValidateEntryInput(req)
			validationRequireOnlyField(t, fields, "entryDate", "AAAA-MM-DD")
		})
	}
}

// Varios campos inválidos se devuelven todos a la vez, uno por campo, y los datos normalizados
// quedan vacíos (006/US1, Principio VII; research.md sección 6).
func TestValidateEntryInput_TodosLosCamposALaVez(t *testing.T) {
	req := EffortEntryInput{
		EntryDate: "24/10/2026",
		Activity:  "   ",
		Hours:     json.RawMessage(`0`),
	}
	got, fields := ValidateEntryInput(req)
	names := validationFieldNames(fields)
	want := []string{"activity", "entryDate", "hours"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("los campos rechazados deberían ser %v, fueron %v", want, names)
	}
	if got != (domain.EntryInput{}) {
		t.Errorf("con campos inválidos la entrada normalizada debería quedar vacía, fue %+v", got)
	}
}

// Pruebas unitarias obligatorias de la validación de la modificación parcial de un registro (T042;
// RN-12, FR-023; research.md sección 6). Igual que las del alta, no llevan la etiqueta de un
// escenario: los nombres TestUS3_* son de las pruebas de integración de handler_test.go (T044).

// Mensajes propios de la modificación, tal como los fija tasks.md (T042).
const (
	validationMsgPatchEmpty     = "Hay que indicar al menos un campo a modificar."
	validationMsgPatchImmutable = "La historia, el integrante y el sprint de un registro no se modifican; para mover horas, eliminá el registro y creá uno nuevo."
)

// validationDecodeEntryPatch decodifica el cuerpo JSON de la modificación como lo haría el
// manejador, de modo que un campo con null llegue presente, y falla la prueba si el JSON no es válido.
func validationDecodeEntryPatch(t *testing.T, body string) EffortEntryPatch {
	t.Helper()
	var req EffortEntryPatch
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("el cuerpo de prueba debe ser JSON válido: %v", err)
	}
	return req
}

// validationRequirePatchRejected exige que la modificación se rechace con exactamente el campo
// indicado y el mensaje esperado, y que no devuelva ningún cambio (FR-025).
func validationRequirePatchRejected(t *testing.T, body, field, message string) {
	t.Helper()
	got, fields := ValidateEntryPatch(validationDecodeEntryPatch(t, body))
	validationRequireOnlyField(t, fields, field, message)
	if got != (domain.EntryPatch{}) {
		t.Errorf("una modificación rechazada no debería devolver cambios, devolvió %+v", got)
	}
}

// Un cuerpo vacío no modifica nada y se rechaza pidiendo al menos un campo (006/US3, FR-023;
// contracts/openapi.yaml, minProperties: 1).
func TestValidateEntryPatch_CuerpoVacio(t *testing.T) {
	got, fields := ValidateEntryPatch(validationDecodeEntryPatch(t, `{}`))
	if len(fields) != 1 {
		t.Fatalf("se esperaba un único error, se obtuvieron %v", fields)
	}
	if fields[0].Message != validationMsgPatchEmpty {
		t.Errorf("mensaje %q, se esperaba %q", fields[0].Message, validationMsgPatchEmpty)
	}
	if got != (domain.EntryPatch{}) {
		t.Errorf("un cuerpo vacío no debería devolver cambios, devolvió %+v", got)
	}
}

// Caso normal: una modificación con solo las horas es válida, las devuelve en centésimas y deja sin
// cambio la fecha y la actividad (006/US3, RN-03, FR-023).
func TestValidateEntryPatch_SoloHoras(t *testing.T) {
	got, fields := ValidateEntryPatch(validationDecodeEntryPatch(t, `{"hours": 6}`))
	if len(fields) != 0 {
		t.Fatalf("la modificación debería ser válida, se obtuvieron %v", fields)
	}
	if got.HoursHundredths == nil || *got.HoursHundredths != 600 {
		t.Errorf("las horas deberían quedar en 600 centésimas, quedaron en %v", got.HoursHundredths)
	}
	if got.EntryDate != nil || got.Activity != nil {
		t.Errorf("la fecha y la actividad no deberían cambiar, se obtuvo %+v", got)
	}
}

// Una modificación con los tres campos editables se normaliza igual que el alta: la actividad
// recortada, la fecha a medianoche UTC y las horas en centésimas (006/US3, RN-02, RN-03, FR-023).
func TestValidateEntryPatch_TodosLosCamposEditables(t *testing.T) {
	body := `{"entryDate": "2026-10-23", "activity": "  Pruebas del formulario  ", "hours": 2.75}`
	got, fields := ValidateEntryPatch(validationDecodeEntryPatch(t, body))
	if len(fields) != 0 {
		t.Fatalf("la modificación debería ser válida, se obtuvieron %v", fields)
	}
	wantDate := time.Date(2026, time.October, 23, 0, 0, 0, 0, time.UTC)
	if got.EntryDate == nil || !got.EntryDate.Equal(wantDate) || got.EntryDate.Location() != time.UTC {
		t.Errorf("la fecha debería ser %v, fue %v", wantDate, got.EntryDate)
	}
	if got.Activity == nil || *got.Activity != "Pruebas del formulario" {
		t.Errorf("la actividad debería devolverse recortada, fue %v", got.Activity)
	}
	if got.HoursHundredths == nil || *got.HoursHundredths != 275 {
		t.Errorf("las horas deberían quedar en 275 centésimas, quedaron en %v", got.HoursHundredths)
	}
}

// Las horas presentes se validan con las mismas reglas que el alta: 0, 25, 1,1, un número entre
// comillas y null se rechazan en hours con el mensaje de ParseHours (006/US3, RN-03, FR-023).
func TestValidateEntryPatch_HorasInvalidas(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		message string
	}{
		{"cero", `{"hours": 0}`, validationMsgHoursRange},
		{"mayor que 24", `{"hours": 25}`, validationMsgHoursRange},
		{"no múltiplo", `{"hours": 1.1}`, validationMsgHoursMultiple},
		{"entre comillas", `{"hours": "6"}`, validationMsgHoursNumber},
		{"null", `{"hours": null}`, validationMsgHoursNumber},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validationRequirePatchRejected(t, tc.body, "hours", tc.message)
		})
	}
}

// La actividad presente se valida con las mismas reglas que el alta: vacía, solo espacios o null se
// rechaza como obligatoria, y con 301 caracteres se rechaza indicando el máximo (006/US3, RN-02,
// FR-023).
func TestValidateEntryPatch_ActividadInvalida(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		message string
	}{
		{"vacía", `{"activity": ""}`, validationMsgActivityEmpty},
		{"solo espacios", `{"activity": "   "}`, validationMsgActivityEmpty},
		{"null", `{"activity": null}`, validationMsgActivityEmpty},
		{"301 caracteres", `{"activity": "` + strings.Repeat("á", 301) + `"}`, validationMsgActivityLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validationRequirePatchRejected(t, tc.body, "activity", tc.message)
		})
	}
}

// La fecha presente se valida con las mismas reglas que el alta: vacía, inexistente, con otro
// formato o null se rechaza en entryDate indicando el formato (006/US3, FR-006, FR-023).
func TestValidateEntryPatch_FechaInvalida(t *testing.T) {
	cases := map[string]string{
		"vacía":        `{"entryDate": ""}`,
		"inexistente":  `{"entryDate": "2026-02-30"}`,
		"otro formato": `{"entryDate": "24/10/2026"}`,
		"null":         `{"entryDate": null}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			validationRequirePatchRejected(t, body, "entryDate", "AAAA-MM-DD")
		})
	}
}

// La historia, el integrante y el sprint de un registro son inmutables: si el cuerpo los incluye se
// rechazan en su propio campo, aunque valgan null, lleguen junto a un cambio válido de horas o sean
// el único campo del cuerpo (006/US3, RN-12, FR-019).
func TestValidateEntryPatch_CamposInmutables(t *testing.T) {
	for _, field := range []string{"storyId", "userId", "sprintId"} {
		t.Run(field+" con valor", func(t *testing.T) {
			body := `{"hours": 6, "` + field + `": "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04"}`
			validationRequirePatchRejected(t, body, field, validationMsgPatchImmutable)
		})
		t.Run(field+" en null", func(t *testing.T) {
			body := `{"hours": 6, "` + field + `": null}`
			validationRequirePatchRejected(t, body, field, validationMsgPatchImmutable)
		})
		t.Run(field+" como único campo", func(t *testing.T) {
			body := `{"` + field + `": null}`
			validationRequirePatchRejected(t, body, field, validationMsgPatchImmutable)
		})
	}
}

// Varios campos inválidos se devuelven todos a la vez, uno por campo, incluidos los inmutables, y no
// se devuelve ningún cambio (006/US3, Principio VII, FR-025).
func TestValidateEntryPatch_TodosLosCamposALaVez(t *testing.T) {
	body := `{"entryDate": "24/10/2026", "activity": " ", "hours": 0, "storyId": null, "userId": null, "sprintId": null}`
	got, fields := ValidateEntryPatch(validationDecodeEntryPatch(t, body))
	names := validationFieldNames(fields)
	want := []string{"activity", "entryDate", "hours", "sprintId", "storyId", "userId"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("los campos rechazados deberían ser %v, fueron %v", want, names)
	}
	if got != (domain.EntryPatch{}) {
		t.Errorf("con campos inválidos no debería devolver cambios, devolvió %+v", got)
	}
}
