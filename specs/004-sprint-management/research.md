# Investigación (Fase 0): Gestión de Sprints

**Feature**: `specs/004-sprint-management` | **Fecha**: 2026-10-21

Este documento resuelve las incógnitas técnicas del plan. La spec no tiene preguntas abiertas: las
decisiones de producto se cerraron en la sesión de clarificación del 2026-09-25 y en la enmienda del
2026-10-02 (sección Clarifications de [spec.md](./spec.md)). Cada sección registra la decisión, su
fundamento y las alternativas descartadas.

---

## 0. Contexto: esta feature se construye sobre la 001, la 002 y la 003

- Las features `specs/001-user-auth` y `specs/002-project-members` están implementadas e integradas
  en `main` (último merge: PR #287, fase 10 de la 002).
- La feature `specs/003-product-backlog` tiene **plan y tareas** en la rama `repo/003-plan`, pero
  **todavía no está implementada ni integrada**. Este plan se apoya en el diseño de ese plan: el
  módulo `internal/backlog/`, la tabla `user_stories`, los tipos `StoryStatus` y `StoryPriority`, la
  función `Readiness`, el middleware `RequireStoryAccess` y el puerto `SprintAssignments`. Si la
  implementación de la 003 se aparta de su plan, los nombres de este documento se ajustan al
  implementar; las decisiones no cambian.

Esta feature reutiliza sin cambios:

- El módulo de Go `github.com/juanku21/seguimiento-y-medicion/backend`, Gin, GORM y los contenedores
  de PostgreSQL de desarrollo (`smye_dev`, puerto 5432) y de test (`smye_test`, puerto 5433).
- El middleware de sesión de la 001, inyectado como `gin.HandlerFunc` desde `cmd/app/main.go`, y el
  accesor `authdelivery.CurrentUserID`.
- El middleware `RequireMembership` de la 002 a través del subgrupo `/api/v1/projects/:projectId`.
- El middleware `RequireStoryAccess` de la 003, inyectado como `gin.HandlerFunc` (sección 2).
- Los esquemas de error `ErrorResponse`, `FieldError` y `ValidationErrorResponse` de `auth/delivery`.
- El límite global de 1 MiB del cuerpo (`httpserver.LimitRequestBody`).
- La zona horaria única del sistema (`APP_TIMEZONE`, `config.Config.Location`) y el patrón de reloj
  inyectado (`now func() time.Time`) que la 002 usa en `StatusService`.
- El ayudante `testsupport.OpenDatabase`, con su lock de la base de test.

**Prerrequisito de implementación**: la 003 integrada en `main`. Esta feature **no tiene fase de
arranque** propia: no agrega dependencias, servicios ni variables de entorno.

---

## 1. Versiones del stack: sin cambios

**Decisión**: se mantienen las versiones fijadas por la 001, la 002 y la 003 (Go 1.27.2, Gin
v1.12.0, GORM v1.31.2 con `gorm.io/driver/postgres` v1.6.3, `google/uuid` v1.6.0, `testify`
v1.12.1, Next.js 16.3.8, React 19.3.0, Tailwind CSS 4.3.3, Vitest 5.0.3). **Esta feature no agrega
ninguna dependencia de Go ni de npm.**

**Rationale**: validación con la biblioteca estándar; transacciones, bloqueos de fila
(`gorm.io/gorm/clause`) e índices parciales (opción `where` de las etiquetas de índice de GORM) con
GORM; aritmética de calendario con `time`.

**Alternativas consideradas**: restricción `EXCLUDE` de PostgreSQL con `daterange` para impedir la
superposición — descartada: exige la extensión `btree_gist` para combinar la igualdad de `uuid` con
el solapamiento de rangos, y SQL en crudo fuera de `AutoMigrate`.

---

## 2. Un módulo `sprints` y la forma de las rutas (FR-056 a FR-058)

### Decisión

Se agrega **un** módulo vertical, `backend/internal/sprints/`, con sus cuatro capas. Expone once
endpoints con tres formas de ruta, siguiendo la misma regla que la 003: el recurso de la ruta es el
que define la respuesta de "no acceso".

| Endpoint | Historia | Recurso de la ruta | Middleware de acceso |
| --- | --- | --- | --- |
| `POST /api/v1/projects/{projectId}/sprints` | US1 | el proyecto | `RequireMembership` (002) |
| `GET /api/v1/projects/{projectId}/sprints` | US6 | el proyecto | `RequireMembership` (002) |
| `GET /api/v1/sprints/{sprintId}` | US6 | el sprint | `RequireSprintAccess` (nuevo) |
| `PUT /api/v1/sprints/{sprintId}` | US7 | el sprint | `RequireSprintAccess` |
| `DELETE /api/v1/sprints/{sprintId}` | US7 | el sprint | `RequireSprintAccess` |
| `POST /api/v1/sprints/{sprintId}/stories` | US2 | el sprint | `RequireSprintAccess` |
| `DELETE /api/v1/sprints/{sprintId}/stories` | US2 | el sprint | `RequireSprintAccess` |
| `POST /api/v1/sprints/{sprintId}/start` | US3 | el sprint | `RequireSprintAccess` |
| `POST /api/v1/sprints/{sprintId}/close` | US5 | el sprint | `RequireSprintAccess` |
| `PUT /api/v1/stories/{storyId}/status` | US4 | la historia | `RequireStoryAccess` (003) |
| `GET /api/v1/stories/{storyId}/sprints` | US6 | la historia | `RequireStoryAccess` (003) |

(El listado de sprints cerrados es el mismo `GET` de la colección con `?status=closed`; sección 10.)

### Rationale

- **Tres respuestas de "no acceso" distintas en la spec**: "proyecto inexistente" al crear y listar
  (tabla de Entradas y Salidas, US1-11, US6-12), "sprint inexistente" al operar un sprint (tablas de
  modificación y eliminación, US6-13, US7-15) e "historia inexistente" al asignar una historia ajena
  (US2-12). Cada forma de ruta tiene **un** middleware con **una** salida `404`, igual que la 003.
- **El avance se registra sobre la historia, no sobre el sprint**. La entrada de la spec es
  "identificador de una historia del sprint Activo y el estado destino", y US4-7 exige rechazar con
  un mensaje propio el avance sobre una historia del backlog **sin** sprint: con la ruta anidada bajo
  un sprint, ese caso no tendría ruta posible. El sprint se deduce del compromiso abierto de la
  historia.
- **Las dos rutas de historia reutilizan `RequireStoryAccess` de la 003**, que `cmd/app/main.go`
  pasa como `gin.HandlerFunc`, igual que hoy pasa el middleware de sesión. Así la respuesta de
  historia ajena es idéntica por construcción a la de la 003 y `sprints/delivery` no importa
  `backlog/delivery`. Gin admite que dos módulos registren rutas bajo `/api/v1/stories/:storyId`
  porque el nombre del parámetro coincide.
- **Quitar historias en lote con `DELETE` y parámetros de consulta repetidos**
  (`?storyId=a&storyId=b`): un `DELETE` con cuerpo no tiene semántica definida y algunos
  intermediarios lo descartan; los parámetros repetidos ya son la forma de la 003 para los filtros.
- **Iniciar y cerrar como acciones `POST`**: son transiciones con efectos (congelar Story Points,
  devolver historias al backlog), no reemplazos de una representación. La 002 separó el cambio de
  estado del proyecto en su propio endpoint por el mismo motivo.

### Alternativas consideradas

- **Todo anidado bajo `/api/v1/projects/{projectId}/sprints/{sprintId}`**: descartada; quien no es
  integrante recibiría el `404` de proyecto y no el de sprint que pide la spec.
- **Un único `PATCH /sprints/{id}` con `status`** para iniciar y cerrar: descartada; mezcla en un
  endpoint la corrección de datos (solo en Planificado) con dos transiciones que tienen reglas
  propias, y obliga a rechazar combinaciones de campos.
- **`PUT /sprints/{sprintId}/stories/{storyId}/status`** para el avance: descartada por US4-7.
- **Reimplementar en `sprints` una consulta de acceso a la historia**: descartada (DRY) y con riesgo
  de que los dos `404` dejen de ser idénticos.

---

## 3. Acceso a un sprint: una consulta y un `404` (FR-057, FR-058, SC-014)

**Decisión**: `RequireSprintAccess` resuelve con **una consulta** la existencia del sprint, la
membresía vigente del titular de la sesión en su proyecto y el estado del proyecto:

```text
sprints JOIN project_members ON project_id = sprints.project_id AND user_id = <titular>
        JOIN projects        ON projects.id = sprints.project_id
WHERE sprints.id = <sprintId>
```

Sin fila, o con un identificador que no es UUID, responde siempre el mismo `404` (`El sprint no
existe.`). Con fila, deja en el contexto un `SprintAccess` con el identificador del sprint, el de su
proyecto y si el proyecto está Finalizado.

**Rationale**: es la técnica de `RequireMembership` (002) y de `RequireStoryAccess` (003). Un sprint
eliminado (US7-15) no tiene fila y cae en la misma salida.

**Alternativas consideradas**: `RequireMembership` con el proyecto leído desde el sprint — descartada,
dos consultas y dos posibles cuerpos `404`.

---

## 4. Fechas, superposición y validación (FR-002 a FR-005, FR-013, SC-004)

### Decisión

- `start_date` y `end_date` son columnas `date`, como las del proyecto en la 002. En la API viajan
  como `YYYY-MM-DD` (`time.DateOnly`) y se representan en Go como medianoche UTC.
- Validación en `delivery`, antes del servicio, devolviendo **todos** los campos inválidos a la vez:
  nombre recortado de 1 a 100 runas; Sprint Goal recortado de 1 a 500 runas; ambas fechas
  obligatorias y con formato válido; fin **estrictamente** posterior al inicio.
- **Superposición**: dos períodos `[s1, e1]` y `[s2, e2]` del mismo proyecto se superponen cuando
  `s1 < e2 AND s2 < e1`. Con desigualdades estrictas, compartir solo el día de borde (`e1 = s2`) no
  es superposición (FR-005, US1-4, US7-5), y la contención total o el período idéntico sí lo son
  (SC-004). Se compara contra todos los sprints del proyecto, **cualquiera sea su estado**, y en la
  modificación se excluye al propio sprint (FR-013).
- La regla vive en una función de dominio pura, `PeriodsOverlap(a, b Period) bool`, con prueba
  unitaria; el repositorio la traduce a la condición `start_date < ? AND ? < end_date`.
- El rechazo indica **con qué sprint** se superpone (identificador y nombre). Si hay varios, se
  informa el de fecha de inicio más temprana, para que el mensaje sea determinista.

### Concurrencia

Dos creaciones simultáneas en el mismo proyecto podrían pasar ambas la verificación. La creación y la
modificación toman primero un **bloqueo de fila sobre el proyecto** (`SELECT ... FROM projects WHERE
id = ? FOR UPDATE`) y verifican la superposición dentro de esa transacción: las escrituras de
calendario de un mismo proyecto quedan serializadas y la verificación ve siempre el último estado.

### Rationale

- El bloqueo del proyecto es la pieza más simple que hace verdadero SC-004 bajo concurrencia sin
  extensiones de PostgreSQL. Su costo es serializar las escrituras de sprints de **un** proyecto, que
  son acciones humanas poco frecuentes.
- Las desigualdades estrictas expresan literalmente la definición de FR-005 ("comparten al menos un
  día que no sea el día de borde").

### Alternativas consideradas

- **`EXCLUDE USING gist`**: descartada (sección 1).
- **Nivel de aislamiento `SERIALIZABLE`**: descartada; obliga a reintentar transacciones ante fallos
  de serialización, lógica que el resto del proyecto no tiene.

---

## 5. Un solo sprint Activo por proyecto (FR-027, SC-005)

**Decisión**: el inicio toma el mismo bloqueo de fila del proyecto (sección 4) y, dentro de la
transacción, verifica que no exista otro sprint `active` en el proyecto. Como **respaldo**, un índice
único parcial `idx_sprints_one_active_per_project` sobre `sprints(project_id) WHERE status =
'active'`, declarado con la opción `where` de la etiqueta de índice de GORM. Si el índice llegara a
rechazar una escritura, el repositorio traduce `gorm.ErrDuplicatedKey` al mismo `409` de "ya hay un
sprint activo".

**Rationale**: SC-005 exige cero proyectos con dos sprints activos con 10 inicios simultáneos. El
bloqueo da el mensaje correcto (cuál es el sprint activo) en el caso normal; el índice garantiza la
invariante aunque un camino futuro olvide el bloqueo. Es el mismo criterio de "regla en el servicio,
restricción en la base como respaldo" que la 003 dejó anotado para las claves foráneas.

**Prueba**: diez goroutines inician diez sprints Planificados distintos del mismo proyecto contra la
base de test; exactamente una obtiene `200` y nueve `409` (`sprint_concurrency_test.go`).

---

## 6. Compromiso de historias: una tabla para el plan vivo y el registro congelado (FR-017 a FR-025, FR-042, FR-055)

### Decisión

Una tabla `sprint_stories` con una fila por historia comprometida en un sprint. La misma fila sirve
en los dos momentos de la vida del sprint:

- **Mientras el sprint está abierto** (Planificado o Activo): `sprint_id`, `story_id`,
  `committed_at` y `completed_at` (la fecha y hora de completado dentro de ese sprint, `NULL` si no
  está Completada). Los campos congelados están en `NULL`.
- **Al cerrar el sprint**: se completan en la misma transacción `title_at_close`,
  `priority_at_close`, `story_points_at_close` y `outcome` (`completed` o `not_completed`). Desde ese
  momento la fila no se modifica nunca más.

Quitar una historia **borra** su fila (la historia quitada no figura en el sprint, supuesto "Las
historias quitadas antes del cierre no figuran en el sprint"). El historial de una historia (FR-055)
son todas sus filas: las congeladas de sprints cerrados y, si la tiene, la abierta.

**Índice único parcial** `idx_sprint_stories_one_open_per_story` sobre `sprint_stories(story_id)
WHERE outcome IS NULL`: una historia tiene como máximo un compromiso abierto (FR-020).

### Rationale

- La spec define el **Compromiso de historia en el sprint** como entidad con sprint, historia,
  momento de compromiso y fecha de completado, y dice que "es lo que permite que el historial de una
  historia muestre todos los sprints por los que pasó". Una sola tabla con la misma fila antes y
  después del cierre es la representación directa de esa entidad.
- Separar la fila viva de la congelada en dos tablas obligaría a copiar sprint, historia y fecha de
  completado al cerrar, y a unir dos tablas para el historial de una historia.
- `outcome IS NULL` coincide exactamente con "compromiso en un sprint abierto", porque solo el cierre
  completa `outcome`. Así el índice parcial no depende del estado del sprint, que vive en otra tabla.

### Alternativas consideradas

- **Columna `sprint_id` en `user_stories`**: descartada; pierde el historial (RN-17, SC-015) y no
  tiene dónde guardar la fecha de completado por sprint.
- **Tabla de compromisos más tabla de historias de la instantánea**: descartada por lo anterior.
- **Borrado lógico de la fila al quitar la historia**: descartada; la spec dice que la historia
  quitada no figura en el sprint, y la marca permanente de "estuvo en un sprint" se resuelve aparte
  (sección 7).

---

## 7. "Estuvo alguna vez en un sprint": columna `ever_committed` en `user_stories` (RC-06, US7-7)

### El problema

La 003 impide eliminar una historia que **alguna vez** estuvo asignada a un sprint, aunque se la
haya quitado o el sprint se haya eliminado (RC-06 de esta spec; FR-041 de la 003). Pero quitar una
historia borra su fila de `sprint_stories` (sección 6) y eliminar un sprint Planificado borra las
filas de ese sprint (FR-048): ninguna de las dos tablas de esta feature conserva ese hecho.

### Decisión

Se agrega a `user_stories` la columna `ever_committed boolean NOT NULL DEFAULT false`. La escribe
**solo** esta feature, en la misma transacción que crea el compromiso, y nunca vuelve a `false`. El
puerto `SprintAssignments` de la 003 la lee para `EverAssigned`.

### Rationale

- Es el único dato que sobrevive a la baja de la historia y a la eliminación del sprint, que son
  exactamente los dos casos de US7-7 y de US6-4 de la 003.
- La 003 ya establece que la feature de sprints es la única que escribe `user_stories.status`
  (su FR-027 y su research, sección 11). Escribir una segunda columna desde el mismo repositorio no
  agrega un nuevo punto de acoplamiento.
- Un booleano es la representación más simple de un hecho que solo se consulta como sí o no.

### Costo

Es un cambio en un archivo de la 003: el campo `EverCommitted` del modelo
`backlogdomain.UserStory`, para que `AutoMigrate` cree la columna. `AutoMigrate` la agrega con su
valor por defecto sobre una tabla con datos.

### Alternativas consideradas

- **Tabla propia de "marcas" de historias**: descartada; una tabla con clave primaria, marcas de
  tiempo y clave foránea para guardar un booleano por historia.
- **Conservar las filas de compromisos quitados con `removed_at`**: descartada; no resuelve la
  eliminación del sprint, cuyas filas tienen que desaparecer con él.
- **Impedir eliminar sprints con historias**: contradice FR-048 y US7-2.

---

## 8. Admisión de historias y atomicidad del lote (FR-017 a FR-020, FR-023, RN-08, SC-007)

### Decisión

`POST /api/v1/sprints/{sprintId}/stories` recibe `{"storyIds": ["...", "..."]}` (entre 1 y 100
identificadores distintos). En una transacción:

1. Bloquea la fila del sprint (`FOR UPDATE`) y verifica: proyecto no Finalizado, sprint no Cerrado.
2. Bloquea las filas de las historias pedidas (`FOR UPDATE`, ordenadas por `id` para no provocar
   interbloqueos entre dos lotes que comparten historias).
3. Si alguna no existe o es de otro proyecto, responde `404` de historia inexistente (US2-12).
4. Evalúa **cada** historia con las tres condiciones de admisión y acumula **todos** los motivos:
   `not_ready` (con lo que le falta, calculado con `Readiness` de la 003), `completed` y
   `committed_elsewhere` (con el sprint en el que está).
5. Si hay al menos un motivo, responde `409` con la lista y no escribe nada (FR-023, US2-11).
6. Si no, inserta una fila de `sprint_stories` por historia y marca `ever_committed = true`.

Una historia que **ya** está comprometida en **este mismo** sprint se rechaza también como
`committed_elsewhere` con el sprint actual: reasignarla no tiene efecto y aceptarla en silencio
ocultaría un error del cliente.

**Rationale**:

- `Readiness` de la 003 es la única fuente de la regla "lista para planificar" (research de la 003,
  sección 10); reutilizarla cumple DRY.
- Bloquear las historias impide que dos lotes simultáneos comprometan la misma historia en dos
  sprints. El índice parcial de la sección 6 es el respaldo.
- Informar todos los motivos de todas las historias evita que la persona corrija una y tropiece con
  la siguiente, igual que la eliminación de la 003 lista todos sus motivos.
- El límite de 100 identificadores no es un requisito de la spec sino una cota de entrada (Principio
  VII); con el límite de 1 MiB del cuerpo ya existe una cota, y 100 supera cualquier sprint real.

**Alternativas consideradas**: aceptar las válidas y rechazar las inválidas — prohibido por FR-023.

---

## 9. Restricción sobre historias comprometidas (FR-018, RN-22, RC-05)

**Decisión**: esta feature **no** agrega código a `backlog/service`. La 003 ya consulta el puerto
`SprintAssignments.Find` antes de estimar con `null` o de vaciar los criterios (su data-model,
sección 5). Esta feature reemplaza el cuerpo de `internal/backlog/repository/sprint_assignments.go`:

```text
CommittedToOpenSprint = EXISTS (sprint_stories WHERE story_id = ? AND outcome IS NULL)
EverAssigned          = user_stories.ever_committed
```

Con eso, US2-14 y US2-4 pasan a ser verdaderos con datos reales.

**Señalado al desarrollador — ventana de carrera**: la 003 verifica el puerto **antes** de tomar su
bloqueo de fila (su plan, punto 6). Si una asignación (sección 8) se confirma entre esa verificación
y la escritura de la 003, una historia podría quedar comprometida sin Story Points o sin criterios.
La asignación de esta feature sí bloquea la historia y relee su estado, así que el único camino
abierto es el de la 003. Se resuelve moviendo la consulta del puerto **dentro** de la transacción que
ya bloquea la historia en `backlog/repository` (la estimación necesitaría tomar el mismo bloqueo). No
se incluye en este plan porque es un cambio de diseño de la 003; conviene decidirlo al implementar
la 003 o revisarlo con `/revisar-seguridad`.

---

## 10. Listado, historial de cerrados y orden (FR-050 a FR-052, FR-054)

### Decisión

- `GET /api/v1/projects/{projectId}/sprints` devuelve todos los sprints del proyecto. Admite el
  filtro repetible `?status=planned|active|closed`; `?status=closed` es el **historial de sprints
  cerrados** de FR-051.
- **Un solo esquema de ítem** para los dos usos: nombre, estado, Sprint Goal, fechas previstas,
  momentos reales de inicio y cierre (o `null`), cantidad de historias comprometidas y un objeto
  `results` que es `null` salvo en los sprints Cerrados, con los tres totales de Story Points y las
  cantidades de historias planificadas y completadas, tomados de la instantánea.
- **Orden**: `start_date DESC, id ASC` (FR-052).
- **Una consulta** con subconsultas agregadas sobre `sprint_stories` por sprint: cantidad de filas,
  cantidad con `outcome = 'completed'`, `SUM(story_points_at_close)` y la suma de las completadas.
  Sin N+1.
- **Vacío**: lista vacía con `emptyReason`: `no_sprints` si el proyecto no tiene sprints y
  `no_matches` si los tiene pero ninguno cumple el filtro; `null` si hay resultados (FR-054, US6-10).
  La segunda consulta de existencia solo se hace cuando el resultado filtrado viene vacío, igual que
  la 003.

### Rationale

- La spec pide el historial como una lista de sprints cerrados con más datos. Un filtro sobre el
  mismo recurso evita un segundo endpoint con la misma consulta y el mismo orden; el objeto
  `results` nulo hace explícito que los totales solo existen tras el cierre (FR-006).
- La cantidad de historias del listado (FR-050) y las historias planificadas del historial (FR-051)
  salen del mismo conteo: en un sprint abierto son sus compromisos vigentes y en uno cerrado son sus
  filas congeladas.

**Alternativas consideradas**: endpoint `/sprints/history` separado — descartada por duplicar la
consulta y el orden.

---

## 11. Instantánea de cierre: qué se guarda y qué se deriva (FR-028, FR-043 a FR-045, SC-008, SC-009)

### Decisión

| Dato de la instantánea | Dónde vive | Cuándo se fija |
| --- | --- | --- |
| Story Points comprometidos al inicio | `sprints.committed_story_points` | Al iniciar (FR-028) |
| Fecha de inicio y de fin previstas | `sprints.start_date`, `sprints.end_date` | Inmutables desde el inicio (FR-016) |
| Momento real de inicio | `sprints.started_at` | Al iniciar |
| Momento real de cierre y quién cerró | `sprints.closed_at`, `sprints.closed_by_id` | Al cerrar (FR-040) |
| Factor de horas por Story Point vigente | `sprints.hours_per_story_point_hundredths` | Al cerrar, copiado de `projects` |
| Historias involucradas con título, prioridad, Story Points y resultado | `sprint_stories` (campos `*_at_close` y `outcome`) | Al cerrar |
| Story Points planificados al cierre | `SUM(story_points_at_close)` | Derivado de filas inmutables |
| Story Points completados | `SUM(story_points_at_close) WHERE outcome = 'completed'` | Derivado de filas inmutables |
| Historias planificadas y completadas | `COUNT(*)` y `COUNT(*) WHERE outcome = 'completed'` | Derivado de filas inmutables |

El factor se guarda en **centésimas de hora**, igual que `projects.hours_per_story_point_hundredths`
en la 002, para no redondear.

### Rationale

- Lo que no se puede reconstruir después (los Story Points al inicio, el factor del proyecto, los
  datos de cada historia al cerrar) se **copia**. Lo que es una suma de esas copias se **deriva**: las
  filas congeladas no cambian nunca, así que la suma tampoco, y no hay dos fuentes que puedan
  contradecirse. Es el mismo criterio con el que la 003 deriva "lista para planificar".
- Las cuatro fechas no se duplican en otra tabla porque la fila del sprint ya es inmutable desde que
  deja de estar Planificado (FR-016, FR-046): la instantánea "congela las cuatro" sin copiarlas.
- Un sprint vacío cierra con sumas en cero (`COALESCE`, FR-045, US5-6).

### Alternativas consideradas

- **Tabla `sprint_snapshots` 1:1 con todos los totales**: descartada; duplica datos derivables y
  agrega una tabla que siempre se lee junto con el sprint.
- **Leer el factor del proyecto al consultar**: prohibido por FR-044 y US5-4.

---

## 12. Transiciones y operaciones permitidas por estado (FR-016, FR-024, FR-032, FR-046, FR-047)

**Decisión**: el tipo `SprintStatus` (`planned`, `active`, `closed`) declara en dominio
`ValidateTransition(to)`, que solo acepta `planned → active` y `active → closed`, igual que
`ProjectStatus` en la 002. Las demás operaciones se deciden con una tabla de dominio:

| Operación | `planned` | `active` | `closed` |
| --- | --- | --- | --- |
| Modificar datos | Sí | `409` solo Planificado | `409` cerrado definitivo |
| Eliminar | Sí, sin esfuerzo | `409` nunca iniciado | `409` nunca iniciado |
| Asignar / quitar historias | Sí | Sí | `409` cerrado no admite cambios |
| Iniciar | Sí | `409` transición inválida | `409` cerrado definitivo |
| Registrar avance de sus historias | `409` solo en Activo | Sí | `409` cerrado no admite cambios |
| Cerrar | `409` solo se cierra un Activo | Sí | `409` cerrado definitivo |

Cada celda de rechazo tiene su error de dominio y su mensaje (data-model, sección 9). SC-006 se
verifica recorriendo la matriz completa de origen y destino.

---

## 13. Avance de historias (FR-033 a FR-038, SC-011)

### Decisión

`PUT /api/v1/stories/{storyId}/status` recibe `{"status": "in_progress" | "completed"}`.

- Transiciones admitidas, declaradas en `sprints/domain` como función pura
  `ValidateStoryProgress(from, to backlogdomain.StoryStatus) error`: `pending → in_progress`,
  `in_progress → completed` y `completed → in_progress`. Cualquier otra, incluido el salto
  `pending → completed` y el destino `pending`, se rechaza con `409` enumerando las permitidas.
- En una transacción: se bloquea la historia, se busca su compromiso abierto con el sprint, y se
  toma un bloqueo compartido sobre la fila del sprint (`FOR SHARE`) para que un cierre simultáneo no
  se intercale.
- Sin compromiso abierto: si la historia está Completada y su último compromiso es de un sprint
  Cerrado, `409` "un sprint cerrado no admite cambios" (US4-8); si no, `409` "solo se registra avance
  sobre historias del sprint activo" (US4-7). Con compromiso en un sprint Planificado, el mismo
  `409` de sprint activo (US4-6).
- Al completar: `user_stories.status = completed` y `sprint_stories.completed_at = now`. Al
  desmarcar: `status = in_progress` y `completed_at = NULL`. Un nuevo completado registra el último
  momento (FR-035).
- El estado destino fuera de `pending`, `in_progress` y `completed` es un `400` de validación.

**Rationale**: la fecha de completado es un dato **del compromiso** ("fecha y hora en que se completó
dentro de ese sprint", entidad Compromiso), por eso vive en `sprint_stories` y no en
`user_stories`, cuyo modelo pertenece a la 003. El bloqueo compartido del sprint deja que varios
integrantes registren avance a la vez y solo se serializa contra el cierre, que toma el exclusivo.

---

## 14. Cierre (FR-039 a FR-045, SC-008 a SC-010)

**Decisión**: una transacción que:

1. Bloquea el sprint (`FOR UPDATE`); rechaza si el proyecto está Finalizado o si el sprint no está
   `active`.
2. Lee y bloquea las historias comprometidas (`FOR UPDATE`, ordenadas por `id`).
3. Para cada fila de `sprint_stories`: completa `title_at_close`, `priority_at_close`,
   `story_points_at_close` y `outcome` según el estado de la historia.
4. Devuelve al backlog las no Completadas: `status = pending` (FR-041). Las Completadas quedan como
   están.
5. Copia el factor del proyecto, fija `closed_at = now`, `closed_by_id` y `status = closed`.

Responde `200` con el detalle del sprint cerrado, que ya incluye la instantánea.

**Rationale**: todo el cierre es atómico; si algo falla, el sprint sigue Activo y ninguna historia se
movió. El cierre anticipado o tardío no se compara con la fecha de fin (FR-040, US5-7, US5-8).

---

## 15. Días restantes del sprint Activo (FR-053, RN-23, RC-11)

**Decisión**: el servicio calcula, solo para un sprint `active`:

```text
today     = fecha de calendario de now() en APP_TIMEZONE
remaining = días(end_date − today)
si remaining ≥ 0 → remainingDays = remaining, overdueDays = null
si remaining < 0 → remainingDays = null,      overdueDays = −remaining
```

En el detalle viajan como `schedule: {"remainingDays": 10, "overdueDays": null}`; `schedule` es
`null` para un sprint Planificado o Cerrado. La función es pura, recibe la fecha de fin, el instante y
la zona, y tiene prueba unitaria con los tres casos de la spec (10, 0 y vencido hace 3) más el borde
de medianoche en la zona del sistema.

**Rationale**: la spec define el conteo con el día de fin incluido ("el último día del sprint vale
0") y exige no devolver negativos. Dos campos mutuamente excluyentes expresan "vencido hace N" sin un
número negativo ni un texto que el dashboard (009) tenga que interpretar. El reloj y la zona se
inyectan como en `StatusService` de la 002, lo que hace los escenarios US6-4 a US6-6 reproducibles.

**Alternativas consideradas**: un único entero con signo — prohibido por RN-23; un texto ya
formateado — descartado, mezcla presentación en la API.

---

## 16. Eliminación de un sprint Planificado (FR-048, RN-20, RC-10, SC-016)

**Decisión**:

- El servicio verifica proyecto no Finalizado y sprint `planned`. Un sprint `planned` nunca fue
  iniciado, porque no hay transición de vuelta (sección 12).
- Consulta un puerto propio, `SprintEffort.HasEffort(ctx, sprintID) (bool, error)`, que implementará
  `specs/006-effort-tracking`. Hasta entonces su implementación en `sprints/repository/` informa lo
  que hoy es cierto: ningún sprint tiene esfuerzo registrado.
- Si hay esfuerzo, `409` indicando que primero hay que eliminar esos registros.
- Si no, en una transacción: devuelve las historias comprometidas a `pending` (ya lo están, porque un
  sprint Planificado no admite avance; se escribe igual para que la regla sea explícita), borra las
  filas de `sprint_stories` y borra el sprint. `ever_committed` queda en `true` (US7-7). Responde
  `204`.

**Señalado para la 006**: su tabla de registros de esfuerzo tendrá una clave foránea hacia
`sprints.id` con la restricción por defecto (`RESTRICT`). Es el respaldo de esta regla: si un
registro llegara entre la verificación y el borrado, la base rechazaría el borrado, y el repositorio
debe traducir `gorm.ErrForeignKeyViolated` al mismo `409`. Es el mismo patrón que la 003 dejó
anotado para sus eliminaciones.

**Rationale**: un puerto por feature productora, igual que la 002 y la 003; sin él US7-8 y US7-14 no
se podrían probar hoy. Con un doble de `testify/mock` se prueban con valores reales.

---

## 17. Integración con la 002 y la 003

### 17.1 Puerto `SprintDirectory` de la 002

Esta feature reemplaza el cuerpo de `internal/projects/repository/sprint_directory.go`, tal como
dejó anotado la 002: `FindActiveSprint` lee `sprints` con `status = 'active'` y devuelve
identificador y nombre, o `(nil, nil)`. El constructor pasa a recibir la conexión, así que cambian
`cmd/app/main.go` y los ayudantes de prueba de `projects/delivery`. `projects/service` no cambia.
Con eso la vista de estado del proyecto muestra el sprint activo real y la regla "no se finaliza un
proyecto con sprint activo" (RC-04) pasa a ser efectiva.

**Señalado al desarrollador — ventana de carrera con la finalización del proyecto**:
`StatusService.ChangeStatus` de la 002 consulta el sprint activo y después escribe el estado, sin
bloqueo. Si un sprint se inicia entre esas dos operaciones, el proyecto podría quedar Finalizado con
un sprint Activo. El inicio de esta feature sí toma el bloqueo del proyecto (sección 5); el cierre de
la ventana del lado de la 002 sería que `UpdateStatus` tome el mismo bloqueo y repita la consulta
dentro de la transacción. No se incluye en este plan porque modifica el servicio de la 002; se
recomienda tratarlo en `/revisar-seguridad` de esta feature.

### 17.2 Puerto `SprintAssignments` de la 003

Se reemplaza el cuerpo de `internal/backlog/repository/sprint_assignments.go` (sección 9).

### 17.3 Modelo `UserStory` de la 003

Se agrega el campo `EverCommitted` (sección 7).

### 17.4 Frontend de la 003

El detalle de la historia agrega la sección "Sprints en los que estuvo" con los datos de
`GET /api/v1/stories/{storyId}/sprints` (FR-055).

### 17.5 Dirección de las importaciones

```text
sprints/domain     → backlog/domain, projects/domain, auth/domain
sprints/repository → sprints/domain, backlog/domain, projects/domain
backlog/repository → sprints/domain   (solo sprint_assignments.go)
projects/repository → sprints/domain  (solo sprint_directory.go)
```

Ningún paquete de `domain` importa `sprints`, así que no hay ciclos.

---

## 18. Solo lectura del proyecto Finalizado (FR-059, RN-19, SC-012)

**Decisión**: en las rutas de sprint, el estado del proyecto llega con `SprintAccess`; en la
creación lo lee el repositorio dentro de la transacción que bloquea el proyecto; en el avance lo lee
la transacción que bloquea la historia. Toda escritura responde `409` (`El proyecto está finalizado y
sus sprints son de solo lectura.`), evaluado **antes** que cualquier otra regla de estado. Listado,
detalle e historial de una historia no lo consultan.

**Rationale**: el mismo criterio y el mismo código `409` que la 002 y la 003.

---

## 19. Autoría y atribución (FR-009, FR-040, FR-060)

**Decisión**:

- `sprints.created_by_id` guarda al autor y `sprints.closed_by_id` a quien cerró (FR-040 lo pide
  explícitamente). El detalle devuelve `createdBy` y `closedBy` con identificador y nombre completo,
  leídos con un `JOIN` que selecciona solo `users.id` y `users.full_name`.
- La identidad de quien ejecuta cada acción sale siempre de la sesión, nunca del cuerpo.
- No se persisten "último modificador", "iniciado por" ni "completado por": el historial de
  auditoría está fuera de alcance, y es la misma lectura de "atribuida al titular de la sesión" que
  hicieron la 002 (FR-051) y la 003 (FR-049).

**Señalado al desarrollador**: si el equipo entiende que FR-060 pide persistir quién inició el
sprint o quién completó cada historia, se agregan `started_by_id` en `sprints` y `completed_by_id`
en `sprint_stories`. No se incluyen porque ningún escenario ni criterio de éxito los verifica.

---

## 20. Códigos de estado y forma de los errores

**Decisión**: se reutilizan los esquemas de error de la 001 y se fija esta correspondencia:

| Situación | Código | Requisito |
| --- | --- | --- |
| Entrada inválida (nombre, Sprint Goal, fechas, lista de historias, estado destino, filtro) | `400` | FR-002 a FR-004, FR-011 |
| Sin sesión válida | `401` | FR-056 |
| Proyecto inexistente o ajeno (crear y listar) | `404` | FR-058 |
| Sprint inexistente, eliminado o ajeno | `404` | FR-058 |
| Historia inexistente, ajena o de otro proyecto | `404` | FR-058, US2-12 |
| Escritura sobre los sprints de un proyecto Finalizado | `409` | FR-059 |
| Período superpuesto (con el sprint en conflicto) | `409` | FR-005 |
| Operación no admitida por el estado del sprint (tabla de la sección 12) | `409` | FR-016, FR-024, FR-032, FR-046, FR-047 |
| Otro sprint Activo (con el sprint activo) | `409` | FR-027 |
| Lote con historias no admisibles (con los motivos por historia) | `409` | FR-018 a FR-020, FR-023 |
| Quitar una historia Completada | `409` | FR-025 |
| Avance fuera del sprint Activo o transición de historia no admitida | `409` | FR-036 a FR-038 |
| Eliminación con esfuerzo registrado | `409` | FR-048 |

**Precedencia**: `401` → `404` → `400` → `409`. Entre los `409`, la solo lectura del proyecto va
primero y después el estado del sprint, porque cualquiera de los dos vuelve irrelevantes las demás
reglas.

**Rationale**: igual que en la 002 y la 003, el `409` agrupa los conflictos con el estado actual de
un recurso; el `400` queda para entradas mal formadas.

---

## Resumen: incógnitas resueltas

| Incógnita | Resolución | Sección |
| --- | --- | --- |
| Dependencias nuevas | Ninguna | 1 |
| Organización y forma de las rutas | Un módulo `sprints`; colección en el proyecto, ítem plano de sprint, avance e historial sobre la historia | 2 |
| `404` indistinguible para sprints | `RequireSprintAccess` con una consulta y una salida | 3 |
| Superposición de fechas bajo concurrencia | Desigualdades estrictas y bloqueo de fila del proyecto | 4 |
| Un solo sprint Activo | Bloqueo del proyecto e índice único parcial de respaldo | 5 |
| Persistencia del compromiso y del registro congelado | `sprint_stories`, misma fila antes y después del cierre | 6 |
| "Estuvo alguna vez en un sprint" | Columna `ever_committed` en `user_stories` | 7 |
| Admisión y atomicidad del lote | Bloqueo de historias, `Readiness` de la 003, `409` con todos los motivos | 8 |
| Historia comprometida que deja de estar lista | Implementación real del puerto `SprintAssignments` de la 003 | 9 |
| Listado e historial de cerrados | Un endpoint con filtro, `results` nulo hasta el cierre | 10 |
| Instantánea | Se copia lo irrecuperable; se derivan las sumas | 11 |
| Transiciones y operaciones por estado | `SprintStatus.ValidateTransition` y tabla de dominio | 12 |
| Avance de historias | Endpoint sobre la historia; completado en el compromiso | 13 |
| Cierre | Una transacción con bloqueos | 14 |
| Días restantes | Función pura con reloj y zona inyectados; dos campos excluyentes | 15 |
| Eliminación con esfuerzo | Puerto `SprintEffort` hacia la 006 | 16 |
| Integración con la 002 y la 003 | Reemplazo de dos puertos y un campo nuevo en `UserStory` | 17 |
| Proyecto Finalizado | `409` antes que cualquier regla de estado | 18 |
| Atribución | Autor y quien cerró persistidos; el resto desde la sesión | 19 |
| Códigos HTTP | Tabla y precedencia | 20 |

### Puntos abiertos para el desarrollador (no bloquean el diseño)

1. **La 003 todavía no está implementada** (sección 0): este plan usa los nombres de su plan.
2. **Ventana de carrera de la 003** al quitar Story Points o criterios a una historia que se está
   comprometiendo (sección 9).
3. **Ventana de carrera de la 002** al finalizar un proyecto mientras se inicia un sprint (sección
   17.1).
4. **Lectura de FR-060** (sección 19): se persisten el autor y quien cerró; confirmar que no se pide
   guardar quién inició ni quién completó cada historia.
5. **Punto de integración para anotar al planificar la 006** (sección 16): implementar `SprintEffort`
   y traducir la violación de clave foránea al eliminar un sprint en el mismo `409`.
