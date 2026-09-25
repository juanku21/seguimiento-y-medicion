# Checklist de Calidad de la Especificación: Gestión de Sprints

**Propósito**: validar que la especificación esté completa y sea de calidad suficiente antes de pasar
a la planificación
**Creado**: 2026-09-25
**Feature**: [spec.md](../spec.md)

## Calidad del Contenido

- [x] Sin detalles de implementación (lenguajes, frameworks, APIs)
- [x] Centrada en el valor para el usuario y las necesidades del negocio
- [x] Redactada para personas no técnicas
- [x] Todas las secciones obligatorias completadas

## Completitud de los Requisitos

- [x] No quedan marcadores [NEEDS CLARIFICATION] — los 3 que quedaban (FR-025, FR-029 y FR-048) se
      resolvieron con el usuario el 2026-09-25
- [x] Los requisitos son verificables y no ambiguos (FR-001 a FR-060, numeración continua y sin
      saltos)
- [x] Los criterios de éxito son medibles (SC-001 a SC-017)
- [x] Los criterios de éxito son independientes de la tecnología
- [x] Todos los escenarios de aceptación están definidos (US1 a US7, 75 escenarios, cada historia con
      caso normal, alternativo, límite y de error)
- [x] Los casos límite están identificados (sección "Casos Límite", 22 casos)
- [x] El alcance está claramente delimitado (sección "Fuera de Alcance")
- [x] Dependencias y supuestos identificados (RC-01 a RC-09, con las dependencias de
      `specs/001-user-auth`, `specs/002-project-members` y `specs/003-product-backlog`)

## Preparación de la Feature

- [x] Todos los requisitos funcionales tienen criterios de aceptación claros
- [x] Los escenarios de usuario cubren los flujos principales
- [x] La feature cumple con los resultados medibles definidos en Criterios de Éxito
- [x] No se filtran detalles de implementación en la especificación

## Notas

- Los ítems incompletos exigen actualizar la spec antes de `/speckit-clarify` o `/speckit-plan`.
- Iteración 1 de validación (2026-09-25, al redactar la spec): 13/16 ítems en verde. Los 3 en rojo
  dependían de los marcadores de FR-025, FR-029 y FR-048.
- Iteración 2 (2026-09-25, al resolver esos tres marcadores con el usuario): 16/16 ítems en verde.
  1. Ninguna acción sobre los sprints cambia el estado del proyecto (FR-029, FR-049, RN-07).
  2. Un sprint Planificado que nunca fue iniciado puede eliminarse (FR-048, RN-20).
  3. Una historia Completada no puede quitarse de su sprint (FR-025, RN-21).
- Iteración 3 (2026-09-25, `/speckit-clarify`): 16/16 ítems en verde, sin regresiones. Se cerraron
  tres ambigüedades nuevas que el barrido detectó:
  4. La instantánea guarda los Story Points comprometidos al inicio y los planificados al cierre; su
     diferencia es el cambio de alcance (FR-028, FR-043, RN-15, SC-009).
  5. Una historia comprometida en un sprint abierto no puede quedar sin Story Points ni sin criterios
     de aceptación (FR-018, RN-22, RC-05).
  6. La instantánea congela las cuatro fechas del sprint: inicio y fin previstas, inicio y cierre
     reales (FR-043, FR-051).

### ⚠️ Pendiente de alinear con `specs/003-product-backlog`

La decisión 5 impone una restricción sobre operaciones que la spec 003 define sin condiciones:

- La spec 003 (FR-018) permite devolver una historia a "sin estimar" sin restricciones, y su US5
  escenario 4 muestra como válido vaciar la lista de criterios de aceptación.
- Con esta spec, esas dos operaciones se rechazan mientras la historia esté comprometida en un sprint
  abierto.

Conviene enmendar la spec 003 para que su FR-018 y su US5 remitan a esta condición. Eso alcanzaría a
los issues #17 y #19 de GitHub (003-US3 y 003-US5). Mientras no se haga, las dos specs se contradicen
sobre el mismo caso.

### Consistencia verificada con las specs anteriores

- `specs/003-product-backlog` FR-027 establece que los cambios de estado de una historia los produce
  esta feature; acá se cumple en FR-033 a FR-038 (RC-03).
- `specs/002-project-members` exige que no haya sprint activo para finalizar un proyecto; recogido en
  RC-04.
- `specs/002-project-members` congela el factor de horas por Story Point al cerrar un sprint (su
  RN-15); recogido en FR-043 y RN-15 de esta spec.
- `specs/003-product-backlog` impide eliminar una historia que estuvo en un sprint; recogido en RC-06
  y verificado en US7, incluso cuando el sprint se elimina después.
- El conflicto entre el enunciado original (iniciar un sprint pasaba el proyecto a En curso) y la
  spec 002 (FR-047, que reserva el cambio de estado al propietario) se resolvió desacoplando ambas
  cosas dentro de esta spec, sin enmendar la 002.

### Supuestos vigentes, candidatos a revisar en la planificación

1. La no superposición de fechas alcanza también a los sprints Cerrados.
2. El inicio y el cierre son siempre manuales; el sistema no actúa por calendario.
3. Las historias quitadas del sprint antes del cierre no figuran en su instantánea ni en su historial.
4. No hay límite de sprints por proyecto ni de historias por sprint.
