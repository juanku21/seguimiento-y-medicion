# Guía de validación: Dashboard del Proyecto

**Feature**: `specs/009-project-dashboard` | **Fecha**: 2026-10-28

Cómo comprobar que el dashboard funciona de punta a punta. Los pasos de instalación y arranque son
los del `README.md` (secciones 3 a 7) y no se repiten acá. Las fuentes de cada sección están en
[contracts/consumed-api.md](./contracts/consumed-api.md) y los textos esperados, en
[contracts/ui-contract.md](./contracts/ui-contract.md).

## Prerrequisitos

- Las features 001 a 008 en `main`. El dashboard no tiene backend propio: si falta alguna, la
  sección que depende de ella informa que no pudo cargarse.
- Las dependencias del frontend instaladas con `npm ci` en `frontend/`, que incluyen `recharts@3.10.1`
  y `react-is@19.3.0`, fijadas sin rango en `package.json`.
- Base de desarrollo, backend y frontend en ejecución (`README.md`, secciones 4, 6 y 7).

## 1. Pruebas automáticas

```bash
cd frontend
npm test                       # todas las pruebas de Vitest
npx vitest run src/features/dashboard   # solo las del dashboard
npm run test:coverage          # reporte de cobertura (Principio II)
npm run lint
npm run format:check
```

**Resultado esperado**: todas pasan. Buscar `009/US` en `frontend/src` devuelve los seis bloques
`describe("009/USn - …")` y los **45** `it` de los escenarios
(`grep -rn "009/US" frontend/src --include=*.test.tsx`).

Las pruebas no usan el backend ni la base: sustituyen `fetch` por un backend de prueba que responde
según los contratos. Por eso no hace falta levantar el entorno de test de Docker para esta feature.

## 2. Validación manual en el navegador

Preparar los datos con la interfaz o con los endpoints de las features 002 a 007, con dos usuarios
registrados: A (propietario) y B (no integrante).

| # | Preparación | Acción | Resultado esperado |
| --- | --- | --- | --- |
| 1 | A crea un proyecto, sin historias ni sprints | Abrir `/projects/{id}` y seguir "Dashboard" | Resumen con A como único integrante (US1-7), "No hay ningún sprint en curso." y el estado vacío "Sin datos todavía" en lugar de indicadores y gráficos (US6-1) |
| 2 | Agregar 6 historias sin estimar | Recargar | Aviso de planificación con las 6 historias (US6-3); sigue el estado vacío (US6-2) |
| 3 | Estimar, crear un sprint que termine dentro de 10 días, comprometer historias e iniciarlo | Recargar | Sprint activo con nombre, Sprint Goal, fechas y "10 días restantes" (US1-1), los mismos que muestra el detalle del sprint en `/projects/{id}/sprints/{sprintId}` (US1-6). Indicadores del sprint activo con la marca "Parcial" (US2-2). Velocidad "No calculable" con "Todavía no hay sprints cerrados." (US2-4). Gráfico de velocidad vacío (US3-4); esfuerzo y calidad con un único punto, marcado "(parcial)", y "Todavía no hay evolución que comparar." |
| 4 | Cargar horas, completar historias, registrar defectos de distintas severidades y cerrar el sprint | Recargar | Velocidad igual a los Story Points completados, sin marca. Gráfico de velocidad con un punto. Distribución con las cuatro severidades y el total (US5-2). Valores "Provisoria" en las horas reales del sprint cerrado durante el plazo de gracia |
| 5 | Repetir el paso 4 hasta tener 4 sprints cerrados y uno activo | Recargar | Velocidad con los 4 cerrados, sin el activo (US3-2); esfuerzo y calidad con los 5, el activo marcado (US4-2). Cada valor coincide con `GET /api/v1/projects/{id}/metrics/series` (SC-004) |
| 6 | Con el dashboard abierto, cerrar el sprint activo desde otra pestaña | Volver al dashboard y recargar | Sin sprint activo y con el sprint ya cerrado (US6-8, SC-013). Antes de recargar, la pantalla no cambió sola (FR-031) |
| 7 | — | B abre `/projects/{id}/dashboard` | "El proyecto no existe.", igual que con un identificador inventado (US1-8, US1-9) |
| 8 | Detener el backend | Recargar el dashboard | "No se pudo mostrar el dashboard." con "Reintentar" (US6-9). Volver a levantar el backend y pulsar "Reintentar" muestra el dashboard (US6-7) |
| 9 | A cambia el proyecto a Finalizado | Recargar | Estado Finalizado, fecha de finalización real y "No hay ningún sprint en curso." (US1-2); el resto se muestra igual (FR-036) |
| 10 | En otro proyecto, crear primero un sprint con fechas posteriores y después uno con fechas anteriores; iniciar y cerrar los dos en orden de fechas | Abrir su dashboard | Los tres gráficos muestran los sprints por fecha, no por orden de creación (SC-008) |

**Comparación con la fuente (SC-004)**: con la sesión de A, comparar cada cifra del dashboard con
`GET /api/v1/projects/{id}/metrics`, `.../metrics/active-sprint` y `.../metrics/series` en Swagger
(`http://localhost:8080/swagger/index.html`). No debe haber ninguna diferencia, salvo el formato
(coma decimal, unidad y signo tipográfico).

## 3. Falla de una sección (SC-010)

La falla aislada de una sección se verifica con las pruebas automáticas, porque en el navegador
todos los endpoints viven en el mismo backend. Las pruebas de US6 responden `500` en un endpoint por
vez, o una porción malformada de las series, y comprueban que solo esa sección lo informa (US6-5,
US6-6).
