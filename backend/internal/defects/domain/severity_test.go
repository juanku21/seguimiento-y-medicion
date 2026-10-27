package domain

import "testing"

// Escenario: Las cuatro severidades se aceptan / severidad no admitida se rechaza, RN-04 (007/US1)
func TestUS1_SeveridadValida(t *testing.T) {
	cases := []struct {
		name  string
		value DefectSeverity
		valid bool
	}{
		{"critical", "critical", true},
		{"high", "high", true},
		{"medium", "medium", true},
		{"low", "low", true},
		{"urgent", "urgent", false},
		{"vacia", "", false},
		{"mayuscula", "Critical", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.value.Valid(); got != c.valid {
				t.Errorf("Valid(%q) = %v, se esperaba %v", c.value, got, c.valid)
			}
		})
	}
}

// Escenario: Las cuatro severidades (Crítica, Alta, Media y Baja) se aceptan, con su orden, RN-04 (007/US1)
func TestUS1_SeveridadRango(t *testing.T) {
	cases := []struct {
		value DefectSeverity
		rank  int
	}{
		{DefectSeverityCritical, 0},
		{DefectSeverityHigh, 1},
		{DefectSeverityMedium, 2},
		{DefectSeverityLow, 3},
	}
	for _, c := range cases {
		t.Run(string(c.value), func(t *testing.T) {
			if got := SeverityRank(c.value); got != c.rank {
				t.Errorf("SeverityRank(%q) = %d, se esperaba %d", c.value, got, c.rank)
			}
		})
	}
}
