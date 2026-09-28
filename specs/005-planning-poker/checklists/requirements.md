# Checklist de Calidad de la Especificación: Planning Poker en Tiempo Real

**Propósito**: validar que la especificación esté completa y sea de calidad suficiente antes de pasar
a la planificación
**Creado**: 2026-09-26
**Feature**: [spec.md](../spec.md)

## Calidad del Contenido

- [x] Sin detalles de implementación (lenguajes, frameworks, APIs, protocolos, esquemas de datos)
- [x] Centrada en el valor para el usuario y las necesidades del negocio
- [x] Redactada para personas no técnicas
- [x] Todas las secciones obligatorias completadas (objetivo, entradas y salidas, escenarios,
      requisitos, reglas de negocio, restricciones, casos límite, condiciones de error, criterios de
      aceptación y de éxito, fuera de alcance, supuestos)

## Completitud de los Requisitos

- [x] No quedan marcadores [NEEDS CLARIFICATION] — los 2 que quedaban (FR-063 y FR-075) se resolvieron
      en la sesión de clarificación del 2026-09-26
- [x] Los requisitos son verificables y no ambiguos (FR-001 a FR-089, numeración continua y sin
      saltos)
- [x] Los criterios de éxito son medibles (SC-001 a SC-025)
- [x] Los criterios de éxito son independientes de la tecnología
- [x] Todos los escenarios de aceptación están definidos (US1 a US8, 79 escenarios, cada historia con
      caso normal, alternativo, límite y de error)
- [x] Los casos límite están identificados (sección "Casos Límite", 20 casos, que cubren los 7 del
      enunciado)
- [x] El alcance está claramente delimitado (sección "Fuera de Alcance", que incluye los 5 puntos
      exigidos por el enunciado)
- [x] Dependencias y supuestos identificados (RC-01 a RC-14 y sección "Supuestos")

## Preparación de la Feature

- [x] Todos los requisitos funcionales tienen criterios de aceptación claros
- [x] Los escenarios de usuario cubren los flujos principales
- [x] La spec es coherente con las features de las que depende (`specs/002-project-members`,
      `specs/003-product-backlog` y `specs/004-sprint-management`)
- [x] La feature cumple con los resultados medibles definidos en Criterios de Éxito
- [x] No se filtran detalles de implementación en la especificación

## Notas

- Los ítems incompletos exigen actualizar la spec antes de `/speckit-clarify` o `/speckit-plan`.
- Iteración 1 de validación (2026-09-26): 15/16 ítems en verde. El único ítem en rojo eran las 2
  preguntas abiertas marcadas en FR-063 y FR-075.
- Iteración 2 de validación (2026-09-26, tras la sesión de clarificación): 16/16 ítems en verde. Las
  dos decisiones quedaron registradas en la sección Clarifications de la spec:
  1. **Caducidad de una sesión abandonada** (FR-063, RN-31): una sesión Abierta se cancela
     automáticamente por inactividad a los 60 minutos sin ningún hecho (incorporaciones,
     desconexiones, votos, revelaciones, rondas nuevas y cambios de facilitador); cada hecho reinicia
     la cuenta. Así se destraba la historia sin ampliar los permisos de cancelación más allá del
     facilitador. Cubierto por US7 (escenarios 5, 6 y 9) y SC-021.
  2. **Reestimación directa con una sesión abierta** (FR-075, RN-32, RC-12): el cambio directo se
     acepta, la sesión no se interrumpe y el valor acordado al cerrar reemplaza el vigente (última
     escritura gana). La reestimación directa no se muestra como un hecho de la sesión, para no
     introducir el ancla que la feature evita. No hace falta enmendar `specs/003-product-backlog`.
     Cubierto por US5 (escenario 5) y SC-022.
- Iteración 3 de validación (2026-09-26, sesión de `/speckit-clarify`): 17/17 ítems en verde, sin
  regresiones. Se resolvieron 5 ambigüedades y la spec pasó de FR-088 a FR-089, de RN-33 a RN-36, de
  RC-12 a RC-14 y de SC-022 a SC-025:
  1. **Eliminación de una historia con historial de estimación** (FR-089, RN-33, RC-13): no se puede
     eliminar. Enmienda `specs/003-product-backlog` (FR-041, FR-043, RN-16, RC-05 y su nuevo RC-10).
  2. **Escala comprometida** (RC-14, FR-073, SC-004): 10 participantes por sesión y 10 sesiones en
     paralelo, 100 conexiones concurrentes, como capacidad objetivo y no como tope.
  3. **Varias conexiones de una misma persona** (FR-013, FR-014, FR-024, RN-34, SC-024): un único
     participante por usuario; conectado mientras le quede una conexión viva.
  4. **Pedidos simultáneos del rol de facilitador** (FR-072, RN-35, SC-025): prospera solo el primero
     que se procesa.
  5. **Retiro de un voto emitido** (FR-022, RN-36): no se puede retirar, solo cambiar por otra carta.
- Decisiones tomadas como supuestos, señaladas por su impacto y candidatas a revisar en la
  planificación:
  1. Registrar la estimación acordada exige al menos una ronda revelada (FR-051, RN-20).
  2. Iniciar una ronda nueva exige que la ronda vigente esté Revelada (FR-046, RN-17).
  3. El umbral de desconexión es de 30 segundos sin restablecer la presencia (FR-014).
  4. El relevo del facilitador es explícito y definitivo: quien fue relevado no recupera el rol al
     reconectarse (FR-068 a FR-070, RN-25).
  5. La sesión Abierta se cancela automáticamente si la historia pasa a Completada o el proyecto pasa
     a Finalizado (FR-061, RN-30).
  6. El valor acordado es siempre numérico: "sin estimar" no es un resultado posible de una sesión
     (FR-050, RC-05).
  7. El umbral de inactividad que caduca una sesión es de 60 minutos (FR-063), derivado de la decisión
     de la iteración 2.
- Consistencia verificada con las features de las que depende:
  - `specs/003-product-backlog`: se reutiliza la escala de RN-06 sin extenderla (RC-04); el único
    campo de la historia que esta feature escribe son sus Story Points (RC-03); la condición "lista
    para planificar" se recalcula como consecuencia, no se gestiona acá (RC-07). La feature cierra el
    hueco que RC-06 de esa spec anticipaba y **no la enmienda**: la estimación directa sigue permitida
    tal como está escrita allí (RC-12, RN-32).
  - `specs/004-sprint-management`: estimar una historia comprometida en un sprint abierto está
    permitido porque la sesión reemplaza un valor de Story Points por otro y nunca deja la historia
    sin estimación (RC-08); el estado de la historia lo sigue produciendo la feature de sprints
    (RC-06).
  - `specs/002-project-members`: el permiso de acceso es la membresía vigente y el estado Finalizado
    del proyecto impide toda escritura (RC-02); la autoría de los votos se conserva al quitar a un
    integrante, igual que en su RN-14.
