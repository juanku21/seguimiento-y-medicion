# CLAUDE.md

Guía operativa para Claude Code en este repositorio. Ante cualquier discrepancia, prevalece `.specify/memory/constitution.md`.

## Git y GitHub

### Operaciones de Git (prohibidas)
Está prohibido ejecutar cualquier operación que escriba en el repositorio de código, ya sea con comandos de Git (commit, branch, checkout -b, merge, rebase, push, stash, tag, reset, etc.) o con herramientas MCP (crear ramas, crear, modificar o eliminar archivos, hacer push, abrir o mergear pull requests). Solo se permiten operaciones de lectura (status, diff, log, branch sin argumentos). Todas las operaciones de escritura sobre el código las realiza una persona (Principio IX).

### Issues de GitHub (permitidas solo vía MCP y con confirmación)
El agente puede usar el servidor MCP de GitHub exclusivamente para gestionar issues del tablero:
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
- La conversación principal no llama directamente a las herramientas de escritura del servidor MCP de GitHub.
