# Investigación (Fase 0): Gestión de Proyectos e Integrantes

**Feature**: `specs/002-project-members` | **Fecha**: 2026-10-07

Decisiones técnicas que resuelven las incógnitas de la Fase 0 del [plan](./plan.md). Cada sección
indica la decisión, su justificación y las alternativas descartadas.

---

## 0. Contexto: esta feature se construye sobre la 001

RC-01 de la spec declara la dependencia de `specs/001-user-auth`. Hoy el repositorio contiene
`specs/`, `.specify/`, `.claude/`, `.github/`, `CLAUDE.md` y `README.md`: **no existen `backend/`
ni `frontend/`**, porque la feature 001 está planificada pero todavía no implementada.

### Prerrequisito bloqueante

| Prerrequisito | Estado | Acción requerida |
| --- | --- | --- |
| Feature 001 implementada e integrada en `main` | **Pendiente** | Completar `specs/001-user-auth/tasks.md` antes de la primera tarea de 002 |

De la 001 esta feature reutiliza, sin modificarlos:

- La tabla `users` y el correo normalizado, para buscar integrantes (FR-019, FR-020).
- El middleware de sesión y su `401` (FR-046), aplicado al grupo de rutas de esta feature.
- `internal/platform/config` (variables de entorno) e `internal/platform/database` (conexión GORM
  con `TranslateError` y `AutoMigrate`).
- El ayudante de pruebas que carga `backend/.env.test` y vacía las tablas antes de cada caso.

A diferencia de la 001, esta feature **no** incluye arranque de proyecto: no crea módulos de Go,
contenedores, Dockerfiles ni configuración de pruebas. Todo eso ya existe.

---

## 1. Versiones del stack: sin cambios

**Decisión**: se mantienen exactamente las versiones fijadas en
[`specs/001-user-auth/research.md`](../001-user-auth/research.md), sección 1. Esta feature **no
agrega ninguna dependencia** de Go ni de npm.

**Rationale**: el Principio IV fija el stack y obliga a pedir autorización para cualquier
dependencia fuera de la tabla. Las tres necesidades que podrían haber empujado una dependencia
nueva se resuelven con la biblioteca estándar o con lo ya autorizado:

| Necesidad | Resolución sin dependencia nueva |
| --- | --- |
| Aritmética decimal exacta del factor (FR-006, FR-045) | Enteros de centésimas (sección 3) |
| Zona horaria del sistema (FR-039) | `time.LoadLocation` más `time/tzdata`, ambos de la biblioteca estándar (sección 4) |
| Dobles de prueba para los puertos hacia backlog y sprints | `stretchr/testify`, ya autorizado (sección 9) |

**No se solicita ninguna autorización adicional.**

**Alternativas consideradas**: `github.com/shopspring/decimal` para el factor — descartada, no está
en la tabla de dependencias autorizadas del Principio IV y la sección 3 muestra que no hace falta.

---

## 2. Unicidad del nombre por propietario (FR-004, FR-011, SC-007)

### Decisión

La tabla `projects` tiene **dos columnas de nombre**:

- `name varchar(100)`: el nombre tal como la persona lo escribió, ya recortado en los extremos. Es
  el que se muestra en el listado y en la vista de estado.
- `name_normalized varchar(100)`: el mismo nombre en minúsculas y recortado. No se muestra nunca.

La unicidad se garantiza con un **índice único compuesto** `(owner_id, name_normalized)` y
traduciendo `gorm.ErrDuplicatedKey` al error de nombre repetido, igual que la 001 hace con el correo.

**Rationale**:

- FR-004 compara sin distinguir mayúsculas ni espacios extremos, pero el nombre **se muestra** y la
  persona espera leer "Portal de Clientes", no "portal de clientes". En la 001 el correo normalizado
  sí es el que se muestra (escenario US4-2 de esa spec), así que allí una sola columna alcanzaba;
  acá no. Las dos columnas no son duplicación ociosa: guardan dos datos con dos propósitos
  distintos, uno para mostrar y otro para comparar.
- El índice único compuesto resuelve FR-011 y SC-007 (10 creaciones simultáneas con el mismo
  nombre dejan un solo proyecto) sin condición de carrera. Un "consultar y después insertar" la
  tendría.
- FR-014 (excluir el propio proyecto al validar la unicidad en una modificación) y el escenario
  US5-3 (renombrar con el mismo nombre que ya tenía) salen gratis: un `UPDATE` que no cambia
  `name_normalized` no viola el índice.

**Alternativas consideradas**:

- **Índice único por expresión** `(owner_id, lower(trim(name)))`: descartada. `AutoMigrate` de GORM
  no crea índices por expresión de forma portable; habría que ejecutar un `CREATE UNIQUE INDEX` en
  crudo después de migrar, es decir una pieza móvil más para ahorrar una columna.
- **Una sola columna con el nombre normalizado**, como el correo en la 001: descartada, pierde la
  capitalización que la persona escribió.
- **Validar la unicidad con una consulta previa**: descartada por la condición de carrera que
  SC-007 mide explícitamente.

---

## 3. Factor de horas por Story Point: precisión exacta sin dependencias (FR-006, FR-045, RN-07)

### Decisión

El factor se **almacena y se calcula como un entero de centésimas de hora**, en la columna
`hours_per_story_point_hundredths integer NOT NULL`. El rango válido es `1` a `4000`, que es
exactamente "mayor que 0, menor o igual a 40, con hasta 2 decimales".

En la entrada HTTP, el campo `hoursPerStoryPoint` se recibe como **`json.Number`** (biblioteca
estándar), que conserva el texto literal que llegó. La validación cuenta los decimales sobre ese
texto y rechaza `6.555` **sin redondear**; después lo convierte a centésimas.

Las horas estimadas de FR-045 se calculan en enteros:

```text
horas_en_centésimas = total_story_points × hours_per_story_point_hundredths
```

El escenario US4-5 queda exacto: `16 × 650 = 10400` centésimas = **104,00 horas**.

**Rationale**:

- RN-07 prohíbe redondear y el supuesto de la spec dice que el factor "se usa sin redondeos
  intermedios". Un `float64` no puede garantizarlo: `0.01` no es representable en binario y una
  cadena de multiplicaciones acumula error. Los enteros lo garantizan por construcción.
- Si el campo se deserializara directamente a `float64`, el backend **no podría distinguir** `6.5`
  de `6.500` ni detectar que llegaron tres decimales, porque esa información se pierde en el
  parseo. `json.Number` la conserva, y es de la biblioteca estándar.
- Los Story Points son enteros: la escala de `specs/003-product-backlog` (FR-014) es
  `0, 1, 2, 3, 5, 8, 13, …` y rechaza decimales. Así que el producto es entero y no hay división
  intermedia.

**Alternativas consideradas**:

- **Columna `numeric(4,2)` con una librería decimal** (`shopspring/decimal`): descartada por el
  Principio IV — dependencia no autorizada — y porque no aporta nada sobre los enteros en un
  cálculo de una sola multiplicación.
- **Columna `numeric(4,2)` leída como `float64`**: descartada, reintroduce el error de
  representación y no detecta los tres decimales.
- **Columna `numeric(4,2)` leída como `string`**: descartada, obliga a parsear en cada cálculo y
  deja la aritmética en el mismo lugar donde la dejan los enteros, con más ceremonia.

### Representación en la API

`hoursPerStoryPoint` y `estimatedHours` se exponen en JSON como **números con hasta 2 decimales**
(`6.5`, `104`), no como cadenas: todo valor de 2 decimales en el rango de esta feature es
representable de forma exacta en el `double` de JSON, así que no hay riesgo de que la persona vea
un número distinto del que se guardó. La exactitud que importa está garantizada aguas arriba, en el
almacenamiento y el cálculo.

---

## 4. Fechas de calendario y zona horaria del sistema (FR-039, supuesto "Fechas sin hora")

### Decisión

- `start_date`, `planned_end_date` y `actual_end_date` son columnas **`date`** de PostgreSQL, sin
  hora ni zona. En Go son `time.Time` con etiqueta `gorm:"type:date"`, y en la API se serializan
  como cadenas `YYYY-MM-DD` (`format: date` de OpenAPI), convertidas explícitamente en los DTO de
  la capa `delivery` con el layout `2006-01-02`.
- La zona horaria única del sistema se configura con una variable de entorno nueva,
  **`APP_TIMEZONE`** (por ejemplo `America/Argentina/Buenos_Aires`), que se carga con
  `time.LoadLocation` al arrancar. Si el valor no es una zona válida, la aplicación **no arranca** y
  lo informa, igual que hace la 001 con `JWT_SECRET`.
- La fecha de finalización real se deriva así: `time.Now().In(systemLocation)` y de ahí se toma la
  fecha de calendario. Nunca se toma de la entrada del usuario (FR-040).
- `cmd/app/main.go` importa `_ "time/tzdata"` para **embutir la base de zonas horarias en el
  binario**.

**Rationale**:

- Tratar las fechas como `timestamptz` obligaría a elegir una hora arbitraria y a razonar sobre
  desplazamientos en cada comparación (FR-005: la prevista no puede ser anterior a la de inicio).
  El tipo `date` hace que la comparación sea la natural y que no exista el concepto de "la misma
  fecha en otra zona".
- `time/tzdata` es de la biblioteca estándar y resuelve un problema real: la imagen `alpine` del
  `Dockerfile` del backend no trae la base de zonas horarias, así que sin ese `import`
  `time.LoadLocation("America/Argentina/Buenos_Aires")` fallaría **solo dentro del contenedor**.
  Embutirla cuesta un `import` y unos pocos megabytes, y hace que el caso límite "finalización
  cerca de la medianoche" se comporte igual en desarrollo y en producción.
- Validar la zona al arrancar evita descubrir en la primera finalización que la variable estaba mal
  escrita.

**Alternativas consideradas**:

- **Fijar la zona en el código**: descartada, FR-039 la pide configurable.
- **Instalar el paquete `tzdata` en la imagen de Docker**: descartada, deja el comportamiento
  dependiendo de la imagen base en lugar del binario.
- **Guardar la fecha de finalización real como `timestamptz` y convertir al leer**: descartada,
  haría que la fecha mostrada dependa de quién consulta, y FR-039 la fija en una sola zona.

---

## 5. Autorización: el `404` indistinguible y el `403` del propietario (FR-049, FR-050, SC-005)

### Decisión

Dos middlewares encadenados sobre el grupo `/api/v1/projects/:projectId`, después del middleware de
sesión de la 001:

1. **`RequireMembership`**: busca la membresía por `(project_id, user_id)` del titular de la
   sesión. Si no existe —porque el proyecto no existe, o porque existe y quien pide no es
   integrante— responde **`404`** con el **mismo cuerpo** en los dos casos y corta la cadena. Si
   existe, deja el proyecto y la membresía en el contexto.
2. **`RequireOwnership`**: aplicado solo al subgrupo de acciones de administración (modificar,
   cambiar de estado, alta y baja de integrantes). Compara el titular con `projects.owner_id` y
   responde **`403`** si no coincide.

**Rationale**:

- Una **sola consulta** decide el `404` de FR-049 y de paso trae el proyecto, así que el camino
  indistinguible no es una convención que haya que recordar en cada manejador: es la única salida
  posible del middleware. SC-005 se verifica comparando las respuestas de un identificador
  inexistente y de un proyecto ajeno, que son literalmente el mismo cuerpo generado en el mismo
  lugar.
- Separar los dos middlewares hace que el orden `404` antes de `403` sea estructural: quien no es
  integrante nunca llega a saber que existe un proyecto al que le falta permiso. El orden inverso
  filtraría la existencia del proyecto.
- El estado Finalizado (FR-018) **no** se verifica en un middleware sino en la capa de servicio,
  porque no es una regla de acceso sino de dominio: depende de qué operación se pide y la consulta
  sí está permitida sobre un proyecto Finalizado.

**Alternativas consideradas**:

- **Verificar la membresía dentro de cada servicio**: descartada, repite la regla en seis lugares y
  cada olvido es una filtración (DRY y SC-005).
- **Un único middleware con un parámetro de rol**: descartada, el `404` y el `403` tienen
  precedencia distinta y mezclarlos en una función con bandera oscurece justamente lo que SC-005
  mide.
- **Responder `403` a quien no es integrante**: prohibido por FR-049.

---

## 6. Membresía: alta, baja y nueva alta (FR-023, FR-027, FR-029, SC-008)

### Decisión

- Índice **único compuesto** `(project_id, user_id)` en `project_members`; el alta duplicada se
  resuelve traduciendo `gorm.ErrDuplicatedKey` al error de integrante duplicado (FR-022, FR-023,
  SC-008).
- La baja es un **borrado físico** de la fila de `project_members`. **No** se usa el borrado lógico
  de GORM (`gorm.DeletedAt`).
- Volver a agregar a alguien inserta una fila nueva, con su nueva fecha de incorporación (FR-029).

**Rationale**:

- La membresía es el permiso de acceso, no un registro histórico. FR-027 conserva el esfuerzo, los
  votos y los defectos porque esos registros referencian al **usuario**, no a la membresía:
  borrarla no toca ninguno de ellos. SC-011 se verifica contando esos registros antes y después de
  la baja.
- El borrado lógico sería **incompatible** con el índice único: una fila marcada como borrada
  seguiría ocupando el par `(project_id, user_id)` y el nuevo alta de FR-029 fallaría, salvo
  incluyendo la columna de borrado en el índice, que es exactamente la complejidad que el borrado
  físico evita.
- El historial de membresías (quién entró y salió cuándo) está en "Fuera de Alcance" de la spec
  ("Historial de auditoría consultable"), así que conservarlo sería alcance no pedido
  (Principio I).

**Alternativas consideradas**:

- **Borrado lógico con `revoked_at`**, como las sesiones de la 001: descartada. Allí era obligatorio
  porque FR-026 de esa spec exige invalidar una sesión ya emitida; acá nada pide recordar membresías
  pasadas.
- **Índice único parcial** `WHERE revoked_at IS NULL`: descartada, `AutoMigrate` no lo declara de
  forma portable y resuelve un problema que el borrado físico no tiene.

---

## 7. El propietario: una sola fuente de verdad

### Decisión

`projects.owner_id` es la **única** representación de la propiedad. La tabla `project_members`
**no** tiene columna `is_owner`: la marca de propietario que FR-030 y FR-032 muestran se calcula
comparando `project_members.user_id` con `projects.owner_id`.

**Rationale**: dos columnas que nunca pueden discrepar son el mismo dato escrito dos veces (DRY,
Principio I), y abren la pregunta de cuál manda si discrepan. Es el mismo criterio con el que la 001
reutilizó `created_at` como momento de emisión de la sesión en lugar de agregar `issued_at`.

La entidad "Integrante del proyecto" de la spec enumera "si es el propietario" entre sus atributos;
el modelo lo cumple como **atributo derivado**, no como columna.

**Alternativas consideradas**: columna `is_owner boolean` en `project_members` — descartada por la
duplicación y porque RC-04 fija que el propietario es único y no se transfiere en esta feature, así
que tampoco aporta flexibilidad futura que alguien vaya a usar.

Por la misma razón, la **fecha de incorporación** de FR-030 es `project_members.created_at`: una
membresía se crea en el instante en que la persona se incorpora, así que una columna `joined_at`
sería ese mismo dato duplicado.

---

## 8. Lectura de la tabla `users` desde el módulo de proyectos (FR-019, FR-020, FR-030)

### Decisión

El módulo `internal/projects/` lee la tabla `users` **únicamente** a través de un adaptador de solo
lectura, `internal/projects/repository/user_directory.go`, que:

- Busca un usuario por su correo normalizado, para el alta de integrantes (FR-019, FR-020).
- Resuelve el nombre y el correo de los integrantes de un proyecto con **un solo `JOIN`** entre
  `project_members` y `users` (FR-030).
- **Nunca selecciona `password_hash`**: las consultas enumeran explícitamente
  `users.id, users.full_name, users.email`.

La interfaz que ese adaptador satisface se declara en `internal/projects/domain/`, con un tipo
propio del módulo (`UserRef`) que tiene solo identificador, nombre completo y correo.

**Rationale**:

- El `JOIN` evita el N+1 que tendría llamar al repositorio de la 001 una vez por integrante. SC-003
  pide la vista de estado en menos de 3 segundos y un proyecto con muchos integrantes no debe
  degradarse.
- Declarar la interfaz en `projects/domain` con un tipo propio mantiene la dirección de dependencias
  del Principio III (todo apunta a `domain`) y, sobre todo, hace que **el hash de la contraseña no
  entre nunca al módulo de proyectos**, ni siquiera como campo de una estructura en memoria. Es la
  misma garantía estructural que la 001 buscó al no tener ninguna columna donde quepa una
  contraseña.
- Concentrar en un archivo todo el acceso a `users` deja la regla auditable con una sola lectura: si
  mañana hay que cambiar cómo se accede a las cuentas, hay un único lugar donde mirar.

**Alternativas consideradas**:

- **Reusar `auth/domain.UserRepository` desde `projects/service`**: descartada. Devuelve
  `auth/domain.User`, que incluye `PasswordHash`, y solo ofrece búsqueda de uno en uno, lo que
  fuerza el N+1 del listado de integrantes.
- **Agregar `FindByIDs` a `auth/domain.UserRepository`**: descartada. Modifica el contrato de la
  feature 001 para cubrir una necesidad que la 001 no tiene, y el Principio III pide que esas
  interfaces declaren solo lo que su feature usa.
- **Duplicar el modelo `User` dentro del módulo de proyectos**: descartada, dos modelos GORM sobre
  la misma tabla es una invitación a que `AutoMigrate` discrepe consigo mismo.

---

## 9. Puertos hacia el backlog y los sprints (FR-041 a FR-045, RN-16)

### El problema

FR-042 a FR-045 exigen que la vista de estado informe el sprint activo, la cantidad de historias del
backlog por estado y el total de Story Points con su equivalente en horas. FR-041 y RN-16 exigen
rechazar la finalización mientras haya un sprint activo. Esos datos los producen
`specs/003-product-backlog` y `specs/004-sprint-management`, que **no están implementadas**, y RC-06
dice que esta feature define qué se muestra, no cómo se genera.

### Decisión

`internal/projects/domain/` declara **dos puertos** con exactamente las operaciones que la spec
necesita:

| Puerto | Operación | Requisito que la pide |
| --- | --- | --- |
| `BacklogSummaryProvider` | `Summarize(ctx, projectID) (BacklogSummary, error)` | FR-042, FR-044, FR-045 |
| `SprintDirectory` | `FindActiveSprint(ctx, projectID) (*ActiveSprint, error)` | FR-041, FR-042, FR-043, RN-16 |

`BacklogSummary` lleva el reparto de historias por estado como un **mapa de estado a cantidad** y el
total de Story Points estimados. `ActiveSprint` lleva el identificador y el nombre del sprint, más
nada.

Mientras las features 003 y 004 no existan, las implementaciones viven en
`internal/projects/repository/` e informan un backlog vacío y la ausencia de sprint activo. Cuando
esas features lleguen, se reemplaza el cuerpo de esos dos archivos **sin tocar `projects/service`**.

**Rationale**:

- **No es infraestructura especulativa.** Los cuatro requisitos que los puertos sirven están en esta
  spec y hay escenarios de aceptación que los prueban. Sin el puerto de sprints, FR-041 y RN-16 no
  se pueden implementar ni probar; sin el de backlog, el escenario US4-5 (16 Story Points y 104
  horas con factor 6,5) no se puede escribir.
- El mapa de estado a cantidad **no fija el conjunto de estados del backlog**, que es de la feature
  003 (supuesto explícito de la spec: "esta spec no fija ese conjunto de estados"). Un tipo
  enumerado acá acoplaría las dos features en la dirección equivocada.
- Con los puertos, las pruebas unitarias obligatorias del cálculo de horas (Principio II: "el
  cálculo de métricas" y "la estimación") se escriben con dobles de `testify` y valores distintos de
  cero, que es lo único que hace verificable FR-045 hoy.
- El comportamiento de las implementaciones actuales no es un atajo ni un dato inventado: en el
  estado actual del producto **todo proyecto tiene el backlog vacío y ningún sprint**, que es
  exactamente lo que FR-043 y FR-044 mandan informar.

**Alternativas consideradas**:

- **Devolver ceros directamente en el servicio, con un comentario**: descartada. Deja FR-041 sin
  implementar, RN-16 sin prueba y el cálculo de FR-045 sin forma de ejercitarse con valores reales.
- **Implementar en esta feature una parte del backlog y de los sprints**: prohibido, está en "Fuera
  de Alcance" de la spec y violaría el Principio I.
- **Diferir toda la vista de estado hasta que existan 003 y 004**: descartada, US4 es una historia
  de prioridad P2 de esta feature con su propia prueba independiente.

---

## 10. La identidad del titular de la sesión entre módulos (FR-051)

**Decisión**: el middleware de sesión de la feature 001 deja el identificador del titular en el
contexto de Gin, y el módulo de proyectos lo lee a través del **accesor exportado** de
`internal/auth/delivery`. El middleware en sí se inyecta como `gin.HandlerFunc` desde
`cmd/app/main.go`, que es el único lugar que conoce las dos features.

**Rationale**: el Principio III reserva `cmd/app/` para componer dependencias y arrancar el
servidor, y esto es precisamente una dependencia que componer. Así `projects/delivery` no importa el
paquete `delivery` de la 001 para obtener el middleware, solo para leer la identidad con una función
exportada.

**Señalado al desarrollador**: la feature 001 debe exportar ese accesor (algo como
`CurrentUserID(c *gin.Context) (uuid.UUID, bool)`). Si al implementarla quedó como una constante no
exportada o como una clave literal, esta feature agrega el accesor en `auth/delivery` en lugar de
repetir el literal de la clave en otro módulo, que es la variante frágil.

**Alternativas consideradas**:

- **Repetir el literal de la clave del contexto en `projects/delivery`**: descartada, un cambio en la
  001 rompería la 002 en silencio.
- **Mover el middleware a `internal/platform/`**: descartada por ahora. Obliga a reorganizar código
  ya entregado de la 001 para un beneficio que la inyección desde `cmd/app/` ya consigue. Conviene
  reconsiderarlo cuando una tercera feature necesite lo mismo.

---

## 11. Listado: orden estable y cantidad de integrantes (FR-032, FR-035)

**Decisión**: el listado se resuelve con **una sola consulta** que une `project_members` (filtrada
por el titular de la sesión) con `projects`, y agrega la cantidad de integrantes con una subconsulta
de conteo. El orden es `start_date DESC, name_normalized ASC`.

**Rationale**:

- Una consulta por proyecto para contar integrantes sería un N+1 en la pantalla de entrada al
  sistema.
- FR-035 pide "nombre ascendente" y se ordena por `name_normalized` en lugar de por `name`: así el
  orden no depende de cómo se capitalizó cada nombre, que es lo que hace al orden "estable y
  predecible" en los términos del requisito.
- El filtro por membresía en la misma consulta cumple FR-033 por construcción: un proyecto ajeno no
  tiene fila de membresía del titular, así que no puede aparecer.

**Alternativas consideradas**: cargar los proyectos y después contar integrantes con `Preload` —
descartada, trae todas las filas de membresía a memoria para devolver un número.

---

## 12. Códigos de estado y forma de los errores

**Decisión**: se reutilizan los esquemas de error de la feature 001 (`ErrorResponse`,
`FieldError`, `ValidationErrorResponse`) y se fija esta correspondencia:

| Situación | Código | Requisito |
| --- | --- | --- |
| Entrada inválida (nombre, descripción, fechas, factor, estado destino) | `400` | FR-010, FR-013 |
| Sin sesión válida | `401` | FR-046 |
| Integrante que no es el propietario pide una acción de administración | `403` | FR-050, SC-006 |
| Proyecto inexistente **o** quien pide no es integrante | `404` | FR-049, SC-005 |
| La persona a quitar no es integrante del proyecto | `404` | FR-026 |
| Nombre repetido del mismo propietario | `409` | FR-004, FR-011 |
| El usuario ya es integrante | `409` | FR-022, FR-023 |
| Transición de estado inválida | `409` | FR-038, SC-009 |
| Finalizar con un sprint activo | `409` | FR-041, RN-16 |
| Cualquier acción de administración sobre un proyecto Finalizado | `409` | FR-018, RN-11 |
| El propietario intenta quitarse a sí mismo | `409` | FR-025, RN-12 |
| El correo no corresponde a ningún usuario registrado | `422` | FR-021 |

**Rationale**:

- El `422` para "correo sin cuenta" existe para **no gastar el `404`** en algo que no es el recurso
  de la ruta. En `POST /api/v1/projects/{projectId}/members` el recurso de la ruta es el proyecto, y
  FR-049 exige que un `404` de ese endpoint signifique siempre y solo "el proyecto no existe o no
  sos integrante". Si el correo inexistente también respondiera `404`, el propietario no podría
  distinguir un correo mal escrito de un problema con el proyecto, que es justamente lo que RC-03
  quiere evitar.
- En cambio, en `DELETE /api/v1/projects/{projectId}/members/{userId}` el integrante **sí** es un
  recurso de la ruta, así que el `404` de FR-026 es el código natural. La precedencia del middleware
  garantiza que el `404` del proyecto se evalúe primero, así que no hay ambigüedad.
- El `409` agrupa los conflictos con el estado actual del recurso (nombre tomado, membresía
  existente, transición no permitida, proyecto de solo lectura, sprint abierto). Son todos "la
  petición es válida pero el estado del sistema no la admite".

**Alternativas consideradas**:

- **`403` para el proyecto Finalizado**: descartada, no es una cuestión de permisos — el propietario
  tiene el permiso y la acción sigue estando prohibida.
- **`404` para el correo sin cuenta**: descartada por la ambigüedad descrita.
- **`400` para las transiciones inválidas**: descartada, la entrada está bien formada; lo que no
  encaja es el estado del proyecto.

### `PUT` para la modificación, no `PATCH`

**Decisión**: `PUT /api/v1/projects/{projectId}` recibe **los cinco campos editables completos**
(nombre, descripción, fechas y factor), no un subconjunto.

**Rationale**: FR-013 exige "exactamente las mismas validaciones de nombre, descripción, fechas y
factor que en la creación", y FR-015 exige que la modificación sea atómica. Con la representación
completa, la validación de la creación se reutiliza tal cual y no hay que razonar sobre qué significa
"ausente" frente a "borrar el valor" en cada campo —una ambigüedad real en la descripción, que es
opcional y anulable—. También hace imposible el cambio parcial que FR-015 prohíbe.

**Lectura de la spec**: la tabla de entradas y salidas dice "los datos a modificar" y el escenario
US5-2 cambia solo la fecha prevista. Con `PUT` ese escenario se cumple igual: el cliente manda los
cinco campos con los cuatro anteriores sin cambios, que es lo que hace el formulario de edición, ya
que se precarga con los valores vigentes. **Es una decisión de forma del endpoint, no un cambio de
alcance**; conviene que quede confirmada porque afecta al contrato.

**Alternativas consideradas**: `PATCH` con los cinco campos opcionales — descartada, obliga a
distinguir "no enviado" de "enviado vacío" en la descripción y duplica la validación en dos modos.

### La lista de integrantes no tiene endpoint propio

**Decisión**: FR-030 se satisface a través de `GET /api/v1/projects/{projectId}`, que ya devuelve la
lista de integrantes con el nombre completo, la marca de propietario, la fecha de incorporación y el
correo cuando corresponde. **No se expone `GET /api/v1/projects/{projectId}/members`.**

**Rationale**: un endpoint cuya respuesta es un subconjunto exacto de otra ya existente es una pieza
más que testear, documentar y mantener sin requisito que la pida (Principio I). La pantalla de
administración de integrantes de US3 vive dentro de la vista de estado, así que consume esa misma
respuesta. La regla de visibilidad del correo queda además en **un solo lugar**, lo que es
justamente lo que hace verificable FR-030.

**Alternativas consideradas**: un endpoint dedicado — descartada por lo anterior; si en el futuro la
lista necesitara paginación o filtros propios, se agrega entonces.

---

## 13. Organización en módulos (Principio III)

**Decisión**: un módulo nuevo, `backend/internal/projects/`, con sus cuatro capas (`domain/`,
`repository/`, `service/`, `delivery/`). Las dos entidades persistidas (`Project`,
`ProjectMember`), el tipo de estado y los dos puertos hacia otras features viven en su `domain/`.

No se crea ningún paquete nuevo en `internal/platform/`: la configuración y la conexión que esta
feature necesita ya existen, y solo se le agrega la lectura de `APP_TIMEZONE` al paquete
`platform/config` de la 001.

**Rationale**:

- Proyectos e integrantes son la misma feature: la membresía no tiene sentido sin el proyecto y no
  hay ninguna funcionalidad pedida que atraviese esa frontera. Separarlos en dos módulos sería
  abstracción prematura (Principio I), el mismo razonamiento con el que la 001 no separó `user` de
  `auth`.
- El módulo de proyectos **no** importa `internal/auth/service` ni `internal/auth/repository`. Sus
  dos únicos puntos de contacto con la 001 son el accesor de identidad de `auth/delivery`
  (sección 10) y la tabla `users` leída por su propio adaptador (sección 8).

**Alternativas consideradas**:

- **Módulos `projects` y `members` separados**: descartada por abstracción prematura.
- **Agregar las entidades de proyecto al módulo `auth` existente**: descartada, son features
  distintas y el Principio III pide organización vertical por feature.

---

## Resumen: incógnitas resueltas

| Incógnita | Resolución |
| --- | --- |
| ¿Hace falta alguna dependencia nueva? | No. Ninguna (sección 1) |
| ¿Cómo se garantiza la unicidad del nombre por propietario bajo concurrencia? | Columna `name_normalized` más índice único `(owner_id, name_normalized)` (sección 2) |
| ¿Cómo se guarda y se calcula el factor sin perder precisión ni agregar dependencias? | Enteros de centésimas y `json.Number` en la entrada (sección 3) |
| ¿Cómo se determina la fecha de finalización real? | Columnas `date`, `APP_TIMEZONE` validada al arrancar y `time/tzdata` embutido (sección 4) |
| ¿Cómo se hace indistinguible el proyecto ajeno del inexistente? | Un middleware de membresía con una sola consulta y una sola salida `404` (sección 5) |
| ¿Cómo se permite volver a agregar a un integrante quitado? | Borrado físico de la membresía más índice único `(project_id, user_id)` (sección 6) |
| ¿Dónde vive la marca de propietario y la fecha de incorporación? | Derivadas de `projects.owner_id` y de `project_members.created_at` (sección 7) |
| ¿Cómo lee este módulo la tabla `users` sin ver el hash de la contraseña? | Un adaptador de solo lectura que nunca selecciona `password_hash` (sección 8) |
| ¿Cómo se informan el sprint activo y el backlog si esas features no existen? | Dos puertos en `domain` con implementaciones que informan vacío (sección 9) |
| ¿Cómo obtiene este módulo la identidad del titular de la sesión? | Accesor exportado de `auth/delivery`; el middleware se inyecta desde `cmd/app` (sección 10) |
| ¿Cómo se arma el listado sin N+1 y con orden estable? | Una consulta con `JOIN` y subconsulta de conteo, ordenada por `start_date DESC, name_normalized ASC` (sección 11) |
| ¿Qué código HTTP corresponde a cada rechazo? | Tabla de la sección 12 |
| ¿Cómo se organiza el código? | Un módulo `internal/projects/` con cuatro capas (sección 13) |

### Puntos abiertos para el desarrollador (no bloquean el diseño)

1. **La feature 001 tiene que estar implementada e integrada en `main`** antes de la primera tarea de
   esta feature. Hoy no existen `backend/` ni `frontend/` (sección 0).
2. **Variable de entorno nueva**: `APP_TIMEZONE` hay que agregarla a `backend/.env.example`,
   `backend/.env` y `backend/.env.test`, que los crea la feature 001 (sección 4).
3. **Accesor de identidad en `auth/delivery`**: si la 001 quedó sin exportarlo, esta feature lo
   agrega en lugar de repetir el literal de la clave del contexto (sección 10).
4. **Las implementaciones de los dos puertos se reemplazan** cuando lleguen
   `specs/003-product-backlog` y `specs/004-sprint-management`; conviene dejarlo anotado en el plan
   de esas features (sección 9).
5. **RC-03 es una exposición aceptada por la spec**: la respuesta `422` confirma que un correo está
   o no registrado en el sistema. Conviene que lo repase `/revisar-seguridad` junto con el resto de
   la feature.
