# Software Metrics & Estimation

Software Metrics & Estimation es un sistema diseñado para la estimación, seguimiento, medición y control de proyectos de software. La plataforma centraliza la administración del flujo de trabajo a partir del Product Backlog de cada proyecto, la estimación del volumen de tareas en Story Points, que el equipo puede acordar en sesiones de Planning Poker en tiempo real, el registro de las horas que cada integrante invierte en cada historia y su comparación con el esfuerzo estimado a partir de los Story Points, el registro y seguimiento de los defectos, con el sprint en que se detectó y el sprint en que se resolvió cada uno, y el cálculo automático de las métricas de estimación, seguimiento y calidad (Story Points planificados y completados, velocidad, horas estimadas y reales con su desviación, porcentaje de historias completadas y defectos), por sprint y para todo el proyecto.

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

El sistema se construye por funcionalidades, cada una especificada en su carpeta de [`specs/`](./specs). Hoy están implementadas las nueve primeras:

- **Autenticación y cuentas de usuario** ([`specs/001-user-auth`](./specs/001-user-auth)): registro de cuentas, inicio y cierre de sesión con JWT, protección de las funciones del sistema, consulta del propio perfil y registro de eventos de autenticación.
- **Gestión de proyectos e integrantes** ([`specs/002-project-members`](./specs/002-project-members)): creación y modificación de proyectos con su factor de horas por Story Point, listado de los proyectos propios aislado de los ajenos, alta y baja de integrantes por correo electrónico, consulta del estado del proyecto en una sola pantalla y avance del estado (`planned` → `in_progress` → `finished`) con registro de la fecha de finalización real.
- **Product Backlog** ([`specs/003-product-backlog`](./specs/003-product-backlog)): registro de historias de usuario con título, descripción, prioridad (Alta, Media o Baja) y criterios de aceptación ordenados; listado del backlog ordenado por prioridad y antigüedad, con filtros por prioridad y estado; estimación en Story Points con la escala 0, 1, 2, 3, 5, 8, 13, 20, 40 y 100 (o "sin estimar"); indicación de si cada historia está lista para planificar; refinamiento y eliminación de las historias que nunca se trabajaron. Cualquier integrante del proyecto puede hacer todas estas acciones; cuando el proyecto está finalizado, su backlog queda de solo lectura. El estado de una historia (Pendiente, En progreso, Completada) se muestra pero no se cambia desde el backlog: lo cambia la gestión de sprints.
- **Gestión de sprints** ([`specs/004-sprint-management`](./specs/004-sprint-management)): planificación de sprints con nombre, Sprint Goal y fechas de inicio y fin que no se superponen con las de otro sprint del proyecto; compromiso en el sprint de las historias del backlog listas para planificar, y su retiro; inicio del sprint (como máximo uno Activo por proyecto), con los días restantes o vencidos a la vista; registro del avance de cada historia del sprint Activo (Pendiente → En progreso → Completada, con la posibilidad de desmarcarla mientras el sprint siga Activo); cierre del sprint, que devuelve al backlog las historias no completadas y congela una instantánea con los Story Points comprometidos al inicio, planificados al cierre y completados, las historias involucradas con su resultado y el factor de horas por Story Point vigente; historial de sprints cerrados y, en cada historia, los sprints en los que estuvo; y modificación o eliminación de los sprints Planificados que nunca se iniciaron. Los estados del sprint solo avanzan Planificado → Activo → Cerrado, y el cierre es definitivo. Cualquier integrante del proyecto puede operar sus sprints; ninguna acción sobre los sprints cambia el estado del proyecto, y cuando el proyecto está finalizado sus sprints quedan de solo lectura.
- **Planning Poker en tiempo real** ([`specs/005-planning-poker`](./specs/005-planning-poker)): estimación colaborativa de una historia del backlog, en la que cada integrante del proyecto participa desde su propia sesión y ve los cambios de los demás en menos de 2 segundos, sin recargar la pantalla. Cualquier integrante abre una sesión sobre una historia que no esté Completada (como máximo una sesión abierta por historia) y queda como facilitador; el resto se une desde el detalle de la historia. Cada participante vota en secreto con una carta de la escala (0, 1, 2, 3, 5, 8, 13, 20, 40 o 100) o con "?" para indicar que no sabe, y puede cambiarla pero no retirarla. Los votos se revelan solos cuando votaron todos los participantes conectados, o antes a pedido del facilitador; al revelar se muestran el consenso o la divergencia con el mínimo, el máximo y sus autores. El facilitador inicia rondas nuevas hasta que el equipo acuerda un valor, y al registrarlo la sesión se finaliza y el valor pasa a ser los Story Points de la historia; también puede cancelar la sesión sin registrar nada. Un participante que pierde la conexión deja de figurar como conectado a los 30 segundos, conserva su voto al volver y, si es el facilitador y lleva 5 minutos desconectado, cualquier otro participante conectado puede tomar el rol. El sistema cancela solo las sesiones abiertas cuando su historia pasa a Completada, cuando el proyecto se finaliza o después de 60 minutos sin actividad. Cada historia guarda el historial de sus sesiones con sus rondas y votos, y una historia con historial de estimación ya no se puede eliminar del backlog.
- **Registro de esfuerzo** ([`specs/006-effort-tracking`](./specs/006-effort-tracking)): cada integrante del proyecto registra, sobre cualquier historia del backlog y cualquiera sea su estado o su estimación, las horas que trabajó, en qué fecha y en qué actividad. Las horas van de 0,25 a 24 en múltiplos de 0,25, la actividad tiene entre 1 y 300 caracteres y la fecha no puede ser futura (según `APP_TIMEZONE`) ni anterior al inicio del proyecto; además, una persona no puede superar las 24 horas en un mismo día sumando todos sus proyectos, aun ante pedidos simultáneos. Cada registro queda atribuido a quien lo cargó y anclado al sprint en el que estaba la historia en ese momento (Planificado, Activo o ninguno si estaba solo en el backlog), y ese ancla no cambia nunca más. Después de cerrar un sprint corre un plazo de gracia, de 48 horas por defecto (`EFFORT_GRACE_PERIOD_HOURS`), durante el cual todavía se registra y se corrige esfuerzo sobre sus historias, incluidas las que volvieron al backlog; vencido el plazo, ese esfuerzo queda definitivo. Solo el autor puede corregir la fecha, la actividad o las horas de un registro, o eliminarlo. Cada historia muestra su total de horas reales, el desglose por integrante y la lista de registros, y el proyecto ofrece el listado de su esfuerzo con filtros combinables por integrante, historia, sprint (o sin sprint) y rango de fechas, con su total. La comparación entre lo estimado y lo real calcula, por historia y para todo el proyecto, las horas estimadas (Story Points por el factor de horas por Story Point vigente), las horas reales, la diferencia absoluta y la diferencia porcentual con 2 decimales; una historia sin estimar o estimada en 0 Story Points muestra como "No calculable" lo que no se puede calcular, nunca un 0. Los registros de una persona quitada del proyecto se conservan con su nombre; una historia que alguna vez tuvo esfuerzo registrado ya no se puede eliminar del backlog, aunque después se borren sus registros, ni un sprint Planificado que tiene esfuerzo registrado; y cuando el proyecto está finalizado su esfuerzo queda de solo lectura.
- **Gestión de defectos** ([`specs/007-defect-tracking`](./specs/007-defect-tracking)): cualquier integrante del proyecto registra un defecto con su descripción (de 1 a 2000 caracteres), su severidad (Crítica, Alta, Media o Baja), la historia a la que afecta, en cualquier estado, incluida Completada, y el sprint de detección, que debe estar Activo o Cerrado; si el proyecto tiene un sprint Activo, el formulario lo propone por defecto, y un proyecto que nunca inició un sprint no admite defectos. El defecto nace Abierto y sin sprint de resolución, y avanza con las únicas transiciones Abierto → En progreso, En progreso → Resuelto, Resuelto → Abierto (reapertura), Abierto → Descartado y En progreso → Descartado; Descartado es definitivo. Para resolverlo hay que indicar el sprint de resolución, Activo o Cerrado, que no puede empezar antes que el de detección (el mismo sprint es válido); reabrirlo borra el sprint de resolución y conserva el de detección. La descripción y la severidad se corrigen en cualquier estado salvo Descartado, pero la historia y el sprint de detección solo mientras el defecto está Abierto. El proyecto lista sus defectos de la severidad más alta a la más baja y, a igual severidad, del más reciente al más antiguo, con filtros combinables por estado, severidad, historia y sprint (el filtro por sprint trae los detectados y los resueltos en él), y cada historia muestra los suyos. Los defectos Descartados siguen a la vista en los listados; una historia con defectos, aun todos descartados, ya no se puede eliminar del backlog; los defectos de una persona quitada del proyecto se conservan con su nombre; y cuando el proyecto está finalizado sus defectos quedan de solo lectura. Dos acciones simultáneas e incompatibles sobre el mismo defecto nunca se aplican las dos: la segunda se rechaza con el estado vigente. Esta spec no calcula indicadores: produce los defectos con sus dos anclas de sprint para las métricas.
- **Cálculo de métricas** ([`specs/008-metrics-calculation`](./specs/008-metrics-calculation)): el backend calcula, solo en lectura y a partir de los datos de las specs anteriores, las métricas de cada sprint y del proyecto completo: Story Points planificados y completados, horas estimadas (Story Points por el factor de horas por Story Point), horas reales, desviación en horas y en porcentaje, porcentaje de historias completadas y defectos detectados y resueltos. Para el proyecto agrega la velocidad (promedio de los Story Points completados en los últimos 3 sprints Cerrados), los acumulados, las seis series por sprint que graficará el dashboard, el esfuerzo fuera de sprint, las historias sin estimar y el desglose de defectos por severidad. Es la única fuente de las fórmulas: el dashboard y los reportes mostrarán estos valores sin recalcular. Cada métrica indica si tiene valor y si ese valor todavía puede cambiar (ver [Métricas: estados y definitividad](#métricas-estados-y-definitividad)). Solo los integrantes del proyecto pueden consultarlas, incluso con el proyecto finalizado.
- **Dashboard del proyecto** ([`specs/009-project-dashboard`](./specs/009-project-dashboard)): pantalla de solo lectura en `/projects/<projectId>/dashboard`, a la que acceden los integrantes del proyecto (también finalizado), que muestra sin recalcular los valores del cálculo de métricas. De arriba hacia abajo presenta el resumen del proyecto (nombre, estado, fechas e integrantes), el sprint Activo con sus días restantes o vencidos, las advertencias (avisos de planificación y datos inconsistentes, que solo aparecen si existen), los indicadores clave del sprint activo y del proyecto, los gráficos por sprint de velocidad, esfuerzo estimado contra real y defectos detectados contra resueltos, cada uno con su tabla de datos equivalente, y los defectos abiertos por severidad. Cada valor se muestra como número, **No calculable** (con su explicación) o **No aplica**, y las métricas de sprint llevan la marca **Parcial** o **Provisoria** cuando todavía pueden cambiar. Cada sección carga su información por separado: si una falla muestra solo su mensaje de error y las demás siguen visibles; un proyecto sin sprints muestra "Sin datos todavía", y si falla el resumen se ofrece **Reintentar**. Esta spec no agrega endpoints propios ni cambia datos: consume los de las specs anteriores. El detalle de las secciones está en [Panel de Control y Reportes](#panel-de-control-y-reportes).

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

- **Panel de Control (Dashboard):** Muestra de forma visual el estado del proyecto e indicadores clave del progreso. Se abre en `/projects/<projectId>/dashboard`, es de solo lectura (solo consulta datos, no modifica nada) y no recalcula nada: muestra los valores que entrega el cálculo de métricas. Sus secciones, de arriba hacia abajo, son:
  - **Resumen del proyecto:** nombre, estado, fechas (inicio, finalización prevista y, si el proyecto está finalizado, la real) e integrantes en solo lectura.
  - **Sprint activo:** nombre, Sprint Goal, fechas y días restantes o vencidos del sprint Activo, o el aviso de que no hay ninguno.
  - **Advertencias:** avisos de planificación (por ejemplo, historias del backlog sin estimar) y, aparte, datos inconsistentes que alguien debe revisar. Si no hay advertencias, la sección no aparece.
  - **Indicadores clave:** los del sprint activo y los del proyecto (Story Points, horas estimadas y reales, desviación, porcentaje de historias completadas, defectos y velocidad).
  - **Gráficos por sprint:** velocidad, esfuerzo (estimado contra real) y defectos (detectados contra resueltos), cada uno con su tabla de datos equivalente.
  - **Defectos abiertos por severidad:** distribución de los defectos que siguen abiertos.

  Cada valor se muestra con uno de estos estados: el número, **No calculable** (con la explicación de por qué, por ejemplo un cociente con denominador 0) o **No aplica** (la métrica no tiene sentido para el estado del sprint). Además, las métricas de sprint llevan la marca **Parcial** (sprint Activo o acumulado que lo incluye) o **Provisoria** (sprint Cerrado cuyo valor todavía puede cambiar); los valores definitivos no llevan marca. El significado de cada estado está en [Métricas: estados y definitividad](#métricas-estados-y-definitividad).

  **Degradación por sección:** cada sección consulta su propia información, así que si una falla muestra solo su mensaje de error (por ejemplo, "No se pudo cargar el gráfico de velocidad.") y las demás siguen visibles. Un proyecto sin sprints muestra el estado vacío "Sin datos todavía" en lugar de las secciones de métricas. Si falla el resumen del proyecto, la pantalla muestra "No se pudo mostrar el dashboard." con el botón **Reintentar** (no se ofrece cuando el proyecto no existe).
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
| Backend | Go 1.27.3, Gin, GORM, JWT (`golang-jwt/jwt/v5`), bcrypt |
| Base de datos | PostgreSQL 18.6 (imagen `postgres:18.6-alpine`) |
| Frontend | Node.js 24.21.0, Next.js 16.4.2, React 19.3.0, Tailwind CSS 4.3.3, TypeScript 5.9.3, Recharts 3.10.1 (gráficos del dashboard) |
| Documentación de la API | Swagger generado con `swag` 1.16.6 |
| Pruebas | `go test` con testify en el backend; Vitest 5.0.3 con Testing Library en el frontend |
| Contenedores | Docker y Docker Compose |

Los entornos de **desarrollo** y de **test** son disjuntos: cada uno tiene su propio contenedor de PostgreSQL, su propia base, su propio puerto y sus propios archivos de variables de entorno. Las pruebas nunca tocan la base de desarrollo.

| Entorno | Contenedor | Base | Puerto del host | Archivos de entorno |
| --- | --- | --- | --- | --- |
| Desarrollo | `smye-dev-postgres-1` | `smye_dev` | 5432 | `backend/postgres/.env`, `backend/.env`, `frontend/.env.local` |
| Test | `smye-test-postgres-1` | `smye_test` | 5433 | `backend/postgres/.env.test`, `backend/.env.test`, `frontend/.env.test` |

**Tiempo real del Planning Poker.** El backend difunde los cambios de cada sesión de estimación con Server-Sent Events (SSE): el navegador abre un canal `text/event-stream` con `fetch` y lo lee a medida que llegan los eventos, mientras que cada acción (votar, revelar, iniciar una ronda, registrar, cancelar o tomar el rol de facilitador) es un endpoint REST común. No usa WebSockets ni agrega dependencias, servicios o variables de entorno. Un barrido interno del backend corre cada segundo y se encarga de las desconexiones, las cancelaciones automáticas, las bajas de integrantes y los cambios de la historia de las sesiones abiertas.

**Supuesto de una sola instancia del backend.** El registro de las conexiones abiertas a cada sesión vive en la memoria del proceso del backend y no se comparte entre procesos. Por eso el backend debe ejecutarse en **una sola instancia** (un único `go run ./cmd/app` o un único contenedor), que alcanza para el objetivo de 10 sesiones en paralelo con 10 participantes cada una. Con dos o más réplicas detrás de un balanceador, cada una entregaría los cambios solo a los canales abiertos en ella y llevaría su propia cuenta de quién está conectado: participantes de una misma sesión conectados a réplicas distintas no verían en tiempo real los cambios hechos a través de otra réplica, y la presencia, de la que dependen la revelación automática y el relevo del facilitador, dejaría de ser confiable (por ejemplo, el arranque de una réplica marcaría como desconectados a quienes siguen conectados a otra). Escalar a varias instancias exigiría un canal compartido entre procesos y queda fuera del alcance actual. Al arrancar, el backend marca como desconectados a los participantes de las sesiones abiertas, porque ninguna conexión sobrevive a un reinicio; las sesiones siguen abiertas y cada persona se vuelve a unir al reconectar.

---

## Puesta en Marcha

Los comandos están escritos para una terminal tipo bash (Linux, macOS o Git Bash en Windows). Cuando un paso cambia en PowerShell, se indica la variante. Todas las rutas son relativas a la raíz del repositorio.

### 1. Prerrequisitos

| Herramienta | Versión | Para qué |
| --- | --- | --- |
| Git | cualquiera reciente | Obtener el código |
| Go | 1.27.3 | Compilar y probar el backend |
| Node.js | 24.21.0 (incluye npm 11) | Ejecutar y probar el frontend |
| Docker | 29.x | Ejecutar PostgreSQL y las imágenes de la aplicación |
| Docker Compose | v5.x (plugin `docker compose`) | Levantar las bases de datos |

Con Go 1.21 o posterior instalado alcanza: si la versión local es anterior a la 1.27.3 que declara `backend/go.mod`, Go descarga automáticamente la versión correcta la primera vez que se ejecuta un comando dentro de `backend/` (comportamiento por defecto, `GOTOOLCHAIN=auto`). Se puede comprobar con `go version` dentro de `backend/`.

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
| `backend/.env` y `backend/.env.test` | `EFFORT_GRACE_PERIOD_HOURS` | Opcional, por defecto `48`: plazo de gracia, en horas enteras (`≥ 1`), para registrar y corregir esfuerzo después de cerrar un sprint. Si el valor no es un entero mayor o igual que 1, el backend no arranca |

Los archivos del frontend no necesitan cambios: `NEXT_PUBLIC_API_BASE_URL=http://localhost:8080` apunta al backend local. El resto de las claves (`DB_HOST=localhost`, `DB_USER=smye`, `SERVER_PORT=8080`, `CORS_ALLOWED_ORIGIN=http://localhost:3000`, `APP_TIMEZONE=America/Argentina/Buenos_Aires`) sirven tal como vienen en las plantillas.

**`APP_TIMEZONE`** es la zona horaria única del sistema, en formato IANA (por ejemplo `America/Argentina/Buenos_Aires`, `America/Montevideo` o `UTC`). El backend la usa para fijar fechas de calendario que no dependen del dispositivo de quien opera: cuando un proyecto pasa a `finished`, su fecha de finalización real es la fecha del cambio en esta zona, y lo mismo vale para las fechas reales de inicio y de cierre de un sprint y para el cálculo de sus días restantes. No es un secreto, pero vive en `backend/.env` y `backend/.env.test` como el resto de la configuración, y debe estar en los dos. Si se crearon los archivos de entorno antes de la gestión de proyectos (spec 002), hay que agregarles la línea:

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

Debe responder `smye_test`. No hace falta crear tablas: el backend crea y migra el esquema automáticamente al conectarse (`AutoMigrate` de GORM), incluidas las tablas `projects` y `project_members` de la gestión de proyectos, las tablas `user_stories` y `acceptance_criteria` del Product Backlog, las tablas `sprints` y `sprint_stories` de la gestión de sprints, las tablas `estimation_sessions`, `estimation_rounds`, `estimation_participants` y `estimation_votes` del Planning Poker, y la tabla `defects` de la gestión de defectos.

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
| `http://localhost:3000/projects/<projectId>` | Estado del proyecto e integrantes en una sola pantalla, con el sprint activo, el reparto del backlog por estado y los enlaces **Ver backlog**, **Ver sprints**, **Esfuerzo** y **Defectos**; el propietario ve además los correos, el alta y la baja de integrantes y el botón para avanzar el estado (sin sesión redirige a `/login`; un proyecto inexistente o ajeno muestra "El proyecto no existe.") |
| `http://localhost:3000/projects/<projectId>/edit` | Modificación de nombre, descripción, fechas y horas por Story Point; solo el propietario ve el formulario y el resto de los integrantes ve "Solo el propietario del proyecto puede realizar esta acción." (sin sesión redirige a `/login`; un proyecto inexistente o ajeno muestra "El proyecto no existe.") |
| `http://localhost:3000/projects/<projectId>/backlog` | Backlog del proyecto ordenado por prioridad y antigüedad, con filtros por prioridad y estado, la marca de las historias listas para planificar y el acceso a **Nueva historia** (sin sesión redirige a `/login`) |
| `http://localhost:3000/projects/<projectId>/backlog/new` | Formulario de alta de una historia: título, descripción, prioridad, Story Points y criterios de aceptación (sin sesión redirige a `/login`) |
| `http://localhost:3000/projects/<projectId>/backlog/<storyId>` | Detalle de la historia con sus criterios en orden, autor y fechas, y la sección "Sprints en los que estuvo" con el resultado en cada uno; desde ahí se estima en Story Points y se elimina con confirmación. La sección "Planning Poker" ofrece **Abrir Planning Poker**, muestra la "Sesión en curso" con su facilitador, su ronda y el enlace **Unirse**, y lista el historial de sesiones cerradas (estado, valor acordado, motivo de una cancelación automática, cantidad de rondas, quién la abrió y cuándo cerró) con el enlace **Ver sesión**, o indica "La historia no tiene sesiones de estimación.". La sección "Esfuerzo" muestra el total de horas reales, el desglose por integrante, la comparación entre estimado y real (Story Points, horas estimadas, horas reales, diferencia y diferencia porcentual, con "Sin estimar" y "No calculable" donde corresponde) y la lista de registros con fecha, autor, actividad, horas y sprint ("Sin sprint" si no tiene), o "Todavía no hay esfuerzo registrado."; en los registros propios que todavía no son definitivos ofrece **Editar** (con **Guardar cambios**) y **Eliminar** (con **Confirmar eliminación**), y al final el formulario con **Fecha** (precargada con la de hoy), **Actividad** y **Horas** para **Registrar esfuerzo**. La sección "Defectos" lista los defectos de la historia (descripción, severidad, estado y sprints) o indica "Todavía no hay defectos registrados.", y ofrece **Registrar defecto** con la historia ya elegida (sin sesión redirige a `/login`; una historia inexistente o de un proyecto ajeno muestra "La historia no existe.") |
| `http://localhost:3000/projects/<projectId>/backlog/<storyId>/edit` | Refinamiento de título, descripción, prioridad y criterios de aceptación, que se pueden agregar, quitar y reordenar (sin sesión redirige a `/login`; una historia inexistente o de un proyecto ajeno muestra "La historia no existe.") |
| `http://localhost:3000/projects/<projectId>/backlog/<storyId>/poker/<sessionId>` | Sala de Planning Poker. Si la sesión está abierta, muestra en vivo la historia con sus criterios de aceptación, el mazo de cartas para **Tu voto** (la carta propia queda resaltada), la ronda vigente con los votos ocultos hasta la revelación y después el resultado (consenso o divergencia, con el mínimo y el máximo), los participantes con su marca de facilitador, si ya votaron y si están conectados, y para el facilitador las acciones **Revelar**, **Nueva ronda**, **Registrar estimación** y **Cancelar sesión** (con confirmación); mientras el facilitador está desconectado se muestra cuánto falta para que el rol quede disponible y, cumplidos los 5 minutos, aparece **Tomar el rol**. Las rondas anteriores se listan con sus votos y su resultado. Si la sesión está cerrada, muestra su estado (Finalizada o Cancelada), el valor acordado o el motivo de la cancelación automática, y todas sus rondas y votos. Ofrece **Volver a la historia** (sin sesión redirige a `/login`) |
| `http://localhost:3000/projects/<projectId>/effort` | Esfuerzo del proyecto: los filtros **Integrante**, **Historia**, **Sprint** (con la opción **Sin sprint**), **Desde** y **Hasta**, que se combinan y se aplican al cambiar cada uno; la lista de registros que los cumplen, con su total, o "Todavía no hay esfuerzo registrado en el proyecto." y "Ningún registro coincide con los filtros." según el caso; la sección "Estimado contra real", con una fila por historia del proyecto (Story Points, horas estimadas, horas reales, diferencia y diferencia porcentual), y el enlace **Volver al proyecto** (sin sesión redirige a `/login`) |
| `http://localhost:3000/projects/<projectId>/sprints` | Sprints del proyecto con su estado, en dos pestañas: **Todos** y **Cerrados** (`?view=closed`), esta última con el historial de sprints cerrados y sus totales congelados; ofrece el acceso a **Nuevo sprint** y el regreso al proyecto (sin sesión redirige a `/login`) |
| `http://localhost:3000/projects/<projectId>/sprints/new` | Formulario de creación de un sprint: nombre, Sprint Goal, fecha de inicio y fecha de fin (sin sesión redirige a `/login`) |
| `http://localhost:3000/projects/<projectId>/sprints/<sprintId>` | Detalle del sprint con su estado, Sprint Goal, fechas y, si está Activo, los días restantes o vencidos; las acciones **Iniciar sprint** y **Cerrar sprint** (con confirmación), **Editar sprint** y **Eliminar sprint** mientras está Planificado, las historias comprometidas con el control de su avance y **Quitar del sprint**, el diálogo **Comprometer historias del backlog** mientras el sprint está abierto y el resultado congelado cuando está cerrado (sin sesión redirige a `/login`; un sprint inexistente, eliminado o de un proyecto ajeno muestra "El sprint no existe.") |
| `http://localhost:3000/projects/<projectId>/sprints/<sprintId>/edit` | Modificación de nombre, Sprint Goal y fechas de un sprint Planificado; un sprint Activo muestra "Solo se puede modificar un sprint Planificado." y uno Cerrado, "El sprint está cerrado y no admite cambios." (sin sesión redirige a `/login`; un sprint inexistente o de un proyecto ajeno muestra "El sprint no existe.") |
| `http://localhost:3000/projects/<projectId>/defects` | Defectos del proyecto, de la severidad más alta a la más baja, con su descripción, severidad, estado, historia y sprints de detección y de resolución; los filtros **Estado**, **Severidad**, **Historia** y **Sprint** se combinan, y la lista indica "Todavía no hay defectos registrados." o "Ningún defecto cumple los filtros elegidos." según el caso; ofrece **Registrar defecto** y **Volver al proyecto** (sin sesión redirige a `/login`) |
| `http://localhost:3000/projects/<projectId>/defects/new` | Formulario de registro de un defecto: **Descripción**, **Severidad**, **Historia** y **Sprint de detección**, con el sprint Activo propuesto por defecto; admite `?storyId=<storyId>` para preseleccionar la historia (sin sesión redirige a `/login`) |
| `http://localhost:3000/projects/<projectId>/defects/<defectId>` | Detalle del defecto con su descripción, severidad, estado, historia, sprint de detección, sprint de resolución, autor y fecha del último cambio de estado; ofrece las acciones **Pasar a En progreso**, **Resolver** (con **Sprint de resolución**), **Reabrir** y **Descartar** (con confirmación) según el estado, y **Corregir datos** con **Guardar cambios**; un defecto Descartado o de un proyecto finalizado se muestra sin acciones (sin sesión redirige a `/login`; un defecto inexistente o de un proyecto ajeno muestra solo el aviso de que no existe) |
| `http://localhost:3000/projects/<projectId>/dashboard` | Dashboard del proyecto: resumen, sprint activo, advertencias, indicadores clave, gráficos de velocidad, esfuerzo y defectos por sprint, y defectos abiertos por severidad, con las marcas **Parcial** y **Provisoria** donde corresponde; un proyecto sin sprints muestra "Sin datos todavía", y una sección que falla muestra su propio mensaje sin ocultar las demás (sin sesión redirige a `/login`; un proyecto inexistente o ajeno muestra "No se pudo mostrar el dashboard.") |
| <http://localhost:8080/swagger/index.html> | Documentación interactiva de la API |

Recorrido mínimo: crear una cuenta en `/register`, iniciar sesión en `/login`, ver el perfil en `/profile` y cerrar sesión desde ahí.

Recorrido de la gestión de proyectos, con el backend y el frontend en ejecución:

1. Crear dos cuentas en `/register` (por ejemplo, Ana y Bruno). Ana será la propietaria y Bruno el integrante.
2. Iniciar sesión como Ana. Desde `/profile`, el enlace **Mis proyectos** lleva a `/projects`, que la primera vez muestra la lista vacía con un mensaje, no un error.
3. Abrir `/projects/new` y crear un proyecto con nombre, descripción opcional, fecha de inicio, fecha de finalización prevista y horas por Story Point (entre 0,01 y 40, con hasta dos decimales). El proyecto nace en estado **Planificado**, con Ana como propietaria y única integrante, y aparece en `/projects`.
4. Obtener el identificador del proyecto (`projectId`). El listado de la interfaz todavía no enlaza al detalle, así que el identificador se toma de la respuesta de `GET /api/v1/projects` (campo `id`), por ejemplo desde Swagger: en <http://localhost:8080/swagger/index.html>, ejecutar `POST /api/v1/auth/login` con las credenciales de Ana, copiar el token en **Authorize** como `Bearer <token>` y ejecutar `GET /api/v1/projects`.
5. Abrir `/projects/<projectId>`: muestra el estado, las fechas, el factor, los integrantes, el sprint activo (ninguno mientras no se inicie un sprint) y el resumen del backlog. Desde ahí Ana agrega a Bruno escribiendo su correo electrónico; solo se pueden agregar cuentas ya registradas.
6. Abrir `/projects/<projectId>/edit` para modificar los datos del proyecto. Solo la propietaria ve el formulario.
7. En `/projects/<projectId>`, usar **Pasar a En curso** y después **Finalizar proyecto**. Al finalizar se registra la fecha de finalización real con la fecha del día en la zona de `APP_TIMEZONE`, y el proyecto queda de solo lectura: ya no admite modificaciones, cambios de integrantes ni cambios de estado.
8. Iniciar sesión como Bruno: ve el proyecto en `/projects` y su detalle, pero no los correos de los integrantes ni las acciones de administración. Si Ana lo quita del proyecto antes de finalizarlo, para Bruno el proyecto pasa a comportarse como inexistente.

Recorrido del Product Backlog, con un proyecto que todavía no está finalizado y una sesión de cualquiera de sus integrantes:

1. Abrir `/projects/<projectId>` y seguir el enlace **Ver backlog**, que lleva a `/projects/<projectId>/backlog`. La primera vez muestra "El backlog todavía no tiene historias.".
2. Usar **Nueva historia** para registrar una historia con título, prioridad, Story Points y dos criterios de aceptación. Solo el título y la prioridad son obligatorios; una historia sin Story Points queda "sin estimar". La pantalla confirma "Historia creada" con su estado Pendiente; al volver a `/projects/<projectId>/backlog`, la historia aparece marcada como lista para planificar, porque tiene Story Points y al menos un criterio.
3. En el backlog, filtrar por prioridad y por estado, y quitar los filtros. Si ninguna historia coincide se ve "Ninguna historia coincide con los filtros", distinto del mensaje de backlog vacío.
4. Hacer clic en una historia para abrir su detalle en `/projects/<projectId>/backlog/<storyId>`. Desde ahí se la estima eligiendo un valor de la escala (0, 1, 2, 3, 5, 8, 13, 20, 40 o 100) o se la devuelve a "sin estimar".
5. Para refinarla, agregar `/edit` a la dirección del detalle (`/projects/<projectId>/backlog/<storyId>/edit`): se pueden cambiar el título, la descripción y la prioridad, y agregar, quitar o reordenar criterios. El estado y los Story Points no se cambian desde este formulario.
6. Volver al detalle y usar **Eliminar historia**; la historia se elimina recién al confirmar, y la pantalla regresa al backlog. Solo se pueden eliminar historias que nunca se trabajaron.
7. Si el proyecto pasa a **Finalizado**, el backlog y los detalles se siguen pudiendo consultar, pero crear, refinar, estimar y eliminar se rechazan con "El proyecto está finalizado y su backlog es de solo lectura.".

Recorrido de la gestión de sprints, con un proyecto que todavía no está finalizado, al menos dos historias listas para planificar en su backlog y una sesión de cualquiera de sus integrantes:

1. Abrir `/projects/<projectId>` y seguir el enlace **Ver sprints**, que lleva a `/projects/<projectId>/sprints`. La primera vez muestra la lista vacía con un mensaje, no un error.
2. Usar **Nuevo sprint** para crear un sprint con nombre, Sprint Goal (hasta 500 caracteres) y fechas de inicio y fin; la fecha de fin debe ser posterior a la de inicio, y el período no puede superponerse con el de otro sprint del proyecto (compartir el día de borde no es superposición). El sprint nace **Planificado**.
3. Abrir el sprint desde la lista para ver su detalle en `/projects/<projectId>/sprints/<sprintId>`. Con **Comprometer historias del backlog**, elegir historias listas para planificar y confirmarlas con **Comprometer historias**; cada una se puede retirar con **Quitar del sprint**, y vuelve al backlog como Pendiente.
4. Mientras está Planificado, **Editar sprint** lleva a `/projects/<projectId>/sprints/<sprintId>/edit` para corregir nombre, Sprint Goal y fechas, y **Eliminar sprint** lo descarta tras confirmar.
5. Usar **Iniciar sprint**: el sprint pasa a **Activo** y el detalle muestra los días restantes, con el día de fin incluido, o cuántos días lleva vencido. Solo puede haber un sprint Activo por proyecto, e iniciarlo no cambia el estado del proyecto. La vista del proyecto muestra ahora ese sprint como activo.
6. En el detalle del sprint Activo, mover una historia con **Pasar a En progreso** y después **Marcar como completada**; mientras el sprint siga Activo, **Volver a En progreso** la desmarca.
7. Usar **Cerrar sprint** y confirmar: las historias no completadas vuelven al backlog como Pendientes y el detalle muestra la instantánea congelada (Story Points comprometidos al inicio, planificados al cierre y completados, historias planificadas y completadas, factor al cierre y fecha real de cierre). Un sprint cerrado no se reabre ni se modifica.
8. En `/projects/<projectId>/sprints`, la pestaña **Cerrados** muestra el historial de sprints cerrados con sus totales. En el detalle de una historia del backlog, la sección "Sprints en los que estuvo" muestra cada sprint y su resultado.
9. Si el proyecto pasa a **Finalizado**, los sprints se siguen pudiendo consultar, pero toda modificación se rechaza con "El proyecto está finalizado y sus sprints son de solo lectura.".

Recorrido del Planning Poker, con un proyecto que todavía no está finalizado, una historia de su backlog que no está Completada y tres integrantes (por ejemplo, Ana, Bruno y Carla), cada uno con su sesión iniciada en su propia pestaña o navegador (la sesión de la aplicación se guarda por pestaña, así que cada pestaña nueva pide iniciar sesión):

1. Ana abre el detalle de la historia en `/projects/<projectId>/backlog/<storyId>` y, en la sección "Planning Poker", pulsa **Abrir Planning Poker**. La aplicación la lleva a la sala en `/projects/<projectId>/backlog/<storyId>/poker/<sessionId>`, con la ronda 1 en votación y Ana como facilitadora.
2. Bruno y Carla abren el mismo detalle, ven "Sesión en curso" y pulsan **Unirse**. Las tres salas muestran a los tres participantes conectados, sin recargar. Si alguien intenta abrir otra sesión sobre la misma historia, ve el rechazo y el enlace **Unirse** a la que ya está abierta.
3. Cada uno elige una carta en **Tu voto**: la propia queda resaltada y los demás solo ven que ya votó. El voto se puede cambiar por otra carta hasta la revelación, pero no retirar.
4. Al votar el último participante conectado, la ronda se revela sola y se ven todas las cartas, con el consenso o la divergencia y el mínimo y el máximo. Ana también puede usar **Revelar** antes, si hay al menos un voto.
5. Si hay divergencia, Ana usa **Nueva ronda** y el equipo vuelve a votar; la ronda anterior queda a la vista con sus votos.
6. Ana elige el valor acordado en **Registrar estimación** y lo confirma: las tres salas muestran la sesión Finalizada y el detalle de la historia muestra los nuevos Story Points. Con **Cancelar sesión** y su confirmación, la sesión se cierra sin cambiar la historia.
7. De vuelta en el detalle, la sección "Planning Poker" lista la sesión con su valor y su cantidad de rondas; **Ver sesión** abre su resultado con todas las rondas y los votos. A partir de ahí la historia ya no se puede eliminar del backlog.
8. Para ver el relevo del facilitador, Ana cierra su pestaña con una sesión abierta: a los 30 segundos figura desconectada en las salas de Bruno y Carla, que ven cuánto falta para que el rol quede disponible, y a los 5 minutos cualquiera de los dos puede usar **Tomar el rol**.

Recorrido del registro de esfuerzo, con un proyecto En curso cuya fecha de inicio es anterior a hoy, al menos dos historias en su backlog y dos integrantes (por ejemplo, Ana y Bruno). Para ver vencer el plazo de gracia sin esperar 48 horas, se puede poner `EFFORT_GRACE_PERIOD_HOURS=1` en `backend/.env` y reiniciar el backend:

1. Ana compromete una historia en un sprint y lo inicia. En el detalle de esa historia (`/projects/<projectId>/backlog/<storyId>`), en la sección "Esfuerzo", escribe una actividad y 6,5 horas, deja la fecha de hoy y pulsa **Registrar esfuerzo**. La pantalla confirma "Se registraron 6,5 h el …" y el registro aparece en la lista con el nombre del sprint Activo; el total y el desglose de Ana suben a 6,5 h.
2. Ana registra horas en otra historia que está solo en el backlog: el registro figura como "Sin sprint".
3. Ana prueba valores inválidos (1,3 horas, 0 horas, 25 horas, una actividad de solo espacios o la fecha de mañana): la pantalla indica qué campo corregir y por qué, y no se crea nada. Si con un registro nuevo superaría las 24 horas del día sumando todos sus proyectos, el rechazo indica cuántas horas le quedan disponibles para esa fecha.
4. Bruno abre la misma historia: ve el total, la línea de Ana en el desglose y sus registros, pero no los botones **Editar** ni **Eliminar**, que solo aparecen en los registros propios.
5. Ana usa **Editar** en su registro, cambia las horas a 4 y pulsa **Guardar cambios**: el total baja y el sprint del registro no cambia. Con **Eliminar** y **Confirmar eliminación**, el registro deja de contar en los totales.
6. Ana cierra el sprint dejando la historia sin completar. Mientras dura el plazo de gracia, todavía puede registrar y corregir horas en esa historia, y los registros nuevos quedan anclados al sprint Cerrado. Vencido el plazo, los registros de ese sprint quedan definitivos (ya no ofrecen **Editar** ni **Eliminar**) y un registro nuevo sobre la historia, que volvió al backlog, queda "Sin sprint".
7. Desde `/projects/<projectId>`, el enlace **Esfuerzo** lleva a `/projects/<projectId>/effort`. Ahí se filtra por integrante, por historia, por sprint o "Sin sprint" y por un rango de fechas; los filtros se combinan, el total corresponde a lo filtrado y, sin coincidencias, la lista queda vacía con total 0, sin error.
8. En la sección "Estimado contra real" de esa misma pantalla, cada historia muestra sus horas estimadas (Story Points por el factor del proyecto), sus horas reales y las diferencias. Una historia sin estimar muestra "No calculable" en las horas estimadas y en las diferencias; una estimada en 0 Story Points muestra 0 h estimadas y la diferencia porcentual "No calculable". Si Ana cambia el factor de horas por Story Point en `/projects/<projectId>/edit`, las horas estimadas se recalculan y las reales no cambian.
9. Si Ana quita a Bruno del proyecto, los registros de Bruno siguen a la vista con su nombre y en los totales. Si Ana finaliza el proyecto, el esfuerzo se sigue pudiendo consultar, pero registrar, corregir y eliminar se rechazan con "El proyecto está finalizado y su esfuerzo es de solo lectura.".

Recorrido de la gestión de defectos, con un proyecto que todavía no está finalizado, al menos una historia en su backlog, un sprint Activo y, para elegir otro sprint de detección o de resolución, un sprint Cerrado, y dos integrantes (por ejemplo, Ana y Bruno):

1. Ana abre el detalle de la historia en `/projects/<projectId>/backlog/<storyId>`: en la sección "Defectos" ve "Todavía no hay defectos registrados." y pulsa **Registrar defecto**. El formulario llega con la historia elegida y con el sprint Activo propuesto como **Sprint de detección**, que se puede cambiar por uno Cerrado. Un sprint Planificado no se acepta, y en un proyecto que nunca inició un sprint el registro se rechaza porque no hay sprint de detección disponible.
2. Ana escribe una descripción, elige la severidad Alta y pulsa **Registrar defecto**. El defecto nace **Abierto** y sin sprint de resolución. Si prueba una descripción vacía, una de más de 2000 caracteres o una severidad inválida, la pantalla indica qué campo corregir y no crea nada.
3. En `/projects/<projectId>/defects` (enlace **Defectos** del proyecto) ve el defecto. Registra otros de distinta severidad y usa los filtros **Estado**, **Severidad**, **Historia** y **Sprint**: se combinan, el filtro por sprint trae los detectados y los resueltos en él, y sin coincidencias se ve "Ningún defecto cumple los filtros elegidos.", distinto del mensaje de proyecto sin defectos.
4. Ana abre el detalle del defecto. Con **Corregir datos** y **Guardar cambios** cambia la descripción y la severidad; mientras el defecto esté Abierto también puede corregir la historia y el sprint de detección. Con **Pasar a En progreso**, esos dos vínculos quedan fijos y la descripción y la severidad se siguen pudiendo corregir.
5. Bruno abre el mismo defecto y lo trabaja: cualquier integrante puede cambiar el estado de cualquier defecto. Con **Resolver** elige el **Sprint de resolución** (Activo o Cerrado, no anterior al de detección; el mismo sprint es válido) y el defecto queda **Resuelto**. Resolver sin sprint de resolución o con uno anterior al de detección se rechaza y el defecto no cambia. Un defecto Abierto no pasa directo a Resuelto, y uno En progreso no vuelve a Abierto.
6. Si el defecto reaparece, **Reabrir** lo devuelve a Abierto, borra el sprint de resolución y conserva el de detección; puede volver a resolverse, y vale siempre el sprint de la última resolución.
7. Para un duplicado, **Descartar** (con confirmación) lo deja **Descartado** desde Abierto o En progreso, no desde Resuelto. Un defecto Descartado sigue en los listados con su estado, no admite más cambios y no tiene sprint de resolución. La historia con defectos, aunque todos estén descartados, ya no se puede eliminar del backlog.
8. Si Ana finaliza el proyecto, los defectos se siguen pudiendo consultar, pero registrar, corregir, cambiar de estado, reabrir y descartar se rechazan con "El proyecto está finalizado y sus defectos son de solo lectura.".

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
| `POST` | `/api/v1/projects/{projectId}/stories` | Creación de una historia en el backlog del proyecto (cualquier integrante) |
| `GET` | `/api/v1/projects/{projectId}/stories` | Listado del backlog ordenado por prioridad y antigüedad; admite los filtros opcionales `priority` (`high`, `medium`, `low`) y `status` (`pending`, `in_progress`, `completed`) |
| `GET` | `/api/v1/stories/{storyId}` | Detalle de una historia con sus criterios de aceptación (cualquier integrante) |
| `PATCH` | `/api/v1/stories/{storyId}` | Refinamiento de título, descripción, prioridad o criterios de aceptación; solo cambian los campos enviados |
| `DELETE` | `/api/v1/stories/{storyId}` | Eliminación de una historia que nunca se trabajó (cualquier integrante) |
| `PUT` | `/api/v1/stories/{storyId}/story-points` | Estimación en Story Points con la escala 0, 1, 2, 3, 5, 8, 13, 20, 40 y 100, o `null` para dejarla sin estimar |
| `POST` | `/api/v1/projects/{projectId}/sprints` | Creación de un sprint Planificado en el proyecto (cualquier integrante) |
| `GET` | `/api/v1/projects/{projectId}/sprints` | Listado de los sprints del proyecto por fecha de inicio descendente; admite el filtro opcional y repetible `status` (`planned`, `active`, `closed`), y con `?status=closed` devuelve el historial de cerrados con sus totales congelados |
| `GET` | `/api/v1/sprints/{sprintId}` | Detalle de un sprint con sus historias comprometidas y, si está cerrado, su instantánea (cualquier integrante) |
| `PUT` | `/api/v1/sprints/{sprintId}` | Modificación de nombre, Sprint Goal y fechas de un sprint Planificado |
| `DELETE` | `/api/v1/sprints/{sprintId}` | Eliminación de un sprint Planificado que nunca fue iniciado |
| `POST` | `/api/v1/sprints/{sprintId}/stories` | Compromiso de historias del backlog en un sprint Planificado o Activo |
| `DELETE` | `/api/v1/sprints/{sprintId}/stories` | Retiro de historias del sprint, que vuelven al backlog como Pendientes |
| `POST` | `/api/v1/sprints/{sprintId}/start` | Inicio de un sprint Planificado; como máximo un sprint Activo por proyecto |
| `POST` | `/api/v1/sprints/{sprintId}/close` | Cierre del sprint Activo: devuelve al backlog las historias no completadas y congela su instantánea |
| `PUT` | `/api/v1/stories/{storyId}/status` | Registro del avance de una historia del sprint Activo (`pending`, `in_progress`, `completed`) |
| `GET` | `/api/v1/stories/{storyId}/sprints` | Sprints en los que estuvo una historia y su resultado en cada uno |
| `POST` | `/api/v1/stories/{storyId}/estimation-sessions` | Apertura de una sesión de Planning Poker sobre la historia; quien la abre queda como facilitador (cualquier integrante) |
| `GET` | `/api/v1/stories/{storyId}/estimation-sessions` | Historial de estimación de la historia: sesiones cerradas de la más reciente a la más antigua y, si la hay, la sesión en curso sin votos |
| `GET` | `/api/v1/estimation-sessions/{sessionId}` | Consulta de una sesión con todas sus rondas; de la ronda en votación solo se ve el voto propio |
| `GET` | `/api/v1/estimation-sessions/{sessionId}/events` | Canal de eventos en tiempo real (`text/event-stream`); abrirlo une a quien pide como participante |
| `PUT` | `/api/v1/estimation-sessions/{sessionId}/vote` | Emisión o cambio del voto en la ronda vigente, con una carta de la escala o `"?"` |
| `POST` | `/api/v1/estimation-sessions/{sessionId}/reveal` | Revelación de la ronda vigente (solo el facilitador) |
| `POST` | `/api/v1/estimation-sessions/{sessionId}/rounds` | Inicio de una ronda nueva después de revelar la vigente (solo el facilitador) |
| `POST` | `/api/v1/estimation-sessions/{sessionId}/finalize` | Registro de la estimación acordada, que pasa a ser los Story Points de la historia, y finalización de la sesión (solo el facilitador) |
| `POST` | `/api/v1/estimation-sessions/{sessionId}/cancel` | Cancelación de la sesión sin registrar estimación (solo el facilitador) |
| `POST` | `/api/v1/estimation-sessions/{sessionId}/facilitator` | Toma del rol de facilitador cuando el vigente lleva 5 minutos o más desconectado (cualquier participante conectado) |
| `POST` | `/api/v1/stories/{storyId}/effort-entries` | Registro de esfuerzo sobre una historia (fecha, actividad y horas), atribuido a quien tiene la sesión y anclado al sprint en el que está la historia (cualquier integrante) |
| `GET` | `/api/v1/stories/{storyId}/effort` | Esfuerzo de una historia: total de horas reales, desglose por integrante y lista de registros |
| `GET` | `/api/v1/stories/{storyId}/effort-comparison` | Comparación entre el esfuerzo estimado y el real de una historia |
| `PATCH` | `/api/v1/effort-entries/{entryId}` | Corrección de la fecha, la actividad o las horas de un registro propio; solo cambian los campos enviados (solo el autor) |
| `DELETE` | `/api/v1/effort-entries/{entryId}` | Eliminación de un registro propio (solo el autor) |
| `GET` | `/api/v1/projects/{projectId}/effort-entries` | Esfuerzo del proyecto con su total; admite los filtros opcionales y combinables `userId`, `storyId`, `sprintId` (o `none` para los registros sin sprint), `from` y `to` (`AAAA-MM-DD`, inclusivos) |
| `GET` | `/api/v1/projects/{projectId}/effort-comparison` | Comparación entre el esfuerzo estimado y el real de todas las historias del proyecto |
| `POST` | `/api/v1/projects/{projectId}/defects` | Registro de un defecto (descripción, severidad, historia y sprint de detección); nace Abierto y sin sprint de resolución (cualquier integrante) |
| `GET` | `/api/v1/projects/{projectId}/defects` | Defectos del proyecto de la severidad más alta a la más baja y, a igual severidad, del más reciente al más antiguo; admite los filtros opcionales y combinables `status` (`open`, `in_progress`, `resolved`, `discarded`) y `severity` (`critical`, `high`, `medium`, `low`), ambos repetibles, y `storyId` y `sprintId` (detectados o resueltos en ese sprint); una lista vacía trae `emptyReason` (`no_defects` o `no_matches`) |
| `GET` | `/api/v1/stories/{storyId}/defects` | Defectos de una historia (cualquier integrante) |
| `GET` | `/api/v1/defects/{defectId}` | Detalle de un defecto con sus dos sprints (cualquier integrante) |
| `PATCH` | `/api/v1/defects/{defectId}` | Corrección de `description` y `severity` (salvo Descartado) y de `storyId` y `detectedSprintId` (solo con el defecto Abierto); solo cambian los campos enviados |
| `PUT` | `/api/v1/defects/{defectId}/status` | Cambio de estado: `in_progress`, `resolved` (con `resolvedSprintId`), `open` (reapertura de un Resuelto) o `discarded` |
| `GET` | `/api/v1/projects/{projectId}/metrics` | Métricas del proyecto: velocidad con su serie, acumulados de los sprints Cerrados y del Activo, esfuerzo fuera de sprint y en sprints Planificados, historias del backlog sin estimar y defectos con su desglose por severidad (cualquier integrante) |
| `GET` | `/api/v1/projects/{projectId}/metrics/series` | Las seis series por sprint (Story Points planificados y completados, horas estimadas y reales, defectos detectados y resueltos) con los sprints Cerrados y el Activo en orden cronológico (cualquier integrante) |
| `GET` | `/api/v1/projects/{projectId}/metrics/active-sprint` | Métricas del sprint Activo, todas parciales; si no hay sprint Activo responde `200` con `activeSprint: false` y `metrics: null` (cualquier integrante) |
| `GET` | `/api/v1/projects/{projectId}/metrics/sprints/{sprintId}` | Las nueve métricas de un sprint (Cerrado, Activo o Planificado) según su estado (cualquier integrante) |

Un proyecto inexistente y un proyecto del que no se es integrante responden el mismo `404`, para no revelar qué proyectos existen. Las acciones de administración devuelven `403` a un integrante que no es el propietario. Del mismo modo, una historia inexistente y una historia de un proyecto del que no se es integrante responden el mismo `404`, y lo mismo ocurre con un sprint inexistente, eliminado o de un proyecto ajeno. Cuando el proyecto está finalizado, las acciones que modifican el backlog (crear, refinar, estimar y eliminar) y todas las escrituras sobre sus sprints responden `409`. También responden `409` las operaciones de sprints que violan una regla del dominio, por ejemplo un período superpuesto, un segundo sprint Activo o la modificación de un sprint cerrado.

En el Planning Poker, una sesión inexistente y una sesión de un proyecto del que no se es integrante responden el mismo `404`. Las acciones reservadas al facilitador devuelven `403` a cualquier otro participante, y votar sin estar conectado a la sesión también responde `403`. Responden `409` las acciones sobre una sesión ya Finalizada o Cancelada (con el resultado registrado y, si la canceló el sistema, su motivo: `inactivity`, `story_completed` o `project_finished`), la apertura de una segunda sesión sobre una historia que ya tiene una abierta (con el identificador de esa sesión), la apertura sobre una historia Completada o en un proyecto finalizado, y las acciones que no corresponden al estado de la ronda, como revelar sin votos o iniciar una ronda antes de revelar la vigente. Eliminar del backlog una historia con historial de estimación responde `409` con el motivo `estimation_sessions`.

En el registro de esfuerzo, un proyecto, una historia o un registro inexistentes, con identificador mal formado o de un proyecto del que no se es integrante responden el mismo `404`. Los datos inválidos (horas fuera de rango o que no son múltiplo de 0,25, actividad vacía o de más de 300 caracteres, fecha futura o anterior al inicio del proyecto, intento de cambiar la historia, el autor o el sprint de un registro) responden `400` con cada campo a corregir y su motivo, y lo mismo un filtro del listado del proyecto mal formado, inexistente o de otro proyecto, o un rango de fechas invertido. Corregir o eliminar un registro ajeno responde `403`. Responden `409`, con su motivo en el campo `code`, las escrituras sobre un proyecto finalizado (`read_only`), sobre un sprint cerrado con el plazo de gracia vencido (`effort_finalized`) y el registro o la corrección que superaría las 24 horas del día de esa persona (`daily_cap_exceeded`, con las horas que le quedan disponibles en `availableHours`). Eliminar del backlog una historia que alguna vez tuvo esfuerzo registrado responde `409` con el motivo `effort`, y eliminar un sprint Planificado con esfuerzo registrado también responde `409`.

En la gestión de defectos, un proyecto, una historia, un sprint o un defecto inexistentes, con identificador mal formado o de un proyecto del que no se es integrante responden el mismo `404`, y lo mismo una historia o un sprint de otro proyecto al registrar, corregir o resolver. Los datos inválidos (descripción vacía, de solo espacios o de más de 2000 caracteres, severidad inexistente, historia o sprint de detección ausentes, sprint de detección o de resolución Planificado, resolución sin sprint de resolución o con uno anterior al de detección, `status` o `resolvedSprintId` en la corrección de datos) responden `400` con cada campo a corregir y su motivo, y lo mismo un filtro del listado con un estado o una severidad inexistentes o con una historia o un sprint de otro proyecto. Responden `409`, con su motivo en el campo `code`, las escrituras sobre un proyecto finalizado (`read_only`), el registro en un proyecto sin sprint Activo ni Cerrado (`no_detection_sprint`), una transición no admitida, incluida la que llega tarde porque otra acción cambió antes el estado del defecto (`invalid_transition`, con el estado vigente), la modificación de un defecto Descartado (`defect_discarded`) y la corrección de la historia o del sprint de detección de un defecto que ya no está Abierto (`links_frozen`). Eliminar del backlog una historia con defectos responde `409` con el motivo `defects`.

#### Métricas: estados y definitividad

Los cuatro endpoints de métricas son de solo lectura, exigen sesión (`401` sin ella) y responden el mismo `404` ante un proyecto inexistente, mal formado o del que no se es integrante, y ante un sprint inexistente, mal formado o de otro proyecto. Un proyecto finalizado se consulta con normalidad. Cada respuesta refleja un único instante de los datos y trae `warnings` (por ejemplo, `unestimated_backlog_stories` cuando hay historias del backlog sin estimar); una advertencia nunca impide recibir las métricas. Las horas y los porcentajes tienen exactamente 2 decimales y los Story Points son enteros.

Cada métrica viaja con un `status` y un `value`:

| `status` | Significado |
| --- | --- |
| `available` | Hay valor; un 0 es un valor real. |
| `not_calculable` | El cociente tiene denominador 0 (por ejemplo, la desviación porcentual de un sprint sin horas estimadas, o la velocidad sin sprints Cerrados); `value` es `null`. Nunca impide calcular las demás métricas. |
| `not_applicable` | La métrica no tiene sentido para el estado del sprint (por ejemplo, las horas reales de un sprint Planificado); `value` es `null`. |

Las métricas de un sprint llevan además `finality`, que indica si el valor todavía puede cambiar:

| `finality` | Significado |
| --- | --- |
| `definitive` | El valor ya no cambia. |
| `provisional` | Métrica de un sprint Cerrado que todavía puede moverse: las horas reales y las desviaciones mientras dure el plazo de gracia (`EFFORT_GRACE_PERIOD_HOURS`), y los defectos detectados y resueltos, que pueden registrarse o resolverse después del cierre. |
| `partial` | Métrica de un sprint Activo, o acumulado que incluye al Activo. |

En un sprint Planificado solo se informan los Story Points planificados y las horas estimadas con el factor vigente, con `finality: null`; las otras siete métricas son `not_applicable`. Un sprint Cerrado se calcula con su instantánea de cierre.

**Canal de eventos del Planning Poker.** `GET /api/v1/estimation-sessions/{sessionId}/events` responde `text/event-stream` (Server-Sent Events) y queda abierto:

- **Autenticación**: se protege igual que cualquier otro endpoint, con la cabecera `Authorization: Bearer <token>`, y responde `401` sin sesión válida y `404` a quien no es integrante del proyecto. Por eso el frontend lo lee con `fetch` y no con `EventSource`, que no admite cabeceras; el token nunca viaja en la dirección. Si la sesión de estimación ya estaba cerrada, responde `409` con el resultado registrado y no se une.
- **Unirse**: abrir el canal incorpora a quien pide como participante conectado de la ronda vigente y, si ya había votado en ella, recupera su voto. Varias pestañas de una misma persona cuentan como un único participante, que sigue conectado mientras le quede al menos un canal abierto; cuando se cierra el último, deja de figurar como conectado a los 30 segundos si no vuelve.
- **Evento `state`**: su `data` es la sesión completa en JSON, vista por quien recibe el evento (nunca incluye votos ajenos antes de la revelación). Es lo primero que llega al abrir el canal y se repite ante cada hecho de la sesión y ante cada cambio del título, la descripción o los criterios de la historia. Cada instantánea lleva `version`, y el cliente descarta las de versión menor a la que ya tiene.
- **Evento `removed`**: con `data: {}`, cuando quien escucha dejó de ser integrante del proyecto; después el servidor cierra el canal.
- **Ping**: un comentario `: ping`, sin datos, cada 15 segundos, que mantiene viva la conexión y detecta los cortes.
- **Cierre**: cuando la sesión se finaliza o se cancela, el servidor envía el último `state` y cierra el canal. También lo cierra al vencer el token con el que se abrió, porque el token solo se valida al abrir el canal; al reconectar con el token vencido, la API responde `401` y la interfaz lleva a `/login`. Un cierre de sesión hecho desde otro dispositivo no corta un canal ya abierto hasta ese vencimiento, aunque las acciones REST sí se rechazan de inmediato.
- **Reconexión**: si el canal se corta por otro motivo, la sala lo vuelve a abrir sola con una espera de 1, 2 y 5 segundos (tope 5), conservando la última instantánea; se detiene ante `401`, `404` o `409`.

Para observar el canal sin la interfaz alcanza con `curl -N -H "Authorization: Bearer <token>" http://localhost:8080/api/v1/estimation-sessions/<sessionId>/events` (en PowerShell, `curl.exe`); `-N` desactiva el búfer para ver cada evento apenas llega.

Las guías de validación de punta a punta, con ejemplos de `curl` para cada historia, están en [`specs/001-user-auth/quickstart.md`](./specs/001-user-auth/quickstart.md), [`specs/002-project-members/quickstart.md`](./specs/002-project-members/quickstart.md), [`specs/003-product-backlog/quickstart.md`](./specs/003-product-backlog/quickstart.md), [`specs/004-sprint-management/quickstart.md`](./specs/004-sprint-management/quickstart.md), [`specs/005-planning-poker/quickstart.md`](./specs/005-planning-poker/quickstart.md), [`specs/006-effort-tracking/quickstart.md`](./specs/006-effort-tracking/quickstart.md) y [`specs/007-defect-tracking/quickstart.md`](./specs/007-defect-tracking/quickstart.md); la de la 005 incluye además cómo seguir el canal de eventos de varias personas en terminales separadas, y la de la 006, en lugar de ejemplos de `curl`, trae una validación manual de punta a punta paso a paso con el resultado esperado de cada paso.

### 9. Ejecutar las pruebas

Las pruebas del backend corren siempre contra la base de **test** (`smye_test`, puerto 5433), que debe estar levantada (paso 4), y requieren Swagger generado (paso 5). Cada caso de integración vacía las tablas de esa base antes de ejecutarse; nunca lee `backend/.env` ni toca la base de desarrollo.

Los comandos que abarcan varios paquetes llevan `-p 1`: el advisory lock de `backend/internal/platform/testsupport/database.go` ya serializa los paquetes sobre la base de test, y correrlos en paralelo los deja esperándose entre sí hasta poder superar su espera máxima de 5 minutos.

```bash
cd backend
go vet ./...
go test -p 1 ./...

# Solo las pruebas de un módulo (por ejemplo, la gestión de proyectos, el backlog, los sprints, el Planning Poker, el esfuerzo, los defectos o las métricas)
go test -p 1 ./internal/projects/...
go test -p 1 ./internal/backlog/...
go test -p 1 ./internal/sprints/...
go test -p 1 ./internal/poker/...
go test -p 1 ./internal/effort/...
go test -p 1 ./internal/defects/...
go test -p 1 ./internal/metrics/...

# Planning Poker: concurrencia repetida cinco veces y escala del canal de eventos
# (10 sesiones x 10 conexiones; se omite con -short)
go test -p 1 ./internal/poker/... -run Concurrency -count=5
go test -p 1 ./internal/poker/... -run Scale

# Detector de carreras sobre el Planning Poker; requiere CGO_ENABLED=1 y un compilador de C (gcc).
# Lleva -timeout 40m porque, con -race, el paquete delivery tarda unos 11 minutos y supera
# el límite por defecto de 10 minutos de go test
CGO_ENABLED=1 go test -p 1 -race -timeout 40m ./internal/poker/...

# La misma corrida sin instalar gcc, en un contenedor efímero oficial con la misma versión de Go
docker run --rm --network host -v "$(pwd):/src" -w /src -e CGO_ENABLED=1 golang:1.27.3 go test -p 1 -race -timeout 40m ./internal/poker/...

# Solo las pruebas de una historia de usuario (por ejemplo, US1)
go test -p 1 ./... -run TestUS1
```

La variante en contenedor se ejecuta desde `backend/` con la base de test levantada (paso 4). `--network host` hace que el contenedor llegue a la base de test en `localhost:5433` y use `backend/.env.test` tal cual. En Git Bash hay que anteponer `MSYS_NO_PATHCONV=1` al comando, para que no convierta `/src` en una ruta de Windows. En PowerShell se usa `${PWD}` en lugar de `$(pwd)`: `-v "${PWD}:/src"`.

El frontend no necesita servicios levantados: Vitest simula el navegador con jsdom.

```bash
cd frontend
npm test              # Vitest, una sola corrida
npm run test:watch    # Vitest en modo observación
npm run lint          # ESLint
npx next typegen && npx tsc --noEmit   # Comprobación de tipos
```

`next typegen` genera los tipos de las rutas de Next.js (por ejemplo, `LayoutProps`) que `tsc` necesita; sin él, la comprobación de tipos falla en un clon recién hecho. `npm run dev` y `npm run build` también los generan.

Cada escenario de las specs tiene una prueba rastreable: buscar `001/US1` (o `002/US3`, `003/US5`, `004/US7`, `005/US8`, `006/US5`, `007/US6`, etc.) en el código de pruebas muestra cuáles lo cubren. El prefijo es el número de la spec y `USn`, la historia de usuario. Los comandos de cobertura están en [Cobertura de Pruebas](#cobertura-de-pruebas).

### 10. Ejecución con Docker

El backend y el frontend tienen cada uno su `Dockerfile`. El del backend ejecuta `swag init` durante la construcción, así que no requiere el paso 5.

**Backend**, desde `backend/`, con la base de desarrollo levantada:

```bash
docker build -t smye-backend .
docker run --rm --env-file .env -e DB_HOST=host.docker.internal -p 8080:8080 smye-backend
```

`--env-file .env` le pasa al contenedor todas las variables de `backend/.env`, incluida `APP_TIMEZONE`. La imagen `alpine` no trae la base de zonas horarias, pero no hace falta instalarla: el binario la incluye (`time/tzdata`). Dentro de un contenedor, `localhost` es el propio contenedor y no la máquina anfitriona, por eso `DB_HOST` se reemplaza por `host.docker.internal`, que apunta al anfitrión. En Linux hay que agregar `--add-host=host.docker.internal:host-gateway` al `docker run`. Se ejecuta un único contenedor del backend, por el [supuesto de una sola instancia](#arquitectura-y-stack-tecnológico) del Planning Poker.

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
| En la sala de Planning Poker, alguien no aparece conectado o no ve los cambios de los demás | El backend se está ejecutando en más de una instancia y cada participante quedó conectado a una distinta: ejecutar una sola, por el [supuesto de una sola instancia](#arquitectura-y-stack-tecnológico). Si queda en "Conectando con la sesión…", el backend no está en ejecución o no llega a la API (ver la fila anterior); la sala reintenta sola cada pocos segundos. |
| La sala de Planning Poker lleva a `/login` en medio de una sesión de estimación | Venció el token: el canal de eventos se cierra al vencer y la reconexión recibe `401`. Iniciar sesión de nuevo y volver a la sala; el voto de la ronda vigente se conserva. |
| `go test -race` falla con `-race requires cgo` | El detector de carreras necesita `CGO_ENABLED=1` y un compilador de C (`gcc`) en el `PATH`. Sin instalar `gcc`, usar la variante en contenedor del [paso 9](#9-ejecutar-las-pruebas). |
| `go test -race` termina con `panic: test timed out after 10m0s` | Con `-race`, el paquete delivery del Planning Poker tarda unos 11 minutos y supera el límite por defecto de `go test`: agregar `-timeout 40m`, como en el [paso 9](#9-ejecutar-las-pruebas). |

---

## Cobertura de Pruebas

El backend y el frontend exponen cada uno un comando para generar el reporte de cobertura de pruebas. Los reportes son artefactos reproducibles y no se versionan (`.gitignore`).

### Backend

Las pruebas de integración corren siempre contra la base de **test** (`smye_test`, puerto 5433), configurada en `backend/.env.test`; nunca contra la base de desarrollo. Antes de medir la cobertura hay que levantar ese contenedor y tener generada la documentación Swagger ([paso 5](#5-generar-la-documentación-swagger)):

```bash
cd backend/postgres
docker compose --env-file .env.test -p smye-test up -d
```

Con la base de test arriba, desde `backend/` (`-p 1` corre los paquetes en serie, por el motivo explicado en el [paso 9](#9-ejecutar-las-pruebas)):

```bash
# Ejecuta todas las pruebas y guarda el perfil de cobertura en coverage.out
go test -p 1 ./... -coverprofile=coverage.out

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

El último análisis (2026-10-26) no reporta vulnerabilidades que alcancen al código. Para llegar a ese resultado se subió Go a 1.27.3, que corrige [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809) en `crypto/x509` de la biblioteca estándar. Queda un único aviso a nivel de módulo, justificado a continuación:

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
