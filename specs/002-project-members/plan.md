# Plan de Implementación: Gestión de Proyectos e Integrantes

**Rama**: `002-project-members` | **Fecha**: 2026-10-07 | **Spec**: [spec.md](./spec.md)

**Entrada**: Especificación de feature en `specs/002-project-members/spec.md`

---

## Resumen

Esta feature permite crear proyectos, listarlos, modificarlos, hacer avanzar su estado, administrar
sus integrantes y consultar en una sola pantalla cómo va el proyecto. El proyecto es el ámbito al
que pertenecen el backlog, los sprints, el esfuerzo, los defectos y las métricas de las ocho
features siguientes: sin proyectos con integrantes definidos no hay a qué referir ninguna medición
ni cómo decidir quién ve o registra qué (FR-046 a FR-051, RN-02, RN-03).

El enfoque técnico tiene cuatro decisiones que lo definen:

1. **El aislamiento entre proyectos es estructural, no una convención.** FR-049 y SC-005 exigen que
   un proyecto ajeno sea indistinguible de uno inexistente. Se resuelve con un middleware de
   membresía que hace **una sola consulta** y tiene **una sola salida `404`** para los dos casos; un
   segundo middleware agrega el `403` del propietario. Así el orden —`404` antes que `403`— es una
   propiedad de la cadena de middlewares y no algo que haya que recordar en seis manejadores.
2. **El factor de horas por Story Point se almacena y se calcula en centésimas enteras.** RN-07
   prohíbe redondear y el supuesto de la spec pide usarlo "sin redondeos intermedios"; un `float64`
   no puede garantizarlo. El campo se recibe como `json.Number` para poder contar los decimales y
   rechazar `6.555` sin redondear, algo imposible si se deserializara a `float64`. No hace falta
   ninguna librería decimal, que además no está autorizada.
3. **Unicidad y no duplicación delegadas a la base de datos.** FR-011 y SC-007 (10 creaciones
   simultáneas con el mismo nombre) y FR-023 y SC-008 (10 altas simultáneas del mismo integrante) se
   resuelven con índices únicos compuestos y traduciendo `gorm.ErrDuplicatedKey`; un "consultar y
   después insertar" tendría la condición de carrera que esos criterios miden.
4. **Dos puertos hacia el backlog y los sprints.** FR-041 a FR-045 piden informar el sprint activo y
   el reparto del backlog, y rechazar la finalización mientras haya un sprint abierto. Esos datos los
   producen las features 003 y 004, que no existen todavía, así que `domain/` declara dos puertos con
   exactamente las operaciones que la spec necesita y las implementaciones actuales informan lo que
   hoy es cierto: ningún proyecto tiene historias ni sprints.

A diferencia de la 001, esta feature **no incluye arranque de proyecto**: reutiliza el módulo de Go,
la aplicación Next.js, los contenedores de PostgreSQL, la tabla `users` y el middleware de sesión ya
construidos. Agrega un módulo, dos tablas, seis endpoints, cuatro pantallas y **una** variable de
entorno.

Las decisiones y las alternativas descartadas están en [research.md](./research.md).

---

## Contexto Técnico

**Lenguaje y versión**: Go 1.27.1 (backend); TypeScript sobre Node.js 24.21.0 (frontend)

**Dependencias principales**: sin cambios respecto de la feature 001 — Gin v1.12.0, GORM v1.31.2 con
driver `postgres` v1.6.3, `google/uuid` v1.6.0, `gin-contrib/cors` v1.7.9, `swaggo/swag` v1.16.6 con
`gin-swagger` v1.6.1 y `swaggo/files` v1.0.1; Next.js 16.3.8 con React 19.3.0 y Tailwind CSS 4.3.3.
**Esta feature no agrega ninguna dependencia** ([research.md](./research.md), sección 1)

**Almacenamiento**: PostgreSQL, imagen `postgres:18.6-alpine`, en los mismos contenedores disjuntos
de desarrollo (puerto 5432, base `smye_dev`) y de test (puerto 5433, base `smye_test`) que creó la
001. Dos tablas nuevas: `projects` y `project_members`

**Pruebas**: `testify` v1.12.1 y la biblioteca estándar de Go (`net/http/httptest`) en el backend;
Vitest 5.0.3 con `@testing-library/react` 16.3.3 y jsdom 30.1.2 en el frontend

**Plataforma objetivo**: API HTTP en Linux (contenedor) y navegador moderno; desarrollo en Windows

**Tipo de proyecto**: aplicación web — backend de API y frontend separados en directorios de primer
nivel

**Objetivos de rendimiento**: SC-003 fija que la vista de estado se obtiene en **menos de 3
segundos** y en una sola pantalla. Es el único objetivo de rendimiento de la spec y el diseño lo
cumple resolviendo esa vista en un número acotado de consultas —proyecto, integrantes con un `JOIN`,
resumen del backlog y sprint activo— sin ningún N+1. El listado de proyectos sigue el mismo criterio:
una consulta con `JOIN` y subconsulta de conteo. **No se fija una meta de peticiones por segundo**:
inventarla sería alcance no solicitado (Principio I)

**Restricciones**: proyecto ajeno indistinguible de inexistente en el 100 % de las acciones (FR-049,
SC-005); cero proyectos con nombre repetido por propietario con 10 solicitudes simultáneas (SC-007);
cero membresías duplicadas con 10 altas simultáneas (SC-008); factor sin redondeos con hasta 2
decimales (FR-006, RN-07); fecha de finalización real derivada de una zona horaria única y
configurable, e inmodificable después (FR-039, FR-040, SC-010); un proyecto Finalizado es de solo
lectura (FR-018); quitar a un integrante no pierde ni un registro (FR-027, SC-011)

**Escala y alcance**: 6 endpoints HTTP, 2 entidades persistidas, 2 puertos hacia otras features, 4
pantallas de frontend (listado, creación, estado y edición), 6 historias de usuario con **55**
escenarios de aceptación y 16 reglas de negocio

### Incógnitas resueltas en la Fase 0

No queda ningún `NEEDS CLARIFICATION`. Las cinco preguntas abiertas de la spec se habían resuelto en
la sesión de clarificación del 2026-09-25; las incógnitas **técnicas** se resolvieron en
[research.md](./research.md): unicidad del nombre por propietario, precisión del factor, fechas de
calendario y zona horaria, el `404` indistinguible, el ciclo de la membresía, la fuente de verdad de
la propiedad, la lectura de la tabla `users` desde este módulo, los puertos hacia backlog y sprints,
la identidad del titular entre módulos, el orden del listado, los códigos de estado HTTP y la
organización en módulos.

### Puntos señalados al desarrollador (no bloquean el diseño)

1. **La feature 001 no está implementada.** Hoy el repositorio no tiene `backend/` ni `frontend/`:
   solo existen `specs/`, `.specify/`, `.claude/`, `.github/`, `CLAUDE.md` y `README.md`. Esta
   feature **no puede empezar** hasta que `specs/001-user-auth/tasks.md` esté completo e integrado
   en `main`. Es un prerrequisito de orden, no un problema de diseño.
2. **Variable de entorno nueva**: `APP_TIMEZONE` (por ejemplo `America/Argentina/Buenos_Aires`) hay
   que agregarla a `backend/.env.example`, `backend/.env` y `backend/.env.test`. Se valida al
   arrancar: si falta o no es una zona válida, la aplicación no arranca. Detalle en
   [research.md](./research.md), sección 4.
3. **Accesor de identidad en `auth/delivery`**: este módulo lee el titular de la sesión a través de
   una función exportada de la feature 001. Si la 001 quedó con la clave del contexto sin exportar,
   esta feature agrega el accesor allí en lugar de repetir el literal en otro módulo, que es la
   variante frágil. Detalle en [research.md](./research.md), sección 10.
4. **Las implementaciones de los dos puertos se reemplazan** cuando lleguen
   `specs/003-product-backlog` y `specs/004-sprint-management`. Conviene anotarlo al planificar esas
   features, porque es su punto de integración con esta. Detalle en
   [research.md](./research.md), sección 9.
5. **RC-03 es una exposición aceptada por la spec**: la respuesta `422` del alta de integrantes
   confirma si un correo está registrado en el sistema. La spec la acepta de forma explícita y
   razonada, y la 001 mantiene su mensaje genérico en el inicio de sesión. **Conviene que lo repase
   `/revisar-seguridad`** junto con el resto de la feature.

---

## Verificación de la Constitución

*PUERTA: debe pasar antes de la investigación de la Fase 0. Reevaluada después del diseño de la
Fase 1.*

### Primera evaluación (antes de la Fase 0)

| Principio | Puerta | Resultado |
| --- | --- | --- |
| I — Simplicidad deliberada | ¿El alcance se limita a lo pedido? | **PASA**. Los 6 endpoints se corresponden uno a uno con las 6 historias. Sin invitaciones por correo, sin transferencia de propiedad, sin roles, sin eliminación ni archivado, sin búsqueda ni paginación, sin auditoría: todo eso está en "Fuera de Alcance". |
| II — TDD | ¿Hay forma de escribir la prueba antes del código? | **PASA**. Los 55 escenarios son pruebas nombradas; el entorno de test ya existe desde la 001. Las validaciones, las 16 reglas de negocio y el cálculo de horas son pruebas unitarias obligatorias. |
| III — Arquitectura por capas | ¿La estructura cabe en el layout obligatorio? | **PASA**. Un módulo `internal/projects/` con sus cuatro capas; el frontend usa las capas ya existentes. |
| IV — Stack fijo y versiones estables | ¿Hace falta algo fuera de la tabla? | **PASA**. Ninguna dependencia nueva; la aritmética exacta se resuelve con enteros y la zona horaria con la biblioteca estándar. |
| V — Contenerización | ¿Dev y test son disjuntos? | **PASA**. Se reutilizan los contenedores de la 001, ya separados por puerto, base, volumen y archivo de entorno. Esta feature no agrega servicios externos. |
| VI — Seguridad y secretos | ¿Hay secretos fuera de `.env`? | **PASA**. La única variable nueva, `APP_TIMEZONE`, vive en los `.env` con su `.env.example` sincronizado. Esta feature no maneja credenciales. |
| VII — Robustez y validación | ¿Toda entrada se valida antes del servicio? | **PASA**. Validación en `delivery`, incluidos el conteo de decimales del factor y el parseo de fechas, para que ningún error llegue sin manejar. |
| VIII — Estándares y documentación viva | ¿Swagger, README e idiomas? | **PASA**. Identificadores y valores de estado en inglés, comentarios, mensajes y Swagger en español, README actualizado en el mismo cambio. |
| IX — Gobierno del repositorio | ¿El plan pide operaciones prohibidas? | **PASA**. El plan no ejecuta ninguna escritura de Git; los commits los hace una persona. |

**Resultado: todas las puertas pasan. Sin violaciones que justificar.**

### Reevaluación (después del diseño de la Fase 1)

| Principio | Qué se revisó en el diseño | Resultado |
| --- | --- | --- |
| I | Dos tablas y ninguna columna sin requisito que la pida. Se descartaron `is_owner` y `joined_at` en la membresía por duplicar datos que ya están en `projects.owner_id` y en `created_at`. No se agregó un endpoint de lista de integrantes porque la vista de estado ya la incluye. Las interfaces de repositorio declaran 11 operaciones, todas usadas; sin CRUD especulativo y sin `Delete` de proyecto, que RC-05 descarta. | **PASA** |
| II | Los 55 escenarios se mapean a pruebas nombradas (tabla de trazabilidad más abajo). Las pruebas unitarias obligatorias son las de validaciones, las de las reglas RN-01 a RN-16 y **las del cálculo de horas estimadas**, que el Principio II exige por ser estimación. Los dos puertos son lo que hace verificable FR-045 con valores distintos de cero. | **PASA** |
| III | `domain/` no importa a ninguna otra capa. El módulo no importa `auth/service` ni `auth/repository`: sus dos contactos con la 001 son el accesor de identidad de `auth/delivery` y la tabla `users` leída por su propio adaptador. El middleware de sesión se inyecta desde `cmd/app/`, que es su trabajo. | **PASA** |
| IV | Ninguna dependencia nueva de Go ni de npm. La aritmética decimal se resuelve con enteros en lugar de `shopspring/decimal`, que no está autorizada; la zona horaria con `time.LoadLocation` y `time/tzdata`, ambos de la biblioteca estándar. | **PASA** |
| V | Sin servicios nuevos. Las dos tablas se crean con `AutoMigrate` en las mismas bases disjuntas; el ayudante de pruebas de la 001 se extiende para vaciarlas. | **PASA** |
| VI | `APP_TIMEZONE` validada al arrancar. El adaptador de lectura de `users` **nunca selecciona `password_hash`**, y el tipo `UserRef` del módulo no tiene ningún campo de contraseña: el hash no entra a este módulo ni como columna ni en memoria. El correo de los integrantes se omite para quien no es propietario. | **PASA** |
| VII | Los seis endpoints validan antes de la capa de servicio. `gorm.ErrDuplicatedKey` se traduce a `409` en lugar de propagarse como error interno. El factor se valida sobre su texto literal y las fechas se parsean con error manejado. Sin `panic` en el camino de ninguna petición. | **PASA** |
| VIII | El contrato OpenAPI está en español; las anotaciones `swag` también. Identificadores en inglés en los dos lenguajes, con los valores de estado `planned`/`in_progress`/`finished` y las etiquetas en español en el frontend. `any` prohibido: los tipos del frontend son explícitos. | **PASA** |
| IX | Los artefactos generados son archivos de la copia de trabajo. No hay ninguna operación de Git en el plan. | **PASA** |

**Resultado: todas las puertas siguen pasando después del diseño. La sección de seguimiento de
complejidad queda vacía porque no hay ninguna violación que justificar.**

#### Tres decisiones que la constitución obliga a documentar

Ninguna es una violación, pero las tres son concesiones conscientes y quedan registradas:

1. **Dos columnas de nombre** (`name` y `name_normalized`) donde la feature 001 usó una sola para el
   correo. No es duplicación ociosa: acá el nombre **se muestra** y la persona espera leer la
   capitalización que escribió, mientras que el correo normalizado de la 001 sí es el que se muestra.
   La alternativa, un índice único por expresión, exige un `CREATE UNIQUE INDEX` en crudo porque
   `AutoMigrate` no lo declara de forma portable: una pieza móvil más para ahorrar una columna
   ([research.md](./research.md), sección 2).
2. **Dos puertos con implementaciones que informan vacío** hasta que existan las features 003 y 004.
   No es infraestructura "por si acaso": FR-041 a FR-045 están en esta spec, con escenarios que los
   prueban, y sin los puertos FR-041 y RN-16 no se pueden implementar ni el escenario US4-5 escribir.
   Lo que informan hoy es lo que hoy es cierto ([research.md](./research.md), sección 9).
3. **El módulo de proyectos lee la tabla `users` de la feature 001** a través de un adaptador propio
   de solo lectura, en lugar de reusar `auth/domain.UserRepository`. Esa interfaz devuelve un tipo que
   incluye el hash de la contraseña y solo busca de uno en uno, lo que forzaría un N+1 en la lista de
   integrantes. Concentrar el acceso en un archivo que nunca selecciona `password_hash` deja la regla
   auditable con una sola lectura ([research.md](./research.md), sección 8).

---

## Estructura del Proyecto

### Documentación de esta feature

```text
specs/002-project-members/
├── spec.md              # Especificación (entrada)
├── plan.md              # Este archivo (salida de /speckit-plan)
├── research.md          # Salida de la Fase 0
├── data-model.md        # Salida de la Fase 1
├── quickstart.md        # Salida de la Fase 1
├── contracts/
│   └── openapi.yaml     # Salida de la Fase 1
├── checklists/
│   └── requirements.md  # Checklist de calidad de la spec
└── tasks.md             # Salida de /speckit-tasks — NO lo crea /speckit-plan
```

### Código fuente (raíz del repositorio)

Las líneas marcadas con `NUEVO` las crea esta feature; las marcadas con `TOCA` son archivos de la
feature 001 que esta feature modifica de forma puntual. Todo lo demás existe y no se toca.

```text
backend/
├── cmd/
│   └── app/
│       └── main.go                         # TOCA: compone el módulo de proyectos e importa time/tzdata
├── internal/
│   ├── auth/                               # Feature 001 — sin cambios, salvo:
│   │   └── delivery/
│   │       └── middleware.go               # TOCA: exporta el accesor del titular de la sesión
│   ├── projects/                           # NUEVO: módulo único de esta feature
│   │   ├── domain/
│   │   │   ├── project.go                  # Entidad Project
│   │   │   ├── project_member.go           # Entidad ProjectMember
│   │   │   ├── status.go                   # ProjectStatus y las transiciones permitidas
│   │   │   ├── errors.go                   # Errores de dominio (nombre tomado, membresía, permisos, solo lectura)
│   │   │   ├── repository.go               # Interfaces de repositorio (solo lo que se usa)
│   │   │   └── ports.go                    # UserDirectory, BacklogSummaryProvider, SprintDirectory
│   │   ├── repository/
│   │   │   ├── project_repository.go       # GORM; traduce ErrDuplicatedKey; creación transaccional
│   │   │   ├── project_member_repository.go# GORM; alta, baja y lista con JOIN a users
│   │   │   ├── user_directory.go           # Única lectura de `users`; nunca selecciona password_hash
│   │   │   ├── backlog_summary.go          # Implementación del puerto de backlog
│   │   │   └── sprint_directory.go         # Implementación del puerto de sprints
│   │   ├── service/
│   │   │   ├── project_service.go          # Crear, modificar, listar y vista de estado
│   │   │   ├── status_service.go           # Transiciones y fecha de finalización real
│   │   │   ├── member_service.go           # Alta y baja de integrantes
│   │   │   └── estimation.go               # Horas estimadas = Story Points × factor (centésimas)
│   │   └── delivery/
│   │       ├── handler.go                  # 6 manejadores HTTP con anotaciones swag en español
│   │       ├── dto.go                      # Peticiones y respuestas; el correo se omite si no es propietario
│   │       ├── validation.go               # Validación y normalización de entrada
│   │       ├── middleware.go               # RequireMembership (404) y RequireOwnership (403)
│   │       └── routes.go                   # Rutas públicas ninguna; todas protegidas
│   └── platform/
│       └── config/
│           └── config.go                   # TOCA: lee y valida APP_TIMEZONE
├── postgres/                               # Sin cambios
├── .env.example                            # TOCA: agrega APP_TIMEZONE
├── .env                                    # TOCA (no versionado)
├── .env.test                               # TOCA (no versionado)
├── Dockerfile                              # Sin cambios
├── go.mod                                  # Sin cambios: ninguna dependencia nueva
└── go.sum                                  # Sin cambios

frontend/
├── src/
│   ├── app/
│   │   ├── projects/
│   │   │   ├── page.tsx                    # NUEVO: US2 listado
│   │   │   ├── new/page.tsx                # NUEVO: US1 creación
│   │   │   └── [projectId]/
│   │   │       ├── page.tsx                # NUEVO: US3, US4 y US6 vista de estado
│   │   │       └── edit/page.tsx           # NUEVO: US5 modificación
│   │   └── profile/page.tsx                # TOCA: enlace al listado de proyectos
│   ├── features/
│   │   └── projects/                       # NUEVO
│   │       ├── components/                 # ProjectList, ProjectForm, ProjectStatusCard,
│   │       │                               # MemberList, AddMemberForm, StatusActions
│   │       ├── api.ts                      # Llamadas a los 6 endpoints
│   │       └── validation.ts               # Reglas de nombre, descripción, fechas y factor en el formulario
│   ├── types/
│   │   └── project.ts                      # NUEVO: tipos de peticiones y respuestas; sin `any`
│   └── utils/
│       └── project-status.ts               # NUEVO: etiquetas en español de los tres estados
├── Dockerfile                              # Sin cambios
└── package.json                            # Sin cambios: ninguna dependencia nueva

README.md                                   # TOCA: documenta APP_TIMEZONE
```

**Pruebas**: en el backend, junto al código del paquete que prueban, según la convención de Go
(`project_service_test.go`, `handler_test.go`, `validation_test.go`, `status_test.go`,
`estimation_test.go`, `middleware_test.go`). En el frontend, junto al componente o módulo
(`project-form.test.tsx`). No se crea un árbol `tests/` separado.

**Decisión de estructura**: se mantiene la aplicación web con `backend/` y `frontend/` como
directorios de primer nivel (Principio III). El backend agrega **un** módulo vertical,
`internal/projects/`, con sus cuatro capas y las dependencias apuntando hacia `domain/`. Se eligió un
único módulo en lugar de separar `projects` y `members` porque la membresía no tiene sentido sin el
proyecto y ninguna funcionalidad pedida atraviesa esa frontera ([research.md](./research.md),
sección 13). El frontend no agrega capas: usa las diez que ya declara el Principio III.

---

## Trazabilidad de escenarios a pruebas (Principio II)

Cada escenario Given/When/Then de la spec se implementa como al menos una prueba cuyo nombre
identifica la historia y el escenario. Buscar `002/US1` a `002/US6` en el código de pruebas devuelve
la trazabilidad completa.

**Backend (Go)**: función `TestUSn_<Escenario>` en el paquete del módulo, precedida por el
comentario `// Escenario: <nombre del escenario en spec.md> (002/USn)`. Las variantes de un mismo
escenario se agrupan como subtests con `t.Run`.

**Frontend (Vitest)**: `describe("002/USn - <título de la historia>")` con un
`it("<nombre del escenario en spec.md>")` por escenario.

| Historia | Escenarios | Pruebas de backend previstas |
| --- | --- | --- |
| US1 Crear un proyecto | 11 | `TestUS1_CreacionExitosa`, `TestUS1_SinDescripcion`, `TestUS1_MismoNombreOtroPropietario`, `TestUS1_FechasIguales`, `TestUS1_LimitesDelFactor`, `TestUS1_LimitesDelNombre`, `TestUS1_CamposFaltantes`, `TestUS1_FactorInvalido`, `TestUS1_FechaPrevistaAnterior`, `TestUS1_NombreRepetido`, `TestUS1_SinSesion` |
| US2 Ver mis proyectos | 6 | `TestUS2_ListadoCompleto`, `TestUS2_ListadoVacio`, `TestUS2_IntegranteQuitadoNoVeElProyecto`, `TestUS2_ProyectoConUnIntegrante`, `TestUS2_ProyectoAjenoEsInexistente`, `TestUS2_SinSesion` |
| US3 Gestionar integrantes | 11 | `TestUS3_AltaExitosa`, `TestUS3_CorreoConEspaciosYMayusculas`, `TestUS3_NuevaAltaTrasBaja`, `TestUS3_VisibilidadDelCorreo`, `TestUS3_SoloElPropietarioComoIntegrante`, `TestUS3_BajaConservaElHistorial`, `TestUS3_CorreoSinCuenta`, `TestUS3_YaEsIntegrante`, `TestUS3_NoEsPropietario`, `TestUS3_PropietarioNoPuedeQuitarse`, `TestUS3_ProyectoFinalizado` |
| US4 Consultar el estado | 7 | `TestUS4_EstadoCompleto`, `TestUS4_ProyectoFinalizado`, `TestUS4_SinSprintActivo`, `TestUS4_ProyectoReciénCreado`, `TestUS4_HorasEstimadas`, `TestUS4_NoIntegranteEsInexistente`, `TestUS4_SinSesion` |
| US5 Modificar el proyecto | 9 | `TestUS5_ModificacionExitosa`, `TestUS5_ProyectoEnCursoSeModifica`, `TestUS5_MismoNombreQueYaTenia`, `TestUS5_CambioDeFactorConSprintsCerrados`, `TestUS5_LimitesDelFactor`, `TestUS5_NombreDeOtroProyectoPropio`, `TestUS5_EntradaInvalidaSinCambioParcial`, `TestUS5_NoEsPropietario`, `TestUS5_ProyectoFinalizado` |
| US6 Avanzar el estado | 11 | `TestUS6_RecorridoCompleto`, `TestUS6_FinalizacionAnticipada`, `TestUS6_FinalizadoSoloSeConsulta`, `TestUS6_FinalizaElMismoDiaQueEmpezo`, `TestUS6_ArranqueSinCondiciones`, `TestUS6_RetrocesoRechazado`, `TestUS6_SaltoDeEstadoRechazado`, `TestUS6_MismoEstadoRechazado`, `TestUS6_FinalizadoEsSoloLectura`, `TestUS6_NoEsPropietario`, `TestUS6_FinalizarConSprintActivo` |

**Total: 55 escenarios.**

Además, pruebas unitarias **obligatorias** por el Principio II (validaciones, reglas de negocio y
estimación), que cubren SC-012:

| Objeto de prueba | Reglas cubiertas |
| --- | --- |
| Propietario e integrante en el mismo acto de creación | RN-01 |
| Clasificación de acciones de administración frente a consulta | RN-02, RN-03 |
| Validación y normalización del nombre del proyecto | RN-04, RN-05 |
| Normalización del correo del integrante | RN-05 |
| Orden de las fechas de inicio y de finalización prevista | RN-06 |
| Validación del factor: rango, precisión de 2 decimales y rechazo sin redondear | RN-07 |
| **Cálculo de horas estimadas en centésimas enteras** | RN-08 |
| Máquina de estados: las dos transiciones válidas y las seis inválidas | RN-09 |
| Fecha de finalización real: derivación con `APP_TIMEZONE` e inmutabilidad | RN-10 |
| Solo lectura del proyecto Finalizado en las cuatro acciones de administración | RN-11 |
| El propietario no puede quitarse | RN-12 |
| Membresía única por proyecto y usuario | RN-13 |
| La baja no alcanza a ningún registro fuera de `project_members` | RN-14 |
| El cambio de factor no toca los sprints cerrados | RN-15 |
| Rechazo de la finalización con sprint activo | RN-16 |

Comparación con los criterios de éxito: SC-004 se verifica con una prueba sin sesión por cada uno de
los 6 endpoints; SC-005 comparando byte a byte las respuestas `404` de identificador inexistente y de
proyecto ajeno **en cada acción**; SC-006 con las cuatro acciones de administración pedidas por un
integrante que no es propietario; SC-007 con `TestUS1_NombreRepetido` más la prueba de 10 creaciones
simultáneas; SC-008 con la prueba de 10 altas simultáneas del mismo usuario; SC-009 con las seis
transiciones inválidas; SC-010 con `TestUS6_RecorridoCompleto` y `TestUS6_FinalizadoEsSoloLectura`;
SC-011 con `TestUS3_BajaConservaElHistorial`; SC-012 con la tabla de arriba.

---

## Secuencia de trabajo prevista

El orden lo fija `/speckit-tasks`; esto es la dependencia entre bloques, no la lista de tareas.

0. **Prerrequisito**: la feature 001 implementada e integrada en `main`. Sin `backend/` ni
   `frontend/` no hay dónde escribir la primera prueba de esta feature.
1. **Fase técnica de arranque** (sin prueba asociada: configuración). Agregar `APP_TIMEZONE` a los
   tres archivos de entorno del backend y su lectura validada en `platform/config`; importar
   `time/tzdata` en `cmd/app/main.go`; exportar el accesor del titular de la sesión en
   `auth/delivery`; extender el ayudante de pruebas para vaciar `projects` y `project_members`.
2. **Fase técnica de dominio**: entidades `Project` y `ProjectMember`, el tipo `ProjectStatus`, los
   errores de dominio, las interfaces de repositorio, los tres puertos y **las implementaciones de
   los puertos de backlog y de sprints** que informan vacío, más `AutoMigrate` de las dos tablas. Es
   el bloque que bloquea a todas las historias. Las implementaciones de los dos puertos van acá, y
   no dentro de US4, porque no tienen lógica de negocio —devuelven el estado que hoy es cierto— y
   porque la respuesta de la creación de US1 ya incluye el resumen del backlog.
3. **US1 Crear un proyecto** (P1): validación, **cálculo de horas estimadas**, repositorio con
   creación transaccional de proyecto más membresía del propietario y traducción de
   `ErrDuplicatedKey`, servicio, endpoint, pantalla de creación. El cálculo aparece acá, antes de
   US4, porque la respuesta de la creación ya expone `estimatedHours`; US4 lo ejercita con valores
   distintos de cero.
4. **US2 Ver mis proyectos** (P1): middleware de membresía con su `404` único, consulta de listado
   con `JOIN` y conteo, endpoint, pantalla de listado.
5. **US3 Gestionar integrantes** (P2): adaptador de lectura de `users`, middleware de propiedad con
   su `403`, alta y baja, lista de integrantes con la visibilidad del correo, pantalla de
   administración dentro de la vista de estado.
6. **US4 Consultar el estado** (P2): armado completo de la vista con los dos puertos, endpoint de
   vista de estado, pantalla de estado, y la verificación de que la vista resuelve en un número
   acotado de consultas sin N+1 (SC-003).
7. **US5 Modificar el proyecto** (P3): validación reutilizada, actualización atómica, endpoint,
   pantalla de edición.
8. **US6 Avanzar el estado** (P3): máquina de transiciones, fecha de finalización real con la zona
   del sistema, verificación del sprint activo, endpoint, acciones de estado en la interfaz.
9. **Cierre**: Swagger comparado con el contrato, `README.md` con la variable nueva, comandos de
   cobertura, `govulncheck` y la verificación transversal de trazabilidad y puertas de seguridad
   (SC-004, SC-005, SC-006, SC-012).

En cada historia el ciclo es el del Principio II: escribir la prueba, **verificar que falla y
detenerse** para que el desarrollador registre el commit RED con `/redactar-commit`, implementar,
verificar que pasa, refactorizar. Un agente no avanza de RED a GREEN sin ese alto.

---

## Seguimiento de Complejidad

> Se completa SOLO si la verificación de la constitución tiene violaciones que justificar.

**Sin violaciones.** Las dos puertas de evaluación pasan en su totalidad. Las tres concesiones
documentadas —la columna `name_normalized`, los dos puertos hacia features que todavía no existen y
el adaptador propio de lectura de `users`— son consecuencias necesarias de requisitos de la spec
(FR-004, FR-041 a FR-045, FR-030) y no desvíos de la constitución; están justificadas en la sección
"Verificación de la Constitución" y en [research.md](./research.md).
