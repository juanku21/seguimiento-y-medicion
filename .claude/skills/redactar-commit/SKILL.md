---
name: redactar-commit
description: Propone el mensaje de commit para los cambios en staging, siguiendo la guía de commits del equipo, y lo entrega como comando listo para copiar. Nunca ejecuta el commit. Usar únicamente cuando el desarrollador ejecute /redactar-commit.
disable-model-invocation: true
argument-hint: "[ID de tarea (T013) o número de sub-issue (#58), opcional]"
allowed-tools: Read, Grep, Glob, Bash(git branch --show-current), Bash(git status:*), Bash(git diff:*), Bash(git log:*), mcp__github__list_issues, mcp__github__issue_read
---

# Redactar commit

Proponés el mensaje de commit para los cambios que el desarrollador puso en *staging*. El desarrollador lo revisa, lo modifica si hace falta y lo ejecuta él en su terminal.

## Reglas que no se negocian

- **Nunca ejecutes el commit.** Tampoco `git add`, `push`, `reset`, `stash`, `restore` ni ningún otro comando que modifique el repositorio o el índice. Solo usás `branch --show-current`, `status`, `diff` y `log`.
- No crees ni edites archivos: la propuesta se entrega solo en el chat.
- Nunca propongas `--amend`, `--no-verify`, `rebase` ni `push --force`.
- Un commit corresponde a una sola tarea.
- El mensaje va en español, sin emojis.

## Reglas del mensaje

Resumen de la guía de commits del equipo. Si se modifica la guía, hay que actualizar también esta sección.

**Formato:**

```
tipo(ámbito): descripción

Cuerpo opcional.

Spec: NNN/Txxx
TDD: red|green|refactor
Closes #N
```

**Tipos:** `feat` (funcionalidad nueva), `fix` (corrección), `test` (solo pruebas), `refactor` (sin cambio de comportamiento), `docs` (solo documentación), `style` (solo formato), `chore` (configuración, Docker, dependencias, estructura).

**Ámbitos:** `auth` (001), `proyectos` (002), `backlog` (003), `sprints` (004), `poker` (005), `esfuerzo` (006), `defectos` (007), `metricas` (008), `dashboard` (009), `reportes` (010), `infra`, `deps`, `repo`. El ámbito es el mismo para backend y frontend.

**Primera línea:**
- 72 caracteres o menos en total.
- La descripción va en minúsculas, sin punto final.
- Verbo en presente, tercera persona: completa la frase "este commit..." (`agrega`, `corrige`, `valida`).
- Describe el cambio, no la tarea: nunca solo el ID de la tarea.

**Cuerpo:**
- Explica qué cambia y por qué, con líneas de 72 caracteres o menos.
- Es obligatorio si el cambio no es evidente, hay una decisión de diseño o se aparta del plan.
- En un commit RED, indica por qué fallan las pruebas.

**Pies:**
- `Spec: NNN/Txxx` siempre.
- `TDD:` en reglas de negocio, cálculos y validaciones: `red` para pruebas nuevas que fallan, `green` para el código que las hace pasar y `refactor` para mejoras sin cambio de comportamiento.
- `Closes #N` si el commit termina la tarea, o `Refs #N` si es intermedio. `#N` es el número de la sub-issue de la tarea, nunca el de la historia.
- `BREAKING CHANGE: ...` solo si rompe compatibilidad, además de `!` después del ámbito.

## Paso 1. Verificar la rama y el staging

1. Ejecutá `git branch --show-current`. Si es `main`, avisá que los commits se hacen en la rama de la historia y detenete.
2. Ejecutá `git diff --staged --stat`. Si no hay nada en *staging*:
   - Mostrá `git status --short`.
   - Sugerí qué archivos agregar para **una** tarea, como comando `git add ...` para que lo ejecute el desarrollador.
   - Detenete.
3. Si el *staging* incluye archivos `.env` reales, credenciales, binarios o archivos temporales, advertilo antes de cualquier propuesta y sugerí sacarlos con `git restore --staged <archivo>`.

## Paso 2. Identificar la tarea

1. Si se pasó un ID de tarea o un número de sub-issue en `$ARGUMENTS`, usalo.
2. Si no, deducí la historia del nombre de la rama (`hu/NNN-usX-...` o `fase/NNN-...`), leé las tareas en `specs/NNN-.../tasks.md` y comparalas con los archivos en *staging*.
3. Para obtener el número de la sub-issue:
   - Con `list_issues`, filtrando por la etiqueta `spec:NNN`, buscá la historia por su marca `<!-- speckit:NNN/USx -->`.
   - Con `issue_read`, leé sus sub-issues y buscá la marca `<!-- speckit:NNN/Txxx -->`.
   - Si el servidor MCP no está disponible, pedile el número al desarrollador.
4. Si los cambios corresponden a más de una tarea, no propongas un mensaje. Indicá qué archivos van con cada tarea y sugerí separarlos con `git restore --staged`.
5. Si no podés determinar la tarea con seguridad, preguntá antes de seguir.

## Paso 3. Reunir contexto

1. Leé el diff completo: `git diff --staged`.
2. Revisá los commits previos de la rama: `git log origin/main..HEAD --no-merges --format="%h%x1f%s%x1f%b%x1e"`. Así sabés:
   - Si la tarea ya tiene commits con `Refs`.
   - En qué etapa de TDD está. Por ejemplo, si ya existe el RED de las pruebas relacionadas, este commit es GREEN.
3. Leé la descripción de la tarea en `tasks.md`.

## Paso 4. Decidir el mensaje

1. **Tipo:**
   - Solo archivos de prueba → `test`.
   - Funcionalidad nueva → `feat`.
   - Solo documentación → `docs`.
   - Configuración o infraestructura → `chore`.
   - Mejora sin cambio de comportamiento → `refactor`.
2. **`Closes` o `Refs`:**
   - Usá `Closes` si el diff, junto con los commits previos de la tarea, cubre lo que describe la tarea y los archivos que indica `tasks.md`.
   - Si no, usá `Refs` y explicá qué parece faltar.
3. **TDD:** incluí el pie solo si la tarea involucra reglas de negocio, cálculos o validaciones. Si es `red` y no sabés por qué fallan las pruebas, preguntalo o dejalo como supuesto.
4. Verificá:
   - La primera línea cumple las reglas, incluido el máximo de 72 caracteres.
   - El mensaje **no contiene comillas simples ni dobles**. Si hacen falta, reformulá la frase.

## Paso 5. Presentar la propuesta

Mostrá, en este orden:

1. **Supuestos a verificar:** la tarea identificada, si se usa `Closes` o `Refs` y por qué, y la etapa de TDD. Escribilo en pocas líneas.
2. **Comando** en un bloque de código, con un `-m` por párrafo, cada uno entre comillas simples:

   ```
   git commit -m 'feat(auth): implementa el servicio de registro de cuentas' -m 'Normaliza el correo antes de verificar unicidad para evitar
   duplicados que solo difieren en mayúsculas.' -m 'Spec: 001/T013
   TDD: green
   Closes #58'
   ```

   Este formato funciona igual en PowerShell y en Git Bash. Si no hay cuerpo, se omite ese `-m`.
3. Una línea que diga que el desarrollador debe revisarlo, puede editarlo antes de ejecutarlo, y después hacer `git push`.

Si el desarrollador pide cambios, aplicalos y volvé a mostrar el comando completo. Nunca lo ejecutes, aunque te lo pida: recordale que el commit lo hace él.
