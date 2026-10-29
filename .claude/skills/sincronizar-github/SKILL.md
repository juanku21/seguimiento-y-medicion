---
name: sincronizar-github
description: Publica o actualiza en GitHub las historias de usuario o las tareas de una spec de Spec Kit, con confirmación humana previa. Uso: /sincronizar-github historias NNN o /sincronizar-github tareas NNN.
argument-hint: "historias|tareas NNN"
disable-model-invocation: true
---

# Sincronizar una spec con GitHub

Argumentos recibidos: $ARGUMENTS

Sigue estos pasos en orden. No llames nunca por tu cuenta a las herramientas de escritura del servidor MCP `github`: toda escritura la hace el subagente `sync-github`.

**Regla obligatoria para el tipo `tareas`:** toda tarea de `tasks.md` termina vinculada como sub-issue de su padre (la historia de usuario de su fase o la fase técnica). Es un requisito de la sincronización, no un paso opcional: no se puede excluir, ni posponer, ni dar por cumplido sin la verificación del subagente.

## 1. Validar los argumentos

- El primer argumento debe ser `historias` o `tareas`; el segundo, el número de feature de tres dígitos (por ejemplo `001`).
- Busca la carpeta `specs/NNN-*/`. Debe existir exactamente una.
- Para `historias`, debe existir `spec.md` en esa carpeta. Para `tareas`, debe existir `tasks.md`.
- Si algo falla, explica el problema y detente.

## 2. Pedir el plan

Delega en el subagente `sync-github` con este mensaje, completando los datos:

    Modo: PLAN
    Tipo: [historias|tareas]
    Feature: specs/NNN-nombre/

Espera a que el subagente termine y te entregue el plan. Guarda su identificador de agente.

Para el tipo `tareas`, antes de mostrar el plan comprueba que:

- tenga la sección "Vinculación de tareas (obligatoria)" con **todas** las tareas de `tasks.md` (compara la cantidad de IDs `Txxx` del archivo con las filas de la tabla);
- cada tarea que no figura como "vinculada" tenga su acción `VINCULAR` en "Acciones";
- cada padre que falta tenga su `CREAR` antes que sus tareas.

Si algo de eso falla, no muestres el plan. Reanuda el mismo subagente indicando qué falta y pidiéndole que regenere el plan completo. Si el segundo plan sigue incompleto, informa el problema al usuario y detente.

## 3. Pedir la confirmación

- Muestra el plan al usuario completo, sin resumirlo.
- Si el plan no tiene acciones distintas de `SIN CAMBIOS`, informa que GitHub ya está sincronizado y termina. Para el tipo `tareas`, eso solo vale si la tabla de vinculación muestra todas las tareas como "vinculadas".
- Avisa que las acciones `VINCULAR` son obligatorias y no se pueden excluir.
- Si hay "Observaciones", destácalas antes de preguntar.
- Pregunta explícitamente si aprueba el plan, con tres opciones: aprobar, cancelar o pedir ajustes.
- Solo una aprobación explícita del usuario en esta conversación cuenta como aprobación.

## 4. Actuar según la respuesta

- **Cancelar:** termina sin escribir nada en GitHub.
- **Ajustes:** si el usuario quiere cambiar el contenido de una historia o tarea, recuérdale que la fuente de verdad es `specs/`: debe corregir la spec o `tasks.md` y volver a ejecutar esta skill. Si el ajuste es excluir acciones puntuales del plan, toma nota de cuáles y pasa al punto siguiente con esa lista. **No aceptes** la exclusión de una acción `VINCULAR`, ni la de un `CREAR` de historia o fase técnica si alguna de sus tareas se crea o se vincula en el mismo plan: explica que dejaría tareas sin padre y vuelve a preguntar. Si el usuario quiere dejar fuera una tarea, debe excluir su `CREAR` y su `VINCULAR` juntos. Esa tarea queda pendiente y el informe final la marca como no vinculada.
- **Aprobar:** reanuda el mismo subagente `sync-github` (con SendMessage a su identificador) con este mensaje:

      PLAN APROBADO. Modo: APLICAR. Ejecuta todas las acciones del plan en su orden.
      Acciones excluidas por el usuario: [ninguna | lista de números].

  Si no es posible reanudarlo, invoca un subagente `sync-github` nuevo con `Modo: APLICAR`, el mismo tipo y feature, el plan aprobado completo y las exclusiones, e indícale que regenere los cuerpos con las mismas reglas y que, si su resultado no coincide con el plan, se detenga y lo reporte.

## 5. Informar el resultado

Muestra el informe del subagente. Si hubo un error, explícalo en lenguaje llano y sugiere volver a ejecutar la skill después de corregirlo: las marcas de trazabilidad evitan duplicados.

Para el tipo `tareas`, revisa la sección "Verificación de vínculos" antes de dar la sincronización por terminada:

- Si falta la sección, o si alguna tarea figura como no vinculada o vinculada a otro padre, informa que la sincronización **quedó incompleta**. Nombra cada tarea afectada e indica volver a ejecutar `/sincronizar-github tareas NNN`, que propondrá los `VINCULAR` pendientes.
- Solo informa éxito si todas las tareas de `tasks.md` figuran vinculadas a su padre correcto.
