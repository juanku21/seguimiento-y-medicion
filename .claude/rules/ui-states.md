---
paths:
  - "frontend/src/features/**/components/**"
  - "frontend/src/components/**"
---

# Estados de la interfaz

- Toda vista con datos resuelve carga, vacío, error y, si corresponde, solo lectura.
- La carga conserva `role="status"` y evita saltos de contenido; se prefiere un esqueleto con la forma del resultado.
- Un vacío explica qué pasa y ofrece la acción siguiente cuando la persona puede ejecutarla. Un vacío no se presenta como error.
- Un error dice qué pasó y cómo seguir. Se mantienen los mensajes del backend y `role="alert"`.
- Los textos los revisa la skill `ux-copy`: voseo, verbos concretos y la misma palabra para la misma acción en todo el flujo.
