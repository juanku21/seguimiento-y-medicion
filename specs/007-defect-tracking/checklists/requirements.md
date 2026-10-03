# Checklist de Calidad de la Especificación: Gestión de Defectos

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

- [x] No quedan marcadores [NEEDS CLARIFICATION] — los 2 abiertos (FR-038 y FR-039) se resolvieron
      con el usuario el 2026-10-03
- [x] Los requisitos son verificables y no ambiguos (FR-001 a FR-056, numeración continua y sin
      saltos)
- [x] Los criterios de éxito son medibles (SC-001 a SC-020)
- [x] Los criterios de éxito son independientes de la tecnología
- [x] Todos los escenarios de aceptación están definidos (US1 a US6, 73 escenarios, cada historia con
      caso normal, alternativo, límite y de error)
- [x] Los casos límite están identificados (sección "Casos Límite", 21 casos, que incluyen los cinco
      pedidos en el enunciado)
- [x] El alcance está claramente delimitado (sección "Fuera de Alcance")
- [x] Dependencias y supuestos identificados (RC-01 a RC-10, con las dependencias de
      `specs/001-user-auth`, `specs/002-project-members`, `specs/003-product-backlog` y
      `specs/004-sprint-management`)

## Preparación de la Feature

- [x] Todos los requisitos funcionales tienen criterios de aceptación claros
- [x] Los escenarios de usuario cubren los flujos principales
- [x] La feature cumple con los resultados medibles definidos en Criterios de Éxito
- [x] No se filtran detalles de implementación en la especificación

## Notas

- Los ítems incompletos exigen actualizar la spec antes de `/speckit-clarify` o `/speckit-plan`.
- Iteración 1 de validación (2026-10-03, al redactar la spec): 15/16 ítems en verde. El ítem en rojo
  eran los dos marcadores [NEEDS CLARIFICATION] de FR-038 y FR-039, que se presentaron al usuario
  como preguntas.
- Iteración 2 (2026-10-03, al resolver ambos marcadores con el usuario): 16/16 ítems en verde.
  1. **Sprint de detección corregible solo mientras el defecto está Abierto** (FR-038, RN-16,
     SC-011). Alcanza a FR-033, FR-035, la tabla de modificación, los escenarios 5, 8, 11 y 12 de
     US4 y dos casos límite.
  2. **Estado terminal Descartado**, alcanzable desde Abierto y desde En progreso (FR-039, FR-017,
     FR-018, FR-036, FR-040, FR-054, RN-11, RN-12, RN-19, RN-21 a RN-23, RC-05, RC-09, SC-005,
     SC-010, SC-012). Se agregó **US6** con 9 escenarios, cuatro casos límite y cinco filas de la
     tabla de condiciones de error.
- Consecuencias de la decisión 1 que conviene tener presentes, documentadas en Supuestos y en Casos
  Límite: la ventana mira el estado actual, no si el defecto fue trabajado alguna vez, así que un
  defecto reabierto vuelve a admitir la corrección. Y un defecto Abierto nunca tiene sprint de
  resolución, por lo que la corrección jamás puede dejar los dos sprints en orden inválido: no hace
  falta una validación cruzada.
- Consecuencia de la decisión 2: un defecto descartado por error no se recupera. Se aceptó el costo
  porque volver a registrarlo es barato.
- Iteración 3 (2026-10-03, `/speckit-clarify`): 16/16 ítems en verde, sin regresiones. Se cerraron
  tres ambigüedades que el barrido detectó, ninguna de las cuales tenía marcador:
  3. **Consistencia bajo concurrencia** (FR-018, RN-24, SC-019): cada transición se valida contra el
     estado vigente al aplicarla, no contra el leído. Dos acciones incompatibles simultáneas nunca
     se aplican las dos. Sin esto, el carácter definitivo de Descartado se saltea con una carrera y
     una resolución se pierde sin rastro, porque el historial de auditoría está fuera de alcance.
  4. **Ventana única de corrección para los dos vínculos** (FR-033, FR-038, RN-16, SC-010, SC-011):
     la historia relacionada se congela igual que el sprint de detección, es decir, corregible solo
     con el defecto Abierto. Antes la historia se podía revincular incluso con el defecto Resuelto,
     lo que permitía mover la cuenta de defectos por historia hacia atrás.
  5. **Sin paginación** (FR-040 y sección "Fuera de Alcance"): los listados devuelven el conjunto
     completo, en línea con `specs/002-project-members` y `specs/003-product-backlog`.
- La decisión 4 reemplazó el escenario 2 de US4 y el 11, y agregó el escenario 6; la 3 agregó dos
  casos límite y una fila de condiciones de error. No quedó texto contradictorio: se verificó que
  no sobrevive la frase "la última acción válida es la que queda" ni la afirmación de que el sprint
  de detección "no cambia a lo largo de la vida del defecto".

### Diferidos, a resolver en la planificación

- **Objetivos de rendimiento y escala** (latencia de los listados, volumen esperado de defectos por
  proyecto): ausentes, igual que en las specs 001 a 006. Es una omisión consistente en todo el
  proyecto, no un hueco de esta feature, y depende de decisiones de infraestructura.
- **Observabilidad y disponibilidad**: ninguna spec del proyecto las define todavía.
- La cobertura del enunciado se verificó punto por punto:
  - Los 5 puntos de ALCANCE están cubiertos por US1 a US5. Los puntos 3 y 4 (estados y sprint de
    resolución) se unificaron en US2 porque no son testeables por separado: resolver exige indicar
    el sprint. La reapertura se separó en US5 por tener prioridad distinta. US6 (descarte) no
    estaba en el enunciado: surgió de la clarificación de FR-039.
  - Las 9 REGLAS DE NEGOCIO del enunciado están en RN-01 a RN-15.
  - Los 5 CASOS LÍMITE pedidos están en la sección "Casos Límite".
  - Las 7 CONDICIONES DE ERROR pedidas están en la tabla "Condiciones de Error".
  - Los 5 puntos de FUERA DE ALCANCE están en la sección homónima.

### Decisiones que el enunciado no fijaba y se resolvieron con supuesto

1. **Transición En progreso → Abierto rechazada** (FR-020). El enunciado enumera tres transiciones y
   no incluye esta. Se rechaza explícitamente en vez de admitirla en silencio, siguiendo el criterio
   de `specs/004-sprint-management` (su FR-038), que también rechaza todo lo no enumerado.
2. **Proyecto sin ningún sprint iniciado** (FR-010). El sprint de detección es obligatorio y no puede
   estar Planificado, así que un proyecto que nunca arrancó un sprint no admite defectos. Es una
   consecuencia de las reglas dadas, no una decisión nueva, pero conviene que esté explícita porque
   es un bloqueo real al empezar un proyecto.
3. **Orden de los listados** (FR-048): severidad descendente y después fecha de creación.
4. **Orden entre sprints por fecha de inicio prevista** (RC-07), apoyado en que los períodos de dos
   sprints del mismo proyecto no se superponen.

### Consistencia verificada con las specs anteriores

- `specs/002-project-members` FR-018 vuelve de solo lectura un proyecto Finalizado; recogido en
  FR-054 y RN-02.
- `specs/002-project-members` FR-027 conserva los defectos de un integrante dado de baja; recogido en
  FR-055, RN-20 y RC-03.
- `specs/003-product-backlog` FR-043 y RN-16 impiden eliminar una historia con defectos asociados;
  recogido en RN-19 y RC-05.
- `specs/004-sprint-management` define los tres estados de sprint y la no superposición de períodos
  (su RN-05); usados en FR-006, FR-024 y RC-07.
- `specs/004-sprint-management` FR-048 solo permite eliminar sprints Planificados, que nunca pueden
  ser ancla de un defecto; por eso esta feature no necesita condicionar esa eliminación, a
  diferencia de `specs/006-effort-tracking` (RC-08). Es una diferencia deliberada entre las dos
  features y conviene no "corregirla" por simetría.
- `specs/006-effort-tracking` ancla el esfuerzo a un sprint de forma inmutable; el supuesto
  provisional de FR-038 adopta el mismo criterio para el sprint de detección.

### Supuestos vigentes, candidatos a revisar en la planificación

1. Los defectos no tienen dueño: cualquier integrante los registra y los actualiza.
2. La reapertura no conserva historial: solo vale el sprint de la última resolución.
3. La severidad no tiene historial y se puede cambiar en cualquier estado.
4. No hay bloqueo ante edición concurrente: la última acción válida es la que queda.
5. Sin límite de defectos por historia ni por proyecto, y sin paginación, en línea con
   `specs/002-project-members` y `specs/003-product-backlog`, que declaran la paginación fuera de
   alcance.
