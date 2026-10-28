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

// TallyDefects agrupa los conteos por estado y severidad en detectados, resueltos y abiertos.
func TallyDefects(counts []DefectCount) ProjectDefects {
	return ProjectDefects{}
}
