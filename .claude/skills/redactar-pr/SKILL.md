---
name: redactar-pr
description: Redacta el Pull Request de la rama actual (historia de usuario o fase técnica) con la plantilla del repositorio y, solo tras la confirmación explícita del desarrollador, lo crea en GitHub contra main. Usar únicamente cuando el desarrollador ejecute /redactar-pr.
disable-model-invocation: true
argument-hint: "[número del issue de la historia, opcional]"
allowed-tools: Read, Grep, Glob, Bash(git branch --show-current), Bash(git status:*), Bash(git log:*), Bash(git diff:*), Bash(git rev-parse:*), Bash(git rev-list:*), Bash(git remote get-url:*), mcp__github__list_issues, mcp__github__issue_read, mcp__github__list_pull_requests
---

# Redactar Pull Request

Redactás el Pull Request que cierra la rama actual. Trabajás en la sesión principal porque el desarrollador revisa y corrige la redacción con vos antes de publicarla.

## Reglas que no se negocian

- Solo podés usar Git en modo lectura: `branch --show-current`, `status`, `log`, `diff`, `rev-parse`, `rev-list` y `remote get-url`. Nunca ejecutes `commit`, `push`, `pull`, `fetch`, `merge`, `rebase`, `checkout`, `switch`, `reset`, `stash` ni `tag`. Si hace falta alguna de esas operaciones, se la indicás al desarrollador.
- La única operación de escritura permitida en GitHub es `create_pull_request`, y solo después de la confirmación explícita del paso 8.
- Nunca mergees, apruebes, cierres ni modifiques un Pull Request. Nunca crees, edites ni cierres issues.
- Todo el texto del PR va en español, sin emojis.
- No inventes datos. Si algo no se puede determinar (una prueba que cubra un escenario, un resultado de pruebas), lo decís explícitamente.

## Paso 1. Identificar la rama y el issue

1. Ejecutá `git branch --show-current`. Si es `main`, detenete: el PR se crea desde una rama de historia.
2. Deducí el elemento a partir del nombre de la rama:
   - `hu/NNN-usX-...` → historia de usuario `NNN/USx`.
   - `fase/NNN-setup`, `fase/NNN-foundational` o `fase/NNN-polish` → fase técnica.
3. Si se pasó un número de issue como argumento (`$ARGUMENTS`), ese issue tiene prioridad sobre el nombre de la rama.
4. Si no podés deducir el elemento, preguntale al desarrollador y no sigas.

## Paso 2. Verificar precondiciones

Comprobá todas y reportá juntas las que fallen:

| Precondición | Comando | Esperado |
| --- | --- | --- |
| Sin cambios sin commitear | `git status --porcelain` | Salida vacía |
| La rama existe en el remoto | `git rev-parse --abbrev-ref @{u}` | Devuelve `origin/<rama>` |
| Todo está pusheado | `git rev-list --count @{u}..HEAD` | `0` |
| La rama incluye lo último de main | `git rev-list --count HEAD..origin/main` | `0` |
| Hay commits propios | `git rev-list --count origin/main..HEAD` | Mayor que `0` |

Si alguna falla, indicá qué tiene que hacer el desarrollador y detenete. Para actualizar con main, la indicación es `git fetch origin`, `git merge origin/main`, correr las pruebas y `git push`. Nunca sugieras `rebase`.

Recordale además que los datos de `origin/main` son los del último `git fetch` que hizo.

## Paso 3. Leer el issue y sus sub-issues

1. Con `list_issues`, filtrando por las etiquetas `historia-usuario` (o `fase-tecnica`) y `spec:NNN`, en todos los estados, buscá el issue cuya primera línea del cuerpo sea la marca `<!-- speckit:NNN/USx -->` (o `<!-- speckit:NNN/fase:setup -->`). Nunca uses `search_issues`.
2. Con `issue_read`, leé el issue y sus sub-issues. De cada sub-issue tomá número, título, estado e ID de tarea (de la marca `<!-- speckit:NNN/Txxx -->`).
3. Con `list_pull_requests`, verificá que no exista un PR abierto desde esta rama. Si existe, informá su enlace y detenete.

## Paso 4. Analizar los commits

1. Ejecutá:
   `git log origin/main..HEAD --no-merges --reverse --format="%h%x1f%s%x1f%b%x1e"`
2. De cada commit extraé hash, primera línea y los pies `Spec:`, `TDD:`, `Closes #N` y `Refs #N`.
3. Registrá estas observaciones:
   - Sub-issues sin un commit con `Closes #N`. **Es crítica**: preguntale al desarrollador si continúa. Si continúa, esas tareas van en "Pendientes y fuera de alcance".
   - Commits cuya primera línea no cumple `tipo(ámbito): descripción`, supera 72 caracteres o no tiene el pie `Spec:`.
   - Commits con `Closes #N` sobre issues que no son sub-issues de esta historia, o sobre la propia historia.

No propongas reescribir commits: la historia no se reescribe. Las observaciones quedan informadas al revisor en "Notas para el revisor".

## Paso 5. Analizar los cambios

1. `git diff --stat origin/main...HEAD` para el panorama general.
2. Revisá el diff de `go.mod`, `package.json`, `.env.example`, migraciones y registro de rutas para completar "Cambios técnicos relevantes".
3. Si hay dependencias nuevas, verificá si están autorizadas según la constitución. Si no podés confirmarlo, marcalo en "Notas para el revisor".

## Paso 6. Criterios de aceptación y pruebas

1. Leé la historia en `specs/NNN-.../spec.md` y sus escenarios de aceptación.
2. Para cada escenario, buscá con Grep en los archivos de prueba modificados (`*_test.go`, `*.feature`, pruebas del frontend) la prueba que lo cubre. Si no la encontrás, escribí "sin prueba identificada"; no la supongas.
3. Pedile al desarrollador la salida de los comandos de pruebas y cobertura definidos en el README. No ejecutes las pruebas vos. Si no los aporta, la sección "Pruebas" queda con "A completar por el autor" y lo indicás como observación.

## Paso 7. Redactar

1. Leé `.github/pull_request_template.md` y completá todas sus secciones. Eliminá los comentarios HTML de instrucciones.
2. Título: `tipo(ámbito): descripción (NNN/USx)`, de 72 caracteres o menos. El tipo suele ser `feat` para historias y `chore` para fases técnicas, y el ámbito es el del módulo, según el formato de commits de la constitución (Principio IX, ámbito b).
3. En "Historia de usuario" va `Closes #N` con el número del issue de la historia. Las sub-issues no van ahí, porque las cierran sus commits.
4. Las tablas de tareas y de evidencia TDD se arman con los datos de los pasos 3 y 4.
5. Mostrale al desarrollador, en este orden:
   - Las observaciones de los pasos 2 a 6, si hay.
   - El título.
   - El cuerpo completo dentro de un bloque de código Markdown.
6. Terminá preguntando si quiere cambios o si confirma la creación del PR.
7. Aplicá los cambios que pida y volvé a mostrar la versión completa.

## Paso 8. Crear el Pull Request

1. Creá el PR solo si el desarrollador confirma de forma explícita (por ejemplo, "confirmo" o "crear el PR"). Ante respuestas ambiguas, preguntá de nuevo.
2. Obtené owner y repositorio con `git remote get-url origin`.
3. Llamá a `create_pull_request` con `head` = la rama actual, `base` = `main`, el título y el cuerpo aprobados, y `draft` = `false`.
4. No asignes revisores: los asigna GitHub mediante `CODEOWNERS`.
5. Informá el enlace del PR y recordale al desarrollador que, si hace cambios pedidos en la revisión, debe agregar commits nuevos y pushearlos; el PR se actualiza solo.
6. Si la creación falla, mostrá el error tal como llegó y no reintentes con otros parámetros sin preguntar.
