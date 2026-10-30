# Investigación (Fase 0): Reportes de Proyecto y de Sprint

**Feature**: `specs/010-project-reports` | **Fecha**: 2026-10-30 | **Plan**: [plan.md](./plan.md)

Cada sección registra una decisión con el formato Decisión / Justificación / Alternativas
consideradas. Las tres decisiones de producto ya se cerraron en la sesión de clarificación del
2026-10-04. La librería de PDF la autorizó el desarrollador el 2026-10-30 (sección 14). Lo que sigue
resuelve las incógnitas técnicas que la spec deja para la etapa de diseño (RC-10).

---

## 1. Un endpoint agregador en un módulo `reports` del backend

### Decisión

Un módulo vertical nuevo, `backend/internal/reports/`, con sus cuatro capas. Arma el reporte completo
en una sola respuesta y escribe el registro de generación. El frontend pide el reporte con una única
operación y no compone consultas de otras features.

### Justificación

- **FR-034 exige escribir.** Cada generación deja un registro. Sin backend propio, el frontend
  tendría que hacer una escritura aparte, y un reporte podría mostrarse sin quedar registrado
  (SC-018).
- **Las fuentes no están expuestas como las necesita el reporte.** La estimación pide la última
  sesión Finalizada *anterior al cierre* del sprint (sección 5). El esfuerzo pide subtotales por
  historia y por integrante del sprint. Componerlo desde el frontend implicaría una consulta de
  sesiones por historia: 80 historias son 80 pedidos (SC-010).
- **Las métricas se reutilizan dentro del proceso.** La 008 dejó previsto `MetricsService` como
  "punto de entrada que reutilizará la 010 dentro del proceso" (research de la 008, sección 2). Así no
  hay una segunda implementación de ninguna fórmula (RC-08).

### Alternativas consideradas

- **Componer en el frontend, como el dashboard (009)**: no resuelve la escritura, multiplica las
  consultas y obliga a recortar las sesiones por fecha en el navegador.
- **Extender el módulo `metrics` con el reporte**: mezcla una feature de solo lectura sin tablas con
  otra que escribe y que lee seis fuentes. Rompe la responsabilidad única (Principio I, SOLID).

---

## 2. Dos rutas `POST` sin cuerpo bajo el subgrupo del proyecto

### Decisión

- `POST /api/v1/projects/{projectId}/reports/sprints/{sprintId}`: reporte de un sprint.
- `POST /api/v1/projects/{projectId}/reports/project`: reporte del proyecto completo.

Las dos cuelgan del subgrupo `/:projectId` de la 002, que ya exige sesión y membresía vigente
(FR-037, FR-038). No leen cuerpo. Responden `200` con el reporte. Errores:

| Caso | Código | Cuerpo |
| --- | --- | --- |
| Sin sesión | `401` | El de la 001 |
| Proyecto inexistente o ajeno | `404` | El de la 002 (`RequireMembership`) |
| Sprint inexistente, de otro proyecto o con identificador mal formado | `404` | `"El sprint no existe."`, igual que la 004 y la 008 |
| Sprint Planificado | `409` | `"El sprint todavía no arrancó: no tiene nada que reportar."` |
| Falla al leer el encabezado o al guardar el registro de generación | `500` | Mensaje genérico, sin detalles |

### Justificación

Es `POST` porque escribe el registro de generación y no es idempotente: dos generaciones son dos
registros. Dos rutas sin cuerpo repiten la forma de `/metrics/sprints/:sprintId` y de `/metrics`, y
evitan validar un discriminador de alcance (KISS). El sprint de otro proyecto responde lo mismo que
uno inexistente porque se busca acotado al `projectId` de la ruta (FR-039). Se usa `409` porque es el
código que la 004 usa para una operación inválida según el estado del sprint.

Se responde `200` y no `201`: el registro de generación no es un recurso que se pueda leer (la spec
deja fuera de alcance consultarlo), así que no hay `Location` que devolver.

### Alternativas consideradas

- **`GET`**: un `GET` que escribe viola la semántica de HTTP. Además, un prefetch o una recarga
  dejarían registros que nadie pidió.
- **Una sola ruta con `{"scope": ..., "sprintId": ...}` en el cuerpo**: suma validación de entrada y
  combinaciones inválidas (alcance de proyecto con `sprintId`) sin ganar nada.

---

## 3. Métricas: `MetricsService` en proceso y la forma JSON de la 008

### Decisión

- `reports/domain` declara el puerto `MetricsSource`, con `SprintMetrics(ctx, projectID, sprintID)`
  y `ProjectMetrics(ctx, projectID)`. Lo cumple `*metricsservice.MetricsService` tal cual, sin
  adaptador.
- La sección de métricas lleva **exactamente** el cuerpo que devuelve la 008: `SprintMetrics` en el
  reporte de sprint y `ProjectMetricsResponse` en el de proyecto. Para eso `metrics/delivery` exporta
  sus constructores `NewSprintMetrics` y `NewProjectMetrics`, hoy privados (**TOCA** la 008, sin
  cambiarle el comportamiento).
- El reporte de proyecto lleva las métricas **del proyecto** (velocidad, acumulado, esfuerzo fuera de
  sprints y en Planificados, historias sin estimar y defectos). No lleva una tabla por sprint: las
  series por sprint son del dashboard (US3-1: "métricas … del proyecto completo").
- Las advertencias de la 008 viajan dentro de esos cuerpos, pero el reporte no las muestra: la spec
  no las pide. Las marcas de historia "sin estimar" ya aparecen en las secciones de historias y
  estimaciones.

### Justificación

Reusar la forma JSON garantiza que los cinco estados de valor y la finalidad lleguen sin aplanarse
(FR-017, FR-020) y que el frontend use los mismos tipos que el dashboard (sección 17). El puerto en el
dominio de `reports` permite sustituir la fuente en la prueba de US4-6 (sección 8).

### Alternativas consideradas

- **DTO propio de métricas en `reports/delivery`**: sería un segundo mapeo de los mismos valores, con
  riesgo de divergir (DRY).
- **Llamar por HTTP al endpoint de la 008**: agrega red, autenticación y un modo de falla nuevo dentro
  del mismo proceso.

---

## 4. Historias: instantánea, backlog vigente, agrupamiento y orden

### Decisión

| Grupo | Fuente | Estado informado |
| --- | --- | --- |
| Sprint Cerrado | `sprint_stories` del sprint: `title_at_close`, `priority_at_close`, `story_points_at_close`, `outcome` | `completed` o `not_completed` (resultado congelado) |
| Sprint Activo | Compromisos abiertos (`outcome IS NULL`) unidos a `user_stories` vigente | Estado vigente: `pending`, `in_progress` o `completed` |
| Historias no planificadas (solo en el de proyecto) | `user_stories` del proyecto sin ninguna fila en `sprint_stories` cuyo sprint esté `active` o `closed`. Una historia comprometida solo en Planificados entra acá, porque FR-007 deja fuera esos sprints (spec, sesión del 2026-10-30) | Estado vigente |

- **Agrupamiento**: el reporte de sprint tiene un solo grupo, el del sprint. El de proyecto tiene un
  grupo por sprint Cerrado o Activo, en el orden `start_date ASC, id ASC` que usa la 008 (FR-011), y
  al final el grupo de no planificadas, que se muestra siempre (FR-036, US3-7).
- **Una historia en dos sprints** aparece en los dos grupos, cada uno con sus datos y su resultado
  (FR-012). Sale directo de la consulta: hay una fila de `sprint_stories` por compromiso.
- **Orden dentro del grupo**: prioridad (`high`, `medium`, `low`), después título y después
  `story_id`. Es un orden total y determinista, así que dos reportes del mismo sprint Cerrado listan
  igual (SC-017).
- Los Story Points nulos se informan como `null` y se muestran "Sin estimar".

### Justificación

Es la misma regla de lectura que `loadSprintStories` de la 008: la instantánea en el Cerrado y los
compromisos abiertos en el resto. Por eso las secciones de historias y de métricas cuadran (FR-021,
RN-22). La consulta se escribe en `reports/repository`, como cada módulo lee las tablas ajenas que
necesita (por ejemplo, `backlog/repository/sprint_assignments.go`).

### Alternativas consideradas

- **Ordenar por el orden del backlog**: el backlog no tiene una posición congelada en la
  instantánea. Un reporte de sprint Cerrado cambiaría al reordenar el backlog.
- **Una historia replanificada una sola vez, con su último estado**: contradice FR-012.

---

## 5. Estimaciones: la última sesión Finalizada, con corte al cierre

### Decisión

La sección repite los mismos grupos y el mismo orden que la de historias. Cada fila lleva los Story
Points de esa historia en ese grupo (instantánea o vigente, igual que en la sección 4) y el estado de
la estimación colaborativa:

| `poker` | Cuándo | Texto |
| --- | --- | --- |
| `agreed` | Hay al menos una sesión `finished` con `closed_at` dentro del corte | "Planning Poker: N rondas, valor acordado V" |
| `no_agreement` | Hay sesiones abiertas dentro del corte, pero ninguna `finished` dentro de él (canceladas, abiertas o finalizadas después) | "Sin valor acordado por estimación colaborativa" |
| `none` | No hay ninguna sesión abierta dentro del corte | "No hubo sesión de estimación" |

- **Sesión elegida** (RN-09, FR-014): la `finished` con mayor `closed_at` y, ante empate, mayor `id`.
  `rounds` es la cantidad de filas de `estimation_rounds` de esa sesión y el valor acordado es
  `agreed_story_points`.
- **Corte**: en un grupo de sprint Cerrado, el corte es el `closed_at` del sprint. Una sesión cuenta
  si su `created_at` es anterior o igual al corte, y aporta valor acordado solo si además terminó
  `finished` con `closed_at` anterior o igual al corte. En el Activo y en las no planificadas no hay
  corte.

### Justificación

Sin corte, una nueva sesión de Planning Poker después del cierre cambiaría el reporte de un sprint
Cerrado, que es justo lo que SC-017 prueba "reestimando sus historias". El corte hace que la
estimación informada sea la que estaba vigente al cerrar, coherente con los Story Points de la
instantánea. Distinguir `none` de `no_agreement` cubre los dos textos de la spec: US1-4 ("no hubo
sesión de estimación") y el caso límite de la sesión Cancelada ("no hay valor acordado por estimación
colaborativa").

### Alternativas consideradas

- **Una sola fila por historia en el reporte de proyecto**: obliga a elegir qué Story Points mostrar
  para una historia replanificada y rompe el paralelo con la sección de historias.
- **Contar las rondas reveladas en lugar de todas**: la spec dice "cantidad de rondas" y en una sesión
  Finalizada toda ronda iniciada forma parte de su historial.

---

## 6. Esfuerzo: los registros anclados al sprint, como en la 008

### Decisión

- **Registros incluidos**: los de `effort_entries` con `sprint_id` igual a un sprint del alcance (el
  sprint del reporte, o los Cerrados y el Activo del proyecto). Es el mismo criterio con el que la 008
  suma `ActualHundredths`, así que el total de la sección coincide con "Horas reales" de la sección de
  métricas (SC-004).
- **Forma**: un grupo por sprint, con una fila por historia (horas y subtotal del grupo); una tabla
  por integrante para todo el alcance; y el total general. Todo sale de **una** consulta con
  `GROUP BY`, así que subtotales y total no se contradicen entre sí.
- **Título de la historia**: el de la instantánea del sprint si la historia está en ella; si no, el
  vigente. Esto cubre un registro de una historia que salió del sprint Activo. **Nombre del
  integrante**: `users.full_name` (RC-06). Los usuarios no se eliminan, así que el nombre se conserva
  aunque la persona deje el proyecto.
- **Orden**: grupos como en la sección 4; historias por horas descendentes y después título; personas
  por nombre y después `user_id`.
- El esfuerzo sin sprint y el anclado a sprints Planificados **no** van en esta sección: son cifras de
  la 008 (`effortOutsideSprintsHours`, `effortInPlannedSprintsHours`) y aparecen en la sección de
  métricas del reporte de proyecto.

### Justificación

La 008 no define horas por historia ni por integrante, así que esas cifras no compiten con ninguna
fórmula suya. Son sumas de los registros de la 006, que es la fuente que nombra RC-06. FR-008 se
respeta porque toda cifra que la 008 sí define (las horas reales) sale de la 008 y aparece en la
sección de métricas. La sección de esfuerzo muestra el detalle, con el mismo criterio de anclaje. Una
prueba de integración verifica que el total de la sección es igual a `actualHours` de la 008, con los
mismos datos.

En un sprint Activo, alguien podría cargar horas entre la lectura del esfuerzo y la de las métricas, y
los dos totales diferirían en ese registro. Se acepta: el reporte ya está marcado como parcial
(FR-005) y la ventana es de milisegundos. Un sprint Cerrado con el plazo de gracia vencido no cambia.

### Alternativas consideradas

- **Tomar el total de la sección de la 008 y los subtotales de los registros**: dos lecturas para una
  misma tabla, que en el Activo pueden no sumar.
- **Agregar el desglose al módulo `metrics`**: amplía la 008 con cifras que su spec no define.

---

## 7. Defectos: listado de la 007, totales de la 008

### Decisión

- **Alcance del listado**, siempre sin `discarded` (RC-09):
  - Reporte de sprint: defectos del proyecto con `detected_sprint_id` igual al sprint o
    `resolved_sprint_id` igual al sprint.
  - Reporte de proyecto: todos los defectos del proyecto.
- **Columnas** (FR-018): severidad, estado, historia relacionada, sprint de detección y sprint de
  resolución. Se suma la **descripción**, porque sin ella dos filas con la misma severidad y la misma
  historia no se distinguen. Es un dato que cualquier integrante ya ve (RC-11).
- **Título de la historia relacionada**: el de la instantánea del sprint del reporte si la historia
  está en ella; si no, el vigente (sección 6).
- **Totales**: salen de la 008. En el de sprint, `defectsDetected` y `defectsResolved`; en el de
  proyecto, `defects.detected.total` y `defects.resolved.total`.
- **Orden**: `start_date` del sprint de detección, después `created_at` y después `id`.

### Justificación

Los totales son cifras de la 008 y no se recuentan (FR-008). El listado usa el mismo filtro que la
consulta `loadSprintDefects` de la 008, así que las filas detectadas en el sprint suman el total de
detectados y las resueltas en el sprint, el de resueltos (SC-016). Como los totales dependen de las
métricas, si la 008 falla la sección de defectos también queda como no obtenible (sección 8).

### Alternativas consideradas

- **Contar las filas del listado como totales**: es reimplementar el conteo de la 008, que RC-09
  señala como la referencia.
- **Omitir la descripción, como lista FR-018**: el listado pierde utilidad sin ganar nada.

---

## 8. Estados de sección y fallas parciales

### Decisión

- Cada sección lleva `state`: `complete`, `empty` o `unavailable` (FR-023, FR-024). El encabezado no
  tiene estado: si falla, el reporte entero responde `500`, porque sin proyecto ni autor no hay
  reporte que mostrar.
- **Cuándo es `empty`**:

  | Sección | `empty` cuando | Lo que se conserva |
  | --- | --- | --- |
  | Historias | Ningún grupo tiene historias | Los grupos, cada uno con `empty`; el de no planificadas siempre está |
  | Estimaciones | Igual que historias | Igual que historias |
  | Esfuerzo | No hay registros en el alcance | Total `0.00` |
  | Métricas | Solo en el reporte de proyecto sin ningún sprint Cerrado ni Activo (US3-10) | Nada |
  | Defectos | El listado está vacío | Los totales, en `0` (US4-2) |

  En el reporte de sprint, la sección de métricas nunca está `empty`: un sprint sin historias igual
  tiene sus nueve métricas (US1-6).
- **Cuándo es `unavailable`**: la lectura de la que depende la sección devolvió error. El servicio
  hace **lecturas independientes** por sección (no una transacción común): una falla en una no aborta
  las demás (FR-025). Cada falla se registra con `slog` y el cuerpo no expone detalles. Dependencias:
  historias y estimaciones comparten la lectura de historias; defectos depende de su listado **y** de
  las métricas (totales); el esfuerzo solo de sus registros.
- La prueba de US4-6 compone el servicio con un `MetricsSource` o un repositorio que devuelve error en
  una sola lectura. Es el único lugar donde se sustituye una capa (Principio III).

### Justificación

Lecturas separadas son la única forma de que una falla de SQL no se lleve el resto: en PostgreSQL, un
error dentro de una transacción la aborta. Para un sprint Cerrado los datos están congelados, así que
leer por separado no cambia el resultado. Para el Activo, el reporte ya es parcial (sección 6).

### Alternativas consideradas

- **Una transacción `REPEATABLE READ` para todo**: es consistente, pero una falla aborta todas las
  secciones (FR-025).
- **Omitir la sección que falla**: lo prohíben FR-023 y FR-024.

---

## 9. Parcialidad y período

### Decisión

- `partial` es `true` en un reporte de sprint Activo (RN-03) y en un reporte de proyecto que incluye
  al Activo (RN-06). Se decide con el estado de los sprints leídos, no con las métricas, para que la
  marca exista aunque la sección de métricas falle.
- **Período**: en el de sprint, `startDate` y `endDate` del sprint. En el de proyecto, el menor
  `startDate` y el mayor `endDate` de los sprints incluidos; sin sprints, `null`, que se muestra
  "Sin sprints reportables".
- Un proyecto Finalizado no puede tener un sprint Activo, así que su reporte nunca es parcial (US3-4).

### Justificación

Son reglas de negocio con prueba unitaria obligatoria (Principio II). Ponerlas en funciones puras del
dominio (`IsPartial`, `ReportPeriod`) las aísla del armado.

### Alternativas consideradas

- **Tomar la marca de `cumulative.includesActiveSprint` de la 008**: deja el reporte sin marca si la
  sección de métricas no se pudo obtener, y SC-006 exige el 100 %.

---

## 10. Registro de generación

### Decisión

- Tabla `report_generations` (data-model.md, sección 1) con proyecto, sprint (nulo en el de
  proyecto), alcance, autor e instante de generación. Sin copia del documento (FR-034, RN-24).
- **Momento**: el servicio toma `now` una sola vez. Arma el reporte, inserta el registro con ese
  instante y recién entonces responde. El mismo instante es el `generatedAt` del encabezado.
- **Si el insert falla, responde `500`** y no entrega el reporte: SC-018 exige que el 100 % de las
  generaciones quede registrada. Un reporte rechazado por `404` o `409` no deja registro, porque no se
  generó.
- **Autor**: el `user_id` de la sesión (`CurrentUserID` de la 001), con clave foránea a `users`. El
  nombre se lee de `users.full_name` para el encabezado (FR-041). Un integrante que deja el proyecto
  conserva su cuenta, así que el registro sigue mostrando su nombre (FR-034).
- **Sin endpoint de lectura**: consultar el historial queda fuera de alcance.

### Justificación

Guardar después de armar permite registrar solo lo que se entregó. El insert es la última operación,
así que si falla no queda nada a medias.

### Alternativas consideradas

- **Copiar el nombre del autor en el registro**: los usuarios no se eliminan y la 001 no permite
  cambiar el nombre, así que la copia no protege de nada (YAGNI).
- **Registrar antes de armar**: deja registros de reportes que no llegaron a entregarse.

---

## 11. Proyecto Finalizado

### Decisión

El módulo no consulta el estado del proyecto para autorizar. La condición de solo lectura de la 002
la aplica cada repositorio de escritura de las otras features, y `report_generations` no es un dato
del proyecto (FR-035). El estado se lee solo para el encabezado.

### Justificación

FR-040: generar un reporte no escribe en el proyecto. Una prueba de integración genera el reporte de
un proyecto Finalizado y verifica `200` y que el registro existe.

### Alternativas consideradas

- Ninguna razonable: bloquear contradice FR-040.

---

## 12. Frontend: una pantalla de reportes con el reporte en memoria

### Decisión

- Ruta `/projects/[projectId]/reports`, un Server Component que monta `SessionGuard` y `ReportsView`.
  `ProjectOverview` suma el enlace "Reportes" (**TOCA**, como hizo la 009 con "Dashboard").
- `ReportsView` muestra un selector de alcance con "Proyecto completo" y los sprints Activo y Cerrados
  (de `listSprints` de la 004; los Planificados no se ofrecen), y el botón "Generar reporte". Son tres
  pasos desde el proyecto (SC-001).
- El reporte generado vive en el estado del componente. Exportar no lo modifica (FR-033). Generar otro
  lo reemplaza. Recargar la página lo pierde, porque el documento no se guarda (FR-034).
- La generación usa el cliente HTTP existente (`postAuthorizedJson`). El `404` muestra el mismo
  mensaje de proyecto o sprint inexistente que el resto del producto; el `409` muestra el mensaje del
  backend.

### Justificación

Un selector y un botón cubren los dos alcances sin una pantalla por alcance. Ofrecer solo los sprints
reportables evita un error previsible. El `409` igual se maneja, por si el estado cambió entre el
listado y la generación.

### Alternativas consideradas

- **Una ruta por reporte (`/reports/sprints/[sprintId]`)**: invita a recargar, y cada recarga sería
  una generación nueva con su registro.
- **Guardar el reporte en `sessionStorage` para sobrevivir a la recarga**: es almacenar el documento
  en otra forma, sin que nadie lo pidiera.

---

## 13. Fidelidad pantalla–PDF: un único modelo de presentación

### Decisión

`buildReportView(report, timeZone)`, una función pura en `features/reports/view-model.ts`, convierte
la respuesta en un `ReportView` de **textos ya formateados**: el título de cada sección, su estado y
su mensaje, y cada tabla con sus encabezados y sus filas como cadenas. La pantalla
(`ReportDocument`) y el PDF (`buildPdfDocument`) consumen ese mismo `ReportView`; ninguno formatea por
su cuenta.

### Justificación

RN-15 y SC-003 se cumplen por construcción: los dos destinos leen las mismas cadenas en el mismo
orden. La prueba es directa: se extraen los textos del documento de pdfmake y los de la pantalla
renderizada, y se comparan sección por sección.

### Alternativas consideradas

- **Capturar la pantalla como imagen (html2canvas)**: el texto deja de ser texto, corta tablas al
  paginar y suma otra dependencia.
- **`window.print()` con CSS de impresión**: el nombre del archivo, la numeración y la descarga quedan
  en manos del navegador, y no se puede probar.

---

## 14. PDF con pdfmake 0.3.13 en el navegador

### Decisión

- **Dependencia**: `pdfmake@0.3.13`, fijada sin rango, **autorizada por el desarrollador el
  2026-10-30** (Principio IV). Trae Roboto embebida en `build/vfs_fonts`, que cubre los acentos, las
  eñes y los signos del español (FR-031).
- **Sin `@types/pdfmake`**: el paquete no trae tipos. En lugar de sumar `@types/pdfmake@0.3.3`, que no
  está autorizado, se declaran en `frontend/src/types/pdfmake.d.ts` solo las piezas que se usan:
  `createPdf`, `download`, `addVirtualFileSystem` y el subconjunto del documento (texto, tablas, pie,
  márgenes y estilos). No se usa `any` (Principio VIII). Si el desarrollador prefiere los tipos
  oficiales, se reemplaza esa declaración por la dependencia, con su autorización.
- **Carga diferida**: `export-pdf.ts` importa `pdfmake/build/pdfmake` y `pdfmake/build/vfs_fonts` con
  `import()` dentro del manejador del botón. No pesa en la carga de la página y no se evalúa en el
  servidor de Next.js.
- **Documento** (`buildPdfDocument(view)`), pensado para FR-028, FR-032 y RN-16:
  - Pie en todas las páginas: "Página N de M" (`footer(currentPage, pageCount)`).
  - Cada tabla con `headerRows: 1`, que repite el encabezado en cada página, y `dontBreakRows: true`,
    que no parte una fila entre dos páginas.
  - Anchos de columna con `*` y `auto`: el texto largo se ajusta en la celda, no se corta.
  - La marca "Reporte parcial" como bloque destacado debajo del título, en la primera página, y en el
    pie de cada página, para que una hoja suelta tampoco parezca definitiva (US2-2).
  - Las secciones `empty` y `unavailable` llevan su título y su mensaje (US4-5).
- La presentación visual concreta (tamaños y colores) se fija en las tareas, sobre la paleta de
  Tailwind que ya usa la interfaz.

### Justificación

El documento de pdfmake es un objeto de datos, así que se prueba en Vitest sin navegador. Repetir
encabezados y numerar páginas son opciones nativas, sin cálculo propio de paginación. Generar en el
navegador desde el `ReportView` es lo que garantiza la fidelidad (sección 13) y que una falla no
toque la pantalla (sección 16).

### Alternativas consideradas

Presentadas al desarrollador el 2026-10-30:

- **@react-pdf/renderer 4.9.0**: hay que armar a mano la repetición del encabezado de tabla; es más
  pesada y cuesta más probarla en jsdom.
- **jspdf 4.2.1 + jspdf-autotable 5.0.9**: las fuentes estándar no garantizan acentos y eñes sin
  embeber una TTF, y son dos dependencias.
- **Generar en el backend con Go**: vuelve a armar el reporte, y en un sprint Activo el PDF podría no
  coincidir con la pantalla (FR-027).

---

## 15. Nombre del archivo

### Decisión

`reportFileName(input)`, una función pura en `frontend/src/utils/report-file-name.ts`:

```text
Reporte_<proyecto>_<alcance>_<AAAA-MM-DD>_<HH-mm-ss>.pdf
```

- `<alcance>` es `proyecto` o `sprint-<nombre del sprint>`. Así un reporte de proyecto se distingue
  de uno de sprint del mismo día (US2-3).
- Fecha y hora salen de `generatedAt`, en la zona horaria del navegador, la misma con que el
  encabezado muestra la generación. Con los segundos, dos reportes del mismo día se distinguen sin
  abrirlos (US2-6, FR-029).
- **Saneado** (FR-030), aplicado a cada parte variable:
  1. Normalización NFD y eliminación de las marcas diacríticas: "Gestión Ñandú" pasa a
     "Gestion Nandu".
  2. Todo carácter fuera de `A–Z`, `a–z`, `0–9` y `-` pasa a `-`, incluidos `\ / : * ? " < > |`,
     los espacios y los controles.
  3. Se colapsan los guiones repetidos y se recortan los de los extremos.
  4. Cada parte se limita a 60 caracteres. Si queda vacía, se usa `proyecto` o `sprint`.

### Justificación

El resultado es válido en Windows, macOS y Linux, y sigue siendo legible porque conserva las letras
base. Es una regla de validación, con prueba unitaria obligatoria.

### Alternativas consideradas

- **Conservar los acentos en el nombre**: los sistemas actuales los aceptan, pero adjuntos de correo
  y algunos repositorios de documentación los rompen.
- **Un sufijo aleatorio para distinguir**: no es legible ni dice nada de cuándo se generó.

---

## 16. Falla de la exportación

### Decisión

`exportReportPdf(view, fileName)` envuelve en `try/catch` la carga diferida, `createPdf` y
`download`. Si algo falla, `ReportsView` muestra con `role="alert"` el mensaje "No se pudo generar el
PDF. Intentá de nuevo." y deja el reporte en pantalla sin tocar. El botón queda habilitado para
reintentar con el mismo `ReportView`, sin volver a pedir el reporte (FR-033, SC-013). Exportar no hace
ninguna consulta al backend.

### Justificación

Como el PDF se arma desde el estado local, reintentar no regenera nada ni deja un registro nuevo.

### Alternativas consideradas

- **Registrar también cada exportación**: FR-034 registra la generación, no la exportación.

---

## 17. Reutilización de la 009 y prerrequisito

### Decisión

El frontend reutiliza de la 009 `types/metrics.ts`, `formatDecimal` y `formatShare`
(`utils/metric-format.ts`) y `explainNotCalculable` (`utils/metric-explanations.ts`). El componente
`MetricValue` no se reutiliza, porque el reporte necesita **textos** para alimentar el PDF (sección
13). En su lugar, `metricText(metric, kind)`, en el modelo de presentación, devuelve el texto
equivalente: el valor formateado, "No calculable: <explicación>" o "No aplica", seguido de la marca
"Parcial", "Provisoria" o "Definitiva" cuando la 008 la informa (FR-019, FR-020).

**Prerrequisito**: la implementación del frontend de la 010 empieza con la 009 integrada en `main`.
Hoy la 009 tiene plan y tareas, pero no código.

### Justificación

Así una métrica se formatea igual en el dashboard y en el reporte (DRY), y los "no calculable"
llevan la misma explicación.

### Alternativas consideradas

- **Duplicar los formateadores en `features/reports`**: dos versiones del mismo número.
- **Renderizar `MetricValue` y leer su texto**: ata el PDF al DOM.

---

## 18. Pruebas

### Decisión

- **Backend, integración** (`reports/delivery`, `TestUSn_<Escenario>`): contra la base de test con el
  router real, como la 008. Siembran proyecto, sprints, historias, sesiones de Planning Poker, esfuerzo
  y defectos con SQL directo o con los servicios de cada feature. Verifican el cuerpo, el código y la
  fila de `report_generations`. Para US4-6, el servicio se compone con una fuente que falla.
- **Backend, unitarias obligatorias** (`reports/domain`): `EnsureReportable`, `IsPartial`,
  `ReportPeriod`, `SelectEstimation` (última Finalizada y corte), `SortStories`, `SectionState`, y la
  regla de título de la instantánea o el vigente.
- **Frontend** (Vitest con un backend de prueba según el contrato): `describe("010/USn - …")` para las
  cuatro historias, montando `ReportsView`. `export-pdf` se reemplaza con `vi.mock` para capturar el
  documento y el nombre, o para forzar la falla.
- **Frontend, unitarias obligatorias**: `reportFileName`, `buildReportView` (estados, "no calculable",
  marcas), `buildPdfDocument` (`headerRows`, `dontBreakRows`, pie, marca de parcial, secciones vacías)
  y el reconocedor `isReport`.
- **Comprobación manual** (quickstart): abrir un PDF real de 80 historias y revisar la paginación y los
  acentos. jsdom no puede verificar el render de pdfkit.

### Justificación

Es la combinación que exige el Principio II: integración por defecto y unitarias en reglas de negocio,
validaciones y cálculos de presentación.

### Alternativas consideradas

- **Generar el PDF real en Vitest y analizar sus páginas**: suma un lector de PDF como dependencia de
  prueba para verificar lo que ya garantizan `headerRows`, `dontBreakRows` y el pie.
