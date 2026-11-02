---
paths:
  - "frontend/src/**/*.tsx"
---

# Accesibilidad

- Objetivo: WCAG 2.1 AA. Las pantallas terminadas se auditan con las skills `accessibility-review` y `web-design-guidelines`.
- Se conservan los roles, nombres accesibles y relaciones ARIA actuales: son parte del contrato que verifican las pruebas.
- Todo control funciona con teclado y tiene foco visible con el token `outline-focus`.
- Las áreas táctiles miden al menos 44 px en móvil.
- Un estado nunca se comunica solo con color: va con texto o ícono.
- Hay un solo `h1` por pantalla y los niveles de título no se saltean.
