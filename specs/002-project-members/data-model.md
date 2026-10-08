# Modelo de Datos (Fase 1): Gestión de Proyectos e Integrantes

**Feature**: `specs/002-project-members` | **Fecha**: 2026-10-07

Entidades derivadas de la sección "Entidades Clave" de [spec.md](./spec.md). Las decisiones de
diseño que las justifican están en [research.md](./research.md).

Reglas transversales obligatorias (restricciones técnicas de la constitución):

- La clave primaria de los modelos persistidos es **UUID**; prohibidos los enteros secuenciales.
- Todo modelo registra **fecha y hora de creación y de última actualización** (`created_at`,
  `updated_at`, gestionadas por GORM).
- El esquema se crea con `AutoMigrate` de GORM.
- Los identificadores del código están en inglés (Principio VIII).

**Glosario**: donde la spec dice "caracteres", este documento dice **runas** (`rune` de Go, un punto
de código Unicode), que es la unidad en la que se cuenta al validar y la que también cuenta
`varchar(n)` de PostgreSQL. Las dos coinciden en todos los límites de esta feature —1 y 100 para el
nombre, 1000 para la descripción, 254 para el correo—, así que no hay ninguna diferencia de
comportamiento. El factor es el único valor que no se mide en runas: se guarda en **centésimas
enteras** (ver la sección 1).

Esta feature **no modifica** la tabla `users` de `specs/001-user-auth`: solo la lee, y únicamente a
través del adaptador descrito en la sección 5 ([research.md](./research.md), sección 8).

---

## 1. `Project` — tabla `projects`

Representa un emprendimiento de software que el equipo estima, planifica, sigue y mide. Es el ámbito
al que pertenecen el backlog, los sprints y todas las mediciones.

| Campo (Go) | Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- | --- |
| `ID` | `id` | `uuid` | PK, generado en la aplicación con `google/uuid` | FR-009 |
| `OwnerID` | `owner_id` | `uuid` | `NOT NULL`, FK a `users.id`, indexada | FR-007, RN-01 |
| `Name` | `name` | `varchar(100)` | `NOT NULL`; nombre tal como se escribió, ya recortado | FR-002 |
| `NameNormalized` | `name_normalized` | `varchar(100)` | `NOT NULL`; minúsculas y recortado; **índice único compuesto** con `owner_id` | FR-004, FR-011, RN-04, RN-05 |
| `Description` | `description` | `varchar(1000)` | **anulable**; opcional | FR-003 |
| `StartDate` | `start_date` | `date` | `NOT NULL` | FR-005 |
| `PlannedEndDate` | `planned_end_date` | `date` | `NOT NULL`; nunca anterior a `start_date` | FR-005, RN-06 |
| `ActualEndDate` | `actual_end_date` | `date` | **anulable**; no nulo solo si el estado es `finished` | FR-008, FR-039, RN-10 |
| `HoursPerStoryPointHundredths` | `hours_per_story_point_hundredths` | `integer` | `NOT NULL`; **centésimas de hora**, entre `1` y `4000` | FR-006, RN-07 |
| `Status` | `status` | `varchar(16)` | `NOT NULL`; uno de los tres valores de la sección 3 | FR-008, FR-036 |
| `CreatedAt` | `created_at` | `timestamptz` | `NOT NULL` | Constitución |
| `UpdatedAt` | `updated_at` | `timestamptz` | `NOT NULL` | Constitución |

### Reglas de validación

| Regla | Detalle | Requisito |
| --- | --- | --- |
| Nombre obligatorio | Tras `TrimSpace`, entre **1 y 100 runas**. Un nombre ausente o compuesto solo por espacios equivale a ausente y se rechaza. | FR-002, RN-04 |
| Nombre único por propietario | Garantizado por el índice único `(owner_id, name_normalized)`, no por consulta previa. En una modificación, el propio proyecto queda excluido porque un `UPDATE` que no cambia `name_normalized` no viola el índice. | FR-004, FR-011, FR-014, SC-007 |
| Descripción opcional | Si viene, **hasta 1000 runas** tras `TrimSpace`. Una descripción vacía o solo de espacios se guarda como nula. | FR-003 |
| Fechas obligatorias | `start_date` y `planned_end_date` son obligatorias y se reciben como `YYYY-MM-DD`. | FR-005 |
| Orden de las fechas | `planned_end_date >= start_date`. **Iguales es válido**; solo se rechaza anterior. | FR-005, RN-06 |
| Fecha de inicio libre | Puede ser pasada, presente o futura; **no** se compara con la fecha actual. | Supuesto "Fecha de inicio libre" |
| Factor | Se recibe como `json.Number` para conservar el texto literal. **Mayor que 0, menor o igual a 40 y con hasta 2 decimales**; más de 2 decimales se rechaza **sin redondear**. Equivale a `1 <= hundredths <= 4000`. | FR-006, RN-07 |
| Fecha de finalización real | **No se acepta como entrada por ninguna vía.** La fija el sistema al pasar a `finished` y después no se modifica ni se borra. | FR-039, FR-040, RN-10 |

### Notas de diseño

- **Dos columnas de nombre**: `name` es lo que se muestra y `name_normalized` es con lo que se
  compara. A diferencia del correo en la feature 001, acá la capitalización que la persona escribió
  importa ([research.md](./research.md), sección 2).
- **El factor se almacena en centésimas enteras** para que RN-07 ("sin redondear") y FR-045 ("sin
  redondeos intermedios") sean garantías aritméticas y no convenciones
  ([research.md](./research.md), sección 3).
- **No hay columna `is_owner` ni `joined_at` en la membresía**: la propiedad es `projects.owner_id` y
  la fecha de incorporación es `project_members.created_at` ([research.md](./research.md),
  sección 7).
- `status` se modela como **constantes de Go tipadas**, no como `enum` de PostgreSQL: agregar un
  valor en la base obligaría a una migración por cada estado nuevo sin aportar nada que
  `AutoMigrate` y la validación en el dominio no cubran. Es el mismo criterio que la 001 aplicó a
  `auth_events.event_type`.
- `actual_end_date` es anulable porque solo existe en el estado `finished` (FR-008).
- No se definen borrados en cascada hacia `project_members`: esta feature **no borra proyectos**
  (RC-05).

---

## 2. `ProjectMember` — tabla `project_members`

Representa la participación de un usuario en un proyecto y es lo que habilita su acceso. Su
existencia **es** el permiso: no hay membresía sin acceso ni acceso sin membresía.

| Campo (Go) | Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- | --- |
| `ID` | `id` | `uuid` | PK | Constitución |
| `ProjectID` | `project_id` | `uuid` | `NOT NULL`, FK a `projects.id`, **índice único compuesto** con `user_id` | FR-022, FR-023, RN-13 |
| `UserID` | `user_id` | `uuid` | `NOT NULL`, FK a `users.id`, indexada | FR-019 |
| `CreatedAt` | `created_at` | `timestamptz` | `NOT NULL`; **es la fecha de incorporación** | FR-030, Constitución |
| `UpdatedAt` | `updated_at` | `timestamptz` | `NOT NULL` | Constitución |

### Reglas de validación

| Regla | Detalle | Requisito |
| --- | --- | --- |
| Correo normalizado | Antes de buscar al usuario se aplica `TrimSpace` + `ToLower`, igual que la feature 001. | FR-020, RN-05 |
| El usuario tiene que existir | Si el correo normalizado no corresponde a ninguna cuenta, el alta se rechaza. | FR-021 |
| Membresía única | Garantizada por el índice único `(project_id, user_id)`, no por consulta previa. | FR-022, FR-023, RN-13, SC-008 |
| El propietario no se quita | La baja de `user_id == projects.owner_id` se rechaza. | FR-025, RN-12 |
| Solo sobre proyectos no finalizados | El alta y la baja se rechazan si el proyecto está `finished`. | FR-018, RN-11 |

### Ciclo de vida de la membresía

```text
     alta por el propietario            baja por el propietario
   (POST .../members)                 (DELETE .../members/{userId})
            |                                      |
            v                                      v
      [ INTEGRANTE ]  -------------------->  [ SIN MEMBRESÍA ]
      la fila existe                         la fila no existe
            ^                                      |
            |______________________________________|
                    nueva alta (FR-029): fila nueva,
                    nueva fecha de incorporación
```

- **La baja es un borrado físico de la fila.** No se usa el borrado lógico de GORM
  (`gorm.DeletedAt`), que sería incompatible con el índice único y haría fallar la nueva alta de
  FR-029 ([research.md](./research.md), sección 6).
- El acceso se pierde **de inmediato**, porque el middleware de membresía consulta esta tabla en
  cada petición: para la persona quitada el proyecto pasa a comportarse como inexistente (FR-028).
- **Quitar la membresía no borra nada más.** El esfuerzo, los votos de estimación y los defectos de
  las features siguientes referencian al **usuario**, no a la membresía, así que se conservan y
  siguen mostrando su nombre (FR-027, RN-14, SC-011).
- La lista de integrantes **nunca está vacía**: el propietario siempre tiene su membresía (FR-007,
  RN-01).

---

## 3. Estado del proyecto — tipo `ProjectStatus`

Representa el punto del ciclo de vida en que está el proyecto. No es una tabla: es un valor de la
columna `projects.status`, modelado como constantes de Go tipadas.

| Valor almacenado | Etiqueta en la interfaz | Significado |
| --- | --- | --- |
| `planned` | Planificado | Estado inicial de todo proyecto (FR-008) |
| `in_progress` | En curso | El proyecto arrancó |
| `finished` | Finalizado | El proyecto cerró; **es de solo lectura** (FR-018, RN-11) |

Los identificadores están en inglés y las etiquetas en español (Principio VIII): la traducción vive
en el frontend, no en la base.

### Transiciones

```text
      creación
         |
         v
   [ planned ] ----------> [ in_progress ] ----------> [ finished ]
                                                    ^
                                                    |
                              requiere que NO haya sprint activo
                              y fija actual_end_date (FR-039, FR-041)

   Rechazadas (FR-038): planned -> finished (salto)
                        in_progress -> planned (retroceso)
                        finished -> planned, finished -> in_progress (retroceso)
                        planned -> planned, in_progress -> in_progress,
                        finished -> finished (estado actual)
```

- Las **únicas** transiciones admitidas son `planned → in_progress` e `in_progress → finished`
  (FR-037). Las **seis** combinaciones restantes se rechazan, que es lo que mide SC-009.
- `planned → in_progress` **no exige ninguna condición** sobre el backlog ni sobre los sprints
  (FR-041, escenario US6-5).
- `in_progress → finished` se rechaza mientras exista un sprint activo (FR-041, RN-16), consultado
  por el puerto `SprintDirectory` de la sección 4.
- Al entrar en `finished` se fija `actual_end_date` con la fecha de calendario del momento del
  cambio **en la zona horaria del sistema** (`APP_TIMEZONE`), no en la del dispositivo de quien
  finaliza (FR-039). Después no se modifica ni se borra (FR-040, RN-10).
- `finished` es **terminal**: ninguna modificación de datos, cambio de estado ni alta o baja de
  integrantes se admite, y la consulta sigue permitida a cualquier integrante (FR-018, escenario
  US6-3).

---

## 4. Puertos hacia otras features (no persistidos)

La vista de estado y la regla de finalización necesitan datos que producen
`specs/003-product-backlog` y `specs/004-sprint-management`, que todavía no existen. RC-06 fija que
esta feature define **qué se muestra**, no cómo se genera. Se declaran dos puertos en
`internal/projects/domain/` con exactamente las operaciones que la spec pide
([research.md](./research.md), sección 9).

### `BacklogSummaryProvider`

```text
Summarize(ctx, projectID) (BacklogSummary, error)
```

| Campo de `BacklogSummary` | Tipo | Para qué |
| --- | --- | --- |
| `CountsByState` | mapa de estado (texto) a cantidad (entero) | FR-042, FR-044 |
| `EstimatedStoryPoints` | entero | FR-042, FR-045 |

El reparto por estado es un **mapa**, no un tipo enumerado: el conjunto de estados del backlog lo
define la feature 003 y esta spec declara explícitamente que no lo fija.

`EstimatedStoryPoints` suma **solo las historias estimadas**; las que están "sin estimar" se cuentan
en `CountsByState` pero no suman al total (FR-045, escenario US4-5). Una historia estimada en 0
Story Points sí está estimada: suma 0 al total, no se excluye.

### `SprintDirectory`

```text
FindActiveSprint(ctx, projectID) (*ActiveSprint, error)
```

| Campo de `ActiveSprint` | Tipo | Para qué |
| --- | --- | --- |
| `ID` | `uuid` | FR-042 |
| `Name` | texto | FR-042 |

Devuelve nulo cuando el proyecto no tiene sprint activo, que **no es un error** y se informa de
forma explícita (FR-043).

### Estado actual de las implementaciones

Mientras las features 003 y 004 no existan, las implementaciones viven en
`internal/projects/repository/` e informan un backlog vacío y la ausencia de sprint activo. Eso no
es un valor inventado: en el estado actual del producto **ningún proyecto tiene historias ni
sprints**, que es exactamente lo que FR-043 y FR-044 mandan informar. Cuando esas features lleguen
se reemplaza el cuerpo de esos dos archivos, sin tocar `projects/service`.

### Cálculo de las horas estimadas (FR-045, RN-08)

```text
horas_estimadas_en_centésimas = EstimatedStoryPoints × hours_per_story_point_hundredths
```

Todo en enteros, sin divisiones intermedias. El escenario US4-5 queda exacto:
`16 × 650 = 10400` centésimas = **104,00 horas**. La división por 100 ocurre una sola vez, al
serializar la respuesta.

---

## 5. Lectura de la tabla `users`

El módulo de proyectos **no declara ningún modelo GORM sobre `users`** y la lee únicamente a través
de un adaptador de solo lectura ([research.md](./research.md), sección 8). La interfaz se declara en
`internal/projects/domain/` con un tipo propio del módulo:

| Campo de `UserRef` | Para qué |
| --- | --- |
| `ID` | Atribución y comparación con `projects.owner_id` |
| `FullName` | Lista de integrantes (FR-030) |
| `Email` | Lista de integrantes, **solo para el propietario** (FR-030) |

**`UserRef` no tiene ningún campo de contraseña.** El hash no entra al módulo de proyectos ni como
columna, ni como campo en memoria: las consultas del adaptador enumeran explícitamente
`users.id`, `users.full_name` y `users.email`. Es la misma garantía estructural que la feature 001
buscó al no tener ninguna columna donde quepa una contraseña.

### Visibilidad del correo (FR-030)

| Quién consulta | Nombre completo | Marca de propietario | Fecha de incorporación | Correo |
| --- | --- | --- | --- | --- |
| El propietario | sí | sí | sí | **sí** |
| Cualquier otro integrante | sí | sí | sí | **no** |
| Quien no es integrante | — | — | — | — (recibe `404`) |

La omisión del correo ocurre al construir el DTO de salida en la capa `delivery`, que es la única que
conoce quién hizo la petición. El servicio devuelve los datos completos y la decisión de qué mostrar
está en un solo lugar.

---

## 6. Relaciones

```text
   users  (feature 001 — esta feature solo la lee)
     | id (PK)
     |
     +----< projects.owner_id          NOT NULL   (un usuario, N proyectos propios)
     |
     +----< project_members.user_id    NOT NULL   (un usuario, N membresías)

   projects
     | id (PK)
     |
     +----< project_members.project_id NOT NULL   (un proyecto, N integrantes; al menos 1)
```

- `projects.owner_id`: obligatoria. Todo proyecto tiene propietario y es único (RC-04: no se
  transfiere en esta feature).
- `project_members`: índice único `(project_id, user_id)` — un usuario no es integrante dos veces del
  mismo proyecto (RN-13).
- **Invariante**: todo proyecto tiene al menos la membresía de su propietario, creada en la misma
  transacción que el proyecto (FR-007, RN-01), y esa membresía no se puede quitar (FR-025, RN-12).
- Las relaciones **no se navegan desde `Project`**: no hay campos de colección ni `Preload`. Las
  interfaces de repositorio declaran solo las operaciones que esta feature usa y no se genera CRUD
  especulativo (Principio III).
- No se definen borrados en cascada porque esta feature no borra proyectos ni usuarios (RC-05, y la
  eliminación de cuentas está fuera del alcance de la 001).

---

## 7. Interfaces de repositorio (capa `domain`)

Declaran exclusivamente lo que esta feature necesita. Las dependencias apuntan hacia `domain`
(Principio III).

| Interfaz | Operación | Usada por |
| --- | --- | --- |
| `ProjectRepository` | `Create(ctx, project, ownerMembership) error` | Crear el proyecto (transacción proyecto + membresía del propietario) |
| `ProjectRepository` | `FindByID(ctx, id) (*Project, error)` | Vista de estado y acciones de administración |
| `ProjectRepository` | `Update(ctx, project) error` | Modificar datos y cambiar de estado |
| `ProjectRepository` | `ListForMember(ctx, userID) ([]ProjectListItem, error)` | Listado de proyectos propios |
| `ProjectMemberRepository` | `FindByProjectAndUser(ctx, projectID, userID) (*ProjectMember, error)` | Middleware de membresía |
| `ProjectMemberRepository` | `Create(ctx, member) error` | Alta de integrante |
| `ProjectMemberRepository` | `Delete(ctx, projectID, userID) error` | Baja de integrante |
| `ProjectMemberRepository` | `ListMembers(ctx, projectID) ([]MemberView, error)` | Lista de integrantes de la vista de estado |
| `UserDirectory` | `FindByEmail(ctx, normalizedEmail) (*UserRef, error)` | Alta de integrante por correo |
| `BacklogSummaryProvider` | `Summarize(ctx, projectID) (BacklogSummary, error)` | Vista de estado |
| `SprintDirectory` | `FindActiveSprint(ctx, projectID) (*ActiveSprint, error)` | Vista de estado y regla de finalización |

- `ProjectRepository.Create` recibe la membresía del propietario como parámetro para resolver
  **proyecto más membresía en una sola transacción**, en lugar de exponer la transacción al
  servicio. Es el mismo patrón que la feature 001 usó para la cuenta y su evento, y lo que hace que
  FR-010 ("sin crear el proyecto ni ninguna membresía") sea atómico por construcción.
- `ListForMember` y `ListMembers` devuelven tipos de lectura (`ProjectListItem`, `MemberView`) y no
  entidades, porque resuelven en una sola consulta datos que no viven en una sola tabla: la cantidad
  de integrantes y el nombre de cada persona ([research.md](./research.md), secciones 8 y 11).
- `Delete` recibe `(projectID, userID)` y no el identificador de la membresía: es lo que trae la
  ruta `DELETE /api/v1/projects/{projectId}/members/{userId}`, y así no hace falta una búsqueda
  previa solo para obtener un identificador.

**No se declaran**: `Delete` de proyecto, `List` global, búsquedas por nombre, ni ninguna operación
de escritura sobre `users`. Ninguna funcionalidad de la spec las pide, y RC-05 descarta
explícitamente la eliminación de proyectos.

---

## 8. Trazabilidad: requisitos cubiertos por el modelo

| Requisito | Cómo lo cubre el modelo |
| --- | --- |
| FR-002, RN-04 | `name varchar(100)` más validación de 1 a 100 runas tras `TrimSpace` |
| FR-003 | `description varchar(1000)` anulable |
| FR-004, FR-011, FR-014, RN-05, SC-007 | Columna `name_normalized` más índice único `(owner_id, name_normalized)` |
| FR-005, RN-06 | Columnas `date` más validación `planned_end_date >= start_date` |
| FR-006, RN-07 | `hours_per_story_point_hundredths` entre 1 y 4000, con el texto literal validado desde `json.Number` |
| FR-007, RN-01 | `projects.owner_id` más la membresía del propietario creada en la misma transacción |
| FR-008, FR-036 | `status` con valor inicial `planned` y `actual_end_date` nula |
| FR-009 | `projects.id` UUID, estable e inmutable y distinto del nombre |
| FR-010, FR-015 | `Create` y `Update` transaccionales: ningún cambio parcial |
| FR-016 | `Update` de datos no toca `status` ni `actual_end_date` |
| FR-017, RN-15, RC-07 | El factor vive en `projects`; los sprints cerrados guardan el suyo en su propia feature, así que el cambio no los alcanza |
| FR-018, RN-11 | `status = finished` verificado en el servicio antes de toda acción de administración |
| FR-019 a FR-023, RN-13, SC-008 | `UserDirectory.FindByEmail` más índice único `(project_id, user_id)` |
| FR-024, FR-025, RN-12 | `Delete(projectID, userID)` con la comparación contra `projects.owner_id` |
| FR-026 | `Delete` sobre una fila que no existe se traduce a `404` |
| FR-027, FR-029, RN-14, SC-011 | Borrado físico de la membresía; nada más referencia a la membresía |
| FR-028 | El middleware consulta `project_members` en cada petición |
| FR-030 | `ListMembers` con `JOIN` a `users`; el correo se omite en el DTO salvo para el propietario |
| FR-031 a FR-035 | `ListForMember` filtrado por membresía, con conteo y orden `start_date DESC, name_normalized ASC` |
| FR-037, FR-038, SC-009 | Transiciones validadas contra la máquina de estados de la sección 3 |
| FR-039, FR-040, RN-10, SC-010 | `actual_end_date date` fijada con `APP_TIMEZONE` y sin ninguna vía de edición |
| FR-041, RN-16 | `SprintDirectory.FindActiveSprint` consultado antes de finalizar |
| FR-042 a FR-045, RN-08 | `BacklogSummaryProvider` más el producto en centésimas enteras |
| FR-046 a FR-050, RN-02, RN-03, SC-004 a SC-006 | Middleware de sesión de la 001, middleware de membresía (`404`) y middleware de propiedad (`403`) |
| FR-051 | El titular de la sesión identifica al autor de toda acción |
