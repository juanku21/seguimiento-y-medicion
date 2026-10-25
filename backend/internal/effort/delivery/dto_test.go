package delivery

import (
	"encoding/json"
	"testing"
)

// Pruebas unitarias obligatorias del tipo de 2 decimales (T010; SC-015, SC-022; research.md
// sección 5). No son escenarios Given/When/Then de la spec: la trazabilidad queda en el comentario
// de cada prueba (006/US1).

// Formato con exactamente 2 decimales y signo a partir del entero de centésimas, incluido el caso
// del redondeo de 008/US1-1 (1310 → 13.10) (006/US1, SC-015, SC-022).
func TestHundredths_MarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		value Hundredths
		want  string
	}{
		{name: "seis horas y media", value: 650, want: "6.50"},
		{name: "entero conserva los decimales", value: 2000, want: "20.00"},
		{name: "negativo entero", value: -2000, want: "-20.00"},
		{name: "un cuarto de hora", value: 25, want: "0.25"},
		{name: "cero", value: 0, want: "0.00"},
		{name: "negativo menor que uno", value: -5, want: "-0.05"},
		{name: "porcentaje redondeado", value: 1310, want: "13.10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatalf("json.Marshal(%d) devolvió error: %v", int(tt.value), err)
			}
			if string(got) != tt.want {
				t.Errorf("json.Marshal(%d) = %s, want %s", int(tt.value), got, tt.want)
			}
		})
	}
}

// Un puntero nulo representa "no calculable" y se serializa null, también como campo de una
// respuesta (006/US1, SC-022).
func TestHundredths_MarshalJSON_NilPointer(t *testing.T) {
	var value *Hundredths

	got, err := json.Marshal(struct {
		Percent *Hundredths `json:"percent"`
	}{Percent: value})
	if err != nil {
		t.Fatalf("json.Marshal con puntero nulo devolvió error: %v", err)
	}
	if string(got) != `{"percent":null}` {
		t.Errorf("json.Marshal con puntero nulo = %s, want {\"percent\":null}", got)
	}
}

// El resultado es un número JSON válido: dentro de un objeto se vuelve a decodificar con
// json.Unmarshal como número y conserva el valor (006/US1, SC-015, SC-022).
func TestHundredths_MarshalJSON_ValidNumber(t *testing.T) {
	tests := []struct {
		name  string
		value Hundredths
		want  json.Number
	}{
		{name: "positivo", value: 650, want: "6.50"},
		{name: "negativo menor que uno", value: -5, want: "-0.05"},
		{name: "cero", value: 0, want: "0.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.value
			encoded, err := json.Marshal(struct {
				Hours *Hundredths `json:"hours"`
			}{Hours: &value})
			if err != nil {
				t.Fatalf("json.Marshal(%d) devolvió error: %v", int(tt.value), err)
			}

			// Se decodifica a un mapa genérico para exigir un número JSON y no un texto entre comillas.
			var decoded map[string]any
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("json.Unmarshal(%s) devolvió error: %v", encoded, err)
			}
			if _, isNumber := decoded["hours"].(float64); !isNumber {
				t.Fatalf("hours = %#v en %s, want un número JSON", decoded["hours"], encoded)
			}

			// Con json.Number se compara el texto literal sin pasar por float64.
			var literal struct {
				Hours json.Number `json:"hours"`
			}
			if err := json.Unmarshal(encoded, &literal); err != nil {
				t.Fatalf("json.Unmarshal(%s) a json.Number devolvió error: %v", encoded, err)
			}
			if literal.Hours != tt.want {
				t.Errorf("hours = %s, want %s", literal.Hours, tt.want)
			}
		})
	}
}
