# Guía de validación (Fase 1): Reportes de Proyecto y de Sprint

**Feature**: `specs/010-project-reports` | **Fecha**: 2026-10-30

Escenarios ejecutables que prueban la feature de punta a punta. Los contratos están en
[contracts/openapi.yaml](./contracts/openapi.yaml) y [contracts/ui-contract.md](./contracts/ui-contract.md);
el modelo, en [data-model.md](./data-model.md).

## Prerrequisitos

- Las features 001 a 009 integradas en `main` (la 010 reutiliza tipos y formateadores de la 009).
- Docker Desktop en ejecución, Go 1.27.3 y Node.js con las dependencias del frontend instaladas.
- `pdfmake@0.3.13` instalada sin rango en `frontend/package.json`.

## 1. Pruebas automatizadas

```bash
# Base de test (puerto y datos disjuntos del desarrollo, Principio V)
cd backend/postgres
docker compose --env-file .env.test -p smye-test up -d
cd ../..

# Backend: integración y unitarias del módulo, y la suite completa con cobertura
cd backend
go test ./internal/reports/...
go test ./... -coverprofile=coverage.out

# Frontend
cd ../frontend
npm run test -- src/features/reports src/utils/report-file-name.test.ts
npm run test:coverage
npm run lint
npm run format:check
```

**Resultado esperado**: todo en verde. Trazabilidad: `grep -rn "010/US" backend frontend/src`
devuelve los 35 escenarios (10 + 8 + 11 + 6), y cada `TestUSn_` del backend lleva su comentario
`// Escenario: … (010/USn)`.

## 2. API a mano

Con el entorno de desarrollo levantado (`backend/postgres` con `.env` y `go run ./cmd/app`) y un
token de un integrante en `$TOKEN`:

```bash
# Reporte de un sprint Cerrado → 200, partial false, seis secciones
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/projects/$PROJECT/reports/sprints/$CLOSED_SPRINT

# Reporte del proyecto → 200; partial true si hay un sprint Activo
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/projects/$PROJECT/reports/project

# Sprint Planificado → 409 "El sprint todavía no arrancó: no tiene nada que reportar."
curl -s -o /dev/null -w "%{http_code}\n" -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/projects/$PROJECT/reports/sprints/$PLANNED_SPRINT
```

Verificaciones:

| Qué | Cómo | Esperado |
| --- | --- | --- |
| Cifras de la 008 sin recalcular (SC-004) | Comparar `metrics.sprint` con `GET …/metrics/sprints/$CLOSED_SPRINT` | Idénticos |
| Esfuerzo cuadra (research, sección 6) | `effort.totalHours` contra `metrics.sprint.actualHours.value` | Iguales |
| Defectos cuadran (SC-016) | Filas con `detectedSprint.id` del sprint contra `defects.totals.detected` | Iguales, sin Descartados |
| Registro de generación (SC-018) | `SELECT scope, sprint_id, generated_by_id, generated_at FROM report_generations ORDER BY generated_at DESC LIMIT 2;` | Una fila por cada `200`, ninguna por el `409` |
| Reproducibilidad (SC-017) | Renombrar y reestimar una historia del sprint Cerrado y volver a generar | Todo igual salvo `generatedAt` |
| Swagger | `http://localhost:8080/swagger/index.html`, etiqueta "Reportes" | Las dos operaciones, en español, iguales al contrato |

## 3. Interfaz y PDF a mano

Con el frontend en `npm run dev`:

1. Abrir un proyecto, entrar en **Reportes**, elegir un sprint Cerrado y **Generar reporte**. Se ven
   las seis secciones en el orden del contrato de interfaz (SC-001: tres pasos).
2. **Exportar a PDF**. El archivo se descarga con un nombre como
   `Reporte_<proyecto>_sprint-<nombre>_<fecha>_<hora>.pdf` (SC-002: un paso).
3. Abrir el PDF y comparar sección por sección con la pantalla (SC-003).
4. Elegir el sprint Activo: aparece la marca "Reporte parcial" en la pantalla, debajo del título del
   PDF y en el pie de cada página (SC-006).
5. **Paginación** (SC-010): con un proyecto de 80 historias (sembrado con un script de prueba o a
   mano), generar el reporte de proyecto y exportarlo. Todas las páginas muestran "Página N de M",
   las tablas que cruzan de página repiten su encabezado y ninguna fila queda partida ni
   superpuesta.
6. **Caracteres** (SC-011, SC-012): con un proyecto llamado "Ñandú: Gestión/Compras *beta*" y una
   historia "Integración con AFIP — Facturación electrónica", el PDF muestra los acentos, la eñe y
   la raya, y el archivo se llama `Reporte_Nandu-Gestion-Compras-beta_…pdf`.
7. **Dos reportes el mismo día** (US2-6): exportar dos veces el mismo alcance; los nombres difieren en
   la hora.
8. **Falla de exportación** (SC-013): en DevTools, bloquear la URL del fragmento de pdfmake (Network →
   Block request URL) y exportar. Aparece "No se pudo generar el PDF. Intentá de nuevo." y el reporte
   sigue en pantalla. Al desbloquearla, exportar funciona sin volver a generar.
