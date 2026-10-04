# Checklist de Calidad de la Especificación: Cálculo de Métricas

**Propósito**: validar que la especificación esté completa y sea de calidad suficiente antes de pasar
a la planificación
**Creado**: 2026-10-03
**Feature**: [spec.md](../spec.md)

## Calidad del Contenido

- [x] Sin detalles de implementación (lenguajes, frameworks, APIs)
- [x] Centrada en el valor para el usuario y las necesidades del negocio
- [x] Redactada para personas no técnicas
- [x] Todas las secciones obligatorias completadas

## Completitud de los Requisitos

- [x] No quedan marcadores [NEEDS CLARIFICATION] — los 3 abiertos (FR-019, FR-029 y FR-036) se
      resolvieron con el usuario el 2026-10-03
- [x] Los requisitos son verificables y no ambiguos (FR-001 a FR-043, numeración continua y sin
      saltos)
- [x] Los criterios de éxito son medibles (SC-001 a SC-021)
- [x] Los criterios de éxito son independientes de la tecnología
- [x] Todos los escenarios de aceptación están definidos (US1 a US5, 55 escenarios, cada historia con
      caso normal, alternativo, límite y de error)
- [x] Los casos límite están identificados (sección "Casos Límite", 19 casos, que incluyen los ocho
      pedidos en el enunciado)
- [x] El alcance está claramente delimitado (sección "Fuera de Alcance")
- [x] Dependencias y supuestos identificados (RC-01 a RC-11, con las dependencias de las specs 001,
      002, 003, 004, 006 y 007)

## Preparación de la Feature

- [x] Todos los requisitos funcionales tienen criterios de aceptación claros
- [x] Los escenarios de usuario cubren los flujos principales
- [x] La feature cumple con los resultados medibles definidos en Criterios de Éxito
- [x] No se filtran detalles de implementación en la especificación

## Notas

- Los ítems incompletos exigen actualizar la spec antes de `/speckit-clarify` o `/speckit-plan`.
- Iteración 1 de validación (2026-10-03, al redactar la spec): 15/16 ítems en verde. El ítem en rojo
  eran los tres marcadores [NEEDS CLARIFICATION], que se presentaron al usuario como preguntas.
- Iteración 2 (2026-10-03, al resolver los tres con el usuario): 16/16 ítems en verde.
  1. **Estado por métrica en los sprints cerrados** (FR-019, RN-23, RN-24, SC-015). Cuatro métricas
     siempre definitivas, tres provisorias hasta que vence el plazo de gracia, y los dos conteos de
     defectos siempre provisorios. Obligó a separar dos términos que podían confundirse: "parcial"
     (todo el conjunto de un sprint Activo) y "provisoria" (una métrica puntual de uno Cerrado).
  2. **Esfuerzo fuera de sprint como métrica propia** (FR-029, RN-25, SC-016). No se suma al
     acumulado, así que los acumulados siguen cuadrando exactamente con la suma de los sprints y
     ninguna hora real queda invisible.
  3. **Historias sin estimar contadas en el backlog** (FR-036, FR-037, RN-26, RC-05, SC-017). Se
     informa a nivel de proyecto y nunca por sprint; FR-037 deja asentado que una historia sin
     estimar dentro de un sprint sería una inconsistencia de datos, no un caso normal.
- Consecuencia aceptada de la decisión 1: los conteos de defectos de un sprint cerrado no llegan a
  ser definitivos nunca, porque la spec 007 permite registrar defectos contra sprints cerrados sin
  límite de tiempo. La alternativa era ponerle un plazo también a los defectos, que nadie pidió.
- Iteración 3 (2026-10-03, `/speckit-clarify`): 16/16 ítems en verde, sin regresiones. Se cerraron
  tres ambigüedades que el barrido detectó, ninguna de las cuales tenía marcador:
  4. **El acumulado del proyecto se marca como parcial cuando incluye al sprint activo** (FR-023,
     RN-27, RN-28, SC-018). Era un hueco abierto por la decisión 1: si una métrica de sprint
     declara si puede moverse, un total que mezcla sprints cerrados con uno en curso también tiene
     que hacerlo. Se precisó además que la velocidad nunca es parcial, porque solo mira cerrados.
  5. **Coherencia interna de cada respuesta** (FR-003, RN-29, SC-019). Todas las métricas de una
     consulta corresponden a un único instante. FR-003 ya exigía determinismo, pero eso solo
     garantizaba que dos consultas iguales coincidieran, no que una sola fuera coherente consigo
     misma: sin esta regla podían salir Story Points completados mayores que los planificados.
  6. **Advertencias en la respuesta** (FR-037, FR-038, FR-039, RN-30, RN-31, SC-020). Ante una
     inconsistencia de datos el sistema calcula igual y avisa, en vez de calcular en silencio o
     rechazar la consulta. Se formalizaron dos clases de advertencia —de planificación y de
     inconsistencia de datos— para que el contador de historias sin estimar y un dato roto no se
     confundan.

- Iteración 4 (2026-10-03, enmienda derivada de `specs/009-project-dashboard`): 16/16 ítems en
  verde, sin regresiones. FR-022 pasó de definir una sola serie —la de velocidad, solo con Story
  Points completados— a definir una métrica de **series del proyecto** que entrega seis series
  juntas: Story Points planificados y completados, horas estimadas y reales, y defectos detectados
  y resueltos, todas por sprint, en orden cronológico y con el estado de valor de cada punto. La
  serie de velocidad sigue consultable por separado, restringida a sprints cerrados, porque es la
  que alimenta FR-020. Alcanza a FR-022, RC-12 (nueva) y la sección Clarifications.
- Motivo de la enmienda: `specs/009-project-dashboard` tiene prohibido armar series por su cuenta.
  Si hubiera tenido que construir cinco de las seis consultando sprint por sprint, habría pasado a
  ser dueño del orden de los gráficos, que es justo lo que la regla de fuente única evita.
- La enmienda no toca ninguna fórmula ni ningún resultado numérico, así que los escenarios con datos
  concretos de la tabla de cobertura siguen siendo válidos sin cambios.

### Vocabulario de estados, ya consolidado

La spec maneja cinco calificadores distintos y la revisión verificó que no se solapan:

| Término | Qué califica | Dónde |
| --- | --- | --- |
| **Parcial** | Todo el conjunto de métricas de un sprint Activo, y el acumulado que lo incluye | FR-014, FR-023 |
| **Provisoria** | Una métrica puntual de un sprint Cerrado que todavía puede moverse | FR-019 |
| **Definitiva** | Una métrica que ya no cambia | FR-019, FR-023 |
| **No calculable** | Un cociente cuyo denominador es 0 | FR-030, FR-031 |
| **No aplicable** | Una métrica sin sentido para el estado del sprint consultado | FR-015, FR-035 |

### Requisito especial del enunciado: cada fórmula con datos numéricos

El enunciado exigía que cada fórmula tuviera al menos un escenario con datos concretos y resultado
exacto. Cobertura verificada, con un único proyecto de ejemplo de factor 6 para que los números se
encadenen entre historias:

| Fórmula | Escenario | Resultado exacto |
| --- | --- | --- |
| Story Points planificados | US1-1 | 21 |
| Story Points completados | US1-1 | 13 |
| Horas estimadas | US1-1 | 126 = 21 × 6 |
| Horas reales | US1-1 | 142,5 |
| Desviación absoluta (positiva) | US1-1 | +16,5 h |
| Desviación porcentual (positiva) | US1-1 | +13,10 % (13,0952…) |
| Desviación absoluta y porcentual (negativas) | US1-2 | −16 h y −12,70 % |
| Porcentaje de historias completadas | US1-1, US1-8 | 50,00 % y 33,33 % |
| Defectos detectados y resueltos por sprint | US5-1 | 7 y 4 |
| Velocidad con 1, 2, 3 y 4 cerrados | US2-3, US2-4, US2-1, US2-2 | 13,00 / 17,00 / 14,00 / 16,33 |
| Serie de velocidad | US2-5 | 13, 21, 8, 20 |
| Acumulados del proyecto | US3-1 | 63 SP, 39 SP, 378 h, 312,5 h, −65,5 h, −17,33 % |
| Porcentaje acumulado de historias | US3-2 | 63,64 % (63,6363…) |
| Métricas de sprint activo | US4-1 | 108 h, −68 h, −62,96 % |
| Defectos del proyecto con severidad | US5-2 | 12 / 5 / 7 |

Los casos con decimal periódico (33,33 %, 16,33, 63,64 %, −62,96 %) están elegidos a propósito para
que las pruebas detecten un redondeo intermedio indebido.

### ⚠️ Contradicciones detectadas contra specs anteriores

Las tres preguntas abiertas no son huecos del enunciado: son choques reales con specs ya escritas.

1. **FR-019 — "las métricas de un sprint cerrado nunca cambian" es falso para 3 de las 8.** La
   instantánea de `specs/004-sprint-management` (FR-043) congela historias, Story Points y factor,
   pero no las horas reales ni los defectos. El esfuerzo de un sprint cerrado puede crecer durante su
   plazo de gracia (`specs/006-effort-tracking`, FR-016) y un defecto nuevo puede nombrar un sprint
   cerrado como sprint de detección (`specs/007-defect-tracking`, FR-006). Además,
   `specs/006-effort-tracking` (RC-10) ya ordena a esta feature tratar ese esfuerzo como provisorio,
   que es el supuesto adoptado.
2. **FR-029 — el acumulado del proyecto no ve el esfuerzo fuera de sprint.**
   `specs/006-effort-tracking` admite horas sobre historias que están solo en el backlog, sin sprint
   asociado. Como el acumulado se define como suma de sprints, esas horas quedan invisibles y las
   horas reales del proyecto quedan subestimadas.
3. **FR-036 — la advertencia de historias "sin estimar" es inalcanzable dentro de un sprint.**
   `specs/004-sprint-management` (FR-018) solo admite historias listas para planificar, es decir con
   Story Points asignados, de modo que el contador pedido sería siempre 0 en las métricas por sprint.

### Consistencia verificada con las specs anteriores

- `specs/004-sprint-management` FR-043 congela la instantánea; es la base de FR-013 y RC-04.
- `specs/006-effort-tracking` FR-015 vuelve inmutable el ancla de cada registro al sprint; eso hace
  estable el esfuerzo por sprint aunque las historias se replanifiquen (RC-06).
- `specs/007-defect-tracking` RC-09 obliga a excluir los defectos Descartados; recogido en FR-018.
- `specs/007-defect-tracking` FR-029 borra el sprint de resolución al reabrir; es la causa de RN-14.
- `specs/002-project-members` define el factor con hasta 2 decimales, lo que garantiza que las horas
  estimadas no necesiten redondeo propio.

### Supuestos vigentes, candidatos a revisar en la planificación

1. "Los últimos 3 sprints cerrados" se ordenan por momento real de cierre.
2. La serie de velocidad se ordena por fecha de inicio prevista.
3. "Defecto abierto" agrupa los estados Abierto y En progreso.
4. Las métricas se calculan en cada consulta y no se almacenan resultados precalculados.
5. Los conteos de defectos nunca son "no calculable", porque contar no divide.
