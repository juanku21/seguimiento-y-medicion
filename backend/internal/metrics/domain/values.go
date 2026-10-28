// Package domain declara los tipos de valor, las fórmulas puras y las interfaces del módulo de
// métricas; no depende de ninguna otra capa.
package domain

// ValueStatus indica si una métrica tiene valor y, si no, el motivo.
type ValueStatus string

const (
	// ValueAvailable indica que la métrica tiene valor.
	ValueAvailable ValueStatus = "available"
	// ValueNotCalculable indica que el denominador es 0 (FR-030).
	ValueNotCalculable ValueStatus = "not_calculable"
	// ValueNotApplicable indica que la métrica no tiene sentido para el estado del sprint (FR-015).
	ValueNotApplicable ValueStatus = "not_applicable"
)

// Finality indica cuán definitivo es el valor de una métrica de sprint o del acumulado.
type Finality string

const (
	// FinalityDefinitive indica un valor que ya no cambiará.
	FinalityDefinitive Finality = "definitive"
	// FinalityProvisional indica un valor que todavía puede cambiar.
	FinalityProvisional Finality = "provisional"
	// FinalityPartial indica un valor construido con datos parciales.
	FinalityPartial Finality = "partial"
)

// Metric es un valor con su estado.
type Metric struct {
	Status ValueStatus
	// Value solo es significativo con available. Su unidad depende del campo: Story Points,
	// o centésimas de hora o de punto porcentual.
	Value int
	// Finality solo se informa en métricas de sprint y en puntos de serie; es nil en el Planificado.
	Finality *Finality
}
