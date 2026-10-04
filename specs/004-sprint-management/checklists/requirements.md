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
      saltos; FR-048 enmendado el 2026-10-02)
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
- Iteración 4 (2026-10-02, enmienda derivada de `specs/006-effort-tracking`): 16/16 ítems en verde,
  sin regresiones. Un sprint Planificado que nunca fue iniciado ya no es eliminable sin más: la
  eliminación se rechaza mientras tenga registros de esfuerzo asociados, porque esos registros están
  anclados a él de forma inmutable y borrarlos destruiría horas reales. Alcanza a FR-048, RN-20,
  RC-10 (nuevo), SC-016, el escenario 14 de US7, la tabla de entradas y salidas de la eliminación,
  la de condiciones de error, los casos límite y el supuesto "Eliminar un sprint es definitivo".
- Iteración 5 (2026-10-03, enmienda derivada de `specs/009-project-dashboard`): 16/16 ítems en
  verde, sin regresiones. El detalle de un sprint Activo ahora incluye sus **días restantes** y la
  condición de **vencido**, con el día de fin incluido, de modo que el último día del sprint vale 0
  y nunca se devuelve un número negativo. Alcanza a FR-053, RN-23 (nueva), RC-11 (nueva) y la
  sección Clarifications. El motivo es que el dashboard tiene prohibido derivar valores por su
  cuenta, incluida la aritmética de fechas, así que el dato tiene que venir de donde vive el
  sprint. No cambia ninguna regla existente ni ningún escenario de las siete historias.
- Iteración 7 (2026-10-04, barrido de clarificación sobre las specs 001 a 010): 16/16 ítems en
  verde. Se cubrieron con escenarios de aceptación dos capacidades que las enmiendas habían
  agregado sin verificación: los **días restantes** del sprint activo y la lectura de un sprint
  cerrado **desde la instantánea** con título y prioridad congelados. Son cinco escenarios nuevos
  en US6 (actuales 4 a 8). Sin ellos, FR-053 y FR-043 eran requisitos que nadie iba a probar ni
  implementar, porque el tablero se arma desde las historias. **Esto cambia el cuerpo del issue
  #26** y exige volver a sincronizar la 004.
- Iteración 6 (2026-10-04, enmienda derivada de `specs/010-project-reports`): 16/16 ítems en verde,
  sin regresiones. La instantánea de cierre ahora congela, de cada historia involucrada, su
  **título**, su **prioridad** y sus **Story Points** al momento del cierre, además de su
  resultado. Alcanza a FR-043, RN-15, RC-12 (nueva) y la sección Clarifications. El motivo es que
  el reporte de un sprint cerrado arma su sección de historias solo con la instantánea: sin el
  título y la prioridad habría tenido que leerlos del backlog vigente y el documento podría
  mostrar una historia renombrada o reestimada junto a métricas calculadas sobre los valores
  viejos. No cambia ningún escenario de las siete historias de usuario.
- Iteración 3 (2026-09-25, `/speckit-clarify`): 16/16 ítems en verde, sin regresiones. Se cerraron
  tres ambigüedades nuevas que el barrido detectó:
  4. La instantánea guarda los Story Points comprometidos al inicio y los planificados al cierre; su
     diferencia es el cambio de alcance (FR-028, FR-043, RN-15, SC-009).
  5. Una historia comprometida en un sprint abierto no puede quedar sin Story Points ni sin criterios
     de aceptación (FR-018, RN-22, RC-05).
  6. La instantánea congela las cuatro fechas del sprint: inicio y fin previstas, inicio y cierre
     reales (FR-043, FR-051).

### ✅ Alineado con `specs/003-product-backlog` (verificado el 2026-10-04)

La decisión 5 imponía una restricción sobre operaciones que la spec 003 definía sin condiciones, y
quedó pendiente de reflejarse allá. El barrido del 2026-10-04 verificó que **ya está resuelta**:

- Su FR-018 condiciona la vuelta a "sin estimar" a que la historia no esté comprometida en un
  sprint abierto.
- Su FR-022 rechaza dejar sin criterios a una historia comprometida en un sprint abierto.
- El escenario 4 de su US5 vacía la lista de criterios solo en una historia "que no está
  comprometida en ningún sprint".

Las dos specs dicen lo mismo. La advertencia anterior había quedado sin actualizar después de que
la spec 003 se enmendara.

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
