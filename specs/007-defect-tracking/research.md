# Investigación (Fase 0): Gestión de Defectos

**Feature**: `specs/007-defect-tracking` | **Fecha**: 2026-10-25 | **Plan**: [plan.md](./plan.md)

Este documento resuelve las incógnitas técnicas del plan. Las decisiones de producto ya están
cerradas en la spec (sesión de clarificación del 2026-10-03); acá solo se decide **cómo** se
construyen. Cada sección sigue el formato Decisión / Rationale / Alternativas consideradas.

---

## 0. Contexto: sobre qué se construye esta feature

Esta feature consume datos de cuatro features ya integradas y cierra un puerto que la 003 dejó
anotado para ella:

| Feature | Qué aporta | Cómo lo consume la 007 |
| --- | --- | --- |
| 001 | `users`, sesión JWT, `RequireSession`, `CurrentUserID` | Autoría de cada acción y nombre del autor (`users.full_name`) |
| 002 | `projects` (estado), `project_members`, `RequireMembership` | Acceso y regla de solo lectura del proyecto Finalizado |
| 003 | `user_stories`, `RequireStoryAccess`, puerto `DefectRecords` | Historia relacionada, defectos de una historia, bloqueo de eliminación |
| 004 | `sprints` (`status`, `start_date`), listado de sprints con filtro `status` | Sprint de detección y de resolución, orden entre ambos, sprint propuesto por defecto |
| 005 | — | Ninguna relación. La 007 no toca el módulo `poker` |
| 006 | — | Ninguna relación de datos. Se toma su forma de rutas, de `404` y de orden de bloqueos como precedente |

El puerto que esta feature reemplaza:

- `backlog/repository/defect_records.go` (`DefectRecords.HasDefects(storyID)`), que hoy responde
  `false` y bloquea la eliminación de una historia con defectos (003, FR-041, FR-043 y RN-16; 007,
  RC-05 y RN-19).

La 004 **no** dejó un puerto para defectos, y no hace falta: un sprint que es ancla de un defecto
nunca puede eliminarse (RC-08; sección 13).

---

## 1. Versiones del stack: sin cambios

### Decisión

Se mantienen exactamente las versiones vigentes en `main`: Go 1.27.3, Gin v1.12.0, GORM v1.31.2 con
`gorm.io/driver/postgres` v1.6.3, `google/uuid` v1.6.0, `swaggo/*`, `testify` v1.12.1; Next.js
16.4.2, React 19.3.0, Tailwind CSS 4.3.3, TypeScript 5.9.3, Vitest 5.0.3. **Ninguna dependencia
nueva** de Go ni de npm, y **ninguna variable de entorno nueva**.

### Rationale

Todo lo que pide la spec se resuelve con lo existente: `clause.Locking` de GORM para los bloqueos de
fila, `CASE` en `ORDER BY` para el orden por severidad (el mismo recurso que usa el listado del
backlog de la 003 para la prioridad), `fetch` y componentes propios en el frontend.

### Alternativas consideradas

- **Una librería de máquinas de estado**: la máquina tiene cuatro estados y cinco transiciones; un
  mapa y una función pura la expresan en menos líneas que la configuración de cualquier librería.
  Requeriría autorización (Principio IV).

---

## 2. Un módulo `defects` y la forma de las rutas (FR-001, FR-018, FR-033, FR-040, FR-043, FR-049)

### Decisión

Un único módulo vertical `backend/internal/defects/` con sus cuatro capas. **Seis endpoints**:

| # | Método y ruta | Historia | Middleware de acceso |
| --- | --- | --- | --- |
| 1 | `POST /api/v1/projects/{projectId}/defects` | US1 | `RequireMembership` de la 002 (subgrupo) |
| 2 | `GET /api/v1/projects/{projectId}/defects` | US3, US6 | `RequireMembership` de la 002 (subgrupo) |
| 3 | `GET /api/v1/stories/{storyId}/defects` | US3 | `RequireStoryAccess` de la 003 |
| 4 | `GET /api/v1/defects/{defectId}` | US3, US6 (FR-049) | `RequireDefectAccess` (nuevo) |
| 5 | `PATCH /api/v1/defects/{defectId}` | US4 | `RequireDefectAccess` (nuevo) |
| 6 | `PUT /api/v1/defects/{defectId}/status` | US2, US5, US6 | `RequireDefectAccess` (nuevo) |

- El **alta cuelga del proyecto** y no de la historia: la historia relacionada es un atributo
  corregible del defecto (US4-2), no su padre, y el proyecto es lo que nunca cambia (FR-013). La
  historia y el sprint de detección viajan en el cuerpo.
- **Un único endpoint de cambio de estado** con el estado destino en el cuerpo
  (`{"status": "resolved", "resolvedSprintId": "…"}`), que cubre pasar a En progreso, resolver,
  reabrir y descartar. Es la forma que ya usa la 004 para el estado de una historia
  (`PUT /api/v1/stories/{storyId}/status`), y es como la spec describe la entrada ("identificador del
  defecto y estado destino").
- Las rutas de proyecto cuelgan del subgrupo `/api/v1/projects/:projectId` que la 002 ya protege con
  sesión y membresía; la de historia recibe el `RequireStoryAccess` de la 003 como `gin.HandlerFunc`
  desde `cmd/app/`, igual que hicieron la 004, la 005 y la 006; las de ítem van en un grupo plano
  `/api/v1/defects/:defectId`.

### Rationale

- Reutilizar `RequireMembership` y `RequireStoryAccess` hace que el `404` de proyecto y de historia
  sea **byte a byte el mismo** que el de la 002 y la 003 (FR-053, SC-017), sin duplicar consultas.
- La forma `colección bajo el padre, ítem plano` es la convención de la 003, la 004, la 005 y la 006.
- Un solo endpoint de estado concentra en un único lugar la validación de la máquina de estados
  contra el estado vigente (FR-018, sección 9). Cuatro endpoints (`/start`, `/resolve`, `/reopen`,
  `/discard`) repetirían el mismo bloqueo, la misma relectura y la misma respuesta de rechazo.

### Alternativas consideradas

- **`POST /api/v1/stories/{storyId}/defects`**: daría gratis el `404` de historia, pero convierte la
  historia en padre de un recurso que puede cambiar de historia (US4-2) y deja el proyecto implícito
  en una feature cuya regla central es el proyecto (FR-013, FR-054). Descartada.
- **Un endpoint por transición** (`POST /defects/{id}/resolve`, etc.): más rutas para la misma
  regla. La 004 los usa para el sprint (`/start`, `/close`) porque cada transición tiene efectos
  distintos y cuerpos distintos; aquí las cuatro comparten forma y la única diferencia es el sprint
  de resolución. Descartada por el Principio I.
- **El estado dentro del `PATCH`**: la spec exige rechazarlo (US4-15, FR-035) para que la
  modificación nunca dispare una transición. Descartada.
- **Un endpoint que proponga el sprint de detección**: el listado de sprints de la 004 con
  `status=active&status=closed` ya da todo lo necesario (sección 14).

---

## 3. Acceso a un defecto: una consulta y un `404` (FR-051, FR-053, SC-017)

### Decisión

`RequireDefectAccess` resuelve en **una sola consulta** la existencia del defecto y la membresía
vigente del titular de la sesión en el proyecto de su historia:

```text
defects → user_stories (project_id) → project_members (project_id, user_id = sesión) → projects (status)
```

Devuelve un `DefectAccess` con `DefectID`, `ProjectID` y `ProjectFinished`. El identificador mal
formado, el inexistente y el de un proyecto del que quien pide no es integrante comparten **una
única salida 404** con el mismo cuerpo (`"El defecto no existe."`).

No hay verificación de autoría: cualquier integrante vigente actúa sobre cualquier defecto (FR-021,
FR-052, RN-01), así que no existe el `403` de la 006.

### Rationale

- Es el patrón de `RequireStoryAccess`, `RequireSprintAccess`, `RequireEstimationAccess` y
  `RequireEffortEntryAccess`.
- FR-055 queda resuelto sin código especial: una persona quitada del proyecto no tiene fila en
  `project_members`, así que recibe el `404` aunque haya registrado el defecto; los integrantes
  vigentes lo siguen viendo y trabajando.
- `ProjectFinished` permite rechazar las escrituras sin una segunda lectura en el caso común; la
  verificación definitiva se repite dentro de la transacción con el proyecto bloqueado (sección 10).

### Alternativas consideradas

- **Ítem anidado** (`/api/v1/projects/{projectId}/defects/{defectId}`): obliga a validar que el
  defecto pertenece al proyecto de la ruta sin aportar nada; la 003, la 004 y la 006 usan ítems
  planos.

---

## 4. Modelo: una tabla `defects` y una marca en `user_stories` (FR-012 a FR-014, FR-022, FR-056, RC-05)

### Decisión

- **Una tabla** `defects` con: `id` (UUID), `story_id`, `description` (`varchar(2000)`), `severity`,
  `status`, `detected_sprint_id`, `resolved_sprint_id` anulable, `created_by_id`, `updated_by_id`,
  `status_changed_at`, `status_changed_by_id`, `created_at` y `updated_at`. Claves foráneas hacia
  `user_stories`, `sprints` (dos) y `users` (tres) **sin `ON DELETE CASCADE`**.
- **Una columna nueva** `user_stories.ever_had_defects` (`boolean NOT NULL DEFAULT false`), que el
  alta de un defecto pone en `true` sobre su historia, y la revinculación (US4-2) sobre la historia
  nueva. Nadie la vuelve a `false`. El puerto `DefectRecords` de la 003 la lee.
- **Sin columna `project_id`**: el proyecto se deriva de la historia, y la revinculación solo admite
  historias del mismo proyecto (FR-004, FR-038), así que el proyecto del defecto nunca cambia
  (FR-013).
- **Atribución sin historial** (FR-014, FR-022, FR-056): `created_by_id` es el autor del registro
  (FR-055 lo muestra aunque lo quiten); `updated_by_id` es quien hizo la última escritura de
  cualquier tipo; `status_changed_at` y `status_changed_by_id` son el momento y el autor del último
  cambio de estado, inicializados con el alta (el nacimiento en Abierto es el primer estado).

El detalle de columnas, índices y reglas está en [data-model.md](./data-model.md).

### Rationale

- **RC-05 y RN-19 exigen una marca permanente**, y un `EXISTS` sobre `defects.story_id` no alcanza:
  los defectos no se borran, pero **sí cambian de historia** (US4-2). Si el único defecto de una
  historia se revincula a otra, la historia original "tuvo defectos asociados" y no debe volver a
  ser eliminable ("tener defectos vuelve a la historia no eliminable para siempre", Casos Límite).
  Es el mismo problema que la 004 resolvió con `ever_committed` y la 006 con `ever_had_effort`, y se
  resuelve igual: una columna escrita por otro módulo con `UpdateColumn`, sin tocar `updated_at`.
- **A diferencia de la 006, sí hace falta `updated_by_id`**: en la 006 solo el autor modifica su
  registro, así que la última escritura es siempre suya; aquí cualquier integrante modifica
  cualquier defecto (RN-01), y FR-056 pide que cada modificación quede atribuida. Sin historial de
  auditoría (Fuera de Alcance), lo mínimo es la atribución de la última.
- **`status_changed_at` y `status_changed_by_id`** son la lectura literal de FR-022 ("el momento de
  cada cambio de estado y a quién se atribuye") sin historial: se conserva el último. Inicializarlos
  en el alta evita columnas anulables y un caso especial en la interfaz.
- Las claves foráneas sin cascada hacen que ni borrar una historia, ni un sprint, ni un usuario
  puedan arrastrar defectos (RN-19, RN-20, FR-055).

### Alternativas consideradas

- **`EXISTS` sobre `defects` en el puerto de la 003**: incumple RC-05 tras una revinculación.
  Descartada.
- **`project_id` desnormalizado**: ahorra un `JOIN` en el listado del proyecto a cambio de una
  columna derivable que hay que mantener coherente con la historia. Descartada por el Principio I y
  por el precedente de la 006.
- **Una tabla de historial de estados**: daría FR-022 completo, pero la spec deja fuera el historial
  consultable (Fuera de Alcance) y no hay ningún consumidor. Descartada por YAGNI.
- **Restricciones `CHECK` en la base** (por ejemplo, `resolved_sprint_id` no nulo si y solo si el
  estado es `resolved`): ninguna tabla del proyecto las usa y la invariante ya la garantiza una única
  función pura con prueba unitaria obligatoria (sección 8). Se descarta para no introducir un segundo
  lugar donde vive la regla.

---

## 5. Valores de dominio y orden de los listados (FR-003, FR-017, FR-048)

### Decisión

- **Severidad** (`DefectSeverity`, texto en `defects.severity`): `critical`, `high`, `medium`, `low`.
- **Estado** (`DefectStatus`, texto en `defects.status`): `open`, `in_progress`, `resolved`,
  `discarded`.
- Los valores se guardan como texto y no como enum de PostgreSQL, igual que los estados de historia
  y de sprint. Las etiquetas en español (Crítica, Alta, Media, Baja; Abierto, En progreso, Resuelto,
  Descartado) viven en el frontend (`utils/defect-labels.ts`).
- **Orden de todo listado** (FR-048):

  ```sql
  ORDER BY CASE defects.severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
           defects.created_at DESC, defects.id DESC
  ```

  `id` desempata dos altas en el mismo instante, para que el orden sea idéntico ante consultas
  repetidas (SC-014).

### Rationale

Es el mismo recurso que usa el listado del backlog de la 003 para la prioridad. El rango de severidad
no se almacena porque es derivable del valor.

### Alternativas consideradas

- **Una columna `severity_rank`**: dato derivado. Descartada.
- **Severidad como entero** (`0..3`): obliga a traducir en cada respuesta y hace ilegibles las
  consultas manuales. Descartada.

---

## 6. Validación de entrada (FR-002 a FR-004, FR-006, FR-016, FR-023, FR-034, FR-035, FR-037)

### Decisión

La validación de forma vive en `defects/delivery/validation.go` y devuelve **todos** los campos
inválidos a la vez como `400` con `ValidationErrorResponse` de la 001, antes de llegar al servicio
(Principio VII):

| Endpoint | Campo | Regla | Mensaje (resumen) |
| --- | --- | --- | --- |
| Alta, `PATCH` | `description` | Obligatoria en el alta; 1 a 2000 caracteres (runas) tras recortar espacios extremos | "La descripción es obligatoria." / "…como máximo 2000 caracteres." |
| Alta, `PATCH` | `severity` | Obligatoria en el alta; `critical`, `high`, `medium` o `low` | "La severidad debe ser Crítica, Alta, Media o Baja." |
| Alta, `PATCH` | `storyId` | Obligatorio en el alta; UUID | "La historia relacionada es obligatoria." |
| Alta, `PATCH` | `detectedSprintId` | Obligatorio en el alta; UUID | "El sprint de detección es obligatorio." |
| `PATCH` | `status`, `resolvedSprintId` | Prohibidos | "El estado y el sprint de resolución se cambian con las acciones de estado del defecto." |
| `PATCH` | `projectId`, `createdBy`… | Prohibidos | "El proyecto y el autor de un defecto no se modifican." |
| `PATCH` | (cuerpo) | Al menos un campo editable | "Indicá al menos un dato para modificar." |
| Estado | `status` | Obligatorio; uno de los cuatro estados | "El estado debe ser Abierto, En progreso, Resuelto o Descartado." |
| Estado | `resolvedSprintId` | Obligatorio si `status = resolved`; prohibido en cualquier otro destino; UUID | "El sprint de resolución es obligatorio para resolver." / "El sprint de resolución solo se indica al resolver." |

- Un identificador con forma inválida en `storyId`, `detectedSprintId` o `resolvedSprintId` se
  reporta como `400` de forma ("…no es un identificador válido."), igual que la 004 con los
  identificadores de historias del cuerpo. Uno con forma válida que no existe o es de otro proyecto
  se trata en la sección 7.
- El `PATCH` es parcial: cada campo es opcional. Los campos prohibidos se decodifican como
  `json.RawMessage` **solo para rechazarlos** (patrón de `StoryPatch` de la 003 y `EntryPatch` de la
  006). El `POST` no declara ningún campo de autoría: Gin los ignora y el servicio toma siempre el
  autor de la sesión (FR-056).
- Las reglas que dependen de datos —sprint Planificado, orden entre sprints, pertenencia al
  proyecto, ventana de corrección, máquina de estados, proyecto Finalizado— son **reglas de dominio**
  y se evalúan dentro de la transacción con los datos releídos (secciones 7 a 11). Las que se
  refieren a un campo concreto (sprint Planificado, orden entre sprints) se informan también como
  `400` con `field`, para que el formulario las muestre junto al campo.

### Rationale

- La atomicidad (FR-016, FR-037) es consecuencia directa: nada se escribe si la validación falla, y
  las reglas de dominio se evalúan antes de cualquier escritura dentro de la transacción.
- Rechazar `resolvedSprintId` fuera de una resolución evita que un cliente crea que lo registró al
  pasar a En progreso o al descartar (RN-23: un descartado nunca tiene sprint de resolución).

### Alternativas consideradas

- **`DisallowUnknownFields`** para todo el cuerpo: ninguna feature lo usa; rechazaría campos
  inofensivos y cambiaría la convención.
- **Ignorar en silencio `status` en el `PATCH`**: la spec pide rechazarlo (US4-15).

---

## 7. Referencias del cuerpo: historia y sprints del proyecto (FR-004, FR-006, FR-007, FR-010, FR-024, FR-053)

### Decisión

| Situación | Respuesta |
| --- | --- |
| `storyId` inexistente o de otro proyecto | `404` con el cuerpo de la 003: `"La historia no existe."` |
| `detectedSprintId` o `resolvedSprintId` inexistente o de otro proyecto | `404` con el cuerpo de la 004: `"El sprint no existe."` |
| Sprint de detección `planned` | `400`, `field: detectedSprintId`: "Un defecto solo se detecta en un sprint Activo o Cerrado." |
| Sprint de resolución `planned` | `400`, `field: resolvedSprintId`: "Un defecto solo se resuelve en un sprint Activo o Cerrado." |
| Sprint de resolución con `start_date` anterior al del sprint de detección | `400`, `field: resolvedSprintId`: "El sprint de resolución no puede empezar antes que el de detección." |
| Alta en un proyecto sin ningún sprint `active` ni `closed` | `409`, `code: no_detection_sprint`: "Todavía no hay ningún sprint Activo ni Cerrado que pueda ser sprint de detección." |

- La pertenencia se verifica con el `project_id` del proyecto del defecto (el de la ruta en el alta,
  el de `DefectAccess` en las demás): `WHERE id = ? AND project_id = ?`. "No existe" y "es de otro
  proyecto" producen la misma consulta vacía y la misma respuesta.
- **El orden de las comprobaciones del alta** es: proyecto Finalizado (`409 read_only`) → proyecto
  sin sprints iniciados (`409 no_detection_sprint`) → historia del proyecto (`404`) → sprint del
  proyecto (`404`) → sprint no Planificado (`400`). Así US1-16 responde lo mismo envíe el cliente un
  sprint Planificado o uno inexistente: lo que falla es el proyecto, no el dato.
- **El orden entre sprints** compara `sprints.start_date` (RC-07); el mismo sprint en ambos campos es
  válido (FR-025). Como un sprint `active` o `closed` ya no admite cambios de fechas (la 004 solo
  modifica sprints `planned`), las dos fechas comparadas son inmutables desde que el sprint es
  utilizable como ancla.

### Rationale

- El `404` para una referencia del cuerpo es el precedente de la 004: comprometer en un sprint una
  historia de otro proyecto responde el `404` de historia de la 003. Reutilizar los mismos cuerpos
  hace que la respuesta sea indistinguible de la de un identificador inexistente (FR-053, SC-017).
- "Sin sprints iniciados" es un estado del proyecto y no un error del dato enviado, por eso es un
  `409` con `code` y no un `400` de campo: la interfaz lo muestra como aviso del formulario, no junto
  a un campo.

### Alternativas consideradas

- **`400` con `field: storyId` para la historia ajena**: distinguiría la historia ajena de la ruta
  con un código distinto al de la 004 sin aportar nada. Descartada.
- **`422 Unprocessable Entity`**: ninguna feature lo usa. Descartada por coherencia.

---

## 8. La máquina de estados como función pura (FR-017 a FR-020, FR-023 a FR-032, FR-039, RN-11 a RN-17, RN-21 a RN-23)

### Decisión

`defects/domain/status.go` declara el tipo `DefectStatus`, el mapa de transiciones admitidas y una
función pura que decide el resultado de un pedido de cambio de estado:

```text
allowedDefectTransitions = {
  open:        [in_progress, discarded],
  in_progress: [resolved, discarded],
  resolved:    [open],
  discarded:   [],
}

PlanTransition(current DefectStatus, to DefectStatus, detected SprintAnchor, resolution *SprintAnchor)
    (resolvedSprintID *uuid.UUID, err error)
```

- Si `to` no está en `allowedDefectTransitions[current]` → `InvalidTransitionError{Current, Requested, Allowed}`.
  El mensaje para la persona se elige por caso: salto de Abierto a Resuelto (FR-019), vuelta de En
  progreso a Abierto (FR-020), reapertura de lo que no está Resuelto (FR-030), descarte de un
  Resuelto (FR-039), cualquier acción sobre un Descartado (RN-22) y el genérico con las permitidas.
- `to = resolved` exige `resolution` (`ErrResolutionSprintRequired`, FR-023); el sprint debe estar
  `active` o `closed` (`ErrResolutionSprintPlanned`, FR-024) y su `StartDate` no puede ser anterior
  a la de `detected` (`ErrResolutionBeforeDetection`, FR-025). Devuelve el ID del sprint de
  resolución.
- `to = open` (reapertura) devuelve `nil`: el sprint de resolución se borra (FR-029).
- `to = in_progress` y `to = discarded` devuelven `nil`: un defecto que llega a esos estados nunca
  tiene sprint de resolución (el origen es `open` o `in_progress`, que no lo tienen; RN-23).
- El sprint de detección y la historia **no aparecen en la salida**: ninguna transición puede
  cambiarlos (FR-027, RN-16, SC-010).

`SprintAnchor` es un tipo de lectura con `ID`, `Status` y `StartDate`.

### Rationale

- Una función pura con el mapa como única fuente de verdad hace que la prueba unitaria obligatoria
  recorra las **16 combinaciones** de origen y destino (las 5 válidas y las 11 inválidas de SC-005,
  incluidas las 4 salidas desde Descartado) sin base de datos.
- Que la función devuelva el `resolved_sprint_id` resultante, y no "un booleano más la orden de
  borrar", deja una sola forma de escribirlo: el repositorio asigna siempre la salida. RN-15, RN-17 y
  RN-23 se convierten en propiedades de la función.

### Alternativas consideradas

- **Métodos sobre la entidad** (`defect.Resolve(sprint)`): esconde la regla en la entidad persistida
  y obliga a cargarla completa para probarla. Descartada por el patrón del proyecto
  (`SprintStatus.ValidateTransition`, `ValidateStoryProgress`, `ResolveAnchor`).
- **Una tabla de transiciones en la base**: configuración para cinco reglas fijas. Descartada.

---

## 9. Transiciones bajo concurrencia (FR-018, RN-24, SC-019)

### Decisión

Toda escritura sobre un defecto existente —cambio de estado y modificación— toma, dentro de su
transacción, un **bloqueo `FOR UPDATE` sobre la fila del defecto** y evalúa las reglas con el estado
**releído bajo ese bloqueo**. La segunda de dos acciones simultáneas espera a que la primera
confirme, relee el estado ya cambiado y se valida contra él:

| Estado leído por ambas | Primera en aplicarse | Segunda | Resultado de la segunda |
| --- | --- | --- | --- |
| `in_progress` | Resolver | Descartar | `409 invalid_transition`, `currentStatus: resolved` |
| `in_progress` | Descartar | Resolver | `409 invalid_transition`, `currentStatus: discarded` |
| `open` | Pasar a En progreso | Pasar a En progreso | `409 invalid_transition`, `currentStatus: in_progress` |
| `open` | Descartar | Modificar | `409 defect_discarded` (FR-018 última oración, FR-036) |
| `open` | Pasar a En progreso | Corregir un vínculo | `409 links_frozen` (FR-038) |

El rechazo informa siempre el **estado vigente** (`currentStatus`) y las transiciones permitidas desde
él (`allowedStatuses`), de modo que la interfaz puede refrescar el defecto sin otra consulta.

Dos correcciones simultáneas de descripción o severidad también se serializan por el mismo bloqueo, y
se aplican las dos en orden: gana la última escritura, porque ninguna invariante las relaciona
(Supuestos).

### Rationale

- La spec exige validar "contra el estado vigente al aplicarla, no contra el estado leído antes"
  (FR-018). El bloqueo de fila es la pieza mínima que lo garantiza y no requiere que el cliente
  envíe el estado que cree vigente.
- La invariante protegida es la máquina de estados y, en particular, el carácter definitivo de
  Descartado (Clarifications). El bloqueo serializa solo las escrituras **de ese defecto**, así que
  la contención es despreciable.
- Es el mecanismo que la 004 usa para el cierre y el inicio de sprints (`FOR UPDATE` y relectura).

### Alternativas consideradas

- **Concurrencia optimista con `If-Match` o un campo `version`**: obliga a que el cliente envíe la
  versión y a reintentar; además rechazaría dos modificaciones de descripción compatibles, que la
  spec quiere aceptar. Descartada.
- **`UPDATE … WHERE id = ? AND status = ?` condicional**: evita el bloqueo explícito pero, cuando no
  afecta filas, hay que releer para informar el estado vigente, y no sirve para la modificación de
  datos con su ventana de corrección. Descartada por tener dos mecanismos para lo mismo.
- **Aislamiento `SERIALIZABLE`**: obliga a reintentar transacciones abortadas y no lo usa ninguna
  otra feature.

---

## 10. Transacciones y orden de bloqueos (FR-016, FR-018, FR-037, FR-054, RC-05)

### Decisión

**Alta** (`DefectRepository.Create`), en una transacción:

1. Leer el proyecto con **`FOR SHARE`** (patrón `Create` de la 003): Finalizado →
   `ErrDefectsReadOnly` (FR-054).
2. Verificar que el proyecto tiene al menos un sprint `active` o `closed`
   (`ErrNoDetectionSprint`, FR-010).
3. Leer el sprint de detección del proyecto con **`FOR KEY SHARE`**: inexistente o ajeno →
   `ErrSprintNotFound`; `planned` → `ErrDetectionSprintPlanned` (FR-006, FR-007).
4. Marcar la historia: `UPDATE user_stories SET ever_had_defects = true WHERE id = ? AND project_id =
   ?` con `UpdateColumn`, sin tocar `updated_at`. Cero filas → `ErrStoryNotFound` (FR-004). La
   sentencia valida la pertenencia y bloquea la historia en una sola operación.
5. Insertar el defecto en `open`, sin sprint de resolución, con `created_by_id`, `updated_by_id` y
   `status_changed_by_id` iguales al titular de la sesión (FR-011, FR-056).

**Cambio de estado** (`DefectRepository.ChangeStatus`), en una transacción:

1. Proyecto del defecto con **`FOR SHARE OF projects`** (patrón `lockWritableStoryProject` de la
   003): Finalizado → `ErrDefectsReadOnly`.
2. Defecto con **`FOR UPDATE`** y su sprint de detección (`id`, `status`, `start_date`).
3. Si el destino es `resolved` y viene `resolvedSprintId`: el sprint del proyecto con
   **`FOR KEY SHARE`** (inexistente o ajeno → `ErrSprintNotFound`).
4. `PlanTransition` (sección 8) con el estado releído.
5. Escribir `status`, `resolved_sprint_id`, `status_changed_at`, `status_changed_by_id` y
   `updated_by_id`; `updated_at` lo completa GORM.

**Modificación** (`DefectRepository.Update`), en una transacción: proyecto `FOR SHARE` → defecto
`FOR UPDATE` → si cambia el sprint de detección, el sprint nuevo `FOR KEY SHARE` → si cambia la
historia, el `UPDATE` que marca la historia nueva (como en el paso 4 del alta) → reglas de la sección
11 → escritura de los campos presentes y de `updated_by_id`.

**Por qué no hay interbloqueos con la 004 ni con la 003.**

- Las escrituras de la 004 que tocan historias (`Close`, `AssignStories`, `RemoveStories`,
  `ChangeStoryProgress`) bloquean **`sprints → user_stories`**. Las de esta feature toman, cuando las
  necesitan, **`projects (SHARE) → defects → sprints (KEY SHARE) → user_stories`**: nunca bloquean
  una historia antes que un sprint. Es el mismo criterio que la 006 adoptó para no chocar con
  `Close`, y aquí se cumple de forma explícita porque el sprint se bloquea con su propia sentencia
  antes de tocar la historia, en lugar de depender del orden en que PostgreSQL verifica las claves
  foráneas del `INSERT`.
- `Delete` de un sprint y `Start` toman `projects (UPDATE) → sprints`; esta feature toma el proyecto
  antes que todo lo demás, igual que ellas.
- `Delete` de una historia en la 003 toma `projects (SHARE) → user_stories (DELETE)`; esta feature
  respeta el mismo orden.
- La fila de `defects` solo la bloquea esta feature, siempre después del proyecto.

**Eliminación de una historia con un defecto en curso (RN-19).** El servicio de la 003 consulta
`DefectRecords.HasDefects` antes de borrar. Si un defecto se registra entre esa consulta y el borrado,
caben dos órdenes: si el alta bloqueó primero la historia (paso 4), el `DELETE` de la 003 espera, y al
continuar viola la clave foránea `defects.story_id`; si el `DELETE` bloqueó primero, el `UPDATE` del
alta espera y al continuar no encuentra la historia (`404`). En el primer caso, `story_repository.go`
traduce la violación de esa clave foránea al mismo `DeletionBlockedError` con el motivo `defects`,
igual que ya hace con la de `estimation_sessions` (sección 13).

### Rationale

- Tomar el proyecto con `FOR SHARE` es el mecanismo que la 003, la 004 y la 006 ya usan para que el
  proyecto no pase a Finalizado entre la verificación y la escritura; el cambio de estado de la 002
  actualiza esa fila y queda serializado (SC-015).
- `FOR KEY SHARE` sobre el sprint es el bloqueo más débil que impide borrarlo, y es lo que el `INSERT`
  tomaría de todos modos por la clave foránea; tomarlo explícitamente fija el orden.
- El `UPDATE` de la marca hace tres cosas a la vez —valida la pertenencia, bloquea la historia contra
  su borrado y cumple RC-05— con una sola sentencia.

### Alternativas consideradas

- **Leer la historia con `FOR SHARE` y marcarla después**: dos sentencias para lo que hace una, y el
  paso de `SHARE` a `NO KEY UPDATE` sobre la misma fila abre una ventana de espera innecesaria.
- **Bloquear el sprint con `FOR SHARE`**: también impediría su cierre concurrente, sin ninguna
  necesidad, porque `active` y `closed` son igualmente válidos como ancla.

---

## 11. Modificación y ventana de corrección de los vínculos (FR-033 a FR-038, RN-16, RN-18, RN-22)

### Decisión

Con el defecto releído bajo `FOR UPDATE` (sección 9), el servicio aplica, en este orden:

1. `EnsureDefectEditable(status)`: `discarded` → `ErrDefectDiscarded` (`409 defect_discarded`;
   FR-036, RN-22). Rige para **cualquier** campo, incluidos descripción y severidad.
2. Para cada vínculo presente (`storyId`, `detectedSprintId`) **cuyo valor difiere del vigente**:
   `EnsureLinksCorrectable(status)`: distinto de `open` → `ErrLinksFrozen` (`409 links_frozen`;
   FR-038, US4-12).
3. Las validaciones de pertenencia y de estado del sprint de la sección 7, iguales a las del alta
   (FR-034, FR-038, US4-11, US4-13).
4. La escritura de los campos presentes, en una sola sentencia. Nunca toca `status`,
   `resolved_sprint_id`, `created_by_id` ni `status_changed_*` (FR-035, RN-18).

- Un vínculo enviado **con el mismo valor que ya tiene** no es una corrección y se acepta sin
  efecto en cualquier estado salvo Descartado. Así un formulario que reenvía todos los campos no
  falla por los que no cambió, y US4-7 ("la misma descripción y severidad, sin efectos") se extiende
  con el mismo criterio a los vínculos.
- Una modificación sin ningún cambio efectivo igual se acepta y responde `200` con el defecto; se
  escribe `updated_by_id` y GORM actualiza `updated_at`. No se intenta detectar el "sin cambios"
  para evitar la escritura, porque la spec no lo pide y no altera ningún dato de medición.
- Como un defecto `open` nunca tiene sprint de resolución, corregir el de detección no puede romper
  el orden entre ambos (FR-038) y no se verifica.

### Rationale

- La ventana se evalúa contra el estado releído, así que el caso límite de la corrección que llega
  junto con un paso a En progreso se resuelve por el bloqueo (sección 9).
- US4-9 (ventana reabierta tras una reapertura) no necesita código: la regla mira el estado actual.

### Alternativas consideradas

- **Rechazar todo vínculo presente fuera de Abierto, aunque no cambie**: más simple de escribir,
  pero obliga a la interfaz a omitir campos según el estado y castiga al cliente por reenviar lo que
  ya está. Descartada.

---

## 12. Consultas y filtros (FR-040 a FR-049)

### Decisión

- **Defectos del proyecto** (`GET /projects/{id}/defects`): una consulta sobre `defects` unido a
  `user_stories` (filtrado por `project_id` y para el título de la historia), a `sprints` dos veces
  (detección y resolución, este con `LEFT JOIN`) y a `users` para los nombres. **Sin paginación ni
  tope** (FR-040). Orden de la sección 5.
- **Filtros** (query string, todos opcionales y conjuntivos entre sí; FR-041):

  | Parámetro | Valor | Regla |
  | --- | --- | --- |
  | `status` | Repetible: `open`, `in_progress`, `resolved`, `discarded` | Los valores de un mismo parámetro se combinan con "o" (FR-040: "quien quiera excluirlos filtra por los otros tres estados") |
  | `severity` | Repetible: `critical`, `high`, `medium`, `low` | Ídem |
  | `storyId` | UUID | Una historia del proyecto (FR-047) |
  | `sprintId` | UUID | Un sprint del proyecto; devuelve los defectos con `detected_sprint_id = ?` **o** `resolved_sprint_id = ?` (FR-042) |

  Es la convención del listado del backlog de la 003 (`status` y `priority` repetibles leídos con
  `c.QueryArray`). Un valor de estado o severidad inexistente se rechaza con `400`,
  `field: status|severity` y los valores admitidos (FR-046). Un `storyId` o `sprintId` mal formado,
  inexistente o de otro proyecto se rechaza con `400`, `field: storyId|sprintId` y el **mismo
  mensaje** (`"El filtro no corresponde al proyecto."`), sin distinguir los casos (FR-047, SC-017).
- **Distinción en el filtro por sprint** (FR-042): cada defecto de la respuesta trae
  `detectedSprint` y `resolvedSprint` completos; el cliente sabe si coincidió por detección, por
  resolución o por ambas comparando con el filtro. No se agrega un campo calculado.
- **Lista vacía** (FR-045): la respuesta lleva `emptyReason`: `no_defects` si el proyecto no tiene
  ningún defecto (US3-6) o `no_matches` si los filtros no encuentran ninguno (US3-8); `null` si la
  lista tiene elementos. Es el patrón del listado de sprints de la 004 y del esfuerzo de la 006.
- **Defectos de una historia** (`GET /stories/{id}/defects`): la misma consulta con
  `defects.story_id = ?`, mismo orden y mismo `emptyReason` (`no_defects` o `null`; US3-7).
- **Detalle** (`GET /defects/{id}`): el mismo tipo de lectura que un elemento del listado, que ya
  contiene todos los datos, su estado y sus dos sprints (FR-049, US6-6).
- Los nombres de autor salen de `users`, que nunca se borra, así que una persona quitada sigue
  apareciendo con su nombre (FR-055, RN-20). Ningún `SELECT` incluye `password_hash`.

### Rationale

- El filtro repetible es la lectura más directa de FR-040, que explícitamente sugiere excluir los
  descartados filtrando por "los otros tres estados".
- Devolver ambos sprints en cada elemento cubre FR-042 y FR-044 con un único tipo de respuesta.

### Alternativas consideradas

- **Filtro `excludeDiscarded=true`**: un parámetro nuevo para lo que el filtro de estado ya expresa.
  Descartado.
- **Campo `matchedBy: detected|resolved|both`**: derivable de los datos de la respuesta; solo tendría
  sentido con un filtro de sprint y quedaría `null` en todo lo demás. Descartado.
- **Paginación**: fuera de alcance (Clarifications).

---

## 13. Integración con la 003 y la 004

### Decisión

- `backlog/domain/user_story.go`: se agrega el campo `EverHadDefects` (`ever_had_defects boolean NOT
  NULL DEFAULT false`); la 003 nunca lo escribe, igual que `EverCommitted` y `EverHadEffort`.
- `backlog/repository/defect_records.go`: `NewDefectRecords(db)` pasa a recibir la conexión y
  `HasDefects` lee `user_stories.ever_had_defects`. `backlog/service` no cambia.
- `backlog/repository/story_repository.go` (`Delete`): además de la clave foránea de
  `estimation_sessions`, reconoce la de `defects.story_id` (`fk_defects_story`) y la traduce a
  `DeletionBlockedError{Reasons: [defects]}` (sección 10).
- `cmd/app/main.go`: compone el módulo y pasa `db` al puerto.
- **La 004 no se toca.** RC-08 garantiza que ningún defecto puede apuntar a un sprint eliminable: la
  004 solo elimina sprints `planned` que nunca se iniciaron, esta feature solo acepta sprints `active`
  o `closed`, y un sprint nunca vuelve a `planned`. No hay ventana de carrera: si el alta lee el
  sprint como `active` o `closed`, ya no puede eliminarse; si lo lee `planned`, se rechaza. La clave
  foránea sin cascada queda como respaldo.

### Rationale

Es exactamente lo que el puerto dejó escrito para esta feature, más la traducción de la clave foránea
que la 005 ya estableció como precedente en el mismo método. Ni `backlog/service` ni
`sprints/service` cambian, y sus pruebas con dobles siguen valiendo.

### Alternativas consideradas

- **Un puerto `SprintDefects` en la 004**: no tendría ningún caso en el que responda `true` para un
  sprint eliminable. Descartado por YAGNI.

---

## 14. Sprint de detección propuesto por defecto (FR-008 a FR-010, US1-2, US1-4)

### Decisión

La propuesta es **responsabilidad del frontend**: el formulario de registro consulta el listado de
sprints de la 004 con `GET /api/v1/projects/{projectId}/sprints?status=active&status=closed`, propone
el sprint `active` si existe y ofrece los `closed` como alternativas; sin `active`, no preselecciona
ninguno (FR-009); sin ninguno de los dos, muestra el aviso de US1-16 y deshabilita el envío. El
backend no propone nada: valida el sprint recibido y, si el proyecto no tiene ninguno iniciado,
responde `409 no_detection_sprint` (FR-010).

### Rationale

- "Proponer por defecto" es comportamiento de la interfaz; el backend ya expone el dato con el filtro
  repetible de estados de la 004.
- La regla que sí protege la medición (nunca un sprint `planned`, nunca un proyecto sin sprints
  iniciados) queda en el backend.

### Alternativas consideradas

- **Que el backend complete el sprint activo si falta `detectedSprintId`**: convierte un campo
  obligatorio (FR-006, US1-11) en opcional y hace ambiguo el error. Descartada.
- **Un endpoint `GET /projects/{id}/defects/defaults`**: un endpoint para un dato que ya existe.

---

## 15. Frontend (SC-001 a SC-003)

### Decisión

- **Página nueva** `app/projects/[projectId]/defects/page.tsx`: listado del proyecto con filtros
  (estado y severidad como selección múltiple, historia, sprint), indicación de lista vacía según
  `emptyReason`, y botón "Registrar defecto" que abre el formulario.
- **Página nueva** `app/projects/[projectId]/defects/new/page.tsx`: formulario de registro con
  descripción, severidad, historia (preseleccionada si se llega desde una historia, con
  `?storyId=`) y sprint de detección con la propuesta de la sección 14. Registrar es: abrir el
  formulario, escribir la descripción, elegir la severidad, elegir la historia y confirmar (SC-001: 5
  pasos, con el sprint ya propuesto; 4 si se llega desde la historia).
- **Página nueva** `app/projects/[projectId]/defects/[defectId]/page.tsx`: detalle con todos los
  datos; edición de descripción y severidad, y de historia y sprint de detección solo si el defecto
  está Abierto; acciones de estado según `allowedStatuses` del estado vigente (pasar a En progreso,
  resolver con selector de sprint de resolución, reabrir, descartar con confirmación). Un Descartado
  se muestra de solo lectura. Ante un `409 invalid_transition` se refresca el defecto con el
  `currentStatus` recibido.
- **Detalle de la historia** (`app/projects/[projectId]/backlog/[storyId]/page.tsx`, de la 003): se
  agrega la sección "Defectos" (`StoryDefectsSection`) con la lista de sus defectos y el enlace a
  registrar uno sobre esa historia (SC-003).
- **Resumen del proyecto** (`project-overview`, de la 002): un enlace a "Defectos".
- Validación en el cliente con las mismas reglas de forma (`features/defects/validation.ts`), como
  ayuda; el backend sigue siendo la autoridad. Etiquetas en `utils/defect-labels.ts`.
- Las transiciones permitidas para mostrar los botones se derivan en el cliente con una copia del
  mapa de la sección 8 (`allowedTransitions`), validada por una prueba que recorre las 16
  combinaciones; el backend la vuelve a validar.
- Antes de escribir código de Next.js se lee la documentación de la versión instalada en
  `frontend/node_modules/next/dist/docs/` (CLAUDE.md).

### Rationale

La lista filtrada y el detalle concentran el seguimiento (US2 a US6); la sección en la historia es
donde el integrante ya está cuando descubre el problema (SC-003). SC-002 —encontrar los Críticos
abiertos en menos de 30 segundos— son dos selecciones en los filtros.

### Alternativas consideradas

- **Modales en lugar de páginas** para registrar y para el detalle: las páginas de la 003 y la 004
  usan rutas propias (`new`, `[id]`); se sigue el mismo patrón.
- **Que el backend devuelva `allowedStatuses` en cada defecto**: agrega un campo derivado a todas
  las respuestas por una necesidad exclusiva de la interfaz. Se devuelve solo en el `409`, donde la
  interfaz no puede deducirlo porque el estado cambió.

---

## 16. Códigos HTTP

| Situación | Código | Cuerpo |
| --- | --- | --- |
| Alta | `201` | El defecto creado |
| Modificación y cambio de estado | `200` | El defecto actualizado |
| Consultas | `200` | Ver contrato |
| Sin sesión | `401` | `ErrorResponse` de la 001 |
| Campo inválido, campo prohibido, sprint Planificado, sprint de resolución anterior o ausente, filtro inválido | `400` | `ValidationErrorResponse` |
| Proyecto, historia o defecto inexistente o ajeno (ruta) | `404` | `ErrorResponse` idéntico al de la 002, la 003 o el propio |
| Historia o sprint inexistente o ajeno (cuerpo) | `404` | `ErrorResponse` idéntico al de la 003 o la 004 |
| Proyecto Finalizado | `409` | `DefectConflictResponse` (`code: read_only`) |
| Proyecto sin sprints Activos ni Cerrados | `409` | `DefectConflictResponse` (`code: no_detection_sprint`) |
| Transición no permitida desde el estado vigente | `409` | `DefectConflictResponse` (`code: invalid_transition`, con `currentStatus` y `allowedStatuses`) |
| Modificación de un Descartado | `409` | `DefectConflictResponse` (`code: defect_discarded`) |
| Corrección de un vínculo fuera de Abierto | `409` | `DefectConflictResponse` (`code: links_frozen`, con `currentStatus`) |

Los `409` llevan un campo `code` para que la interfaz los distinga sin interpretar el texto, igual
que los de la 006.

---

## Puntos abiertos que no bloquean el diseño

1. **Conflicto previsible con la rama de la 006**: esta feature modifica
   `backlog/domain/user_story.go`, `platform/database/postgres.go`, `platform/testsupport/database.go`
   y `cmd/app/main.go`, que la 006 también toca. La fase fundacional de la 006 ya está en `main`
   (`effort_entries`, `ever_had_effort`), así que los cambios son aditivos, pero quien integre la
   segunda rama tendrá que resolver el orden de las líneas.
2. **Traducción de la clave foránea en el `Delete` de la 003**: depende del nombre que GORM genera
   para la restricción (`fk_defects_story`), igual que la de la 005. La prueba de integración de la
   carrera lo fija.
3. **La 008 consume esta tabla**: `detected_sprint_id`, `resolved_sprint_id`, `status` y `severity`
   con índices por sprint. Esta feature no hace ninguna agregación (RC-09).
