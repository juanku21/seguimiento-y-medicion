# Checklist de Calidad de la Especificación: Reportes de Proyecto y de Sprint

**Propósito**: validar que la especificación esté completa y sea de calidad suficiente antes de pasar
a la planificación
**Creado**: 2026-10-04
**Feature**: [spec.md](../spec.md)

## Calidad del Contenido

- [x] Sin detalles de implementación (lenguajes, frameworks, librerías de PDF, APIs)
- [x] Centrada en el valor para el usuario y las necesidades del negocio
- [x] Redactada para personas no técnicas
- [x] Todas las secciones obligatorias completadas

## Completitud de los Requisitos

- [x] No quedan marcadores [NEEDS CLARIFICATION] — los 3 abiertos se resolvieron con el usuario el
      2026-10-04
- [x] Los requisitos son verificables y no ambiguos (FR-001 a FR-041, numeración continua y sin
      saltos)
- [x] Los criterios de éxito son medibles (SC-001 a SC-019)
- [x] Los criterios de éxito son independientes de la tecnología
- [x] Todos los escenarios de aceptación están definidos (US1 a US4, 35 escenarios, cada historia con
      caso normal, alternativo, límite y de error)
- [x] Los casos límite están identificados (sección "Casos Límite", 16 casos, que incluyen los seis
      pedidos en el enunciado)
- [x] El alcance está claramente delimitado (sección "Fuera de Alcance")
- [x] Dependencias y supuestos identificados (RC-01 a RC-11, con las dependencias de las specs 001 a
      008; es la feature con más dependencias del producto)

## Preparación de la Feature

- [x] Todos los requisitos funcionales tienen criterios de aceptación claros
- [x] Los escenarios de usuario cubren los flujos principales
- [x] La feature cumple con los resultados medibles definidos en Criterios de Éxito
- [x] No se filtran detalles de implementación en la especificación

## Notas

- Los ítems incompletos exigen actualizar la spec antes de `/speckit-clarify` o `/speckit-plan`.
- Iteración 1 de validación (2026-10-04, al redactar la spec): 15/16 ítems en verde. El ítem en rojo
  son los tres marcadores [NEEDS CLARIFICATION], que se presentaron al usuario como preguntas.
- La cobertura del enunciado se verificó punto por punto:
  - Los 4 puntos de ALCANCE están cubiertos: US1 (generar y ver el de sprint), US3 (generar y ver el
    de proyecto), US2 (exportar a PDF). La vista en pantalla no se separó en una historia propia
    porque no es testeable sin generar antes el reporte.
  - Los 6 puntos de CONTENIDO MÍNIMO están en FR-009 a FR-018, uno por sección.
  - Las 5 REGLAS DE NEGOCIO del enunciado están en RN-01 a RN-06, RN-15 a RN-17 y RN-19.
  - Los 6 CASOS LÍMITE pedidos están en la sección "Casos Límite".
  - Las 3 CONDICIONES DE ERROR pedidas están en la tabla "Condiciones de Error".
  - Los 5 puntos de FUERA DE ALCANCE están en la sección homónima.

- Iteración 2 (2026-10-04, al resolver los tres marcadores con el usuario): 16/16 ítems en verde.
  1. **Un reporte de sprint cerrado sale íntegramente de la instantánea** (FR-021, FR-022, RN-22,
     RN-23, SC-018). Se enmendó `specs/004-sprint-management` para que la instantánea congele
     también el título y la prioridad de cada historia involucrada (su FR-043, RN-15 y RC-12). Con
     eso, dos reportes del mismo sprint cerrado son idénticos y la sección de historias nunca
     contradice a la de métricas.
  2. **Se guarda el registro de generación, no el documento** (FR-034, FR-035, RN-24, SC-019).
  3. **El reporte de proyecto incluye un grupo de historias no planificadas** (FR-036, RN-25), con
     la misma lógica con que `specs/008-metrics-calculation` informa el esfuerzo fuera de sprint.
- Consecuencia de la decisión 2 que obligó a corregir una regla: **RN-02 era falsa**. Decía que
  generar un reporte no modifica ningún dato, y ahora sí escribe su registro de generación. Se
  reformuló, y FR-035 aclara que ese registro no vulnera la condición de solo lectura de un
  proyecto Finalizado, porque es el rastro de una lectura y no un dato del proyecto.

### Las tres preguntas abiertas y por qué importaban

1. **FR-021 — un reporte de sprint cerrado puede contradecirse a sí mismo.** Las métricas salen de
   la instantánea de cierre (`specs/008-metrics-calculation`, FR-013), pero el título, la prioridad
   y los Story Points de las historias viven en el backlog y pueden haber cambiado. Un reporte
   generado hoy podría listar una historia con 13 Story Points y, al lado, métricas calculadas sobre
   los 5 que tenía al cerrarse. La instantánea de la spec 004 conserva las historias involucradas,
   sus Story Points y su resultado, pero no el título ni la prioridad, así que ninguna de las dos
   fuentes alcanza por sí sola.
2. **FR-033 — "archivar" sugiere persistencia, pero nada la define.** Si los reportes no se guardan,
   dos reportes del mismo sprint generados en fechas distintas pueden diferir y nadie podrá
   reconstruir el que se compartió con un cliente.
3. **FR-034 — el reporte de proyecto agrupa por sprint y el backlog no tiene grupo.** Una historia
   que nunca entró a un sprint no pertenece a ninguno, así que quedaría fuera del documento. Es el
   mismo patrón que apareció en `specs/008-metrics-calculation` con el esfuerzo fuera de sprint, que
   allí se resolvió informándolo por separado.

### Decisiones que el enunciado no fijaba y se resolvieron con supuesto

1. **Solo las sesiones Finalizadas de Planning Poker aportan resumen** (FR-013, RN-09). Una sesión
   Cancelada no fijó ningún valor; informarla como estimación sería engañoso.
2. **Cuando una historia tuvo varias sesiones Finalizadas, vale la última** (FR-014, RN-09), que es
   la que fijó el valor vigente de la historia.
3. **El listado de defectos excluye los Descartados** (RC-09). Es la única forma de que el listado
   cuadre con sus totales, que la feature de métricas calcula ya excluidos.
4. **Una sección que no pudo obtenerse se distingue de una sección sin datos** (FR-023, RN-12). No
   tener horas cargadas y no poder leerlas significan cosas distintas, y confundirlas haría que un
   fallo pareciera un proyecto sin actividad.
5. **El reporte no muestra nada que un integrante no vería en el sistema** (RC-11). Un PDF
   descargado no tiene control de acceso.

### Consistencia verificada con las specs anteriores

- `specs/008-metrics-calculation` es la única fuente de cifras, igual que para
  `specs/009-project-dashboard` (RC-08). Con esta feature ya son dos los consumidores, lo que
  refuerza que ninguna de las dos recalcule.
- `specs/008-metrics-calculation` define cinco estados de valor; el reporte los conserva sin
  aplanarlos (FR-020, RN-14), con el mismo criterio que el dashboard.
- `specs/005-planning-poker` distingue sesiones Finalizadas de Canceladas y solo las primeras tienen
  valor acordado; recogido en FR-013 a FR-015 y RC-05.
- `specs/006-effort-tracking` conserva el nombre de quien cargó horas aunque ya no sea integrante;
  es lo que mantiene legible un reporte histórico (RC-06).
- `specs/004-sprint-management` congela la instantánea al cerrar, que es lo que vuelve reproducible
  un reporte de sprint cerrado (RC-04) y lo que origina la pregunta 1.
- `specs/007-defect-tracking` excluye los Descartados de los conteos; recogido en RC-09.
