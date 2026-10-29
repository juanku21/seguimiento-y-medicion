# Investigación de la Fase 0: Dashboard del Proyecto

**Feature**: `specs/009-project-dashboard` | **Fecha**: 2026-10-28 | **Plan**: [plan.md](./plan.md)

Cada sección registra una decisión con el formato **Decisión / Justificación / Alternativas
descartadas**. Las tres decisiones de producto de la spec se cerraron el 2026-10-03; las secciones 2,
6 y 7 las tomó el desarrollador el 2026-10-28, al planificar.

---

## 1. Dónde vive la feature: solo frontend, sin endpoint nuevo

**Decisión**: el dashboard es una pantalla de Next.js que compone llamadas a endpoints que **ya
existen** en `main`: el estado del proyecto (002), el listado y el detalle de sprints (004) y tres
endpoints de métricas (008). El backend no cambia: ni módulo nuevo, ni endpoint de agregación, ni
tablas.

**Justificación**:

- RC-04 y FR-002 prohíben que el dashboard calcule. Un endpoint `GET /dashboard` del backend no
  calcularía nada tampoco, pero agregaría un módulo cuyo único trabajo sería reenviar respuestas
  de otros módulos (Principio I).
- La degradación por sección (FR-028, FR-029, SC-010) sale gratis si cada sección tiene su propia
  consulta: una falla queda en la sección que la sufre. Con un endpoint agregador habría que inventar
  un formato de "falla parcial" dentro de un `200`, que es más difícil de probar y de documentar.
- Todas las respuestas necesarias ya exigen sesión y membresía, y ya responden el `404`
  indistinguible (FR-033 a FR-035). El dashboard hereda esas garantías sin repetirlas.

**Alternativas descartadas**:

- **Endpoint agregador en el backend**: una sola llamada, pero con el costo descrito y con un riesgo
  extra: el backend del dashboard tendría que decidir qué hacer cuando falla una de sus partes, que
  es una regla de presentación.
- **Server Components que piden los datos en el servidor de Next.js**: el token vive en
  `sessionStorage` (decisión de la 001), así que el servidor de Next.js no lo tiene. Todas las
  pantallas protegidas del proyecto piden los datos desde el cliente dentro de `SessionGuard`, y el
  dashboard sigue el mismo patrón (`app/projects/[projectId]/page.tsx`).

---

## 2. Librería de gráficos: Recharts 3.10.1

**Decisión**: **Recharts `3.10.1`** con su peer **`react-is` `19.3.0`**, la misma versión que `react`
y `react-dom`. Autorizada por el desarrollador el 2026-10-28, como prevé el Principio IV ("las
dependencias de gráficos (spec 009) se autorizan al planificar esas specs").

**Justificación**:

- `3.10.1` es la última versión estable (`latest` en npm, publicada el 2026-07-25). Las `3.11.0-canary`
  están prohibidas por el Principio IV. Su peer de React admite `^19.0.0`.
- Dibuja **SVG** con componentes React. jsdom puede inspeccionar el SVG, así que las pruebas de Vitest
  verifican la leyenda, los rótulos de los ejes y los nombres de los sprints sin simular un canvas
  (SC-007).
- Tiene barras agrupadas, leyenda, tooltip y rótulos de ejes sin código propio, que son exactamente
  los requisitos de FR-021.

**Alternativas descartadas**:

- **Chart.js 4.6.1 con react-chartjs-2 5.3.1**: dibuja en `<canvas>`, que jsdom no implementa. Las
  pruebas no podrían verificar leyendas ni ejes sin simular el canvas, y la accesibilidad depende
  por completo de código propio.
- **SVG propio sin dependencia**: cero dependencias, pero habría que escribir y probar a mano las
  escalas, las marcas de los ejes, las etiquetas con 20 sprints y el tooltip. Es más código propio
  que el que ahorra.

**Consecuencias técnicas**:

- Los componentes de gráfico son Client Components (`"use client"`). Ya lo es toda la vista, porque
  depende de la sesión del cliente.
- `ResponsiveContainer` mide su contenedor con `ResizeObserver`, que jsdom no tiene. Las pruebas
  instalan un `ResizeObserver` mínimo en el archivo de prueba y el contenedor recibe
  `initialDimension`, para que el SVG se dibuje con un tamaño conocido. No se agrega un archivo de
  setup global: solo lo necesitan los archivos que montan gráficos.

---

## 3. Composición de la pantalla y fuentes de cada sección

**Decisión**: seis secciones de la spec más dos bloques de apoyo, cada uno con su fuente. La vista
pide primero el resumen y monta el resto solo cuando el resumen llegó.

| Sección | Fuente (endpoint existente) | ¿Degradable? |
| --- | --- | --- |
| Resumen del proyecto | `GET /api/v1/projects/{projectId}` (002) | **No** (FR-030) |
| Sprint activo (parte del resumen) | `GET /api/v1/sprints/{sprintId}` (004), con el `activeSprint.id` del resumen | Sí |
| Indicadores clave | `GET .../metrics` (008) y `GET .../metrics/active-sprint` (008) | Sí |
| Gráfico de velocidad | `GET .../metrics/series` (008) | Sí |
| Gráfico de esfuerzo | `GET .../metrics/series` (008) | Sí |
| Gráfico de calidad | `GET .../metrics/series` (008) | Sí |
| Distribución de defectos abiertos | `GET .../metrics` (008), bloque `defects.open` | Sí |
| Advertencias | `GET .../metrics` (008), lista `warnings` | Sí |
| Estado vacío del proyecto | `GET /api/v1/projects/{projectId}/sprints` (004), campo `emptyReason` | Sí |

**Justificación**:

- **El resumen se pide primero y bloquea**: FR-030 dice que sin resumen el dashboard no se muestra.
  Si las demás secciones se pidieran en paralelo, igual habría que esconderlas. Pedirlas después
  cuesta un viaje más, pero evita pedir datos que nunca se van a mostrar y evita mostrar seis
  errores de `404` cuando el proyecto no existe.
- **El sprint activo es una sección degradable dentro del resumen**: la spec lo ubica en el resumen,
  pero lo que da contexto al resto es el proyecto, no el sprint. Si falla el detalle del sprint, el
  resumen se muestra igual y solo ese bloque informa la falla.
- **Los días restantes se toman del detalle del sprint** (`schedule.remainingDays` u
  `schedule.overdueDays`), que calcula la 004 en la zona horaria del sistema (su FR-053 y RN-23). El
  dashboard reutiliza el componente `SprintSchedule` de la 004, que ya presenta "N días restantes" y
  "Vencido hace N días" sin restar fechas (FR-009, FR-010).
- **Una sola consulta de series alimenta los tres gráficos**: FR-026 lo exige. Para que los tres
  degraden por separado igual (US6-5), cada gráfico **valida su propia porción** de la respuesta:
  si el cuerpo llega con la serie de defectos malformada, solo el gráfico de calidad informa la
  falla. Si falla la consulta entera, los tres gráficos lo informan cada uno en su lugar (FR-029).
- **Las advertencias salen solo de `GET .../metrics`**: el dominio de la 008 arma las del proyecto
  juntando la de planificación y las de inconsistencia de cada sprint Cerrado y Activo
  (`CalculateProject`). Las de las series y las del sprint activo son un subconjunto de esas.
  Mostrar las tres listas repetiría avisos, y deduplicarlas sería lógica propia.
- **"El proyecto no tiene sprints" lo dice la 004**: el listado de sprints responde
  `emptyReason: no_sprints`. Deducirlo de que las series vengan vacías sería un error, porque un
  proyecto con solo sprints Planificados tiene series vacías y sí tiene sprints. Si esa consulta
  falla, la vista muestra las secciones normalmente, cada una con su propio estado vacío.

**Alternativas descartadas**:

- **Pedir todo en paralelo con `Promise.all`**: una falla tumbaría todas las secciones.
- **Pedir el sprint activo con `GET .../sprints?status=active`**: el resumen ya trae
  `activeSprint.id`, y el listado no trae `schedule`. Habría que pedir el detalle de todos modos.
- **Pedir las series tres veces, una por gráfico**: degradaría por consulta, pero triplica la carga
  sin ningún beneficio real; la validación por porción cubre la falla parcial.

---

## 4. Consultas por sección y actualización

**Decisión**: cada sección usa `useAuthorizedQuery` (ya existe en `hooks/`) con su propia función de
carga estabilizada con `useCallback`. Para el reintento del resumen, el estado de error del hook
suma el código HTTP (`httpStatus`, porque `status` ya es el discriminante del estado), y la vista vuelve a cargar cambiando un contador que entra en las
dependencias del `useCallback`.

**Justificación**:

- El hook ya resuelve la sesión, el descarte de respuestas viejas y los tres estados. Reutilizarlo
  es lo que hacen las demás vistas (Principio I, DRY).
- FR-030 pide invitar a reintentar cuando falla el resumen, pero un `404` (proyecto inexistente o
  ajeno, FR-035) no se reintenta: es una respuesta definitiva. Distinguir los dos casos requiere el
  código HTTP en el estado de error. Agregar un campo al estado es compatible con todos los usos
  actuales, que solo leen `message`.
- FR-031 y RN-17: la pantalla no se refresca sola. Cada vez que se monta, pide todo de nuevo, así que
  volver a la pantalla o recargarla muestra los datos vigentes (US6-7, US6-8, SC-013). No hay
  intervalos, ni WebSockets, ni caché del cliente.

**Alternativas descartadas**:

- **Un `useEffect` propio en la vista**, como `ProjectOverview`: duplica el hook.
- **`router.refresh()` para reintentar**: refresca los Server Components y no vuelve a ejecutar los
  efectos del cliente, así que no repetiría la consulta.

**Supuesto**: la recarga al volver a la pantalla depende de que `cacheComponents` siga desactivado en
`next.config.ts`. Con esa opción, Next.js preserva la página oculta con `<Activity>` en lugar de
desmontarla (`frontend/node_modules/next/dist/docs/01-app/02-guides/preserving-ui-state.md`), y
FR-031 dejaría de cumplirse al volver. Activarla exige revisar esta decisión.

---

## 5. Presentación de los estados de valor (RC-05, FR-013 a FR-017)

**Decisión**: un único componente, `MetricValue`, presenta cualquier `Metric` o `SprintMetric` de la
008, y es el único lugar del frontend que decide cómo se ve cada estado:

| Entrada de la 008 | Presentación |
| --- | --- |
| `status: available` (incluido `value: 0`) | La cifra formateada. El 0 se muestra como "0", "0 h" o "0,00 %" |
| `status: not_calculable` | "No calculable" y, debajo, la explicación del motivo (sección 6) |
| `status: not_applicable` | "No aplica" |
| `finality: partial` | Marca visible **"Parcial"** junto a la cifra |
| `finality: provisional` | Marca visible **"Provisoria"**, con otro texto y otro estilo que "Parcial" |
| `finality: definitive` o `null` | Sin marca |

**Justificación**:

- FR-017 y RN-06 piden que el 0, el "no calculable" y el "no aplicable" se distingan entre sí, y
  FR-014 prohíbe el guion, el espacio en blanco y el 0. Tres textos distintos lo cumplen, y una
  prueba unitaria por estado lo verifica.
- RN-05 pide distinguir "provisoria" de "parcial". Dos palabras distintas, no dos colores del mismo
  texto, porque el color solo no es accesible.
- Las marcas de bloque, como la de los acumulados (`cumulative.finality`, US2-3), se muestran una
  vez en el encabezado del bloque, porque la 008 dice que califican al conjunto entero (su FR-023).
  La velocidad nunca lleva marca, porque la 008 no la envía (su RN-28, US2-2).

**Formato de los números (FR-013)**: el backend serializa horas y porcentajes con 2 decimales, pero
`JSON.parse` convierte `14.00` en `14`. Mostrar "14,00" es devolverle al número los decimales que ya
tenía, no redondearlo. Se reutilizan los formateadores de la 006 (`utils/effort-format.ts`):
`formatHours`, `formatSignedHours` y `formatPercent`, este último para la desviación, con signo. Se
agregan dos en `utils/metric-format.ts`: `formatDecimal` para la velocidad ("14,00") y
`formatShare` para el porcentaje de historias completadas, sin signo ("27,78 %"). Una prueba
unitaria verifica que ninguno cambia el valor recibido: con todos los valores de la spec
(14,00; −62,96; 27,78; 142,5; 13,10) el texto, leído de vuelta, es exactamente el número que llegó.

**Alternativas descartadas**:

- **Que cada indicador decida su estado por su cuenta**: es la grieta por la que aparece un 0 en
  lugar de un "no calculable".
- **`Intl.NumberFormat` con `maximumFractionDigits: 2`**: redondea si alguna vez llega un tercer
  decimal, y eso es una transformación propia (FR-013). `toFixed(2)` sobre un valor que ya tiene a lo
  sumo dos decimales no redondea nada, y la prueba unitaria lo fija.

---

## 6. Explicación de los "no calculable" (FR-014)

**Decisión**: la explicación es un texto fijo por métrica, porque en la 008 cada métrica tiene **un
solo** denominador posible (su FR-030 y FR-031). El texto lo elige `explainNotCalculable(kind)` en
`utils/metric-explanations.ts`:

| Métrica | Única causa en la 008 | Texto |
| --- | --- | --- |
| Velocidad | Sin sprints Cerrados | "Todavía no hay sprints cerrados." |
| Desviación porcentual | Horas estimadas en 0 | "Las horas estimadas son 0." |
| Porcentaje de historias completadas | Sin historias planificadas | "No hay historias planificadas." |

Cualquier otra métrica que llegue como `not_calculable` muestra "No se puede calcular con los datos
actuales.", para que ningún caso quede sin explicación. Según la 008 eso no ocurre nunca, pero el
frontend no se cae si ocurre (Principio VII).

**Justificación**: la explicación no es un cálculo, es un texto que nombra el denominador que la 008
ya documentó. Si la 008 algún día enviara el motivo en la respuesta, este archivo desaparece.

**Alternativas descartadas**: enmendar la 008 para que envíe un código de motivo. Agregaría un campo
al contrato para un dato que hoy siempre es el mismo por métrica.

---

## 7. Indicadores clave: sprint activo y acumulado del proyecto

**Decisión**: los indicadores se organizan en **dos grupos**. Decidido por el desarrollador el
2026-10-28.

- **Indicadores del sprint activo** (`GET .../metrics/active-sprint`): Story Points planificados y completados,
  porcentaje de historias completadas, horas estimadas y reales, desviación absoluta y porcentual.
  Todas llegan con `finality: partial` y la muestran (US2-2, RN-04). Sin sprint Activo
  (`activeSprint: false`), el grupo dice "No hay ningún sprint en curso." y no muestra ceros
  (FR-011, RN-12).
- **Indicadores del proyecto** (`GET .../metrics`): velocidad del equipo (sin marca, US2-2), defectos abiertos
  (`defects.open.total`; un 0 es un 0, US2-6) y el acumulado: Story Points planificados y
  completados, porcentaje de historias completadas, horas estimadas y reales y su desviación, con la
  marca de `cumulative.finality` en el encabezado del grupo (US2-3).

**Justificación**:

- US2-1 toma del sprint activo los Story Points, el porcentaje y las horas, y US2-5 lo confirma (un
  sprint activo con 0 horas estimadas). Pero US2-3 pide mostrar la marca de parcial del **acumulado**,
  que solo existe si el acumulado se muestra.
- Sin el grupo del proyecto, un proyecto Finalizado o sin sprint Activo dejaría vacíos cinco de los
  seis indicadores, justo cuando el dashboard "más sentido tiene mirarlo" (Casos Límite).
- No se agrega ninguna métrica nueva (RC-09): las cifras del acumulado ya vienen en la misma
  respuesta que la velocidad.

**Alternativas descartadas**: mostrar solo los seis de FR-012 y cubrir US2-3 con una nota suelta.

---

## 8. Gráfico de velocidad: solo los sprints Cerrados

**Decisión**: el gráfico de velocidad usa las series `plannedStoryPoints` y `completedStoryPoints` de
`GET .../metrics/series` y **omite los puntos con `sprintStatus: "active"`**. Es la única selección
que hace el dashboard y se resuelve en una función pura, `closedSprintPoints`, con prueba unitaria.
Decidido por el desarrollador el 2026-10-28.

**Justificación**:

- US3-2 pide que el gráfico de velocidad muestre solo los sprints Cerrados, "porque la velocidad se
  mide sobre sprints terminados". La serie de la 008 incluye al Activo, marcado como parcial (su
  FR-022 y US2-7).
- Omitir puntos según un campo que ya trae la 008 no suma, no reordena y no completa nada: el orden
  y los valores de los puntos que quedan son exactamente los recibidos. FR-026 prohíbe armar series,
  reordenarlas y completar puntos faltantes, y esto no hace ninguna de las tres cosas.
- Los gráficos de esfuerzo y de calidad sí muestran al Activo, marcado como parcial (US4-2).

**Alternativas descartadas**:

- **Mostrar el Activo marcado como parcial**: contradice US3-2 y obliga a enmendar la spec 009.
- **Enmendar la 008** para que entregue una serie de Story Points solo de Cerrados: reabre una
  feature ya integrada en `main` (spec, contrato, backend y pruebas) para algo que se resuelve sin
  calcular.
- **Usar `velocity.series` de `GET .../metrics`**: tiene solo los Story Points completados. Juntarla
  con los planificados de otra serie por `sprintId` sería armar una serie, que FR-026 prohíbe.

---

## 9. Forma de los gráficos y legibilidad con 20 sprints (FR-021, SC-007, SC-009)

**Decisión**: los tres gráficos son **barras agrupadas** de Recharts (`BarChart` con dos `Bar`), con
un componente común, `SprintSeriesChart`, que recibe el título, los rótulos de los ejes, los nombres
de las dos series y los puntos. Cada gráfico va dentro de un `<figure>` con:

- Título visible en `<figcaption>` y como nombre accesible de la figura.
- Rótulo del eje X ("Sprint") y del eje Y ("Story Points", "Horas" o "Defectos").
- Leyenda con el nombre de las dos series.
- Eje X con **todos** los sprints (`interval={0}`), nombres inclinados para que no se superpongan y,
  en el Activo, el sufijo "(parcial)".
- Tooltip con el sprint, los dos valores y su marca de estado.
- Una **tabla de datos** debajo, desplegable con `<details>`, con una fila por sprint, los dos
  valores y sus marcas "Parcial" o "Provisoria".

**Justificación**:

- Las barras agrupadas comparan dos magnitudes por sprint, que es lo que piden los tres gráficos
  (planificado contra completado, estimado contra real, detectado contra resuelto).
- SC-009 pide identificar sin ambigüedad cualquiera de 20 sprints. Las etiquetas inclinadas y el
  tooltip lo resuelven en pantalla; la tabla lo resuelve para lectores de pantalla y es donde las
  pruebas verifican cada valor exacto (SC-004).
- El 0 es un dato (FR-025): el sprint aparece en el eje y en la tabla con "0". La ausencia de dato es
  la ausencia del sprint en la serie, como define la 008. No se usa `minPointSize`, que dibujaría una
  barra donde el valor es 0.
- El orden es el que llega de la 008 (RN-07, RN-20): el componente no ordena.

**Estados de una serie** (FR-023, FR-024): con 0 puntos, el gráfico no se dibuja y en su lugar se
muestra un estado vacío con el título del gráfico y una explicación. Con 1 punto, el gráfico se
dibuja y se agrega "Todavía no hay evolución que comparar." Contar los puntos no es una métrica: es
decidir qué componente mostrar.

**Interpretación de US4-5**: "un proyecto sin sprints con esfuerzo registrado" es el caso de la serie
vacía. Un sprint con 0 horas reales sí aparece, con su 0 (US4-4), porque la 008 lo incluye.

**Alternativas descartadas**:

- **Líneas**: sugieren continuidad entre sprints, y con un solo punto no se dibuja nada.
- **Recortar a los últimos N sprints**: la spec lo descarta en Supuestos ("sin paginación ni recorte
  de sprints en los gráficos").

---

## 10. Distribución de defectos abiertos por severidad (FR-022)

**Decisión**: una lista de las cuatro severidades, en orden Crítica, Alta, Media y Baja, con la
cantidad que trae `defects.open.bySeverity` y el total de `defects.open.total`. Sin gráfico. Con total
0 se agrega "No hay defectos abiertos." como mensaje informativo, no como error (US5-4). Los nombres
de las severidades se reutilizan de `utils/defect-labels.ts` (007).

**Justificación**: la spec pide la distribución "con las cuatro severidades y el total", no un
gráfico; los gráficos obligatorios son tres (FR-018 a FR-020). Mostrar el total que envía la 008 en
lugar de sumar las cuatro cantidades cumple RN-01. Los Descartados no aparecen porque la 008 ya los
excluye (RC-07, US5-6).

**Alternativas descartadas**: un gráfico de torta o de barras. Agrega una figura que necesita su
propio título, ejes y leyenda para cuatro números.

---

## 11. Advertencias (FR-032, RN-21)

**Decisión**: una sección de advertencias, encima de los indicadores, con dos listas separadas por
`kind`:

- `planning`: "Avisos de planificación", con estilo informativo (`role="status"`).
- `data_inconsistency`: "Datos que hay que revisar", con estilo de alerta (`role="alert"`) y un texto
  introductorio que dice que hay información rota que alguien debe revisar.

El texto de cada advertencia es el `message` que envía la 008, sin cambios. Si no hay advertencias,
la sección no se muestra. Las métricas se muestran siempre, haya o no advertencias.

**Justificación**: la 008 ya clasifica las advertencias y redacta los mensajes en español. Separar
por `kind` es la distinción que pide FR-032, y los roles ARIA la hacen verificable en las pruebas.

---

## 12. Ruta, acceso y errores

**Decisión**: ruta `app/projects/[projectId]/dashboard/page.tsx`, un Server Component que solo lee
el parámetro y monta `SessionGuard` con `DashboardView`, igual que `app/projects/[projectId]/page.tsx`.
La vista del proyecto (`ProjectOverview`) suma el enlace "Dashboard" a su navegación.

- Sin sesión (FR-033): `SessionGuard` lleva a `/login`, y un `401` de cualquier consulta descarta la
  sesión en el cliente HTTP (001).
- Proyecto inexistente o ajeno (FR-034, FR-035, US1-8, US1-9): el `404` del resumen muestra el
  mensaje del backend, que es el mismo en los dos casos, y no se monta ninguna otra sección. No se
  ofrece reintentar.
- Falla del resumen distinta de `404` (FR-030, US6-9): "No se pudo mostrar el dashboard." con el
  mensaje recibido y el botón "Reintentar".
- Falla de una sección (FR-028): la sección muestra un mensaje que la nombra, por ejemplo "No se
  pudo cargar el gráfico de defectos.", con `role="alert"`, para identificar cuál falló (US6-5). La
  lista completa está en contracts/ui-contract.md.
- Proyecto Finalizado (FR-036, US1-2): se muestra completo, con la fecha de finalización real. El
  bloque del sprint activo dice "No hay ningún sprint en curso." Un proyecto Finalizado no puede
  tener sprint Activo (002), así que es el mismo caso que US1-3.

**Nota sobre la spec** (resuelta el 2026-10-28, ahora la spec dice lo mismo que US1-2): la tabla de "Entradas y Salidas" decía que el proyecto Finalizado se muestra
"sin sección de sprint activo", y US1-2 dice que la sección "indica que no hay ninguno en curso". El
plan sigue el escenario, que es el que se prueba. No hace falta enmendar la spec: el bloque no
muestra ningún dato de sprint, que es lo que la tabla quiere decir.

**Lectura de la documentación de Next.js**: antes de escribir la página, la implementación lee
`frontend/node_modules/next/dist/docs/01-app/01-getting-started/03-layouts-and-pages.md` y
`05-server-and-client-components.md`, como pide `CLAUDE.md`. La página sigue el patrón vigente de
`params` como `Promise`, igual que las demás páginas del proyecto.

---

## 13. Pruebas

**Decisión**: todas las pruebas son de Vitest en el frontend. Las de escenario siguen el patrón de las
features 006 y 007: montan la vista real dentro de `SessionProvider` y sustituyen solo `fetch` por un
backend de prueba que responde según los contratos de la 002, la 004 y la 008. Se verifica qué
consultas hace la vista y qué muestra con cada respuesta.

- **Escenarios**: un `describe("009/USn - <título>")` por historia y un `it("<nombre>")` por
  escenario, en un archivo por historia (la tabla de trazabilidad está en el plan).
- **Pruebas unitarias obligatorias** (Principio II, reglas de negocio de presentación):
  `MetricValue` con los cinco estados y sus combinaciones; `formatDecimal` y `formatShare`;
  `explainNotCalculable`; `closedSprintPoints`; los estados vacío y de un único punto de
  `SprintSeriesChart`; la validación por porción de la respuesta de series.
- **Falla de una sección**: el backend de prueba responde `500` en el endpoint de esa sección, o un
  cuerpo con la porción malformada en el caso de un solo gráfico (sección 3).
- **Sin pruebas de backend**: esta feature no toca el backend. Sus garantías de sesión, membresía y
  cifras exactas ya las prueban la 002, la 004 y la 008.

**SC-016** pide al menos una prueba por cada regla RN-01 a RN-21 que falle si la regla se rompe; la
tabla del plan las mapea. RN-01 (no calcular) se verifica con datos elegidos para que cualquier
recálculo dé otro número: por ejemplo, una velocidad de 14,00 que no es el promedio de la serie que
llega en la misma respuesta.

**Alternativas descartadas**: pruebas de punta a punta con navegador (Playwright). Sumarían una
dependencia no autorizada y un servicio más, y los escenarios se verifican con Vitest.

---

## 14. Sin cambios de infraestructura

**Decisión**: sin variables de entorno nuevas, sin servicios de Docker nuevos y sin cambios en el
`Dockerfile` del frontend, que instala lo que diga `package-lock.json`. El `.gitignore` no cambia,
porque la feature no genera artefactos nuevos.

**Documentación viva**: el `README.md` suma la pantalla del dashboard y la dependencia de gráficos.
Swagger no cambia, porque no hay endpoints nuevos.
