---
name: revisar-seguridad
description: Revisa la seguridad del código de la rama actual (historia de usuario o fase técnica) con el subagente security-reviewer, presenta los hallazgos y, con la aprobación del desarrollador, agrega a tasks.md las tareas de prueba y corrección. Usar únicamente cuando el desarrollador ejecute /revisar-seguridad.
disable-model-invocation: true
allowed-tools: Read, Grep, Glob, Bash(git branch --show-current), Bash(git diff:*), Bash(git log:*)
---

# Revisar seguridad

Orquestás la revisión de seguridad de la rama actual. El análisis lo hace el subagente `security-reviewer`. Vos presentás los resultados y, solo con la aprobación del desarrollador, agregás las tareas a `tasks.md`.

## Reglas

- La única escritura permitida es en `tasks.md`: agregar las tareas aprobadas y crear la sección Convergence and Security si no existe. No modificás código, pruebas ni otros archivos.
- Nunca reescribís, renumerás, reordenás ni borrás tareas o fases existentes. La única excepción son las tareas de la sección Convergence and Security desde la rama de corrección, según el contrato de abajo.
- No creás issues: eso lo hace el Agile Enabler con `/sincronizar-github`.
- Sin operaciones de Git de escritura.

### Contrato de la sección Convergence and Security

Este contrato es compartido con `/speckit-converge`; no se modifica en una sola de las dos skills.

- **Encabezado exacto**: `## Phase N: Convergence and Security`. Debajo del encabezado, una línea:
  `Propósito: ajustes transversales detectados por /speckit-converge y /revisar-seguridad.`
- **Una sola por `tasks.md`**. Si no existe, se crea con N = número de fase más alto + 1, contando los encabezados `## Fase N` y `## Phase N`. Se ubica después de la última fase, justo antes de `## Dependencias y Orden de Ejecución`; si ese encabezado no existe, al final del archivo. Nunca se renumeran fases existentes.
- **Formato de tarea**:
  `- [ ] Txxx <descripción imperativa con rutas de archivo> per hallazgo de seguridad: <descripción corta del hallazgo> (security) — origen: seguridad, detectado en <NNN/USx o fase>`.
- Las tareas de esta sección **nunca** llevan `[USn]`; la historia se cita solo como texto.
- **IDs**: máximo de todo `tasks.md` + 1, consecutivos, con tres dígitos. Cada hallazgo genera el par prueba → corrección, en ese orden (o solo corrección si no es testeable).
- **Rama de corrección**: la rama `fase/NNN-<slug>` cuya fase en `tasks.md` es Convergence and Security (identificador `convergence-security`).
- **Desde cualquier otra rama**: solo se agregan tareas, con control de duplicados contra las existentes de la sección (pendientes o `[X]`): si una ya cubre la misma vulnerabilidad en el mismo archivo o área, no se agrega y se informa "ya cubierto por Txxx". Nunca se editan ni se quitan tareas.
- **Desde la rama de corrección**: además se pueden editar o quitar tareas de la sección, solo si no están marcadas `[X]` y no tienen commits (`git log --all --oneline --grep="Spec: NNN/Txxx"`). Toda edición o eliminación se lista aparte para el Agile Enabler.

## Paso 1. Determinar el alcance

Ejecutá `git branch --show-current`:

- `hu/NNN-usX-...`: feature `specs/NNN-*/`, alcance = historia USx.
- Rama de corrección (`fase/NNN-<slug>` cuya fase es Convergence and Security): feature `specs/NNN-*/`, alcance = la sección Convergence and Security, en **modo rama de corrección**.
- `fase/NNN-<slug>`: feature `specs/NNN-*/`, alcance = la fase técnica cuyo título corresponde al slug.
- Cualquier otra rama: detenete y preguntá al desarrollador qué historia o fase revisar.

Ubicá en `tasks.md` la sección del alcance. Si no la encontrás, detenete y preguntá. Ubicá también la sección Convergence and Security, si existe, para el control de duplicados.

Si `git diff origin/main...HEAD --stat` no muestra cambios de código, informalo y terminá, salvo en la rama de corrección: ahí seguí igual para verificar las tareas `[X]` de la sección.

## Paso 2. Invocar al revisor

Invocá al subagente `security-reviewer`. Indicale la carpeta de la feature, el alcance (historia USx o fase) y, si corresponde, que está en modo rama de corrección.

## Paso 3. Presentar los resultados

Asigná un destino a cada hallazgo:

- `CRÍTICA` o `ALTA` con alcance "rama" → sección de la historia o fase.
- Todo hallazgo `MEDIA` o `BAJA`, y todo hallazgo de alcance "transversal" o "fuera de la rama" → sección Convergence and Security.
- `CRÍTICA` o `ALTA` de alcance "transversal" o "fuera de la rama" → sección Convergence and Security, con una **advertencia destacada**: la vulnerabilidad ya está en main; recomendá al desarrollador avisar al Agile Enabler para que decida si se trata antes de la ronda de corrección.
- En la rama de corrección, todo va a Convergence and Security.
- Si el destino es Convergence and Security y una tarea existente de esa sección ya cubre la misma vulnerabilidad en el mismo archivo o área: "ya cubierto por Txxx" si está `[X]`, o "pendiente en Txxx" si está pendiente. No se agrega y no bloquea.

Mostrale al desarrollador, sin modificar nada:

1. La tabla de hallazgos, de mayor a menor severidad, con las columnas "Alcance" (rama / transversal / fuera de la rama) y "Destino" (sección de la historia o fase / Convergence and Security / ya cubierto por Txxx / pendiente en Txxx), y las advertencias destacadas que correspondan.
2. Las tareas propuestas, agrupadas por hallazgo, ya armadas según su destino (ver Paso 4).
3. En la rama de corrección: la tabla "Verificación de correcciones" del subagente y, aparte, las ediciones o eliminaciones de tareas de la sección que se propongan.
4. Las observaciones del subagente.

Preguntá qué aprueba. Puede aprobar todas, algunas o ninguna, quitar tareas, pedir cambios en su redacción o cambiar su destino. Si mueve un hallazgo `CRÍTICA` o `ALTA` de alcance "rama" a Convergence and Security, advertile que la vulnerabilidad llega a main.

## Paso 4. Agregar las tareas aprobadas

Solo después de la confirmación explícita:

1. Calculá el ID máximo de todo `tasks.md` y asigná IDs consecutivos desde el siguiente (`Txxx`, con tres dígitos).
2. Conservá el orden de cada par: primero la tarea de prueba y después la de corrección.
3. Armá cada línea a partir de la tarea neutra del subagente, según su destino:
   - **Sección de una historia**: `- [ ] Txxx [USx] <tarea neutra>`. En una fase técnica, sin etiqueta: `- [ ] Txxx <tarea neutra>`. Agregalas al final de la sección, después de su última tarea y antes de cualquier texto de cierre (por ejemplo, un "Checkpoint") o del encabezado siguiente. No crees secciones ni encabezados nuevos.
   - **Convergence and Security**: `- [ ] Txxx <tarea neutra> — origen: seguridad, detectado en <NNN/USx o fase>`, nunca con `[USn]`. Si la sección no existe, creala según el contrato. Agregalas después de su última tarea.
4. En la rama de corrección, aplicá las ediciones o eliminaciones aprobadas solo si la tarea no está marcada `[X]` y `git log --all --oneline --grep="Spec: NNN/Txxx"` no devuelve commits.

## Paso 5. Próximos pasos

Indicale al desarrollador:

1. Que commitee `tasks.md` con tipo `docs(ámbito)` y sin pies (propuesta del mensaje con `/redactar-commit`), haga push y le pida al Agile Enabler que ejecute `/sincronizar-github tareas NNN`. El desarrollador no ejecuta `/sincronizar-github`.
2. Tareas en la sección de la historia o fase: implementarlas en esta rama con `/speckit-implement Txxx`, una por vez y en orden (primero la de prueba, RED; después la de corrección, GREEN), y volver a ejecutar `/revisar-seguridad` antes del PR.
3. Tareas en Convergence and Security: no bloquean el PR; mencionarlas en las notas del PR.
4. En la rama de corrección: mostrar la tabla de verificación; una tarea marcada `[X]` cuya vulnerabilidad persiste bloquea el PR. Listar aparte las tareas editadas o quitadas para que el Agile Enabler las sincronice.

Solo bloquean el PR los hallazgos `CRÍTICA` o `ALTA` de alcance "rama" sin corregir (y, en la rama de corrección, las tareas `[X]` no corregidas). Un hallazgo cubierto por una tarea pendiente de Convergence and Security se informa como "pendiente en Txxx" y no bloquea.

Si no hubo hallazgos, indicá que la rama puede seguir hacia `speckit-converge` (opcional) y `/redactar-pr`.

## Adaptaciones del proyecto

- Contrato compartido con `/speckit-converge` de la sección `## Phase N: Convergence and Security`, ubicada antes de `## Dependencias y Orden de Ejecución`.
- Alcance por rama ampliado con la rama de corrección (fase Convergence and Security), en la que el subagente trabaja en modo rama de corrección.
- Si el diff está vacío en la rama de corrección, se sigue para verificar las tareas `[X]`.
- Destino: `CRÍTICA`/`ALTA` de alcance "rama" a la sección de la historia o fase; el resto a Convergence and Security; en la rama de corrección, todo a Convergence and Security.
- Advertencia destacada para `CRÍTICA`/`ALTA` "transversal" o "fuera de la rama" (ya está en main) y al mover una `CRÍTICA`/`ALTA` "rama" a Convergence and Security.
- Las líneas se arman desde la tarea neutra del subagente: `[USx]` solo en la sección de una historia; `— origen: seguridad, detectado en …` en Convergence and Security.
- La tabla de hallazgos incluye las columnas "Alcance" y "Destino" y el desarrollador puede cambiar el destino.
- Control de duplicados: "ya cubierto por Txxx" o "pendiente en Txxx", que no bloquea.
- Ediciones y eliminaciones de tareas de la sección solo en la rama de corrección, sin `[X]` ni commits, listadas aparte para el Agile Enabler.
- Próximos pasos: commit `docs(ámbito)` sin pies, push y `/sincronizar-github` a cargo del Agile Enabler; solo bloquean las `CRÍTICA`/`ALTA` "rama" sin corregir y las `[X]` no corregidas.
