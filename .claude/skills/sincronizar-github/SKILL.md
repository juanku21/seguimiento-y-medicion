---
name: sincronizar-github
description: Publica o actualiza en GitHub las historias de usuario o las tareas de una spec de Spec Kit, con confirmación humana previa. Uso: /sincronizar-github historias NNN o /sincronizar-github tareas NNN.
argument-hint: "historias|tareas NNN"
disable-model-invocation: true
---

# Sincronizar una spec con GitHub

Argumentos recibidos: $ARGUMENTS

Sigue estos pasos en orden. No llames nunca por tu cuenta a las herramientas de escritura del servidor MCP `github`: toda escritura la hace el subagente `sync-github`.

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

## 3. Pedir la confirmación

- Muestra el plan al usuario completo, sin resumirlo.
- Si el plan no tiene acciones distintas de `SIN CAMBIOS`, informa que GitHub ya está sincronizado y termina.
- Si hay "Observaciones", destácalas antes de preguntar.
- Pregunta explícitamente si aprueba el plan, con tres opciones: aprobar, cancelar o pedir ajustes.
- Solo una aprobación explícita del usuario en esta conversación cuenta como aprobación.

## 4. Actuar según la respuesta

- **Cancelar:** termina sin escribir nada en GitHub.
- **Ajustes:** si el usuario quiere cambiar el contenido de una historia o tarea, recuérdale que la fuente de verdad es `specs/`: debe corregir la spec o `tasks.md` y volver a ejecutar esta skill. Si el ajuste es excluir acciones puntuales del plan, toma nota de cuáles y pasa al punto siguiente con esa lista.
- **Aprobar:** reanuda el mismo subagente `sync-github` (con SendMessage a su identificador) con este mensaje:

      PLAN APROBADO. Modo: APLICAR. Ejecuta todas las acciones del plan en su orden.
      Acciones excluidas por el usuario: [ninguna | lista de números].

  Si no es posible reanudarlo, invoca un subagente `sync-github` nuevo con `Modo: APLICAR`, el mismo tipo y feature, el plan aprobado completo y las exclusiones, e indícale que regenere los cuerpos con las mismas reglas y que, si su resultado no coincide con el plan, se detenga y lo reporte.

## 5. Informar el resultado

Muestra el informe del subagente. Si hubo un error, explícalo en lenguaje llano y sugiere volver a ejecutar la skill después de corregirlo: las marcas de trazabilidad evitan duplicados.
