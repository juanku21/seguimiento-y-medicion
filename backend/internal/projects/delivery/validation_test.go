package delivery

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Pruebas unitarias de la validación y la normalización de entrada de proyectos (T017). Los nombres
// llevan el prefijo "Validacion" para no chocar con las pruebas de integración de handler_test.go
// (T019), que cubren los mismos escenarios de punta a punta.

// requireErrorMentions verifica que la validación rechace la entrada y que el mensaje nombre todas
// las piezas esperadas (campo o límites), sin distinguir mayúsculas de minúsculas.
func requireErrorMentions(t *testing.T, err error, parts ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("se esperaba un error de validación y no hubo error")
	}
	msg := strings.ToLower(err.Error())
	for _, part := range parts {
		if !strings.Contains(msg, strings.ToLower(part)) {
			t.Errorf("el mensaje %q debería mencionar %q", err.Error(), part)
		}
	}
}

// mustDate arma una fecha de calendario sin hora con el layout del contrato (YYYY-MM-DD).
func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("fecha de prueba inválida %q: %v", value, err)
	}
	return date
}

// ptr devuelve un puntero a la cadena recibida, para armar descripciones opcionales.
func ptr(s string) *string {
	return &s
}

// Escenario: Creación exitosa (002/US1)
// Cubre el caso normal: "Portal de Clientes", 2026-10-01 a 2026-12-20 y factor 6,5 pasan todas las
// validaciones, y el factor queda en 650 centésimas de hora.
func TestUS1_ValidacionCreacionExitosa(t *testing.T) {
	if err := ValidateProjectName("Portal de Clientes"); err != nil {
		t.Errorf("el nombre debería ser válido: %v", err)
	}
	if err := ValidateDescription(ptr("Rediseño del portal de autogestión.")); err != nil {
		t.Errorf("la descripción debería ser válida: %v", err)
	}
	if err := ValidateDateOrder(mustDate(t, "2026-10-01"), mustDate(t, "2026-12-20")); err != nil {
		t.Errorf("las fechas deberían ser válidas: %v", err)
	}
	hundredths, err := ParseHoursPerStoryPoint(json.Number("6.5"))
	if err != nil {
		t.Fatalf("el factor 6.5 debería ser válido: %v", err)
	}
	if hundredths != 650 {
		t.Errorf("el factor 6.5 debería quedar en 650 centésimas, quedó en %d", hundredths)
	}
}

// Escenario: Sin descripción (002/US1)
// Cubre FR-003: la descripción es opcional. Ausente, vacía o compuesta solo por espacios es válida
// y se guarda como nula; con contenido se guarda recortada.
func TestUS1_ValidacionSinDescripcion(t *testing.T) {
	cases := []struct {
		name  string
		input *string
		want  *string
	}{
		{"ausente", nil, nil},
		{"vacía", ptr(""), nil},
		{"solo espacios", ptr("   \t  "), nil},
		{"con espacios extremos", ptr("  Rediseño del portal  "), ptr("Rediseño del portal")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateDescription(tc.input); err != nil {
				t.Errorf("la descripción debería ser válida: %v", err)
			}
			got := NormalizeDescription(tc.input)
			switch {
			case tc.want == nil && got != nil:
				t.Errorf("la descripción debería quedar nula, quedó %q", *got)
			case tc.want != nil && got == nil:
				t.Errorf("la descripción debería quedar %q, quedó nula", *tc.want)
			case tc.want != nil && *got != *tc.want:
				t.Errorf("la descripción debería quedar %q, quedó %q", *tc.want, *got)
			}
		})
	}
}

// Escenario: Sin descripción (002/US1)
// Cubre el límite de FR-003: hasta 1000 runas contadas tras recortar los espacios extremos. Se usan
// caracteres de varios bytes para comprobar que se cuentan runas y no bytes.
func TestUS1_ValidacionLimitesDeDescripcion(t *testing.T) {
	valid := []struct {
		name  string
		input string
	}{
		{"1000 caracteres", strings.Repeat("a", 1000)},
		{"1000 caracteres con espacios extremos", "  " + strings.Repeat("a", 1000) + "  "},
		{"1000 caracteres de varios bytes", strings.Repeat("ñ", 1000)},
	}
	for _, tc := range valid {
		t.Run("válida: "+tc.name, func(t *testing.T) {
			if err := ValidateDescription(ptr(tc.input)); err != nil {
				t.Errorf("la descripción debería ser válida: %v", err)
			}
		})
	}

	invalid := []struct {
		name  string
		input string
	}{
		{"1001 caracteres", strings.Repeat("a", 1001)},
		{"1001 caracteres de varios bytes", strings.Repeat("ñ", 1001)},
	}
	for _, tc := range invalid {
		t.Run("inválida: "+tc.name, func(t *testing.T) {
			requireErrorMentions(t, ValidateDescription(ptr(tc.input)), "descripción", "1000")
		})
	}
}

// Escenario: Límites del nombre (002/US1)
// Cubre RN-04: el nombre mide entre 1 y 100 runas contadas tras recortar los espacios extremos; 100
// es válido y 101 se rechaza. Se cuentan runas, no bytes.
func TestUS1_ValidacionLimitesDelNombre(t *testing.T) {
	valid := []struct {
		name  string
		input string
	}{
		{"1 carácter", "P"},
		{"100 caracteres", strings.Repeat("a", 100)},
		{"100 caracteres con espacios extremos", "   " + strings.Repeat("a", 100) + "   "},
		{"100 caracteres de varios bytes", strings.Repeat("ñ", 100)},
	}
	for _, tc := range valid {
		t.Run("válido: "+tc.name, func(t *testing.T) {
			if err := ValidateProjectName(tc.input); err != nil {
				t.Errorf("el nombre debería ser válido: %v", err)
			}
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
			requireErrorMentions(t, ValidateProjectName(tc.input), "nombre", "100")
		})
	}
}

// Escenario: Campos faltantes (002/US1)
// Cubre RN-04 y RN-07: un nombre ausente o compuesto solo por espacios equivale a ausente y se
// rechaza; un factor ausente (json.Number vacío) también se rechaza indicando el campo.
func TestUS1_ValidacionCamposFaltantes(t *testing.T) {
	t.Run("nombre vacío", func(t *testing.T) {
		requireErrorMentions(t, ValidateProjectName(""), "nombre")
	})
	t.Run("nombre solo de espacios", func(t *testing.T) {
		requireErrorMentions(t, ValidateProjectName("   \t "), "nombre")
	})
	t.Run("factor ausente", func(t *testing.T) {
		_, err := ParseHoursPerStoryPoint(json.Number(""))
		requireErrorMentions(t, err, "factor")
	})
}

// Escenario: Nombre repetido (002/US1)
// Cubre RN-05: el nombre se normaliza con TrimSpace y ToLower, de modo que "  portal de clientes  "
// y "Portal de Clientes" resultan en el mismo valor de comparación.
func TestUS1_ValidacionNombreRepetido(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"capitalización original", "Portal de Clientes", "portal de clientes"},
		{"espacios extremos y minúsculas", "  portal de clientes  ", "portal de clientes"},
		{"mayúsculas con tabulaciones", "\tPORTAL DE CLIENTES\n", "portal de clientes"},
		{"letras con tilde", "  Gestión ÁGIL ", "gestión ágil"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeProjectName(tc.input); got != tc.want {
				t.Errorf("NormalizeProjectName(%q) = %q, se esperaba %q", tc.input, got, tc.want)
			}
		})
	}
}

// Escenario: Fechas iguales (002/US1)
// Cubre RN-06 en su límite: una fecha de finalización prevista igual a la de inicio es válida.
func TestUS1_ValidacionFechasIguales(t *testing.T) {
	date := mustDate(t, "2026-10-01")
	if err := ValidateDateOrder(date, date); err != nil {
		t.Errorf("fechas iguales deberían ser válidas: %v", err)
	}
}

// Escenario: Fecha prevista anterior (002/US1)
// Cubre RN-06: una fecha de finalización prevista anterior a la de inicio se rechaza indicando que
// no puede ser anterior, incluso por un solo día.
func TestUS1_ValidacionFechaPrevistaAnterior(t *testing.T) {
	cases := []struct {
		name       string
		start, end string
	}{
		{"un día antes", "2026-10-01", "2026-09-30"},
		{"meses antes", "2026-12-20", "2026-10-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDateOrder(mustDate(t, tc.start), mustDate(t, tc.end))
			requireErrorMentions(t, err, "anterior")
		})
	}
}

// Escenario: Límites del factor (002/US1)
// Cubre RN-07 en sus límites: 0,01 y 40 son el mínimo y el máximo admitidos, y el factor se
// convierte a centésimas de hora enteras sin redondear (1 a 4000).
func TestUS1_ValidacionLimitesDelFactor(t *testing.T) {
	cases := []struct {
		input json.Number
		want  int
	}{
		{"0.01", 1},
		{"40", 4000},
		{"40.00", 4000},
		{"1", 100},
		{"6.5", 650},
		{"6.50", 650},
		{"6.55", 655},
		{"39.99", 3999},
	}
	for _, tc := range cases {
		t.Run(string(tc.input), func(t *testing.T) {
			got, err := ParseHoursPerStoryPoint(tc.input)
			if err != nil {
				t.Fatalf("el factor %s debería ser válido: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("el factor %s debería quedar en %d centésimas, quedó en %d", tc.input, tc.want, got)
			}
		})
	}
}

// Escenario: Factor inválido (002/US1)
// Cubre RN-07: se rechazan 0, los negativos, los mayores que 40 y los que tienen más de 2
// decimales, sin redondear (6.555 no se convierte en 6.56 ni 0.001 en 0), indicando el rango y la
// precisión admitidos.
func TestUS1_ValidacionFactorInvalido(t *testing.T) {
	cases := []struct {
		name  string
		input json.Number
	}{
		{"cero", "0"},
		{"cero con decimales", "0.00"},
		{"negativo", "-1"},
		{"negativo mínimo", "-0.01"},
		{"40,01", "40.01"},
		{"41", "41"},
		{"6,555 con tres decimales", "6.555"},
		{"0,001 con tres decimales", "0.001"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseHoursPerStoryPoint(tc.input)
			requireErrorMentions(t, err, "factor", "40", "2 decimales")
			if got != 0 {
				t.Errorf("un factor rechazado no debería devolver centésimas, devolvió %d", got)
			}
		})
	}
}
