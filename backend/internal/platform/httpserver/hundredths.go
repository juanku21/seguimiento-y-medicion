package httpserver

import "fmt"

// Hundredths es una magnitud en centésimas (de hora o de punto porcentual) que la API serializa como
// número JSON con exactamente 2 decimales (research.md sección 5; SC-015 y SC-022). Un *Hundredths
// nulo representa "no calculable" y se serializa null.
type Hundredths int

// MarshalJSON escribe el entero de centésimas como número con 2 decimales y signo. El signo se separa
// de la magnitud para no perderlo cuando la parte entera es cero (-5 se escribe -0.05); la magnitud se
// toma como uint para que el valor mínimo de int no desborde al cambiarle el signo.
func (h Hundredths) MarshalJSON() ([]byte, error) {
	sign := ""
	magnitude := uint(h)
	if h < 0 {
		sign = "-"
		magnitude = uint(-int(h))
	}
	return fmt.Appendf(nil, "%s%d.%02d", sign, magnitude/100, magnitude%100), nil
}
