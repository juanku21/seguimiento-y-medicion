# Plan de Implementación: Gestión de Sprints

**Rama**: `004-sprint-management` | **Fecha**: 2026-10-21 | **Spec**: [spec.md](./spec.md)

**Entrada**: Especificación de feature en `specs/004-sprint-management/spec.md`

---

## Resumen

Esta feature permite al equipo organizar el trabajo del proyecto en sprints: crearlos con un objetivo
y un período, comprometer historias del backlog, iniciarlos, registrar el avance de sus historias,
cerrarlos congelando una instantánea y consultar el historial. Es el único lugar del producto donde
una historia pasa de planeada a terminada, y la instantánea que congela al cerrar es el dato del que
la 008 calculará velocidad y desvío, el 009 mostrará el sprint en curso y la 010 armará sus reportes.

El enfoque técnico tiene seis decisiones que lo definen:

1. **Tres formas de ruta para tres respuestas de "no acceso".** La colección cuelga del subgrupo del
   proyecto de la 002; las acciones sobre un sprint son planas con un middleware propio,
   `RequireSprintAccess`, con un solo `404`; el avance y el historial de una historia cuelgan de
   `/api/v1/stories/:storyId` y reutilizan `RequireStoryAccess` de la 003, inyectado desde
   `cmd/app/`.
2. **Una fila por compromiso, viva hasta el cierre y congelada después.** `sprint_stories` guarda el
   compromiso y la fecha de completado mientras el sprint está abierto; al cerrar se le completan
   título, prioridad, Story Points y resultado, y no vuelve a cambiar. El historial de una historia
   son sus filas.
3. **La instantánea copia lo irrecuperable y deriva las sumas.** Se copian los Story Points al
   iniciar, el factor del proyecto y los datos de cada historia al cerrar; los totales planificados y
   completados se suman de filas que ya no cambian.
4. **Las invariantes de calendario se protegen con un bloqueo del proyecto.** Crear, modificar,
   iniciar y eliminar toman `FOR UPDATE` sobre la fila del proyecto, de modo que la superposición y
   el único sprint Activo se verifican siempre sobre el último estado; dos índices únicos parciales
   son el respaldo en la base.
5. **"Estuvo alguna vez en un sprint" es una columna de `user_stories`.** Quitar una historia o
   eliminar el sprint borra su compromiso, pero la 003 necesita recordar el hecho para impedir
   eliminarla: `ever_committed` lo conserva.
6. **Esta feature cierra los dos puertos que la 002 y la 003 dejaron abiertos hacia los sprints**
   (`SprintDirectory` y `SprintAssignments`) y deja el suyo hacia la 006 (`SprintEffort`).

Esta feature **no tiene fase de arranque**: agrega un módulo, dos tablas, una columna, once
endpoints y cuatro pantallas, sin dependencias ni variables de entorno nuevas.

Las decisiones y las alternativas descartadas están en [research.md](./research.md).

---

## Contexto Técnico

**Lenguaje y versión**: Go 1.27.2 (backend); TypeScript sobre Node.js 24.21.0 (frontend)

**Dependencias principales**: sin cambios respecto de la 003 — Gin v1.12.0, GORM v1.31.2 con driver
`postgres` v1.6.3, `google/uuid` v1.6.0, `gin-contrib/cors` v1.7.9, `swaggo/swag` v1.16.6 con
`gin-swagger` v1.6.1 y `swaggo/files` v1.0.1; Next.js 16.3.8 con React 19.3.0 y Tailwind CSS 4.3.3.
**Esta feature no agrega ninguna dependencia** ([research.md](./research.md), sección 1)

**Almacenamiento**: PostgreSQL, imagen `postgres:18.6-alpine`, en los mismos contenedores disjuntos
de desarrollo (puerto 5432, base `smye_dev`) y de test (puerto 5433, base `smye_test`). Dos tablas
nuevas, `sprints` y `sprint_stories`, y una columna nueva, `user_stories.ever_committed`

**Pruebas**: `testify` v1.12.1 (incluido `testify/mock` para el doble del puerto de esfuerzo) y
`net/http/httptest` en el backend; Vitest 5.0.3 con `@testing-library/react` 16.3.3 y jsdom 30.1.2 en
el frontend

**Plataforma objetivo**: API HTTP en Linux (contenedor) y navegador moderno; desarrollo en Windows

**Tipo de proyecto**: aplicación web — backend de API y frontend separados en directorios de primer
nivel

**Objetivos de rendimiento**: la spec no fija tiempos de respuesta. SC-001 a SC-003 son tiempos de
tarea de la persona (planificar un sprint con cinco historias en menos de 3 minutos, completar una
historia en menos de 15 segundos y 3 pasos, ver el sprint activo en una pantalla), que dependen de la
interfaz: el detalle del sprint concentra objetivo, fechas, días restantes, historias y su estado, y
el avance es un control por fila. Del lado del servidor, el listado se resuelve en **una consulta** con
agregados por subconsulta y el detalle en **dos** (sprint con autores, e historias), sin N+1. **No se
fija una meta de peticiones por segundo**: sería alcance no solicitado (Principio I)

**Restricciones**: cero superposiciones (SC-004) y cero proyectos con dos sprints Activos bajo 10
inicios simultáneos (SC-005); 100 % de las transiciones no permitidas rechazadas (SC-006); admisión
exacta (SC-007); instantánea inmutable (SC-008, SC-009); devolución completa al backlog (SC-010);
avance solo dentro del sprint Activo (SC-011); proyecto Finalizado de solo lectura (SC-012); sesión y
membresía en el 100 % de las acciones (SC-013); `404` indistinguible (SC-014); ningún sprint con
esfuerzo eliminado (SC-016); ninguna acción cambia el estado del proyecto (FR-049)

**Escala y alcance**: 11 endpoints HTTP, 2 entidades persistidas más una columna en una entidad de la
003, 1 puerto propio y 2 puertos ajenos reemplazados, 4 pantallas de frontend (listado e historial,
creación, detalle con las acciones del ciclo de vida, y edición) más una sección en el detalle de la
historia de la 003, 7 historias de usuario con **82** escenarios de aceptación y 23 reglas de negocio

### Incógnitas resueltas en la Fase 0

No queda ningún `NEEDS CLARIFICATION`. Las decisiones de producto se cerraron en la sesión del
2026-09-25 y en la enmienda del 2026-10-02. Las incógnitas **técnicas** se resolvieron en
[research.md](./research.md): forma de las rutas y de los `404`, superposición y sprint Activo único
bajo concurrencia, persistencia del compromiso y de la instantánea, marca permanente de "estuvo en un
sprint", atomicidad del lote, restricción sobre historias comprometidas, listado e historial,
transiciones, avance, cierre, días restantes, eliminación con esfuerzo, integración con la 002 y la
003, solo lectura, atribución y códigos HTTP.

### Puntos señalados al desarrollador (no bloquean el diseño)

1. **La 003 no está implementada todavía.** Su plan y sus tareas están en la rama `repo/003-plan`;
   este plan usa los nombres de ese diseño (`internal/backlog/`, `user_stories`, `Readiness`,
   `RequireStoryAccess`, `SprintAssignments`). La implementación de la 004 **requiere la 003
   integrada en `main`**; si la 003 se aparta de su plan, se ajustan los nombres al implementar
   ([research.md](./research.md), sección 0).
2. **Rama de trabajo**: el plan se generó en la rama `repo/004-plan` y `.specify/feature.json`
   (ignorado por Git) ahora apunta a `specs/004-sprint-management`.
3. **Esta feature toca código de la 002 y de la 003**: reemplaza el cuerpo de
   `projects/repository/sprint_directory.go` y de `backlog/repository/sprint_assignments.go` (sus
   constructores pasan a recibir la conexión), agrega el campo `EverCommitted` a
   `backlogdomain.UserStory` y una sección al detalle de la historia en el frontend
   ([research.md](./research.md), secciones 7 y 17).
4. **Dos ventanas de carrera en código ajeno, no resueltas en este plan**: la 003 verifica si una
   historia está comprometida antes de tomar su bloqueo (sección 9) y la 002 verifica el sprint
   activo antes de finalizar el proyecto sin bloqueo (sección 17.1). Ambas se cierran moviendo la
   verificación dentro de la transacción que bloquea la fila; se recomienda tratarlas en
   `/revisar-seguridad` de esta feature.
5. **Lectura de FR-060**: se persisten el autor y quien cerró; quién inició y quién completó cada
   historia salen de la sesión pero no se guardan, igual que en la 002 y la 003
   ([research.md](./research.md), sección 19).
6. **Punto de integración para anotar al planificar la 006**: implementar `SprintEffort` y traducir
   la violación de la clave foránea de sus registros hacia `sprints.id` al mismo `409` de eliminación
   bloqueada ([research.md](./research.md), sección 16).

---

## Verificación de la Constitución

*PUERTA: debe pasar antes de la investigación de la Fase 0. Reevaluada después del diseño de la
Fase 1.*

### Primera evaluación (antes de la Fase 0)

| Principio | Puerta | Resultado |
| --- | --- | --- |
| I — Simplicidad deliberada | ¿El alcance se limita a lo pedido? | **PASA**. Los 11 endpoints se corresponden con las 7 historias (uno por historia, salvo US2 con dos —asignar y quitar—, US6 con tres —listado, detalle e historial de una historia— y US7 con dos —modificar y eliminar—). Sin métricas, gráficos, capacidad, ceremonias, notificaciones, plantillas, automatismos por calendario ni auditoría: todo eso está en "Fuera de Alcance". |
| II — TDD | ¿Hay forma de escribir la prueba antes del código? | **PASA**. Los 82 escenarios son pruebas nombradas; el entorno de test existe desde la 001. Las validaciones, la superposición, las transiciones, la admisión, los días restantes y las 23 reglas de negocio son pruebas unitarias obligatorias. |
| III — Arquitectura por capas | ¿La estructura cabe en el layout obligatorio? | **PASA**. Un módulo `internal/sprints/` con sus cuatro capas; el frontend usa las capas existentes. |
| IV — Stack fijo y versiones estables | ¿Hace falta algo fuera de la tabla? | **PASA**. Ninguna dependencia nueva. |
| V — Contenerización | ¿Dev y test son disjuntos? | **PASA**. Se reutilizan los contenedores de la 001; esta feature no agrega servicios. |
| VI — Seguridad y secretos | ¿Hay secretos fuera de `.env`? | **PASA**. Sin variables nuevas. La autoría se lee de `users` sin seleccionar `password_hash`. |
| VII — Robustez y validación | ¿Toda entrada se valida antes del servicio? | **PASA**. Cuerpos, parámetros de ruta, filtros y listas de identificadores se validan en `delivery`. |
| VIII — Estándares y documentación viva | ¿Swagger, README e idiomas? | **PASA**. Identificadores y valores de dominio en inglés; comentarios, mensajes y Swagger en español; README actualizado en el mismo cambio. |
| IX — Gobierno del repositorio | ¿El plan pide operaciones prohibidas? | **PASA**. El plan no ejecuta ninguna escritura de Git. |

**Resultado: todas las puertas pasan. Sin violaciones que justificar.**

### Reevaluación (después del diseño de la Fase 1)

| Principio | Qué se revisó en el diseño | Resultado |
| --- | --- | --- |
| I | Dos tablas y una columna, ninguna sin requisito que la pida. Se descartaron una tabla de instantáneas (sus totales se derivan), una tabla de "marcas" de historias (basta un booleano), `started_by_id` y `completed_by_id` (sin escenario que los verifique) y la restricción `EXCLUDE` (exigía una extensión). `SprintRepository` declara 13 operaciones, todas usadas; el puerto `SprintEffort`, una. | **PASA** |
| II | Los 82 escenarios se mapean a pruebas nombradas (tabla más abajo). Las pruebas unitarias obligatorias cubren validaciones, `PeriodsOverlap`, las dos funciones de transición, `Admission`, `Schedule` y las reglas RN-01 a RN-23. El reloj y la zona inyectados hacen reproducibles los días restantes y los momentos reales; el doble del puerto de esfuerzo hace verificables US7-8 y US7-14. | **PASA** |
| III | `sprints/domain` importa solo `backlog/domain`, `projects/domain` y `auth/domain`. `sprints/delivery` no importa `backlog/delivery` ni `projects/delivery`: recibe el subgrupo del proyecto y `RequireStoryAccess` desde `cmd/app/`. `projects/repository` y `backlog/repository` importan `sprints/domain` para sus puertos; ningún `domain` importa `sprints`, así que no hay ciclos. | **PASA** |
| IV | Ninguna dependencia nueva de Go ni de npm. Bloqueos con `gorm.io/gorm/clause` e índices parciales con la opción `where` de las etiquetas de GORM, ambos parte de GORM. | **PASA** |
| V | Sin servicios nuevos. Las tablas se crean con `AutoMigrate` en las mismas bases disjuntas; el ayudante de pruebas las vacía. | **PASA** |
| VI | La autoría selecciona solo `users.id` y `users.full_name`. Ninguna respuesta revela datos de sprints, historias o proyectos ajenos. El cuerpo sigue limitado a 1 MiB y las listas de identificadores a 100. | **PASA** |
| VII | Los once endpoints validan antes del servicio. Errores de dominio traducidos a `404` y `409`; violaciones de los índices parciales traducidas al mismo `409` que la regla. Toda escritura es transaccional y sin `panic` en el camino de una petición. | **PASA** |
| VIII | El contrato OpenAPI está en español. Valores `planned`/`active`/`closed` y `completed`/`not_completed` en la API, con etiquetas en español en el frontend. `any` prohibido. | **PASA** |
| IX | Los artefactos generados son archivos de la copia de trabajo. | **PASA** |

**Resultado: todas las puertas siguen pasando después del diseño. La sección de seguimiento de
complejidad queda vacía.**

#### Tres decisiones que la constitución obliga a documentar

Ninguna es una violación, pero son concesiones conscientes y quedan registradas:

1. **Un puerto, `SprintEffort`, con una implementación que informa "sin esfuerzo"** hasta que exista
   la 006. FR-048, RN-20, US7-8, US7-14 y SC-016 están en esta spec y sin el puerto no se podrían
   implementar ni probar ([research.md](./research.md), sección 16).
2. **Esta feature modifica código de la 002 y de la 003**: los dos puertos que esas features dejaron
   anotados para que esta los implemente, y un campo en el modelo de la historia
   ([research.md](./research.md), secciones 7 y 17).
3. **Dos rutas de esta feature cuelgan de `/api/v1/stories/:storyId`**, el prefijo de la 003. Es lo
   que permite registrar avance sobre una historia sin conocer su sprint (US4-7) y responder con el
   mismo `404` de historia de la 003 ([research.md](./research.md), sección 2).

---

## Estructura del Proyecto

### Documentación de esta feature

```text
specs/004-sprint-management/
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

Las líneas marcadas con `NUEVO` las crea esta feature; las marcadas con `TOCA` son archivos de las
features 001 a 003 que esta feature modifica de forma puntual. Todo lo demás existe y no se toca.

```text
backend/
├── cmd/
│   └── app/
│       └── main.go                          # TOCA: compone el módulo sprints, pasa RequireStoryAccess y la conexión a los dos puertos
├── internal/
│   ├── sprints/                             # NUEVO: módulo único de esta feature
│   │   ├── domain/
│   │   │   ├── sprint.go                    # Entidad Sprint
│   │   │   ├── sprint_story.go              # Entidad SprintStory (compromiso y registro congelado)
│   │   │   ├── status.go                    # SprintStatus, ValidateTransition y operaciones por estado
│   │   │   ├── period.go                    # Period y PeriodsOverlap
│   │   │   ├── progress.go                  # ValidateStoryProgress y Outcome
│   │   │   ├── admission.go                 # Admission y AdmissionProblem
│   │   │   ├── schedule.go                  # Schedule: días restantes o vencido
│   │   │   ├── errors.go                    # Errores de dominio y errores con datos de conflicto
│   │   │   ├── repository.go                # SprintRepository y tipos de lectura
│   │   │   └── ports.go                     # SprintEffort
│   │   ├── repository/
│   │   │   ├── sprint_repository.go         # GORM; transacciones, bloqueos, superposición y agregados
│   │   │   └── sprint_effort.go             # Puerto de esfuerzo: informa "sin esfuerzo" hasta la 006
│   │   ├── service/
│   │   │   └── sprint_service.go            # Create, List, Get, Update, Delete, Assign, Remove, Start, ChangeProgress, Close, StoryHistory
│   │   └── delivery/
│   │       ├── handler.go                   # 11 manejadores HTTP con anotaciones swag en español
│   │       ├── dto.go                       # Peticiones y respuestas
│   │       ├── validation.go                # Nombre, Sprint Goal, fechas, listas de identificadores, estado y filtros
│   │       ├── middleware.go                # RequireSprintAccess (404 único)
│   │       └── routes.go                    # Colección en el subgrupo del proyecto; ítem plano; rutas sobre la historia
│   ├── backlog/
│   │   ├── domain/
│   │   │   └── user_story.go                # TOCA: campo EverCommitted
│   │   └── repository/
│   │       └── sprint_assignments.go        # TOCA: lee sprint_stories y ever_committed en lugar de informar vacío
│   ├── projects/
│   │   └── repository/
│   │       └── sprint_directory.go          # TOCA: lee el sprint activo real
│   └── platform/
│       ├── database/
│       │   └── postgres.go                  # TOCA: AutoMigrate de Sprint y SprintStory
│       └── testsupport/
│           └── database.go                  # TOCA: vacía sprints y sprint_stories
├── postgres/                                # Sin cambios
├── .env.example                             # Sin cambios
├── Dockerfile                               # Sin cambios
├── go.mod                                   # Sin cambios
└── go.sum                                   # Sin cambios

frontend/
├── src/
│   ├── app/
│   │   └── projects/
│   │       └── [projectId]/
│   │           ├── sprints/
│   │           │   ├── page.tsx             # NUEVO: US6 listado y pestaña de historial de cerrados
│   │           │   ├── new/page.tsx         # NUEVO: US1 creación
│   │           │   └── [sprintId]/
│   │           │       ├── page.tsx         # NUEVO: US2 a US6 detalle, compromiso, inicio, avance, cierre; US7 eliminación
│   │           │       └── edit/page.tsx    # NUEVO: US7 modificación
│   │           └── backlog/
│   │               └── [storyId]/page.tsx   # TOCA: sección "Sprints en los que estuvo" (FR-055)
│   ├── features/
│   │   ├── sprints/                         # NUEVO
│   │   │   ├── components/                  # SprintList, SprintHistoryTable, SprintForm, SprintDetailCard,
│   │   │   │                                # SprintSchedule, CommitStoriesDialog, SprintStoryList,
│   │   │   │                                # StoryProgressControl, SprintLifecycleActions, StorySprintHistory
│   │   │   ├── api.ts                       # Llamadas a los 11 endpoints
│   │   │   └── validation.ts                # Reglas de nombre, Sprint Goal y fechas en el formulario
│   │   └── projects/
│   │       └── components/
│   │           └── project-overview.tsx     # TOCA: enlace a los sprints
│   ├── types/
│   │   └── sprint.ts                        # NUEVO: tipos de peticiones y respuestas; sin `any`
│   └── utils/
│       └── sprint-labels.ts                 # NUEVO: etiquetas de estado del sprint y de resultado
├── Dockerfile                               # Sin cambios
└── package.json                             # Sin cambios: ninguna dependencia nueva

README.md                                    # TOCA: describe los sprints y sus endpoints
```

**Pruebas**: en el backend, junto al código del paquete que prueban (`period_test.go`,
`status_test.go`, `progress_test.go`, `admission_test.go`, `schedule_test.go`, `validation_test.go`,
`sprint_service_test.go`, `handler_test.go`, `middleware_test.go`, `sprint_concurrency_test.go`), más
una prueba de integración de cada puerto reemplazado en `projects/repository` y
`backlog/repository`. En el frontend, junto al componente o módulo (`sprint-form.test.tsx`,
`validation.test.ts`, `sprint-schedule.test.tsx`). No se crea un árbol `tests/` separado.

**Decisión de estructura**: se mantiene la aplicación web con `backend/` y `frontend/` como
directorios de primer nivel (Principio III). El backend agrega **un** módulo vertical,
`internal/sprints/`, con sus cuatro capas y las dependencias apuntando hacia `domain/`. Un único
módulo para sprints y compromisos, porque un compromiso no existe fuera de su sprint. El frontend no
agrega capas: las pantallas de sprints se anidan bajo el proyecto para la navegación, aunque las
rutas de ítem de la API sean planas. Antes de escribir código de Next.js se lee la documentación de
la versión instalada en `frontend/node_modules/next/dist/docs/` (CLAUDE.md).

---

## Trazabilidad de escenarios a pruebas (Principio II)

Cada escenario Given/When/Then de la spec se implementa como al menos una prueba cuyo nombre
identifica la historia y el escenario. Buscar `004/US1` a `004/US7` en el código de pruebas devuelve
la trazabilidad completa.

**Backend (Go)**: función `TestUSn_<Escenario>` en el paquete del módulo, precedida por el
comentario `// Escenario: <nombre del escenario en spec.md> (004/USn)`. Las variantes de un mismo
escenario se agrupan como subtests con `t.Run`.

**Frontend (Vitest)**: `describe("004/USn - <título de la historia>")` con un
`it("<nombre del escenario en spec.md>")` por escenario.

| Historia | Escenarios | Pruebas de backend previstas |
| --- | --- | --- |
| US1 Planificar un sprint | 11 | `TestUS1_CreacionEnProyectoEnCurso`, `TestUS1_ProyectoPlanificado`, `TestUS1_VariosPlanificados`, `TestUS1_FechasContiguas`, `TestUS1_LimiteDelSprintGoal`, `TestUS1_DuracionMinima`, `TestUS1_CamposObligatorios`, `TestUS1_FinNoPosteriorAlInicio`, `TestUS1_Superposicion`, `TestUS1_ProyectoFinalizado`, `TestUS1_NoIntegrante` |
| US2 Comprometer historias | 14 | `TestUS2_AsignacionDeTresHistorias`, `TestUS2_AsignacionEnSprintActivo`, `TestUS2_QuitarDeSprintPlanificado`, `TestUS2_ReestimacionDeComprometida`, `TestUS2_QuitarUnicaHistoriaDeSprintActivo`, `TestUS2_TercerSprint`, `TestUS2_HistoriaNoLista`, `TestUS2_HistoriaCompletada`, `TestUS2_ComprometidaEnOtroSprint`, `TestUS2_SprintCerrado`, `TestUS2_LoteParcialmenteInvalido`, `TestUS2_HistoriaDeOtroProyecto`, `TestUS2_QuitarHistoriaCompletada`, `TestUS2_ComprometidaNoDejaDeEstarLista` |
| US3 Iniciar un sprint | 9 | `TestUS3_InicioConHistorias`, `TestUS3_ProyectoPlanificadoNoCambia`, `TestUS3_ProyectoEnCursoNoCambia`, `TestUS3_SprintVacio`, `TestUS3_InicioAntesDeLaFecha`, `TestUS3_OtroSprintActivo`, `TestUS3_SprintYaActivo`, `TestUS3_SprintCerrado`, `TestUS3_ProyectoFinalizado` |
| US4 Registrar el avance | 9 | `TestUS4_AvanceHastaCompletada`, `TestUS4_DesmarcarCompletada`, `TestUS4_RecompletarRegistraElUltimo`, `TestUS4_ReestimacionDuranteElSprint`, `TestUS4_UltimoDiaDelSprint`, `TestUS4_SprintPlanificado`, `TestUS4_HistoriaSinSprint`, `TestUS4_SprintCerrado`, `TestUS4_SaltoDePendienteACompletada` |
| US5 Cerrar y congelar | 11 | `TestUS5_CierreParcial`, `TestUS5_TodasCompletadas`, `TestUS5_AlcanceAgregado`, `TestUS5_InmutableTrasElCierre`, `TestUS5_SinCompletadas`, `TestUS5_SprintVacio`, `TestUS5_CierreAnticipado`, `TestUS5_CierreTardio`, `TestUS5_SprintPlanificado`, `TestUS5_SprintCerrado`, `TestUS5_ProyectoFinalizado` |
| US6 Consultar el historial | 13 | `TestUS6_HistorialDeCerrados`, `TestUS6_ListadoCompletoYFiltro`, `TestUS6_HistorialDeUnaHistoria`, `TestUS6_DiasRestantes`, `TestUS6_TerminaHoy`, `TestUS6_Vencido`, `TestUS6_SinDiasRestantesFueraDeActivo`, `TestUS6_DetalleDesdeLaInstantanea`, `TestUS6_CuatroFechas`, `TestUS6_ProyectoSinSprints`, `TestUS6_CerradosSinCompletadas`, `TestUS6_NoIntegrante`, `TestUS6_SprintInexistente` |
| US7 Corregir o descartar | 15 | `TestUS7_ModificacionCompleta`, `TestUS7_EliminacionConHistorias`, `TestUS7_MoverFechas`, `TestUS7_MismoNombre`, `TestUS7_FechasContiguas`, `TestUS7_EliminacionSinHistorias`, `TestUS7_HistoriaDeSprintEliminado`, `TestUS7_EsfuerzoEliminadoAntes`, `TestUS7_DatosInvalidos`, `TestUS7_Superposicion`, `TestUS7_ModificarActivo`, `TestUS7_ModificarCerrado`, `TestUS7_EliminarActivoOCerrado`, `TestUS7_EsfuerzoRegistrado`, `TestUS7_SprintEliminado` |

**Total: 82 escenarios.**

**Cómo se preparan los escenarios que dependen de otras features**:

- Las historias se crean con los endpoints de la 003, que el router de prueba compone junto con el
  módulo de sprints. Así US2-4, US2-14 y US7-7 ejercitan la integración real con el puerto
  `SprintAssignments` reemplazado.
- El esfuerzo registrado (US7-8, US7-14) se prepara con un doble de `testify/mock` del puerto
  `SprintEffort`, inyectado al componer el router de prueba.
- Los instantes (US4-5, US5-7, US5-8, US6-4 a US6-6, US6-9) se fijan con el reloj inyectado en el
  servicio y la zona de `.env.test`.
- El cambio del factor del proyecto (US5-4) usa el endpoint de modificación de la 002.

**Pruebas transversales sin etiqueta de escenario**: la prueba sin sesión de cada uno de los once
endpoints (SC-013), la comparación byte a byte de los `404` (SC-014), la matriz completa de
transiciones del sprint (SC-006) y la de 10 inicios simultáneos (SC-005) se nombran sin `004/USn`
(por ejemplo `TestSprintEndpoints_SinSesion`, `TestSprintStart_Concurrency`), porque no derivan de un
escenario de la spec y no deben alterar el conteo de 82.

Además, pruebas unitarias **obligatorias** por el Principio II (validaciones, estimación y reglas de
negocio), que cubren SC-017:

| Objeto de prueba | Reglas cubiertas |
| --- | --- |
| Acceso por membresía sin distinguir propietario: `RequireSprintAccess` y el servicio con un actor que no es propietario | RN-01 |
| `SprintStatus.ValidateTransition`: la matriz completa de 9 pares origen-destino | RN-02, RN-18 |
| Validación y recorte del nombre (0, 1, 100 y 101 runas) y del Sprint Goal (0, 1, 500 y 501 runas; solo espacios) | RN-03 |
| Validación de fechas: ausentes, mal formadas, fin igual al inicio, fin anterior y fin al día siguiente | RN-04 |
| `PeriodsOverlap`: borde compartido, un día de solape, período idéntico, contención total y disjuntos | RN-05 |
| Un solo sprint Activo: regla del servicio y traducción de la violación del índice parcial | RN-06 |
| Ninguna operación escribe `projects`: el estado del proyecto antes y después de crear, iniciar, cerrar y eliminar | RN-07 |
| `Admission`: cada motivo por separado, combinaciones y uso de `Readiness` de la 003 | RN-08 |
| Operaciones por estado: asignar y quitar en `planned`, `active` y `closed` | RN-09, RN-18 |
| Quitar devuelve `pending` y sin compromiso abierto | RN-10 |
| `ValidateStoryProgress`: las tres transiciones admitidas y todas las demás combinaciones | RN-11, RN-12 |
| Cierre: historias no completadas a `pending` y conservación de todas las filas | RN-13, RN-14 |
| Cierre: contenido de la instantánea, `Outcome` y factor copiado | RN-15 |
| Inmutabilidad tras el cierre: reestimación, renombre y cambio de factor | RN-16 |
| Historial de una historia que pasó por tres sprints | RN-17 |
| Proyecto Finalizado: rechazo de las ocho escrituras | RN-19 |
| Eliminación: `planned` sin esfuerzo, `planned` con esfuerzo, `active` y `closed` | RN-20 |
| Quitar una historia `completed` | RN-21 |
| Historia comprometida: rechazo de `null` y de criterios vacíos en la 003 a través del puerto real | RN-22 |
| `Schedule`: 10 días, 0 días, vencido hace 3 y el borde de medianoche en `APP_TIMEZONE` | RN-23 |

Comparación con los criterios de éxito: SC-004 con `TestUS1_Superposicion`, `TestUS7_Superposicion` y
las pruebas de `PeriodsOverlap`; SC-005 con `TestSprintStart_Concurrency`; SC-006 con la matriz de
transiciones; SC-007 con las pruebas de `Admission`; SC-008 con `TestUS5_InmutableTrasElCierre`;
SC-009 con `TestUS5_AlcanceAgregado`; SC-010 con `TestUS5_CierreParcial` y `TestUS5_SinCompletadas`;
SC-011 con `TestUS4_SprintPlanificado`, `TestUS4_HistoriaSinSprint` y `TestUS4_SprintCerrado`;
SC-012 con las pruebas de proyecto Finalizado; SC-013 con la prueba sin sesión por endpoint; SC-014
con la comparación de los `404`; SC-015 con `TestUS6_HistorialDeUnaHistoria`; SC-016 con
`TestUS7_EliminarActivoOCerrado` y `TestUS7_EsfuerzoRegistrado`; SC-017 con la tabla de arriba.

---

## Secuencia de trabajo prevista

El orden lo fija `/speckit-tasks`; esto es la dependencia entre bloques, no la lista de tareas.

0. **Prerrequisito**: las features 001, 002 y **003** integradas en `main`. La 003 hoy no se cumple.
1. **Fase técnica fundacional**: entidades `Sprint` y `SprintStory`, `SprintStatus`, errores de
   dominio, `SprintRepository`, el puerto `SprintEffort` con su implementación "sin esfuerzo", el
   campo `EverCommitted` en `UserStory`, `AutoMigrate` de las dos tablas con sus índices parciales y
   el vaciado en `testsupport`. Es el bloque que bloquea a todas las historias.
2. **US1 Planificar** (P1): validaciones de nombre, Sprint Goal y fechas; `PeriodsOverlap`;
   creación con bloqueo del proyecto; `FindDetail` con el autor, porque la respuesta de la creación
   ya es el detalle; endpoint en el subgrupo del proyecto; pantalla de creación.
3. **US2 Comprometer** (P1): `RequireSprintAccess`; `Admission`; asignación y baja en lote con
   bloqueos; `ever_committed`; reemplazo del puerto `SprintAssignments` de la 003 con su prueba de
   integración; endpoints; diálogo de compromiso en el detalle.
4. **US3 Iniciar** (P2): `ValidateTransition`; inicio con bloqueo del proyecto y congelamiento de
   Story Points; reemplazo del puerto `SprintDirectory` de la 002 con su prueba de integración y
   ajuste de `cmd/app/main.go` y de los ayudantes de prueba de `projects/delivery`; prueba de
   concurrencia; endpoint; botón de inicio.
5. **US4 Registrar el avance** (P2): `ValidateStoryProgress`; avance con bloqueo de la historia y
   bloqueo compartido del sprint; endpoint sobre la historia con `RequireStoryAccess` inyectado;
   control de avance por fila.
6. **US5 Cerrar** (P2): `Outcome`; cierre transaccional con congelamiento y devolución al backlog;
   copia del factor; endpoint; botón de cierre con confirmación.
7. **US6 Consultar** (P3): listado con agregados, filtro y `emptyReason`; `Schedule` con reloj y
   zona; detalle con historias vivas o congeladas; historial de una historia; pantallas de listado e
   historial; sección en el detalle de la historia de la 003.
8. **US7 Corregir o descartar** (P3): modificación con bloqueo y superposición excluyéndose;
   eliminación con el puerto de esfuerzo; endpoints; pantalla de edición y botón de eliminación.
9. **Cierre**: Swagger comparado con el contrato, `README.md`, enlace a los sprints desde la vista
   del proyecto, comandos de cobertura, `govulncheck` y la verificación transversal de trazabilidad
   y de las puertas de seguridad (SC-005, SC-006, SC-013, SC-014, SC-017).

En cada historia el ciclo es el del Principio II: escribir la prueba, **verificar que falla y
detenerse** para que el desarrollador registre el commit RED con `/redactar-commit`, implementar,
verificar que pasa, refactorizar. Un agente no avanza de RED a GREEN sin ese alto.

---

## Seguimiento de Complejidad

> Se completa SOLO si la verificación de la constitución tiene violaciones que justificar.

**Sin violaciones.** Las dos puertas de evaluación pasan en su totalidad. Las tres concesiones
documentadas —el puerto hacia la 006, las modificaciones puntuales de la 002 y la 003, y las dos
rutas bajo el prefijo de historias— son consecuencias necesarias de requisitos de la spec (FR-048,
RC-04, RC-06, RC-10, FR-033, FR-055, FR-058) y no desvíos de la constitución; están justificadas en la
sección "Verificación de la Constitución" y en [research.md](./research.md).
