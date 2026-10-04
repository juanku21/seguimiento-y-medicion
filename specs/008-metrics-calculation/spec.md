# Especificación de Feature: Cálculo de Métricas

**Directorio de feature**: `specs/008-metrics-calculation`

**Rama**: `008-metrics-calculation`

**Creada**: 2026-10-03

**Estado**: Clarificada — sin preguntas abiertas (sesión de clarificación del 2026-10-03; enmendada
el 2026-10-03 a partir de `specs/009-project-dashboard`)

**Entrada**: Descripción del usuario: "Cálculo de métricas para Software Metrics & Estimation, un
sistema web multiusuario para estimar, planificar, seguir y medir proyectos de software con Scrum.
Depende de: gestión de proyectos (factor de horas por Story Point); Product Backlog; gestión de
Sprints (instantánea congelada al cierre); registro de esfuerzo (cada registro asociado a un
sprint); gestión de defectos (sprint de detección y de resolución)."

---

## Objetivo

Calcular de forma correcta, consistente y verificable las métricas de estimación, seguimiento y
calidad de un proyecto, por sprint y para el proyecto completo.

Esta feature es la única fuente de verdad de las fórmulas. El dashboard y los reportes consumen
estos resultados y no recalculan nada por su cuenta.

Las seis features anteriores producen datos: puntos estimados, sprints cerrados, horas trabajadas,
defectos. Ninguna los interpreta. Este es el lugar donde esos datos se convierten en las respuestas
que el producto promete: cuánto rinde el equipo, cuánto se desvía de lo que estima, qué calidad
entrega. Y es el único lugar: si el dashboard sumara por su cuenta y un reporte sumara distinto, el
sistema daría dos verdades sobre el mismo proyecto y perdería el único valor que tiene, que es ser
confiable.

De ahí la obsesión de esta spec con los casos degenerados. Una división por cero que devuelve 0 no
es un número aproximado: es una mentira que se parece a un dato. Un sprint sin historias con 0 % de
avance sugiere fracaso donde no hubo nada que hacer. Por eso cada fórmula define explícitamente qué
pasa cuando el denominador es cero, y la respuesta nunca es 0 ni un error: es "no calculable", que
es lo único honesto.

---

## Clarifications

### Session 2026-10-03

- Q: La instantánea de cierre no congela las horas reales ni los defectos, y ambos pueden cambiar después de cerrar un sprint. ¿Qué métricas de un sprint cerrado son entonces definitivas? (FR-019) → A: cada métrica lleva su propio estado. Las cuatro que salen de la instantánea son siempre definitivas; las horas reales y las dos desviaciones son provisorias mientras el plazo de gracia del sprint siga vigente y definitivas después; los dos conteos de defectos son siempre provisorios, porque nada los congela.
- Q: El esfuerzo cargado sobre historias que estaban solo en el backlog no pertenece a ningún sprint y quedaría invisible en los acumulados. ¿Qué se hace con él? (FR-029) → A: se informa por separado como "esfuerzo fuera de sprint", sin sumarlo al acumulado. Así no se pierde ninguna hora real y los acumulados siguen cuadrando con la suma de los sprints.
- Q: Dentro de un sprint nunca hay historias "sin estimar", porque la spec 004 no las admite. ¿A qué se refiere entonces la advertencia? (FR-036) → A: al backlog del proyecto; se informa cuántas historias del backlog están sin estimar, que es la única lectura en la que la advertencia tiene sentido.
- Q: Si los acumulados del proyecto incluyen al sprint activo, cuyas métricas son parciales, ¿el acumulado también se marca como parcial? (FR-023, FR-014) → A: sí; el acumulado se marca como parcial siempre que incluya al sprint activo, y como definitivo cuando solo haya sprints cerrados. Se reusa el término "parcial" que ya existe, sin inventar uno nuevo.
- Q: Si alguien modifica datos mientras se arma la respuesta de métricas, ¿las distintas cifras de esa respuesta pueden reflejar momentos distintos? (FR-003) → A: no; todas las métricas de una misma consulta se calculan sobre una vista coherente de los datos, correspondiente a un único instante. Una respuesta internamente contradictoria sería peor que una desactualizada.
- Q: Cuando el sistema detecta una inconsistencia de datos al calcular, ¿avisa en la respuesta o calcula en silencio? (FR-037) → A: calcula y avisa. Devuelve las métricas con el supuesto aplicado más una advertencia que identifica el dato inconsistente. Calcular en silencio entregaría cifras subestimadas con apariencia de correctas, y rechazar la consulta dejaría inutilizable todo el tablero por un único dato roto.

### Session 2026-10-03 — enmienda derivada de `specs/009-project-dashboard`

- Q: El dashboard necesita seis series por sprint para sus gráficos y esta feature exponía una sola, la de Story Points completados. ¿De dónde salen las otras cinco? (FR-022) → A: de acá. FR-022 pasa a definir una métrica de **series del proyecto** que entrega las seis juntas, en orden cronológico y con el estado de valor de cada punto. Esta feature sigue siendo dueña de todo lo que se grafica, incluido el orden; el dashboard presenta y no arma series.

---

## Entradas y Salidas Esperadas

### Métricas de un sprint

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador de un sprint Cerrado | Story Points planificados y completados, horas estimadas y reales, desviación absoluta y porcentual, porcentaje de historias completadas, defectos detectados y resueltos; cada métrica acompañada de su estado, **definitiva** o **provisoria** |
| Sesión de un integrante e identificador del sprint Activo | Las mismas métricas, calculadas con los datos actuales y marcadas como **parciales** |
| Sesión de un integrante e identificador de un sprint Planificado | Únicamente Story Points planificados y horas estimadas; el resto se informa como no aplicable |
| Denominador en cero en alguna fórmula | Esa métrica se devuelve como **"no calculable"**; las demás se calculan igual |
| Sprint inexistente o de otro proyecto | Respuesta de sprint inexistente, sin revelar ningún dato |
| Quien pide no es integrante | Respuesta de proyecto inexistente |

### Métricas del proyecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador del proyecto | Velocidad del equipo, serie de velocidad por sprint, acumulados de Story Points, horas, desviación y porcentaje de historias completadas —marcados como **parciales** si incluyen al sprint activo—, esfuerzo fuera de sprint, cantidad de historias del backlog sin estimar, y métricas de defectos con desglose por severidad |
| Proyecto sin sprints cerrados | Velocidad y serie como "no calculable" y vacía respectivamente; los acumulados reflejan solo el sprint activo si existe |
| Proyecto sin sprints | Todas las métricas en su forma vacía o "no calculable", sin que eso sea un error |
| Proyecto inexistente o del que quien pide no es integrante | Respuesta de proyecto inexistente |

### Métricas de defectos

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador del proyecto | Defectos detectados, resueltos y abiertos del proyecto, cada uno con su desglose por severidad (Crítica, Alta, Media y Baja) |
| Sesión de un integrante e identificador de un sprint | Defectos detectados y resueltos de ese sprint |
| Proyecto sin defectos | Todos los conteos en 0, con el desglose en 0 por cada severidad; no es "no calculable" porque contar no divide |

---

## Escenarios de Usuario y Pruebas *(obligatorio)*

> **Nota sobre los datos de los escenarios**: salvo que se indique lo contrario, todos usan el mismo
> proyecto de ejemplo, con **factor de horas por Story Point = 6**.

### Historia de Usuario 1 (US1) — Consultar las métricas de esfuerzo y avance de un sprint cerrado (Prioridad: P1)

Un integrante abre un sprint ya cerrado y ve cuánto se había planificado, cuánto se completó, cuántas
horas costó y cuánto se desvió de lo estimado.

**Por qué esta prioridad**: es el núcleo de la feature y la base de todo lo demás. La velocidad y los
acumulados del proyecto se construyen sobre estos números; si están mal, todo lo que sigue está mal.

**Prueba independiente**: se puede probar completa cerrando un sprint con historias parcialmente
completadas y esfuerzo registrado, consultando sus métricas y comparándolas con el cálculo manual.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un sprint Cerrado cuya instantánea registra 4 historias planificadas que
   suman 21 Story Points, de las cuales 2 se completaron por 13 Story Points, con 142,5 horas de
   esfuerzo registradas y factor 6 congelado al cierre, **cuando** un integrante consulta sus
   métricas, **entonces** obtiene exactamente: Story Points planificados **21**, completados **13**,
   horas estimadas **126** (21 × 6), horas reales **142,5**, desviación absoluta **+16,5 horas**,
   desviación porcentual **+13,10 %** (16,5 / 126 × 100 = 13,0952… redondeado a 2 decimales) y
   porcentaje de historias completadas **50,00 %** (2 / 4 × 100).
2. *(Caso alternativo)* **Dado** el mismo sprint con 110 horas reales en lugar de 142,5, **cuando**
   un integrante consulta sus métricas, **entonces** la desviación absoluta es **−16 horas** y la
   porcentual **−12,70 %** (−16 / 126 × 100 = −12,6984…), de modo que el signo negativo indica
   sobrestimación.
3. *(Caso alternativo)* **Dado** el mismo sprint cerrado y que después el propietario cambia el
   factor del proyecto de 6 a 8, **cuando** un integrante vuelve a consultar sus métricas,
   **entonces** las horas estimadas siguen siendo **126** y no 168, porque el sprint usa el factor
   congelado en su instantánea.
4. *(Caso alternativo)* **Dado** el mismo sprint cerrado y que después alguien reestima una de sus
   historias de 5 a 13 Story Points, **cuando** un integrante consulta sus métricas, **entonces**
   los Story Points planificados siguen siendo **21**, porque provienen de la instantánea.
5. *(Caso alternativo)* **Dado** un sprint Cerrado hace 10 horas, con un plazo de gracia de 48
   horas, **cuando** un integrante consulta sus métricas, **entonces** los Story Points
   planificados, los completados, las horas estimadas y el porcentaje de historias completadas
   vienen marcados como **definitivos**, mientras que las horas reales, las dos desviaciones y los
   dos conteos de defectos vienen marcados como **provisorios**.
6. *(Caso alternativo)* **Dado** el mismo sprint una vez vencido su plazo de gracia, **cuando** un
   integrante vuelve a consultar, **entonces** las horas reales y las dos desviaciones pasan a
   **definitivas**, y los dos conteos de defectos siguen siendo **provisorios**, porque nada los
   congela.
7. *(Caso alternativo)* **Dado** un sprint Cerrado con 142,5 horas reales dentro de su plazo de
   gracia, **cuando** alguien carga 3 horas más sobre una historia de ese sprint y un integrante
   vuelve a consultar, **entonces** las horas reales pasan a **145,5** y la desviación se recalcula,
   mientras que los Story Points y las horas estimadas no cambian.
8. *(Caso límite)* **Dado** un sprint Cerrado cuyas 3 historias planificadas estaban todas estimadas
   en 0 Story Points, con 12 horas reales registradas, **cuando** un integrante consulta sus
   métricas, **entonces** obtiene Story Points planificados **0**, horas estimadas **0** (0 × 6),
   horas reales **12**, desviación absoluta **+12 horas** y desviación porcentual **"no
   calculable"**, porque no hay base contra la cual calcular el porcentaje.
9. *(Caso límite)* **Dado** un sprint Cerrado sin ninguna historia planificada, **cuando** un
   integrante consulta sus métricas, **entonces** obtiene Story Points planificados **0**,
   completados **0**, horas estimadas **0**, y tanto la desviación porcentual como el porcentaje de
   historias completadas como **"no calculable"**; la desviación absoluta es igual a las horas
   reales registradas.
10. *(Caso límite)* **Dado** un sprint Cerrado con 4 historias planificadas y las 4 completadas,
    **cuando** un integrante consulta sus métricas, **entonces** el porcentaje de historias
    completadas es **100,00 %**.
11. *(Caso límite)* **Dado** un sprint Cerrado con 3 historias planificadas y 1 completada,
    **cuando** un integrante consulta sus métricas, **entonces** el porcentaje de historias
    completadas es **33,33 %** (1 / 3 × 100 = 33,333…), redondeado solo en el resultado final.
12. *(Caso límite)* **Dado** una historia de 5 Story Points planificada en el sprint 1, no completada
    y replanificada en el sprint 2, donde sí se completa, **cuando** un integrante consulta ambos
    sprints, **entonces** la historia cuenta como planificada en los dos y como completada solo en el
    sprint 2.
13. *(Caso de error)* **Dado** un identificador de sprint que no existe, **cuando** un integrante
    consulta sus métricas, **entonces** obtiene una respuesta de sprint inexistente.
14. *(Caso de error)* **Dado** un sprint de otro proyecto, **cuando** un integrante intenta consultar
    sus métricas, **entonces** la respuesta es idéntica a la de un sprint inexistente, sin revelar
    ningún dato del otro proyecto.
15. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
    consultar las métricas de uno de sus sprints, **entonces** la respuesta es de proyecto
    inexistente.

---

### Historia de Usuario 2 (US2) — Consultar la velocidad del equipo y su evolución (Prioridad: P1)

Un integrante quiere saber cuántos Story Points completa el equipo por sprint en promedio y cómo
viene evolucionando esa cifra.

**Por qué esta prioridad**: la velocidad es la métrica que da nombre al producto y la que el equipo
usa para planificar el sprint siguiente. Es independientemente testeable: alcanza con cerrar sprints
con distintos resultados y verificar el promedio.

**Prueba independiente**: se puede probar completa cerrando 1, 2, 3 y 4 sprints con Story Points
completados distintos y verificando el valor de la velocidad en cada paso.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto con 3 sprints cerrados que completaron 13, 21 y 8 Story
   Points, **cuando** un integrante consulta la velocidad del equipo, **entonces** obtiene
   exactamente **14,00** ((13 + 21 + 8) / 3 = 42 / 3).
2. *(Caso normal)* **Dado** un proyecto con 4 sprints cerrados que completaron 13, 21, 8 y 20 Story
   Points en ese orden, **cuando** un integrante consulta la velocidad, **entonces** obtiene
   **16,33** ((21 + 8 + 20) / 3 = 49 / 3 = 16,333…), porque solo se promedian los 3 últimos cerrados
   y el primero queda fuera.
3. *(Caso alternativo)* **Dado** un proyecto con exactamente 1 sprint cerrado que completó 13 Story
   Points, **cuando** un integrante consulta la velocidad, **entonces** obtiene **13,00**, el
   promedio de todos los cerrados.
4. *(Caso alternativo)* **Dado** un proyecto con exactamente 2 sprints cerrados que completaron 13 y
   21 Story Points, **cuando** un integrante consulta la velocidad, **entonces** obtiene **17,00**
   ((13 + 21) / 2).
5. *(Caso alternativo)* **Dado** el proyecto con 4 sprints cerrados, **cuando** un integrante
   consulta la serie de velocidad por sprint, **entonces** obtiene los 4 valores **13, 21, 8 y 20**
   en orden cronológico, sin promediar y sin recortar a los últimos 3.
6. *(Caso normal)* **Dado** el mismo proyecto, **cuando** un integrante consulta las series del
   proyecto, **entonces** obtiene en una sola respuesta las **seis** series —Story Points
   planificados y completados, horas estimadas y reales, y defectos detectados y resueltos—, cada
   una con un punto por sprint, en orden cronológico ascendente y con cada punto identificando a
   qué sprint corresponde.
7. *(Caso alternativo)* **Dado** un proyecto con 3 sprints cerrados y uno Activo, **cuando** un
   integrante consulta las series, **entonces** las seis incluyen los 4 sprints, y los puntos del
   sprint Activo vienen marcados como **parciales**.
8. *(Caso alternativo)* **Dado** un proyecto con un sprint Planificado además de los anteriores,
   **cuando** un integrante consulta las series, **entonces** el sprint Planificado no aparece en
   ninguna de las seis.
9. *(Caso límite)* **Dado** un sprint que no completó ningún Story Point y no tuvo horas
   registradas, **cuando** un integrante consulta las series, **entonces** ese sprint aparece con
   valor **0** en las series correspondientes, distinguible de la ausencia de dato.
10. *(Caso límite)* **Dado** un proyecto sin sprints cerrados ni activo, **cuando** un integrante
    consulta las series, **entonces** las seis vienen vacías, sin que eso sea un error.
11. *(Caso límite)* **Dado** un proyecto con 3 sprints cerrados que no completaron ninguna historia,
    **cuando** un integrante consulta la velocidad, **entonces** obtiene **0,00**, que es un promedio
    válido y distinto de "no calculable".
12. *(Caso límite)* **Dado** un proyecto sin ningún sprint cerrado pero con un sprint Activo,
    **cuando** un integrante consulta la velocidad, **entonces** obtiene **"no calculable"** y la
    serie vacía, porque la velocidad solo mira sprints cerrados.
13. *(Caso límite)* **Dado** un proyecto sin ningún sprint, **cuando** un integrante consulta la
    velocidad, **entonces** obtiene **"no calculable"**, sin que eso sea un error.
14. *(Caso límite)* **Dado** un proyecto con 3 sprints cerrados y que después se cierra un cuarto,
    **cuando** un integrante consulta la velocidad, **entonces** el valor cambia porque la ventana de
    3 se desplaza; los sprints que salen de la ventana conservan sus propias métricas intactas.
15. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
    consultar su velocidad, **entonces** la respuesta es de proyecto inexistente.

---

### Historia de Usuario 3 (US3) — Consultar las métricas acumuladas del proyecto (Prioridad: P2)

Un integrante quiere la foto completa del proyecto: cuánto se planificó y se completó en total,
cuántas horas se estimaron y se trabajaron, y cuánto se desvía el conjunto.

**Por qué esta prioridad**: es la lectura que responde "cómo venimos" a escala de proyecto. Necesita
que las métricas por sprint ya estén bien, así que va después de US1.

**Prueba independiente**: se puede probar completa con dos sprints cerrados y uno activo, sumando a
mano y comparando contra el acumulado devuelto.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto con dos sprints cerrados —el primero con 21 Story Points
   planificados, 13 completados, 126 horas estimadas y 142,5 reales; el segundo con 24
   planificados, 21 completados, 144 estimadas y 130 reales— y un sprint Activo con 18
   planificados, 5 completados, 108 estimadas y 40 reales, **cuando** un integrante consulta los
   acumulados, **entonces** obtiene Story Points planificados **63**, completados **39**, horas
   estimadas **378**, horas reales **312,5**, desviación absoluta **−65,5 horas** y desviación
   porcentual **−17,33 %** (−65,5 / 378 × 100 = −17,3280…).
2. *(Caso alternativo)* **Dado** el mismo proyecto, **cuando** un integrante consulta el porcentaje
   acumulado de historias completadas con 11 historias planificadas y 7 completadas en total,
   **entonces** obtiene **63,64 %** (7 / 11 × 100 = 63,6363…).
3. *(Caso alternativo)* **Dado** el mismo proyecto con un sprint Planificado además de los
   anteriores, **cuando** un integrante consulta los acumulados, **entonces** el sprint Planificado
   **no** se incluye: los acumulados abarcan los sprints cerrados y el activo, y nada más.
4. *(Caso alternativo)* **Dado** el mismo proyecto, que tiene un sprint Activo, **cuando** un
   integrante consulta los acumulados, **entonces** el conjunto viene marcado como **parcial**,
   porque incluye un sprint a mitad de camino, mientras que la velocidad y su serie vienen sin esa
   marca, porque solo miran sprints cerrados.
5. *(Caso alternativo)* **Dado** un proyecto cuyo sprint activo se cierra y no se inicia ninguno
   nuevo, **cuando** un integrante consulta los acumulados, **entonces** el conjunto pasa a estar
   marcado como **definitivo**, porque ya no contiene ningún sprint en curso.
6. *(Caso alternativo)* **Dado** una historia de 5 Story Points planificada en dos sprints
   consecutivos y completada en el segundo, **cuando** un integrante consulta los acumulados,
   **entonces** aporta **10** Story Points planificados y **5** completados, porque se cuenta una vez
   por cada sprint en que estuvo planificada.
7. *(Caso límite)* **Dado** un proyecto cuyos únicos sprints están Planificados, **cuando** un
   integrante consulta los acumulados, **entonces** todos los totales son 0 y las métricas con
   denominador cero son **"no calculable"**.
8. *(Caso límite)* **Dado** un proyecto sin ningún sprint, **cuando** un integrante consulta los
   acumulados, **entonces** obtiene totales en 0 y desviaciones **"no calculable"**, sin que eso sea
   un error.
9. *(Caso límite)* **Dado** un proyecto cuyos sprints cerrados suman 0 horas estimadas pero tienen
   horas reales registradas, **cuando** un integrante consulta los acumulados, **entonces** la
   desviación absoluta es igual a las horas reales y la porcentual es **"no calculable"**.
10. *(Caso alternativo)* **Dado** el mismo proyecto con 18 horas registradas sobre historias que
    estaban solo en el backlog, **cuando** un integrante consulta los acumulados, **entonces** ve
    **esfuerzo fuera de sprint: 18 horas** como cifra separada, y las horas reales acumuladas siguen
    siendo **312,5**, de modo que el acumulado sigue cuadrando con la suma de los sprints.
11. *(Caso límite)* **Dado** un proyecto donde todo el esfuerzo se cargó dentro de sprints, **cuando**
    un integrante consulta los acumulados, **entonces** el esfuerzo fuera de sprint se informa como
    **0 horas**, no se omite del resultado.
12. *(Caso alternativo)* **Dado** un proyecto cuyo backlog tiene 6 historias sin estimar, **cuando**
    un integrante consulta las métricas del proyecto, **entonces** ve el contador en **6** junto con
    la advertencia de que las cifras de planificación pueden quedar cortas.
13. *(Caso límite)* **Dado** un proyecto cuyo backlog no tiene ninguna historia sin estimar,
    **cuando** un integrante consulta las métricas, **entonces** el contador es **0** y no se
    muestra ninguna advertencia.
14. *(Caso de error)* **Dado** un identificador de proyecto que no existe, **cuando** un integrante
    consulta sus acumulados, **entonces** obtiene una respuesta de proyecto inexistente.

---

### Historia de Usuario 4 (US4) — Consultar las métricas de un sprint activo o planificado (Prioridad: P2)

Un integrante quiere ver cómo viene el sprint en curso sin esperar a que cierre, y qué se espera del
que todavía no arrancó.

**Por qué esta prioridad**: es lo que permite reaccionar durante el sprint en vez de enterarse al
final. Va después de los cerrados porque comparte las fórmulas y agrega solo la marca de parcialidad.

**Prueba independiente**: se puede probar completa con un sprint Activo y uno Planificado,
verificando la marca "parcial" y que el Planificado solo informa dos métricas.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un sprint Activo con 18 Story Points planificados, 5 completados y 40
   horas reales registradas, con factor vigente 6, **cuando** un integrante consulta sus métricas,
   **entonces** obtiene horas estimadas **108** (18 × 6), horas reales **40**, desviación absoluta
   **−68 horas**, desviación porcentual **−62,96 %** (−68 / 108 × 100 = −62,9629…), y todas las
   métricas marcadas como **parciales**.
2. *(Caso alternativo)* **Dado** el mismo sprint Activo al que se le suma una historia de 8 Story
   Points, **cuando** un integrante vuelve a consultar, **entonces** los Story Points planificados
   pasan a **26** y las horas estimadas a **156**, porque un sprint activo se calcula con los datos
   actuales.
3. *(Caso alternativo)* **Dado** un proyecto cuyo propietario cambia el factor de 6 a 8 mientras hay
   un sprint Activo con 18 Story Points planificados, **cuando** un integrante consulta sus
   métricas, **entonces** las horas estimadas pasan a **144** (18 × 8), porque el sprint activo usa
   el factor vigente y no uno congelado.
4. *(Caso alternativo)* **Dado** un sprint Planificado con 15 Story Points planificados y factor
   vigente 6, **cuando** un integrante consulta sus métricas, **entonces** obtiene Story Points
   planificados **15** y horas estimadas **90**, y el resto de las métricas se informa como no
   aplicable para un sprint que todavía no arrancó.
5. *(Caso límite)* **Dado** un sprint Activo sin ninguna historia, **cuando** un integrante consulta
   sus métricas, **entonces** obtiene 0 en los totales, **"no calculable"** en las métricas con
   denominador cero, y la marca de parcial.
6. *(Caso límite)* **Dado** un sprint Activo sin ninguna hora registrada todavía, **cuando** un
   integrante consulta sus métricas, **entonces** las horas reales son **0** y la desviación
   absoluta es igual a las horas estimadas en negativo, lo que es esperable al inicio del sprint y
   no un error.
7. *(Caso de error)* **Dado** un proyecto sin sprint Activo, **cuando** un integrante pide las
   métricas del sprint activo, **entonces** obtiene una indicación de que no hay sprint activo, no
   un error ni un conjunto de ceros.

---

### Historia de Usuario 5 (US5) — Consultar las métricas de defectos (Prioridad: P3)

Un integrante quiere saber cuántos defectos aparecieron y cuántos se arreglaron, por sprint y en todo
el proyecto, y cómo se reparten por gravedad.

**Por qué esta prioridad**: es el eje de calidad, independiente del de esfuerzo. El equipo puede
operar con las métricas de esfuerzo solas, pero sin esta no hay forma de ver si entrega bien.

**Prueba independiente**: se puede probar completa registrando defectos de distintas severidades,
resolviendo algunos, reabriendo otro y verificando los conteos.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un sprint con 7 defectos cuyo sprint de detección es ese sprint y 4
   defectos Resueltos cuyo sprint de resolución es ese mismo sprint, **cuando** un integrante
   consulta sus métricas de defectos, **entonces** obtiene detectados **7** y resueltos **4**.
2. *(Caso normal)* **Dado** un proyecto con 12 defectos en total —3 Críticos, 4 Altos, 3 Medios y 2
   Bajos—, de los cuales 5 están Resueltos, **cuando** un integrante consulta las métricas de
   defectos del proyecto, **entonces** obtiene detectados **12**, resueltos **5**, abiertos **7**, y
   el desglose por severidad de cada uno de los tres conteos.
3. *(Caso alternativo)* **Dado** un defecto detectado en el sprint 1 y resuelto en el sprint 2,
   **cuando** un integrante consulta ambos sprints, **entonces** cuenta como detectado en el 1 y
   como resuelto en el 2, nunca en los dos.
4. *(Caso alternativo)* **Dado** un defecto detectado y resuelto en el mismo sprint, **cuando** un
   integrante consulta ese sprint, **entonces** cuenta una vez como detectado y una vez como
   resuelto.
5. *(Caso alternativo)* **Dado** un defecto que estaba Resuelto en el sprint 2 y después se reabre,
   **cuando** un integrante vuelve a consultar las métricas, **entonces** deja de contar como
   resuelto en el sprint 2, porque la reapertura le borró el sprint de resolución, y sigue contando
   como detectado en su sprint de detección.
6. *(Caso límite)* **Dado** un proyecto con 3 defectos Descartados además de los 12 anteriores,
   **cuando** un integrante consulta las métricas de defectos, **entonces** los descartados **no**
   se cuentan en detectados, resueltos ni abiertos, y el total sigue siendo 12.
7. *(Caso límite)* **Dado** un proyecto sin ningún defecto, **cuando** un integrante consulta sus
   métricas de defectos, **entonces** obtiene **0** en los tres conteos y **0** en cada severidad; el
   resultado es 0 y no "no calculable", porque contar no implica dividir.
8. *(Caso límite)* **Dado** un defecto cuyo sprint de detección es un sprint ya Cerrado, registrado
   después del cierre, **cuando** un integrante consulta ese sprint, **entonces** el defecto cuenta
   como detectado en él.
9. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
   consultar sus métricas de defectos, **entonces** la respuesta es de proyecto inexistente.

---

### Casos Límite

- **Proyecto sin sprints**: todas las métricas por sprint no aplican, la velocidad y la serie son "no
  calculable" y vacía, y los acumulados son 0 con las divisiones en "no calculable". No es un error.
- **Sprint sin historias planificadas**: Story Points en 0, horas estimadas en 0, porcentaje de
  historias completadas y desviación porcentual en "no calculable".
- **Sprint con todas las historias en 0 Story Points**: horas estimadas 0; la desviación absoluta es
  computable e igual a las horas reales, y la porcentual es "no calculable".
- **Horas reales registradas con horas estimadas en 0**: la desviación absoluta es positiva e igual a
  las horas reales; la porcentual es "no calculable". Nunca se informa una desviación infinita.
- **Exactamente 1 sprint cerrado**: la velocidad es el valor de ese sprint.
- **Exactamente 2 sprints cerrados**: la velocidad es el promedio de los dos.
- **Exactamente 3 sprints cerrados**: la velocidad es el promedio de los tres.
- **Exactamente 4 sprints cerrados**: la velocidad promedia solo los 3 últimos; el primero queda
  fuera de la ventana pero conserva intactas sus propias métricas y su lugar en la serie.
- **Velocidad de sprints que no completaron nada**: es 0,00, que es un promedio real y distinto de
  "no calculable".
- **Historia planificada en dos sprints consecutivos**: cuenta como planificada en ambos y como
  completada solo en el último; en los acumulados del proyecto sus Story Points planificados se
  suman dos veces y los completados una sola.
- **Cambio del factor de horas por Story Point después de cerrar sprints**: los sprints cerrados
  conservan sus horas estimadas, calculadas con el factor congelado; el sprint activo y los
  planificados pasan a usar el factor nuevo. Los acumulados del proyecto mezclan ambos, por
  construcción.
- **Defecto reabierto**: deja de contar como resuelto en el sprint en que se había resuelto, porque
  la reapertura le borra el sprint de resolución; sigue contando como detectado.
- **Defecto descartado**: no cuenta en ningún conteo de defectos.
- **Defecto registrado sobre un sprint ya cerrado**: cuenta como detectado en ese sprint, aunque el
  sprint esté cerrado desde hace tiempo.
- **Esfuerzo cargado sobre un sprint cerrado dentro de su plazo de gracia**: aumenta las horas reales
  de ese sprint después del cierre, por lo que esas métricas vienen marcadas como provisorias hasta
  que el plazo vence (FR-019).
- **Resultado de una división que da exactamente un número con más de 2 decimales**: se redondea solo
  al final; los pasos intermedios conservan toda la precisión.
- **Proyecto Finalizado**: sus métricas se consultan con normalidad, porque consultar no escribe.
- **Escritura concurrente durante una consulta de métricas**: si alguien carga horas o completa una
  historia mientras la respuesta se está armando, esa respuesta refleja un único instante, anterior
  o posterior al cambio, pero nunca una mezcla de los dos. Puede quedar desactualizada; no puede
  quedar contradictoria.
- **Dos consultas seguidas con una escritura en el medio**: devuelven valores distintos, y eso es
  correcto. La garantía es de coherencia interna de cada respuesta, no de inmutabilidad entre
  consultas.

---

## Requisitos *(obligatorio)*

### Requisitos Funcionales

**Alcance y rol de la feature**

- **FR-001**: El sistema DEBE calcular las métricas definidas en esta especificación y DEBE ser la
  única fuente de las fórmulas; ninguna otra feature recalcula una métrica por su cuenta.
- **FR-002**: Esta feature DEBE limitarse a leer y calcular: NO DEBE crear, modificar ni eliminar
  historias, sprints, registros de esfuerzo, defectos ni ningún otro dato del producto.
- **FR-003**: Para un mismo conjunto de datos de entrada, el sistema DEBE devolver siempre el mismo
  resultado, sin importar desde dónde se consulte la métrica. Además, todas las métricas de una
  misma consulta DEBEN calcularse sobre una vista coherente de los datos, correspondiente a un único
  instante: ninguna respuesta puede mezclar cifras tomadas en momentos distintos. En particular,
  NUNCA DEBE devolverse una combinación imposible, como Story Points completados mayores que los
  planificados o una desviación que no se corresponda con las horas informadas a su lado. Que la
  respuesta quede levemente desactualizada respecto del dato más reciente es aceptable; que se
  contradiga a sí misma, no.

**Métricas por sprint**

- **FR-004**: **Story Points planificados** de un sprint = suma de los Story Points de las historias
  planificadas en ese sprint.
- **FR-005**: **Story Points completados** de un sprint = suma de los Story Points de las historias
  completadas en ese sprint.
- **FR-006**: **Horas estimadas** de un sprint = Story Points planificados × factor de horas por
  Story Point aplicable al sprint según FR-013 a FR-015.
- **FR-007**: **Horas reales** de un sprint = suma de las horas de los registros de esfuerzo
  asociados a ese sprint.
- **FR-008**: **Desviación de esfuerzo absoluta** = horas reales − horas estimadas. Un valor positivo
  indica subestimación: se trabajó más de lo previsto.
- **FR-009**: **Desviación de esfuerzo porcentual** = (horas reales − horas estimadas) / horas
  estimadas × 100, con el mismo signo que la absoluta.
- **FR-010**: **Porcentaje de historias completadas** = historias completadas / historias
  planificadas × 100, contado por cantidad de historias y no por Story Points.
- **FR-011**: **Defectos detectados** de un sprint = cantidad de defectos cuyo sprint de detección es
  ese sprint.
- **FR-012**: **Defectos resueltos** de un sprint = cantidad de defectos en estado Resuelto cuyo
  sprint de resolución es ese sprint.

**Origen de los datos según el estado del sprint**

- **FR-013**: Para un sprint **Cerrado**, el sistema DEBE tomar las historias planificadas, las
  completadas, sus Story Points y el factor de horas exclusivamente de la instantánea congelada al
  cierre (`specs/004-sprint-management`, FR-043). Ninguna reestimación ni cambio de factor posterior
  DEBE alterar esos valores.
- **FR-014**: Para el sprint **Activo**, el sistema DEBE calcular con los datos actuales —historias
  vigentes, Story Points vigentes y factor vigente del proyecto— y DEBE marcar todas sus métricas
  como **parciales**, para que nadie las confunda con un resultado definitivo.
- **FR-015**: Para un sprint **Planificado**, el sistema DEBE informar únicamente Story Points
  planificados y horas estimadas, calculadas con el factor vigente, y DEBE presentar las demás
  métricas como no aplicables, no como 0.
- **FR-016**: Cuando el proyecto no tiene sprint Activo, el sistema DEBE devolver una indicación
  explícita de que no hay sprint activo, en lugar de un error o un conjunto de ceros.
- **FR-017**: El sistema NO DEBE incluir los sprints Planificados en ninguna métrica acumulada del
  proyecto ni en la velocidad.
- **FR-018**: El sistema DEBE excluir de todo conteo de defectos los que están en estado Descartado,
  tal como exige `specs/007-defect-tracking` (su RC-09).
- **FR-019**: Cada métrica de un sprint Cerrado DEBE devolverse acompañada de su estado,
  **definitiva** o **provisoria**, porque la instantánea de cierre no congela todo:
  - **Siempre definitivas**: Story Points planificados, Story Points completados, horas estimadas y
    porcentaje de historias completadas. Salen de la instantánea (FR-013) y nada posterior las
    altera.
  - **Provisorias mientras el plazo de gracia del sprint siga vigente, definitivas después**: horas
    reales, desviación absoluta y desviación porcentual. El esfuerzo de un sprint cerrado puede
    crecer dentro de ese plazo (`specs/006-effort-tracking`, FR-016), y su RC-10 obliga a tratarlo
    como provisorio hasta que venza.
  - **Siempre provisorias**: defectos detectados y defectos resueltos. Nada los congela: un defecto
    nuevo puede nombrar un sprint ya Cerrado como sprint de detección en cualquier momento
    (`specs/007-defect-tracking`, FR-006) y una reapertura puede quitar un defecto del conteo de
    resueltos (su FR-029).

  El estado **provisoria** y la marca **parcial** de FR-014 son cosas distintas y el sistema NO DEBE
  confundirlas: "parcial" califica al conjunto completo de métricas de un sprint Activo, que todavía
  está corriendo; "provisoria" califica a una métrica concreta de un sprint ya Cerrado, cuyo valor
  todavía puede moverse.

**Métricas del proyecto**

- **FR-020**: **Velocidad del equipo** = promedio de los Story Points completados de los últimos 3
  sprints Cerrados. Si hay menos de 3 sprints cerrados, es el promedio de todos los cerrados.
- **FR-021**: El sistema DEBE determinar cuáles son "los últimos 3" por momento real de cierre,
  tomando los 3 más recientes.
- **FR-022**: **Series del proyecto**: el sistema DEBE entregar, en una única consulta, seis series
  por sprint en orden cronológico ascendente, listas para graficarse sin ningún trabajo adicional de
  quien las consume:
  1. Story Points planificados por sprint.
  2. Story Points completados por sprint, que es la serie de **velocidad por sprint**.
  3. Horas estimadas por sprint.
  4. Horas reales por sprint.
  5. Defectos detectados por sprint.
  6. Defectos resueltos por sprint.
  Cada punto DEBE identificar a qué sprint corresponde y DEBE respetar el estado de valor del sprint
  al que pertenece (FR-014, FR-019): un punto de un sprint Activo viene marcado como parcial y uno
  de un sprint Cerrado viene con su estado definitivo o provisorio. Un valor 0 DEBE distinguirse de
  la ausencia de dato. Las seis series abarcan los sprints Cerrados y el Activo si existe; los
  sprints Planificados NO DEBEN incluirse, por aplicación de FR-017. La serie de velocidad DEBE
  poder consultarse también por separado, restringida a los sprints Cerrados, porque es la que
  alimenta el cálculo de FR-020.
- **FR-023**: El sistema DEBE calcular los acumulados del proyecto —Story Points planificados y
  completados, horas estimadas y reales, desviación absoluta y porcentual, y porcentaje de historias
  completadas— sumando los valores de todos los sprints Cerrados más el sprint Activo si existe. El
  conjunto acumulado DEBE venir marcado como **parcial** siempre que incluya al sprint Activo, y
  como **definitivo** cuando se construya solo con sprints Cerrados, porque un total que contiene un
  sprint a mitad de camino todavía puede moverse. La marca califica al conjunto acumulado entero, no
  a cada cifra por separado. En cambio, la velocidad del equipo y la serie de velocidad NO DEBEN
  marcarse como parciales aunque el proyecto tenga un sprint Activo, porque se construyen únicamente
  con sprints Cerrados y con la métrica definitiva de Story Points completados.
- **FR-024**: Las métricas acumuladas que son cocientes DEBEN calcularse sobre los totales
  acumulados, nunca promediando los porcentajes de cada sprint.
- **FR-025**: Una historia planificada en más de un sprint DEBE contarse como planificada una vez por
  cada sprint en que estuvo, y como completada solo en el sprint en que se completó.
- **FR-026**: **Defectos del proyecto**: el sistema DEBE informar la cantidad de defectos detectados,
  resueltos y abiertos del proyecto.
- **FR-027**: El sistema DEBE considerar **abierto** a todo defecto que no esté Resuelto ni
  Descartado, es decir los que están en estado Abierto o En progreso.
- **FR-028**: El sistema DEBE acompañar cada uno de los tres conteos de defectos del proyecto con su
  desglose por severidad: Crítica, Alta, Media y Baja.
- **FR-029**: El sistema DEBE informar el **esfuerzo fuera de sprint** del proyecto: la suma de las
  horas de los registros de esfuerzo que no están asociados a ningún sprint, por haberse cargado
  sobre historias que estaban solo en el backlog (`specs/006-effort-tracking`, FR-014). Esa cifra
  DEBE presentarse como una métrica propia y separada, y NO DEBE sumarse a las horas reales
  acumuladas de FR-023, para que los acumulados sigan cuadrando exactamente con la suma de los
  sprints. El sistema DEBE informarla aunque sea 0.

**Cálculo, redondeo y valores no calculables**

- **FR-030**: Cuando el denominador de una división es 0, el sistema DEBE devolver **"no calculable"**
  para esa métrica. NUNCA DEBE devolver 0, un valor infinito, un texto vacío ni un error.
- **FR-031**: El valor "no calculable" DEBE alcanzar al menos a: la desviación porcentual cuando las
  horas estimadas son 0; el porcentaje de historias completadas cuando no hay historias
  planificadas; y la velocidad cuando no hay sprints cerrados.
- **FR-032**: Un valor "no calculable" en una métrica NO DEBE impedir el cálculo de las demás: el
  resto del conjunto se devuelve normalmente.
- **FR-033**: El sistema DEBE redondear los porcentajes y los promedios a 2 decimales **únicamente en
  el resultado final**, y NO DEBE redondear en ningún paso intermedio.
- **FR-034**: Las sumas de Story Points DEBEN ser números enteros, y las de horas DEBEN conservar la
  precisión de un cuarto de hora que `specs/006-effort-tracking` garantiza.
- **FR-035**: El sistema DEBE distinguir de forma visible un 0 real de un "no calculable" y de un "no
  aplicable": los tres significan cosas distintas y no deben presentarse igual.
- **FR-036**: El sistema DEBE informar, como métrica del proyecto, la **cantidad de historias del
  backlog sin estimar**: las historias del proyecto que llevan la marca "sin estimar" de
  `specs/003-product-backlog`. Es una advertencia, no un cálculo: avisa que hay trabajo pendiente
  sin dimensionar y que, por lo tanto, las cifras de planificación del proyecto pueden quedar
  cortas. El sistema DEBE acompañarla de esa advertencia cuando el valor sea mayor que 0.
- **FR-037**: El contador de FR-036 NO DEBE formar parte de las métricas por sprint, porque dentro de
  un sprint nunca hay historias sin estimar: `specs/004-sprint-management` (su FR-018) solo admite
  historias con Story Points asignados. Si una historia de un sprint apareciera sin estimar, el
  sistema DEBE tratarla como 0 Story Points, DEBE calcular igual el resto de las métricas y DEBE
  devolver una **advertencia de inconsistencia de datos** que identifique la historia y el sprint
  afectados. NO DEBE calcular en silencio ni rechazar la consulta: lo primero entregaría cifras
  subestimadas con apariencia de correctas y lo segundo dejaría inutilizable todo el conjunto de
  métricas por un único dato roto.
- **FR-038**: Toda respuesta de métricas DEBE poder acompañarse de una lista de **advertencias**, y
  el sistema DEBE distinguir dos clases:
  - **Advertencia de planificación**, cuando el dato es válido pero incompleto. Es el caso del
    contador de historias del backlog sin estimar (FR-036).
  - **Advertencia de inconsistencia de datos**, cuando el sistema encuentra una combinación que las
    reglas de las features anteriores no deberían permitir. Es el caso de FR-037.
  Las dos clases DEBEN presentarse de forma distinguible, porque la primera es parte de la operación
  normal y la segunda indica que hay datos rotos que alguien tiene que revisar.
- **FR-039**: La presencia de una advertencia NUNCA DEBE impedir la entrega de las métricas: el
  conjunto se devuelve completo y la advertencia viaja junto a él.

**Autorización y visibilidad**

- **FR-040**: El sistema DEBE exigir una sesión válida para toda consulta de métricas.
- **FR-041**: El sistema DEBE restringir la consulta de las métricas de un proyecto a sus integrantes
  vigentes.
- **FR-042**: Ante una solicitud sobre un proyecto o un sprint a los que quien pide no tiene acceso,
  el sistema DEBE responder exactamente igual que ante un identificador inexistente, sin revelar
  ningún dato ni la existencia del recurso.
- **FR-043**: El sistema DEBE permitir consultar las métricas de un proyecto en estado Finalizado,
  porque consultar no es una escritura.

---

### Reglas de Negocio

| ID | Regla |
| --- | --- |
| RN-01 | Esta feature es la única fuente de las fórmulas; el dashboard y los reportes consumen sus resultados sin recalcular. |
| RN-02 | Esta feature solo lee: nunca modifica datos de otras features. |
| RN-03 | Un sprint Cerrado usa exclusivamente su instantánea congelada para historias, Story Points y factor, y esas métricas son definitivas. |
| RN-04 | Un sprint Activo se calcula con los datos actuales y sus métricas se marcan como parciales. |
| RN-05 | Un sprint Planificado solo informa Story Points planificados y horas estimadas. |
| RN-06 | Los sprints Planificados no entran en los acumulados del proyecto ni en la velocidad. |
| RN-07 | Las horas estimadas son Story Points planificados por el factor aplicable al sprint. |
| RN-08 | La desviación positiva indica subestimación; la negativa, sobrestimación. |
| RN-09 | El porcentaje de historias completadas se cuenta por cantidad de historias, no por Story Points. |
| RN-10 | La velocidad promedia los últimos 3 sprints cerrados, o todos los cerrados si hay menos de 3. |
| RN-11 | Los cocientes acumulados se calculan sobre los totales, nunca promediando porcentajes por sprint. |
| RN-12 | Una historia planificada en varios sprints cuenta como planificada en cada uno y como completada solo en uno. |
| RN-13 | Un defecto cuenta como detectado en su sprint de detección y como resuelto en su sprint de resolución, nunca en los dos por el mismo concepto. |
| RN-14 | Un defecto reabierto deja de contar como resuelto, porque pierde su sprint de resolución. |
| RN-15 | Los defectos Descartados no cuentan en ningún conteo. |
| RN-16 | Un defecto abierto es el que no está ni Resuelto ni Descartado. |
| RN-17 | Toda división por cero da "no calculable", nunca 0 ni un error. |
| RN-18 | Un "no calculable" en una métrica no impide calcular las demás. |
| RN-19 | Los porcentajes y promedios se redondean a 2 decimales solo en el resultado final. |
| RN-20 | Un 0 real, un "no calculable" y un "no aplicable" se presentan de forma distinguible. |
| RN-21 | Solo los integrantes vigentes del proyecto consultan sus métricas. |
| RN-22 | Las métricas de un proyecto Finalizado se consultan con normalidad. |
| RN-23 | Cada métrica de un sprint cerrado informa si es definitiva o provisoria; las horas reales y sus desviaciones son provisorias hasta que vence el plazo de gracia, y los conteos de defectos son siempre provisorios. |
| RN-24 | "Parcial" califica a todas las métricas de un sprint Activo; "provisoria" califica a una métrica concreta de un sprint Cerrado. No son lo mismo. |
| RN-25 | El esfuerzo que no pertenece a ningún sprint se informa aparte y no se suma a las horas reales acumuladas. |
| RN-26 | La cantidad de historias del backlog sin estimar se informa como advertencia a nivel de proyecto, nunca por sprint. |
| RN-27 | El acumulado del proyecto es parcial cuando incluye al sprint activo y definitivo cuando solo contiene sprints cerrados. |
| RN-28 | La velocidad y su serie nunca son parciales, porque solo miran sprints cerrados. |
| RN-29 | Todas las métricas de una misma consulta corresponden a un único instante; una respuesta nunca se contradice a sí misma. |
| RN-30 | Ante una inconsistencia de datos, el sistema calcula igual, aplica el supuesto documentado y devuelve una advertencia que identifica el dato afectado. |
| RN-31 | Una advertencia nunca impide la entrega de las métricas, y las de planificación se distinguen de las de inconsistencia de datos. |

---

### Restricciones

- **RC-01**: Esta feature depende de `specs/001-user-auth`: solo usuarios con sesión válida consultan
  métricas.
- **RC-02**: Esta feature depende de `specs/002-project-members`: la membresía vigente es el permiso
  de acceso y el factor de horas por Story Point es el multiplicador de las horas estimadas.
- **RC-03**: Esta feature depende de `specs/003-product-backlog`: los Story Points, su escala y la
  marca "sin estimar" se definen allí.
- **RC-04**: Esta feature depende de `specs/004-sprint-management`: los estados del sprint y, sobre
  todo, la instantánea congelada al cierre (su FR-043), que es la que vuelve reproducibles las
  métricas históricas. Sin esa instantánea, reestimar una historia vieja cambiaría la velocidad de un
  sprint cerrado hace meses.
- **RC-05**: `specs/004-sprint-management` (su FR-018) solo admite en un sprint historias listas para
  planificar, es decir con Story Points asignados. En consecuencia, dentro de un sprint nunca hay
  historias "sin estimar", y la advertencia correspondiente se informa a nivel de proyecto contando
  las historias del backlog (FR-036, FR-037).
- **RC-06**: Esta feature depende de `specs/006-effort-tracking`: las horas reales de un sprint son
  las de los registros anclados a él, y ese ancla es inmutable (su FR-015), lo que hace que el
  esfuerzo por sprint sea estable aunque las historias se replanifiquen.
- **RC-07**: `specs/006-effort-tracking` (su RC-10) obliga a tratar como provisorio el esfuerzo de un
  sprint cuyo plazo de gracia sigue vigente, y como definitivo el del resto. Esta feature recoge esa
  obligación en FR-019, y necesita por lo tanto poder saber si el plazo de gracia de un sprint
  cerrado sigue abierto, dato que se deriva de su momento real de cierre y del plazo configurado del
  sistema (`specs/006-effort-tracking`, FR-016 y RC-11).
- **RC-08**: Esta feature depende de `specs/007-defect-tracking`: los estados del defecto y sus dos
  anclas de sprint. Su RC-09 obliga a excluir los defectos Descartados de todo conteo, recogido en
  FR-018.
- **RC-09**: `specs/007-defect-tracking` admite registrar un defecto cuyo sprint de detección es un
  sprint ya Cerrado (su FR-006), y una reapertura quita un defecto del conteo de resueltos (su
  FR-029). Por eso los dos conteos de defectos de un sprint cerrado pueden cambiar en cualquier
  momento y nunca llegan a ser definitivos (FR-019).
- **RC-10**: La presentación visual de estas métricas —gráficos, tableros, colores, formatos—
  corresponde a la feature de dashboard, y su exportación a la de reportes. Esta feature entrega
  valores, no vistas.
- **RC-11**: Las métricas de esta feature son de solo lectura y no tienen estado propio: no se
  almacenan resultados que puedan quedar desactualizados respecto de los datos de origen.
- **RC-12**: `specs/009-project-dashboard` consume las series de FR-022 para sus tres gráficos y
  tiene prohibido armarlas por su cuenta. Por eso las seis series salen de acá ya ordenadas y con el
  estado de valor de cada punto: si el dashboard tuviera que completarlas o reordenarlas, volvería a
  ser dueño de parte de lo que se grafica, que es justamente lo que la regla de fuente única evita.

---

### Condiciones de Error

| Condición | Comportamiento esperado |
| --- | --- |
| Consulta de métricas sin sesión válida | Rechazo con indicación de iniciar sesión. |
| Consulta sobre un proyecto inexistente | Respuesta de proyecto inexistente. |
| Consulta sobre un sprint inexistente | Respuesta de sprint inexistente. |
| Consulta sobre un sprint que pertenece a otro proyecto | Respuesta idéntica a la de un sprint inexistente, sin revelar dato alguno del otro proyecto. |
| Consulta de quien no es integrante vigente del proyecto | Respuesta idéntica a la de un identificador inexistente. |
| Consulta de las métricas del sprint activo cuando no hay ninguno activo | Indicación explícita de que no hay sprint activo; no es un error ni un conjunto de ceros. |
| División por cero en cualquier fórmula | La métrica se devuelve como "no calculable"; las demás se calculan igual. No es un error. |
| Proyecto o sprint sin datos suficientes para una métrica | La métrica se devuelve como "no calculable" o como no aplicable según corresponda; nunca como 0. |
| Consulta de métricas de un proyecto Finalizado | Se responde con normalidad: consultar no escribe. |
| Inconsistencia de datos detectada al calcular, como una historia sin estimar dentro de un sprint | Las métricas se calculan igual con el supuesto documentado y la respuesta incluye una advertencia de inconsistencia que identifica el dato afectado. No es un error y no bloquea la consulta. |

---

### Entidades Clave

- **Métricas de un sprint**: conjunto derivado, no almacenado, compuesto por Story Points
  planificados y completados, horas estimadas y reales, desviación absoluta y porcentual, porcentaje
  de historias completadas, defectos detectados y defectos resueltos, más el indicador de si el
  resultado es parcial. Cualquiera de los valores derivados puede ser "no calculable".
- **Métricas de un proyecto**: conjunto derivado, no almacenado, compuesto por la velocidad del
  equipo, la serie de velocidad por sprint, los acumulados de Story Points, horas, desviación y
  porcentaje de historias completadas, y los conteos de defectos con su desglose por severidad.
- **Valor no calculable**: estado posible de cualquier métrica que sea un cociente, cuando su
  denominador es 0. Es distinto de 0 y distinto de "no aplicable".
- **Valor no aplicable**: estado de una métrica que no tiene sentido para el estado del sprint
  consultado, como las horas reales de un sprint Planificado.
- **Advertencia**: aviso que viaja junto a un conjunto de métricas sin impedir su entrega. Tiene dos
  clases: de planificación, cuando el dato es válido pero incompleto, y de inconsistencia de datos,
  cuando el sistema encuentra una combinación que las reglas de las features anteriores no deberían
  permitir.
- **Instantánea de cierre** *(entidad de `specs/004-sprint-management`)*: fuente exclusiva de las
  historias, los Story Points y el factor de un sprint Cerrado.
- **Registro de esfuerzo** *(entidad de `specs/006-effort-tracking`)*: fuente de las horas reales, a
  través de su sprint asociado.
- **Defecto** *(entidad de `specs/007-defect-tracking`)*: fuente de los conteos de calidad, a través
  de su estado, su severidad y sus dos sprints.
- **Proyecto**, **Integrante** y **factor de horas por Story Point** *(entidades de
  `specs/002-project-members`)*: delimitan el alcance de cada cálculo y quién puede verlo.

---

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **SC-001**: El 100 % de las fórmulas definidas en FR-004 a FR-012 y FR-020 a FR-029 tiene al menos
  una prueba automatizada con datos concretos y resultado exacto esperado, tomada de los escenarios
  de esta especificación.
- **SC-002**: El 100 % de los resultados calculados coincide con el cálculo manual sobre el mismo
  conjunto de datos, verificado con el juego de ejemplo completo de los escenarios de US1 a US5.
- **SC-003**: Cero divisiones por cero que produzcan 0, un valor infinito o un error: en el 100 % de
  los casos degenerados el resultado es "no calculable", verificado con una prueba por cada
  denominador posible (horas estimadas, historias planificadas y cantidad de sprints cerrados).
- **SC-004**: El 100 % de los porcentajes y promedios se devuelve con exactamente 2 decimales, y
  ningún resultado difiere del valor obtenido redondeando únicamente al final, verificado con al
  menos un caso de decimal periódico como 1 / 3 y 49 / 3.
- **SC-005**: Las métricas de un sprint cerrado no cambian ante reestimaciones de sus historias ni
  cambios del factor del proyecto, verificado comparando el resultado antes y después de ambas
  modificaciones.
- **SC-006**: La velocidad coincide con el cálculo esperado para 0, 1, 2, 3 y 4 sprints cerrados,
  verificado con una prueba por cantidad.
- **SC-007**: El 100 % de las métricas de un sprint Activo viene marcado como parcial, y ninguna
  métrica de un sprint Cerrado viene marcada así.
- **SC-008**: Cero defectos Descartados contabilizados en cualquiera de los conteos, verificado
  comparando los totales antes y después de descartar un defecto.
- **SC-009**: Un defecto reabierto desaparece del conteo de resueltos de su sprint en el 100 % de los
  casos, y permanece en el de detectados.
- **SC-010**: El desglose por severidad suma exactamente el total de su conteo en el 100 % de los
  casos, verificado para detectados, resueltos y abiertos.
- **SC-011**: Un integrante obtiene las métricas completas de un sprint en una sola consulta, sin
  combinar información de otros lugares del sistema.
- **SC-012**: Dos consultas de la misma métrica sobre los mismos datos devuelven el mismo valor en el
  100 % de los casos.
- **SC-013**: El 100 % de las consultas exige sesión válida y membresía vigente, verificado con una
  prueba por tipo de consulta.
- **SC-014**: Cero filtraciones de datos entre proyectos: en el 100 % de los intentos de consulta de
  quien no es integrante, la respuesta es indistinguible de la de un identificador inexistente.
- **SC-015**: El 100 % de las métricas de un sprint cerrado viene con su estado, y la clasificación
  coincide con FR-019 en los tres casos, verificado consultando un sprint dentro de su plazo de
  gracia y otro con el plazo vencido.
- **SC-016**: Cero horas de esfuerzo invisibles: la suma de las horas reales acumuladas más el
  esfuerzo fuera de sprint iguala el total de horas registradas del proyecto en el 100 % de los
  casos.
- **SC-017**: La cantidad de historias del backlog sin estimar coincide con el conteo manual en el
  100 % de los casos, y nunca aparece entre las métricas de un sprint.
- **SC-018**: El acumulado del proyecto viene marcado como parcial en el 100 % de los casos en que
  hay un sprint Activo y como definitivo en el 100 % de los casos en que no lo hay, verificado
  consultando antes y después de cerrar el sprint activo. La velocidad nunca viene marcada como
  parcial en ninguno de los dos casos.
- **SC-019**: Cero respuestas internamente contradictorias, verificado consultando las métricas de
  un proyecto mientras se cargan horas y se completan historias de forma concurrente: en 20
  consultas de 20, los Story Points completados no superan a los planificados y la desviación
  informada coincide con las horas informadas en la misma respuesta.
- **SC-020**: El 100 % de las inconsistencias de datos detectables produce una advertencia
  identificable en la respuesta y ninguna bloquea la entrega de las métricas, verificado forzando
  una historia sin estimar dentro de un sprint y comprobando que el conjunto se devuelve completo,
  con esa historia contada como 0 puntos y la advertencia presente.
- **SC-021**: El 100 % de las reglas de negocio (RN-01 a RN-31) tiene al menos una prueba
  automatizada asociada que falla si la regla se rompe.

---

## Fuera de Alcance

- Gráficos y presentación visual de las métricas, que corresponden a la feature de dashboard. Esta
  feature entrega valores, no vistas.
- Reportes y exportación a PDF o cualquier otro formato, que corresponden a la feature de reportes.
- Predicciones, proyecciones y estimaciones de fecha de finalización a partir de la velocidad.
- Gráfico y cálculo de burndown.
- Métricas por integrante: esta feature no mide el rendimiento individual de las personas.
- Métricas comparativas entre proyectos distintos.
- Alertas, umbrales o semáforos sobre los valores calculados.
- Almacenamiento histórico de los resultados calculados: las métricas se derivan de los datos de
  origen en cada consulta.
- Definición de métricas nuevas por parte del usuario.

---

## Supuestos

- **"Los últimos 3 sprints cerrados" se ordenan por momento real de cierre**: es el criterio que
  refleja la secuencia real de trabajo del equipo. `specs/004-sprint-management` registra ese momento
  al cerrar (su FR-040), así que el dato existe siempre.
- **La serie de velocidad se ordena cronológicamente por fecha de inicio prevista**: como los
  períodos de dos sprints del mismo proyecto no se superponen, ese orden es total y no ambiguo. Es el
  mismo criterio que `specs/007-defect-tracking` usa para comparar sprints.
- **El porcentaje de historias completadas cuenta historias, no Story Points**: el enunciado lo
  define así explícitamente, y convive con el porcentaje de puntos que se deduce de los Story Points
  planificados y completados.
- **Los acumulados suman los sprints y el esfuerzo fuera de sprint va aparte**: decisión confirmada
  el 2026-10-03 (FR-029, RN-25). El acumulado sigue siendo exactamente la suma de los sprints, de
  modo que siempre cuadra con las métricas por sprint, y las horas que nunca entraron a un sprint se
  informan como una cifra propia en vez de perderse. Quien necesite el total de horas del proyecto
  suma las dos cifras a la vista.
- **Hay tres estados posibles para una métrica de sprint cerrado**: decisión confirmada el
  2026-10-03 (FR-019, RN-23). Las cuatro que salen de la instantánea son definitivas; las horas
  reales y sus desviaciones son provisorias hasta que vence el plazo de gracia; los dos conteos de
  defectos no llegan a ser definitivos nunca. Esto último es una consecuencia aceptada de que la
  spec 007 permita registrar defectos contra sprints ya cerrados sin límite de tiempo: la
  alternativa habría sido ponerle un plazo también a los defectos, que nadie pidió.
- **"Parcial" y "provisoria" son términos distintos y no intercambiables**: parcial es del conjunto
  de un sprint activo; provisoria es de una métrica puntual de un sprint cerrado. Mezclarlos haría
  que un sprint cerrado pareciera estar todavía corriendo.
- **Una historia replanificada infla los Story Points planificados acumulados**: es la consecuencia
  directa de contarla en cada sprint, y es deseable, porque el acumulado mide esfuerzo de
  planificación y no tamaño del backlog.
- **"Defecto abierto" es el que no está Resuelto ni Descartado**: el enunciado pide contar los
  abiertos sin definirlos, y `specs/007-defect-tracking` tiene cuatro estados. Agrupar Abierto y En
  progreso es la única lectura en la que detectados, resueltos y abiertos cubren todo el universo sin
  solaparse.
- **Los conteos de defectos no son "no calculable"**: contar no divide, así que la ausencia de
  defectos da 0, que es un dato real.
- **Las horas estimadas no se redondean**: la regla de redondeo alcanza a porcentajes y promedios.
  Como el factor tiene hasta 2 decimales y los Story Points son enteros, el producto ya tiene a lo
  sumo 2 decimales.
- **Las métricas se calculan al momento de la consulta**: no se guardan resultados precalculados, de
  modo que nunca puede haber una métrica desactualizada respecto de sus datos de origen.
- **Coherencia interna por respuesta, no inmutabilidad entre consultas**: decisión confirmada el
  2026-10-03 (FR-003, RN-29). Cada respuesta corresponde a un único instante, así que sus cifras
  cuadran entre sí. Dos consultas separadas por una escritura devuelven valores distintos, y eso es
  correcto: la garantía es de consistencia interna, no de que el número se quede quieto.
- **Un proyecto Finalizado conserva métricas consultables**: es donde más sentido tiene mirarlas, para
  cerrar el aprendizaje del proyecto.
- **Idioma de la interfaz**: los mensajes y las etiquetas que ve la persona están en español.

---

## Preguntas Abiertas

Ninguna. Las tres decisiones que quedaron abiertas al redactar la especificación se resolvieron el
2026-10-03 y están registradas en la sección Clarifications:

1. **Qué métricas de un sprint cerrado son definitivas** (FR-019, RN-23, RN-24, SC-015): cada
   métrica lleva su estado. Cuatro son siempre definitivas, tres son provisorias hasta que vence el
   plazo de gracia y los dos conteos de defectos son siempre provisorios.
2. **El esfuerzo sin sprint asociado** (FR-029, RN-25, SC-016): se informa aparte como "esfuerzo
   fuera de sprint", sin sumarse al acumulado.
3. **La advertencia de historias sin estimar** (FR-036, FR-037, RN-26, RC-05, SC-017): se informa a
   nivel de proyecto contando el backlog, nunca por sprint.

Las demás decisiones que podrían haber quedado abiertas se resolvieron con supuestos explícitos,
documentados en la sección Supuestos.
