# Investigación (Fase 0): Cálculo de Métricas

**Feature**: `specs/008-metrics-calculation` | **Fecha**: 2026-10-26 | **Plan**: [plan.md](./plan.md)

Este documento resuelve las incógnitas técnicas del plan. Las decisiones de producto ya están
cerradas en la spec (sesiones de clarificación del 2026-10-03); acá solo se decide **cómo** se
construyen. Cada sección sigue el formato Decisión / Rationale / Alternativas consideradas.

---

## 0. Contexto: sobre qué se construye esta feature

Esta feature no escribe nada (FR-002, RN-02): lee los datos de seis features y los convierte en
métricas en el momento de la consulta (RC-11).

| Feature | Qué aporta | Cómo lo consume la 008 |
| --- | --- | --- |
| 001 | Sesión JWT, `RequireSession` | Toda consulta exige sesión (FR-040) |
| 002 | `projects.hours_per_story_point_hundredths`, `project_members`, `RequireMembership` | Factor vigente (sprint Activo y Planificado) y acceso (FR-041, FR-042) |
| 003 | `user_stories.story_points` (NULL = sin estimar), `user_stories.status` | Puntos vigentes, historias completadas del sprint Activo y contador del backlog sin estimar (FR-036) |
| 004 | `sprints` (estado, fechas, `closed_at`, factor congelado) y `sprint_stories` (instantánea: `story_points_at_close`, `outcome`) | Fuente exclusiva de un sprint Cerrado (FR-013), orden de la ventana de velocidad (FR-021) y de las series |
| 005 | — | Ninguna relación |
| 006 | `effort_entries.sprint_id` y `hours_hundredths`; `GraceOpen`; `RoundPercentHundredths`; `EFFORT_GRACE_PERIOD_HOURS` | Horas reales por sprint (FR-007), esfuerzo fuera de sprint (FR-029), estado provisorio (FR-019) y redondeo compartido (RC-13 de la 006) |
| 007 | `defects` (`status`, `severity`, `detected_sprint_id`, `resolved_sprint_id`) | Conteos de calidad (FR-011, FR-012, FR-026 a FR-028) |

**Dependencia de implementación**: el módulo `defects` de la 007 está planificado pero todavía no
está en `main`. Solo US5 lee la tabla `defects` y usa los tipos de `defectsdomain`, y recién ahí se
declaran los tipos de defectos del dominio de métricas. Por eso las historias US1 a US4 pueden
implementarse con la 001 a la 006 en `main`, y US5 espera a que se integre la 007 completa.

---

## 1. Versiones del stack: sin cambios

### Decisión

Se mantienen las versiones vigentes en `main`: Go 1.27.3, Gin v1.12.0, GORM v1.31.2 con
`gorm.io/driver/postgres` v1.6.3, `google/uuid` v1.6.0, `swaggo/*` y `testify` v1.12.1. **Ninguna
dependencia nueva** y **ninguna variable de entorno nueva**: el plazo de gracia se toma de
`EFFORT_GRACE_PERIOD_HOURS`, que ya carga `config.Load` para la 006.

### Rationale

Todo lo que pide la spec son sumas, conteos y cocientes sobre enteros. La librería estándar y GORM
alcanzan; una librería de decimales sería innecesaria porque la aritmética se hace en centésimas
enteras (sección 5).

### Alternativas consideradas

- **`shopspring/decimal`**: resolvería el redondeo, pero la 006 ya fijó la convención con enteros y
  RC-13 de la 006 obliga a compartirla. Requeriría autorización (Principio IV) sin aportar nada.
- **Una variable propia para el plazo de gracia de métricas**: dos variables para un mismo plazo
  podrían divergir, y FR-019 depende exactamente del plazo de la 006 (RC-07).

---

## 2. Un módulo `metrics` de solo lectura (FR-001, FR-002, RC-11)

### Decisión

Un módulo vertical `backend/internal/metrics/` con sus cuatro capas y **ninguna tabla propia**:

- `domain/`: tipos de valor, reglas de estado y **todas las fórmulas como funciones puras** que reciben
  los datos leídos y el instante de la consulta. Es la única fuente de las fórmulas (RN-01).
- `repository/`: una sola implementación GORM que lee, en una transacción de solo lectura, los datos
  crudos del proyecto o del sprint (sección 4). No calcula: solo suma y cuenta por sprint.
- `service/`: toma el instante del reloj inyectado y el plazo de gracia, llama al repositorio y
  aplica las funciones del dominio. Es también el punto de entrada que reutilizará la 010 dentro del
  proceso.
- `delivery/`: cuatro manejadores `GET`, la traducción a la forma de la API y sus anotaciones Swagger.

No hay `AutoMigrate` nuevo ni vaciado nuevo en `testsupport`.

### Rationale

Separar "leer" de "calcular" deja las fórmulas en funciones puras con pruebas unitarias exhaustivas,
que es lo que exige el Principio II para métricas, y deja al repositorio con una responsabilidad
única. Sin persistencia no hay métrica que pueda quedar desactualizada (RC-11).

### Alternativas consideradas

- **Calcular en SQL** (`SUM`, `AVG`, `ROUND` en la consulta): la fórmula quedaría repartida entre SQL y
  Go, el redondeo de PostgreSQL (`numeric`) no es el de la 006, y las pruebas unitarias no podrían
  ejercitar las fórmulas sin base de datos.
- **Repartir las métricas en los módulos de origen** (velocidad en `sprints`, horas en `effort`):
  contradice RN-01; la spec quiere un único lugar.
- **Vistas materializadas o tablas de resultados**: prohibidas por RC-11 y por "Fuera de Alcance".

---

## 3. Rutas: todo cuelga del proyecto (FR-016, FR-040 a FR-043, US1-13 a US1-15)

### Decisión

Cuatro endpoints `GET` en el subgrupo `/api/v1/projects/:projectId` de la 002, que ya exige sesión y
`RequireMembership`:

| Ruta | Qué devuelve |
| --- | --- |
| `GET /api/v1/projects/{projectId}/metrics` | Métricas del proyecto: velocidad con su serie, acumulados, esfuerzo fuera de sprint y en sprints planificados, backlog sin estimar, defectos del proyecto y advertencias |
| `GET /api/v1/projects/{projectId}/metrics/series` | Las seis series del proyecto (FR-022) |
| `GET /api/v1/projects/{projectId}/metrics/active-sprint` | Métricas del sprint Activo o la indicación de que no hay ninguno (FR-016) |
| `GET /api/v1/projects/{projectId}/metrics/sprints/{sprintId}` | Métricas de un sprint en cualquier estado |

### Rationale

- **US1-14 y US1-15 piden respuestas distintas**: un integrante que pide un sprint de otro proyecto
  recibe "sprint inexistente", y quien no es integrante recibe "proyecto inexistente". Con el sprint
  bajo el proyecto, `RequireMembership` resuelve el segundo caso con el `404` de la 002 y el
  repositorio resuelve el primero buscando el sprint **con** `project_id` igual al de la ruta, con el
  `404` y el cuerpo de la 004 (`"El sprint no existe."`). Bajo el grupo plano `/api/v1/sprints/:id`
  de la 004, quien no es integrante recibiría el `404` de sprint y no el de proyecto.
- `active-sprint` es un segmento estático distinto de `sprints/:sprintId`, así que no compite con el
  parámetro en el árbol de rutas de Gin.
- Los defectos del proyecto viajan en la respuesta del proyecto y los del sprint en la del sprint:
  la tabla "Métricas de defectos" de la spec no necesita un endpoint propio. La serie de velocidad
  "consultable por separado" (FR-022) es `velocity.series` de la respuesta del proyecto, restringida a
  sprints Cerrados.
- Las series tienen endpoint propio porque el dashboard las pide como una sección aparte que puede
  fallar sola (009, FR-028), y porque son la parte más pesada de la respuesta.
- Un proyecto Finalizado se consulta igual (FR-043): `RequireMembership` no mira el estado.

### Alternativas consideradas

- **Un único endpoint con todo**: el dashboard no podría degradar por sección, y cada consulta de un
  sprint arrastraría las series completas.
- **Un endpoint de defectos y uno de velocidad**: duplicarían datos de la respuesta del proyecto sin
  un consumidor que los pida por separado (Principio I).
- **`/sprints/active/metrics`**: haría competir `active` con `:sprintId` en el mismo segmento.

---

## 4. Coherencia: una transacción `REPEATABLE READ` de solo lectura por consulta (FR-003, RN-29, SC-019)

### Decisión

Cada método del repositorio abre **una** transacción con
`sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}` y hace en ella todas sus lecturas.
El servicio toma `now` **una sola vez** por consulta, del reloj inyectado, y lo usa para todas las
decisiones de plazo de gracia de esa respuesta.

### Rationale

En PostgreSQL, `REPEATABLE READ` fija una instantánea de la base en la primera sentencia de la
transacción: todas las lecturas siguientes ven exactamente ese instante, aunque otra transacción
confirme cambios en el medio. Es la garantía literal de FR-003 ("una vista coherente, correspondiente
a un único instante") sin bloquear a nadie: las lecturas no toman bloqueos de fila, así que no puede
haber interbloqueos con las escrituras de la 003, la 004, la 006 ni la 007. `ReadOnly` hace que la
base rechace cualquier escritura accidental (FR-002). Como no hay escrituras en la transacción,
`REPEATABLE READ` nunca produce errores de serialización.

Tomar `now` una vez evita que, en el borde exacto del plazo de gracia, dos métricas del mismo sprint
salgan con estados distintos.

### Alternativas consideradas

- **`READ COMMITTED` (el defecto)**: cada sentencia ve su propio instante; una carga de horas entre
  la lectura de las historias y la del esfuerzo produciría una desviación que no cuadra con las horas
  de al lado (SC-019).
- **Una única sentencia SQL gigante**: daría el mismo instante, pero mezclaría en una consulta
  ilegible lo que son cinco lecturas independientes.
- **`SERIALIZABLE`**: más caro y sin beneficio para una transacción que no escribe.
- **Bloqueos `FOR SHARE`**: frenarían las escrituras de las otras features para servir una lectura.

---

## 5. Aritmética exacta y redondeo compartido (FR-030, FR-033, FR-034, SC-003, SC-004)

### Decisión

- Toda magnitud se maneja como **entero**: Story Points en unidades, horas y porcentajes en
  **centésimas**, igual que la 006. Las horas reales son la suma de `hours_hundredths`, siempre
  múltiplos de 25 (cuarto de hora, FR-034). Las horas estimadas son `puntos × factor en centésimas`,
  un producto entero exacto sin redondeo (Supuestos de la spec).
- Los cocientes se calculan con **una única** función de redondeo, `effortdomain.RoundPercentHundredths`
  (redondeo mitad lejos de cero, en un solo paso), que RC-13 de la 006 obliga a compartir:
  - Desviación porcentual = `RoundPercentHundredths(desviación, estimadas)`.
  - Porcentaje de historias completadas = `RoundPercentHundredths(completadas, planificadas)`.
  - Velocidad (promedio en centésimas de punto) = `RoundPercentHundredths(suma, 100 × cantidad)`,
    que es `suma × 100 / cantidad` redondeado una sola vez. Se envuelve en
    `metricsdomain.AverageHundredths` para que la lectura sea obvia.
- Un denominador 0 **no llega nunca** a la función: el dominio lo detecta antes y devuelve el estado
  `not_calculable` (sección 6).
- El tipo `Hundredths` que la 006 usa para serializar centésimas como número JSON con exactamente 2
  decimales se **mueve** a `platform/httpserver/hundredths.go`, y `effort/delivery` pasa a usarlo con
  un alias de tipo (`type Hundredths = httpserver.Hundredths`) para no cambiar ninguna de sus líneas
  de uso.

Comprobación con los escenarios: 1650/12600 → 1309,52 → **13,10**; −1600/12600 → **−12,70**; 2/4 →
**50,00**; 1/3 → **33,33**; 4900/3 → **16,33**; −6550/37800 → **−17,33**; 7/11 → **63,64**.

### Rationale

Con enteros no hay error de punto flotante en ningún paso intermedio, que es exactamente lo que pide
FR-033 ("redondear únicamente en el resultado final"). Compartir la función con la 006 cumple RC-13
de esa feature: si la convención cambiara, cambia en un solo lugar. Mover `Hundredths` en lugar de
copiarlo cumple DRY sin que `metrics/delivery` importe `effort/delivery`.

### Alternativas consideradas

- **`float64` y `math.Round` al final**: 16,5 / 126 × 100 en coma flotante puede quedar a un ulp del
  empate y redondear distinto según el orden de las operaciones.
- **Copiar `Hundredths` en `metrics/delivery`**: duplica una pieza con reglas de signo no triviales
  (Principio I, DRY).
- **Que `metrics/delivery` importe `effort/delivery`**: el proyecto evita importar la capa de entrega
  de otro módulo; los únicos cruces se componen en `cmd/app/`.

---

## 6. Valor, "no calculable" y "no aplicable" (FR-030 a FR-032, FR-035, RN-17, RN-18, RN-20)

### Decisión

Cada métrica viaja como un objeto con un **estado explícito** y un valor que solo existe en un caso:

```json
{ "status": "available",      "value": 13.10 }
{ "status": "not_calculable", "value": null }
{ "status": "not_applicable", "value": null }
```

En el dominio es `Metric{Status ValueStatus; Value int}`, con `Value` en la unidad del campo
(Story Points o centésimas). Los conteos de defectos del proyecto y las dos cifras de esfuerzo
separado son enteros o centésimas simples, sin estado, porque contar y sumar no dividen y nunca son
"no calculable" (Supuestos de la spec).

### Rationale

Un `null` solo no alcanza: tendría que significar a la vez "no calculable" y "no aplicable", y FR-035
exige distinguirlos. Con el estado explícito, un 0 real (`available`, `0`) no se confunde con
ninguno de los dos, y el dashboard puede explicar el motivo (009, FR-014) sin inferirlo. Cada métrica
lleva su propio estado, así que una no calculable no afecta a las demás (FR-032).

### Alternativas consideradas

- **`null` + un campo `reason` opcional**: obliga a mirar dos campos para saber si hay valor, y un
  consumidor descuidado leería `null` como 0.
- **Omitir la métrica**: borra la diferencia entre "no aplica" y "me olvidé de mandarla".
- **Códigos centinela (`-1`)**: un número que miente, exactamente lo que la spec prohíbe.

---

## 7. Estado de valor: definitiva, provisoria y parcial (FR-014, FR-019, FR-023, RN-23, RN-24, RN-27, RN-28)

### Decisión

Las métricas de un sprint llevan además `finality`:

| Sprint | Métricas | `finality` |
| --- | --- | --- |
| Cerrado | Story Points planificados y completados, horas estimadas, % de historias completadas | `definitive` siempre |
| Cerrado | Horas reales, desviación absoluta y porcentual | `provisional` si `GraceOpen(closed_at, now, grace)`, `definitive` si no |
| Cerrado | Defectos detectados y resueltos | `provisional` siempre |
| Activo | Las nueve | `partial` |
| Planificado | Story Points planificados y horas estimadas | `null` (ver abajo) |
| Planificado | Las otras siete | `null`, con `status: not_applicable` |

La decisión es una función pura, `SprintFinality(sprintStatus, metricKind, graceOpen)`, con una prueba
unitaria por combinación.

El **acumulado** del proyecto lleva un único `finality` para el bloque entero: `partial` si incluye al
sprint Activo y `definitive` si no (FR-023). Sus cifras no llevan `finality` propio.

La **velocidad** y su serie no tienen campo `finality`: por construcción nunca pueden marcarse como
parciales (RN-28).

**Sprint Planificado**: sus dos cifras no llevan estado de valor (FR-015, aclarado el 2026-10-26), y se
devuelve `finality: null` en lugar de inventar un término nuevo o de reusar "parcial", que la spec
reserva al sprint que está corriendo (RN-24). Sus dos cifras son una previsión que puede cambiar hasta que el sprint arranque; el
dashboard no las muestra (009 solo usa el Activo y las series, que excluyen a los Planificados).

### Rationale

Un enum con tres valores distintos impide confundir "provisoria" con "parcial" (RN-24). Ponerlo en
cada métrica del sprint permite que SC-007 y SC-015 se verifiquen métrica por métrica. En el
acumulado, la spec dice expresamente que la marca califica al conjunto, no a cada cifra.

`GraceOpen` es la función de la 006 (`now < closed_at + grace`); reutilizarla garantiza que el
esfuerzo de un sprint deja de poder corregirse exactamente en el mismo instante en que sus horas
pasan a definitivas (RC-07). Un sprint Cerrado sin `closed_at` se trata como plazo vencido, con el
mismo criterio que `EnsureEntryWritable` de la 006, y produce además una advertencia de inconsistencia
(sección 13).

### Alternativas consideradas

- **Un booleano `partial` en el sprint y otro `provisional` en cada métrica**: dos campos para un
  estado con tres valores, con combinaciones imposibles (`partial` y `provisional` a la vez).
- **`finality: "partial"` para el Planificado**: haría que un sprint que no arrancó parezca en curso.

---

## 8. De dónde sale cada dato según el estado del sprint (FR-004 a FR-007, FR-013 a FR-015, FR-025)

### Decisión

| Dato | Sprint Cerrado | Sprint Activo o Planificado |
| --- | --- | --- |
| Historias planificadas | Filas de `sprint_stories` del sprint (la instantánea; incluye las `not_completed`) | Filas de `sprint_stories` del sprint con `outcome IS NULL` |
| Story Points de cada historia | `sprint_stories.story_points_at_close` | `user_stories.story_points` vigente |
| Historia completada | `outcome = 'completed'` | `user_stories.status = 'completed'` |
| Factor | `sprints.hours_per_story_point_hundredths` (congelado) | `projects.hours_per_story_point_hundredths` (vigente) |
| Horas reales | `SUM(effort_entries.hours_hundredths)` con `sprint_id` = el sprint | Ídem (en el Planificado no se informa como métrica; sección 11) |
| Defectos detectados | `COUNT(*)` de `defects` con `detected_sprint_id` = el sprint y `status <> 'discarded'` | Ídem (en el Planificado no se informa) |
| Defectos resueltos | `COUNT(*)` de `defects` con `resolved_sprint_id` = el sprint y `status = 'resolved'` | Ídem |

El repositorio devuelve por sprint la lista de historias con `(story_id, title, story_points *int,
completed bool)`, más las horas y los dos conteos ya sumados; el dominio suma los puntos y cuenta las
historias.

### Rationale

- La instantánea es la fuente exclusiva del Cerrado (FR-013): ninguna reestimación ni cambio de factor
  la toca (US1-3, US1-4, SC-005).
- En el Activo, "completada" usa el **mismo criterio** con el que `Close` congela el `outcome`
  (`sprintsdomain.Outcome`): en el instante del cierre, los Story Points completados del Activo
  coinciden con los del Cerrado.
- Contar la historia una vez por sprint (FR-025, RN-12) sale solo: cada compromiso es una fila de
  `sprint_stories`, y una historia replanificada tiene una fila por sprint.
- `defects.resolved_sprint_id` es no nulo si y solo si el defecto está Resuelto (007, RN-23), así que
  la condición `status = 'resolved'` es redundante pero explícita; protege el conteo si algún dato
  rompiera esa regla (FR-012 lo define con ambas condiciones).
- El esfuerzo se suma por `sprint_id`, que es inmutable (006, FR-015), así que las horas por sprint no
  se mueven cuando una historia se replanifica (RC-06).

### Alternativas consideradas

- **Reusar `sprintResultsSelect` de `sprints/repository`**: obligaría a `metrics/repository` a
  importar el repositorio de otro módulo. Ese fragmento sirve al detalle del sprint de la 004, que es
  una pantalla de esa feature; se deja señalado en el plan (puntos al desarrollador).
- **`sprint_stories.completed_at IS NOT NULL` para el Activo**: es equivalente mientras la 004 lo
  mantenga, pero el criterio de cierre es el estado de la historia, y conviene usar el mismo.

---

## 9. Velocidad y series (FR-020 a FR-022, US2, SC-006)

### Decisión

- **Ventana de velocidad**: los sprints Cerrados ordenados por `closed_at DESC, id DESC`; se toman los
  3 primeros, o todos si hay menos (FR-020, FR-021). Con 0 cerrados es `not_calculable` (FR-031). La
  respuesta informa cuántos sprints entraron en el promedio.
- **Orden cronológico** de la serie de velocidad y de las seis series: `start_date ASC, id ASC`
  (Supuestos de la spec; los períodos no se superponen, y `id` solo desempata ante datos rotos).
- **Serie de velocidad** (`velocity.series` del proyecto): un punto por sprint Cerrado, con el
  `completedStoryPoints` de la instantánea, sin promediar ni recortar (US2-5). Sin `finality`.
- **Seis series** (`/metrics/series`): cada serie es una lista de puntos; un punto identifica su sprint
  (`sprintId`, `sprintName`, `sprintStatus`) y lleva el `Metric` con su `finality`. Incluyen los
  Cerrados y el Activo; nunca los Planificados (FR-017). Las seis tienen **exactamente** los mismos
  sprints en el mismo orden: un sprint sin datos aparece con `0` (US2-9), y "ausencia de dato"
  significa que el sprint no está en la serie.
- Un sprint incluido siempre tiene valor en las seis: los puntos, las horas y los conteos son sumas,
  y las horas estimadas son un producto. Por eso ninguna serie lleva `not_calculable`.

### Rationale

La spec usa dos órdenes distintos a propósito: el momento de cierre decide **qué** entra en la
ventana, y la fecha de inicio decide **cómo** se dibuja la secuencia. Que las seis series compartan
los sprints es lo que permite al dashboard graficarlas sin completar puntos faltantes (009, FR-026).

### Alternativas consideradas

- **Ordenar la ventana por `end_date`**: un sprint puede cerrarse antes o después de su fin previsto;
  FR-021 pide el momento real.
- **Una matriz `sprints × métricas`**: más compacta, pero la spec pide series, y cada consumidor
  tendría que transponerla.

---

## 10. Acumulados del proyecto (FR-023 a FR-025, US3, SC-018)

### Decisión

El dominio calcula primero las métricas de cada sprint Cerrado y del Activo, y después **suma los
totales** (Story Points planificados y completados, horas estimadas y reales, historias planificadas y
completadas). Las desviaciones y el porcentaje se calculan sobre esos totales, nunca promediando
porcentajes (FR-024, RN-11). Con todos los sprints Planificados o sin sprints, los totales son 0 y los
cocientes `not_calculable` (US3-7, US3-8).

### Rationale

Sumar sobre las métricas ya calculadas por sprint garantiza que el acumulado sea exactamente la suma
de lo que muestra cada sprint (FR-003), incluida la mezcla de factores congelados y vigente que la
spec acepta por construcción.

### Alternativas consideradas

- **Una consulta agregada por proyecto**: tendría que repetir la regla "instantánea o datos vigentes"
  en SQL, y el acumulado podría divergir de la suma de los sprints.

---

## 11. Esfuerzo fuera de sprint y en sprints Planificados (FR-015, FR-029, RN-25, SC-016)

### Decisión

Las métricas del proyecto informan **dos** cifras de esfuerzo separadas de los acumulados, ambas
aunque valgan 0:

- `effortOutsideSprints`: suma de los registros con `sprint_id IS NULL` de historias del proyecto
  (FR-029).
- `effortInPlannedSprints`: suma de los registros anclados a sprints en estado Planificado.

La invariante de SC-016 que se prueba pasa a ser **horas reales acumuladas + esfuerzo fuera de sprint
+ esfuerzo en sprints Planificados = total de horas registradas del proyecto**.

**Esta decisión la tomó el desarrollador el 2026-10-26** durante la planificación, y se integró a la
spec con `/speckit-clarify` el mismo día (FR-029, RN-25, SC-016, la tabla de "Métricas del proyecto"
y el escenario US3-12).

### Rationale

La 006 ancla al sprint Planificado el esfuerzo de una historia comprometida en él (006, FR-014 y su
US1-3), y ese anclaje es inmutable. Sin la segunda cifra, esas horas no estarían en los acumulados
(FR-017), ni en las horas reales del Planificado (FR-015, "no aplicable"), ni en el esfuerzo fuera de
sprint (que FR-029 define como "no asociado a ningún sprint"): serían invisibles, contra SC-016.
Cuando el sprint se inicia, sus horas pasan solas a los acumulados, porque el ancla no cambia.

### Alternativas consideradas (presentadas al desarrollador)

- **Sumarlas al esfuerzo fuera de sprint**: cambia la definición de FR-029 y mezcla horas que sí tienen
  sprint con las que no.
- **Informarlas como horas reales del Planificado**: contradice FR-015 y RN-05, y aun así quedarían
  afuera de los acumulados.

---

## 12. Defectos (FR-011, FR-012, FR-018, FR-026 a FR-028, US5, SC-008 a SC-010)

### Decisión

- **Por sprint**: detectados y resueltos con los dos conteos de la sección 8.
- **Por proyecto**: una consulta agrupada por `(status, severity)` sobre los defectos de historias del
  proyecto, excluyendo `discarded`. El dominio arma:
  - detectados = todos los no descartados;
  - resueltos = `resolved`;
  - abiertos = `open` + `in_progress` (FR-027, RN-16);
  - cada uno con el desglose por las cuatro severidades, que siempre aparecen, aunque valgan 0
    (FR-028, US5-7).
- Los conteos del proyecto son enteros sin `status` de valor: contar no divide (Supuestos).
- Los valores de severidad y estado se toman de `defectsdomain` (007), no se repiten como cadenas.

### Rationale

Derivar los tres conteos de un único agrupamiento hace que el desglose sume el total por construcción
(SC-010) y que detectados = resueltos + abiertos siempre cuadre. Un defecto reabierto pierde su
`resolved_sprint_id` (007, FR-029), así que sale del conteo de resueltos sin lógica adicional (RN-14,
SC-009).

### Alternativas consideradas

- **Tres consultas, una por conteo**: dentro de la misma transacción serían coherentes, pero la suma
  podría no cuadrar si cambia la definición de una sola de ellas.

---

## 13. Advertencias (FR-036 a FR-039, RN-26, RN-30, RN-31, SC-017, SC-020)

### Decisión

Toda respuesta lleva `warnings`, una lista (vacía si no hay ninguna) de objetos con:

- `kind`: `planning` o `data_inconsistency` (FR-038).
- `code`: identificador estable en inglés.
- `message`: texto en español para la persona.
- `sprintId`, `storyId`, `count`: opcionales, según el código.

Códigos:

| `kind` | `code` | Cuándo | Supuesto aplicado |
| --- | --- | --- | --- |
| `planning` | `unestimated_backlog_stories` | El proyecto tiene historias con `story_points IS NULL` (FR-036). Solo en la respuesta del proyecto | — |
| `data_inconsistency` | `unestimated_story_in_sprint` | Una historia de un sprint no tiene Story Points (vigentes en el Activo, congelados en el Cerrado) (FR-037) | Cuenta como 0 puntos |
| `data_inconsistency` | `missing_frozen_factor` | Un sprint Cerrado sin factor congelado | Se usa el factor vigente del proyecto |
| `data_inconsistency` | `missing_close_time` | Un sprint Cerrado sin `closed_at` | Plazo de gracia vencido; queda último en la ventana de velocidad |

Las advertencias de inconsistencia de un sprint viajan en la respuesta de ese sprint y en las del
proyecto (métricas y series) cuando el sprint entra en ellas. El contador `unestimatedBacklogStories`
se informa siempre como número (0 incluido); la advertencia `planning` solo aparece si es mayor que 0
(US3-14).

### Rationale

La 004 no permite ninguno de los tres datos rotos, pero los punteros existen en el modelo y el
Principio VII prohíbe un `panic` o un 500 por ellos. RN-30 pide calcular igual con un supuesto
documentado y avisar identificando el dato. Separar `kind` de `code` permite que el dashboard
distinga las dos clases (009, FR-032) sin conocer cada código.

### Alternativas consideradas

- **Solo el caso de FR-037**: dejaría sin definir el comportamiento ante los otros dos punteros nulos,
  que el código tiene que manejar de todos modos.
- **Responder 500 ante un dato roto**: prohibido por FR-039 y RN-31.

---

## 14. Acceso y códigos HTTP (FR-040 a FR-043, SC-013, SC-014)

### Decisión

| Situación | Respuesta |
| --- | --- |
| Sin sesión | `401` `{"error":"Se requiere iniciar sesión."}` de la 001 |
| Proyecto inexistente, ajeno o con identificador mal formado | `404` `{"error":"El proyecto no existe."}` de `RequireMembership` (002) |
| Sprint inexistente, de otro proyecto o con identificador mal formado | `404` `{"error":"El sprint no existe."}`, el mismo cuerpo que la 004 |
| Proyecto sin sprint Activo en `active-sprint` | `200` `{"activeSprint": false, "metrics": null}` (FR-016) |
| Cualquier otra consulta válida, incluido un proyecto Finalizado | `200` |
| Error inesperado | `500` genérico registrado con `slog` |

`metrics/delivery` no tiene middleware propio: el sprint se busca dentro del repositorio con
`project_id` de la ruta y su ausencia vuelve como `metricsdomain.ErrSprintNotFound`.

### Rationale

Reusar los mismos cuerpos hace que las respuestas sean byte a byte idénticas a las de un identificador
inexistente en cada feature (SC-014). No hay `400`: ninguna ruta tiene cuerpo ni query.

### Alternativas consideradas

- **`404` para "no hay sprint activo"**: la spec dice que no es un error (FR-016, US4-7).
- **`204 No Content`**: no lleva una indicación explícita en el cuerpo.

---

## 15. Pruebas (Principio II, SC-001 a SC-021)

### Decisión

- **Unitarias** en `metrics/domain`: todas las fórmulas, `AverageHundredths`, `SprintFinality`, la
  ventana de velocidad, el armado de series, los acumulados, los conteos de defectos, las advertencias
  y la distinción de los tres estados de valor, con datos armados en memoria y un `now` fijo. Cubren
  los casos degenerados con una prueba por denominador (SC-003) y los decimales periódicos (SC-004).
- **De integración** en `metrics/delivery`: cada escenario de la spec como prueba nombrada, con un
  router que compone las rutas de la 002, la 003, la 004, la 006 y la 007 para preparar los datos por
  sus propios endpoints, y el reloj inyectado para mover `now` más allá del plazo de gracia.
- **Concurrencia** (SC-019): 20 consultas de métricas del proyecto mientras otras goroutines cargan
  horas y completan historias; en cada respuesta se verifica completados ≤ planificados y
  desviación = reales − estimadas.
- Las pruebas que dependen de la tabla `defects` usan los endpoints de la 007.

### Rationale

Las fórmulas son lo que la cátedra evalúa con pruebas unitarias; los escenarios Given/When/Then son
pruebas de integración de punta a punta, como en las features anteriores.

### Alternativas consideradas

- **Preparar los datos con inserciones directas**: más rápido, pero no ejercitaría la integración real
  con la instantánea de cierre y con el anclaje del esfuerzo. Se reserva para forzar los datos rotos de
  la sección 13 (SC-020), que ninguna API permite crear.

---

## 16. Frontend: fuera de esta feature (RC-10)

### Decisión

Esta feature no agrega páginas, componentes ni tipos al frontend. La presentación de las métricas, sus
estados de valor y sus advertencias es de la 009, y su exportación de la 010.

### Rationale

RC-10: "Esta feature entrega valores, no vistas". Agregar tipos TypeScript sin un componente que los
use sería código especulativo (Principio I); los creará la 009 a partir de este contrato.

### Alternativas consideradas

- **Una página mínima de métricas**: duplicaría el trabajo del dashboard.
