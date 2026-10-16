---
name: "speckit-converge"
description: "Assess the current codebase against the feature's spec, plan, and tasks, then append any remaining unbuilt work as new tasks to tasks.md so implement can complete it."
compatibility: "Requires spec-kit project structure with .specify/ directory"
metadata:
  author: "github-spec-kit"
  source: "templates/commands/converge.md"
user-invocable: true
disable-model-invocation: true
---


## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Pre-Execution Checks

**Check for extension hooks (before convergence)**:

- Check if `.specify/extensions.yml` exists in the project root.
- If it exists, read it and look for entries under the `hooks.before_converge` key
- If the YAML cannot be parsed or is invalid, skip hook checking silently and continue normally
- Filter out hooks where `enabled` is explicitly `false`. Treat hooks without an `enabled` field as enabled by default.
- For each remaining hook, do **not** attempt to interpret or evaluate hook `condition` expressions:
  - If the hook has no `condition` field, or it is null/empty, treat the hook as executable
  - If the hook defines a non-empty `condition`, skip the hook and leave condition evaluation to the HookExecutor implementation
- When constructing command invocations from hook command names, replace dots (`.`) with hyphens (`-`). For example, `speckit.git.commit` → `/speckit-git-commit`.
- For each executable hook, output the following based on its `optional` flag:
  - **Optional hook** (`optional: true`):

    ```text
    ## Extension Hooks

    **Optional Pre-Hook**: {extension}
    Command: `/{command}`
    Description: {description}

    Prompt: {prompt}
    To execute: `/{command}`
    ```

  - **Mandatory hook** (`optional: false`):

    ```text
    ## Extension Hooks

    **Automatic Pre-Hook**: {extension}
    Executing: `/{command}`
    EXECUTE_COMMAND: {command}

    Wait for the result of the hook command before proceeding to the Goal.
    ```
    After emitting the block above you MUST actually invoke the hook and wait for it to finish before continuing. Run it the same way you would run the command yourself in this agent/session (the invocation may differ from the literal `{command}` id shown above, e.g. a skills-mode agent runs it as `/skill:speckit-...` or `$speckit-...`). Emitting the block alone does not run the hook.

- If no hooks are registered or `.specify/extensions.yml` does not exist, skip silently

## Goal

Cerrar la brecha entre lo que piden la spec, el plan y las tareas **dentro del alcance de la rama
actual** (una historia de usuario o una fase técnica; ver paso 1) y lo que el código implementa hoy.
`spec.md`, `plan.md` y `tasks.md` son la **única fuente de intención**, con la constitución como
restricción de gobierno. Se evalúa el estado actual del código del alcance, se determina qué
requisitos, escenarios de aceptación, decisiones del plan y tareas existentes no se cumplen o se
cumplen solo en parte, y cada trabajo pendiente se **agrega como una tarea nueva y trazable** en el
destino que le corresponde (la sección de la historia o fase, o la sección Convergence and
Security; ver paso 6), para que `/speckit-implement` la complete. Este comando DEBE ejecutarse solo
después de que `/speckit-tasks` produjo un `tasks.md` completo y de que `/speckit-implement` se
ejecutó sobre las tareas del alcance.

Los hallazgos se basan en el estado actual del código, no en su historial. Git se usa únicamente en
modo lectura y solo para tres cosas: identificar la rama (`git branch --show-current`), saber qué
archivos agregó o modificó la rama (`git diff origin/main...HEAD --name-only`) y comprobar si una
tarea ya tiene commits (`git log --all --oneline --grep="Spec: NNN/Txxx"`). Está prohibida toda
operación de escritura de Git.

## Operating Constraints

**SOLO AGREGAR, NUNCA REESCRIBIR**: la única escritura del comando es agregar tareas a `tasks.md`,
en la sección de la historia o fase del alcance o en la sección Convergence and Security (creándola
si no existe, según el contrato de abajo). NO DEBE:

- modificar `spec.md` ni `plan.md` de ninguna forma;
- crear encabezados o secciones, salvo la sección Convergence and Security cuando no existe;
- reescribir, renumerar, reordenar ni borrar tareas o fases existentes. La única excepción son las
  tareas de la sección Convergence and Security desde la rama `fase/NNN-convergence-security`, según
  el contrato;
- modificar, crear ni borrar código de la aplicación: completar las tareas agregadas es trabajo de
  `/speckit-implement`.

Si el alcance ya cumple todo, el comando DEBE dejar `tasks.md` **sin cambiar ni un byte** (sin
crear una sección Convergence and Security vacía) y reportar un resultado limpio.

**Constitution Authority**: The project constitution (`.specify/memory/constitution.md`) is
**non-negotiable**. Code that violates a MUST principle is the highest-severity finding and
produces a corresponding remediation task. If the constitution is an unfilled template,
skip constitution checks gracefully rather than failing.

### Contrato de la sección Convergence and Security

Este contrato es compartido con `/revisar-seguridad`; no se modifica en una sola de las dos skills.

- **Encabezado exacto**: `## Phase N: Convergence and Security`. Identificador de fase para
  `sync-github`: `convergence-security`. Debajo del encabezado, una línea:
  `Propósito: ajustes transversales detectados por /speckit-converge y /revisar-seguridad.`
- **Una sola por `tasks.md`**. Si no existe, se crea con N = número de fase más alto + 1, contando
  los encabezados `## Fase N` y `## Phase N`. Se ubica después de la última fase, justo antes de
  `## Dependencias y Orden de Ejecución`; si ese encabezado no existe, al final del archivo. Nunca
  se renumeran fases existentes.
- **Formato de tarea**:
  `- [ ] Txxx <descripción imperativa con rutas de archivo> per <origen> (<tipo>) — origen: converge, detectado en <rama>`.
  `<origen>` es `FR-xxx`, `SC-xxx`, `NNN/USn ACn`, `plan: <decisión>` o `Constitución <principio>`.
  `<tipo>` es `missing`, `partial`, `contradicts` o `unrequested`.
- Las tareas de esta sección **NUNCA** llevan la etiqueta `[USn]`, ni siquiera si el origen es una
  historia (`sync-github` las clasificaría mal). La historia se cita como texto: `per 001/US1 AC2`.
- **IDs**: máximo de todo `tasks.md` + 1, consecutivos, con tres dígitos.
- **Desde cualquier rama que no sea `fase/NNN-convergence-security`**: solo se agregan tareas.
  Antes de agregar, cada propuesta se compara con las tareas existentes de la sección (pendientes o
  `[X]`): si ya hay una que cubre el mismo problema en el mismo archivo o área, no se agrega y se
  informa "ya cubierto por Txxx". Nunca se editan ni se quitan tareas.
- **Desde la rama `fase/NNN-convergence-security`**: además se pueden editar o quitar tareas de esta
  sección, solo si no están marcadas `[X]` y no tienen commits (verificar con
  `git log --all --oneline --grep="Spec: NNN/Txxx"`). Toda edición o eliminación se lista aparte
  para que el Agile Enabler la sincronice.

## Execution Steps

### 1. Initialize Convergence Context

**Determinar el alcance por rama.** Ejecutar `git branch --show-current`:

- `hu/NNN-usX-...`: feature `specs/NNN-*/`. Alcance = historia USx: sus escenarios de aceptación,
  los FR que usa y las tareas de su sección en `tasks.md`.
- `fase/NNN-convergence-security`: feature `specs/NNN-*/`. Alcance = la sección Convergence and
  Security, con la feature completa como contexto.
- `fase/NNN-<otro slug>`: feature `specs/NNN-*/`. Alcance = la fase técnica cuyo título corresponde
  al slug.
- Cualquier otra rama: detenerse y preguntar al desarrollador qué historia o fase analizar. Nunca
  analizar la feature completa salvo en `fase/NNN-convergence-security`.

Run `.specify/scripts/powershell/check-prerequisites.ps1 -Json -RequireSpec -RequireTasks -IncludeTasks` once from repo root and parse JSON for FEATURE_DIR and AVAILABLE_DOCS. Si el script falla o el `FEATURE_DIR` que devuelve no corresponde a la carpeta `specs/NNN-*/` del número de la rama, resolver las rutas desde `specs/NNN-*/`. Derive absolute paths:

- SPEC = FEATURE_DIR/spec.md
- PLAN = FEATURE_DIR/plan.md
- TASKS = FEATURE_DIR/tasks.md
- CONSTITUTION = `.specify/memory/constitution.md` (if present)
If `spec.md`, `plan.md`, or `tasks.md` is missing, STOP with a clear, actionable message naming the
prerequisite command to run (`/speckit-specify` for a missing spec, `/speckit-plan` for a missing plan,
`/speckit-tasks` for missing tasks). Do not produce partial output.
For single quotes in args like "I'm Groot", use escape syntax: e.g 'I'\''m Groot' (or double-quote if possible: "I'm Groot").

Ubicar en `tasks.md` la sección del alcance. Si no se encuentra (salvo la sección Convergence and
Security, que puede no existir todavía), detenerse y preguntar. Obtener además la lista de archivos
que la rama agregó o modificó con `git diff origin/main...HEAD --name-only`.

### 2. Load Artifacts (Progressive Disclosure)

Load only the minimal necessary context from each artifact:

**From spec.md:**

- Functional Requirements (FR-###)
- Success Criteria (SC-###) — include only items requiring buildable work; exclude
  post-launch outcome metrics and business KPIs
- User Stories and their Acceptance Scenarios
- Edge Cases (if present)

**From plan.md:**

- Architecture/stack choices and technical decisions
- Data Model references
- Phases and named touch-points (files/components the plan says will be created or edited)
- Technical constraints

**From tasks.md:**

- Task IDs (to compute the next ID and next phase number)
- Descriptions, phase grouping, and referenced file paths
- Las tareas actuales de la sección Convergence and Security, si existe (pendientes y `[X]`), para
  detectar propuestas ya cubiertas

**From constitution (if not an unfilled template):**

- Principle names and MUST/SHOULD normative statements

### 3. Build the Intent Inventory

Crear un modelo interno (sin repetir los artefactos en crudo), limitado al alcance del paso 1:

- **Inventario de requisitos**: una clave estable por cada FR-### / SC-### / escenario de
  aceptación del alcance (por ejemplo `001/US1 AC2`), más las decisiones del plan y los principios
  de la constitución que imponen obligaciones construibles sobre ese alcance. En una historia, el
  inventario incluye solo sus escenarios, los FR que usa y las tareas de su sección; en una fase
  técnica, las tareas de esa fase. La feature completa entra en el inventario solo en la rama
  `fase/NNN-convergence-security`.
- **Mapa de código del alcance**: a partir de las rutas que nombran `plan.md` y las tareas del
  alcance, más una búsqueda por palabras clave de los conceptos de cada requisito, derivar los
  archivos y componentes a evaluar. La evaluación se limita a ellos: **no** se infiere alcance más
  allá de lo que definen los artefactos.
- El trabajo pendiente de otras historias o fases no es una brecha: no se reporta.

### 4. Assess the Codebase and Classify Findings

For each item in the intent inventory, inspect the current code in scope and produce a
`Finding` only where there is a gap. Classify every finding by **gap type**:

- **`missing`**: the required work is absent from the code entirely.
- **`partial`**: the work exists but does not yet fully satisfy the requirement /
  acceptance criterion / plan decision.
- **`contradicts`**: the code does something that conflicts with stated intent or a
  constitution MUST principle.
- **`unrequested`**: the code contains work not called for by the spec, plan, or tasks
  (surfaced for awareness — converge does **not** delete code, it only appends a task to
  review/justify or remove it).

Each `Finding` records: a stable id, the `source-ref` it traces to, the `gap-type`, a
severity, and a short human-readable description with the evidence (the file/area observed).

**Casos límite:**

- **Poco o nada de código todavía**: tratar todo el alcance de la rama como trabajo `missing`
  pendiente, en lugar de fallar.
- **No queda nada pendiente**: cero hallazgos; seguir la rama "converged" del paso 7.

### 5. Assign Severity

- **CRITICAL**: violates a constitution MUST principle, or a `missing`/`contradicts` gap
  that blocks baseline functionality of a P1 user story.
- **HIGH**: a `missing` or `partial` gap on a core functional requirement or acceptance
  criterion.
- **MEDIUM**: a `partial` gap on a secondary requirement, or an `unrequested` addition with
  unclear justification.
- **LOW**: minor partial gaps, polish, or low-risk `unrequested` additions.

### 6. Presentar los hallazgos y pedir confirmación

**Destino de cada hallazgo:**

- `missing` o `partial` dentro del alcance de una historia o fase → al final de la sección de esa
  historia o fase, después de su última tarea y antes de su texto de cierre (por ejemplo
  "Checkpoint") o del encabezado siguiente. En una historia, con `[USx]`. No se crean encabezados.
- `contradicts` de severidad CRITICAL en código que la rama agregó o modificó (archivos del
  `git diff` del paso 1) → mismo destino que el punto anterior.
- Resto de `contradicts`, todo `unrequested` y las brechas transversales (el mismo problema afecta
  código fuera del alcance de la rama) → sección Convergence and Security.
- En la rama `fase/NNN-convergence-security`, todo va a la sección Convergence and Security.
- Una propuesta para Convergence and Security que ya está cubierta por una tarea existente de esa
  sección → "ya cubierto por Txxx"; no se agrega.

Antes de escribir nada, mostrar un resumen por severidad (sin escribir archivos todavía):

## Convergence Findings

| ID | Gap Type | Severity | Source | Evidence | Remaining Work | Destino |
|----|----------|----------|--------|----------|----------------|---------|
| F1 | missing  | HIGH     | FR-008 | Ejemplo: no hay validación en path/to/handler.go | Agregar la validación | Fase 3: Historia de Usuario 1 |
| F2 | unrequested | MEDIUM | plan: contrato de la API | Ruta pública extra en path/to/main.go | Justificar o quitar la ruta | Convergence and Security |
| F3 | partial  | LOW      | Constitución VIII | Comentario en inglés en path/to/file.go | Traducir el comentario | ya cubierto por T097 |

En la rama `fase/NNN-convergence-security`, listar aparte las ediciones y eliminaciones propuestas
de tareas de la sección, con el resultado de la verificación de `[X]` y de commits de cada una.

**Métricas del resumen:**

- Alcance analizado (rama, historia o fase)
- Requisitos y escenarios de aceptación revisados
- Decisiones del plan revisadas
- Principios de la constitución revisados (o "omitido — plantilla")
- Hallazgos por tipo de brecha (missing / partial / contradicts / unrequested)
- Hallazgos por severidad
- Hallazgos por destino

Preguntar al desarrollador qué aprueba. Puede quitar tareas, ajustar su redacción o cambiar su
destino. Escribir solo después de su confirmación explícita.

### 7. Agregar las tareas aprobadas (o reportar converged)

**Si hay uno o más hallazgos aprobados** (resultado `tasks_appended`):

1. Recorrer todos los IDs de `tasks.md`; sea `M` el máximo. Asignar IDs consecutivos con tres
   dígitos `T{M+1:03d}, T{M+2:03d}, …`, ordenando las tareas CRITICAL y HIGH primero y, entre ellas,
   las violaciones de la constitución al comienzo, descritas como `CRITICAL`. Nunca reutilizar ni
   renumerar IDs existentes.
2. **Tareas para la sección de la historia o fase**: agregarlas después de la última tarea de esa
   sección y antes de su texto de cierre o del encabezado siguiente, con este formato:

   ```markdown
   - [ ] T042 [USx] <descripción imperativa con rutas de archivo> per <origen> (<tipo>)
   ```

   En una fase técnica, sin `[USx]`. `<origen>` es `FR-xxx`, `SC-xxx`, `NNN/USn ACn`,
   `plan: <decisión>` o `Constitución <principio>`; `<tipo>` es `missing`, `partial`,
   `contradicts` o `unrequested`.
3. **Tareas para la sección Convergence and Security**: si la sección no existe, crearla según el
   contrato (encabezado, línea de propósito y ubicación). Agregar las tareas después de su última
   tarea, con el formato del contrato y sin `[USn]`.
4. **Solo en la rama `fase/NNN-convergence-security`**: aplicar las ediciones y eliminaciones
   aprobadas de tareas de la sección, únicamente si la tarea no está marcada `[X]` y
   `git log --all --oneline --grep="Spec: NNN/Txxx"` no devuelve commits.

**Si no hay hallazgos aprobados** (resultado `converged`):

- **No** modificar `tasks.md`, ni siquiera con una sección vacía.
- Reportar: **"✅ Converged — la implementación del alcance cumple la spec, el plan y las tareas."**
- Incluir los conteos del resumen de lo revisado.

### 8. Cierre

- **Si se agregaron tareas** (`tasks_appended`):
  - Listar los IDs agregados y la sección de cada uno.
  - Indicar al desarrollador que commitee `tasks.md` con tipo `docs(ámbito)` y sin pies (propuesta
    del mensaje con `/redactar-commit`), haga push y le pida al Agile Enabler que ejecute
    `/sincronizar-github tareas NNN`.
  - Tareas en la sección de la historia o fase: implementarlas en esta rama con
    `/speckit-implement Txxx` antes del PR.
  - Tareas en Convergence and Security: no bloquean el PR; se implementan en
    `fase/NNN-convergence-security` cuando el equipo la planifique.
  - En la rama `fase/NNN-convergence-security`: listar aparte las tareas editadas o quitadas, para
    que el Agile Enabler las sincronice.
- **Nunca** recomendar `/speckit-implement` sobre la feature completa.
- **Si no hay brechas en el alcance** (`converged`): indicar que puede seguir con `/redactar-pr`.

### 9. Check for extension hooks

After producing the result, check if `.specify/extensions.yml` exists in the project root.

- If it exists, read it and look for entries under the `hooks.after_converge` key
- If the YAML cannot be parsed or is invalid, skip hook checking silently and continue normally
- Filter out hooks where `enabled` is explicitly `false`. Treat hooks without an `enabled` field as enabled by default.
- For each remaining hook, do **not** attempt to interpret or evaluate hook `condition` expressions:
  - If the hook has no `condition` field, or it is null/empty, treat the hook as executable
  - If the hook defines a non-empty `condition`, skip the hook and leave condition evaluation to the HookExecutor implementation
- Report the convergence outcome (`converged` or `tasks_appended`) in-session before listing
  any hooks, so users can decide whether to run optional follow-up commands.
- When constructing command invocations from hook command names, replace dots (`.`) with hyphens (`-`). For example, `speckit.git.commit` → `/speckit-git-commit`.
- For each executable hook, output the following based on its `optional` flag:
  - **Optional hook** (`optional: true`):

    ```text
    ## Extension Hooks

    **Optional Hook**: {extension}
    Command: `/{command}`
    Description: {description}

    Prompt: {prompt}
    To execute: `/{command}`
    ```

  - **Mandatory hook** (`optional: false`):

    ```text
    ## Extension Hooks

    **Automatic Hook**: {extension}
    Executing: `/{command}`
    EXECUTE_COMMAND: {command}
    ```
    After emitting the block above you MUST actually invoke the hook and wait for it to finish before continuing. Run it the same way you would run the command yourself in this agent/session (the invocation may differ from the literal `{command}` id shown above, e.g. a skills-mode agent runs it as `/skill:speckit-...` or `$speckit-...`). Emitting the block alone does not run the hook.

- If no hooks are registered or `.specify/extensions.yml` does not exist, skip silently

## Adaptaciones del proyecto

Una actualización de Spec Kit puede sobrescribir este archivo; en ese caso hay que volver a aplicar
estas adaptaciones.

- Frontmatter: `disable-model-invocation: true`.
- Alcance por rama (`hu/NNN-usX-...`, `fase/NNN-convergence-security`, `fase/NNN-<slug>`, otra rama → preguntar); la feature completa solo se analiza en `fase/NNN-convergence-security`.
- Si el script de prerrequisitos no resuelve la feature por la rama, las rutas se toman de `specs/NNN-*/`.
- El trabajo pendiente de otras historias no es una brecha y no se reporta.
- Git solo en lectura: `branch --show-current`, `diff origin/main...HEAD --name-only` y `log --all --grep`.
- Destino por hallazgo: missing/partial del alcance y contradicts CRITICAL del diff van a la sección de la historia o fase; el resto va a Convergence and Security.
- Contrato de la sección `## Phase N: Convergence and Security` (compartido con `/revisar-seguridad`), que reemplaza la `## Phase N: Convergence` original.
- La tabla de hallazgos incluye la columna "Destino" y la escritura requiere confirmación explícita del desarrollador.
- El cierre reemplaza el handoff original: commit `docs(ámbito)` sin pies, `/sincronizar-github tareas NNN` por el Agile Enabler, `/speckit-implement Txxx` solo para tareas del alcance y `/redactar-pr` si no hay brechas.
