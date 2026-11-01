---
paths:
  - "frontend/src/features/**/components/**"
---

# Presentación de datos

- Datos comparables entre filas van en tabla, con desplazamiento horizontal propio en móvil. Las tarjetas quedan para elementos con identidad propia, como un proyecto.
- Las cifras usan `tabular-nums` y se alinean a la derecha en las tablas.
- Los gráficos siguen la skill `dataviz` y toman sus colores de los tokens `--chart-*`. Conservan título, ejes rotulados, leyenda y su tabla de datos.
- El frontend no calcula métricas: muestra los valores del backend y sus estados (no calculable, no aplicable, parcial).
