---

description: "Lista de tareas de la feature 002 — Gestión de Proyectos e Integrantes"
---

# Tareas: Gestión de Proyectos e Integrantes

**Entrada**: documentos de diseño de `specs/002-project-members/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/openapi.yaml](./contracts/openapi.yaml),
[quickstart.md](./quickstart.md)

**⛔ Prerrequisito bloqueante de toda la feature**: la feature
[`specs/001-user-auth`](../001-user-auth/spec.md) tiene que estar **implementada e integrada en
`main`**. Hoy el repositorio no tiene `backend/` ni `frontend/`. Esta feature reutiliza el módulo de
Go, la aplicación Next.js, los contenedores de PostgreSQL, la tabla `users` y el middleware de
sesión que construye la 001: sin eso no hay dónde escribir la primera prueba (T001).

**Pruebas**: **incluidas y obligatorias**. El Principio II de la constitución es NO NEGOCIABLE: la
prueba se escribe antes del código de producción. Las pruebas unitarias son obligatorias para las
validaciones, las reglas de negocio RN-01 a RN-16 y **el cálculo de horas estimadas**, que es
estimación.

**Organización**: las tareas se agrupan por historia de usuario para que cada una se implemente y
se pruebe de forma independiente.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[USn]**: historia de usuario a la que pertenece la tarea
- Cada descripción incluye la ruta exacta del archivo

## Convenciones de ruta

- **Backend**: módulo nuevo `backend/internal/projects/{domain,repository,service,delivery}/`; se
  tocan de forma puntual `backend/cmd/app/main.go`,
  `backend/internal/platform/config/config.go`, `backend/internal/platform/database/postgres.go` y
  `backend/internal/auth/delivery/middleware.go`
- **Frontend**: `frontend/src/app/projects/`, `frontend/src/features/projects/`,
  `frontend/src/types/project.ts`, `frontend/src/utils/project-status.ts`
- **Pruebas**: junto al código que prueban (Go: `*_test.go` en el mismo paquete; Vitest:
  `*.test.ts`/`*.test.tsx` junto al módulo)

## Ciclo obligatorio en cada tarea

Según el Principio II y la skill `/speckit-implement`:

1. **Tarea de pruebas** → se escribe la prueba, se verifica que **falla** y se **detiene**. El
   desarrollador registra el commit con `/redactar-commit`, pie `TDD: red`.
2. **Tarea de implementación** → código mínimo que hace pasar la prueba (`TDD: green`), pausa, y
   refactor si corresponde (`TDD: refactor`).
3. **Tarea técnica** → sin prueba asociada, sin pie TDD.

Ningún agente avanza de RED a GREEN sin ese alto.

---

## Fase 1: Setup (Configuración compartida)

**Propósito**: lo que esta feature necesita del entorno ya construido por la 001. No hay arranque de
proyecto: no se crean módulos de Go, contenedores, Dockerfiles ni configuración de pruebas. Todas
las tareas son **técnicas**.

**Rama**: `fase/002-setup`

- [ ] T001 **PUERTA OBLIGATORIA**: verificar que la feature 001 está implementada e integrada en
      `main` antes de cualquier otra tarea — que existan `backend/` y `frontend/`, que `go version`
      devuelva `1.27.1`, que los dos contenedores de `backend/postgres/docker-compose.yml` estén
      arriba en los puertos 5432 y 5433, y que `POST /api/v1/auth/register` y
      `POST /api/v1/auth/login` respondan `201` y `200`. Si falta algo, detenerse y avisar al
      desarrollador
- [ ] T002 [P] Agregar la clave `APP_TIMEZONE=America/Argentina/Buenos_Aires` a
      `backend/.env.example` (versionado) y replicarla en `backend/.env` y `backend/.env.test`
      (**no versionados**), manteniendo `.env.example` sincronizado con sus claves (Principio VI) —
      depende de T001
- [ ] T003 Leer y validar `APP_TIMEZONE` en `backend/internal/platform/config/config.go` con
      `time.LoadLocation`, usando solo la biblioteca estándar: si la variable falta o no es una zona
      horaria válida, **la aplicación no arranca** y lo informa, igual que hace la 001 con
      `JWT_SECRET` (research.md sección 4) — depende de T002
- [ ] T004 [P] Agregar `import _ "time/tzdata"` en `backend/cmd/app/main.go` para **embutir la base
      de zonas horarias en el binario**, porque la imagen `alpine` del `Dockerfile` del backend no
      la trae y sin eso `time.LoadLocation` falla solo dentro del contenedor (research.md
      sección 4) — depende de T001
- [ ] T005 [P] Exportar en `backend/internal/auth/delivery/middleware.go` el accesor del titular de
      la sesión (algo como `CurrentUserID(c *gin.Context) (uuid.UUID, bool)`), para que el módulo de
      proyectos lo lea sin repetir el literal de la clave del contexto (research.md sección 10). Es
      el **único** cambio de esta feature sobre el código de la 001 fuera de la configuración —
      depende de T001

**Checkpoint**: la feature 001 verificada, `APP_TIMEZONE` leída y validada al arrancar, zonas
horarias embutidas y el accesor de identidad disponible.

---

## Fase 2: Foundational (Prerrequisitos bloqueantes)

**Propósito**: el dominio del módulo nuevo, sus contratos de repositorio y de puertos, la migración
de las dos tablas, el aislamiento de las pruebas y el montaje del grupo de rutas. Todas las tareas
son **técnicas**: declaran estructuras y contratos, sin lógica de negocio.

**⚠️ CRÍTICO**: ninguna historia de usuario puede empezar hasta que esta fase esté completa.

**Rama**: `fase/002-foundational`

- [ ] T006 [P] Declarar la entidad `Project` en `backend/internal/projects/domain/project.go` con los
      campos de data-model.md sección 1: `ID uuid` PK generado con `google/uuid`,
      `OwnerID uuid NOT NULL` con FK a `users.id` e indexada, `Name varchar(100) NOT NULL`,
      `NameNormalized varchar(100) NOT NULL` con **índice único compuesto con `owner_id`**,
      `Description varchar(1000)` **anulable**, `StartDate date NOT NULL`,
      `PlannedEndDate date NOT NULL`, `ActualEndDate date` **anulable**,
      `HoursPerStoryPointHundredths integer NOT NULL` (**centésimas de hora, entre 1 y 4000**),
      `Status varchar(16) NOT NULL`, `CreatedAt` y `UpdatedAt` `timestamptz NOT NULL`. Sin campos de
      colección ni `Preload` hacia `project_members`
- [ ] T007 [P] Declarar la entidad `ProjectMember` en
      `backend/internal/projects/domain/project_member.go` con los campos de data-model.md
      sección 2: `ID uuid` PK, `ProjectID uuid NOT NULL` con FK a `projects.id` e **índice único
      compuesto con `user_id`**, `UserID uuid NOT NULL` con FK a `users.id` e indexada,
      `CreatedAt timestamptz NOT NULL` (**es la fecha de incorporación**) y
      `UpdatedAt timestamptz NOT NULL`. **Sin columna `is_owner` ni `joined_at`**: la propiedad es
      `projects.owner_id` y la fecha de incorporación es `created_at` (research.md sección 7)
- [ ] T008 [P] Declarar el tipo `ProjectStatus` y sus tres constantes tipadas `planned`,
      `in_progress` y `finished` en `backend/internal/projects/domain/status.go`, en inglés y como
      constantes de Go, **no** como `enum` de PostgreSQL (data-model.md sección 3). Las transiciones
      permitidas son regla de negocio y se implementan en US6, no acá
- [ ] T009 [P] Declarar los errores de dominio en `backend/internal/projects/domain/errors.go`:
      nombre de proyecto ya usado por el propietario (FR-004), proyecto inexistente o sin acceso
      (FR-049), falta de permiso de propietario (FR-050), proyecto finalizado de solo lectura
      (FR-018), usuario sin cuenta registrada (FR-021), ya es integrante (FR-022), no es integrante
      (FR-026), el propietario no puede quitarse (FR-025), transición de estado inválida (FR-038) y
      sprint activo que impide finalizar (FR-041)
- [ ] T010 Declarar en `backend/internal/projects/domain/repository.go` las dos interfaces con
      **exactamente** las ocho operaciones de data-model.md sección 7 —
      `ProjectRepository`: `Create(ctx, project, ownerMembership) error`,
      `FindByID(ctx, id) (*Project, error)`, `Update(ctx, project) error`,
      `ListForMember(ctx, userID) ([]ProjectListItem, error)`;
      `ProjectMemberRepository`: `FindByProjectAndUser(ctx, projectID, userID) (*ProjectMember, error)`,
      `Create(ctx, member) error`, `Delete(ctx, projectID, userID) error`,
      `ListMembers(ctx, projectID) ([]MemberView, error)`; más los tipos de lectura
      `ProjectListItem` y `MemberView`. **Prohibido** declarar borrado de proyecto (RC-05), `List`
      global ni búsquedas por otros campos (Principio III) — depende de T006 y T007
- [ ] T011 Declarar los tres puertos en `backend/internal/projects/domain/ports.go` con los tipos de
      data-model.md secciones 4 y 5: `UserDirectory.FindByEmail(ctx, normalizedEmail) (*UserRef, error)`
      con `UserRef` de **solo** identificador, nombre completo y correo (**ningún campo de
      contraseña**); `BacklogSummaryProvider.Summarize(ctx, projectID) (BacklogSummary, error)` con
      `CountsByState` como **mapa de estado a cantidad** (no un enumerado: el conjunto de estados lo
      define la spec 003) y `EstimatedStoryPoints`; y
      `SprintDirectory.FindActiveSprint(ctx, projectID) (*ActiveSprint, error)` con `ID` y `Name`
- [ ] T012 [P] Implementar el puerto de backlog en
      `backend/internal/projects/repository/backlog_summary.go` informando **backlog vacío**
      (`CountsByState` vacío y `EstimatedStoryPoints` en 0), con un comentario en español que
      indique que `specs/003-product-backlog` reemplazará el cuerpo de este archivo. No es un dato
      inventado: hoy ningún proyecto tiene historias, que es lo que FR-044 manda informar
      (research.md sección 9) — depende de T011
- [ ] T013 [P] Implementar el puerto de sprints en
      `backend/internal/projects/repository/sprint_directory.go` informando **ausencia de sprint
      activo** (devuelve nulo sin error), con un comentario en español que indique que
      `specs/004-sprint-management` reemplazará el cuerpo de este archivo. FR-043 exige que esa
      ausencia no sea un error — depende de T011
- [ ] T014 Agregar `projects` y `project_members` al `AutoMigrate` de
      `backend/internal/platform/database/postgres.go`, de modo que los dos índices únicos
      compuestos —`(owner_id, name_normalized)` y `(project_id, user_id)`— queden creados por las
      etiquetas de GORM y no por un `CREATE INDEX` en crudo (research.md secciones 2 y 6) — depende
      de T006 y T007
- [ ] T015 Extender el ayudante de pruebas de integración en
      `backend/internal/platform/testsupport/database.go` (lo crea la tarea T024 de la feature 001)
      para que vacíe también las dos tablas nuevas:
      `TRUNCATE users, sessions, auth_events, projects, project_members CASCADE`, manteniendo la
      carga de `backend/.env.test` y la verificación de que la base es `smye_test` antes de tocar
      nada. Si al llegar acá el ayudante estuviera en un archivo `*_test.go` de otro paquete,
      moverlo primero a esta ubicación: el contenido de un `_test.go` solo existe para su propio
      paquete y el módulo de proyectos no podría importarlo — depende de T014
- [ ] T016 Crear el grupo de rutas `/api/v1/projects` en
      `backend/internal/projects/delivery/routes.go`, **sin ninguna ruta pública** (FR-046), y
      componer el módulo en `backend/cmd/app/main.go` inyectando el middleware de sesión de la 001
      como `gin.HandlerFunc`, de modo que `projects/delivery` no importe el middleware sino solo el
      accesor de identidad (research.md sección 10) — depende de T005, T010, T011 y T014

**Checkpoint**: las dos tablas migradas en la base de test, el dominio y los contratos declarados,
las pruebas aisladas y el grupo de rutas montado. Las historias pueden empezar.

---

## Fase 3: Historia de Usuario 1 — Crear un proyecto (Prioridad: P1) 🎯 MVP

**Objetivo**: una persona con cuenta crea un proyecto y queda como propietario y primer integrante,
en estado `planned` y sin fecha de finalización real.

**Rama**: `hu/002-us1-crear-un-proyecto`

**Prueba independiente**: `POST /api/v1/projects` devuelve `201` en estado `planned` con un solo
integrante marcado propietario, y un segundo proyecto propio con el mismo nombre en otra
capitalización responde `409`, sin necesitar ninguna otra historia.

### Pruebas de la Historia 1 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [ ] T017 [P] [US1] Pruebas unitarias **obligatorias** de validación y normalización en
      `backend/internal/projects/delivery/validation_test.go`, cubriendo RN-04 a RN-07: nombre de
      **entre 1 y 100 runas tras `TrimSpace`**, con nombre ausente o compuesto solo por espacios
      rechazado (RN-04); nombre normalizado con `TrimSpace` + `ToLower` (RN-05); descripción
      opcional de **hasta 1000 runas tras `TrimSpace`**, guardada como nula si queda vacía (FR-003);
      `planned_end_date >= start_date`, con **iguales válido** y solo anterior rechazado (RN-06); y
      el factor leído como `json.Number` y validado **mayor que 0, menor o igual a 40 y con hasta 2
      decimales**, equivalente a `1 <= hundredths <= 4000`, rechazando `0`, negativos, `40.01` y
      `6.555` **sin redondear** (RN-07)
- [ ] T018 [P] [US1] Pruebas unitarias **obligatorias** del cálculo de horas estimadas en
      `backend/internal/projects/service/estimation_test.go`, cubriendo RN-08:
      `horas_en_centésimas = EstimatedStoryPoints × hours_per_story_point_hundredths`, todo en
      enteros y sin divisiones intermedias. Casos: `16 × 650 = 10400` centésimas (104,00 horas),
      `0` Story Points con cualquier factor da 0, y el factor mínimo `1` y máximo `4000` centésimas
- [ ] T019 [US1] Pruebas de integración de la historia en
      `backend/internal/projects/delivery/handler_test.go`, una función por escenario de spec.md
      precedida por `// Escenario: <nombre del escenario> (002/US1)`:
      `TestUS1_CreacionExitosa` (`201` en `planned`, sin `actualEndDate`, con un solo integrante
      marcado propietario), `TestUS1_SinDescripcion`, `TestUS1_MismoNombreOtroPropietario`,
      `TestUS1_FechasIguales`, `TestUS1_LimitesDelFactor` (0,01 y 40), `TestUS1_LimitesDelNombre`
      (100 válido, 101 rechazado), `TestUS1_CamposFaltantes`, `TestUS1_FactorInvalido` (subtests con
      `t.Run` para 0, negativo, 40,01 y 6,555), `TestUS1_FechaPrevistaAnterior`,
      `TestUS1_NombreRepetido` (`  portal de clientes  ` contra "Portal de Clientes" → `409`) y
      `TestUS1_SinSesion` (`401` y nada creado) — depende de T015 y T016
- [ ] T020 [P] [US1] Prueba de integración de concurrencia en
      `backend/internal/projects/delivery/project_concurrency_test.go`: **10 solicitudes de creación
      simultáneas con el mismo nombre y el mismo propietario** dejan exactamente un proyecto y las
      nueve restantes reciben `409`, verificado contando filas agrupadas por
      `(owner_id, name_normalized)` (FR-011, SC-007) — depende de T015 y T016
- [ ] T021 [P] [US1] Pruebas de frontend en
      `frontend/src/features/projects/components/project-form.test.tsx` y
      `frontend/src/features/projects/validation.test.ts`, con
      `describe("002/US1 - Crear un proyecto")` y un `it` por escenario, incluido que el formulario
      **muestra antes de enviar** el máximo de 100 caracteres del nombre, los 1000 de la descripción
      y el rango del factor de 0,01 a 40 con hasta 2 decimales — depende de T001

### Implementación de la Historia 1

- [ ] T022 [US1] Implementar normalización y validación de entrada en
      `backend/internal/projects/delivery/validation.go`, con los valores límite 1, 100, 1000, 1 y
      4000 declarados como **constantes nombradas en un único lugar**, el factor recibido como
      `json.Number` para poder contar los decimales sobre el texto literal, y las fechas parseadas
      con el layout `2006-01-02` y su error manejado. La validación ocurre antes de la capa de
      servicio (Principio VII)
- [ ] T023 [US1] Implementar el cálculo de horas estimadas en
      `backend/internal/projects/service/estimation.go` en centésimas enteras, sin `float64` y sin
      divisiones intermedias (RN-08, FR-045)
- [ ] T024 [US1] Implementar `Create(ctx, project, ownerMembership)` en
      `backend/internal/projects/repository/project_repository.go`: `INSERT projects` +
      `INSERT project_members` del propietario en **una sola transacción**, de modo que FR-010 ("sin
      crear el proyecto ni ninguna membresía") sea atómico por construcción, y traducir
      `gorm.ErrDuplicatedKey` al error de nombre ya usado, sin "consultar y después insertar"
      (FR-007, FR-011, RN-01, SC-007) — depende de T022
- [ ] T025 [US1] Declarar en `backend/internal/projects/delivery/dto.go` los esquemas de
      contracts/openapi.yaml: `ProjectInput`, `ProjectDetail`, `OwnerRef`, `ProjectMemberView`,
      `BacklogSummary`, `ActiveSprintRef`, y reutilizar `ErrorResponse`, `FieldError` y
      `ValidationErrorResponse` de la 001. Los estados se serializan como `planned`,
      `in_progress` y `finished`; `hoursPerStoryPoint` y `estimatedHours` como **números con hasta 2
      decimales**, dividiendo las centésimas por 100 una sola vez al serializar (research.md
      sección 3)
- [ ] T026 [US1] Implementar la creación en
      `backend/internal/projects/service/project_service.go` (construye el `Project` con UUID, el
      nombre normalizado, el estado `planned` y `actual_end_date` nula, más la membresía del
      propietario) y el armado de la respuesta en
      `backend/internal/projects/service/project_view.go` usando los dos puertos y el cálculo de
      T023 — depende de T012, T013, T023 y T024
- [ ] T027 [US1] Implementar el manejador `createProject` en
      `backend/internal/projects/delivery/handler.go` con anotaciones `swag` en español y
      registrarlo como `POST /api/v1/projects` en
      `backend/internal/projects/delivery/routes.go`, devolviendo `201` con `ProjectDetail`, `400`
      con `ValidationErrorResponse`, `401` y `409` con `ErrorResponse`
      (`Ya existe un proyecto propio con ese nombre.`) — depende de T025 y T026
- [ ] T028 [P] [US1] Declarar los tipos de petición y respuesta en
      `frontend/src/types/project.ts` y las etiquetas en español de los tres estados en
      `frontend/src/utils/project-status.ts` (`planned` → Planificado, `in_progress` → En curso,
      `finished` → Finalizado). **Prohibido el tipo `any`** (Principio VIII) — depende de T001
- [ ] T029 [US1] Implementar la llamada de creación en `frontend/src/features/projects/api.ts` sobre
      el cliente `fetch` de la 001, y las reglas del formulario en
      `frontend/src/features/projects/validation.ts` con los límites 1, 100, 1000, 0,01 y 40 como
      constantes nombradas en un único lugar — depende de T028
- [ ] T030 [US1] Implementar el `ProjectForm` en
      `frontend/src/features/projects/components/project-form.tsx`, con los mensajes en español y
      los límites visibles antes de enviar — depende de T029
- [ ] T031 [US1] Crear la pantalla `frontend/src/app/projects/new/page.tsx`, protegida con la
      guardia de sesión de la 001 — depende de T030

**Checkpoint**: US1 funciona y se prueba sola. El MVP está entregable.

---

## Fase 4: Historia de Usuario 2 — Ver mis proyectos y quedar aislado de los ajenos (Prioridad: P1)

**Objetivo**: cada persona ve los proyectos de los que participa, en un orden estable, y un proyecto
ajeno es indistinguible de uno inexistente.

**Rama**: `hu/002-us2-ver-mis-proyectos`

**Prueba independiente**: creando proyectos con dos usuarios distintos, cada uno ve solo los suyos
en `GET /api/v1/projects`, y el middleware de membresía responde `404` para el proyecto ajeno con el
**mismo cuerpo** que para un identificador inexistente. El `404` se prueba montando el middleware
con `httptest` sobre un manejador protegido de prueba, así que esta historia **no** depende de que
exista el endpoint de vista de estado (que es US4).

### Pruebas de la Historia 2 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [ ] T032 [P] [US2] Pruebas unitarias **obligatorias** del middleware de membresía en
      `backend/internal/projects/delivery/middleware_test.go`, montándolo con `httptest` sobre un
      manejador protegido de prueba y cubriendo RN-03: un integrante pasa; quien no es integrante y
      un identificador inexistente reciben respuestas **byte a byte idénticas** (`404` con
      `El proyecto no existe.`); y el manejador protegido **no se ejecuta** en ninguno de los dos
      casos (FR-049, FR-020 de la 001 para el `401`, SC-005) — depende de T015
- [ ] T033 [US2] Pruebas de integración de la historia en
      `backend/internal/projects/delivery/handler_test.go`, una función por escenario precedida por
      `// Escenario: <nombre del escenario> (002/US2)`: `TestUS2_ListadoCompleto` (propietario de dos
      e integrante de un tercero; cada elemento con nombre, estado, fechas, `memberCount` e
      `isOwner`), `TestUS2_ListadoVacio` (`200` con lista vacía y mensaje, **no** un error),
      `TestUS2_IntegranteQuitadoNoVeElProyecto`, `TestUS2_ProyectoConUnIntegrante`
      (`memberCount: 1`), `TestUS2_ProyectoAjenoEsInexistente` y `TestUS2_SinSesion`. Se asserta
      además el orden **fecha de inicio descendente y, a igual fecha, nombre ascendente** (FR-035) —
      depende de T019
- [ ] T034 [P] [US2] Pruebas de frontend en
      `frontend/src/features/projects/components/project-list.test.tsx` con
      `describe("002/US2 - Ver mis proyectos y quedar aislado de los ajenos")` y un `it` por
      escenario, incluido el estado vacío con su mensaje — depende de T021

### Implementación de la Historia 2

- [ ] T035 [US2] Implementar `FindByProjectAndUser(ctx, projectID, userID)` en
      `backend/internal/projects/repository/project_member_repository.go`: **una sola consulta** que
      resuelve a la vez si el proyecto existe y si quien pide es integrante (research.md sección 5)
- [ ] T036 [US2] Implementar `RequireMembership` en
      `backend/internal/projects/delivery/middleware.go` con **una única salida `404`** para
      proyecto inexistente y para no integrante, dejando el proyecto y la membresía en el contexto,
      y aplicarlo al grupo `/api/v1/projects/:projectId` en
      `backend/internal/projects/delivery/routes.go`. El estado Finalizado **no** se verifica acá:
      es regla de dominio y va en el servicio (FR-049, FR-018) — depende de T035
- [ ] T037 [US2] Implementar `ListForMember(ctx, userID)` en
      `backend/internal/projects/repository/project_repository.go` con **una sola consulta**: `JOIN`
      de `project_members` filtrado por el titular con `projects`, más una subconsulta de conteo de
      integrantes, ordenada por `start_date DESC, name_normalized ASC`. El filtro por membresía
      cumple FR-033 por construcción y evita el N+1 (research.md sección 11) — depende de T024
- [ ] T038 [US2] Implementar el listado en
      `backend/internal/projects/service/project_service.go`, resolviendo `isOwner` por comparación
      con `projects.owner_id` y devolviendo el mensaje en español
      (`Todavía no participás en ningún proyecto.`) solo cuando la lista está vacía (FR-032,
      FR-034) — depende de T037
- [ ] T039 [US2] Añadir `ProjectListItem` y `ProjectListResponse` a
      `backend/internal/projects/delivery/dto.go`, el manejador `listProjects` con anotaciones
      `swag` en español en `backend/internal/projects/delivery/handler.go` y la ruta
      `GET /api/v1/projects` en `backend/internal/projects/delivery/routes.go`, con `200` y `401` —
      depende de T038
- [ ] T040 [US2] Implementar la llamada de listado en `frontend/src/features/projects/api.ts` —
      depende de T028
- [ ] T041 [US2] Implementar el `ProjectList` en
      `frontend/src/features/projects/components/project-list.tsx`, con las etiquetas de estado de
      `frontend/src/utils/project-status.ts` y el estado vacío con su mensaje — depende de T040
- [ ] T042 [US2] Crear la pantalla `frontend/src/app/projects/page.tsx` y agregar el enlace al
      listado desde `frontend/src/app/profile/page.tsx` — depende de T041

**Checkpoint**: US1 y US2 funcionan de forma independiente. El aislamiento entre proyectos está
activo.

---

## Fase 5: Historia de Usuario 3 — Gestionar los integrantes del proyecto (Prioridad: P2)

**Objetivo**: el propietario suma y quita integrantes buscándolos por correo, sin perder lo que esas
personas registraron, y el correo de los integrantes solo lo ve el propietario.

**Rama**: `hu/002-us3-gestionar-integrantes`

**Prueba independiente**: con dos cuentas, el propietario agrega a la segunda por correo, esa
persona pasa a ver el proyecto, el propietario la quita y deja de verlo.

### Pruebas de la Historia 3 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [ ] T043 [P] [US3] Pruebas unitarias **obligatorias** de las reglas de negocio en
      `backend/internal/projects/service/member_service_test.go`, cubriendo RN-02, RN-05, RN-11,
      RN-12 y RN-13: el correo se normaliza con `TrimSpace` + `ToLower` antes de buscar la cuenta
      (RN-05); solo el propietario agrega y quita (RN-02); un proyecto `finished` rechaza las dos
      acciones (RN-11); el propietario no puede quitarse (RN-12); y un usuario no puede ser
      integrante dos veces del mismo proyecto (RN-13)
- [ ] T044 [US3] Pruebas de integración de la historia en
      `backend/internal/projects/delivery/handler_test.go`, una función por escenario precedida por
      `// Escenario: <nombre del escenario> (002/US3)`: `TestUS3_AltaExitosa` (`201` y la persona ya
      consulta el proyecto y lo ve en su listado), `TestUS3_CorreoConEspaciosYMayusculas`
      (`  Bruno@Example.COM  ` encuentra la misma cuenta), `TestUS3_NuevaAltaTrasBaja` (recupera el
      acceso y aparece **una sola vez**), `TestUS3_VisibilidadDelCorreo` (el propietario ve los
      correos; **ningún** otro integrante los ve, y los demás campos son iguales),
      `TestUS3_SoloElPropietarioComoIntegrante` (lista con una persona, marcada propietario),
      `TestUS3_BajaConservaElHistorial` (RN-14: la baja toca **solo** `project_members` y ningún
      registro de la persona se pierde ni queda sin autor, SC-011),
      `TestUS3_CorreoSinCuenta` (`422`, **no** `404`), `TestUS3_YaEsIntegrante` (`409` y la lista no
      cambia), `TestUS3_NoEsPropietario` (`403` y ningún efecto),
      `TestUS3_PropietarioNoPuedeQuitarse` (`409`) y `TestUS3_ProyectoFinalizado` (`409` en alta y
      en baja) — depende de T033
- [ ] T045 [P] [US3] Prueba de integración de concurrencia en
      `backend/internal/projects/delivery/member_concurrency_test.go`: **10 altas simultáneas del
      mismo usuario en el mismo proyecto** registran una sola membresía y las nueve restantes
      reciben `409` (FR-023, SC-008) — depende de T015 y T016
- [ ] T046 [P] [US3] Pruebas de frontend en
      `frontend/src/features/projects/components/member-list.test.tsx` y
      `frontend/src/features/projects/components/add-member-form.test.tsx` con
      `describe("002/US3 - Gestionar los integrantes del proyecto")` y un `it` por escenario,
      incluido que un integrante que no es propietario **no ve** los correos ni las acciones de
      administración — depende de T021

### Implementación de la Historia 3

- [ ] T047 [US3] Implementar `FindByEmail(ctx, normalizedEmail)` en
      `backend/internal/projects/repository/user_directory.go`, el **único** punto del módulo que
      lee la tabla `users`, enumerando explícitamente `users.id, users.full_name, users.email` y
      **sin seleccionar nunca `password_hash`** (research.md sección 8, FR-019, FR-020)
- [ ] T048 [US3] Implementar `Create(ctx, member)`, `Delete(ctx, projectID, userID)` y
      `ListMembers(ctx, projectID)` en
      `backend/internal/projects/repository/project_member_repository.go`: el alta traduce
      `gorm.ErrDuplicatedKey` al error de integrante duplicado; la baja es un **borrado físico** de
      la fila, **sin** `gorm.DeletedAt`, que sería incompatible con el índice único y rompería la
      nueva alta de FR-029; y la lista resuelve nombre y correo con **un solo `JOIN`** a `users`,
      también sin seleccionar `password_hash` (research.md secciones 6 y 8) — depende de T035
- [ ] T049 [US3] Implementar `RequireOwnership` en
      `backend/internal/projects/delivery/middleware.go` comparando el titular con
      `projects.owner_id` y devolviendo `403`, y aplicarlo al subgrupo de acciones de administración
      en `backend/internal/projects/delivery/routes.go`. Se encadena **después** de
      `RequireMembership`, para que quien no es integrante nunca llegue a saber que el proyecto
      existe (FR-047, FR-050, SC-006) — depende de T036
- [ ] T050 [US3] Implementar el alta y la baja en
      `backend/internal/projects/service/member_service.go`: normaliza el correo, busca la cuenta,
      rechaza el proyecto `finished`, rechaza la baja del propietario y resuelve `isOwner` por
      comparación con `projects.owner_id` (FR-019 a FR-029) — depende de T047 y T048
- [ ] T051 [US3] Añadir `AddMemberRequest` a `backend/internal/projects/delivery/dto.go` y construir
      `ProjectMemberView` **omitiendo el campo `email` cuando quien consulta no es el propietario**.
      La decisión de visibilidad vive en un solo lugar, la capa `delivery`, que es la única que sabe
      quién hizo la petición (FR-030) — depende de T025
- [ ] T052 [US3] Implementar los manejadores `addProjectMember` y `removeProjectMember` en
      `backend/internal/projects/delivery/handler.go` con anotaciones `swag` en español y
      registrarlos como `POST /api/v1/projects/{projectId}/members` (`201`, `400`, `401`, `403`,
      `404`, `409`, **`422`** para el correo sin cuenta) y
      `DELETE /api/v1/projects/{projectId}/members/{userId}` (`204`, `401`, `403`, `404`, `409`) en
      `backend/internal/projects/delivery/routes.go`. El `422` existe para que el `404` de esos
      endpoints signifique siempre y solo "el proyecto no existe o no sos integrante"
      (research.md sección 12) — depende de T049, T050 y T051
- [ ] T053 [US3] Implementar las llamadas de alta y baja de integrantes en
      `frontend/src/features/projects/api.ts` — depende de T040
- [ ] T054 [US3] Implementar el `MemberList` en
      `frontend/src/features/projects/components/member-list.tsx` y el `AddMemberForm` en
      `frontend/src/features/projects/components/add-member-form.tsx`, mostrando el correo y las
      acciones de administración **solo** cuando quien consulta es el propietario — depende de T053

**Checkpoint**: US1 a US3 funcionan de forma independiente. El proyecto ya es un espacio de equipo.

---

## Fase 6: Historia de Usuario 4 — Consultar el estado del proyecto (Prioridad: P2)

**Objetivo**: cualquier integrante ve en una sola pantalla los datos generales, el estado, las
fechas, los integrantes, el sprint activo, el reparto del backlog y las horas estimadas.

**Rama**: `hu/002-us4-consultar-el-estado`

**Prueba independiente**: creando un proyecto y agregando un integrante, los dos obtienen la vista
de estado con los datos correctos, incluso sin backlog ni sprints. El escenario de 16 Story Points y
104 horas se prueba con dobles de los dos puertos, así que no necesita las features 003 ni 004.

### Pruebas de la Historia 4 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [ ] T055 [P] [US4] Pruebas unitarias del armado de la vista de estado en
      `backend/internal/projects/service/project_view_test.go`, con **dobles de `testify`** de los
      puertos `BacklogSummaryProvider` y `SprintDirectory`: con 3, 5 y 8 Story Points estimados, una
      historia sin estimar y factor 6,5, el resultado es `estimatedStoryPoints: 16` y
      `estimatedHours: 104` (escenario US4-5, FR-045, RN-08); `hasActiveSprint` refleja lo que
      informa el puerto; y `countsByState` se traslada tal como llega, sin fijar el conjunto de
      estados — depende de T011
- [ ] T056 [US4] Pruebas de integración de la historia en
      `backend/internal/projects/delivery/handler_test.go`, una función por escenario precedida por
      `// Escenario: <nombre del escenario> (002/US4)`: `TestUS4_EstadoCompleto` (todos los campos
      de FR-042 en **una** respuesta), `TestUS4_ProyectoFinalizado` (trae `actualEndDate` y
      `readOnly: true`), `TestUS4_SinSprintActivo` (`hasActiveSprint: false` y
      `activeSprint: null`, **sin ser un error**), `TestUS4_ProyectoReciénCreado` (`countsByState`
      vacío, `estimatedStoryPoints: 0` y `estimatedHours: 0`), `TestUS4_HorasEstimadas`,
      `TestUS4_NoIntegranteEsInexistente` (`404` que no revela ni el nombre) y `TestUS4_SinSesion`.
      Incluye además `TestUS4_VistaDeEstadoSinNMasUno`, que cubre **SC-003** (vista de estado en
      menos de 3 segundos) de la única forma que lo hace determinista: con el `Logger` de GORM
      contando las consultas de una petición a `GET /api/v1/projects/{projectId}` y asertando que
      son **a lo sumo 4** —proyecto, integrantes con `JOIN`, resumen del backlog y sprint activo— y
      que ese número **no crece** al agregar integrantes al proyecto. Una aserción de tiempo de
      pared sería inestable en CI; lo que SC-003 exige del diseño es la ausencia de N+1 — depende
      de T044
- [ ] T057 [P] [US4] Pruebas de frontend en
      `frontend/src/features/projects/components/project-status-card.test.tsx` con
      `describe("002/US4 - Consultar el estado del proyecto")` y un `it` por escenario, incluida la
      indicación explícita de que no hay sprint activo y los contadores en cero — depende de T021

### Implementación de la Historia 4

- [ ] T058 [US4] Completar el armado de la vista de estado en
      `backend/internal/projects/service/project_view.go`: datos generales, propietario, factor,
      estado, las tres fechas, `readOnly` derivado de `status == finished`, la lista de integrantes
      de T048, el sprint activo y el resumen del backlog de los dos puertos, y las horas estimadas
      con el cálculo de T023 (FR-042 a FR-045) — depende de T012, T013, T023, T048 y T055
- [ ] T059 [US4] Implementar el manejador `getProjectDetail` en
      `backend/internal/projects/delivery/handler.go` con anotaciones `swag` en español y
      registrarlo como `GET /api/v1/projects/{projectId}` en
      `backend/internal/projects/delivery/routes.go`, accesible a **cualquier integrante** incluso
      con el proyecto `finished`, con `200`, `401` y `404` (FR-048) — depende de T051 y T058
- [ ] T060 [US4] Implementar la llamada de vista de estado en
      `frontend/src/features/projects/api.ts` y el `ProjectStatusCard` en
      `frontend/src/features/projects/components/project-status-card.tsx` — depende de T053
- [ ] T061 [US4] Crear la pantalla `frontend/src/app/projects/[projectId]/page.tsx` montando el
      `ProjectStatusCard`, el `MemberList` y el `AddMemberForm` de US3, todo en **una sola
      pantalla** (SC-003) — depende de T054 y T060

**Checkpoint**: US1 a US4 funcionan de forma independiente. La vista que responde "¿cómo va el
proyecto?" está entregada.

---

## Fase 7: Historia de Usuario 5 — Modificar los datos del proyecto (Prioridad: P3)

**Objetivo**: el propietario corrige nombre, descripción, fechas y factor, sin que cambien el estado
ni la fecha de finalización real.

**Rama**: `hu/002-us5-modificar-el-proyecto`

**Prueba independiente**: creando un proyecto, modificando cada uno de sus campos y verificando que
la vista de estado refleja los nuevos valores y que `status` y `actualEndDate` quedaron intactos.

### Pruebas de la Historia 5 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [ ] T062 [P] [US5] Pruebas unitarias **obligatorias** de las reglas de la modificación en
      `backend/internal/projects/service/project_service_test.go`, cubriendo FR-014, FR-015, FR-016
      y RN-15: la unicidad del nombre **excluye al propio proyecto**, así que renombrarlo con el
      mismo nombre que ya tenía se acepta; un rechazo no aplica **ningún cambio parcial**; la
      modificación **no altera** `status` ni `actual_end_date`; y el cambio del factor **no toca
      ninguna otra tabla**, de modo que los sprints cerrados conserven el suyo
- [ ] T063 [US5] Pruebas de integración de la historia en
      `backend/internal/projects/delivery/handler_test.go`, una función por escenario precedida por
      `// Escenario: <nombre del escenario> (002/US5)`: `TestUS5_ModificacionExitosa`,
      `TestUS5_ProyectoEnCursoSeModifica` (solo `finished` bloquea),
      `TestUS5_MismoNombreQueYaTenia`, `TestUS5_CambioDeFactorConSprintsCerrados`,
      `TestUS5_LimitesDelFactor` (0,01 y 40), `TestUS5_NombreDeOtroProyectoPropio` (`409` y el
      proyecto queda como estaba), `TestUS5_EntradaInvalidaSinCambioParcial` (subtests con `t.Run`
      para nombre vacío, fecha prevista anterior, factor fuera de rango y factor con más de dos
      decimales), `TestUS5_NoEsPropietario` (`403`) y `TestUS5_ProyectoFinalizado` (`409`) — depende
      de T056
- [ ] T064 [P] [US5] Pruebas de frontend en
      `frontend/src/features/projects/components/project-form.test.tsx` con
      `describe("002/US5 - Modificar los datos del proyecto")` y un `it` por escenario, sobre el
      mismo `ProjectForm` en modo edición — depende de T021

### Implementación de la Historia 5

- [ ] T065 [US5] Implementar `FindByID(ctx, id)` y `Update(ctx, project)` en
      `backend/internal/projects/repository/project_repository.go`, traduciendo
      `gorm.ErrDuplicatedKey` al error de nombre ya usado y actualizando `name`, `name_normalized`,
      `description`, `start_date`, `planned_end_date` y `hours_per_story_point_hundredths` en **una
      sola sentencia**, sin tocar `status` ni `actual_end_date` (FR-012, FR-015, FR-016) — depende
      de T037
- [ ] T066 [US5] Implementar la modificación en
      `backend/internal/projects/service/project_service.go`, reutilizando **la misma validación**
      que la creación (FR-013) y rechazando el proyecto `finished` antes de tocar nada (FR-018,
      RN-11) — depende de T022, T062 y T065
- [ ] T067 [US5] Implementar el manejador `updateProject` en
      `backend/internal/projects/delivery/handler.go` con anotaciones `swag` en español y
      registrarlo como `PUT /api/v1/projects/{projectId}` en el subgrupo de administración de
      `backend/internal/projects/delivery/routes.go`, devolviendo `200` con `ProjectDetail`, `400`,
      `401`, `403`, `404` y `409`. El campo `actualEndDate` que llegue en el cuerpo se **ignora**
      (FR-040) — depende de T059 y T066
- [ ] T068 [US5] Implementar la llamada de modificación en
      `frontend/src/features/projects/api.ts` y el modo edición del `ProjectForm` en
      `frontend/src/features/projects/components/project-form.tsx` — depende de T060
- [ ] T069 [US5] Crear la pantalla `frontend/src/app/projects/[projectId]/edit/page.tsx`, accesible
      solo al propietario — depende de T068

**Checkpoint**: US1 a US5 funcionan de forma independiente.

---

## Fase 8: Historia de Usuario 6 — Avanzar el estado del proyecto (Prioridad: P3)

**Objetivo**: el propietario recorre Planificado → En curso → Finalizado; al finalizar queda
registrada la fecha real, y la finalización se rechaza mientras haya un sprint activo.

**Rama**: `hu/002-us6-avanzar-el-estado`

**Prueba independiente**: creando un proyecto y recorriendo las dos transiciones válidas, verificando
la fecha de finalización real y el rechazo de las seis transiciones no permitidas. El rechazo por
sprint activo se prueba con un doble del puerto `SprintDirectory`.

### Pruebas de la Historia 6 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [ ] T070 [P] [US6] Pruebas unitarias **obligatorias** de la máquina de estados en
      `backend/internal/projects/domain/status_test.go`, cubriendo RN-09 y SC-009: las **dos**
      transiciones válidas (`planned → in_progress` e `in_progress → finished`) se aceptan y las
      **seis** restantes se rechazan — `planned → planned`, `planned → finished`,
      `in_progress → planned`, `in_progress → in_progress`, `finished → planned` y
      `finished → in_progress`
- [ ] T071 [P] [US6] Pruebas unitarias **obligatorias** de la finalización en
      `backend/internal/projects/service/status_service_test.go`, con un doble de `testify` del
      puerto `SprintDirectory`, cubriendo RN-10, RN-11 y RN-16: la fecha de finalización real se
      deriva de `time.Now().In(systemLocation)` y es la **fecha de calendario en la zona del
      sistema** —a las 23:30 hora del sistema la fecha es ese mismo día aunque en UTC ya sea el
      siguiente—; con sprint activo la finalización se rechaza y el proyecto sigue `in_progress`
      **sin** fecha real; y un proyecto `finished` rechaza todo cambio de estado sin alterar la
      fecha ya registrada (**SC-010**: todo proyecto finalizado tiene fecha real y ninguna cambia
      después de registrarse) — depende de T003 y T011
- [ ] T072 [US6] Pruebas de integración de la historia en
      `backend/internal/projects/delivery/handler_test.go`, una función por escenario precedida por
      `// Escenario: <nombre del escenario> (002/US6)`: `TestUS6_RecorridoCompleto` (las dos
      transiciones y la fecha real registrada), `TestUS6_FinalizacionAnticipada` (fecha real
      anterior a la prevista, sin ser un error), `TestUS6_FinalizadoSoloSeConsulta`,
      `TestUS6_FinalizaElMismoDiaQueEmpezo` (fecha real igual a `startDate`),
      `TestUS6_ArranqueSinCondiciones` (`planned → in_progress` sin exigir nada del backlog ni de
      los sprints), `TestUS6_RetrocesoRechazado`, `TestUS6_SaltoDeEstadoRechazado`,
      `TestUS6_MismoEstadoRechazado`, `TestUS6_FinalizadoEsSoloLectura`, `TestUS6_NoEsPropietario`
      (`403`) y `TestUS6_FinalizarConSprintActivo` (`409` y el proyecto sigue `in_progress`) —
      depende de T063
- [ ] T073 [P] [US6] Pruebas de frontend en
      `frontend/src/features/projects/components/status-actions.test.tsx` con
      `describe("002/US6 - Avanzar el estado del proyecto")` y un `it` por escenario, incluido que
      con el proyecto Finalizado la interfaz **oculta** las acciones de administración — depende
      de T021

### Implementación de la Historia 6

- [ ] T074 [US6] Implementar las transiciones permitidas en
      `backend/internal/projects/domain/status.go`: **únicamente** `planned → in_progress` e
      `in_progress → finished`; cualquier otra combinación se rechaza con el error de transición
      inválida, indicando la transición permitida (FR-037, FR-038) — depende de T008 y T070
- [ ] T075 [US6] Implementar el cambio de estado en
      `backend/internal/projects/service/status_service.go`: valida la transición con T074, consulta
      `SprintDirectory.FindActiveSprint` antes de finalizar y rechaza si hay uno activo (FR-041,
      RN-16), y al entrar en `finished` fija `actual_end_date` con la fecha de calendario del momento
      del cambio **en la zona horaria de `APP_TIMEZONE`** (FR-039). Ninguna operación modifica una
      `actual_end_date` ya registrada (FR-040, RN-10) — depende de T013, T065, T071 y T074
- [ ] T076 [US6] Implementar el manejador `changeProjectStatus` en
      `backend/internal/projects/delivery/handler.go` con anotaciones `swag` en español y
      registrarlo como `PATCH /api/v1/projects/{projectId}/status` en el subgrupo de administración
      de `backend/internal/projects/delivery/routes.go`, con `StatusChangeRequest` en
      `backend/internal/projects/delivery/dto.go` y las respuestas `200`, `400` (estado destino
      fuera de los tres valores), `401`, `403`, `404` y `409` con mensajes distintos para salto,
      retroceso, mismo estado, sprint activo y solo lectura — depende de T067 y T075
- [ ] T077 [US6] Implementar la llamada de cambio de estado en
      `frontend/src/features/projects/api.ts` y el `StatusActions` en
      `frontend/src/features/projects/components/status-actions.tsx`, que ofrece **solo** la
      transición disponible según el estado actual — depende de T068
- [ ] T078 [US6] Montar el `StatusActions` en
      `frontend/src/app/projects/[projectId]/page.tsx` y ocultar todas las acciones de
      administración cuando `readOnly` es `true`, mostrando en su lugar la fecha de finalización
      real (FR-018, escenario US4-2) — depende de T061 y T077

**Checkpoint**: las seis historias funcionan de forma independiente.

---

## Fase 9: Polish (Cierre y asuntos transversales)

**Propósito**: documentación viva, cobertura, vulnerabilidades y verificación de las puertas de
calidad. Todas las tareas son **técnicas**.

**Rama**: `fase/002-polish`

- [ ] T079 Regenerar la documentación con `swag init -g cmd/app/main.go -o docs` y verificar que
      `backend/docs/` coincida con [contracts/openapi.yaml](./contracts/openapi.yaml) en las seis
      rutas, sus códigos de respuesta y los catorce esquemas; corregir las anotaciones si difieren
      (Principio VIII) — depende de T076
- [ ] T080 [P] Actualizar `README.md` de la raíz en español documentando la variable `APP_TIMEZONE`
      y el paso a paso para usar la feature, manteniendo el criterio de aceptación de que alguien
      que no conoce el proyecto pueda ejecutarlo siguiendo el documento — depende de T003
- [ ] T081 [P] Revisar que `.gitignore` siga excluyendo `.env`, `.env.test`, `node_modules/`,
      `.next/`, `backend/docs/`, `coverage.out` y `coverage.html`, y que `backend/.env.example` esté
      sincronizado con `backend/.env` incluida la clave `APP_TIMEZONE`, sin secretos versionados
      (Principios VI y IX) — depende de T002
- [ ] T082 Ejecutar `govulncheck ./...` en `backend/` y resolver o justificar por escrito cada
      vulnerabilidad reportada
- [ ] T083 [P] Ejecutar los comandos de cobertura de los dos lados —
      `go test ./... -coverprofile=coverage.out` más `go tool cover -func=coverage.out` en
      `backend/`, y `npm run test:coverage` en `frontend/` — y revisar que el módulo
      `internal/projects/` esté cubierto
- [ ] T084 Verificar la trazabilidad y las puertas de seguridad recorriendo
      `backend/internal/projects/` y `frontend/src/features/projects/`: que buscar `002/US1`
      a `002/US6` en el código de pruebas devuelva los **55** escenarios; que las **16** reglas
      RN-01 a RN-16 tengan prueba (SC-012); que los **seis** endpoints tengan su prueba de `401`
      (SC-004); que el `404` de proyecto inexistente y de proyecto ajeno sea idéntico en **todas**
      las acciones (SC-005); que las **cuatro** acciones de administración devuelvan `403` a un
      integrante que no es propietario (SC-006); y que ninguna respuesta incluya `password_hash` ni
      el correo de los integrantes para quien no es el propietario — depende de T072
- [ ] T085 Ejecutar de punta a punta la [guía de validación](./quickstart.md) completa, incluidas la
      comprobación de la zona horaria en `actual_end_date`, las pruebas de concurrencia y la
      verificación de que las pruebas corrieron contra `smye_test` en el puerto 5433 y no contra
      desarrollo — depende de T079, T080 y T084

---

## Dependencias y Orden de Ejecución

### Dependencias entre fases

- **Fase 1 Setup**: depende de que la **feature 001 esté implementada e integrada en `main`** (T001).
- **Fase 2 Foundational**: depende de la Fase 1. **BLOQUEA** todas las historias.
- **Fases 3 a 8 (historias)**: todas dependen de la Fase 2, en orden de prioridad
  (P1 → P1 → P2 → P2 → P3 → P3).
- **Fase 9 Polish**: depende de que estén completas las historias que se quieran entregar.

### Dependencias entre historias

A diferencia de la feature 001, acá las historias **no son todas independientes entre sí**: el
middleware de membresía y el de propiedad son infraestructura compartida que aparece donde primero
se la necesita.

- **US1 Crear un proyecto (P1)**: puede empezar después de la Fase 2. Sin dependencias de otras
  historias.
- **US2 Ver mis proyectos (P1)**: necesita el repositorio de US1 (T024) para tener proyectos que
  listar, y **aporta** el middleware de membresía (T036) que usan US3 a US6.
- **US3 Gestionar integrantes (P2)**: necesita el middleware de membresía de US2 (T036) y **aporta**
  el middleware de propiedad (T049) y la lista de integrantes (T048) que usan US4 a US6.
- **US4 Consultar el estado (P2)**: necesita la lista de integrantes de US3 (T048) y el cálculo de
  horas de US1 (T023).
- **US5 Modificar el proyecto (P3)**: necesita el middleware de propiedad de US3 (T049), la
  validación de US1 (T022) y el `ProjectDetail` de US4 (T059).
- **US6 Avanzar el estado (P3)**: necesita el middleware de propiedad de US3 (T049), el `Update` de
  US5 (T065) y el puerto de sprints de la Fase 2 (T013).

Cada historia **se prueba** de forma independiente, aunque su implementación se apoye en piezas de
las anteriores: por eso US2 prueba el `404` montando el middleware sobre un manejador de prueba, y
US4 y US6 usan dobles de los puertos en lugar de esperar a las features 003 y 004.

### Dentro de cada historia

- Las pruebas se escriben y **fallan** antes de la implementación (Principio II).
- Dominio antes que repositorio; repositorio antes que servicio; servicio antes que `delivery`.
- Backend antes que la pantalla de frontend que lo consume.
- Una historia se termina antes de pasar a la siguiente prioridad.

### Oportunidades de paralelismo

- Fase 1: T002, T004 y T005 en paralelo (T003 depende de T002).
- Fase 2: T006, T007, T008 y T009 en paralelo; después T012 y T013 en paralelo.
- Dentro de cada historia: las pruebas unitarias, las de concurrencia y las de frontend marcadas
  `[P]` en paralelo entre sí. Las de integración **no** son paralelizables entre historias porque
  las seis comparten `handler_test.go`.
- Fase 9: T080, T081 y T083 en paralelo.
- Con varias personas: el reparto por historia es posible, pero el orden
  US1 → US2 → US3 → {US4, US5, US6} refleja la dependencia real de los middlewares.

---

## Ejemplo de paralelismo: Historia de Usuario 1

```text
# Las cuatro tareas de pruebas marcadas [P] pueden escribirse a la vez:
T017  backend/internal/projects/delivery/validation_test.go
T018  backend/internal/projects/service/estimation_test.go
T020  backend/internal/projects/delivery/project_concurrency_test.go
T021  frontend/src/features/projects/components/project-form.test.tsx

# T019 (handler_test.go) va aparte: ese archivo lo comparten las seis historias.

# En la implementación, estas dos no se pisan:
T028  frontend/src/types/project.ts + frontend/src/utils/project-status.ts
T023  backend/internal/projects/service/estimation.go
```

---

## Estrategia de Implementación

### MVP primero (solo US1)

1. Verificar el prerrequisito: feature 001 en `main` (T001).
2. Completar la Fase 1 (Setup).
3. Completar la Fase 2 (Foundational) — **CRÍTICO**: bloquea todas las historias.
4. Completar la Fase 3 (US1 Crear un proyecto).
5. **DETENERSE y VALIDAR**: probar US1 de forma independiente con la guía de validación.
6. Abrir el pull request de la historia con `/redactar-pr` y demostrar el incremento.

### Entrega incremental

1. Setup + Foundational → base lista (dos fases técnicas, dos pull requests).
2. US1 Crear un proyecto → probar sola → entregar (**MVP**).
3. US2 Ver mis proyectos → probar sola → entregar (con esto el aislamiento entre proyectos queda
   activo, que es la premisa del sistema multiusuario).
4. US3 Gestionar integrantes → probar sola → entregar.
5. US4 Consultar el estado → probar sola → entregar.
6. US5 Modificar el proyecto → probar sola → entregar.
7. US6 Avanzar el estado → probar sola → entregar.
8. Polish → cerrar la feature.

Cada historia añade valor sin romper las anteriores.

### Estrategia con varias personas

1. El equipo completa Setup y Foundational en conjunto.
2. US1 y US2 en secuencia: US2 aporta el middleware de membresía del que dependen las cuatro
   historias restantes.
3. US3 a continuación: aporta el middleware de propiedad y la lista de integrantes.
4. Terminada US3, US4, US5 y US6 pueden avanzar en paralelo, con la salvedad de que US6 necesita el
   `Update` de US5 (T065) y que las tres tocan `handler_test.go` y `handler.go`, así que conviene
   coordinar esos dos archivos.

---

## Notas

- Las tareas `[P]` usan archivos distintos y no tienen dependencias pendientes.
- La etiqueta `[USn]` traza la tarea hasta su historia; buscar `002/US1` a `002/US6` en el código de
  pruebas devuelve la trazabilidad completa de los 55 escenarios exigida por el Principio II.
- Cada tarea se ejecuta con `/speckit-implement Txxx`, una por invocación.
- Cada commit corresponde a una sola tarea y lleva el pie `Spec: 002/Txxx` más `Closes #N` o
  `Refs #N` (Principio IX, ámbito b). Los commits los registra **una persona**, nunca un agente.
- Las pruebas corren siempre contra `smye_test` en el puerto 5433, jamás contra desarrollo
  (Principio V).
- Esta feature **no agrega ninguna dependencia** de Go ni de npm. Si alguna tarea parece necesitar
  una, detenerse y pedir autorización (Principio IV).
- Tres puntos del plan esperan decisión del desarrollador: el prerrequisito de la feature 001
  (T001), el accesor de identidad que la 001 debe exportar (T005) y la exposición aceptada por RC-03
  en la respuesta `422` del alta de integrantes (T052), que conviene pasar por
  `/revisar-seguridad`.
- Las implementaciones de los dos puertos (T012 y T013) se reemplazan cuando lleguen
  `specs/003-product-backlog` y `specs/004-sprint-management`: conviene anotarlo al planificar esas
  features, porque es su punto de integración con esta.
- **Verificaciones que esta feature no puede cerrar sola.** Tres requisitos se prueban acá con
  dobles o asertando que no se toca ninguna otra tabla, y su verificación de punta a punta queda
  para las features que producen esos datos. Hay que anotarlo al planificar la 003 y la 004:

  | Requisito | Cómo se verifica hoy | Quién la cierra |
  | --- | --- | --- |
  | FR-017, RN-15 — los sprints cerrados conservan su factor | T062: el cambio de factor no toca ninguna otra tabla | `specs/004-sprint-management`, en su instantánea de cierre |
  | RN-14, SC-011 — la baja de un integrante no pierde registros | T044: la baja toca solo `project_members` | `specs/006-effort-tracking`, `specs/005-planning-poker` y `specs/007-defect-tracking` |
  | FR-042, FR-045 — reparto real del backlog y horas | T055: dobles de los puertos con valores distintos de cero | `specs/003-product-backlog` |

- **Criterios de éxito no automatizables.** SC-001 (primer proyecto en menos de 2 minutos, 95 % al
  primer intento) y SC-002 (sumar un integrante en menos de 30 segundos y en no más de 3 pasos) son
  métricas de experiencia de uso: no tienen tarea asociada porque ninguna prueba automatizada puede
  hacerlas fallar. Se validan a mano siguiendo la sección "Flujo en la interfaz" de
  [quickstart.md](./quickstart.md), cuyos pasos 2 y 4 están escritos justamente para medirlos.
