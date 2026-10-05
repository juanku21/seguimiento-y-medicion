---
name: revisar-seguridad
description: Revisa la seguridad del código de la rama actual (historia de usuario o fase técnica) con el subagente security-reviewer, presenta los hallazgos y, con la aprobación del desarrollador, agrega a tasks.md las tareas de prueba y corrección. Usar únicamente cuando el desarrollador ejecute /revisar-seguridad.
disable-model-invocation: true
allowed-tools: Read, Grep, Glob, Bash(git branch --show-current), Bash(git diff:*), Bash(git log:*)
---

# Revisar seguridad

Orquestás la revisión de seguridad de la rama actual. El análisis lo hace el subagente `security-reviewer`. Vos presentás los resultados y, solo con la aprobación del desarrollador, agregás las tareas a `tasks.md`.

## Reglas

- La única escritura permitida es agregar tareas aprobadas a `tasks.md`. No modificás código, pruebas ni otros archivos.
- Nunca reescribís, renumerás, reordenás ni borrás tareas existentes.
- No creás issues: eso lo hace `/sincronizar-github`.
- Sin operaciones de Git de escritura.

## Paso 1. Determinar el alcance

Ejecutá `git branch --show-current`:

- `hu/NNN-usX-...`: feature `specs/NNN-*/`, alcance = historia USx.
- `fase/NNN-<slug>`: feature `specs/NNN-*/`, alcance = la fase técnica cuyo título corresponde al slug.
- Cualquier otra rama: detenete y preguntá al desarrollador qué historia o fase revisar.

Ubicá en `tasks.md` la sección del alcance. Si no la encontrás, detenete y preguntá.

Si `git diff origin/main...HEAD --stat` no muestra cambios de código, informalo y terminá.

## Paso 2. Invocar al revisor

Invocá al subagente `security-reviewer`. Indicale la carpeta de la feature y el alcance (historia USx o fase).

## Paso 3. Presentar los resultados

Mostrale al desarrollador, sin modificar nada:

1. La tabla de hallazgos, de mayor a menor severidad.
2. Las tareas propuestas, agrupadas por hallazgo.
3. Los hallazgos fuera de alcance y las observaciones, con la recomendación de registrarlos como defecto o tratarlos en la revisión del PR.

Preguntá qué tareas aprueba. Puede aprobar todas, algunas o ninguna, y pedir cambios en su redacción.

## Paso 4. Agregar las tareas aprobadas

Solo después de la confirmación explícita:

1. Calculá el ID máximo de todo `tasks.md` y asigná IDs consecutivos desde el siguiente (`Txxx`, con tres dígitos).
2. Agregá las tareas al final de la sección del alcance, después de su última tarea y antes de cualquier texto de cierre de la sección (por ejemplo, un "Checkpoint") o del encabezado siguiente. No crees secciones ni encabezados nuevos.
3. Conservá el orden de cada par: primero la tarea de prueba y después la de corrección.
4. En una historia, cada tarea lleva `[USx]`; en una fase técnica, no.

## Paso 5. Próximos pasos

Indicale al desarrollador:

1. Que ejecute `/sincronizar-github tareas NNN` para crear las tareas como sub-issues.
2. Que las implemente con `/speckit-implement Txxx`, una por vez y en orden: primero la de prueba (RED) y después la de corrección (GREEN).
3. Que, después de corregir, vuelva a ejecutar `/revisar-seguridad` para confirmar que no quedan hallazgos.

Si no hubo hallazgos, indicá que la rama puede seguir hacia `speckit-converge` (opcional) y `/redactar-pr`.
