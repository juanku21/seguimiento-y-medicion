# Contrato de interfaz: Dashboard del Proyecto

**Feature**: `specs/009-project-dashboard` | **Fecha**: 2026-10-28

Fija la ruta, los nombres accesibles y los textos fijos en los que se apoyan las pruebas de Vitest.
Las pruebas buscan por rol y por nombre accesible (Testing Library), no por clases ni por estructura,
así que el diseño visual puede cambiar sin romperlas mientras se respete esta tabla.

## Ruta

| Ruta | Página | Acceso |
| --- | --- | --- |
| `/projects/{projectId}/dashboard` | `app/projects/[projectId]/dashboard/page.tsx` | Enlace "Dashboard" en la navegación "Secciones del proyecto" de `/projects/{projectId}` |

Título del documento: `Dashboard | Software Metrics & Estimation`. Encabezado de nivel 1:
"Dashboard".

## Secciones, en orden fijo

| Sección | Elemento y nombre accesible | Contenido |
| --- | --- | --- |
| Resumen | `region` "Resumen del proyecto" | Nombre, estado, "Inicio", "Finalización prevista", "Finalización real" (solo Finalizado), lista "Integrantes" |
| Sprint activo | `region` "Sprint activo" | Nombre, Sprint Goal, "Inicio", "Fin", texto de `SprintSchedule` |
| Advertencias | `region` "Advertencias" (solo si hay alguna) | Lista "Avisos de planificación" (`role="status"`) y lista "Datos que hay que revisar" (`role="alert"`) |
| Indicadores | `region` "Indicadores clave" | Grupos "Indicadores del sprint activo" e "Indicadores del proyecto", cada uno con su encabezado `h3` y sin rol de región |
| Velocidad | `figure` "Velocidad por sprint" | Series "Story Points planificados" y "Story Points completados"; ejes "Sprint" y "Story Points" |
| Esfuerzo | `figure` "Esfuerzo por sprint" | Series "Horas estimadas" y "Horas reales"; ejes "Sprint" y "Horas" |
| Calidad | `figure` "Defectos por sprint" | Series "Defectos detectados" y "Defectos resueltos"; ejes "Sprint" y "Defectos" |
| Distribución | `region` "Defectos abiertos por severidad" | Crítica, Alta, Media, Baja y "Total" |

Con `emptyReason: no_sprints`, las secciones Indicadores, Velocidad, Esfuerzo, Calidad y Distribución
se reemplazan por un `region` "Sin datos todavía" con el texto: "Este proyecto todavía no tiene
sprints, así que no hay métricas que mostrar. Planificá el primer sprint desde la sección Sprints del
proyecto." No se ofrece un enlace, porque la navegación hacia los módulos está fuera de alcance.

## Indicadores

Cada indicador es un par término/valor (`<dt>`/`<dd>`) dentro de su grupo.

| Grupo | Términos |
| --- | --- |
| Indicadores del sprint activo | "Story Points planificados", "Story Points completados", "Historias completadas", "Horas estimadas", "Horas reales", "Desviación", "Desviación porcentual" |
| Indicadores del proyecto | "Velocidad del equipo", "Defectos abiertos", "Story Points planificados", "Story Points completados", "Historias completadas", "Horas estimadas", "Horas reales", "Desviación", "Desviación porcentual" |

Formato del valor (sin transformar el número recibido):

| Métrica | Ejemplo con el dato de la spec |
| --- | --- |
| Velocidad | "14,00" |
| Story Points | "18" |
| Historias completadas | "27,78 %" |
| Horas | "108 h", "142,5 h" (`formatHours` de la 006) |
| Desviación | "−68 h" (`formatSignedHours` de la 006) |
| Desviación porcentual | "−62,96 %" (`formatPercent` de la 006) |
| Defectos abiertos | "7" |

## Textos fijos de estado

| Situación | Texto | Rol |
| --- | --- | --- |
| Valor `not_calculable` | "No calculable" y su explicación ([research.md](../research.md), sección 6) | — |
| Valor `not_applicable` | "No aplica" | — |
| Marca `partial` | "Parcial" | — |
| Marca `provisional` | "Provisoria" | — |
| Sin sprint Activo | "No hay ningún sprint en curso." | — |
| Serie vacía, velocidad | "Todavía no hay sprints cerrados para mostrar la velocidad." | — |
| Serie vacía, esfuerzo y calidad | "Todavía no hay sprints iniciados para mostrar <esfuerzo / defectos>." | — |
| Serie con un único punto | "Todavía no hay evolución que comparar." | — |
| Distribución con total 0 | "No hay defectos abiertos." | — |
| Falla de una sección | El mensaje de la sección, de la lista de abajo | `alert` |
| Falla del resumen distinta de `404` | "No se pudo mostrar el dashboard." y botón "Reintentar" | `alert` |
| Cargando el resumen | "Cargando dashboard…" | `status` |
| Cargando el listado de sprints | "Cargando métricas…" | `status` |

### Mensajes de falla por sección

Cada mensaje nombra la sección que falló (US6-5) y se muestra dentro de esa sección:

| Sección | Mensaje |
| --- | --- |
| Sprint activo | "No se pudo cargar el sprint activo." |
| Indicadores del sprint activo | "No se pudieron cargar los indicadores del sprint activo." |
| Indicadores del proyecto | "No se pudieron cargar los indicadores del proyecto." |
| Velocidad | "No se pudo cargar el gráfico de velocidad." |
| Esfuerzo | "No se pudo cargar el gráfico de esfuerzo." |
| Calidad | "No se pudo cargar el gráfico de defectos." |
| Distribución | "No se pudo cargar la distribución de defectos abiertos." |
| Advertencias | "No se pudieron cargar las advertencias." |

## Gráficos

- Cada `figure` tiene su título en `<figcaption>`, que también es su nombre accesible.
- Cada gráfico tiene una tabla "Datos de <título>" dentro de un `<details>` con el resumen "Ver datos":
  una fila por sprint, en el orden recibido, con el nombre del sprint y los dos valores, y "Parcial"
  o "Provisoria" junto a cada valor que lo tenga.
- En el eje X, el nombre del sprint Activo lleva el sufijo "(parcial)".
