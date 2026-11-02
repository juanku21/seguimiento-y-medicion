---
paths:
  - "frontend/src/**"
---

# Sistema de diseño del frontend

Esta regla está subordinada a la constitución; ante conflicto, prevalece la constitución.

- Dirección visual: "instrumento de medición". Claro y oscuro completos, cifras protagonistas, densidad alta y azul y celeste como únicos colores de marca.
- Los colores salen de los tokens semánticos de `frontend/src/styles/globals.css` (`bg-surface`, `text-foreground-muted`, `border-border`, `bg-primary`). En código nuevo o migrado no se usan `zinc-*`, `gray-*`, hexadecimales ni pares `dark:` para colores.
- El tema lo decide la clase `.dark` en `<html>`, nunca `prefers-color-scheme` directamente desde un componente.
- Todo par de texto y fondo cumple 4.5:1, y los elementos gráficos, 3:1. Se mide antes de agregar un token.
- Antes de crear un componente, se busca en `frontend/src/components/`. Lo que usa más de un módulo vive ahí; lo propio de un módulo, en su `features/<módulo>/components/`.
- Los íconos son de `lucide-react`, decorativos con `aria-hidden` y siempre acompañados de un nombre accesible cuando son la única etiqueta.
- Se evitan los rasgos de plantilla: rótulos en mayúsculas, datos unidos por "·", tarjetas idénticas para todo y flechas agregadas al texto de un enlace.
- Las animaciones responden a una acción de la persona y respetan `prefers-reduced-motion`.
