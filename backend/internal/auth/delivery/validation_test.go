package delivery

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Pruebas unitarias de la validación y la normalización de entrada del registro (T025). Los
// nombres llevan el sufijo "Validacion" para no chocar con las pruebas de integración de
// handler_test.go (T027), que cubren los mismos escenarios de punta a punta.

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

// buildEmail arma un correo sintácticamente válido de exactamente n caracteres (n >= 70), con una
// parte local de 64 caracteres y un dominio de etiquetas de hasta 63 caracteres.
func buildEmail(t *testing.T, n int) string {
	t.Helper()
	local := strings.Repeat("a", 64)
	remaining := n - len(local) - 1 - len(".com")
	var labels []string
	for remaining > 0 {
		size := min(remaining, 63)
		// Si sobra un único carácter tras una etiqueta llena, se reserva para el punto separador.
		if remaining > 63 && remaining-63 < 2 {
			size = 60
		}
		labels = append(labels, strings.Repeat("b", size))
		remaining -= size + 1
	}
	email := local + "@" + strings.Join(labels, ".") + ".com"
	if len(email) != n {
		t.Fatalf("buildEmail generó %d caracteres en lugar de %d", len(email), n)
	}
	return email
}

// Escenario: Correo normalizado (001/US1)
// Cubre RN-01 y RN-02: el correo se recorta y se pasa a minúsculas, de modo que las variantes con
// otra capitalización o con espacios extremos resultan en el mismo correo, sin tocar puntos ni alias.
func TestUS1_ValidacionCorreoNormalizado(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"espacios extremos y mayúsculas", "  Ana@Example.COM  ", "ana@example.com"},
		{"otra capitalización del mismo correo", "ANA@example.com", "ana@example.com"},
		{"tabulaciones y saltos de línea", "\tAna@Example.com\n", "ana@example.com"},
		{"correo ya normalizado", "ana@example.com", "ana@example.com"},
		{"conserva puntos y alias con signo más", "Ana.Perez+Equipo@Example.com", "ana.perez+equipo@example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeEmail(tc.input); got != tc.want {
				t.Errorf("NormalizeEmail(%q) = %q; se esperaba %q", tc.input, got, tc.want)
			}
		})
	}
}

// Escenario: Alta exitosa (001/US1)
// Los datos del caso normal de la spec superan las tres validaciones.
func TestUS1_ValidacionAltaExitosa(t *testing.T) {
	if err := ValidateFullName("Ana Pérez"); err != nil {
		t.Errorf("ValidateFullName rechazó un nombre válido: %v", err)
	}
	if err := ValidateEmail("ana@example.com"); err != nil {
		t.Errorf("ValidateEmail rechazó un correo válido: %v", err)
	}
	if err := ValidatePassword("Proyecto2026"); err != nil {
		t.Errorf("ValidatePassword rechazó una contraseña válida: %v", err)
	}
}

// Escenario: Límites de nombre (001/US1)
// Cubre RN-03: se aceptan 2 y 100 runas contadas tras recortar los espacios extremos, aunque sean
// caracteres multibyte, y se rechazan 1 y 101 con un mensaje que indica el rango permitido.
func TestUS1_ValidacionLimitesDeNombre(t *testing.T) {
	valid := []struct {
		name  string
		input string
	}{
		{"exactamente 2 caracteres", "Al"},
		{"exactamente 100 caracteres", strings.Repeat("a", 100)},
		{"100 caracteres acentuados cuentan como 100", strings.Repeat("é", 100)},
		{"100 caracteres rodeados de espacios", "   " + strings.Repeat("a", 100) + "   "},
		{"2 caracteres rodeados de espacios", "  Al  "},
	}
	for _, tc := range valid {
		t.Run("acepta "+tc.name, func(t *testing.T) {
			if err := ValidateFullName(tc.input); err != nil {
				t.Errorf("ValidateFullName rechazó un nombre válido: %v", err)
			}
		})
	}

	invalid := []struct {
		name  string
		input string
	}{
		{"1 carácter", "A"},
		{"101 caracteres", strings.Repeat("a", 101)},
		{"1 carácter tras recortar espacios", "   A   "},
		{"101 caracteres acentuados", strings.Repeat("é", 101)},
	}
	for _, tc := range invalid {
		t.Run("rechaza "+tc.name, func(t *testing.T) {
			requireErrorMentions(t, ValidateFullName(tc.input), "nombre", "2", "100")
		})
	}
}

// Escenario: Límites de contraseña (001/US1)
// Cubre RN-04: se aceptan 8 y 72 runas con letra y dígito; el conteo es en runas (un emoji o una
// letra acentuada cuentan como un carácter) y se aceptan exactamente 72 bytes en UTF-8.
func TestUS1_ValidacionLimitesDeContrasena(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"exactamente 8 caracteres", "abcdefg1"},
		{"exactamente 72 caracteres", strings.Repeat("a", 71) + "1"},
		{"8 caracteres con emoji", strings.Repeat("😀", 6) + "a1"},
		{"letras acentuadas que suman exactamente 72 bytes", strings.Repeat("á", 35) + "a1"},
	}
	for _, tc := range cases {
		t.Run("acepta "+tc.name, func(t *testing.T) {
			if err := ValidatePassword(tc.input); err != nil {
				t.Errorf("ValidatePassword rechazó una contraseña válida (%d runas, %d bytes): %v",
					utf8.RuneCountInString(tc.input), len(tc.input), err)
			}
		})
	}
}

// Escenario: Correo de hasta 254 caracteres (001/US1)
// Cubre FR-003: se acepta un correo válido de exactamente 254 caracteres y se rechaza uno de 255.
func TestUS1_ValidacionLongitudDeCorreo(t *testing.T) {
	t.Run("acepta exactamente 254 caracteres", func(t *testing.T) {
		if err := ValidateEmail(buildEmail(t, 254)); err != nil {
			t.Errorf("ValidateEmail rechazó un correo de 254 caracteres: %v", err)
		}
	})
	t.Run("rechaza 255 caracteres", func(t *testing.T) {
		requireErrorMentions(t, ValidateEmail(buildEmail(t, 255)), "correo")
	})
}

// Escenario: Entrada inválida (001/US1)
// Cubre el caso de error 8 de la spec y la tabla de condiciones de error: cada entrada inválida se
// rechaza indicando qué campo corregir y por qué, y el mensaje de la contraseña nunca la repite.
func TestUS1_ValidacionEntradaInvalida(t *testing.T) {
	// Nombre ausente o equivalente a ausente: el mensaje identifica el campo.
	t.Run("nombre ausente", func(t *testing.T) {
		requireErrorMentions(t, ValidateFullName(""), "nombre")
	})
	t.Run("nombre solo de espacios", func(t *testing.T) {
		requireErrorMentions(t, ValidateFullName("     "), "nombre")
	})

	// Correo ausente o con formato inválido: el mensaje identifica el campo.
	invalidEmails := []struct {
		name  string
		input string
	}{
		{"correo ausente", ""},
		{"correo sin arroba", "ana.example.com"},
		{"correo sin dominio", "ana@"},
		{"correo sin parte local", "@example.com"},
		{"correo con espacios internos", "ana perez@example.com"},
		{"correo con nombre para mostrar entre comillas", `"Ana" <ana@example.com>`},
		{"correo con nombre para mostrar sin comillas", "Ana <ana@example.com>"},
		{"dos correos separados por coma", "ana@example.com, beto@example.com"},
	}
	for _, tc := range invalidEmails {
		t.Run(tc.name, func(t *testing.T) {
			requireErrorMentions(t, ValidateEmail(tc.input), "correo")
		})
	}

	// Contraseña fuera de rango, en runas o en bytes: todas comparten el mensaje del rango.
	rangeCases := []struct {
		name  string
		input string
	}{
		{"contraseña ausente", ""},
		{"contraseña de 7 caracteres", "abcdef1"},
		{"contraseña de 7 caracteres multibyte que supera 8 bytes", "ááááá1a"},
		{"contraseña de 73 caracteres", strings.Repeat("a", 72) + "1"},
		{"contraseña de menos de 72 caracteres que supera 72 bytes", strings.Repeat("á", 40) + "1"},
		{"contraseña de 72 caracteres que supera 72 bytes", strings.Repeat("ñ", 71) + "1"},
	}
	reference := ValidatePassword(strings.Repeat("a", 72) + "1")
	for _, tc := range rangeCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.input)
			requireErrorMentions(t, err, "contraseña", "8", "72")
			if err != nil && reference != nil && err.Error() != reference.Error() {
				t.Errorf("el mensaje %q debería ser el mismo del rango permitido %q", err.Error(), reference.Error())
			}
			if err != nil && tc.input != "" && strings.Contains(err.Error(), tc.input) {
				t.Errorf("el mensaje repite la contraseña enviada: %q", err.Error())
			}
		})
	}

	// Contraseña sin la composición mínima: el mensaje es distinto del de rango e indica qué falta.
	compositionCases := []struct {
		name  string
		input string
	}{
		{"contraseña sin letra", "12345678"},
		{"contraseña sin dígito", "abcdefgh"},
		{"contraseña sin dígito con letras acentuadas", "ñandúesgrande"},
	}
	for _, tc := range compositionCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.input)
			requireErrorMentions(t, err, "contraseña", "letra", "número")
			if err != nil && reference != nil && err.Error() == reference.Error() {
				t.Errorf("la falta de composición no debería informarse con el mensaje del rango: %q", err.Error())
			}
			if err != nil && strings.Contains(err.Error(), tc.input) {
				t.Errorf("el mensaje repite la contraseña enviada: %q", err.Error())
			}
		})
	}
}

// validateLogin solo exige que el correo, ya normalizado, y la contraseña estén presentes: devuelve
// un detalle por cada campo vacío, en el orden email, password, y ninguno cuando ambos vienen
// (Principio II, validaciones de entrada). Un valor que no cumpla las reglas del registro no se
// rechaza acá, porque simplemente no coincidirá con ninguna cuenta.
func TestValidateLogin_RequiresEmailAndPassword(t *testing.T) {
	cases := []struct {
		name     string
		email    string
		password string
		want     []string
	}{
		{"correo y contraseña presentes", "ana@example.com", "Proyecto2026", nil},
		{"credenciales que no cumplen las reglas del registro", "no-es-un-correo", "corta", nil},
		{"correo vacío", "", "Proyecto2026", []string{"email"}},
		{"contraseña vacía", "ana@example.com", "", []string{"password"}},
		{"correo y contraseña vacíos", "", "", []string{"email", "password"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := validateLogin(tc.email, tc.password)

			got := make([]string, 0, len(fields))
			for _, f := range fields {
				got = append(got, f.Field)
				if f.Message == "" {
					t.Errorf("el campo %q no trae mensaje", f.Field)
				}
				if tc.password != "" && strings.Contains(f.Message, tc.password) {
					t.Errorf("el mensaje de %q repite la contraseña: %q", f.Field, f.Message)
				}
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("campos señalados %v, se esperaba %v", got, tc.want)
			}
		})
	}
}
