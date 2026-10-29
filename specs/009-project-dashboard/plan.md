# Plan de Implementación: Dashboard del Proyecto

**Rama**: `009-project-dashboard` | **Fecha**: 2026-10-28 | **Spec**: [spec.md](./spec.md)

**Entrada**: Especificación de feature en `specs/009-project-dashboard/spec.md`

---

## Resumen

El dashboard es la pantalla donde las cifras de la 008 se vuelven una lectura: el resumen del
proyecto y de su sprint en curso, los indicadores clave, tres gráficos de evolución por sprint y la
distribución de los defectos abiertos por severidad. Su regla central es que **no calcula nada**:
presenta lo que entregan la 002, la 004 y la 008, y respeta los cinco estados de valor de la 008
sin aplanarlos.

El enfoque técnico tiene seis decisiones que lo definen:

1. **Solo frontend, sin endpoint nuevo.** La pantalla compone seis `GET` que ya existen en `main`.
   El backend no cambia (research, sección 1).
2. **El resumen va primero y bloquea; todo lo demás degrada por sección.** Cada sección tiene su
   propia consulta con `useAuthorizedQuery`, y los tres gráficos, que salen de una única consulta de
   series (FR-026), validan cada uno su porción de la respuesta para fallar por separado (research,
   sección 3).
3. **Un único componente presenta los estados de valor.** `MetricValue` es el único lugar que decide
   cómo se ven `available`, `not_calculable`, `not_applicable`, `partial` y `provisional`
   (research, sección 5).
4. **Indicadores en dos grupos**: el sprint activo (todo parcial) y el proyecto (velocidad, defectos
   abiertos y acumulado con su marca), decidido por el desarrollador el 2026-10-28 (research,
   sección 7).
5. **El gráfico de velocidad omite el punto del sprint Activo**, según el campo `sprintStatus` que
   ya trae la 008. Es la única selección del dashboard, decidida por el desarrollador el 2026-10-28
   (research, sección 8).
6. **Recharts 3.10.1 para los gráficos**, autorizada por el desarrollador el 2026-10-28 según el
   Principio IV. Cada gráfico lleva además una tabla de datos accesible (research, secciones 2 y 9).

Esta feature **no tiene endpoints, tablas, variables de entorno ni servicios nuevos**. Agrega una
pantalla, un módulo `features/dashboard/` y dos dependencias del frontend.

Las decisiones y las alternativas descartadas están en [research.md](./research.md).

---

## Contexto Técnico

**Lenguaje y versión**: TypeScript 5.9.3 sobre Next.js 16.4.2 y React 19.3.0 (frontend). El backend
(Go 1.27.3) no cambia

**Dependencias principales**: sin cambios en las existentes. **Nuevas**: `recharts` `3.10.1` y su
peer `react-is` `19.3.0`, fijadas sin rango, autorizadas el 2026-10-28 ([research.md](./research.md),
sección 2)

**Almacenamiento**: N/A. El dashboard no persiste nada ni usa almacenamiento del navegador

**Pruebas**: Vitest 5.0.3 con jsdom 30.1.2 y Testing Library (`@testing-library/react` 16.3.3), con
`fetch` sustituido por un backend de prueba según los contratos, como en las features 006 y 007

**Plataforma objetivo**: navegador, servido por Next.js en contenedor; desarrollo en Windows

**Tipo de proyecto**: aplicación web, con backend de API y frontend separados en directorios de primer
nivel; esta feature solo agrega frontend

**Objetivos de rendimiento**: la spec no fija tiempos. SC-001 y SC-002 piden que un integrante lea el
estado del sprint en menos de 10 segundos y la tendencia en menos de 15: se cumplen con el orden fijo
de las secciones y con el sprint activo en la parte de arriba. La carga hace dos rondas de consultas:
el resumen y, después, cinco en paralelo

**Restricciones**: cero cifras distintas de la fuente (SC-004); cero "no calculable" como 0, guion,
vacío u oculto (SC-005); 100 % de las cifras del activo marcadas parciales y ninguna de un cerrado
(SC-006); título, ejes y leyenda en el 100 % de los gráficos (SC-007); orden cronológico de la 008
(SC-008); 20 sprints legibles (SC-009); degradación independiente (SC-010); falla del resumen que
bloquea (SC-011); estado vacío sin ejes en cero (SC-012); datos vigentes al recargar (SC-013); sesión
y membresía (SC-014, SC-015)

**Escala y alcance**: 1 pantalla, 6 secciones más el bloque de advertencias, 6 consultas de lectura, 0
endpoints, 0 entidades persistidas, 6 historias de usuario con **45** escenarios de aceptación y 21
reglas de negocio

### Incógnitas resueltas en la Fase 0

No queda ningún `NEEDS CLARIFICATION`. Las tres decisiones de producto se cerraron en la sesión de
clarificación del 2026-10-03. Al planificar aparecieron tres puntos que la spec no resolvía, y el
desarrollador los decidió el 2026-10-28: la librería de gráficos, el sprint Activo en el gráfico de
velocidad y la composición de los indicadores. Las incógnitas **técnicas** se resolvieron en
[research.md](./research.md): dónde vive la feature, fuentes por sección, consultas y actualización,
estados de valor y formato, explicaciones, indicadores, velocidad, forma de los gráficos,
distribución, advertencias, ruta y errores, y pruebas.

### Puntos señalados al desarrollador (no bloquean el diseño)

1. **Dependencia nueva autorizada.** `recharts@3.10.1` y `react-is@19.3.0` quedan autorizadas por la
   decisión del 2026-10-28. El Principio IV prevé que la de gráficos se autoriza al planificar esta
   spec. Conviene sumarlas a la tabla de dependencias autorizadas de la constitución con una
   enmienda PATCH, para que la autorización no quede solo en este plan.
2. **Interpretaciones incorporadas a la spec.** Las cuatro interpretaciones que surgieron al
   planificar se incorporaron a la spec el 2026-10-28 (Clarifications, sesión del 2026-10-28;
   FR-012, FR-026, RN-20, Entradas y Salidas y Casos Límite): el gráfico de velocidad omite el
   Activo (research, sección 8), los indicadores van en dos grupos (sección 7), el proyecto
   Finalizado muestra la sección del sprint activo con "No hay ningún sprint en curso." (sección
   12) y US4-5 es el caso de la serie vacía (sección 9).
3. **Toca código de otras features, sin cambiarles el comportamiento**: `useAuthorizedQuery` suma el
   código HTTP al estado de error, y `ProjectOverview` suma el enlace "Dashboard". Las pruebas
   actuales de ambas siguen pasando.
4. **Las advertencias se toman solo de las métricas del proyecto**, porque ya contienen las de las
   series y las del sprint activo (research, sección 3).
5. **Rama de trabajo**: el plan se generó en `repo/009-plan`, y `.specify/feature.json` (ignorado por
   Git) ahora apunta a `specs/009-project-dashboard`. La spec indica la rama `009-project-dashboard`
   para la implementación.

---

## Verificación de la Constitución

*PUERTA: debe pasar antes de la investigación de la Fase 0. Reevaluada después del diseño de la
Fase 1.*

### Primera evaluación (antes de la Fase 0)

| Principio | Puerta | Resultado |
| --- | --- | --- |
| I. Simplicidad deliberada | ¿El alcance se limita a lo pedido? | **PASA**. Seis secciones fijas, sin personalización, filtros, comparación entre proyectos, exportación, burndown, alertas ni actualización en tiempo real, todos en "Fuera de Alcance". Sin backend nuevo. |
| II. TDD | ¿Hay forma de escribir la prueba antes del código? | **PASA**. Los 45 escenarios son pruebas de Vitest con backend de prueba; las reglas de presentación tienen pruebas unitarias. |
| III. Arquitectura por capas | ¿La estructura cabe en el layout obligatorio? | **PASA**. Página en `app/`, módulo en `features/dashboard/`, tipos en `types/`, formateadores en `utils/`. |
| IV. Stack fijo y versiones estables | ¿Hace falta algo fuera de la tabla? | **PASA CON AUTORIZACIÓN**. La librería de gráficos no está en la tabla, pero el Principio IV prevé autorizarla al planificar la 009. El desarrollador autorizó `recharts@3.10.1` y `react-is@19.3.0` el 2026-10-28, ambas estables. |
| V. Contenerización | ¿Dev y test son disjuntos? | **PASA**. Sin servicios nuevos; las pruebas no usan la base. |
| VI. Seguridad y secretos | ¿Hay secretos fuera de `.env`? | **PASA**. Sin configuración nueva; el token sigue en `sessionStorage` y lo envía el cliente HTTP existente. |
| VII. Robustez y validación | ¿Toda entrada se valida? | **PASA**. Cada respuesta pasa por un reconocedor de tipo antes de mostrarse; una respuesta inesperada es una falla de sección, no un error de ejecución. |
| VIII. Estándares y documentación viva | ¿Idiomas, `any`, README? | **PASA**. Identificadores en inglés, textos y comentarios en español, sin `any`. README actualizado en el mismo cambio; Swagger no cambia. |
| IX. Gobierno del repositorio | ¿El plan pide operaciones prohibidas? | **PASA**. El plan no ejecuta ninguna escritura de Git. |

**Resultado: todas las puertas pasan. La única dependencia nueva cuenta con la autorización que
exige el Principio IV.**

### Reevaluación (después del diseño de la Fase 1)

| Principio | Qué se revisó en el diseño | Resultado |
| --- | --- | --- |
| I | Sin endpoint agregador, sin caché, sin estado global ni librería de consultas. La distribución por severidad es una lista, no un cuarto gráfico. Se reutilizan `useAuthorizedQuery`, `SprintSchedule`, `MemberList`, los formateadores de la 006 y los nombres de severidad de la 007 (DRY). Los tipos de `types/metrics.ts` declaran solo los campos que se muestran. | **PASA** |
| II | Los 45 escenarios se mapean a pruebas nombradas (tabla más abajo), y cada regla RN-01 a RN-21 tiene al menos una prueba que falla si se rompe. | **PASA** |
| III | `features/dashboard` importa de `features/projects` (`getProjectDetail`, `MemberList`) y de `features/sprints` (`getSprint`, `listSprints`, `SprintSchedule`), como ya hace `features/effort` con otras features. Ninguna feature importa `features/dashboard`. Los tipos de la 008 van en `types/`, los formateadores en `utils/` y la página en `app/`. | **PASA** |
| IV | `recharts@3.10.1` y `react-is@19.3.0`, versiones exactas y estables, autorizadas. Ninguna otra dependencia. | **PASA** |
| V | Sin cambios en Docker ni en archivos de entorno. | **PASA** |
| VI | Solo lectura (`getJson`). El `404` corta la vista antes de pedir cualquier otra cosa, así que una persona ajena no dispara ninguna consulta más. | **PASA** |
| VII | Reconocedores de tipo para las tres respuestas de la 008 y validación por porción de las series. Un valor inesperado (por ejemplo, un `not_calculable` en una métrica que nunca lo tiene) se muestra con una explicación genérica, sin errores. | **PASA** |
| VIII | Textos de la interfaz, títulos de gráficos y explicaciones en español (Supuestos de la spec). Valores de la API en inglés. `README.md` amplía "Panel de Control y Reportes" y la tabla del stack. | **PASA** |
| IX | Los artefactos generados son archivos de la copia de trabajo. | **PASA** |

**Resultado: todas las puertas siguen pasando después del diseño. La sección de seguimiento de
complejidad queda vacía.**

#### Dos decisiones que la constitución obliga a documentar

Ninguna es una violación, pero son concesiones conscientes y quedan registradas:

1. **El gráfico de velocidad omite puntos de la serie recibida.** No suma, no reordena ni completa,
   que es lo que prohíben FR-026 y RN-20, pero es una selección hecha en el frontend. Está aislada en
   `closedSprintPoints`, con su prueba unitaria (research, sección 8).
2. **El dashboard tiene textos propios para explicar los "no calculable"**, uno por métrica, porque
   la 008 no envía el motivo. No es un cálculo: nombra el único denominador posible de cada métrica
   según la 008 (research, sección 6).

---

## Estructura del Proyecto

### Documentación de esta feature

```text
specs/009-project-dashboard/
├── spec.md               # Especificación (entrada)
├── plan.md               # Este archivo (salida de /speckit-plan)
├── research.md           # Salida de la Fase 0
├── data-model.md         # Salida de la Fase 1
├── quickstart.md         # Salida de la Fase 1
├── contracts/
│   ├── consumed-api.md   # Salida de la Fase 1: operaciones que consume y campos que lee
│   └── ui-contract.md    # Salida de la Fase 1: ruta, nombres accesibles y textos fijos
├── checklists/
│   └── requirements.md   # Checklist de calidad de la spec
└── tasks.md              # Salida de /speckit-tasks; NO lo crea /speckit-plan
```

No hay `openapi.yaml`, porque la feature no expone ninguna API (contracts/consumed-api.md).

### Código fuente (raíz del repositorio)

Las líneas marcadas con `NUEVO` las crea esta feature; las marcadas con `TOCA` son archivos de las
features 001 a 008 que esta feature modifica de forma puntual. Todo lo demás existe y no se toca.

```text
frontend/
├── src/
│   ├── app/
│   │   └── projects/
│   │       └── [projectId]/
│   │           └── dashboard/
│   │               └── page.tsx                  # NUEVO: Server Component; lee params y monta SessionGuard + DashboardView
│   ├── features/
│   │   ├── dashboard/                            # NUEVO: módulo de esta feature
│   │   │   ├── api.ts                            # getProjectMetrics, getActiveSprintMetrics, getProjectSeries, reconocedores de tipo, readSeries
│   │   │   ├── series.ts                         # closedSprintPoints (US3-2)
│   │   │   └── components/
│   │   │       ├── dashboard-view.tsx            # Compone las secciones; resumen bloqueante, reintento, estado vacío
│   │   │       ├── project-summary.tsx           # Nombre, estado, fechas e integrantes (MemberList de la 002)
│   │   │       ├── active-sprint-panel.tsx       # Sprint activo con SprintSchedule de la 004
│   │   │       ├── key-indicators.tsx            # Grupos "Indicadores del sprint activo" e "Indicadores del proyecto"
│   │   │       ├── metric-value.tsx              # Único presentador de Metric y SprintMetric
│   │   │       ├── finality-badge.tsx            # Marcas "Parcial" y "Provisoria"
│   │   │       ├── sprint-series-chart.tsx       # Barras agrupadas de Recharts, estados vacío y de un punto, tabla de datos
│   │   │       ├── velocity-chart.tsx            # Story Points planificados y completados, solo Cerrados
│   │   │       ├── effort-chart.tsx              # Horas estimadas y reales
│   │   │       ├── quality-chart.tsx             # Defectos detectados y resueltos
│   │   │       ├── open-defects-distribution.tsx # Cuatro severidades y total
│   │   │       ├── dashboard-warnings.tsx        # Advertencias por kind
│   │   │       ├── dashboard-empty-state.tsx     # Proyecto sin sprints
│   │   │       └── section-error.tsx             # "No se pudo cargar <sección>."
│   │   └── projects/
│   │       └── components/
│   │           └── project-overview.tsx          # TOCA: suma el enlace "Dashboard" a la navegación
│   ├── hooks/
│   │   └── use-authorized-query.ts               # TOCA: el estado de error suma status (código HTTP)
│   ├── types/
│   │   └── metrics.ts                            # NUEVO: tipos de las respuestas de la 008 que se usan
│   └── utils/
│       ├── metric-format.ts                      # NUEVO: formatDecimal y formatShare
│       └── metric-explanations.ts                # NUEVO: explainNotCalculable
├── package.json                                  # TOCA: recharts 3.10.1 y react-is 19.3.0, sin rango
└── package-lock.json                             # TOCA: regenerado por npm install

backend/                                          # Sin cambios

README.md                                         # TOCA: describe el dashboard y la librería de gráficos
```

**Pruebas**: junto al código que prueban, como en el resto del frontend. Escenarios:
`project-summary.test.tsx` (US1), `key-indicators.test.tsx` (US2), `velocity-chart.test.tsx` (US3),
`effort-chart.test.tsx` (US4), `quality-chart.test.tsx` (US5) y `dashboard-view.test.tsx` (US6), todos
en `features/dashboard/components/` y todos montando `DashboardView` con el backend de prueba.
Unitarias: `metric-value.test.tsx`, `sprint-series-chart.test.tsx`, `features/dashboard/api.test.ts`,
`features/dashboard/series.test.ts`, `utils/metric-format.test.ts` y
`utils/metric-explanations.test.ts`. El backend de prueba, el `ResizeObserver` mínimo y los datos de
la spec se comparten desde `features/dashboard/components/dashboard-test-backend.ts`, que solo
importan los archivos de prueba.

**Decisión de estructura**: se mantiene la aplicación web con `backend/` y `frontend/` como
directorios de primer nivel (Principio III). El frontend agrega **un** módulo,
`features/dashboard/`, con su API y sus componentes, una página en `app/` y archivos en `types/` y
`utils/`. El backend no cambia: todas las cifras ya las expone la 008.

---

## Trazabilidad de escenarios a pruebas (Principio II)

Cada escenario Given/When/Then de la spec se implementa como al menos una prueba cuyo nombre
identifica la historia y el escenario. Buscar `009/US` en el código de pruebas devuelve la
trazabilidad completa.

**Frontend (Vitest)**: un bloque `describe("009/USn - <título de la historia>")` por historia, con un
`it("<nombre del escenario>")` por escenario. Como la spec no les pone nombre a los escenarios, el
nombre de cada `it` es el de esta tabla, y cada prueba lleva antes el comentario
`// Escenario: USn-k (<caso>)` con el número y el tipo de caso de la spec. La spec no nombra sus
escenarios; los nombres de esta tabla son la referencia para `it(...)` que exige el Principio II.

**Backend (Go)**: no aplica; esta feature no tiene backend.

| Historia | `describe` | Escenarios (`it`) |
| --- | --- | --- |
| US1 (9) | `009/US1 - Ver el estado del proyecto y del sprint en curso` | 1 "muestra el resumen y el sprint activo con 10 días restantes"; 2 "muestra el proyecto finalizado con su fecha real y sin sprint en curso"; 3 "informa que no hay sprint en curso sin tratarlo como error"; 4 "muestra 0 días restantes el último día del sprint"; 5 "muestra el sprint vencido y por cuánto"; 6 "muestra los días restantes que entrega la feature de sprints"; 7 "muestra un único integrante sin estado especial"; 8 "informa que el proyecto no existe"; 9 "responde a quien no es integrante igual que a un proyecto inexistente" |
| US2 (7) | `009/US2 - Ver los indicadores clave del momento` | 1 "muestra los indicadores con los valores de la feature de métricas"; 2 "marca como parciales los indicadores del sprint activo y no la velocidad"; 3 "muestra la marca de parcial del acumulado"; 4 "muestra la velocidad no calculable con su explicación"; 5 "muestra la desviación porcentual no calculable y la absoluta como número"; 6 "muestra 0 defectos abiertos como cifra real"; 7 "informa la falla de los indicadores y muestra el resto" |
| US3 (7) | `009/US3 - Ver la evolución de la velocidad` | 1 "grafica los cuatro sprints cerrados en el orden recibido"; 2 "excluye al sprint activo del gráfico de velocidad"; 3 "muestra un único sprint con la indicación de que no hay evolución"; 4 "muestra un estado vacío sin sprints cerrados"; 5 "identifica cada uno de 20 sprints"; 6 "muestra un sprint con 0 Story Points completados"; 7 "informa la falla del gráfico de velocidad y muestra los demás" |
| US4 (6) | `009/US4 - Ver la evolución del esfuerzo` | 1 "grafica las horas estimadas y reales por sprint"; 2 "marca como parcial al sprint activo"; 3 "muestra 0 horas estimadas sin perder el punto"; 4 "muestra 0 horas reales en un sprint sin registros"; 5 "muestra un estado vacío sin sprints con esfuerzo"; 6 "informa la falla del gráfico de esfuerzo y muestra los demás" |
| US5 (7) | `009/US5 - Ver la calidad del producto` | 1 "grafica los defectos detectados y resueltos por sprint"; 2 "muestra la distribución por severidad con su total"; 3 "muestra más resueltos que detectados sin tratarlo como error"; 4 "muestra 0 en las cuatro severidades sin defectos"; 5 "muestra una sola severidad y las otras en 0"; 6 "no muestra los defectos descartados"; 7 "informa la falla de la sección de calidad y muestra el resto" |
| US6 (9) | `009/US6 - Usar el dashboard cuando falta información o falla una sección` | 1 "muestra el estado vacío en un proyecto recién creado"; 2 "muestra el resumen y avisa que no hay sprints"; 3 "muestra la advertencia de planificación"; 4 "distingue la advertencia de inconsistencia de la de planificación"; 5 "aísla la falla del gráfico de calidad"; 6 "informa por separado la falla de dos secciones"; 7 "muestra la sección que vuelve a responder al recargar"; 8 "muestra los datos vigentes al recargar después de cerrar el sprint"; 9 "no muestra el dashboard si falla el resumen e invita a reintentar" |

**Total: 45 escenarios.**

**Cómo se preparan los escenarios**:

- El backend de prueba responde las seis operaciones de
  [contracts/consumed-api.md](./contracts/consumed-api.md) con los datos exactos de la spec (por
  ejemplo, velocidad 14,00, 18 y 5 Story Points, 27,78 %, 108 contra 40 horas y −62,96 % en US2-1).
  Cada prueba cambia solo la respuesta que su escenario necesita.
- Las fallas de sección se fuerzan con un `500` en la operación de esa sección. La falla de un solo
  gráfico (US3-7, US4-6, US5-7, US6-5) se fuerza con su porción de las series malformada, así que
  los otros dos gráficos se dibujan con la misma respuesta.
- La recarga (US6-7, US6-8) se simula desmontando y volviendo a montar `DashboardView` con otra
  respuesta del backend de prueba, que es lo que pasa al recargar la página.
- Las fechas no importan: los días restantes llegan resueltos por la 004 (US1-6). Por eso US1-4 y
  US1-5 usan `remainingDays: 0` y `overdueDays: 3`, sin reloj.
- US1-9 compara el contenido mostrado ante un `404` de "proyecto ajeno" y ante un `404` de
  "inexistente": deben ser idénticos.

**Pruebas transversales sin etiqueta de escenario**: la de sesión (sin token, la guardia redirige a
`/login`, SC-014), la de que ninguna consulta del dashboard usa otro método que `GET` (RN-02), la de
que con el `404` del resumen no sale ninguna otra consulta (SC-015) y la de que el dashboard no
vuelve a consultar sin recargar (RN-17, con temporizadores falsos). Se nombran sin `009/USn` para no
alterar el conteo de 45.

Además, pruebas unitarias **obligatorias** por el Principio II (reglas de negocio de presentación y
validación de entradas), que cubren SC-016:

| Objeto de prueba | Reglas cubiertas |
| --- | --- |
| Datos de US2-1 y US3-1 elegidos para que cualquier recálculo dé otro número (una velocidad que no es el promedio de la serie de la misma respuesta) | RN-01 |
| Ninguna llamada de `features/dashboard/api.ts` usa un método distinto de `GET`; `MemberList` sin acciones | RN-02 |
| `MetricValue` con `not_calculable` en velocidad, desviación porcentual y porcentaje de historias: texto y explicación, nunca "0", "—" ni vacío | RN-03 |
| `MetricValue` con `finality: partial` | RN-04 |
| `MetricValue` con `provisional` y con `partial`: textos distintos | RN-05 |
| `MetricValue` con `available` y `value: 0`, con `not_calculable` y con `not_applicable`: tres textos distintos | RN-06 |
| `SprintSeriesChart` con puntos en un orden que no es alfabético ni por valor: los dibuja en el orden recibido | RN-07 |
| `SprintSeriesChart`: título, rótulos de los dos ejes y leyenda con las dos series | RN-08 |
| `SprintSeriesChart` con un punto | RN-09 |
| `SprintSeriesChart` con cero puntos: no dibuja ejes | RN-10 |
| `DashboardView` con `emptyReason: no_sprints` | RN-11 |
| `KeyIndicators` y `ActiveSprintPanel` sin sprint Activo | RN-12 |
| `ActiveSprintPanel` con `remainingDays` 10 y 0 | RN-13 |
| `ActiveSprintPanel` con `overdueDays` 3: "Vencido hace 3 días", nunca "−3" | RN-14 |
| `DashboardView` con cada una de las cinco consultas degradables en `500`, de a una | RN-15 |
| `DashboardView` con el resumen en `500` y en `404` | RN-16 |
| `DashboardView` con temporizadores falsos: ninguna consulta nueva sin volver a montar | RN-17 |
| `DashboardView` ante el `404` del resumen: ninguna otra consulta | RN-18 |
| `DashboardView` con un proyecto `finished` | RN-19 |
| `closedSprintPoints`: conserva el orden y los valores, solo omite `active`; `readSeries` con una porción malformada | RN-20 |
| `DashboardWarnings` con advertencias de las dos clases: dos listas con roles distintos y las métricas presentes | RN-21 |
| `formatDecimal` y `formatShare` con 14,00; 27,78; 0; 13,10: el texto leído de vuelta es el número recibido | FR-013 |
| `explainNotCalculable` con las tres métricas y con una desconocida | FR-014 |
| Reconocedores de `features/dashboard/api.ts` con cuerpos válidos e inválidos | Principio VII |

Comparación con los criterios de éxito: SC-001 a SC-003 con US1-1 y US2-1 (todo en una pantalla);
SC-004 con las pruebas de US2 a US5, que comparan cada cifra y cada fila de las tablas con los datos
del backend de prueba; SC-005 con US2-4, US2-5 y la fila RN-03; SC-006 con US2-2, US4-2 y la fila
RN-04; SC-007 con la fila RN-08 aplicada a los tres gráficos; SC-008 con US3-1 y la fila RN-07;
SC-009 con US3-5; SC-010 con US6-5, US6-6 y la fila RN-15; SC-011 con US6-9; SC-012 con US6-1 y la
fila RN-10; SC-013 con US6-8; SC-014 y SC-015 con US1-9 y las pruebas transversales; SC-016 con la
tabla de arriba.

---

## Secuencia de trabajo prevista

El orden lo fija `/speckit-tasks`; esto es la dependencia entre bloques, no la lista de tareas.

0. **Prerrequisito**: las features 001 a 008 en `main`. Ya lo están.
1. **Fase técnica fundacional**: instalar `recharts@3.10.1` y `react-is@19.3.0` sin rango;
   `types/metrics.ts`; página, `DashboardView` vacía y enlace desde `ProjectOverview`; backend de
   prueba compartido. Es el bloque que bloquea a todas las historias. Lo que tiene lógica y prueba
   unitaria obligatoria entra en la historia que lo usa primero, para que cada una tenga un RED real.
2. **US1 Resumen y sprint en curso** (P1): resumen bloqueante con su `404`, `ActiveSprintPanel`
   con `SprintSchedule`.
3. **US2 Indicadores clave** (P1): `MetricValue`, `FinalityBadge`, `SectionError`,
   `formatDecimal`, `formatShare`, `explainNotCalculable`, los reconocedores de las métricas y
   `KeyIndicators` con sus dos grupos.
4. **US3 Velocidad** (P2): `SprintSeriesChart` con Recharts, sus estados y su tabla;
   `closedSprintPoints`; `VelocityChart`.
5. **US4 Esfuerzo** (P2): `EffortChart` sobre `SprintSeriesChart`.
6. **US5 Calidad** (P2): `QualityChart` y `OpenDefectsDistribution`.
7. **US6 Información faltante y fallas** (P3): `httpStatus` en el error de `useAuthorizedQuery`,
   reintento del resumen, estado vacío del proyecto, advertencias, degradación cruzada entre
   secciones y recarga.
8. **Cierre**: `README.md`, cobertura (`npm run test:coverage`), `lint`, `format:check` y la
   verificación transversal de trazabilidad (`009/US`) y de las puertas de calidad.

En cada historia el ciclo es el del Principio II: escribir la prueba, **verificar que falla y
detenerse** para que el desarrollador registre el commit RED con `/redactar-commit`, implementar,
verificar que pasa, refactorizar. Un agente no avanza de RED a GREEN sin ese alto.

---

## Seguimiento de Complejidad

> Se completa SOLO si la verificación de la constitución tiene violaciones que justificar.

**Sin violaciones.** Las dos puertas de evaluación pasan en su totalidad. La dependencia nueva cuenta
con la autorización que prevé el Principio IV, y las dos concesiones documentadas, que son la
selección del gráfico de velocidad y los textos de explicación, están justificadas en la sección
"Verificación de la Constitución" y en [research.md](./research.md).
