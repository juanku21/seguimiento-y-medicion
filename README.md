# Software Metrics & Estimation

Software Metrics & Estimation es un sistema diseñado para la estimación, seguimiento, medición y control de proyectos de software. La plataforma centraliza la administración del flujo de trabajo, la estimación del volumen de tareas, el registro del tiempo invertido, la trazabilidad de defectos y el cálculo automático de indicadores de rendimiento y calidad.

---

## Tabla de Contenidos

- [Descripción General](#descripción-general)
- [Funcionalidades del Sistema](#funcionalidades-del-sistema)
  - [Gestión de Proyectos e Integrantes](#gestión-de-proyectos-e-integrantes)
  - [Administración del Backlog](#administración-del-backlog)
  - [Planificación y Control de Iteraciones](#planificación-y-control-de-iteraciones)
  - [Estimación Colaborativa](#estimación-colaborativa)
  - [Registro de Esfuerzo](#registro-de-esfuerzo)
  - [Gestión de Defectos](#gestión-de-defectos)
- [Análisis de Métricas](#análisis-de-métricas)
- [Panel de Control y Reportes](#panel-de-control-y-reportes)
- [Cobertura de Pruebas](#cobertura-de-pruebas)
- [Análisis de Vulnerabilidades](#análisis-de-vulnerabilidades)
- [Equipo de Trabajo](#equipo-de-trabajo)

---

## Descripción General

El sistema ofrece una solución integral para gestionar el ciclo de vida de proyectos de software. Permite coordinar actividades entre colaboradores, evaluar la precisión de las estimaciones frente al esfuerzo real ejecutado y monitorear la salud global del proyecto mediante herramientas visuales y reportes consolidados.

---

## Funcionalidades del Sistema

### Gestión de Proyectos e Integrantes
- Alta, edición y seguimiento del estado general de proyectos.
- Registro de integrantes asociados a cada proyecto.
- Control de fechas de inicio y estimación de finalización.

### Administración del Backlog
- Registro estructurado de tareas e historias con identificador, título, descripción y criterios de aceptación.
- Clasificación por prioridades y estados.
- Dimensionamiento de trabajo mediante puntos de estimación.

### Planificación y Control de Iteraciones
- Creación, seguimiento y cierre de períodos de trabajo.
- Definición de metas para cada ciclo.
- Asignación de tareas a iteraciones específicas y registro de elementos completados.
- Consulta e historial de ciclos anteriores.

### Estimación Colaborativa
- Mecanismo de votación individual que mantiene las estimaciones ocultas hasta finalizar la ronda.
- Visualización simultánea de puntajes y detección automática de diferencias en las valoraciones del equipo.
- Soporte para múltiples rondas de votación hasta alcanzar un acuerdo e ingresar la estimación definitiva.

### Registro de Esfuerzo
- Carga de actividades asociadas a integrantes, fechas y horas trabajadas.
- Comparación cuantitativa entre el tiempo estimado inicialmente y el tiempo real dedicado.

### Gestión de Defectos
- Registro de incidencias con descripción, nivel de severidad y estado de resolución.
- Vinculación directa con la tarea o historia afectada.
- Seguimiento y trazabilidad entre el ciclo de detección del problema y el ciclo de su resolución.

---

## Análisis de Métricas

La aplicación calcula continuamente indicadores para medir la eficiencia y calidad del desarrollo:

- **Volumen de Trabajo:** Comparativa entre la cantidad de puntos planificados y los completados.
- **Velocidad:** Medición del ritmo de entrega alcanzado por el equipo.
- **Desviación de Tiempo:** Diferencia porcentual y directa entre horas estimadas y horas reales registradas.
- **Porcentaje de Avance:** Proporción de historias de usuario completadas sobre el total previsto.
- **Control de Incidencias:** Cantidad de defectos detectados frente a los resueltos.

---

## Panel de Control y Reportes

- **Panel de Control (Dashboard):** Muestra de forma visual el estado del proyecto e indicadores clave del progreso.
- **Reportes Exportables:** Generación de informes detallados por proyecto o por ciclo, con el desglose de tareas, métricas, registro de tiempo y balance de defectos en formato PDF.

---

## Cobertura de Pruebas

El backend y el frontend exponen cada uno un comando para generar el reporte de cobertura de pruebas. Los reportes son artefactos reproducibles y no se versionan (`.gitignore`).

### Backend

Las pruebas de integración corren siempre contra la base de **test** (`smye_test`, puerto 5433), configurada en `backend/.env.test`; nunca contra la base de desarrollo. Antes de medir la cobertura hay que levantar ese contenedor:

```bash
cd backend/postgres
docker compose --env-file .env.test -p smye-test up -d
```

Con la base de test arriba, desde `backend/`:

```bash
# Ejecuta todas las pruebas y guarda el perfil de cobertura en coverage.out
go test ./... -coverprofile=coverage.out

# Muestra la cobertura por función y el total en la terminal
go tool cover -func=coverage.out

# Genera el reporte navegable en coverage.html
go tool cover -html=coverage.out -o coverage.html
```

### Frontend

Desde `frontend/`, con las dependencias instaladas (`npm ci`):

```bash
# Ejecuta Vitest una vez con el proveedor de cobertura v8
npm run test:coverage
```

El resumen se imprime en la terminal y el reporte navegable queda en `frontend/coverage/index.html`.

---

## Análisis de Vulnerabilidades

Las dependencias del backend y la versión de Go se revisan con `govulncheck` (versión 1.8.0), que informa las vulnerabilidades conocidas que alcanzan al código. Desde `backend/`:

```bash
# Con la herramienta instalada (go install golang.org/x/vuln/cmd/govulncheck@v1.8.0)
govulncheck ./...

# Sin instalarla, sin modificar go.mod
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

El último análisis (2026-10-13) no reporta vulnerabilidades que alcancen al código. Queda un único aviso a nivel de módulo, justificado a continuación:

| Aviso | Módulo | Justificación |
| --- | --- | --- |
| [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932) | `golang.org/x/crypto` | El aviso corresponde al paquete `golang.org/x/crypto/openpgp`, que no tiene versión corregida. El proyecto solo usa `golang.org/x/crypto/bcrypt` y ningún paquete importa `openpgp`, por lo que el código vulnerable no se compila en la aplicación. |

---

## Equipo de Trabajo

- **Diaz Rossi, Juan Cruz** - Agile Enable
- **Damiani, Macarena** - Product Builder
- **Maturano, Pablo** - Product Builder
- **Mercado, Martín** - Product Builder
- **Limeres, Tomás** - Product Builder
