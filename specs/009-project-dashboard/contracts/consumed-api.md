# Contrato de la API consumida: Dashboard del Proyecto

**Feature**: `specs/009-project-dashboard` | **Fecha**: 2026-10-28

Esta feature **no expone ni modifica ningún endpoint**. Por eso no tiene un `openapi.yaml` propio:
consume seis operaciones `GET` que ya están en `main`, todas con sesión (`Authorization: Bearer`) y
membresía vigente. La fuente de verdad de cada respuesta es el contrato de la feature dueña. Este
documento fija **qué operación alimenta cada sección y qué campos lee**, para que un cambio en esos
contratos se detecte acá.

| # | Operación | Contrato de origen | Sección del dashboard | Campos que lee |
| --- | --- | --- | --- | --- |
| 1 | `GET /api/v1/projects/{projectId}` | `specs/002-project-members/contracts/openapi.yaml` | Resumen (no degradable) | `name`, `status`, `startDate`, `plannedEndDate`, `actualEndDate`, `members`, `activeSprint.id` |
| 2 | `GET /api/v1/sprints/{sprintId}` | `specs/004-sprint-management/contracts/openapi.yaml` | Sprint activo | `name`, `goal`, `startDate`, `endDate`, `schedule.remainingDays`, `schedule.overdueDays` |
| 3 | `GET /api/v1/projects/{projectId}/sprints` | `specs/004-sprint-management/contracts/openapi.yaml` | Estado vacío del proyecto | `emptyReason` |
| 4 | `GET /api/v1/projects/{projectId}/metrics` | `specs/008-metrics-calculation/contracts/openapi.yaml` (`getProjectMetrics`) | Indicadores del proyecto, distribución, advertencias | `velocity.value`, `cumulative.*`, `defects.open`, `warnings` |
| 5 | `GET /api/v1/projects/{projectId}/metrics/active-sprint` | ídem (`getActiveSprintMetrics`) | Indicadores del sprint activo | `activeSprint`, `metrics.plannedStoryPoints`, `completedStoryPoints`, `estimatedHours`, `actualHours`, `deviationHours`, `deviationPercent`, `storyCompletionPercent` |
| 6 | `GET /api/v1/projects/{projectId}/metrics/series` | ídem (`getProjectSeries`) | Gráficos de velocidad, esfuerzo y calidad | Las seis series de `SeriesPoint` |

## Orden de las consultas

```text
1 ──► (si 200) ──┬──► 2   (solo si activeSprint no es null)
                 ├──► 3
                 ├──► 4
                 ├──► 5
                 └──► 6
```

La operación 1 va primero y sola, porque sin resumen no se muestra nada (FR-030). Las otras cinco
salen en paralelo y cada una falla por separado (FR-028).

## Respuestas que cambian lo que se muestra

| Operación | Respuesta | Qué hace el dashboard |
| --- | --- | --- |
| 1 | `404` (`{"error": "El proyecto no existe."}`) | Muestra el mensaje y no pide nada más. Mismo cuerpo para un proyecto inexistente y uno ajeno (FR-035) |
| 1 | `500`, red caída o cuerpo inesperado | "No se pudo mostrar el dashboard." con "Reintentar" (FR-030) |
| cualquiera | `401` | El cliente HTTP descarta la sesión y la guardia lleva a `/login` (FR-033) |
| 1 | `activeSprint: null` | El bloque del sprint activo dice "No hay ningún sprint en curso." sin pedir la operación 2 |
| 3 | `emptyReason: "no_sprints"` | Estado vacío del proyecto en lugar de indicadores, gráficos y distribución (FR-027) |
| 3 | `emptyReason: null` o `"no_matches"`, o falla | Secciones normales, cada una con su propio estado vacío |
| 5 | `{"activeSprint": false, "metrics": null}` | El grupo del sprint activo dice "No hay ningún sprint en curso." (FR-011) |
| 6 | Una serie vacía | El gráfico muestra su estado vacío (FR-024) |
| 6 | Una serie malformada | Solo el gráfico que la usa informa que no pudo cargarse ([data-model.md](../data-model.md), sección 3) |
| 2, 4, 5, 6 | `500`, red caída o cuerpo inesperado | Cada sección que depende de esa operación lo informa por separado (FR-029) |

## Lo que el dashboard nunca hace con estas respuestas

- No suma, promedia, resta, redondea ni reordena ninguna cifra (FR-002, FR-013, FR-026).
- No vuelve a filtrar defectos Descartados ni sprints Planificados (RC-07): ya vienen excluidos.
- No lee las `warnings` de las operaciones 5 y 6: las de la operación 4 ya las contienen.
- La única selección que hace es omitir los puntos con `sprintStatus: "active"` en el gráfico de
  velocidad (US3-2; [research.md](../research.md), sección 8).
