# Software Metrics & Estimation

Software Metrics & Estimation es un sistema diseñado para la estimación, seguimiento, medición y control de proyectos de software. La plataforma centraliza la administración del flujo de trabajo, la estimación del volumen de tareas, el registro del tiempo invertido, la trazabilidad de defectos y el cálculo automático de indicadores de rendimiento y calidad.

---

## Tabla de Contenidos

- [Descripción General](#descripción-general)
- [Estado Actual](#estado-actual)
- [Funcionalidades del Sistema](#funcionalidades-del-sistema)
  - [Gestión de Proyectos e Integrantes](#gestión-de-proyectos-e-integrantes)
  - [Administración del Backlog](#administración-del-backlog)
  - [Planificación y Control de Iteraciones](#planificación-y-control-de-iteraciones)
  - [Estimación Colaborativa](#estimación-colaborativa)
  - [Registro de Esfuerzo](#registro-de-esfuerzo)
  - [Gestión de Defectos](#gestión-de-defectos)
- [Análisis de Métricas](#análisis-de-métricas)
- [Panel de Control y Reportes](#panel-de-control-y-reportes)
- [Arquitectura y Stack Tecnológico](#arquitectura-y-stack-tecnológico)
- [Puesta en Marcha](#puesta-en-marcha)
  - [1. Prerrequisitos](#1-prerrequisitos)
  - [2. Obtener el código](#2-obtener-el-código)
  - [3. Crear los archivos de entorno](#3-crear-los-archivos-de-entorno)
  - [4. Levantar las bases de datos](#4-levantar-las-bases-de-datos)
  - [5. Generar la documentación Swagger](#5-generar-la-documentación-swagger)
  - [6. Ejecutar el backend](#6-ejecutar-el-backend)
  - [7. Ejecutar el frontend](#7-ejecutar-el-frontend)
  - [8. Comprobar que todo funciona](#8-comprobar-que-todo-funciona)
  - [9. Ejecutar las pruebas](#9-ejecutar-las-pruebas)
  - [10. Ejecución con Docker](#10-ejecución-con-docker)
  - [11. Detener los servicios](#11-detener-los-servicios)
  - [Problemas frecuentes](#problemas-frecuentes)
- [Cobertura de Pruebas](#cobertura-de-pruebas)
- [Análisis de Vulnerabilidades](#análisis-de-vulnerabilidades)
- [Equipo de Trabajo](#equipo-de-trabajo)

---

## Descripción General

El sistema ofrece una solución integral para gestionar el ciclo de vida de proyectos de software. Permite coordinar actividades entre colaboradores, evaluar la precisión de las estimaciones frente al esfuerzo real ejecutado y monitorear la salud global del proyecto mediante herramientas visuales y reportes consolidados.

---

## Estado Actual

El sistema se construye por funcionalidades, cada una especificada en su carpeta de [`specs/`](./specs). Hoy están implementadas las dos primeras:

- **Autenticación y cuentas de usuario** ([`specs/001-user-auth`](./specs/001-user-auth)): registro de cuentas, inicio y cierre de sesión con JWT, protección de las funciones del sistema, consulta del propio perfil y registro de eventos de autenticación.
- **Gestión de proyectos e integrantes** ([`specs/002-project-members`](./specs/002-project-members)): creación y modificación de proyectos con su factor de horas por Story Point, listado de los proyectos propios aislado de los ajenos, alta y baja de integrantes por correo electrónico, consulta del estado del proyecto en una sola pantalla y avance del estado (`planned` → `in_progress` → `finished`) con registro de la fecha de finalización real.

Las funcionalidades descritas en las secciones siguientes son el alcance completo del sistema y se incorporan en las próximas specs.

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

## Arquitectura y Stack Tecnológico

El repositorio separa el backend y el frontend en dos directorios de primer nivel:

| Directorio | Contenido |
| --- | --- |
| `backend/` | API REST en Go con Gin y GORM. `cmd/app/` es el punto de entrada; cada módulo vive en `internal/<módulo>/` con sus capas `delivery/`, `service/`, `repository/` y `domain/`. |
| `backend/postgres/` | `docker-compose.yml` de PostgreSQL, compartido por los entornos de desarrollo y de test. |
| `frontend/` | Aplicación Next.js con Tailwind CSS. El código está en `src/`, organizado en `app/`, `features/`, `components/`, `config/`, `hooks/`, `lib/`, `providers/`, `styles/`, `types/` y `utils/`. |
| `specs/` | Especificación, plan y tareas de cada funcionalidad. |

| Ámbito | Tecnología y versión |
| --- | --- |
| Backend | Go 1.27.2, Gin, GORM, JWT (`golang-jwt/jwt/v5`), bcrypt |
| Base de datos | PostgreSQL 18.6 (imagen `postgres:18.6-alpine`) |
| Frontend | Node.js 24.21.0, Next.js 16.3.8, React 19.3.0, Tailwind CSS 4.3.3, TypeScript 5.9.3 |
| Documentación de la API | Swagger generado con `swag` 1.16.6 |
| Pruebas | `go test` con testify en el backend; Vitest 5.0.3 con Testing Library en el frontend |
| Contenedores | Docker y Docker Compose |

Los entornos de **desarrollo** y de **test** son disjuntos: cada uno tiene su propio contenedor de PostgreSQL, su propia base, su propio puerto y sus propios archivos de variables de entorno. Las pruebas nunca tocan la base de desarrollo.

| Entorno | Contenedor | Base | Puerto del host | Archivos de entorno |
| --- | --- | --- | --- | --- |
| Desarrollo | `smye-dev-postgres-1` | `smye_dev` | 5432 | `backend/postgres/.env`, `backend/.env`, `frontend/.env.local` |
| Test | `smye-test-postgres-1` | `smye_test` | 5433 | `backend/postgres/.env.test`, `backend/.env.test`, `frontend/.env.test` |

---

## Puesta en Marcha

Los comandos están escritos para una terminal tipo bash (Linux, macOS o Git Bash en Windows). Cuando un paso cambia en PowerShell, se indica la variante. Todas las rutas son relativas a la raíz del repositorio.

### 1. Prerrequisitos

| Herramienta | Versión | Para qué |
| --- | --- | --- |
| Git | cualquiera reciente | Obtener el código |
| Go | 1.27.2 | Compilar y probar el backend |
| Node.js | 24.21.0 (incluye npm 11) | Ejecutar y probar el frontend |
| Docker | 29.x | Ejecutar PostgreSQL y las imágenes de la aplicación |
| Docker Compose | v5.x (plugin `docker compose`) | Levantar las bases de datos |

Con Go 1.21 o posterior instalado alcanza: si la versión local es anterior a la 1.27.2 que declara `backend/go.mod`, Go descarga automáticamente la versión correcta la primera vez que se ejecuta un comando dentro de `backend/` (comportamiento por defecto, `GOTOOLCHAIN=auto`). Se puede comprobar con `go version` dentro de `backend/`.

No hace falta instalar PostgreSQL en la máquina: corre en contenedores.

### 2. Obtener el código

```bash
git clone https://github.com/juanku21/seguimiento-y-medicion.git
cd seguimiento-y-medicion
```

### 3. Crear los archivos de entorno

Los secretos y puertos viven en archivos `.env` que **no** se versionan. Cada uno se crea a partir de la plantilla `.env.example` de su directorio:

| Archivo | Se crea desde | Para qué |
| --- | --- | --- |
| `backend/postgres/.env` | `backend/postgres/.env.example` | PostgreSQL de desarrollo |
| `backend/postgres/.env.test` | `backend/postgres/.env.example` | PostgreSQL de test |
| `backend/.env` | `backend/.env.example` | Backend en desarrollo |
| `backend/.env.test` | `backend/.env.example` | Backend en las pruebas |
| `frontend/.env.local` | `frontend/.env.example` | Frontend en desarrollo |
| `frontend/.env.test` | `frontend/.env.example` | Frontend en las pruebas |

Copiar las plantillas:

```bash
cp backend/postgres/.env.example backend/postgres/.env
cp backend/postgres/.env.example backend/postgres/.env.test
cp backend/.env.example backend/.env
cp backend/.env.example backend/.env.test
cp frontend/.env.example frontend/.env.local
cp frontend/.env.example frontend/.env.test
```

En PowerShell, el equivalente de cada línea es `Copy-Item <origen> <destino>`.

Después, editar los valores. Los secretos se pueden generar con `openssl rand -hex 32` (o con cualquier generador de cadenas aleatorias de al menos 32 caracteres):

| Archivo | Clave | Valor |
| --- | --- | --- |
| `backend/postgres/.env` | `POSTGRES_PASSWORD` | Una contraseña propia para la base de desarrollo |
| `backend/postgres/.env.test` | `POSTGRES_PASSWORD` | Una contraseña propia para la base de test |
| `backend/postgres/.env.test` | `POSTGRES_DB` | `smye_test` |
| `backend/postgres/.env.test` | `POSTGRES_HOST_PORT` | `5433` |
| `backend/.env` | `DB_PASSWORD` | La misma que `POSTGRES_PASSWORD` de `backend/postgres/.env` |
| `backend/.env` | `JWT_SECRET` | Un secreto de **al menos 32 caracteres** |
| `backend/.env.test` | `DB_PASSWORD` | La misma que `POSTGRES_PASSWORD` de `backend/postgres/.env.test` |
| `backend/.env.test` | `DB_PORT` | `5433` |
| `backend/.env.test` | `DB_NAME` | `smye_test` |
| `backend/.env.test` | `JWT_SECRET` | Un secreto de **al menos 32 caracteres**, distinto del de desarrollo |

Los archivos del frontend no necesitan cambios: `NEXT_PUBLIC_API_BASE_URL=http://localhost:8080` apunta al backend local. El resto de las claves (`DB_HOST=localhost`, `DB_USER=smye`, `SERVER_PORT=8080`, `CORS_ALLOWED_ORIGIN=http://localhost:3000`, `APP_TIMEZONE=America/Argentina/Buenos_Aires`) sirven tal como vienen en las plantillas.

**`APP_TIMEZONE`** es la zona horaria única del sistema, en formato IANA (por ejemplo `America/Argentina/Buenos_Aires`, `America/Montevideo` o `UTC`). El backend la usa para fijar fechas de calendario que no dependen del dispositivo de quien opera: cuando un proyecto pasa a `finished`, su fecha de finalización real es la fecha del cambio en esta zona. No es un secreto, pero vive en `backend/.env` y `backend/.env.test` como el resto de la configuración, y debe estar en los dos. Si se crearon los archivos de entorno antes de la gestión de proyectos (spec 002), hay que agregarles la línea:

```bash
APP_TIMEZONE=America/Argentina/Buenos_Aires
```

Si `JWT_SECRET` falta o tiene menos de 32 caracteres, si `APP_TIMEZONE` falta o no es una zona horaria válida, o si falta cualquier otra variable, el backend no arranca e informa qué corregir. Las pruebas de integración se niegan a ejecutarse si `backend/.env.test` no apunta a la base `smye_test`.

### 4. Levantar las bases de datos

Los dos entornos usan el mismo `docker-compose.yml` con distinto archivo de variables y distinto nombre de proyecto, lo que da contenedores, volúmenes y puertos separados:

```bash
cd backend/postgres

# Desarrollo: puerto 5432, base smye_dev
docker compose --env-file .env -p smye-dev up -d

# Test: puerto 5433, base smye_test
docker compose --env-file .env.test -p smye-test up -d

# Comprobar que ambos contenedores están en ejecución
docker compose -p smye-dev ps
docker compose -p smye-test ps

cd ../..
```

Para confirmar que el contenedor de test apunta a la base correcta:

```bash
docker exec smye-test-postgres-1 psql -U smye -d smye_test -c "SELECT current_database();"
```

Debe responder `smye_test`. No hace falta crear tablas: el backend crea y migra el esquema automáticamente al conectarse (`AutoMigrate` de GORM), incluidas las tablas `projects` y `project_members` de la gestión de proyectos.

### 5. Generar la documentación Swagger

La documentación Swagger se genera a partir de las anotaciones del código en `backend/docs/`, que **no** se versiona. Como el backend importa ese paquete para publicar Swagger, **un clon recién hecho no compila hasta ejecutar este paso**: sin él, `go build`, `go run` y `go test` fallan con `no required module provides package .../backend/docs`.

Desde `backend/`:

```bash
cd backend

# Con la CLI instalada (una sola vez: go install github.com/swaggo/swag/cmd/swag@v1.16.6)
swag init -g cmd/app/main.go -o docs

# Sin instalarla, sin modificar go.mod
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/app/main.go -o docs
```

Hay que volver a ejecutarlo cada vez que cambian las anotaciones Swagger de algún endpoint. La versión de la CLI (1.16.6) debe coincidir con la de la librería `github.com/swaggo/swag` de `go.mod`.

### 6. Ejecutar el backend

Con la base de desarrollo arriba y Swagger generado, desde `backend/`:

```bash
go run ./cmd/app
```

El backend lee `backend/.env` del directorio actual, por eso se ejecuta desde `backend/`. Queda escuchando en <http://localhost:8080> y la terminal muestra `servidor escuchando`. Se detiene con `Ctrl+C`.

### 7. Ejecutar el frontend

En otra terminal, desde `frontend/`:

```bash
cd frontend
npm ci        # instala las dependencias exactas de package-lock.json (solo la primera vez)
npm run dev
```

El frontend queda en <http://localhost:3000> y lee `frontend/.env.local`. Para una compilación de producción se usa `npm run build` seguido de `npm start`.

### 8. Comprobar que todo funciona

| Dirección | Qué se ve |
| --- | --- |
| <http://localhost:3000/register> | Formulario de registro de cuenta |
| <http://localhost:3000/login> | Formulario de inicio de sesión |
| <http://localhost:3000/profile> | Perfil del usuario con sesión iniciada (sin sesión redirige a `/login`) |
| <http://localhost:3000/projects> | Listado de los proyectos de los que se es integrante, con su estado, fechas, cantidad de integrantes y la marca "Propietario" en los propios (sin sesión redirige a `/login`) |
| <http://localhost:3000/projects/new> | Formulario de creación de proyecto (sin sesión redirige a `/login`) |
| `http://localhost:3000/projects/<projectId>` | Estado del proyecto e integrantes en una sola pantalla; el propietario ve además los correos, el alta y la baja de integrantes y el botón para avanzar el estado (sin sesión redirige a `/login`; un proyecto inexistente o ajeno muestra "El proyecto no existe.") |
| `http://localhost:3000/projects/<projectId>/edit` | Modificación de nombre, descripción, fechas y horas por Story Point; solo el propietario ve el formulario y el resto de los integrantes ve "Solo el propietario del proyecto puede realizar esta acción." (sin sesión redirige a `/login`; un proyecto inexistente o ajeno muestra "El proyecto no existe.") |
| <http://localhost:8080/swagger/index.html> | Documentación interactiva de la API |

Recorrido mínimo: crear una cuenta en `/register`, iniciar sesión en `/login`, ver el perfil en `/profile` y cerrar sesión desde ahí.

Recorrido de la gestión de proyectos, con el backend y el frontend en ejecución:

1. Crear dos cuentas en `/register` (por ejemplo, Ana y Bruno). Ana será la propietaria y Bruno el integrante.
2. Iniciar sesión como Ana. Desde `/profile`, el enlace **Mis proyectos** lleva a `/projects`, que la primera vez muestra la lista vacía con un mensaje, no un error.
3. Abrir `/projects/new` y crear un proyecto con nombre, descripción opcional, fecha de inicio, fecha de finalización prevista y horas por Story Point (entre 0,01 y 40, con hasta dos decimales). El proyecto nace en estado **Planificado**, con Ana como propietaria y única integrante, y aparece en `/projects`.
4. Obtener el identificador del proyecto (`projectId`). El listado de la interfaz todavía no enlaza al detalle, así que el identificador se toma de la respuesta de `GET /api/v1/projects` (campo `id`), por ejemplo desde Swagger: en <http://localhost:8080/swagger/index.html>, ejecutar `POST /api/v1/auth/login` con las credenciales de Ana, copiar el token en **Authorize** como `Bearer <token>` y ejecutar `GET /api/v1/projects`.
5. Abrir `/projects/<projectId>`: muestra el estado, las fechas, el factor, los integrantes, el sprint activo (por ahora ninguno) y el resumen del backlog. Desde ahí Ana agrega a Bruno escribiendo su correo electrónico; solo se pueden agregar cuentas ya registradas.
6. Abrir `/projects/<projectId>/edit` para modificar los datos del proyecto. Solo la propietaria ve el formulario.
7. En `/projects/<projectId>`, usar **Pasar a En curso** y después **Finalizar proyecto**. Al finalizar se registra la fecha de finalización real con la fecha del día en la zona de `APP_TIMEZONE`, y el proyecto queda de solo lectura: ya no admite modificaciones, cambios de integrantes ni cambios de estado.
8. Iniciar sesión como Bruno: ve el proyecto en `/projects` y su detalle, pero no los correos de los integrantes ni las acciones de administración. Si Ana lo quita del proyecto antes de finalizarlo, para Bruno el proyecto pasa a comportarse como inexistente.

La API expone estos endpoints bajo `/api/v1`; solo el registro y el inicio de sesión son públicos, los demás exigen la cabecera `Authorization: Bearer <token>`. Fuera de `/api/v1`, la documentación Swagger (`/swagger/index.html`) también es pública, como excepción documentada a la regla RN-08 de `specs/001-user-auth/spec.md`:

| Método | Ruta | Descripción |
| --- | --- | --- |
| `POST` | `/api/v1/auth/register` | Registro de cuenta |
| `POST` | `/api/v1/auth/login` | Inicio de sesión; devuelve el token y su vencimiento |
| `POST` | `/api/v1/auth/logout` | Cierre de la sesión actual |
| `GET` | `/api/v1/users/me` | Perfil del titular de la sesión |
| `POST` | `/api/v1/projects` | Creación de proyecto; quien lo crea queda como propietario |
| `GET` | `/api/v1/projects` | Listado de los proyectos de los que se es integrante |
| `GET` | `/api/v1/projects/{projectId}` | Estado completo del proyecto (cualquier integrante) |
| `PUT` | `/api/v1/projects/{projectId}` | Modificación de los datos del proyecto (solo el propietario) |
| `PATCH` | `/api/v1/projects/{projectId}/status` | Avance del estado del proyecto (solo el propietario) |
| `POST` | `/api/v1/projects/{projectId}/members` | Alta de un integrante por correo electrónico (solo el propietario) |
| `DELETE` | `/api/v1/projects/{projectId}/members/{userId}` | Baja de un integrante (solo el propietario) |

Un proyecto inexistente y un proyecto del que no se es integrante responden el mismo `404`, para no revelar qué proyectos existen. Las acciones de administración devuelven `403` a un integrante que no es el propietario.

Las guías de validación de punta a punta, con ejemplos de `curl` para cada historia, están en [`specs/001-user-auth/quickstart.md`](./specs/001-user-auth/quickstart.md) y [`specs/002-project-members/quickstart.md`](./specs/002-project-members/quickstart.md).

### 9. Ejecutar las pruebas

Las pruebas del backend corren siempre contra la base de **test** (`smye_test`, puerto 5433), que debe estar levantada (paso 4), y requieren Swagger generado (paso 5). Cada caso de integración vacía las tablas de esa base antes de ejecutarse; nunca lee `backend/.env` ni toca la base de desarrollo.

```bash
cd backend
go vet ./...
go test ./...

# Solo las pruebas de un módulo (por ejemplo, la gestión de proyectos)
go test ./internal/projects/...

# Solo las pruebas de una historia de usuario (por ejemplo, US1)
go test ./... -run TestUS1
```

El frontend no necesita servicios levantados: Vitest simula el navegador con jsdom.

```bash
cd frontend
npm test              # Vitest, una sola corrida
npm run test:watch    # Vitest en modo observación
npm run lint          # ESLint
npx next typegen && npx tsc --noEmit   # Comprobación de tipos
```

`next typegen` genera los tipos de las rutas de Next.js (por ejemplo, `LayoutProps`) que `tsc` necesita; sin él, la comprobación de tipos falla en un clon recién hecho. `npm run dev` y `npm run build` también los generan.

Cada escenario de las specs tiene una prueba rastreable: buscar `001/US1` (o `002/US3`, etc.) en el código de pruebas muestra cuáles lo cubren. El prefijo es el número de la spec y `USn`, la historia de usuario. Los comandos de cobertura están en [Cobertura de Pruebas](#cobertura-de-pruebas).

### 10. Ejecución con Docker

El backend y el frontend tienen cada uno su `Dockerfile`. El del backend ejecuta `swag init` durante la construcción, así que no requiere el paso 5.

**Backend**, desde `backend/`, con la base de desarrollo levantada:

```bash
docker build -t smye-backend .
docker run --rm --env-file .env -e DB_HOST=host.docker.internal -p 8080:8080 smye-backend
```

`--env-file .env` le pasa al contenedor todas las variables de `backend/.env`, incluida `APP_TIMEZONE`. La imagen `alpine` no trae la base de zonas horarias, pero no hace falta instalarla: el binario la incluye (`time/tzdata`). Dentro de un contenedor, `localhost` es el propio contenedor y no la máquina anfitriona, por eso `DB_HOST` se reemplaza por `host.docker.internal`, que apunta al anfitrión. En Linux hay que agregar `--add-host=host.docker.internal:host-gateway` al `docker run`.

**Frontend**, desde `frontend/`:

```bash
docker build --build-arg NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 -t smye-frontend .
docker run --rm -p 3000:3000 smye-frontend
```

`NEXT_PUBLIC_API_BASE_URL` se pasa como argumento de construcción y no de ejecución porque Next.js incrusta su valor en el código del navegador al compilar: para apuntar a otra API hay que reconstruir la imagen. La URL la usa el navegador, así que debe ser accesible desde la máquina del usuario.

### 11. Detener los servicios

```bash
cd backend/postgres

# Detiene los contenedores y conserva los datos
docker compose -p smye-dev stop
docker compose -p smye-test stop

# Elimina los contenedores y conserva los volúmenes
docker compose -p smye-dev down
docker compose -p smye-test down

# Elimina también los datos (agregar -v); útil para empezar de cero la base de test
docker compose -p smye-test down -v
```

El backend y el frontend se detienen con `Ctrl+C` en su terminal.

### Problemas frecuentes

| Síntoma | Causa y solución |
| --- | --- |
| `no required module provides package .../backend/docs` | Falta generar Swagger: ejecutar el [paso 5](#5-generar-la-documentación-swagger) desde `backend/`. |
| `faltan variables de entorno obligatorias: ...` o `JWT_SECRET debe tener al menos 32 bytes` | Revisar `backend/.env` (o `backend/.env.test` en las pruebas) según el [paso 3](#3-crear-los-archivos-de-entorno). |
| `faltan variables de entorno obligatorias: APP_TIMEZONE` o `APP_TIMEZONE "..." no es una zona horaria válida` | Agregar a `backend/.env` y a `backend/.env.test` la línea `APP_TIMEZONE=America/Argentina/Buenos_Aires`, o corregir el valor por un nombre de zona IANA completo (por ejemplo `America/Argentina/Buenos_Aires` y no `Buenos_Aires`). Conviene respetar las mayúsculas del nombre oficial: en macOS una variante en minúsculas puede aceptarse, pero en Linux y dentro del contenedor se rechaza. |
| Error de autenticación de PostgreSQL al conectar | `DB_PASSWORD` no coincide con `POSTGRES_PASSWORD`. Si se cambió la contraseña después del primer arranque, la base conserva la anterior: recrear el volumen con `docker compose -p <proyecto> down -v` y volver a levantarlo. |
| `port is already allocated` al levantar PostgreSQL | Otro proceso usa el puerto 5432 o 5433: liberarlo o cambiar `POSTGRES_HOST_PORT` y el `DB_PORT` correspondiente. |
| Las pruebas fallan con `no se pudo cargar el entorno de test` | Falta `backend/.env.test`. |
| El frontend no llega a la API (error de red o de CORS) | El backend no está en ejecución, o `NEXT_PUBLIC_API_BASE_URL` del frontend y `CORS_ALLOWED_ORIGIN` del backend no coinciden con las direcciones reales. |

---

## Cobertura de Pruebas

El backend y el frontend exponen cada uno un comando para generar el reporte de cobertura de pruebas. Los reportes son artefactos reproducibles y no se versionan (`.gitignore`).

### Backend

Las pruebas de integración corren siempre contra la base de **test** (`smye_test`, puerto 5433), configurada en `backend/.env.test`; nunca contra la base de desarrollo. Antes de medir la cobertura hay que levantar ese contenedor y tener generada la documentación Swagger ([paso 5](#5-generar-la-documentación-swagger)):

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
