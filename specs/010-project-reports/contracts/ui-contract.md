# Contrato de interfaz: Reportes de Proyecto y de Sprint

**Feature**: `specs/010-project-reports` | **Fecha**: 2026-10-30

Fija la ruta, los nombres accesibles, los textos fijos y la forma del PDF en los que se apoyan las
pruebas de Vitest. Las pruebas buscan por rol y por nombre accesible (Testing Library), no por clases.
El PDF se verifica sobre el documento de pdfmake que arma `buildPdfDocument`. Mientras se respete esta
tabla, el diseño visual puede cambiar sin romper las pruebas.

## Ruta

| Ruta | Página | Acceso |
| --- | --- | --- |
| `/projects/{projectId}/reports` | `app/projects/[projectId]/reports/page.tsx` | Enlace "Reportes" en la navegación "Secciones del proyecto" de `/projects/{projectId}` |

Título del documento: `Reportes | Software Metrics & Estimation`. Encabezado de nivel 1: "Reportes".

## Controles

| Control | Elemento y nombre accesible | Comportamiento |
| --- | --- | --- |
| Alcance | `combobox` "Alcance del reporte" | Opción "Proyecto completo" primero; después el sprint Activo y los Cerrados, como "<nombre> (Activo)" o "<nombre> (Cerrado)", en el orden de `listSprints`. Los Planificados no se ofrecen |
| Generar | `button` "Generar reporte" | Deshabilitado mientras se genera; el texto pasa a "Generando…" |
| Exportar | `button` "Exportar a PDF" | Solo con un reporte en pantalla; deshabilitado mientras se exporta ("Exportando…") |

## Mensajes

| Situación | Elemento | Texto |
| --- | --- | --- |
| `404` del proyecto | `alert` | "El proyecto no existe." (el mismo de la 002) |
| `404` del sprint | `alert` | "El sprint no existe." |
| `409` sprint Planificado | `alert` | El `error` del backend: "El sprint todavía no arrancó: no tiene nada que reportar." |
| `500` o respuesta irreconocible | `alert` | "No se pudo generar el reporte. Intentá de nuevo." |
| Falla del PDF | `alert` | "No se pudo generar el PDF. Intentá de nuevo." El reporte sigue en pantalla |

## Documento en pantalla

`article` "Reporte" (`ReportDocument`), con un `h2` igual a `ReportView.title`:

- Sprint: "Reporte del sprint <nombre>".
- Proyecto: "Reporte del proyecto <nombre del proyecto>".

Con `partial: true`, inmediatamente debajo del título va un `status` con el texto **"Reporte parcial:
incluye un sprint en curso."**.

Las seis secciones, siempre en este orden, cada una como `region` con su `h3`:

| Sección | Nombre accesible | Contenido |
| --- | --- | --- |
| Encabezado | "Encabezado" | Lista de términos: "Proyecto", "Alcance" ("Proyecto completo" o "Sprint <nombre>"), "Período" ("DD/MM/AAAA al DD/MM/AAAA" o "Sin sprints reportables"), "Generado el" ("DD/MM/AAAA HH:mm:ss", zona del navegador), "Generado por" |
| Historias | "Historias" | Una tabla por grupo con las columnas "Historia", "Prioridad", "Story Points" y "Estado" |
| Estimaciones | "Estimaciones" | Una tabla por grupo con las columnas "Historia", "Story Points" y "Planning Poker" |
| Esfuerzo | "Esfuerzo" | Una tabla por grupo con "Historia" y "Horas" y pie "Total del sprint"; la tabla "Por integrante" con "Integrante" y "Horas"; y "Total registrado" |
| Métricas | "Métricas" | Tabla con "Métrica" y "Valor" |
| Defectos | "Defectos" | Tabla con "Descripción", "Severidad", "Estado", "Historia", "Detectado en" y "Resuelto en"; después "Detectados: N" y "Resueltos: N" |

**Encabezados de grupo** (historias, estimaciones y esfuerzo): `h4` "<nombre del sprint> (DD/MM/AAAA
al DD/MM/AAAA)", con " — en curso" en el Activo. El último grupo de historias y de estimaciones del
reporte de proyecto es "Historias no planificadas".

**Celdas con texto fijo**:

| Caso | Texto |
| --- | --- |
| Story Points nulos | "Sin estimar" |
| Estado de una historia | Etiquetas de la 003 ("Pendiente", "En progreso", "Completada") y "No completada" para `not_completed` |
| `poker: agreed` | "N rondas, valor acordado V" ("1 ronda" en singular) |
| `poker: no_agreement` | "Sin valor acordado por estimación colaborativa" |
| `poker: none` | "No hubo sesión de estimación" |
| "Resuelto en" sin sprint | "—" (no es una cifra: es la ausencia de sprint de resolución) |

**Métricas**: una fila por métrica, con los nombres del dashboard de la 009. El valor se arma con
`metricText`:

| Estado | Texto |
| --- | --- |
| `available` | El número formateado ("14,00", "27,78 %", "18") |
| `not_calculable` | "No calculable: <explicación de explainNotCalculable>". Nunca "0" (FR-019) |
| `not_applicable` | "No aplica" |
| `finality` | Se agrega " · Parcial", " · Provisoria" o " · Definitiva" (FR-020) |

En el reporte de proyecto se suman las filas "Esfuerzo fuera de sprints", "Esfuerzo en sprints
planificados" e "Historias sin estimar en el backlog", y el acumulado lleva su marca de finalidad.

**Estados de sección y de grupo**:

| Estado | Texto debajo del `h3` (o del `h4` del grupo) |
| --- | --- |
| `empty` | "Sin datos." (`role="note"`) |
| `unavailable` | "No se pudo obtener esta sección." (`role="alert"`) |

Una sección `empty` de defectos muestra igual "Detectados: 0" y "Resueltos: 0" (US4-2).

## PDF

| Aspecto | Regla |
| --- | --- |
| Contenido | Los mismos textos de `ReportView`, en el mismo orden que la pantalla (RN-15) |
| Título | `ReportView.title` como primer bloque |
| Marca parcial | Debajo del título y en el pie de cada página: "Reporte parcial: incluye un sprint en curso." |
| Numeración | Pie en todas las páginas: "Página N de M" |
| Tablas | `headerRows: 1` y `dontBreakRows: true` en todas |
| Fuente | Roboto de `pdfmake/build/vfs_fonts` |
| Nombre | `reportFileName`: `Reporte_<proyecto>_<proyecto \| sprint-<nombre>>_<AAAA-MM-DD>_<HH-mm-ss>.pdf` |

Ejemplos de nombre:

| Proyecto | Alcance | `generatedAt` (zona del navegador) | Archivo |
| --- | --- | --- | --- |
| "Gestión de Compras" | Proyecto | 30/10/2026 14:32:05 | `Reporte_Gestion-de-Compras_proyecto_2026-10-30_14-32-05.pdf` |
| "Ñandú: App/Web *beta*" | Sprint "Sprint 3" | 30/10/2026 09:05:00 | `Reporte_Nandu-App-Web-beta_sprint-Sprint-3_2026-10-30_09-05-00.pdf` |
