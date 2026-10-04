# Checklist de Calidad de la Especificación: Dashboard del Proyecto

**Propósito**: validar que la especificación esté completa y sea de calidad suficiente antes de pasar
a la planificación
**Creado**: 2026-10-03
**Feature**: [spec.md](../spec.md)

## Calidad del Contenido

- [x] Sin detalles de implementación (lenguajes, frameworks, librerías de gráficos, APIs)
- [x] Centrada en el valor para el usuario y las necesidades del negocio
- [x] Redactada para personas no técnicas
- [x] Todas las secciones obligatorias completadas

## Completitud de los Requisitos

- [x] No quedan marcadores [NEEDS CLARIFICATION] — los 3 abiertos (FR-004, FR-026 y FR-032) se
      resolvieron con el usuario el 2026-10-03
- [x] Los requisitos son verificables y no ambiguos (FR-001 a FR-036, numeración continua y sin
      saltos)
- [x] Los criterios de éxito son medibles (SC-001 a SC-016)
- [x] Los criterios de éxito son independientes de la tecnología
- [x] Todos los escenarios de aceptación están definidos (US1 a US6, 45 escenarios, cada historia con
      caso normal, alternativo, límite y de error)
- [x] Los casos límite están identificados (sección "Casos Límite", 16 casos, que incluyen los cinco
      pedidos en el enunciado)
- [x] El alcance está claramente delimitado (sección "Fuera de Alcance")
- [x] Dependencias y supuestos identificados (RC-01 a RC-10, con las dependencias de las specs 001,
      002, 004, 007 y 008)

## Preparación de la Feature

- [x] Todos los requisitos funcionales tienen criterios de aceptación claros
- [x] Los escenarios de usuario cubren los flujos principales
- [x] La feature cumple con los resultados medibles definidos en Criterios de Éxito
- [x] No se filtran detalles de implementación en la especificación

## Notas

- Los ítems incompletos exigen actualizar la spec antes de `/speckit-clarify` o `/speckit-plan`.
- Iteración 1 de validación (2026-10-03, al redactar la spec): 15/16 ítems en verde. El ítem en rojo
  son los tres marcadores [NEEDS CLARIFICATION], que se presentaron al usuario como preguntas.
- La cobertura del enunciado se verificó punto por punto:
  - Los 6 puntos de CONTENIDO están cubiertos: US1 (resumen), US2 (indicadores), US3 (gráfico de
    velocidad), US4 (gráfico de esfuerzo) y US5 (gráfico de calidad más distribución por severidad).
    US6 no estaba en el enunciado: agrupa los estados vacíos y la degradación por sección, que el
    enunciado pedía como casos límite y de error.
  - Las 6 REGLAS DE NEGOCIO del enunciado están en RN-01, RN-03, RN-04, RN-07, RN-08, RN-17 y RN-18.
  - Los 5 CASOS LÍMITE pedidos están en la sección "Casos Límite".
  - Las 2 CONDICIONES DE ERROR pedidas están en la tabla "Condiciones de Error".
  - Los 5 puntos de FUERA DE ALCANCE están en la sección homónima.
  - El criterio de éxito sugerido en el enunciado ("tiempo para identificar el estado del sprint
    activo") es SC-001, con un umbral de 10 segundos.

- Iteración 2 (2026-10-03, al resolver los tres marcadores con el usuario): 16/16 ítems en verde.
  Dos de las tres respuestas exigieron enmendar otras specs, y las enmiendas se aplicaron el mismo
  día:
  1. **Las series llegan armadas desde la 008** (FR-026, RN-20, RC-06). Se incorporó a
     `specs/008-metrics-calculation` una métrica de series del proyecto: su FR-022 pasó de definir
     solo la serie de velocidad a entregar las seis series juntas, ordenadas y con el estado de
     valor de cada punto. Allí quedó registrado en su sección Clarifications y en su RC-12.
  2. **El dashboard no deriva nada, ni fechas** (FR-004, FR-008 a FR-010, RN-01, RC-03). Se enmendó
     `specs/004-sprint-management`: su FR-053 ahora entrega los días restantes del sprint activo y
     la condición de vencido, con el día de fin incluido. Allí quedó en su RN-23 y su RC-11.
  3. **Las advertencias se muestran** (FR-032, RN-21), en sus dos clases y distinguidas entre sí.
- La decisión 2 endureció la regla central de la feature: RN-01 pasó de "el dashboard no calcula
  métricas" a "el dashboard no calcula ni deriva nada". La regla vale por ser absoluta; una
  excepción "porque es trivial" es la grieta por la que después entran otras.

### ⚠️ Efecto de las enmiendas sobre el tablero de GitHub

`specs/008-metrics-calculation` tiene sus historias publicadas como issues #47 a #51. La enmienda
cambió su FR-022, que no forma parte del cuerpo de ninguna historia de usuario, así que en
principio los issues no quedan desactualizados. Conviene confirmarlo con una corrida de
`/sincronizar-github historias 008` en modo plan: si el plan sale todo en SIN CAMBIOS, no hay nada
que hacer. Lo mismo vale para `specs/004-sprint-management` y sus issues #21 a #27.

### Huecos detectados contra `specs/008-metrics-calculation` (resueltos)

Los tres marcadores nacieron del choque entre la regla "el dashboard no calcula nada" y lo que la
feature de métricas entregaba antes de la enmienda.

1. **FR-026 — faltan cinco de las seis series que los gráficos necesitan.** La spec 008 expone una
   sola serie, la de velocidad, y además solo con los Story Points **completados** (su FR-022). Los
   gráficos piden además: Story Points planificados por sprint, horas estimadas por sprint, horas
   reales por sprint, defectos detectados por sprint y defectos resueltos por sprint. Es el hueco
   más grande de los tres y el que más condiciona el diseño.
2. **FR-004 — los días restantes no son una métrica.** Salen de restar la fecha de fin prevista del
   sprint a la fecha de hoy. Es aritmética de calendario, no una fórmula de negocio, pero sigue
   siendo un cálculo y la regla del enunciado no admite excepciones explícitas.
3. **FR-032 — las advertencias de la spec 008 no están en el enunciado del dashboard.** Esa feature
   produce advertencias de planificación y de inconsistencia de datos (su FR-038) justamente para
   que alguien las vea, y el dashboard es su consumidor natural.

### Decisiones que el enunciado no fijaba y se resolvieron con supuesto

1. **El resumen del proyecto es la única sección no degradable** (FR-030, RN-16). El enunciado pide
   que una falla no tumbe el resto, pero mostrar cifras sin saber de qué proyecto son es peor que no
   mostrar nada.
2. **Los días restantes incluyen el día de fin** (FR-009): un sprint que termina hoy muestra 0.
3. **Un sprint vencido lo informa en palabras** en vez de un número negativo (FR-010, RN-14).
4. **Sin paginación ni recorte de sprints en los gráficos**, en línea con el resto del producto. La
   legibilidad con muchos sprints se resuelve en el diseño y se mide en SC-009 con 20 sprints.

### Consistencia verificada con las specs anteriores

- `specs/008-metrics-calculation` define cinco estados de valor —parcial, provisoria, definitiva, no
  calculable y no aplicable—; el dashboard debe respetarlos sin aplanarlos (FR-014 a FR-017, RC-05).
- `specs/008-metrics-calculation` marca el acumulado como parcial si incluye al sprint activo; el
  dashboard refleja esa marca (US2, escenario 3).
- `specs/007-defect-tracking` excluye los defectos Descartados y la spec 008 ya aplica la exclusión;
  el dashboard no vuelve a filtrar (RC-07, US5 escenario 6).
- `specs/004-sprint-management` admite como máximo un sprint Activo por proyecto, lo que hace que la
  sección del sprint activo sea singular y no una lista.
- `specs/002-project-members` exige cerrar el sprint activo antes de finalizar un proyecto, por lo
  que un proyecto Finalizado nunca tiene sprint activo que mostrar (US1, escenario 2).
