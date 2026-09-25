# Especificación de Feature: Gestión de Sprints

**Directorio de feature**: `specs/004-sprint-management`

**Rama**: `004-sprint-management`

**Creada**: 2026-09-25

**Estado**: Clarificada — sin preguntas abiertas (sesión de clarificación del 2026-09-25)

**Entrada**: Descripción del usuario: "Gestión de Sprints para Software Metrics & Estimation, un
sistema web multiusuario para estimar, planificar, seguir y medir proyectos de software con Scrum.
Depende de: gestión de proyectos e integrantes (factor de horas por Story Point por proyecto);
Product Backlog (historias con estados Pendiente / En progreso / Completada, Story Points y condición
lista para planificar)."

---

## Objetivo

Permitir que el equipo organice el trabajo del proyecto en sprints: planificarlos con un objetivo,
comprometer historias, registrar cuáles se completan, cerrarlos y consultar el historial de sprints
anteriores con sus resultados congelados.

El sprint es la unidad de tiempo que convierte un backlog priorizado en un compromiso concreto: esto
es lo que el equipo se propone terminar, entre estas dos fechas, con este objetivo. Es también el
único lugar del producto donde una historia pasa de planeada a terminada, y por lo tanto la fuente de
todo lo que después se mide. La instantánea que se congela al cerrar un sprint —cuántos Story Points
se comprometieron, cuántos se completaron y con qué factor de horas se traducían— es el dato
histórico sobre el que se calculan la velocidad del equipo y el desvío entre lo estimado y lo real.
Si esa instantánea cambiara cada vez que alguien reestima una historia vieja, ninguna comparación
entre sprints sería confiable.

---

## Clarifications

### Session 2026-09-25

- Q: ¿Quién puede iniciar el primer sprint de un proyecto, dado que la spec 002 reserva el cambio de estado del proyecto a su propietario? (FR-029) → A: iniciar un sprint nunca cambia el estado del proyecto; cualquier integrante puede iniciarlo y el propietario pasa el proyecto a En curso por su cuenta.
- Q: ¿Cómo se descarta un sprint Planificado que nunca se va a iniciar? (FR-048) → A: se puede eliminar mientras esté Planificado y no se haya iniciado nunca; sus historias comprometidas vuelven al backlog.
- Q: ¿Se puede quitar de un sprint Activo una historia que ya está Completada? (FR-025) → A: no; primero hay que desmarcarla y recién ahí quitarla.
- Q: ¿Los "Story Points planificados" de un sprint se miden con las historias que tenía al iniciarse o con las que tiene al cerrarse? (FR-043) → A: ambos; la instantánea guarda los Story Points comprometidos al iniciar y los planificados al cerrar, y su diferencia expresa el cambio de alcance.
- Q: ¿Qué pasa si alguien le saca los Story Points o los criterios de aceptación a una historia ya comprometida en un sprint? (FR-018) → A: se rechaza el cambio; mientras la historia esté comprometida en un sprint abierto no puede dejar de estar lista para planificar.
- Q: En el historial de sprints cerrados, ¿las fechas que se muestran son las previstas o las reales? (FR-051) → A: ambas; el historial muestra las fechas previstas y las reales de arranque y cierre, y la instantánea congela las cuatro.

---

## Entradas y Salidas Esperadas

### Creación de un sprint

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador del proyecto, nombre, fecha de inicio, fecha de fin y Sprint Goal | **Éxito**: sprint creado en estado Planificado, sin historias asignadas, atribuido a quien lo creó |
| | **Error de validación**: detalle de qué campo es inválido y por qué; no se crea nada |
| | **Superposición de fechas**: rechazo indicando con qué sprint del proyecto se superpone |
| | **Proyecto Finalizado**: rechazo por proyecto de solo lectura |
| | **Quien pide no es integrante**: respuesta de proyecto inexistente |

### Modificación de un sprint

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador del sprint y los datos a modificar (nombre, fechas, Sprint Goal) | **Éxito**: sprint actualizado; su estado y sus historias asignadas no cambian |
| | **Error de validación o superposición de fechas**: rechazo con el motivo; el sprint queda como estaba |
| | **Sprint Activo o Cerrado**: rechazo indicando que solo se modifica un sprint Planificado |
| | **Quien pide no es integrante**: respuesta de sprint inexistente |

### Asignación y baja de historias

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador del sprint e identificadores de una o más historias del backlog | **Éxito al asignar**: las historias quedan comprometidas en el sprint y dejan de estar disponibles para otro sprint abierto |
| | **Éxito al quitar**: las historias vuelven al backlog en estado Pendiente y sin sprint |
| | **Historia no lista para planificar**: rechazo indicando que le faltan Story Points o criterios de aceptación |
| | **Historia Completada o ya asignada a otro sprint abierto**: rechazo con el motivo; nada se asigna |
| | **Sprint Cerrado**: rechazo indicando que un sprint cerrado no admite cambios |

### Inicio de un sprint

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador de un sprint Planificado | **Éxito**: el sprint pasa a Activo, queda registrado el momento de inicio real y se congelan los Story Points comprometidos al inicio; el estado del proyecto no cambia |
| | **Ya hay un sprint Activo en el proyecto**: rechazo indicando cuál es y que debe cerrarse primero |
| | **Sprint Activo o Cerrado**: rechazo indicando que solo se inicia un sprint Planificado |

### Registro del avance de las historias

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador de una historia del sprint Activo y el estado destino | **Éxito**: la historia queda en el estado destino; al marcarla Completada se registra la fecha y hora de completado |
| | **Historia que no pertenece al sprint Activo**: rechazo indicando que solo se registra avance dentro del sprint activo |
| | **Transición inválida**: rechazo indicando las transiciones permitidas |

### Cierre de un sprint

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador del sprint Activo | **Éxito**: el sprint pasa a Cerrado con su instantánea congelada (Story Points comprometidos al inicio, planificados al cierre y completados, historias involucradas y factor de horas vigente); las historias no completadas vuelven al backlog como Pendientes |
| | **Sprint Planificado o Cerrado**: rechazo indicando que solo se cierra un sprint Activo |
| | **Proyecto Finalizado**: rechazo por proyecto de solo lectura |

### Eliminación de un sprint

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador de un sprint Planificado que nunca fue iniciado | **Éxito**: el sprint deja de existir, su período queda libre y sus historias comprometidas vuelven al backlog en estado Pendiente y sin sprint |
| | **Sprint Activo o Cerrado**: rechazo indicando que solo se elimina un sprint que nunca arrancó |
| | **Proyecto Finalizado**: rechazo por proyecto de solo lectura |
| | **Quien pide no es integrante**: respuesta de sprint inexistente |

### Consulta de sprints

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador del proyecto | Lista de los sprints del proyecto con nombre, estado, fechas, Sprint Goal y cantidad de historias, ordenada de forma estable |
| Sesión de un integrante y filtro de sprints cerrados | Lista de los sprints cerrados con nombre, Sprint Goal, las cuatro fechas (inicio y fin previstas, inicio y cierre reales), historias planificadas, historias completadas, y Story Points comprometidos al inicio, planificados al cierre y completados, tomados de la instantánea |
| Proyecto sin sprints | Lista vacía con la indicación de que el proyecto todavía no tiene sprints |
| Sesión de quien no es integrante | Respuesta de proyecto inexistente, idéntica a la de un identificador que no existe |

---

## Escenarios de Usuario y Pruebas *(obligatorio)*

### Historia de Usuario 1 (US1) — Planificar un sprint (Prioridad: P1)

Un integrante crea el próximo sprint del proyecto: le pone un nombre, define entre qué fechas va a
correr y escribe el objetivo que el equipo se propone alcanzar.

**Por qué esta prioridad**: sin sprint no hay dónde comprometer historias ni dónde registrar avance.
Es la primera porción que entrega valor observable sobre el backlog ya construido.

**Prueba independiente**: se puede probar completa creando un sprint en un proyecto y verificando que
queda en estado Planificado, sin historias y con los datos ingresados.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un integrante de un proyecto En curso sin sprints, **cuando** crea el
   sprint "Sprint 1" del 2026-10-01 al 2026-10-14 con el objetivo "Cerrar el alta de clientes",
   **entonces** el sprint queda creado en estado Planificado, sin historias asignadas y atribuido a
   quien lo creó.
2. *(Caso alternativo)* **Dado** un proyecto en estado Planificado, **cuando** un integrante crea un
   sprint, **entonces** la creación se acepta y el proyecto sigue Planificado, porque el proyecto solo
   avanza al iniciarse un sprint, no al crearlo.
3. *(Caso alternativo)* **Dado** un proyecto que ya tiene un sprint Planificado, **cuando** un
   integrante crea otro sprint con fechas que no se superponen, **entonces** ambos coexisten en estado
   Planificado.
4. *(Caso límite)* **Dado** un proyecto con un sprint que termina el 2026-10-14, **cuando** un
   integrante crea otro que empieza el 2026-10-14, **entonces** la creación se acepta, porque dos
   sprints que solo comparten el día de borde no se consideran superpuestos.
5. *(Caso límite)* **Dado** un integrante de un proyecto, **cuando** crea un sprint con un Sprint Goal
   de exactamente 500 caracteres, **entonces** el sprint se crea; y **cuando** tiene 501, se rechaza.
6. *(Caso límite)* **Dado** un integrante de un proyecto, **cuando** crea un sprint cuya fecha de fin
   es el día siguiente al de inicio, **entonces** el sprint se crea, porque es la duración mínima
   admitida.
7. *(Caso de error)* **Dado** un integrante de un proyecto, **cuando** intenta crear un sprint sin
   nombre, sin Sprint Goal, sin fecha de inicio o sin fecha de fin, **entonces** la creación se
   rechaza indicando qué campo corregir y no se crea nada.
8. *(Caso de error)* **Dado** un integrante de un proyecto, **cuando** intenta crear un sprint cuya
   fecha de fin es anterior o igual a la de inicio, **entonces** la creación se rechaza indicando que
   la fecha de fin debe ser posterior a la de inicio.
9. *(Caso de error)* **Dado** un proyecto con un sprint del 2026-10-01 al 2026-10-14, **cuando** un
   integrante intenta crear otro del 2026-10-10 al 2026-10-20, **entonces** la creación se rechaza
   indicando con qué sprint se superpone.
10. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
    crear un sprint, **entonces** la creación se rechaza porque el proyecto es de solo lectura.
11. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta crear
    un sprint en él, **entonces** la respuesta es de proyecto inexistente y no se crea nada.

---

### Historia de Usuario 2 (US2) — Comprometer historias en un sprint (Prioridad: P1)

Un integrante elige del backlog las historias que el equipo se compromete a terminar en el sprint, y
saca las que ya no entran.

**Por qué esta prioridad**: un sprint sin historias es una caja vacía. Comprometer trabajo es lo que
convierte el sprint en un plan y es la condición previa para medir después cuánto se cumplió.

**Prueba independiente**: se puede probar completa creando un sprint Planificado, asignándole
historias listas para planificar y verificando que quedan comprometidas y que quitarlas las devuelve
al backlog.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un sprint Planificado y tres historias del backlog listas para
   planificar, **cuando** un integrante se las asigna, **entonces** las tres quedan comprometidas en
   el sprint, siguen en estado Pendiente y dejan de estar disponibles para otro sprint abierto.
2. *(Caso alternativo)* **Dado** un sprint Activo, **cuando** un integrante le suma una historia lista
   para planificar, **entonces** la asignación se acepta, porque el alcance puede ajustarse mientras
   el sprint corre.
3. *(Caso alternativo)* **Dado** una historia comprometida en un sprint Planificado, **cuando** un
   integrante la quita, **entonces** vuelve al backlog en estado Pendiente y sin sprint, y puede
   asignarse a otro sprint.
4. *(Caso alternativo)* **Dado** una historia comprometida en un sprint abierto y estimada en 5 Story
   Points, **cuando** un integrante la reestima en 8, **entonces** el cambio se acepta, porque la
   historia sigue estando lista para planificar.
5. *(Caso límite)* **Dado** un sprint Activo con una sola historia, **cuando** un integrante la quita,
   **entonces** el sprint queda sin historias y sigue Activo, sin que eso sea un error.
6. *(Caso límite)* **Dado** una historia que estuvo planificada en dos sprints ya cerrados sin
   completarse, **cuando** un integrante la asigna a un tercer sprint, **entonces** la asignación se
   acepta y su historial muestra los tres sprints.
7. *(Caso de error)* **Dado** una historia sin Story Points o sin criterios de aceptación, **cuando**
   un integrante intenta asignarla a un sprint, **entonces** la asignación se rechaza indicando que la
   historia no está lista para planificar.
8. *(Caso de error)* **Dado** una historia en estado Completada, **cuando** un integrante intenta
   asignarla a un sprint, **entonces** la asignación se rechaza indicando que la historia ya está
   terminada.
9. *(Caso de error)* **Dado** una historia ya comprometida en un sprint Planificado, **cuando** un
   integrante intenta asignarla a otro sprint abierto, **entonces** la asignación se rechaza indicando
   en qué sprint está comprometida.
10. *(Caso de error)* **Dado** un sprint Cerrado, **cuando** un integrante intenta asignarle o quitarle
    una historia, **entonces** la acción se rechaza porque un sprint cerrado no admite cambios.
11. *(Caso de error)* **Dado** un lote de tres historias donde una no está lista para planificar,
    **cuando** un integrante intenta asignar las tres a la vez, **entonces** la operación se rechaza
    entera y ninguna de las tres queda asignada.
12. *(Caso de error)* **Dado** una historia de otro proyecto, **cuando** un integrante intenta
    asignarla a un sprint, **entonces** la asignación se rechaza con una respuesta de historia
    inexistente.
13. *(Caso de error)* **Dado** una historia en estado Completada dentro del sprint Activo, **cuando**
    un integrante intenta quitarla del sprint, **entonces** la baja se rechaza indicando que primero
    debe devolverla a En progreso, y la historia conserva su estado y su fecha de completado.
14. *(Caso de error)* **Dado** una historia comprometida en un sprint Planificado, **cuando** un
    integrante intenta devolverla a "sin estimar" o quitarle su último criterio de aceptación,
    **entonces** el cambio se rechaza indicando que primero debe quitarla del sprint, y la historia
    queda como estaba.

---

### Historia de Usuario 3 (US3) — Iniciar un sprint (Prioridad: P2)

Un integrante marca el arranque del sprint planificado, y a partir de ahí el equipo puede registrar
avance sobre sus historias.

**Por qué esta prioridad**: es la puerta que habilita el registro de avance. Va después de planificar
y comprometer porque necesita que el sprint exista y tenga contenido.

**Prueba independiente**: se puede probar completa creando un sprint Planificado, iniciándolo y
verificando que pasa a Activo, que queda registrado el momento de inicio y que un segundo inicio se
rechaza.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un sprint Planificado con historias comprometidas en un proyecto En
   curso, **cuando** un integrante lo inicia, **entonces** el sprint pasa a Activo y queda registrado
   el momento de inicio real.
2. *(Caso alternativo)* **Dado** un proyecto en estado Planificado con un sprint Planificado,
   **cuando** un integrante que no es el propietario inicia el sprint, **entonces** el sprint pasa a
   Activo y el proyecto sigue en estado Planificado, porque los sprints no cambian el estado del
   proyecto.
3. *(Caso alternativo)* **Dado** un proyecto que ya está En curso, **cuando** se inicia un sprint,
   **entonces** el estado del proyecto tampoco cambia.
4. *(Caso límite)* **Dado** un sprint Planificado sin ninguna historia asignada, **cuando** un
   integrante lo inicia, **entonces** el inicio se acepta y el sprint queda Activo y vacío.
5. *(Caso límite)* **Dado** un sprint cuya fecha de inicio todavía no llegó, **cuando** un integrante
   lo inicia, **entonces** el inicio se acepta, porque el arranque lo decide el equipo y no el
   calendario.
6. *(Caso de error)* **Dado** un proyecto que ya tiene un sprint Activo, **cuando** un integrante
   intenta iniciar otro sprint Planificado, **entonces** la acción se rechaza indicando cuál es el
   sprint activo y que debe cerrarse primero.
7. *(Caso de error)* **Dado** un sprint ya Activo, **cuando** un integrante intenta iniciarlo otra
   vez, **entonces** la acción se rechaza por transición inválida y nada cambia.
8. *(Caso de error)* **Dado** un sprint Cerrado, **cuando** un integrante intenta iniciarlo,
   **entonces** la acción se rechaza porque un sprint cerrado no vuelve atrás.
9. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
   iniciar un sprint, **entonces** la acción se rechaza porque el proyecto es de solo lectura.

---

### Historia de Usuario 4 (US4) — Registrar el avance de las historias del sprint (Prioridad: P2)

Durante el sprint activo, el equipo mueve sus historias: las empieza y las da por terminadas. Si se
dieron por terminadas antes de tiempo, las vuelve atrás.

**Por qué esta prioridad**: es el registro del trabajo real y la única vía por la que una historia
llega a Completada. Sin él, el cierre del sprint no tendría nada que contar.

**Prueba independiente**: se puede probar completa con un sprint Activo, moviendo una historia por
Pendiente, En progreso y Completada, y verificando la fecha y hora de completado.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una historia Pendiente comprometida en el sprint Activo, **cuando** un
   integrante la pasa a En progreso y más tarde a Completada, **entonces** ambas transiciones se
   aceptan y al completarla queda registrada la fecha y hora del cambio.
2. *(Caso alternativo)* **Dado** una historia Completada dentro del sprint Activo, **cuando** un
   integrante la devuelve a En progreso, **entonces** el cambio se acepta y la fecha y hora de
   completado se borra.
3. *(Caso alternativo)* **Dado** una historia que ya volvió de Completada a En progreso, **cuando** un
   integrante la completa de nuevo, **entonces** se registra la fecha y hora del último completado.
4. *(Caso límite)* **Dado** una historia cuyos Story Points cambian mientras el sprint está Activo,
   **cuando** el equipo la completa, **entonces** el avance se registra igual y el sprint tomará el
   valor vigente al momento del cierre para su instantánea.
5. *(Caso límite)* **Dado** el último día del sprint Activo, **cuando** un integrante completa una
   historia, **entonces** el registro se acepta, porque el sprint admite avance hasta que se cierra.
6. *(Caso de error)* **Dado** una historia comprometida en un sprint Planificado, **cuando** un
   integrante intenta pasarla a En progreso o a Completada, **entonces** la acción se rechaza porque
   solo se registra avance dentro del sprint activo.
7. *(Caso de error)* **Dado** una historia del backlog sin sprint, **cuando** un integrante intenta
   completarla, **entonces** la acción se rechaza indicando que solo se completan historias del sprint
   activo.
8. *(Caso de error)* **Dado** una historia de un sprint Cerrado, **cuando** un integrante intenta
   cambiarle el estado, **entonces** la acción se rechaza porque un sprint cerrado no admite cambios.
9. *(Caso de error)* **Dado** una historia Pendiente del sprint Activo, **cuando** un integrante
   intenta pasarla directamente a Completada, **entonces** la acción se rechaza indicando que primero
   debe pasar por En progreso.

---

### Historia de Usuario 5 (US5) — Cerrar un sprint y congelar su resultado (Prioridad: P2)

Un integrante cierra el sprint. Lo que se completó queda registrado para siempre con sus números, y
lo que no se completó vuelve al backlog para replanificarse.

**Por qué esta prioridad**: el cierre produce la instantánea que alimenta todas las métricas del
producto. Va después del registro de avance porque necesita saber qué se completó.

**Prueba independiente**: se puede probar completa con un sprint Activo de historias parcialmente
completadas, cerrándolo y verificando la instantánea y el destino de cada historia.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un sprint Activo con cuatro historias comprometidas que suman 21 Story
   Points, de las cuales dos por 13 puntos están Completadas, **cuando** un integrante lo cierra,
   **entonces** el sprint queda Cerrado con 21 Story Points planificados al cierre y 13 completados, las dos
   historias no completadas vuelven al backlog como Pendientes y sin sprint, y el sprint conserva el
   registro de las cuatro.
2. *(Caso alternativo)* **Dado** un sprint Activo cuyas historias están todas Completadas, **cuando**
   un integrante lo cierra, **entonces** ninguna historia vuelve al backlog y los Story Points
   planificados al cierre y los completados coinciden.
3. *(Caso alternativo)* **Dado** un sprint iniciado con 20 Story Points comprometidos al que después
   se le sumaron dos historias de 8 puntos, **cuando** un integrante lo cierra, **entonces** la
   instantánea guarda 20 Story Points comprometidos al inicio y 36 planificados al cierre, y la
   diferencia deja visible el alcance agregado durante el sprint.
4. *(Caso alternativo)* **Dado** un sprint ya Cerrado, **cuando** alguien reestima una de sus
   historias o el propietario cambia el factor de horas por Story Point del proyecto, **entonces** los
   datos del sprint cerrado no cambian.
5. *(Caso límite)* **Dado** un sprint Activo sin ninguna historia completada, **cuando** un integrante
   lo cierra, **entonces** el cierre se acepta, los Story Points completados quedan en cero y todas
   las historias vuelven al backlog.
6. *(Caso límite)* **Dado** un sprint Activo sin ninguna historia, **cuando** un integrante lo cierra,
   **entonces** el cierre se acepta con la instantánea en cero.
7. *(Caso límite)* **Dado** un sprint Activo cuya fecha de fin todavía no llegó, **cuando** un
   integrante lo cierra, **entonces** el cierre anticipado se acepta y queda registrado el momento
   real de cierre.
8. *(Caso límite)* **Dado** un sprint Activo cuya fecha de fin ya pasó, **cuando** un integrante lo
   cierra, **entonces** el cierre se acepta sin ninguna penalización ni aviso de error.
9. *(Caso de error)* **Dado** un sprint Planificado, **cuando** un integrante intenta cerrarlo,
   **entonces** la acción se rechaza indicando que solo se cierra un sprint Activo.
10. *(Caso de error)* **Dado** un sprint Cerrado, **cuando** un integrante intenta cerrarlo de nuevo o
    reabrirlo, **entonces** la acción se rechaza porque el estado Cerrado es definitivo.
11. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
    cerrar un sprint, **entonces** la acción se rechaza porque el proyecto es de solo lectura.

---

### Historia de Usuario 6 (US6) — Consultar el historial de sprints (Prioridad: P3)

Un integrante repasa los sprints anteriores del proyecto para ver qué se propuso el equipo en cada
uno y cuánto de eso terminó.

**Por qué esta prioridad**: es la lectura de datos que el cierre ya produjo. El equipo puede trabajar
sin esta vista, pero sin ella no hay forma de mirar la evolución del proyecto.

**Prueba independiente**: se puede probar completa cerrando dos sprints con resultados distintos y
verificando que el historial muestra los números congelados de cada uno.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto con tres sprints cerrados, **cuando** un integrante consulta
   el historial, **entonces** ve los tres con su nombre, Sprint Goal, sus cuatro fechas, historias
   planificadas, historias completadas, y Story Points comprometidos al inicio, planificados al
   cierre y completados.
2. *(Caso alternativo)* **Dado** un proyecto con un sprint Cerrado, uno Activo y uno Planificado,
   **cuando** un integrante consulta el listado completo de sprints, **entonces** ve los tres con su
   estado, y el historial de cerrados muestra solo el primero.
3. *(Caso alternativo)* **Dado** una historia que estuvo planificada en tres sprints hasta
   completarse, **cuando** un integrante consulta su historial, **entonces** ve los tres sprints en
   los que estuvo, con el resultado que tuvo en cada uno.
4. *(Caso límite)* **Dado** un sprint previsto del 2026-10-01 al 2026-10-14 que arrancó el
   2026-10-03 y se cerró el 2026-10-10, **cuando** un integrante consulta el historial, **entonces**
   ve las cuatro fechas por separado, de modo que la duración real del sprint es distinguible de la
   prevista.
5. *(Caso límite)* **Dado** un proyecto sin ningún sprint, **cuando** un integrante consulta el
   listado, **entonces** obtiene una lista vacía con la indicación de que todavía no hay sprints, sin
   que eso sea un error.
6. *(Caso límite)* **Dado** un proyecto cuyos sprints cerrados no completaron ninguna historia,
   **cuando** un integrante consulta el historial, **entonces** los ve listados con cero Story Points
   completados, sin que eso sea un error.
7. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
   consultar sus sprints, **entonces** la respuesta es de proyecto inexistente y no revela ningún
   dato.
8. *(Caso de error)* **Dado** un identificador de sprint que no existe, **cuando** un integrante
   intenta consultarlo, **entonces** obtiene una respuesta de sprint inexistente.

---

### Historia de Usuario 7 (US7) — Corregir o descartar un sprint planificado (Prioridad: P3)

Un integrante ajusta el nombre, las fechas o el objetivo de un sprint que todavía no arrancó, o lo
elimina si se creó por error o el equipo decidió no usarlo.

**Por qué esta prioridad**: es una corrección sobre algo que ya funciona. El equipo puede trabajar sin
esta historia, aunque obligaría a acertar los datos en el primer intento y a arrastrar para siempre
los sprints creados de más.

**Prueba independiente**: se puede probar completa creando un sprint Planificado, modificando cada uno
de sus campos, eliminándolo y verificando que los cambios se reflejan, que su período queda libre y
que los mismos intentos sobre un sprint Activo se rechazan.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un sprint Planificado, **cuando** un integrante le cambia el nombre, las
   fechas y el Sprint Goal por valores válidos, **entonces** el sprint queda actualizado y su estado y
   sus historias asignadas no cambian.
2. *(Caso normal)* **Dado** un sprint Planificado que nunca fue iniciado y tiene dos historias
   comprometidas, **cuando** un integrante lo elimina, **entonces** el sprint deja de existir, su
   período queda libre para otro sprint y las dos historias vuelven al backlog en estado Pendiente y
   sin sprint.
3. *(Caso alternativo)* **Dado** un sprint Planificado con historias comprometidas, **cuando** un
   integrante corre sus fechas a otro período libre, **entonces** el cambio se acepta y las historias
   siguen comprometidas.
4. *(Caso límite)* **Dado** un sprint Planificado, **cuando** un integrante lo renombra con el mismo
   nombre que ya tenía, **entonces** el cambio se acepta.
5. *(Caso límite)* **Dado** dos sprints Planificados con fechas contiguas, **cuando** un integrante
   mueve el segundo de modo que empiece el día en que termina el primero, **entonces** el cambio se
   acepta, porque compartir el día de borde no es superposición.
6. *(Caso límite)* **Dado** un sprint Planificado sin ninguna historia, **cuando** un integrante lo
   elimina, **entonces** la eliminación se acepta sin efectos sobre el backlog.
7. *(Caso límite)* **Dado** una historia que estuvo comprometida en un sprint que después se eliminó,
   **cuando** un integrante intenta eliminar esa historia del backlog, **entonces** la eliminación se
   rechaza, porque la historia ya estuvo asignada a un sprint alguna vez.
8. *(Caso de error)* **Dado** un sprint Planificado, **cuando** un integrante intenta dejar el nombre
   o el Sprint Goal vacíos, poner una fecha de fin anterior o igual a la de inicio, o un Sprint Goal
   de más de 500 caracteres, **entonces** la modificación se rechaza indicando el motivo y no se
   aplica ningún cambio parcial.
9. *(Caso de error)* **Dado** un sprint Planificado, **cuando** un integrante intenta moverlo a un
   período que se superpone con otro sprint del proyecto, **entonces** la modificación se rechaza
   indicando con cuál se superpone.
10. *(Caso de error)* **Dado** un sprint Activo, **cuando** un integrante intenta modificar sus datos,
    **entonces** la acción se rechaza indicando que solo se modifica un sprint Planificado.
11. *(Caso de error)* **Dado** un sprint Cerrado, **cuando** un integrante intenta modificar sus datos,
    **entonces** la acción se rechaza porque un sprint cerrado no admite cambios.
12. *(Caso de error)* **Dado** un sprint Activo o uno Cerrado, **cuando** un integrante intenta
    eliminarlo, **entonces** la acción se rechaza indicando que solo se elimina un sprint que nunca
    fue iniciado.
13. *(Caso de error)* **Dado** un sprint ya eliminado, **cuando** un integrante intenta operarlo,
    **entonces** obtiene una respuesta de sprint inexistente.

---

### Casos Límite

- **Sprint cerrado sin ninguna historia completada**: cierre válido; los Story Points completados
  quedan en cero y todas las historias vuelven al backlog.
- **Sprint iniciado sin historias**: inicio válido; se pueden sumar historias mientras esté Activo.
- **Sprint cerrado sin ninguna historia**: cierre válido con la instantánea entera en cero.
- **Sprints con fechas contiguas**: el fin de uno igual al inicio del siguiente no se considera
  superposición; solo hay superposición cuando los períodos comparten más que el día de borde.
- **Fecha de fin igual a la de inicio**: se rechaza; la fecha de fin debe ser estrictamente posterior.
- **Historia que pasa por tres sprints hasta completarse**: válida; cada cierre la devuelve al backlog
  como Pendiente y cada sprint conserva el registro de haberla tenido planificada.
- **Cierre anticipado antes de la fecha de fin**: válido; el momento real de cierre se registra y
  puede ser anterior a la fecha de fin prevista.
- **Cierre posterior a la fecha de fin**: válido; el sistema no cierra sprints por calendario ni avisa
  de atraso.
- **Sprint que arranca tarde y cierra temprano**: válido; la instantánea conserva por separado las
  fechas previstas y las reales, de modo que la duración real nunca se confunde con la planificada.
- **Inicio antes de la fecha de inicio prevista**: válido; el arranque lo decide el equipo.
- **Cambio de Story Points de una historia durante el sprint activo**: permitido mientras la historia
  no esté Completada; afecta los Story Points planificados al cierre, pero no los comprometidos al
  inicio, que quedaron congelados al arrancar.
- **Historia comprometida a la que se le intenta quitar la estimación o los criterios**: se rechaza
  mientras siga en un sprint abierto, de modo que ningún sprint llegue al cierre con historias a
  medio definir. Cambiar un valor de Story Points por otro sigue permitido.
- **Historias sumadas o quitadas con el sprint ya Activo**: los Story Points comprometidos al inicio
  no cambian; la diferencia contra los planificados al cierre es el cambio de alcance del sprint y
  puede ser positiva o negativa.
- **Historia desmarcada y vuelta a marcar antes del cierre**: válido; queda registrada la fecha y hora
  del último completado.
- **Quitar todas las historias de un sprint Activo**: válido; el sprint sigue Activo y vacío.
- **Historia quitada del sprint antes del cierre**: no figura en la instantánea ni en el registro de
  historias planificadas de ese sprint, porque el registro se congela al cerrar.
- **Varios sprints Planificados a la vez**: permitido, siempre que sus fechas no se superpongen; el
  límite de uno solo aplica al estado Activo.
- **Sprint cuyas fechas caen fuera del período del proyecto**: permitido; esta feature no compara las
  fechas del sprint con las del proyecto.
- **Sprint Planificado que el equipo decide no usar**: se elimina, lo que libera su período y devuelve
  sus historias al backlog. Un sprint que ya arrancó no se elimina nunca.
- **Quitar de un sprint una historia ya Completada**: se rechaza; hay que devolverla antes a En
  progreso, de modo que dar por no terminada una historia sea siempre un acto explícito.
- **Sprint Activo en un proyecto todavía Planificado**: situación posible y aceptada, porque iniciar
  un sprint no cambia el estado del proyecto. Pasarlo a En curso queda a cargo de su propietario.
- **Proyecto Finalizado**: los sprints admiten consulta por parte de cualquier integrante y rechazan
  toda acción de escritura.

---

## Requisitos *(obligatorio)*

### Requisitos Funcionales

**Creación del sprint**

- **FR-001**: El sistema DEBE permitir a cualquier integrante de un proyecto crear un sprint a partir
  de nombre, fecha de inicio, fecha de fin y Sprint Goal.
- **FR-002**: El sistema DEBE exigir un nombre de entre 1 y 100 caracteres, contados después de
  recortar los espacios al inicio y al final; un nombre ausente o compuesto solo por espacios se
  rechaza.
- **FR-003**: El sistema DEBE exigir un Sprint Goal de entre 1 y 500 caracteres, contados después de
  recortar los espacios extremos.
- **FR-004**: El sistema DEBE exigir fecha de inicio y fecha de fin, y rechazar la creación si la
  fecha de fin no es estrictamente posterior a la fecha de inicio.
- **FR-005**: El sistema DEBE rechazar un sprint cuyo período se superponga con el de otro sprint del
  mismo proyecto, cualquiera sea su estado. Dos períodos se consideran superpuestos cuando comparten
  al menos un día que no sea el día de borde: el fin de un sprint puede coincidir con el inicio del
  siguiente.
- **FR-006**: Todo sprint DEBE nacer en estado Planificado, sin historias asignadas y sin instantánea
  de cierre.
- **FR-007**: Cada sprint DEBE tener un identificador propio, estable, inmutable y no secuencial,
  distinto del nombre.
- **FR-008**: Cada sprint DEBE pertenecer a exactamente un proyecto y NO DEBE poder moverse a otro.
- **FR-009**: El sistema DEBE registrar en cada sprint quién lo creó, su momento de creación y su
  momento de última actualización.
- **FR-010**: El sistema DEBE permitir que un proyecto tenga varios sprints en estado Planificado a la
  vez, siempre que sus períodos no se superpongan.
- **FR-011**: El sistema DEBE rechazar una creación inválida indicando qué campo corregir y por qué,
  sin crear el sprint.

**Modificación del sprint**

- **FR-012**: El sistema DEBE permitir a cualquier integrante modificar el nombre, la fecha de inicio,
  la fecha de fin y el Sprint Goal de un sprint en estado Planificado.
- **FR-013**: El sistema DEBE aplicar en la modificación exactamente las mismas validaciones de
  nombre, Sprint Goal, fechas y superposición que en la creación (FR-002 a FR-005), excluyendo al
  propio sprint al verificar la superposición.
- **FR-014**: La modificación DEBE ser atómica: si algún dato es inválido, ningún cambio se aplica.
- **FR-015**: La modificación NO DEBE alterar el estado del sprint, sus historias comprometidas ni su
  instantánea.
- **FR-016**: El sistema DEBE rechazar la modificación de un sprint en estado Activo o Cerrado,
  indicando que solo se modifica un sprint Planificado.

**Compromiso de historias**

- **FR-017**: El sistema DEBE permitir a cualquier integrante asignar una o más historias del backlog
  del mismo proyecto a un sprint en estado Planificado o Activo.
- **FR-018**: El sistema DEBE admitir únicamente historias que estén listas para planificar, es decir
  con Story Points asignados y al menos un criterio de aceptación. Mientras una historia siga
  comprometida en un sprint abierto (Planificado o Activo), el sistema DEBE rechazar todo cambio que
  la deje de dejar lista para planificar: devolverla a "sin estimar" o quitarle su último criterio de
  aceptación. Para hacerlo hay que quitarla antes del sprint.
- **FR-019**: El sistema DEBE rechazar la asignación de una historia en estado Completada.
- **FR-020**: El sistema DEBE rechazar la asignación de una historia que ya esté comprometida en otro
  sprint abierto (Planificado o Activo), indicando en cuál está.
- **FR-021**: El sistema DEBE permitir quitar una o más historias de un sprint en estado Planificado o
  Activo.
- **FR-022**: Quitar una historia de un sprint DEBE devolverla al backlog en estado Pendiente y sin
  sprint asignado, dejándola disponible para otro sprint.
- **FR-023**: El sistema DEBE tratar cada operación de asignación o de baja como atómica: si alguna de
  las historias del lote es inválida, ninguna se asigna ni se quita.
- **FR-024**: El sistema DEBE rechazar toda asignación y toda baja de historias sobre un sprint
  Cerrado.
- **FR-025**: El sistema DEBE rechazar el intento de quitar de un sprint una historia en estado
  Completada, indicando que primero hay que devolverla a En progreso. Así ningún completado se borra
  como efecto colateral de una baja.

**Inicio del sprint**

- **FR-026**: El sistema DEBE permitir a cualquier integrante iniciar un sprint en estado Planificado,
  que pasa a Activo.
- **FR-027**: El sistema DEBE admitir como máximo un sprint en estado Activo por proyecto y rechazar
  el inicio de un segundo, indicando cuál es el sprint activo vigente.
- **FR-028**: Al iniciar un sprint, el sistema DEBE registrar el momento real de inicio, que puede
  diferir de la fecha de inicio prevista, y DEBE congelar los Story Points comprometidos al inicio:
  la suma de los Story Points de las historias que el sprint tenía en ese momento. Ese valor no
  cambia después por ninguna vía.
- **FR-029**: Iniciar un sprint NO DEBE modificar el estado del proyecto: un proyecto en estado
  Planificado sigue Planificado aunque tenga un sprint Activo. Pasar el proyecto a En curso es una
  acción aparte, que ejecuta su propietario según `specs/002-project-members`.
- **FR-030**: El sistema DEBE permitir iniciar un sprint sin historias comprometidas.
- **FR-031**: El sistema DEBE permitir iniciar un sprint sin exigir que la fecha de inicio prevista
  haya llegado.
- **FR-032**: El sistema DEBE rechazar el inicio de un sprint en estado Activo o Cerrado.

**Registro del avance de las historias**

- **FR-033**: El sistema DEBE permitir a cualquier integrante cambiar el estado de una historia
  comprometida en el sprint Activo, admitiendo únicamente las transiciones Pendiente → En progreso, En
  progreso → Completada y Completada → En progreso.
- **FR-034**: Al marcar una historia como Completada, el sistema DEBE registrar la fecha y hora del
  cambio.
- **FR-035**: Al devolver una historia de Completada a En progreso, el sistema DEBE borrar su fecha y
  hora de completado; un nuevo completado registra el momento del último cambio.
- **FR-036**: El sistema DEBE admitir cambios de estado de historias únicamente dentro del sprint
  Activo al que la historia pertenece.
- **FR-037**: El sistema DEBE rechazar todo cambio de estado de historias de un sprint Planificado o
  Cerrado, y de historias del backlog sin sprint.
- **FR-038**: El sistema DEBE rechazar cualquier transición de estado de historia distinta de las
  admitidas, incluido el salto de Pendiente a Completada, indicando las transiciones permitidas.

**Cierre del sprint**

- **FR-039**: El sistema DEBE permitir a cualquier integrante cerrar un sprint en estado Activo, que
  pasa a Cerrado.
- **FR-040**: El sistema DEBE permitir el cierre en cualquier momento, tanto antes como después de la
  fecha de fin prevista, y DEBE registrar el momento real de cierre y quién lo cerró.
- **FR-041**: Al cerrar el sprint, el sistema DEBE devolver al backlog todas las historias que no
  estén Completadas, dejándolas en estado Pendiente y sin sprint asignado.
- **FR-042**: Al cerrar el sprint, el sistema DEBE conservar el registro de todas las historias que
  estaban comprometidas en él en ese momento, incluidas las que vuelven al backlog.
- **FR-043**: Al cerrar el sprint, el sistema DEBE congelar una instantánea con los Story Points
  planificados al cierre, los Story Points completados, las historias involucradas con el resultado
  que tuvo cada una y el factor de horas por Story Point vigente del proyecto, todos tomados en el
  momento del cierre. La instantánea DEBE conservar además los Story Points comprometidos al inicio
  registrados por FR-028, de modo que la diferencia entre ambos exprese el cambio de alcance ocurrido
  durante el sprint, y las cuatro fechas del sprint: la de inicio prevista, la de fin prevista, el
  momento real de inicio y el momento real de cierre.
- **FR-044**: Los datos de un sprint Cerrado NO DEBEN cambiar por ninguna modificación posterior de
  las historias, de sus Story Points ni del factor de horas por Story Point del proyecto.
- **FR-045**: El sistema DEBE permitir cerrar un sprint sin ninguna historia completada y sin ninguna
  historia comprometida, devolviendo la instantánea con los valores en cero.
- **FR-046**: El estado Cerrado DEBE ser definitivo: el sistema rechaza reabrir, volver a cerrar,
  iniciar o modificar un sprint cerrado.
- **FR-047**: El sistema DEBE admitir únicamente las transiciones de sprint Planificado → Activo y
  Activo → Cerrado, y rechazar cualquier otra indicando la transición permitida.
- **FR-048**: El sistema DEBE permitir a cualquier integrante eliminar un sprint en estado Planificado
  que nunca fue iniciado. La eliminación DEBE devolver sus historias comprometidas al backlog en
  estado Pendiente y sin sprint, y DEBE liberar su período para otros sprints. El sistema DEBE
  rechazar la eliminación de un sprint Activo o Cerrado, para no destruir registro de trabajo real.
- **FR-049**: Ninguna acción de esta feature DEBE modificar el estado del proyecto, ni al crear, ni al
  iniciar, ni al cerrar, ni al eliminar un sprint.

**Consulta de sprints**

- **FR-050**: El sistema DEBE permitir a cualquier integrante listar los sprints de un proyecto con su
  nombre, estado, fecha de inicio, fecha de fin, Sprint Goal y cantidad de historias comprometidas.
- **FR-051**: El sistema DEBE permitir listar los sprints Cerrados de un proyecto devolviendo, para
  cada uno, nombre, Sprint Goal, las cuatro fechas (inicio prevista, fin prevista, inicio real y
  cierre real), historias planificadas, historias completadas, Story Points comprometidos al inicio,
  Story Points planificados al cierre y Story Points completados, tomados de su instantánea.
- **FR-052**: El listado de sprints DEBE presentarse en un orden estable y predecible: fecha de inicio
  descendente y, a igual fecha, por identificador del sprint.
- **FR-053**: El sistema DEBE permitir consultar el detalle de un sprint con sus datos, su estado, sus
  historias y, si está Cerrado, su instantánea.
- **FR-054**: Cuando el proyecto no tiene sprints, el sistema DEBE devolver un listado vacío con una
  indicación clara, no un error.
- **FR-055**: El sistema DEBE permitir consultar, para una historia dada, todos los sprints en los que
  estuvo planificada y el resultado que tuvo en cada uno.

**Autorización y visibilidad**

- **FR-056**: El sistema DEBE exigir una sesión válida para toda acción de esta feature, sin
  excepciones.
- **FR-057**: El sistema DEBE restringir toda acción sobre los sprints de un proyecto a los
  integrantes vigentes de ese proyecto.
- **FR-058**: Ante una solicitud sobre un proyecto, un sprint o una historia a los que quien pide no
  tiene acceso, el sistema DEBE responder exactamente igual que ante un identificador inexistente, sin
  revelar ningún dato ni la existencia del recurso.
- **FR-059**: Cuando el proyecto está en estado Finalizado, el sistema DEBE tratar sus sprints como de
  solo lectura: rechaza crear, modificar, asignar, quitar, iniciar, registrar avance y cerrar, y
  permite listar y consultar.
- **FR-060**: Toda creación, modificación, asignación, baja, inicio, registro de avance y cierre DEBE
  quedar atribuida al titular de la sesión que la ejecutó.

---

### Reglas de Negocio

| ID | Regla |
| --- | --- |
| RN-01 | Un sprint pertenece a un proyecto y cualquier integrante del proyecto puede operarlo. |
| RN-02 | Los estados del sprint son Planificado, Activo y Cerrado, y solo avanzan en ese orden. |
| RN-03 | El nombre y el Sprint Goal son obligatorios; el Sprint Goal mide hasta 500 caracteres. |
| RN-04 | La fecha de fin de un sprint es estrictamente posterior a su fecha de inicio. |
| RN-05 | Los períodos de dos sprints del mismo proyecto no se superponen; compartir el día de borde no es superposición. |
| RN-06 | Un proyecto tiene como máximo un sprint Activo a la vez. |
| RN-07 | Ninguna acción sobre los sprints cambia el estado del proyecto; pasarlo a En curso es una decisión aparte de su propietario. |
| RN-08 | Solo se comprometen historias listas para planificar, no Completadas y no comprometidas en otro sprint abierto. |
| RN-09 | Las historias se asignan y se quitan en sprints Planificados y Activos; en un sprint Cerrado, no. |
| RN-10 | Quitar una historia de un sprint la devuelve al backlog como Pendiente y sin sprint. |
| RN-11 | Una historia solo se marca Completada dentro del sprint Activo al que pertenece. |
| RN-12 | Una historia Completada puede desmarcarse mientras el sprint siga Activo. |
| RN-13 | Al cerrar un sprint, las historias no completadas vuelven al backlog como Pendientes y sin sprint. |
| RN-14 | Un sprint cerrado conserva el registro de todas las historias que estaban comprometidas en él al momento del cierre. |
| RN-15 | Al cerrar un sprint se congela su instantánea: Story Points comprometidos al inicio, planificados al cierre y completados, historias involucradas, las cuatro fechas del sprint y el factor de horas por Story Point vigente. |
| RN-16 | Los datos de un sprint cerrado no cambian ante modificaciones posteriores de las historias o del factor. |
| RN-17 | Una historia no completada puede planificarse de nuevo en un sprint posterior, y su historial muestra todos los sprints en que estuvo. |
| RN-18 | El estado Cerrado es definitivo: un sprint cerrado no se reabre, no se modifica y no admite cambios en sus historias. |
| RN-19 | En un proyecto Finalizado los sprints son de solo lectura. |
| RN-20 | Un sprint Planificado que nunca fue iniciado puede eliminarse; un sprint Activo o Cerrado, no. |
| RN-21 | Una historia Completada no puede quitarse de su sprint: primero hay que devolverla a En progreso. |
| RN-22 | Una historia comprometida en un sprint abierto no puede dejar de estar lista para planificar; para quitarle los Story Points o el último criterio de aceptación hay que sacarla antes del sprint. |

---

### Restricciones

- **RC-01**: Esta feature depende de la feature de autenticación y cuentas de usuario
  (`specs/001-user-auth`): solo usuarios registrados con sesión válida operan sobre los sprints.
- **RC-02**: Esta feature depende de la feature de gestión de proyectos e integrantes
  (`specs/002-project-members`): el sprint existe dentro de un proyecto, el permiso de acceso es la
  membresía vigente, el factor de horas por Story Point que se congela al cerrar es el del proyecto y
  el estado Finalizado vuelve los sprints de solo lectura.
- **RC-03**: Esta feature depende de la feature de Product Backlog (`specs/003-product-backlog`): las
  historias, sus Story Points, sus estados y la condición "lista para planificar" se definen allí.
  Esta feature las consume y es la única que cambia el estado de una historia, tal como establece
  FR-027 de esa spec.
- **RC-04**: La spec 002 establece que un proyecto no puede finalizarse mientras tenga un sprint
  activo. El orden obligado es cerrar el sprint y después finalizar el proyecto; esta feature no
  provoca ni bloquea la finalización del proyecto.
- **RC-05**: La spec 003 permite reestimar una historia mientras no esté Completada, incluso durante
  el sprint activo. Esta feature no bloquea el cambio de un valor de Story Points por otro, pero sí
  impide que una historia comprometida en un sprint abierto quede sin Story Points o sin criterios de
  aceptación (FR-018, RN-22). Es una restricción que esta feature impone sobre operaciones definidas
  en `specs/003-product-backlog`, que allí no están condicionadas.
- **RC-06**: La spec 003 impide eliminar una historia que alguna vez estuvo asignada a un sprint.
  Comprometer una historia en un sprint, aunque después se la quite o se elimine el sprint entero, la
  vuelve definitivamente no eliminable.
- **RC-07**: Las fechas de inicio y fin del sprint son fechas de calendario sin hora. Los momentos de
  inicio real, de completado de una historia y de cierre incluyen hora y se determinan en la zona
  horaria única y configurable del sistema, la misma que la spec 002 definió para la fecha de
  finalización real de un proyecto.
- **RC-08**: El cálculo de la velocidad del equipo, el burndown y el desvío entre lo estimado y lo
  real corresponde a la feature de métricas. Esta feature produce y conserva la instantánea de cierre
  que esos cálculos consumen, pero no los realiza.
- **RC-09**: Esta feature no compara las fechas del sprint con las del proyecto: un sprint puede
  empezar antes de la fecha de inicio del proyecto o terminar después de su fecha de finalización
  prevista.

---

### Condiciones de Error

| Condición | Comportamiento esperado |
| --- | --- |
| Acción de la feature sin sesión válida | Rechazo con indicación de iniciar sesión; la acción no se ejecuta. |
| Nombre o Sprint Goal ausente o compuesto solo por espacios | Rechazo indicando que el campo es obligatorio. |
| Nombre de más de 100 caracteres o Sprint Goal de más de 500 | Rechazo indicando el máximo permitido. |
| Fecha de inicio o fecha de fin ausente | Rechazo indicando el campo faltante. |
| Fecha de fin anterior o igual a la fecha de inicio | Rechazo indicando que debe ser estrictamente posterior. |
| Período superpuesto con otro sprint del proyecto | Rechazo indicando con qué sprint se superpone; el sprint no se crea ni se modifica. |
| Modificación de un sprint Activo o Cerrado | Rechazo indicando que solo se modifica un sprint Planificado. |
| Asignación de una historia que no está lista para planificar | Rechazo indicando que le faltan Story Points o criterios de aceptación. |
| Asignación de una historia en estado Completada | Rechazo indicando que la historia ya está terminada. |
| Asignación de una historia comprometida en otro sprint abierto | Rechazo indicando en qué sprint está comprometida. |
| Asignación o baja de historias sobre un sprint Cerrado | Rechazo indicando que un sprint cerrado no admite cambios. |
| Lote de historias con al menos una inválida | Rechazo completo de la operación; ninguna historia del lote se asigna ni se quita. |
| Inicio de un sprint habiendo otro Activo en el proyecto | Rechazo indicando cuál es el sprint activo y que debe cerrarse primero. |
| Inicio de un sprint Activo o Cerrado | Rechazo por transición inválida; el estado no cambia. |
| Cambio de estado de una historia fuera del sprint Activo al que pertenece | Rechazo indicando que solo se registra avance dentro del sprint activo. |
| Transición de estado de historia no admitida, incluido el salto de Pendiente a Completada | Rechazo indicando las transiciones permitidas. |
| Cierre de un sprint Planificado o Cerrado | Rechazo indicando que solo se cierra un sprint Activo. |
| Reapertura o modificación de un sprint Cerrado | Rechazo indicando que el estado Cerrado es definitivo. |
| Baja de una historia en estado Completada | Rechazo indicando que primero hay que devolverla a En progreso; la historia no cambia. |
| Quitar los Story Points o el último criterio de aceptación a una historia comprometida en un sprint abierto | Rechazo indicando que primero hay que quitarla del sprint; la historia no cambia. |
| Eliminación de un sprint Activo o Cerrado | Rechazo indicando que solo se elimina un sprint que nunca fue iniciado. |
| Cualquier acción de escritura sobre los sprints de un proyecto Finalizado | Rechazo por proyecto de solo lectura. |
| Solicitud sobre un proyecto, un sprint o una historia de los que quien pide no es integrante | Respuesta idéntica a la de un identificador inexistente, sin revelar ningún dato. |
| Solicitud sobre un identificador de sprint que no existe | Respuesta de sprint inexistente. |

---

### Entidades Clave

- **Sprint**: representa un período acotado de trabajo dentro de un proyecto, con un objetivo
  declarado y un conjunto de historias comprometidas. Atributos relevantes: identificador propio
  estable y no secuencial, proyecto al que pertenece, nombre, Sprint Goal, fecha de inicio prevista,
  fecha de fin prevista, estado, momento real de inicio, momento real de cierre, autor, momento de
  creación y momento de última actualización.
- **Compromiso de historia en el sprint**: representa que una historia forma parte del plan de un
  sprint. Atributos relevantes: sprint, historia, momento en que se comprometió y, cuando corresponde,
  la fecha y hora en que se completó dentro de ese sprint. Es lo que permite que el historial de una
  historia muestre todos los sprints por los que pasó.
- **Instantánea de cierre**: representa el resultado congelado de un sprint en el momento de cerrarse.
  Atributos relevantes: Story Points comprometidos al inicio, Story Points planificados al cierre,
  Story Points completados, historias involucradas con el resultado de cada una, las cuatro fechas
  del sprint (inicio y fin previstas, inicio y cierre reales) y factor de horas por Story Point
  vigente del proyecto. No cambia nunca después de registrarse y es la fuente de las métricas
  históricas.
- **Estado del sprint**: representa el punto del ciclo de vida en que está el sprint. Valores
  posibles: Planificado, Activo y Cerrado, recorridos en ese orden y sin vuelta atrás.
- **Historia de usuario** y **Estado de la historia** *(entidades de `specs/003-product-backlog`)*:
  esta feature las consume para comprometerlas en un sprint y es la única que cambia su estado.
- **Proyecto**, **Integrante del proyecto** y **factor de horas por Story Point** *(entidades de
  `specs/002-project-members`)*: esta feature los consume para ubicar el sprint, decidir quién accede
  y congelar el factor al cerrar.
- **Usuario** *(entidad de `specs/001-user-auth`)*: esta feature lo consume para atribuir la autoría
  de cada acción; no lo modifica.

---

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **SC-001**: Un integrante planifica un sprint completo (crearlo y comprometerle cinco historias) en
  menos de 3 minutos, y el 95 % lo logra en el primer intento, sin ayuda externa.
- **SC-002**: Un integrante marca una historia como completada durante el sprint activo en menos de 15
  segundos y en no más de 3 pasos.
- **SC-003**: Un integrante ve el estado del sprint activo (objetivo, fechas, historias comprometidas
  y cuáles están completadas) en una sola pantalla, sin combinar información de otros lugares del
  sistema.
- **SC-004**: Cero solapamientos de fechas entre sprints del mismo proyecto, verificado intentando
  crear sprints que se superpongan por un día, por el período completo y por contención total.
- **SC-005**: Cero proyectos con más de un sprint Activo, incluso al enviar 10 solicitudes de inicio
  simultáneas sobre sprints distintos del mismo proyecto.
- **SC-006**: El 100 % de las transiciones de sprint no permitidas se rechaza, verificado probando las
  combinaciones de estado origen y destino distintas de las dos transiciones válidas.
- **SC-007**: El 100 % de las historias comprometidas en un sprint cumple las tres condiciones de
  admisión (lista para planificar, no Completada y sin otro sprint abierto), verificado con una prueba
  por condición.
- **SC-008**: El 100 % de los sprints cerrados tiene su instantánea registrada con sus tres totales de
  Story Points y sus cuatro fechas, y ninguno de esos valores cambia después, verificado reestimando
  historias y cambiando el factor del proyecto tras el cierre y comparando la instantánea antes y
  después.
- **SC-009**: El 100 % de los sprints cerrados que sumaron o perdieron alcance mientras estaban
  Activos muestra una diferencia distinta de cero entre los Story Points comprometidos al inicio y
  los planificados al cierre, verificado agregando y quitando historias durante el sprint.
- **SC-010**: El 100 % de las historias no completadas vuelve al backlog como Pendiente y sin sprint
  al cerrar, verificado contando las historias del backlog antes y después del cierre.
- **SC-011**: Cero historias completadas fuera del sprint activo al que pertenecen, verificado
  intentando completarlas desde un sprint Planificado, desde uno Cerrado y desde el backlog.
- **SC-012**: El 100 % de las acciones de escritura sobre los sprints de un proyecto Finalizado se
  rechaza, sin producir ningún cambio.
- **SC-013**: El 100 % de las acciones de esta feature exige sesión válida y membresía vigente en el
  proyecto, verificado con una prueba por acción.
- **SC-014**: Cero filtraciones de datos entre proyectos: en el 100 % de los intentos de acceso de
  quien no es integrante, la respuesta es indistinguible de la de un identificador inexistente.
- **SC-015**: El historial de una historia que pasó por tres sprints muestra los tres, verificado
  recorriendo el ciclo completo de planificación, cierre y replanificación.
- **SC-016**: Cero registros de trabajo real perdidos por eliminación de sprints, verificado
  intentando eliminar un sprint Activo y uno Cerrado y comprobando que ambos intentos se rechazan.
- **SC-017**: El 100 % de las reglas de negocio (RN-01 a RN-22) tiene al menos una prueba automatizada
  asociada que falla si la regla se rompe.

---

## Fuera de Alcance

- Cálculo de métricas y velocidad del equipo, que se especifican en la feature de métricas. Esta
  feature solo produce y conserva la instantánea de cierre que esos cálculos consumen.
- Gráficos de cualquier tipo, incluido el burndown.
- Capacidad por integrante y asignación de historias a personas dentro del sprint.
- Reuniones y actas de ceremonias (planning, daily, review, retrospectiva).
- Registro de esfuerzo en horas sobre las historias del sprint, que se especifica en una feature
  propia.
- Gestión de defectos asociados a las historias del sprint, que se especifica en una feature propia.
- Sprints que abarquen más de un proyecto.
- Inicio y cierre automáticos de sprints por calendario; ambos son siempre decisiones explícitas de
  una persona.
- Plantillas de sprint, sprints recurrentes y duplicación de un sprint anterior.
- Notificaciones al equipo cuando un sprint empieza, termina o cambia de alcance.
- Historial de auditoría consultable de los cambios de un sprint (quién cambió qué y cuándo), más allá
  de la atribución de cada acción y de la instantánea de cierre.

---

## Supuestos

- **Nombre de 1 a 100 caracteres y no único**: el enunciado no fija longitud ni unicidad; se adopta el
  mismo máximo que el nombre de proyecto en `specs/002-project-members` y no se exige unicidad, porque
  la regla de no superposición de fechas ya impide que dos sprints ocupen el mismo período.
- **Sprint Goal obligatorio desde la creación**: el enunciado lo declara obligatorio sin excepciones,
  así que no se admite crear un sprint y completar el objetivo después.
- **Fechas de sprint sin hora**: la fecha de inicio y la de fin son fechas de calendario, como las del
  proyecto en `specs/002-project-members`. El momento real de inicio, el de completado de una historia
  y el de cierre sí incluyen hora.
- **Duración mínima de un día**: como la fecha de fin debe ser estrictamente posterior a la de inicio
  y las fechas no llevan hora, el sprint más corto posible abarca dos días de calendario.
- **La no superposición alcanza a todos los estados**: el período de un sprint Cerrado sigue ocupado y
  ningún sprint nuevo puede solaparse con él, para que el historial no tenga períodos ambiguos.
- **Inicio y cierre manuales**: el sistema no inicia ni cierra sprints por calendario; el enunciado
  contempla el cierre anticipado como caso válido, lo que implica que el control es del equipo.
- **Asignación y baja en lote**: las operaciones admiten una o varias historias y se tratan como una
  unidad, para que un lote parcialmente inválido no deje el sprint a medio armar.
- **Los sprints no tocan el estado del proyecto**: decisión confirmada el 2026-09-25 (FR-029, FR-049,
  RN-07). El enunciado original hacía que iniciar un sprint pasara el proyecto a En curso, pero eso
  chocaba con `specs/002-project-members`, que reserva el cambio de estado del proyecto a su
  propietario. Se resolvió desacoplando ambas cosas: los sprints nunca cambian el estado del
  proyecto y el propietario lo avanza por su cuenta.
- **Eliminar un sprint es definitivo**: decisión confirmada el 2026-09-25 (FR-048). Como solo se
  elimina un sprint que nunca arrancó, no hay trabajo registrado que perder; el borrado no conserva
  copia ni deja rastro consultable.
- **La instantánea combina dos momentos**: decisión confirmada el 2026-09-25 (FR-028, FR-043). Los
  Story Points comprometidos se congelan al iniciar el sprint; los planificados y los completados se
  computan al cerrar. Así una reestimación hecha durante el sprint activo se refleja en los valores
  de cierre pero no altera el compromiso original, y ninguna modificación posterior al cierre afecta
  a nada.
- **Las historias quitadas antes del cierre no figuran en el sprint**: el registro de historias
  planificadas se congela al cerrar, así que una historia comprometida y luego quitada no aparece en
  la instantánea ni en el historial de ese sprint.
- **Sin límite de sprints ni de historias por sprint**: no se define un máximo de sprints por proyecto
  ni de historias comprometidas por sprint, porque no se pidió.
- **Idioma de la interfaz**: los mensajes que ve la persona están en español.

---

## Preguntas Abiertas

Ninguna. Las tres decisiones que quedaron abiertas al redactar la especificación se resolvieron el
2026-09-25 y están registradas en la sección Clarifications: el efecto del inicio de un sprint sobre
el estado del proyecto (FR-029), el destino de un sprint Planificado que no se va a usar (FR-048) y
la baja de una historia Completada (FR-025).
