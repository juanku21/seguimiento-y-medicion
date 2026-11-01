---
paths:
  - "frontend/src/app/**"
  - "frontend/src/features/**/components/**"
---

# Maquetación y navegación

- Ninguna pantalla queda sin un enlace de entrada ni sin una forma de volver.
- Las pantallas autenticadas usan la estructura común: barra superior, navegación del proyecto y migas de pan con el nombre real del proyecto.
- El contenido empieza arriba y a la izquierda: `justify-center` no se usa en el contenedor de una pantalla de la aplicación.
- El ancho se adapta al contenido: los formularios son angostos y los listados y dashboards aprovechan el espacio.
- Cada vista tiene una sola acción primaria. Las acciones delicadas usan la variante de peligro y no dominan la pantalla.
- Las pantallas se revisan a 390 px y a 1440 px, en modo claro y oscuro.
- Las URL existentes no cambian. Los grupos de rutas solo organizan.
- No se inventan funciones sin soporte en la API, como búsqueda global o notificaciones.
