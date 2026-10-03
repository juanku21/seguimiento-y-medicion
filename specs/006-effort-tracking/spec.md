# Especificación de Feature: Registro de Esfuerzo

**Directorio de feature**: `specs/006-effort-tracking`

**Rama**: `006-effort-tracking`

**Creada**: 2026-10-02

**Estado**: Clarificada — sin preguntas abiertas (sesión de clarificación del 2026-10-02)

**Entrada**: Descripción del usuario: "Registro de esfuerzo para Software Metrics & Estimation, un
sistema web multiusuario para estimar, planificar, seguir y medir proyectos de software con Scrum.
Depende de: gestión de proyectos e integrantes (factor de horas por Story Point); Product Backlog;
gestión de Sprints (sprints Planificado / Activo / Cerrado)."

---

## Objetivo

Permitir que cada integrante registre las horas que efectivamente trabajó sobre las historias del
proyecto, con qué fecha y en qué actividad, para poder comparar el esfuerzo estimado —Story Points
multiplicados por el factor de horas por Story Point del proyecto— contra el esfuerzo real.

La estimación en Story Points expresa tamaño relativo, no tiempo. El factor de horas por Story Point
es el traductor que el proyecto define entre una cosa y la otra, pero esa traducción es una hipótesis
hasta que alguien la contrasta con el trabajo que realmente costó. El registro de esfuerzo es el
único lugar del producto donde entra ese dato real. Sin él, el sistema puede decir cuántos puntos se
comprometieron y cuántos se completaron, pero no puede decir si el equipo estima bien ni cuánto se
desvía: la desviación entre lo estimado y lo real, que es una de las razones de ser del producto,
sencillamente no existe como número.

Por eso el registro tiene dos propiedades que lo distinguen de una simple carga de horas. La primera
es que cada registro pertenece a quien lo carga y a nadie más: las horas son un testimonio personal
de trabajo, no un dato administrativo que otro pueda escribir en nombre de un tercero. La segunda es
que cada registro queda anclado al sprint en el que estaba la historia en ese momento, y ese ancla no
se mueve nunca más. Si el ancla se recalculara cada vez que una historia cambia de sprint, el
esfuerzo de un sprint cerrado cambiaría retroactivamente y ninguna métrica por sprint sería
comparable con la siguiente.

---

## Clarifications

### Session 2026-10-02

- Q: ¿Se puede registrar esfuerzo sobre una historia que quedó Completada dentro de un sprint ya Cerrado, por ejemplo cuando alguien carga el lunes las horas del viernes y el sprint se cerró el viernes a la tarde? (FR-016) → A: sí, dentro de un plazo de gracia posterior al cierre; vencido el plazo, el registro se rechaza. Durante ese plazo el esfuerzo del sprint sigue siendo corregible y recién después queda definitivo.
- Q: Dentro del plazo de gracia, ¿las horas de una historia que no se completó y volvió al backlog cuentan para el sprint que acaba de cerrarse o quedan sin sprint? (FR-014, RN-24) → A: cuentan para ese sprint; el registro se reancla al sprint del que la historia volvió, tomando de la instantánea de cierre la lista de historias que el sprint tenía. El plazo de gracia alcanza por igual a las historias Completadas y a las devueltas al backlog.
- Q: Si un mismo integrante envía dos registros a la vez para la misma fecha y entre los dos superan las 24 horas, ¿el sistema debe garantizar que uno se rechace? (FR-005) → A: sí; el tope diario es una invariante que se sostiene aun ante pedidos simultáneos, igual que el límite de un solo sprint Activo por proyecto de `specs/004-sprint-management` (su SC-005). No alcanza con validar al registrar.

---

## Entradas y Salidas Esperadas

### Registro de esfuerzo sobre una historia

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador de la historia, fecha, actividad realizada y horas trabajadas | **Éxito**: registro creado, atribuido al titular de la sesión, asociado al sprint en el que estaba la historia en ese momento o a ninguno si estaba solo en el backlog |
| | **Error de validación**: detalle de qué campo es inválido y por qué; no se crea nada |
| | **Tope diario superado**: rechazo indicando cuántas horas quedan disponibles para esa fecha |
| | **Historia de un sprint Cerrado dentro del plazo de gracia**: registro creado y asociado a ese sprint Cerrado |
| | **Historia de un sprint Cerrado con el plazo de gracia vencido**: rechazo indicando que el sprint ya es definitivo |
| | **Proyecto Finalizado**: rechazo por proyecto de solo lectura |
| | **Quien pide no es integrante**: respuesta de historia inexistente |

### Modificación de un registro propio

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión del autor del registro, identificador del registro y los datos a modificar (fecha, actividad, horas) | **Éxito**: registro actualizado; su autor, su historia y su sprint asociado no cambian |
| | **Error de validación o tope diario superado**: rechazo con el motivo; el registro queda como estaba |
| | **Registro ajeno**: rechazo por falta de permiso; el registro no se toca |
| | **Registro de un sprint Cerrado con el plazo de gracia vencido**: rechazo indicando que ese esfuerzo ya es definitivo |

### Eliminación de un registro propio

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión del autor del registro e identificador del registro | **Éxito**: el registro deja de existir y deja de contar en los totales de la historia, del sprint y del día |
| | **Registro ajeno, o de un sprint Cerrado con el plazo de gracia vencido**: rechazo con el motivo; el registro no se toca |
| | **Proyecto Finalizado**: rechazo por proyecto de solo lectura |

### Consulta del esfuerzo de una historia

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador de la historia | Total de horas reales de la historia, desglose por integrante con su nombre y sus horas, y la lista de registros con fecha, autor, actividad, horas y sprint asociado |
| Historia sin ningún registro | Total en 0 horas, desglose vacío y lista vacía, con la indicación de que todavía no hay esfuerzo registrado |
| Historia con registros de alguien que ya no es integrante | Los registros se muestran igual, con el nombre de quien los cargó |

### Consulta del esfuerzo del proyecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador del proyecto y filtros opcionales por integrante, historia, sprint y rango de fechas | Lista de registros que cumplen todos los filtros, en orden estable, con su total de horas |
| Combinación de filtros sin coincidencias | Lista vacía con su total en 0 horas, sin que eso sea un error |
| Filtro con un valor inválido o de otro proyecto | Rechazo indicando qué filtro es inválido |

### Comparación entre esfuerzo estimado y esfuerzo real

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador del proyecto | Por cada historia: Story Points, horas estimadas, horas reales, diferencia absoluta y diferencia porcentual |
| Historia "sin estimar" | Horas estimadas, diferencia absoluta y diferencia porcentual como "no calculable", nunca como 0 |
| Historia estimada en 0 Story Points | Horas estimadas 0, diferencia absoluta igual a las horas reales y diferencia porcentual "no calculable" |
| Historia sin registros de esfuerzo | Horas reales 0 y la diferencia calculada contra las horas estimadas, sin que eso sea un error |

---

## Escenarios de Usuario y Pruebas *(obligatorio)*

### Historia de Usuario 1 (US1) — Registrar el esfuerzo trabajado (Prioridad: P1)

Un integrante anota, sobre una historia del proyecto, cuántas horas trabajó, qué día y en qué
actividad concreta.

**Por qué esta prioridad**: es el único punto de entrada del dato real al sistema. Sin registros no
hay nada que listar, nada que corregir y nada que comparar: todas las demás historias de esta feature
dependen de que esta exista.

**Prueba independiente**: se puede probar completa registrando horas sobre una historia de un
proyecto y verificando que el registro queda atribuido a quien lo cargó, con el sprint correcto y con
las validaciones de horas, fecha y actividad aplicadas.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un integrante de un proyecto En curso y una historia comprometida en el
   sprint Activo, **cuando** registra 6,5 horas con fecha de hoy y la actividad "Implementación del
   formulario de alta", **entonces** el registro queda creado, atribuido a él, asociado al sprint
   Activo, y las horas reales de la historia aumentan en 6,5.
2. *(Caso alternativo)* **Dado** una historia que está solo en el backlog, sin sprint, **cuando** un
   integrante registra esfuerzo sobre ella, **entonces** el registro se crea sin sprint asociado y
   queda fuera de las métricas por sprint, pero sí dentro de las de la historia y del proyecto.
3. *(Caso alternativo)* **Dado** una historia comprometida en un sprint Planificado que todavía no
   arrancó, **cuando** un integrante registra esfuerzo sobre ella, **entonces** el registro se crea
   asociado a ese sprint Planificado.
4. *(Caso alternativo)* **Dado** una historia en estado Completada dentro de un sprint todavía
   Activo, **cuando** un integrante registra esfuerzo sobre ella, **entonces** el registro se acepta,
   porque una historia terminada puede seguir recibiendo horas mientras el sprint no se cierre.
5. *(Caso alternativo)* **Dado** un integrante que ya registró 3 horas hoy en la historia A,
   **cuando** registra otras 2 horas hoy en la misma historia con otra actividad, **entonces** el
   segundo registro se crea y la historia queda con 5 horas de ese integrante en esa fecha.
6. *(Caso alternativo)* **Dado** una historia "sin estimar", **cuando** un integrante registra
   esfuerzo sobre ella, **entonces** el registro se acepta, porque registrar horas no exige que la
   historia esté estimada.
7. *(Caso alternativo)* **Dado** una historia Completada dentro de un sprint que se cerró el viernes
   a las 18:00, con un plazo de gracia de 48 horas, **cuando** un integrante registra el lunes a las
   9:00 las horas que trabajó el viernes, **entonces** el registro se acepta y queda asociado a ese
   sprint Cerrado, porque el plazo de gracia todavía está vigente.
8. *(Caso alternativo)* **Dado** una historia del mismo sprint que no se completó y por eso volvió al
   backlog al cerrarse, **cuando** un integrante registra el lunes a las 9:00 las horas que trabajó
   el viernes, **entonces** el registro se acepta y también queda asociado a ese sprint Cerrado,
   porque la historia le pertenecía al momento del cierre y el plazo de gracia sigue vigente.
9. *(Caso límite)* **Dado** el mismo sprint cerrado el viernes a las 18:00, **cuando** un integrante
   registra esfuerzo el domingo a las 17:59, **entonces** el registro se acepta por un minuto; y
   **cuando** lo intenta a las 18:01, se rechaza porque el plazo venció.
10. *(Caso límite)* **Dado** un integrante sin horas cargadas en una fecha, **cuando** registra
    exactamente 24 horas en esa fecha, **entonces** el registro se acepta, porque 24 es el máximo
    admitido y no se supera el tope diario.
11. *(Caso límite)* **Dado** un integrante sin horas cargadas en una fecha, **cuando** registra 0,25
    horas, **entonces** el registro se acepta, porque es el mínimo valor admitido por la escala.
12. *(Caso límite)* **Dado** un integrante, **cuando** registra esfuerzo con una actividad de
    exactamente 300 caracteres, **entonces** el registro se crea; y **cuando** tiene 301, se rechaza.
13. *(Caso límite)* **Dado** un proyecto cuya fecha de inicio es el 2026-10-01, **cuando** un
    integrante registra esfuerzo con fecha 2026-10-01, **entonces** el registro se acepta, porque el
    día de inicio del proyecto es una fecha válida.
14. *(Caso límite)* **Dado** un integrante, **cuando** registra esfuerzo con la fecha de hoy según la
    zona horaria del sistema, **entonces** el registro se acepta, porque hoy no es una fecha futura.
15. *(Caso límite)* **Dado** un integrante que ya tiene 20 horas registradas en una fecha repartidas
    entre dos proyectos distintos, **cuando** registra 4 horas más en esa misma fecha en un tercer
    proyecto, **entonces** el registro se acepta y llega justo al tope de 24.
16. *(Caso de error)* **Dado** un integrante con 20 horas ya registradas en una fecha contando todos
    sus proyectos, **cuando** intenta registrar 5 horas más en esa fecha, **entonces** el registro se
    rechaza indicando que solo quedan 4 horas disponibles para ese día, y nada se crea.
17. *(Caso de error)* **Dado** un integrante, **cuando** intenta registrar 0 horas, un valor negativo
    o más de 24 horas, **entonces** el registro se rechaza indicando el rango admitido.
18. *(Caso de error)* **Dado** un integrante, **cuando** intenta registrar 1,3 horas, **entonces** el
    registro se rechaza indicando que las horas se cargan en múltiplos de 0,25.
19. *(Caso de error)* **Dado** un integrante, **cuando** intenta registrar esfuerzo sin actividad o
    con una actividad compuesta solo por espacios, **entonces** el registro se rechaza indicando que
    la actividad es obligatoria.
20. *(Caso de error)* **Dado** un integrante, **cuando** intenta registrar esfuerzo con la fecha de
    mañana, **entonces** el registro se rechaza indicando que no se registran horas en el futuro.
21. *(Caso de error)* **Dado** un proyecto que empezó el 2026-10-01, **cuando** un integrante intenta
    registrar esfuerzo con fecha 2026-09-30, **entonces** el registro se rechaza indicando que la
    fecha es anterior al inicio del proyecto.
22. *(Caso de error)* **Dado** una historia Completada dentro de un sprint cerrado hace cinco días,
    con un plazo de gracia de 48 horas, **cuando** un integrante intenta registrar esfuerzo sobre
    ella, **entonces** el registro se rechaza indicando que el sprint ya es definitivo, y nada se
    crea.
23. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
    registrar esfuerzo sobre una de sus historias, **entonces** el registro se rechaza porque el
    proyecto es de solo lectura, aunque el plazo de gracia de su último sprint siga vigente.
24. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
    registrar esfuerzo sobre una de sus historias, **entonces** la respuesta es de historia
    inexistente y no se crea nada.
25. *(Caso de error)* **Dado** un integrante, **cuando** intenta registrar esfuerzo indicando a otra
    persona como autor, **entonces** el sistema ignora ese dato y atribuye el registro al titular de
    la sesión; el registro nunca queda a nombre de un tercero.

---

### Historia de Usuario 2 (US2) — Consultar el esfuerzo de una historia (Prioridad: P1)

Un integrante abre una historia y ve cuántas horas lleva acumuladas en total y cuánto puso cada
persona del equipo.

**Por qué esta prioridad**: es la lectura mínima que convierte el registro en información útil. Sin
ella, las horas entran al sistema y desaparecen de la vista de quien las cargó, que ni siquiera puede
verificar que quedaron bien.

**Prueba independiente**: se puede probar completa cargando registros de dos integrantes sobre una
misma historia y verificando el total y el desglose por persona.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una historia con tres registros de un integrante que suman 10 horas y dos
   registros de otro que suman 6, **cuando** un integrante consulta el esfuerzo de la historia,
   **entonces** ve un total de 16 horas y un desglose con 10 horas para el primero y 6 para el
   segundo.
2. *(Caso alternativo)* **Dado** una historia con registros de dos sprints distintos y uno sin
   sprint, **cuando** un integrante consulta su esfuerzo, **entonces** ve todos los registros con el
   sprint al que está asociado cada uno, y el total los incluye a todos.
3. *(Caso alternativo)* **Dado** una historia con registros de una persona que fue quitada del
   proyecto, **cuando** un integrante consulta su esfuerzo, **entonces** esos registros siguen
   visibles con el nombre de quien los cargó y siguen contando en el total.
4. *(Caso límite)* **Dado** una historia sin ningún registro de esfuerzo, **cuando** un integrante
   consulta su esfuerzo, **entonces** ve un total de 0 horas, un desglose vacío y la indicación de
   que todavía no hay esfuerzo registrado, sin que eso sea un error.
5. *(Caso límite)* **Dado** una historia con un único registro de 0,25 horas, **cuando** un
   integrante consulta su esfuerzo, **entonces** el total muestra 0,25 horas sin pérdida de
   precisión.
6. *(Caso límite)* **Dado** una historia cuyo único integrante con registros cargó esfuerzo en tres
   fechas distintas, **cuando** un integrante consulta el desglose, **entonces** ve una sola línea
   para esa persona con la suma de las tres fechas, y las tres fechas por separado en la lista de
   registros.
7. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
   consultar el esfuerzo de una de sus historias, **entonces** la respuesta es de historia
   inexistente y no revela ningún dato.
8. *(Caso de error)* **Dado** un identificador de historia que no existe, **cuando** un integrante
   intenta consultar su esfuerzo, **entonces** obtiene una respuesta de historia inexistente.

---

### Historia de Usuario 3 (US3) — Corregir o eliminar un registro propio (Prioridad: P2)

Un integrante que se equivocó al cargar sus horas corrige la fecha, la actividad o la cantidad, o
borra el registro si nunca debió existir.

**Por qué esta prioridad**: la corrección es lo que hace confiable al dato. Un registro equivocado que
no se puede arreglar contamina para siempre la comparación entre lo estimado y lo real. Va después de
registrar y de consultar porque necesita que ambas existan.

**Prueba independiente**: se puede probar completa creando un registro, modificándolo, verificando
que los totales se actualizan, eliminándolo y verificando que desaparece de los totales.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un registro propio de 4 horas asociado a un sprint Activo, **cuando** su
   autor lo corrige a 6 horas y cambia la actividad, **entonces** el registro queda actualizado, el
   total de la historia sube 2 horas y su autor, su historia y su sprint asociado no cambian.
2. *(Caso normal)* **Dado** un registro propio asociado a un sprint Activo, **cuando** su autor lo
   elimina, **entonces** el registro deja de existir y deja de contar en el total de la historia, en
   el del sprint y en el del día.
3. *(Caso alternativo)* **Dado** un registro propio cargado con la fecha equivocada, **cuando** su
   autor le cambia la fecha a otra fecha válida, **entonces** el cambio se acepta, el tope diario se
   vuelve a verificar sobre la fecha nueva y el sprint asociado al registro no cambia.
4. *(Caso alternativo)* **Dado** un registro asociado a un sprint que todavía estaba Planificado al
   crearse y que ahora está Activo, **cuando** su autor lo modifica, **entonces** el cambio se acepta
   y el registro sigue asociado al mismo sprint.
5. *(Caso alternativo)* **Dado** un registro de una historia que después se quitó del sprint y volvió
   al backlog, **cuando** su autor lo modifica, **entonces** el cambio se acepta y el registro
   conserva el sprint al que se asoció al crearse.
6. *(Caso límite)* **Dado** un integrante con 24 horas registradas en una fecha, de las cuales 6 son
   de un registro propio, **cuando** reduce ese registro a 4 horas, **entonces** el cambio se acepta
   y el día queda con 22 horas.
7. *(Caso límite)* **Dado** un integrante con 24 horas registradas en una fecha, de las cuales 6 son
   de un registro propio, **cuando** intenta subir ese registro a 8 horas, **entonces** el cambio se
   rechaza porque el día llegaría a 26, y el registro queda en 6.
8. *(Caso límite)* **Dado** un registro propio que es el único de su historia, **cuando** su autor lo
   elimina, **entonces** la historia vuelve a quedar con 0 horas reales y sin desglose, sin que eso
   sea un error.
9. *(Caso límite)* **Dado** un registro propio asociado a un sprint que se cerró hace 10 horas, con
   un plazo de gracia de 48 horas, **cuando** su autor lo corrige o lo elimina, **entonces** la
   acción se acepta, porque el esfuerzo del sprint sigue siendo corregible hasta que venza el plazo.
10. *(Caso límite)* **Dado** un registro propio asociado a un sprint cerrado hace exactamente 48
    horas con ese mismo plazo de gracia, **cuando** su autor intenta corregirlo, **entonces** la
    acción se rechaza, porque el plazo se cuenta cumplido al alcanzarse.
11. *(Caso de error)* **Dado** un registro de otro integrante, **cuando** alguien intenta modificarlo
    o eliminarlo, **entonces** la acción se rechaza por falta de permiso y el registro no se toca,
    aun cuando quien lo intenta sea el propietario del proyecto y el plazo de gracia siga vigente.
12. *(Caso de error)* **Dado** un registro propio asociado a un sprint Cerrado cuyo plazo de gracia
    venció, **cuando** su autor intenta modificarlo, **entonces** la acción se rechaza indicando que
    ese esfuerzo ya es definitivo.
13. *(Caso de error)* **Dado** un registro propio asociado a un sprint Cerrado cuyo plazo de gracia
    venció, **cuando** su autor intenta eliminarlo, **entonces** la acción se rechaza por el mismo
    motivo y el registro se conserva.
14. *(Caso de error)* **Dado** un registro propio sin sprint asociado, de una historia que nunca entró
    a un sprint, **cuando** su autor lo modifica o lo elimina, **entonces** la acción se acepta,
    porque la inmutabilidad depende del sprint asociado y este registro no tiene ninguno.
15. *(Caso de error)* **Dado** un registro propio, **cuando** su autor intenta modificarlo poniendo
    0 horas, 25 horas, 1,1 horas, una actividad vacía, una fecha futura o una fecha anterior al inicio
    del proyecto, **entonces** el cambio se rechaza con el motivo y el registro queda como estaba.
16. *(Caso de error)* **Dado** un registro propio, **cuando** su autor intenta cambiarle la historia o
    el integrante al que pertenece, **entonces** la acción se rechaza indicando que esos datos no se
    modifican; para mover horas a otra historia hay que eliminar el registro y crear uno nuevo.
17. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
    modificar o eliminar un registro propio, **entonces** la acción se rechaza porque el proyecto es
    de solo lectura.
18. *(Caso de error)* **Dado** una persona que fue quitada del proyecto, **cuando** intenta modificar
    o eliminar los registros que había cargado, **entonces** la acción se rechaza con una respuesta
    de registro inexistente, y los registros siguen visibles para los integrantes vigentes.

---

### Historia de Usuario 4 (US4) — Consultar el esfuerzo del proyecto con filtros (Prioridad: P2)

Un integrante recorre todo el esfuerzo cargado en el proyecto y lo acota por persona, por historia,
por sprint o por un rango de fechas para responder una pregunta concreta.

**Por qué esta prioridad**: es la vista que permite leer el esfuerzo a escala de proyecto, no de una
historia aislada. Es útil desde el primer sprint, pero el equipo puede operar sin ella si tiene la
consulta por historia.

**Prueba independiente**: se puede probar completa cargando registros de varios integrantes, varias
historias y varios sprints, y verificando que cada filtro y cada combinación de filtros devuelve
exactamente el subconjunto esperado con su total.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto con registros de tres integrantes en dos sprints, **cuando**
   un integrante consulta el esfuerzo del proyecto sin filtros, **entonces** ve todos los registros
   con su fecha, autor, historia, actividad, horas y sprint, y el total general de horas.
2. *(Caso alternativo)* **Dado** el mismo proyecto, **cuando** un integrante filtra por un integrante
   concreto, **entonces** ve solo los registros de esa persona y el total correspondiente.
3. *(Caso alternativo)* **Dado** el mismo proyecto, **cuando** un integrante filtra por un sprint
   concreto, **entonces** ve solo los registros asociados a ese sprint, sin importar en qué sprint
   esté hoy cada historia.
4. *(Caso alternativo)* **Dado** el mismo proyecto, **cuando** un integrante filtra por un rango de
   fechas y por una historia a la vez, **entonces** ve solo los registros que cumplen ambas
   condiciones.
5. *(Caso alternativo)* **Dado** un proyecto con registros sin sprint asociado, **cuando** un
   integrante filtra por "sin sprint", **entonces** ve únicamente los registros que se cargaron
   cuando la historia estaba solo en el backlog.
6. *(Caso límite)* **Dado** un rango de fechas cuyo inicio y cuyo fin son el mismo día, **cuando** un
   integrante lo aplica, **entonces** ve los registros de ese único día, porque los extremos del
   rango se incluyen.
7. *(Caso límite)* **Dado** una combinación de filtros que ninguna fila cumple, **cuando** un
   integrante la aplica, **entonces** obtiene una lista vacía con total 0 horas y una indicación
   clara, sin que eso sea un error.
8. *(Caso límite)* **Dado** un proyecto donde todavía nadie cargó horas, **cuando** un integrante
   consulta su esfuerzo, **entonces** obtiene una lista vacía con total 0 horas.
9. *(Caso límite)* **Dado** un filtro por una persona que fue quitada del proyecto, **cuando** un
   integrante lo aplica, **entonces** ve los registros que esa persona dejó, porque la baja no borra
   su historial.
10. *(Caso de error)* **Dado** un rango de fechas cuyo fin es anterior a su inicio, **cuando** un
    integrante lo aplica, **entonces** la consulta se rechaza indicando que el fin no puede ser
    anterior al inicio.
11. *(Caso de error)* **Dado** un filtro por una historia o por un sprint que pertenece a otro
    proyecto, **cuando** un integrante lo aplica, **entonces** la consulta se rechaza indicando que
    el filtro no corresponde al proyecto, sin revelar dato alguno del otro proyecto.
12. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
    consultar su esfuerzo, **entonces** la respuesta es de proyecto inexistente.

---

### Historia de Usuario 5 (US5) — Comparar el esfuerzo estimado con el real (Prioridad: P3)

Un integrante mira, historia por historia, cuántas horas se habían estimado a partir de los Story
Points y cuántas costó realmente, y cuánto se desvió una cosa de la otra.

**Por qué esta prioridad**: es la razón de fondo por la que se registran las horas, pero es
estrictamente una lectura de datos que las historias anteriores ya produjeron. El equipo puede
registrar y consultar esfuerzo sin esta vista; no puede tenerla sin las anteriores.

**Prueba independiente**: se puede probar completa con historias estimadas, "sin estimar" y estimadas
en 0, con y sin registros, verificando las horas estimadas, las reales y las dos diferencias en cada
combinación.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto con factor 6 horas por Story Point y una historia de 5 Story
   Points con 36 horas reales registradas, **cuando** un integrante consulta la comparación,
   **entonces** ve 30 horas estimadas, 36 reales, una diferencia absoluta de +6 horas y una
   diferencia porcentual de +20 %.
2. *(Caso alternativo)* **Dado** la misma historia con 24 horas reales, **cuando** un integrante
   consulta la comparación, **entonces** ve una diferencia absoluta de −6 horas y una porcentual de
   −20 %, de modo que el signo distingue el exceso del ahorro.
3. *(Caso alternativo)* **Dado** una historia cuyas horas reales coinciden exactamente con las
   estimadas, **cuando** un integrante consulta la comparación, **entonces** ve una diferencia
   absoluta de 0 horas y una porcentual de 0 %.
4. *(Caso alternativo)* **Dado** un proyecto cuyo propietario cambia el factor de horas por Story
   Point de 6 a 8, **cuando** un integrante consulta la comparación, **entonces** las horas estimadas
   de todas las historias se recalculan con el factor nuevo y las horas reales no cambian.
5. *(Caso límite)* **Dado** una historia "sin estimar" con 12 horas reales registradas, **cuando** un
   integrante consulta la comparación, **entonces** ve 12 horas reales y las horas estimadas, la
   diferencia absoluta y la porcentual como "no calculable", nunca como 0.
6. *(Caso límite)* **Dado** una historia estimada en 0 Story Points con 4 horas reales registradas,
   **cuando** un integrante consulta la comparación, **entonces** ve 0 horas estimadas, 4 reales, una
   diferencia absoluta de +4 horas y la diferencia porcentual como "no calculable", porque no hay
   base contra la cual calcular el porcentaje.
7. *(Caso límite)* **Dado** una historia de 3 Story Points sin ningún registro de esfuerzo, **cuando**
   un integrante consulta la comparación, **entonces** ve las horas estimadas, 0 horas reales, la
   diferencia absoluta igual a las horas estimadas en negativo y una diferencia porcentual de −100 %.
8. *(Caso límite)* **Dado** una historia estimada en 0 Story Points y sin registros, **cuando** un
   integrante consulta la comparación, **entonces** ve 0 estimadas, 0 reales, diferencia absoluta 0 y
   diferencia porcentual "no calculable".
9. *(Caso límite)* **Dado** un proyecto cuyas historias están todas "sin estimar", **cuando** un
   integrante consulta la comparación, **entonces** ve la lista completa con todas las desviaciones
   como "no calculable", sin que eso sea un error.
10. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
    consultar la comparación, **entonces** la respuesta es de proyecto inexistente.
11. *(Caso de error)* **Dado** un identificador de historia que no existe, **cuando** un integrante
    intenta consultar su comparación, **entonces** obtiene una respuesta de historia inexistente.

---

### Casos Límite

- **Registro de exactamente 24 horas en un día**: se acepta, porque 24 es el máximo admitido por
  registro y coincide con el tope diario; el día queda completo y cualquier registro posterior en esa
  fecha se rechaza.
- **Segundo registro del mismo día que llevaría el total a más de 24**: se rechaza indicando cuántas
  horas quedan disponibles; el primer registro no se altera.
- **Tope diario alcanzado repartido entre varios proyectos**: el límite de 24 horas es de la persona,
  no del proyecto, así que un día completo en un proyecto bloquea el registro en todos los demás.
- **Dos registros simultáneos del mismo integrante para la misma fecha**: si entre ambos superan las
  24 horas, uno se acepta y el otro se rechaza; nunca quedan los dos. Vale igual cuando llegan desde
  proyectos distintos o cuando uno es un alta y el otro una modificación.
- **Historia estimada en 0 Story Points con horas reales registradas**: la diferencia absoluta es
  computable e igual a las horas reales; la porcentual no lo es y se informa como "no calculable".
- **Historia sin ningún registro de esfuerzo**: devuelve 0 horas reales y desglose vacío, no un error;
  en la comparación, la desviación absoluta es igual a las horas estimadas en negativo.
- **Historia "sin estimar" con horas reales**: las horas reales se muestran, las estimadas y ambas
  desviaciones son "no calculable".
- **Registro sobre una historia Completada dentro de un sprint todavía Activo**: se acepta; el estado
  de la historia no condiciona el registro de horas, solo lo condicionan el estado del sprint
  asociado y el del proyecto.
- **Integrante quitado del proyecto**: sus registros siguen visibles, siguen sumando a los totales y
  siguen mostrando su nombre; él pierde el acceso y ya no puede modificarlos ni eliminarlos.
- **Integrante quitado y reincorporado al proyecto**: recupera el acceso a sus registros anteriores y
  puede modificarlos, siempre que su sprint asociado no esté Cerrado.
- **Historia que cambia de sprint después de tener esfuerzo registrado**: los registros existentes
  conservan el sprint original; solo los registros nuevos se asocian al sprint nuevo.
- **Historia que se quita del sprint y vuelve al backlog**: los registros que tenía siguen asociados a
  aquel sprint; los que se carguen después quedan sin sprint.
- **Sprint que se cierra entre dos registros de la misma historia**: mientras el plazo de gracia
  siga vigente, ambos registros se asocian al mismo sprint Cerrado y ambos siguen siendo
  corregibles; vencido el plazo, los dos quedan definitivos a la vez.
- **Registro sobre una historia Completada de un sprint recién cerrado**: se acepta dentro del plazo
  de gracia y se rechaza después; es el caso que motiva la existencia del plazo, porque el trabajo
  del último día del sprint se suele cargar al día siguiente.
- **Plazo de gracia que vence justo en el instante del intento**: el plazo se cuenta cumplido al
  alcanzarse, así que un intento exactamente en el límite se rechaza.
- **Historia no completada al cierre del sprint**: vuelve al backlog, pero el esfuerzo que se cargue
  dentro del plazo de gracia se reancla igual a ese sprint, de modo que las horas del último día
  cuentan para él aunque la historia ya no le pertenezca. Vencido el plazo, el esfuerzo sobre esa
  historia se acepta sin sprint asociado.
- **Historia devuelta al backlog que estuvo en dos sprints cerrados dentro de su plazo de gracia**:
  el registro se ancla al de cierre más reciente, que es el que corresponde al trabajo que se está
  cargando.
- **Historia devuelta al backlog y vuelta a comprometer en un sprint nuevo dentro del plazo de
  gracia del anterior**: prevalece el sprint en el que la historia está comprometida ahora, porque
  el reanclaje solo opera cuando la historia quedó en el backlog.
- **Proyecto que se finaliza mientras el plazo de gracia sigue vigente**: el estado Finalizado manda
  y bloquea todo registro, corrección y eliminación; el plazo de gracia no lo reabre.
- **Proyecto cuya fecha de inicio es futura**: no existe ninguna fecha válida para registrar esfuerzo
  hasta que llegue el día de inicio, porque la fecha no puede ser ni futura ni anterior al inicio.
- **Registro con fecha anterior a la incorporación de la persona al proyecto**: se acepta mientras no
  sea anterior al inicio del proyecto; el sistema no condiciona la fecha a la antigüedad de cada
  integrante.
- **Varios registros del mismo integrante, en la misma historia y la misma fecha**: se aceptan como
  registros independientes, porque corresponden a actividades distintas; solo el tope diario los
  relaciona.
- **Modificación que reduce las horas de un día ya completo**: libera capacidad para ese día de
  inmediato.
- **Eliminación del último registro de una historia**: la historia vuelve al estado de "sin esfuerzo
  registrado"; esto no la vuelve eliminable, porque haber tenido esfuerzo no se revierte.
- **Rango de fechas de un solo día en los filtros**: incluye ese día, porque ambos extremos del rango
  son inclusivos.
- **Totales con fracciones de cuarto de hora**: 0,25 + 0,25 + 0,25 suman exactamente 0,75, sin errores
  de redondeo acumulados.

---

## Requisitos *(obligatorio)*

### Requisitos Funcionales

**Registro de esfuerzo**

- **FR-001**: El sistema DEBE permitir a cualquier integrante vigente de un proyecto registrar
  esfuerzo sobre cualquier historia de ese proyecto, indicando fecha, actividad realizada y horas
  trabajadas.
- **FR-002**: El sistema DEBE atribuir todo registro al titular de la sesión que lo crea, e ignorar
  cualquier intento de indicar a otra persona como integrante del registro.
- **FR-003**: El sistema DEBE exigir una actividad de entre 1 y 300 caracteres, contados después de
  recortar los espacios al inicio y al final; una actividad ausente o compuesta solo por espacios se
  rechaza.
- **FR-004**: El sistema DEBE exigir una cantidad de horas estrictamente mayor que 0, menor o igual a
  24 y múltiplo exacto de 0,25.
- **FR-005**: El sistema DEBE rechazar un registro cuando la suma de las horas que el integrante ya
  tiene cargadas en esa misma fecha, contando todos los proyectos en los que haya registrado
  esfuerzo, más las horas del registro nuevo, supere 24. El rechazo DEBE indicar cuántas horas quedan
  disponibles para esa fecha, sin nombrar los otros proyectos. El tope DEBE sostenerse también
  cuando varios registros del mismo integrante para la misma fecha llegan de forma simultánea: el
  sistema DEBE rechazar los que excedan las 24 horas, de modo que en ningún caso una fecha quede con
  más de 24 horas cargadas. No alcanza con validar cada pedido por separado contra el total leído
  antes de procesarlo.
- **FR-006**: El sistema DEBE exigir una fecha que no sea posterior al día de hoy ni anterior a la
  fecha de inicio del proyecto al que pertenece la historia; ambos extremos son fechas válidas.
- **FR-007**: El sistema DEBE determinar cuál es el día de hoy según la zona horaria única y
  configurable del sistema, independientemente de la zona horaria del dispositivo de quien registra.
- **FR-008**: El sistema DEBE rechazar todo registro de esfuerzo sobre historias de un proyecto en
  estado Finalizado.
- **FR-009**: El sistema DEBE permitir registrar esfuerzo sobre una historia cualquiera sea su estado
  (Pendiente, En progreso o Completada) y cualquiera sea su estimación, incluida la marca "sin
  estimar".
- **FR-010**: El sistema DEBE permitir varios registros del mismo integrante sobre la misma historia y
  la misma fecha, tratándolos como registros independientes sujetos únicamente al tope diario de
  FR-005.
- **FR-011**: Cada registro DEBE tener un identificador propio, estable, inmutable y no secuencial.
- **FR-012**: El sistema DEBE registrar en cada registro de esfuerzo su momento de creación y su
  momento de última actualización.
- **FR-013**: El sistema DEBE rechazar un registro inválido indicando qué campo corregir y por qué,
  sin crear nada; la validación DEBE ser atómica.

**Asociación al sprint**

- **FR-014**: Al crear un registro, el sistema DEBE asociarlo automáticamente al sprint en el que
  estaba comprometida la historia en ese momento, sea ese sprint Planificado o Activo, o dejarlo sin
  sprint si la historia estaba solo en el backlog. Por excepción, cuando la historia volvió al
  backlog al cerrarse un sprint y ese sprint sigue dentro de su plazo de gracia (FR-016), el sistema
  DEBE asociar el registro a ese sprint Cerrado en lugar de dejarlo sin sprint, determinando la
  pertenencia con la lista de historias que el sprint tenía al cerrarse
  (`specs/004-sprint-management`, FR-042). Si la historia figura en más de un sprint en esa
  condición, DEBE prevalecer el de cierre más reciente. Vencido el plazo, el registro sobre esa
  historia vuelve a quedar sin sprint, porque la historia ya está solo en el backlog.
- **FR-015**: La asociación entre un registro y su sprint DEBE ser inmutable: no cambia cuando la
  historia se quita del sprint, cuando se compromete en otro, cuando el sprint se cierra ni cuando el
  registro se modifica. Esa asociación es la que las métricas por sprint DEBEN usar.
- **FR-016**: El plazo de gracia de un sprint corre desde su momento real de cierre y dura un
  período configurable, único para todo el sistema, cuyo valor por defecto es de 48 horas. El plazo
  se cuenta cumplido al alcanzarse: un intento exactamente en el límite ya está fuera de plazo.
  Mientras el plazo siga vigente, el sistema DEBE admitir el registro de esfuerzo sobre cualquier
  historia que perteneciera al sprint al cerrarse y DEBE asociarlo a ese sprint Cerrado, tanto si la
  historia sigue en él por estar Completada como si volvió al backlog. Vencido el plazo, el
  comportamiento depende de dónde esté hoy la historia:
  - Si sigue perteneciendo al sprint Cerrado, por haber quedado Completada, el sistema DEBE rechazar
    el registro indicando que el sprint ya es definitivo.
  - Si volvió al backlog, el sistema DEBE aceptar el registro y dejarlo sin sprint asociado, porque
    la historia ya es una historia del backlog como cualquier otra.
- **FR-017**: El sistema NO DEBE permitir que un registro quede asociado a un sprint de un proyecto
  distinto del de su historia.

**Modificación y eliminación**

- **FR-018**: El sistema DEBE permitir al autor de un registro modificar su fecha, su actividad y sus
  horas.
- **FR-019**: El sistema NO DEBE permitir modificar el integrante, la historia ni el sprint asociado
  de un registro; para mover horas a otra historia hay que eliminar el registro y crear uno nuevo.
- **FR-020**: El sistema DEBE rechazar toda modificación y toda eliminación de un registro asociado a
  un sprint en estado Cerrado cuyo plazo de gracia ya venció, indicando que ese esfuerzo es
  definitivo.
- **FR-021**: El sistema DEBE permitir modificar y eliminar registros asociados a un sprint
  Planificado o Activo, registros asociados a un sprint Cerrado cuyo plazo de gracia sigue vigente,
  y registros sin sprint asociado. El plazo de gracia se evalúa con el mismo criterio de FR-016, de
  modo que registrar, corregir y eliminar esfuerzo de un sprint cerrado se habilitan y se cierran a
  la vez.
- **FR-022**: El sistema DEBE rechazar toda modificación y toda eliminación de un registro ajeno,
  incluso cuando quien lo intenta es el propietario del proyecto.
- **FR-023**: El sistema DEBE aplicar en la modificación exactamente las mismas validaciones de
  actividad, horas, rango, múltiplo, tope diario y fecha que en la creación (FR-003 a FR-007).
- **FR-024**: Al verificar el tope diario en una modificación, el sistema DEBE excluir del cómputo las
  horas vigentes del propio registro que se está modificando, y DEBE evaluar el tope sobre la fecha
  resultante del cambio. La garantía de FR-005 ante pedidos simultáneos rige por igual para las
  modificaciones y para cualquier combinación de altas y modificaciones concurrentes sobre la misma
  fecha del mismo integrante.
- **FR-025**: La modificación DEBE ser atómica: si algún dato es inválido, ningún cambio se aplica y
  el registro queda exactamente como estaba.
- **FR-026**: El sistema DEBE permitir al autor eliminar un registro propio; la eliminación DEBE
  quitarlo de inmediato de los totales de la historia, del sprint, del proyecto y del tope diario de
  esa fecha.
- **FR-027**: El sistema DEBE rechazar toda modificación y toda eliminación de registros cuando el
  proyecto está en estado Finalizado.
- **FR-028**: El sistema DEBE conservar los registros de una persona que fue quitada del proyecto, y
  DEBE impedirle modificarlos o eliminarlos mientras no sea integrante vigente.
- **FR-029**: El sistema NO DEBE eliminar registros de esfuerzo como efecto colateral de ninguna otra
  acción del producto, incluidas la baja de un integrante, la baja de una historia de un sprint y el
  cierre de un sprint.

**Consulta del esfuerzo de una historia**

- **FR-030**: El sistema DEBE permitir a cualquier integrante consultar el esfuerzo de una historia,
  devolviendo el total de horas reales, el desglose por integrante y la lista de registros.
- **FR-031**: El total de horas reales de una historia DEBE ser la suma de las horas de todos sus
  registros, cualquiera sea su autor, su fecha y su sprint asociado, incluidos los registros sin
  sprint.
- **FR-032**: El desglose por integrante DEBE incluir una línea por persona con registros en la
  historia, con su nombre y la suma de sus horas, incluidas las personas que ya no son integrantes.
- **FR-033**: Cada registro listado DEBE mostrar al menos fecha, autor, actividad, horas y sprint
  asociado, o la indicación de que no tiene sprint.
- **FR-034**: Cuando una historia no tiene registros, el sistema DEBE devolver total 0 horas, desglose
  vacío y lista vacía con una indicación clara, no un error.
- **FR-035**: El listado de registros de una historia DEBE presentarse en un orden estable y
  predecible: fecha descendente y, a igual fecha, por momento de creación.

**Consulta del esfuerzo del proyecto**

- **FR-036**: El sistema DEBE permitir a cualquier integrante listar el esfuerzo de un proyecto con
  filtros opcionales y combinables por integrante, por historia, por sprint y por rango de fechas.
- **FR-037**: El sistema DEBE combinar los filtros aplicados de forma conjuntiva: un registro aparece
  solo si cumple todos los filtros indicados.
- **FR-038**: El filtro por rango de fechas DEBE incluir ambos extremos, y DEBE admitir indicar solo
  el extremo inicial, solo el final o ambos.
- **FR-039**: El filtro por sprint DEBE operar sobre el sprint asociado al registro, no sobre el
  sprint en el que esté la historia al momento de consultar, y DEBE admitir el valor "sin sprint".
- **FR-040**: El filtro por integrante DEBE admitir también a las personas que ya no son integrantes
  vigentes del proyecto, para que su historial siga siendo consultable.
- **FR-041**: El listado DEBE devolver, además de los registros, el total de horas del conjunto
  filtrado.
- **FR-042**: Cuando ningún registro cumple los filtros, el sistema DEBE devolver una lista vacía con
  total 0 horas y una indicación clara, no un error.
- **FR-043**: El sistema DEBE rechazar un rango de fechas cuyo extremo final sea anterior al inicial,
  indicando el motivo.
- **FR-044**: El sistema DEBE rechazar un filtro por una historia, un sprint o un integrante que no
  pertenece al proyecto consultado, sin revelar ningún dato del proyecto ajeno.
- **FR-045**: El listado del proyecto DEBE presentarse en un orden estable y predecible: fecha
  descendente y, a igual fecha, por momento de creación.

**Comparación entre esfuerzo estimado y real**

- **FR-046**: El sistema DEBE calcular las horas estimadas de una historia como sus Story Points
  multiplicados por el factor de horas por Story Point vigente del proyecto.
- **FR-047**: El sistema DEBE calcular las horas reales de una historia como el total definido en
  FR-031.
- **FR-048**: El sistema DEBE calcular la diferencia absoluta como las horas reales menos las horas
  estimadas, con signo: positiva cuando se trabajó de más y negativa cuando se trabajó de menos.
- **FR-049**: El sistema DEBE calcular la diferencia porcentual como la diferencia absoluta dividida
  por las horas estimadas, expresada en porcentaje y con el mismo signo.
- **FR-050**: Cuando la historia está "sin estimar", el sistema DEBE presentar las horas estimadas, la
  diferencia absoluta y la diferencia porcentual como "no calculable", y NUNCA como 0.
- **FR-051**: Cuando las horas estimadas son 0 —historia estimada en 0 Story Points—, el sistema DEBE
  presentar las horas estimadas como 0, la diferencia absoluta como las horas reales y la diferencia
  porcentual como "no calculable".
- **FR-052**: El sistema DEBE incluir en la comparación las historias sin ningún registro de esfuerzo,
  con 0 horas reales.
- **FR-053**: El sistema DEBE permitir consultar la comparación de una historia individual y la de
  todas las historias del proyecto.
- **FR-054**: Las horas estimadas DEBEN recalcularse con el factor vigente del proyecto en cada
  consulta; un cambio de factor cambia las horas estimadas y las desviaciones, y nunca las horas
  reales.
- **FR-055**: La comparación DEBE distinguir de forma visible una historia estimada en 0 Story Points
  de una historia "sin estimar".

**Autorización y visibilidad**

- **FR-056**: El sistema DEBE exigir una sesión válida para toda acción de esta feature, sin
  excepciones.
- **FR-057**: El sistema DEBE restringir toda acción sobre el esfuerzo de un proyecto a sus
  integrantes vigentes.
- **FR-058**: El sistema DEBE permitir a todo integrante vigente consultar el esfuerzo de todos los
  demás integrantes dentro del proyecto; la restricción de autoría rige para escribir, no para leer.
- **FR-059**: Ante una solicitud sobre un proyecto, una historia o un registro a los que quien pide no
  tiene acceso, el sistema DEBE responder exactamente igual que ante un identificador inexistente, sin
  revelar ningún dato ni la existencia del recurso.
- **FR-060**: Cuando el proyecto está en estado Finalizado, el sistema DEBE tratar su esfuerzo como de
  solo lectura: rechaza registrar, modificar y eliminar, y permite consultar y comparar.
- **FR-061**: Toda creación, modificación y eliminación DEBE quedar atribuida al titular de la sesión
  que la ejecutó.

---

### Reglas de Negocio

| ID | Regla |
| --- | --- |
| RN-01 | Cada integrante registra únicamente su propio esfuerzo; el integrante del registro es siempre quien lo carga. |
| RN-02 | La actividad realizada es obligatoria y mide hasta 300 caracteres. |
| RN-03 | Las horas de un registro son mayores que 0, menores o iguales a 24 y múltiplos de 0,25. |
| RN-04 | La suma de horas de un integrante en una misma fecha no supera 24, contando todos sus proyectos, ni siquiera ante altas o modificaciones simultáneas. |
| RN-05 | La fecha de un registro no es futura ni anterior a la fecha de inicio del proyecto; ambos extremos son válidos. |
| RN-06 | No se registra esfuerzo en historias de un proyecto Finalizado, ni se modifica ni se elimina el ya registrado. |
| RN-07 | Cada registro queda asociado al sprint en el que estaba la historia al crearlo, o a ninguno si estaba solo en el backlog; por excepción, dentro del plazo de gracia se ancla al sprint del que la historia volvió. |
| RN-08 | La asociación entre un registro y su sprint no cambia nunca, y es la que usan las métricas por sprint. |
| RN-09 | Los registros asociados a un sprint Cerrado se modifican y se eliminan mientras siga vigente su plazo de gracia; vencido el plazo, son definitivos. |
| RN-10 | Un registro sin sprint asociado se modifica y se elimina mientras el proyecto no esté Finalizado. |
| RN-11 | Solo el autor de un registro lo modifica o lo elimina; el propietario del proyecto no es excepción. |
| RN-12 | El integrante, la historia y el sprint de un registro son inmutables; solo cambian la fecha, la actividad y las horas. |
| RN-13 | El esfuerzo se registra sobre una historia cualquiera sea su estado y su estimación. |
| RN-14 | Los registros de una persona quitada del proyecto se conservan, siguen visibles con su nombre y siguen sumando. |
| RN-15 | Todo integrante vigente ve el esfuerzo de todos los demás dentro del proyecto. |
| RN-16 | Las horas estimadas de una historia son sus Story Points por el factor de horas por Story Point del proyecto. |
| RN-17 | Si la historia está "sin estimar", las horas estimadas y ambas desviaciones son "no calculable", nunca 0. |
| RN-18 | Si las horas estimadas son 0, la desviación absoluta es computable y la porcentual es "no calculable". |
| RN-19 | La desviación absoluta es las horas reales menos las estimadas, con signo; la porcentual es esa diferencia sobre las estimadas. |
| RN-20 | Las horas estimadas se recalculan con el factor vigente; cambiar el factor nunca altera las horas reales. |
| RN-21 | Ninguna otra acción del producto elimina registros de esfuerzo como efecto colateral. |
| RN-22 | Una historia con esfuerzo registrado no puede eliminarse, ni siquiera después de borrarse todos sus registros. |
| RN-23 | Un sprint Cerrado admite esfuerzo nuevo y corregido solo durante su plazo de gracia: un período configurable de todo el sistema, por defecto 48 horas, contado desde el cierre real del sprint. |
| RN-24 | El plazo de gracia alcanza por igual a las historias Completadas que siguen en el sprint cerrado y a las no completadas que volvieron al backlog: en ambos casos el esfuerzo tardío se ancla a ese sprint. |
| RN-25 | Vencido el plazo, la historia Completada rechaza todo registro nuevo y la devuelta al backlog lo acepta sin sprint asociado. |

---

### Restricciones

- **RC-01**: Esta feature depende de la feature de autenticación y cuentas de usuario
  (`specs/001-user-auth`): solo usuarios registrados con sesión válida registran y consultan esfuerzo.
- **RC-02**: Esta feature depende de la feature de gestión de proyectos e integrantes
  (`specs/002-project-members`): el permiso de acceso es la membresía vigente, la fecha de inicio del
  proyecto es el piso de las fechas admitidas, el factor de horas por Story Point es el multiplicador
  de las horas estimadas y el estado Finalizado vuelve el esfuerzo de solo lectura.
- **RC-03**: `specs/002-project-members` (FR-027) establece que quitar a un integrante conserva su
  esfuerzo y sigue mostrando su nombre. Esta feature lo cumple en FR-028 y FR-032, y además le impide
  modificar o eliminar lo que dejó.
- **RC-04**: Esta feature depende de la feature de Product Backlog (`specs/003-product-backlog`): las
  historias, sus Story Points, la escala admitida y la marca "sin estimar" se definen allí. Esta
  feature las consume y no modifica ninguna historia.
- **RC-05**: `specs/003-product-backlog` (FR-043, RN-16) impide eliminar una historia que tiene
  esfuerzo registrado. Esa condición queda satisfecha para siempre desde el primer registro: aunque
  después se eliminen todos los registros, la historia no vuelve a ser eliminable.
- **RC-06**: Esta feature depende de la feature de gestión de Sprints
  (`specs/004-sprint-management`): el sprint al que se asocia cada registro, sus estados Planificado,
  Activo y Cerrado y el momento del cierre se definen allí. Esta feature no crea, no modifica y no
  cierra sprints. El plazo de gracia de FR-016 se cuenta desde el momento real de cierre que
  `specs/004-sprint-management` (FR-040) registra al cerrar el sprint, y no modifica ese sprint ni
  su instantánea: el esfuerzo vive fuera de ella. El reanclaje de FR-014 depende además de que esa
  instantánea conserve la lista de historias que el sprint tenía al cerrarse, que es lo que su
  FR-042 garantiza, incluidas las que vuelven al backlog.
- **RC-07**: `specs/004-sprint-management` (FR-048) permite eliminar un sprint Planificado que nunca
  fue iniciado. Si ese sprint tiene registros de esfuerzo asociados, eliminarlo dejaría esos registros
  sin ancla y alteraría las métricas por sprint. Esta feature exige que esa eliminación se rechace
  mientras el sprint tenga esfuerzo asociado, por el mismo motivo por el que la spec 004 ya impide
  eliminar un sprint Activo o Cerrado: no destruir registro de trabajo real. Esta restricción se
  incorporó a `specs/004-sprint-management` el 2026-10-02 y allí está recogida en su FR-048, su
  RN-20 y su RC-10, de modo que las dos specs dicen lo mismo. Quien elimina los registros para poder
  descartar el sprint es su autor, con las reglas de esta feature.
- **RC-08**: La fecha de un registro de esfuerzo es una fecha de calendario sin hora. El momento de
  creación y el de última actualización del registro incluyen hora. El día de hoy, a efectos de
  rechazar fechas futuras, se determina en la zona horaria única y configurable del sistema, la misma
  que `specs/002-project-members` definió para la fecha de finalización real de un proyecto.
- **RC-09**: El tope diario de 24 horas es por persona y atraviesa los límites del proyecto: su
  verificación considera registros de proyectos a los que quien registra tiene acceso y de los que no,
  incluidos los Finalizados. El mensaje de rechazo informa las horas disponibles y nunca nombra los
  otros proyectos, para no filtrar información entre proyectos.
- **RC-10**: El cálculo de métricas agregadas por sprint y por proyecto —esfuerzo total por sprint,
  desviación media del equipo, evolución entre sprints— corresponde a la feature de métricas. Esta
  feature produce los registros y la comparación por historia que esos cálculos consumen, pero no
  realiza las agregaciones. Durante el plazo de gracia de un sprint (FR-016), su esfuerzo total
  todavía puede cambiar, así que la feature de métricas DEBE tratar como provisorio el esfuerzo de
  un sprint cuyo plazo de gracia sigue vigente, y como definitivo el del resto.
- **RC-11**: El plazo de gracia es un parámetro de configuración único del sistema, del mismo tipo
  que la zona horaria de `specs/002-project-members`: no se define por proyecto ni por sprint, y su
  valor por defecto es de 48 horas. La vigencia se evalúa siempre comparando el momento actual
  contra el cierre real del sprint más el valor configurado en ese momento; no se congela un
  vencimiento por sprint. En consecuencia, ampliar el plazo reabre los sprints cerrados que habían
  quedado dentro del valor nuevo, y reducirlo cierra antes los que seguían abiertos. Es la regla con
  menos piezas móviles y hace que el cambio de configuración sea predecible.
- **RC-12**: Esta feature no compara la fecha de un registro con las fechas del sprint asociado: se
  pueden registrar horas con una fecha anterior al inicio del sprint o posterior a su fecha de fin
  prevista, mientras la fecha cumpla FR-006.

---

### Condiciones de Error

| Condición | Comportamiento esperado |
| --- | --- |
| Acción de la feature sin sesión válida | Rechazo con indicación de iniciar sesión; la acción no se ejecuta. |
| Horas iguales a 0, negativas o mayores que 24 | Rechazo indicando el rango admitido; nada se crea ni se modifica. |
| Horas que no son múltiplo de 0,25 | Rechazo indicando que las horas se cargan en múltiplos de un cuarto de hora. |
| Horas ausentes o no numéricas | Rechazo indicando que el campo es obligatorio y numérico. |
| Suma diaria del integrante superior a 24 horas | Rechazo indicando cuántas horas quedan disponibles para esa fecha, sin nombrar otros proyectos. |
| Actividad ausente o compuesta solo por espacios | Rechazo indicando que la actividad es obligatoria. |
| Actividad de más de 300 caracteres | Rechazo indicando el máximo permitido. |
| Fecha ausente | Rechazo indicando que el campo es obligatorio. |
| Fecha posterior al día de hoy | Rechazo indicando que no se registran horas en el futuro. |
| Fecha anterior a la fecha de inicio del proyecto | Rechazo indicando la fecha mínima admitida. |
| Registro sobre una historia de un proyecto Finalizado | Rechazo por proyecto de solo lectura. |
| Modificación o eliminación de un registro ajeno | Rechazo por falta de permiso; el registro no se toca, aunque quien lo intente sea el propietario del proyecto. |
| Registro de esfuerzo sobre una historia de un sprint Cerrado con el plazo de gracia vencido | Rechazo indicando que el sprint ya es definitivo; nada se crea. |
| Modificación o eliminación de un registro asociado a un sprint Cerrado con el plazo de gracia vencido | Rechazo indicando que ese esfuerzo ya es definitivo. |
| Modificación o eliminación de registros en un proyecto Finalizado | Rechazo por proyecto de solo lectura. |
| Intento de cambiar el integrante, la historia o el sprint de un registro | Rechazo indicando que esos datos son inmutables; para mover horas hay que eliminar y volver a crear. |
| Intento de registrar esfuerzo a nombre de otra persona | El dato se ignora y el registro se atribuye al titular de la sesión. |
| Acción de una persona quitada del proyecto sobre sus registros anteriores | Respuesta de registro inexistente; los registros se conservan. |
| Rango de fechas con fin anterior al inicio | Rechazo indicando que el fin no puede ser anterior al inicio. |
| Filtro por una historia, un sprint o un integrante de otro proyecto | Rechazo indicando que el filtro no corresponde al proyecto, sin revelar dato alguno del proyecto ajeno. |
| Solicitud sobre un proyecto, una historia o un registro de los que quien pide no es integrante | Respuesta idéntica a la de un identificador inexistente, sin revelar ningún dato. |
| Solicitud sobre un identificador de registro que no existe | Respuesta de registro inexistente. |
| Consulta de la desviación de una historia "sin estimar" | No es un error: se devuelve "no calculable" en horas estimadas y en ambas desviaciones. |
| Consulta de la desviación porcentual de una historia con 0 horas estimadas | No es un error: se devuelve "no calculable" solo en la desviación porcentual. |

---

### Entidades Clave

- **Registro de esfuerzo**: representa las horas que una persona trabajó sobre una historia en un día
  concreto, y qué hizo. Atributos relevantes: identificador propio estable y no secuencial, historia a
  la que pertenece, integrante que lo cargó, fecha, actividad realizada, horas trabajadas, sprint
  asociado o ausencia de sprint, momento de creación y momento de última actualización. Su integrante,
  su historia y su sprint asociado no cambian nunca.
- **Asociación al sprint**: representa el ancla histórica entre un registro y el sprint en el que
  estaba la historia al cargarlo. Puede estar vacía cuando la historia estaba solo en el backlog. Es
  lo que permite que el esfuerzo de un sprint cerrado siga siendo el mismo para siempre, aunque sus
  historias se replanifiquen después.
- **Plazo de gracia del sprint**: valor derivado, no almacenado, igual al momento real de cierre del
  sprint más el plazo configurado del sistema. Mientras no se alcance, el esfuerzo de ese sprint
  admite altas, correcciones y bajas; alcanzado, el esfuerzo del sprint queda definitivo. Solo tiene
  sentido para los sprints Cerrados.
- **Esfuerzo acumulado de una historia**: valor derivado, no almacenado, igual a la suma de las horas
  de todos los registros de la historia. Se acompaña del desglose por integrante.
- **Comparación de esfuerzo de una historia**: valor derivado, no almacenado, compuesto por Story
  Points, horas estimadas, horas reales, diferencia absoluta y diferencia porcentual. Cualquiera de
  los tres últimos puede ser "no calculable".
- **Historia de usuario** y **Story Points** *(entidades de `specs/003-product-backlog`)*: esta
  feature las consume para ubicar el esfuerzo y calcular las horas estimadas; no las modifica.
- **Sprint** y **Estado del sprint** *(entidades de `specs/004-sprint-management`)*: esta feature los
  consume para anclar cada registro y para decidir si admite cambios; no los modifica.
- **Proyecto**, **Integrante del proyecto** y **factor de horas por Story Point** *(entidades de
  `specs/002-project-members`)*: esta feature los consume para decidir quién accede, cuál es la fecha
  mínima admitida y cómo se traducen los Story Points a horas.
- **Usuario** *(entidad de `specs/001-user-auth`)*: esta feature lo consume para atribuir la autoría
  de cada registro; no lo modifica.

---

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **SC-001**: Un integrante registra sus horas sobre una historia en menos de 30 segundos y en no más
  de 4 pasos, y el 95 % lo logra en el primer intento sin ayuda externa.
- **SC-002**: Un integrante ve el total de horas de una historia y el desglose por persona en una sola
  pantalla, sin combinar información de otros lugares del sistema.
- **SC-003**: Un integrante responde la pregunta "cuántas horas puso cada persona en este sprint" en
  menos de 1 minuto usando los filtros disponibles.
- **SC-004**: El 100 % de los registros queda atribuido a quien lo cargó, verificado intentando crear
  un registro a nombre de otra persona en cada forma posible de indicarlo.
- **SC-005**: Cero integrantes con más de 24 horas registradas en una misma fecha, verificado
  intentando superar el tope con registros en un mismo proyecto, en proyectos distintos, mediante la
  modificación de un registro existente y enviando 10 registros simultáneos del mismo integrante
  para la misma fecha que entre todos superan las 24 horas: el total de la fecha nunca supera 24 y
  los pedidos sobrantes se rechazan.
- **SC-006**: El 100 % de las cantidades de horas fuera de rango o sin el múltiplo de 0,25 se rechaza,
  verificado con al menos un caso por condición: 0, negativo, mayor que 24 y no múltiplo.
- **SC-007**: El 100 % de las fechas futuras y de las anteriores al inicio del proyecto se rechaza, y
  el 100 % de las fechas de los días extremos válidos —hoy y el día de inicio del proyecto— se acepta.
- **SC-008**: Cero modificaciones o eliminaciones de registros ajenos, verificado intentándolo desde
  otro integrante y desde el propietario del proyecto.
- **SC-009**: Cero altas, modificaciones o eliminaciones de esfuerzo en sprints Cerrados con el plazo
  de gracia vencido, verificado sobre un registro creado antes del cierre y comprobando que sus horas
  no cambian una vez vencido el plazo.
- **SC-010**: El 100 % de los registros conserva su sprint asociado original, verificado quitando la
  historia del sprint, comprometiéndola en otro y cerrando el primero, y comparando la asociación
  antes y después.
- **SC-011**: Cero registros de esfuerzo perdidos o sin autor identificable después de quitar a un
  integrante del proyecto, verificado comparando el total de horas de sus historias antes y después de
  la baja.
- **SC-012**: El 100 % de las historias "sin estimar" muestra "no calculable" en horas estimadas y en
  ambas desviaciones, y ninguna muestra 0 en esos campos.
- **SC-013**: El 100 % de las historias estimadas en 0 Story Points muestra una desviación absoluta
  computable y una desviación porcentual "no calculable", sin ningún error de cálculo.
- **SC-014**: La comparación entre estimado y real coincide con el cálculo manual en el 100 % de una
  muestra que incluya al menos una historia por encima, una por debajo, una exacta, una sin registros,
  una sin estimar y una de 0 puntos.
- **SC-015**: Los totales de horas son exactos al cuarto de hora en el 100 % de los casos, verificado
  sumando 100 registros de 0,25 horas y comprobando que el total es exactamente 25.
- **SC-016**: El 100 % de las acciones de escritura sobre el esfuerzo de un proyecto Finalizado se
  rechaza, sin producir ningún cambio.
- **SC-017**: El 100 % de las acciones de esta feature exige sesión válida y membresía vigente en el
  proyecto, verificado con una prueba por acción.
- **SC-018**: Cero filtraciones de datos entre proyectos: en el 100 % de los intentos de acceso o de
  filtrado de quien no es integrante, la respuesta es indistinguible de la de un identificador
  inexistente, y ningún mensaje de tope diario nombra otro proyecto.
- **SC-019**: El 100 % de las combinaciones de filtros devuelve exactamente el subconjunto esperado,
  verificado con una prueba por filtro individual y al menos tres combinaciones de dos o más filtros.
- **SC-020**: El 100 % de los intentos dentro del plazo de gracia se acepta y el 100 % de los
  posteriores se rechaza, verificado con cuatro pruebas por acción (alta, modificación y
  eliminación): justo después del cierre, poco antes del vencimiento, exactamente en el vencimiento
  y después de él.
- **SC-021**: Cero horas de trabajo real perdidas por el cierre de un sprint, medido como la
  proporción de registros cargados dentro del plazo de gracia que el sistema acepta y ancla al
  sprint correcto: debe ser del 100 %, tanto para las historias Completadas como para las devueltas
  al backlog.
- **SC-022**: El 100 % de las reglas de negocio (RN-01 a RN-25) tiene al menos una prueba
  automatizada asociada que falla si la regla se rompe.

---

## Fuera de Alcance

- Temporizadores de trabajo: no hay cronómetro que se inicie y se detenga; las horas siempre se
  cargan a mano después del hecho.
- Hojas de horas semanales, vistas de calendario y carga masiva de varios días en una sola operación.
- Flujos de aprobación o validación de horas por parte de un responsable: lo que un integrante carga
  queda cargado.
- Costos, tarifas horarias, presupuestos y facturación.
- Cálculo de métricas agregadas por sprint y por proyecto, que corresponde a la feature de métricas.
  Esta feature solo produce los registros y la comparación por historia que esos cálculos consumen.
- Gráficos de cualquier tipo, incluidos los de esfuerzo acumulado.
- Capacidad planificada por integrante y comparación entre capacidad y esfuerzo registrado.
- Registro de esfuerzo sobre defectos, tareas técnicas o cualquier elemento que no sea una historia de
  usuario del backlog.
- Categorías o tipos de actividad predefinidos: la actividad es texto libre.
- Notificaciones o recordatorios de carga de horas.
- Exportación del esfuerzo a archivos externos.
- Historial de auditoría consultable de los cambios de un registro, más allá de su momento de última
  actualización.

---

## Supuestos

- **Cualquier integrante registra sobre cualquier historia del proyecto**: el enunciado no restringe
  el registro a las historias del sprint activo ni a las asignadas a la persona, y el producto no
  asigna historias a integrantes. Por eso se admite registrar sobre cualquier historia del proyecto,
  incluidas las que están solo en el backlog.
- **Leer es de todos, escribir es de cada uno**: la regla "cada integrante registra solo su propio
  esfuerzo" limita la escritura. La consulta del esfuerzo del proyecto con desglose y filtro por
  integrante solo tiene sentido si todos los integrantes pueden ver el esfuerzo de todos, así que la
  lectura es abierta dentro del proyecto (FR-058, RN-15).
- **Los campos editables son fecha, actividad y horas**: el enunciado dice "modificar registros
  propios" sin precisar qué. Se excluyen el integrante, la historia y el sprint, porque cambiarlos
  rompería la atribución personal (RN-01) o el ancla inmutable al sprint (RN-08).
- **El tope diario se evalúa excluyendo el registro que se modifica**: de lo contrario, subir de 6 a 7
  horas en un día con 24 horas cargadas se compararía contra 24 + 7 en vez de contra 18 + 7, y ninguna
  corrección sería posible.
- **Varios registros por persona, historia y fecha**: se admiten porque el enunciado pide registrar
  "la actividad realizada", y un mismo día puede tener varias actividades distintas sobre la misma
  historia. El tope diario es la única relación entre ellos.
- **La fecha es de calendario, sin hora**: el enunciado pide una fecha, no un momento. La hora solo
  aparece en los metadatos de creación y actualización del registro.
- **Hoy se calcula en la zona horaria del sistema**: se reutiliza la decisión de
  `specs/002-project-members`, para que un registro cargado a las 23:30 no se rechace por futuro ni se
  acepte por pasado según el dispositivo de quien lo carga.
- **El tope diario cuenta todos los proyectos, incluidos los Finalizados y aquellos de los que la
  persona fue quitada**: el enunciado dice "contando todos sus proyectos" sin excepciones, y el límite
  expresa una restricción física del día, no una regla de un proyecto.
- **El mensaje de tope diario no nombra otros proyectos**: informa las horas disponibles de la fecha,
  para no filtrar entre proyectos la existencia ni el nombre de los demás.
- **El signo de la desviación**: positiva significa que se trabajó más de lo estimado y negativa que
  se trabajó menos. El enunciado pide "la diferencia absoluta y porcentual" sin fijar el sentido; se
  elige el que hace que el exceso de esfuerzo, que es el caso que interesa detectar, se lea como un
  número positivo.
- **"No calculable" es un estado, no un número**: tanto para las historias "sin estimar" como para la
  desviación porcentual con 0 horas estimadas, el valor no se sustituye por 0 ni por un guion
  ambiguo, para que nunca se confunda con una desviación nula real.
- **Las horas estimadas se calculan al momento de la consulta**: no se congelan. El factor de horas
  por Story Point congelado al cerrar un sprint (`specs/004-sprint-management`, FR-043) existe para
  las métricas históricas por sprint, que son de otra feature; la comparación por historia de esta
  feature usa siempre el factor vigente.
- **Eliminar un registro es definitivo**: no hay papelera ni recuperación. A cambio, la eliminación
  está bloqueada en los dos casos donde perder el dato sería grave: registros ajenos y registros de
  un sprint cerrado con el plazo de gracia vencido.
- **El plazo de gracia habilita las tres acciones a la vez**: decisión confirmada el 2026-10-02
  (FR-016, FR-021, RN-23). Admitir el alta pero no la corrección dejaría al integrante atrapado con
  un registro equivocado e inmutable desde el momento en que lo crea, así que durante el plazo el
  esfuerzo del sprint se comporta como el de un sprint abierto y recién al vencer queda congelado.
- **El plazo por defecto es de 48 horas**: cubre el caso que motivó la regla, que es cargar el lunes
  el trabajo del viernes, sin dejar el sprint abierto tanto tiempo como para que sus métricas dejen
  de ser utilizables. Es configurable justamente porque el valor correcto depende del ritmo de cada
  equipo.
- **El plazo de gracia alcanza también a las historias devueltas al backlog**: decisión confirmada
  el 2026-10-02 (FR-014, FR-016, RN-24). Dentro del plazo, el esfuerzo tardío se reancla al sprint
  del que la historia volvió. La alternativa era dejarlo sin sprint, pero eso perdía las horas
  justamente en las historias que no se terminaron, que son las que más interesan al medir
  desviación: el esfuerzo del sprint habría quedado subestimado en el peor caso posible. El dato
  necesario ya existe, porque `specs/004-sprint-management` (FR-042) congela al cerrar la lista de
  historias que el sprint tenía.
- **El reanclaje solo opera mientras la historia siga en el backlog**: si se la vuelve a comprometer
  en otro sprint, manda el sprint actual, por la regla general de FR-014. Y si estuvo en dos sprints
  cerrados que siguen en plazo, manda el de cierre más reciente, que es el que corresponde al
  trabajo que se está cargando.
- **Sin límite de registros por historia ni por proyecto**: no se define un máximo, porque no se pidió;
  la única cota es el tope diario por persona.
- **Idioma de la interfaz**: los mensajes que ve la persona están en español.

---

## Preguntas Abiertas

Ninguna. La única decisión que quedó abierta al redactar la especificación —el registro de esfuerzo
sobre una historia de un sprint ya Cerrado— se resolvió el 2026-10-02 con el plazo de gracia y está
registrada en la sección Clarifications (FR-016, FR-021, RN-23, RN-24, RC-12).

Las demás decisiones que podrían haber quedado abiertas se resolvieron con supuestos explícitos,
documentados en la sección Supuestos.
