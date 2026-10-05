# CLAUDE.md

Guía operativa para Claude Code en este repositorio. Ante cualquier discrepancia, prevalece `.specify/memory/constitution.md`.

## Código

Las reglas de código están en la constitución, que se carga completa en cada sesión:

@.specify/memory/constitution.md

La constitución prevalece sobre cualquier skill, propia o de terceros. En particular:

- Pruebas: de integración por defecto; unitarias obligatorias para métricas, estimación, reglas de negocio y validaciones (Principio II). Cada escenario de la spec se implementa como prueba nombrada según la convención de BDD del Principio II.
- TDD: después de verificar que una prueba falla, detenerse e indicar al desarrollador que registre el commit RED con `/redactar-commit` antes de implementar. Lo mismo al terminar GREEN y REFACTOR.
- Comentarios: en español, uno por bloque, no por línea; identificadores en inglés (Principio VIII).
- Persistencia: GORM; no usar sqlc ni query builders (Principio IV).
- Dependencias fuera de la tabla del Principio IV: pedir autorización antes de instalar.
- Next.js: antes de escribir código de Next.js, leer la documentación de la versión instalada en `frontend/node_modules/next/dist/docs/`. Si una herramienta genera un archivo `AGENTS.md`, avisar al desarrollador.
- Skills de terceros: están en `.claude/skills/`; su origen y sus adaptaciones constan en el commit que las incorporó. No modificarlas sin pedido explícito.

## Git y GitHub

### Operaciones de Git (prohibidas)
Está prohibido ejecutar cualquier operación que escriba en el repositorio de código o en su índice, ya sea con comandos de Git (add, commit, branch con argumentos, checkout, switch, merge, rebase, push, pull, fetch, stash, tag, reset, restore, revert, cherry-pick, etc.) o con herramientas MCP (crear ramas; crear, modificar o eliminar archivos; hacer push; mergear, aprobar, cerrar o modificar pull requests). La prohibición se mantiene aunque el desarrollador lo pida. Solo se permiten operaciones de lectura: status, diff, log, rev-parse, rev-list, branch --show-current y remote get-url. TTodas las operaciones de escritura sobre el repositorio Git las realiza una persona (Principio IX, ámbito a). Editar archivos de la copia de trabajo local y ejecutar pruebas, formateadores y compilaciones está permitido. Estos comandos están además bloqueados en `.claude/settings.json`.

### Commits (solo propuesta)
- Los mensajes de commit se proponen únicamente con la skill `/redactar-commit`, a pedido del desarrollador.
- La propuesta se entrega como texto en el chat. El desarrollador la revisa, la modifica si hace falta y ejecuta el commit él mismo.
- El formato y las reglas están en `docs/guia-commits.md`.

### Pull Requests (apertura con confirmación)
- Los pull requests se redactan y abren únicamente con la skill `/redactar-pr`, a pedido del desarrollador, desde la rama de la historia o fase técnica y siempre hacia `main`.
- Antes de abrir el PR, mostrar el título y el cuerpo completos y esperar la confirmación explícita del desarrollador.
- Nunca mergear, aprobar, cerrar ni modificar pull requests, ni solicitar revisores. La revisión y el merge son exclusivos del Agile Enabler (Principio IX, ámbitos b y d).

### Issues de GitHub (permitidas solo vía MCP y con confirmación)
Para los issues del tablero, el agente puede usar el servidor MCP de GitHub para:
- Crear, editar, etiquetar, comentar y cerrar issues.
- Vincular, reordenar o cambiar de padre sub-issues.
- Crear etiquetas.

Reglas obligatorias:
- La fuente de verdad son los archivos de `specs/`. La sincronización es siempre de `specs/` hacia GitHub, nunca al revés: el agente jamás modifica una spec, un plan o un `tasks.md` a partir del contenido de un issue.
- Antes de crear, editar o cerrar issues, el agente presenta la lista completa de cambios y espera la confirmación explícita de una persona.
- Está prohibido eliminar issues, comentarios o etiquetas. Un issue obsoleto se cierra como "no planificado" con un comentario que explica el motivo.
- Los campos Valor, Prioridad y Estimación, los milestones y las asignaciones los gestiona exclusivamente el equipo humano: el agente nunca los completa ni los modifica.

### Sincronización specs → GitHub
- Toda publicación o actualización de issues a partir de `specs/` se hace únicamente con la skill `/sincronizar-github`, que delega en el subagente `sync-github`.
- La conversación principal no llama directamente a las herramientas de escritura de issues ni de etiquetas del servidor MCP de GitHub. La única herramienta de escritura que usa la conversación principal es `create_pull_request`, y solo dentro de la skill `/redactar-pr`.

### Pedidos sin comando
Si el desarrollador pide en lenguaje natural redactar un commit, abrir un PR o sincronizar con GitHub, no hacerlo por cuenta propia: indicarle que ejecute `/redactar-commit`, `/redactar-pr` o `/sincronizar-github`, según corresponda.
