package domain

import (
	defectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// DefectCount es la cantidad de defectos de un proyecto para un estado y una severidad.
type DefectCount struct {
	Status   defectsdomain.DefectStatus
	Severity defectsdomain.DefectSeverity
	Count    int
}

// DefectTally es un conteo de defectos con su desglose por severidad.
type DefectTally struct {
	Total      int
	BySeverity map[defectsdomain.DefectSeverity]int
}

// ProjectDefects son los defectos detectados, resueltos y abiertos de un proyecto.
type ProjectDefects struct {
	Detected DefectTally
	Resolved DefectTally
	Open     DefectTally
}

// newDefectTally crea un conteo en 0 con las cuatro severidades presentes.
func newDefectTally() DefectTally {
	return DefectTally{BySeverity: map[defectsdomain.DefectSeverity]int{
		defectsdomain.DefectSeverityCritical: 0,
		defectsdomain.DefectSeverityHigh:     0,
		defectsdomain.DefectSeverityMedium:   0,
		defectsdomain.DefectSeverityLow:      0,
	}}
}

// add suma una cantidad al total y a su severidad.
func (t *DefectTally) add(severity defectsdomain.DefectSeverity, count int) {
	t.Total += count
	t.BySeverity[severity] += count
}

// TallyDefects agrupa los conteos por estado y severidad en detectados, resueltos y abiertos.
func TallyDefects(counts []DefectCount) ProjectDefects {
	result := ProjectDefects{
		Detected: newDefectTally(),
		Resolved: newDefectTally(),
		Open:     newDefectTally(),
	}

	// Los descartados se ignoran; los resueltos y los abiertos suman a detectados.
	for _, c := range counts {
		switch c.Status {
		case defectsdomain.DefectStatusResolved:
			result.Resolved.add(c.Severity, c.Count)
			result.Detected.add(c.Severity, c.Count)
		case defectsdomain.DefectStatusOpen, defectsdomain.DefectStatusInProgress:
			result.Open.add(c.Severity, c.Count)
			result.Detected.add(c.Severity, c.Count)
		}
	}
	return result
}
