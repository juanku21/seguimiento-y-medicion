# Checklist de Calidad de la Especificación: Registro de Esfuerzo

**Propósito**: validar que la especificación esté completa y sea de calidad suficiente antes de pasar
a la planificación
**Creado**: 2026-10-02
**Feature**: [spec.md](../spec.md)

## Calidad del Contenido

- [x] Sin detalles de implementación (lenguajes, frameworks, APIs)
- [x] Centrada en el valor para el usuario y las necesidades del negocio
- [x] Redactada para personas no técnicas
- [x] Todas las secciones obligatorias completadas

## Completitud de los Requisitos

- [x] No quedan marcadores [NEEDS CLARIFICATION] — el único abierto (FR-016) se resolvió con el
      usuario el 2026-10-02
- [x] Los requisitos son verificables y no ambiguos (FR-001 a FR-061, numeración continua y sin
      saltos)
- [x] Los criterios de éxito son medibles (SC-001 a SC-022)
- [x] Los criterios de éxito son independientes de la tecnología
- [x] Todos los escenarios de aceptación están definidos (US1 a US5, 73 escenarios, cada historia con
      caso normal, alternativo, límite y de error)
- [x] Los casos límite están identificados (sección "Casos Límite", 23 casos, que incluyen los seis
      pedidos en el enunciado)
- [x] El alcance está claramente delimitado (sección "Fuera de Alcance")
- [x] Dependencias y supuestos identificados (RC-01 a RC-12, con las dependencias de
      `specs/001-user-auth`, `specs/002-project-members`, `specs/003-product-backlog` y
      `specs/004-sprint-management`)

## Preparación de la Feature

- [x] Todos los requisitos funcionales tienen criterios de aceptación claros
- [x] Los escenarios de usuario cubren los flujos principales
- [x] La feature cumple con los resultados medibles definidos en Criterios de Éxito
- [x] No se filtran detalles de implementación en la especificación

## Notas

- Los ítems incompletos exigen actualizar la spec antes de `/speckit-clarify` o `/speckit-plan`.
- Iteración 1 de validación (2026-10-02, al redactar la spec): 15/16 ítems en verde. El único ítem en
  rojo era el marcador [NEEDS CLARIFICATION] de FR-016, que se presentó al usuario como pregunta.
- Iteración 2 (2026-10-02, al resolver ese marcador con el usuario): 16/16 ítems en verde. Se adoptó
  el plazo de gracia: un sprint Cerrado admite esfuerzo nuevo y corregido durante un período
  configurable del sistema, por defecto 48 horas desde su cierre real, y después queda definitivo
  (FR-016, FR-020, FR-021, RN-09, RN-23, RC-06, RC-11, SC-009, SC-020, SC-021). De esa decisión se
  derivaron dos reglas más:
  1. El plazo habilita el alta, la modificación y la eliminación a la vez, para que nadie quede
     atrapado con un registro equivocado e inmutable desde que lo crea (FR-021).
  2. La vigencia se evalúa siempre contra el valor configurado en ese momento, sin congelar un
     vencimiento por sprint: ampliar el plazo reabre los sprints que caen dentro del valor nuevo y
     reducirlo cierra antes los que seguían abiertos (RC-11).
- La cobertura del enunciado se verificó punto por punto:
  - Los 5 puntos de ALCANCE están cubiertos por US1 a US5, una historia por punto.
  - Las 8 REGLAS DE NEGOCIO del enunciado están en RN-01 a RN-09 y RN-16 a RN-19.
  - Los 6 CASOS LÍMITE pedidos están en la sección "Casos Límite".
  - Las 6 CONDICIONES DE ERROR pedidas están en la tabla "Condiciones de Error".
  - Los 5 puntos de FUERA DE ALCANCE están en la sección homónima.

### ✅ Alineado con `specs/004-sprint-management` (2026-10-02)

RC-07 imponía una condición sobre una operación que la spec 004 definía sin restricciones:

- La spec 004 (FR-048, RN-20) permitía eliminar un sprint Planificado que nunca fue iniciado, sin
  condiciones adicionales.
- Con esta spec, esa eliminación debe rechazarse cuando el sprint tiene registros de esfuerzo
  asociados, porque de lo contrario esos registros quedarían sin ancla (FR-015) y el esfuerzo del
  proyecto dejaría de poder leerse por sprint.

La spec 004 se enmendó el 2026-10-02 y ya recoge la condición en su FR-048, su RN-20, su RC-10
(nuevo), su SC-016, el escenario 14 de su US7 y sus tablas de salidas y de errores. Las dos specs
dicen lo mismo y la contradicción está cerrada.

El plazo de gracia adoptado en la iteración 2 no contradice la spec 004, pero sí la matiza: su RN-18
dice que un sprint Cerrado "no admite cambios", y acá el esfuerzo asociado a ese sprint sigue siendo
editable durante el plazo. No hay conflicto real porque el esfuerzo no forma parte del sprint ni de
su instantánea de cierre (la spec 004 no lo menciona entre lo que congela), pero conviene que la
enmienda de la spec 004 lo deje dicho para que RN-18 no se lea como una prohibición más amplia de la
que es.

### Consistencia verificada con las specs anteriores

- `specs/002-project-members` FR-027 conserva el esfuerzo de un integrante dado de baja; recogido en
  FR-028, FR-032, FR-040 y RC-03.
- `specs/002-project-members` FR-018 vuelve de solo lectura un proyecto Finalizado; recogido en
  FR-008, FR-027, FR-060 y RN-06.
- `specs/002-project-members` define la zona horaria única del sistema (FR-039); reutilizada en
  FR-007 y RC-08.
- `specs/003-product-backlog` FR-043 y RN-16 impiden eliminar una historia con esfuerzo registrado;
  recogido en RC-05 y RN-22, incluido el caso de que después se borren todos los registros.
- `specs/003-product-backlog` distingue la estimación 0 de la marca "sin estimar"; esa distinción se
  sostiene en FR-050, FR-051, FR-055, RN-17 y RN-18.
- `specs/004-sprint-management` RN-18 establece que un sprint Cerrado no admite cambios; recogido en
  FR-020 y RN-09, y es la base del supuesto provisional de FR-016.
- `specs/004-sprint-management` FR-043 congela el factor de horas al cerrar un sprint; esta spec no
  lo usa y lo explica en el supuesto "Las horas estimadas se calculan al momento de la consulta".

### Supuestos vigentes, candidatos a revisar en la planificación

1. La lectura del esfuerzo es abierta entre integrantes del proyecto; solo la escritura es personal.
2. El tope diario de 24 horas cuenta registros de proyectos Finalizados y de proyectos de los que la
   persona fue quitada.
3. Las horas estimadas se recalculan siempre con el factor vigente, nunca con uno congelado.
4. El signo positivo de la desviación significa esfuerzo por encima de lo estimado.
5. Un registro sin sprint asociado permanece modificable indefinidamente mientras el proyecto no esté
   Finalizado.
6. El plazo de gracia por defecto es de 48 horas y se configura para todo el sistema, no por
   proyecto ni por equipo.
7. El plazo de gracia alcanza solo a las historias que siguen perteneciendo al sprint cerrado, es
   decir, las Completadas (RN-24). Las horas tardías de una historia devuelta al backlog quedan sin
   sprint asociado. Es una asimetría deliberada, por respeto a la regla de anclaje de FR-014, y es
   el punto de esta spec que más conviene revisar en `/speckit-clarify` si al usar el producto
   resulta contraintuitiva.
