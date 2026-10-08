# Plan de Implementación: Autenticación y Cuentas de Usuario

**Rama**: `001-user-auth` | **Fecha**: 2026-10-05 | **Spec**: [spec.md](./spec.md)

**Entrada**: Especificación de feature en `specs/001-user-auth/spec.md`

---

## Resumen

Esta feature permite crear una cuenta, iniciar sesión, consultar el propio perfil y cerrar sesión, y
protege toda otra funcionalidad del sistema detrás de una sesión válida. Es la base de las nueve
features siguientes: sin identidad verificada, las métricas que produce el producto no son
trazables (FR-021, RN-09).

El enfoque técnico tiene tres decisiones que lo definen:

1. **JWT con sesión persistida.** El Principio IV obliga a usar JWT, pero FR-026 exige invalidar una
   sesión de inmediato y FR-017 exige sesiones simultáneas independientes. Se resuelve emitiendo un
   JWT cuyo claim `jti` apunta a una fila de la tabla `sessions`, que el middleware verifica en cada
   petición protegida. Es la única forma de cumplir ambos requisitos sin abandonar JWT, y reproduce
   literalmente la entidad "Sesión" que declara la spec.
2. **Unicidad delegada a la base de datos.** FR-007 y SC-006 exigen que diez registros simultáneos
   con el mismo correo dejen exactamente una cuenta. Se consigue con un índice único sobre el correo
   normalizado y traduciendo `gorm.ErrDuplicatedKey` al error de correo no disponible; un
   "consultar y después insertar" tendría una condición de carrera.
3. **Eventos de autenticación en la misma transacción que el efecto que registran.** FR-028 usa
   "DEBE" y SC-009 lo mide al 100 %, así que el registro no puede ser best-effort.

Como es la primera feature del repositorio, incluye el arranque del proyecto: módulo de Go,
aplicación Next.js, PostgreSQL de desarrollo y de test con Docker Compose, Dockerfiles y Swagger.

Las decisiones y las alternativas descartadas están en [research.md](./research.md).

---

## Contexto Técnico

**Lenguaje y versión**: Go 1.27.1 (backend); TypeScript sobre Node.js 24.21.0 (frontend)

**Dependencias principales**: Gin v1.12.0, GORM v1.31.2 con driver `postgres` v1.6.3,
`golang-jwt/jwt/v5` v5.3.1, `golang.org/x/crypto` v0.57.0 (bcrypt), `google/uuid` v1.6.0,
`gin-contrib/cors` v1.7.9, `swaggo/swag` v1.16.6 con `gin-swagger` v1.6.1 y `swaggo/files` v1.0.1;
Next.js 16.3.8 con React 19.3.0 y Tailwind CSS 4.3.3

**Almacenamiento**: PostgreSQL, imagen `postgres:18.6-alpine`, en contenedores disjuntos de
desarrollo (puerto 5432, base `smye_dev`) y de test (puerto 5433, base `smye_test`)

**Pruebas**: `testify` v1.12.1 y la biblioteca estándar de Go (`net/http/httptest`) en el backend;
Vitest 5.0.3 con `@testing-library/react` 16.3.3 y jsdom 30.1.2 en el frontend

**Plataforma objetivo**: API HTTP en Linux (contenedor) y navegador moderno; desarrollo en Windows

**Tipo de proyecto**: aplicación web — backend de API y frontend separados en directorios de primer
nivel

**Objetivos de rendimiento**: no hay requisito de rendimiento en la spec. Los criterios medibles son
de experiencia de usuario (SC-001 registro en menos de 2 minutos, SC-002 inicio de sesión y pantalla
protegida en menos de 30 segundos), que ningún diseño razonable incumple. **No se fija una meta de
peticiones por segundo**: inventarla sería alcance no solicitado (Principio I).

**Restricciones**: sesión de 8 horas fijas sin renovación (FR-018); mensaje de credenciales
inválidas idéntico en todos los casos (FR-014, SC-004); cero contraseñas en texto legible en
cualquier salida (FR-011, SC-005); cero cuentas duplicadas con 10 solicitudes simultáneas (SC-006);
sin límite de intentos fallidos, riesgo aceptado explícitamente por la spec

**Escala y alcance**: 4 endpoints HTTP, 3 entidades persistidas, 3 pantallas de frontend
(registro, inicio de sesión, perfil), 5 historias de usuario con 26 escenarios de aceptación

### Incógnitas resueltas en la Fase 0

No queda ningún `NEEDS CLARIFICATION`. Las cinco preguntas abiertas de la spec se habían resuelto en
la sesión de clarificación del 2026-09-20; las incógnitas **técnicas** se resolvieron en
[research.md](./research.md): versiones del stack, revocación de JWT, unicidad bajo concurrencia,
límite de bcrypt, registro de eventos, aislamiento de entornos, transporte del token, Swagger
generado y organización en módulos.

### Puntos señalados al desarrollador (no bloquean el diseño)

1. **Go no está instalado.** Hay que instalar Go 1.27.1 antes de la primera tarea de backend.
2. **Límite de 72 de la contraseña frente a bcrypt.** FR-006 y RN-04 lo expresan en caracteres;
   bcrypt opera en bytes. El plan rechaza como entrada inválida toda contraseña de más de 72 bytes
   en UTF-8, para no violar el Principio VII con un error interno. Es un desvío de la letra de RN-04
   para contraseñas multibyte y **conviene confirmarlo**; si no se acepta, hay que ajustar la spec,
   que es la fuente de verdad. Detalle en [research.md](./research.md), sección 4.
3. **Transporte del token.** Se eligió `Authorization: Bearer` con el token en `sessionStorage`, lo
   que lo deja expuesto a XSS a diferencia de una cookie `HttpOnly`. Ningún requisito fija el
   transporte. Conviene que lo evalúe la revisión de seguridad; el detalle y el impacto del cambio
   están en [research.md](./research.md), sección 9.

---

## Verificación de la Constitución

*PUERTA: debe pasar antes de la investigación de la Fase 0. Reevaluada después del diseño de la
Fase 1.*

### Primera evaluación (antes de la Fase 0)

| Principio | Puerta | Resultado |
| --- | --- | --- |
| I — Simplicidad deliberada | ¿El alcance se limita a lo pedido? | **PASA**. Los 4 endpoints se corresponden uno a uno con US1, US2, US4 y US5; US3 es el middleware. Sin recuperación de contraseña, verificación de correo, roles ni edición de perfil: todo eso está en "Fuera de Alcance". |
| II — TDD | ¿Hay forma de escribir la prueba antes del código? | **PASA**. Cada escenario de la spec es una prueba nombrada; el entorno de test existe desde el primer paso. |
| III — Arquitectura por capas | ¿La estructura cabe en el layout obligatorio? | **PASA**. Un módulo `internal/auth/` con sus cuatro capas; frontend con las diez capas exigidas bajo `src/`. |
| IV — Stack fijo y versiones estables | ¿Hace falta algo fuera de la tabla? | **PASA**. Ninguna dependencia nueva; todas las versiones son estables y fijadas con patch exacto. |
| V — Contenerización | ¿Dev y test son disjuntos? | **PASA**. Contenedores, volúmenes, puertos, bases y archivos de entorno separados. |
| VI — Seguridad y secretos | ¿Hay secretos fuera de `.env`? | **PASA**. `JWT_SECRET` y las credenciales de base viven en `.env`; bcrypt para contraseñas. |
| VII — Robustez y validación | ¿Toda entrada se valida antes del servicio? | **PASA**. Validación en `delivery`, incluido el límite de bytes de bcrypt para que ningún error llegue sin manejar. |
| VIII — Estándares y documentación viva | ¿Swagger, README e idiomas? | **PASA**. Identificadores en inglés, comentarios y Swagger en español, README actualizado en el mismo cambio. |
| IX — Gobierno del repositorio | ¿El plan pide operaciones prohibidas? | **PASA**. El plan no ejecuta ninguna escritura de Git; los commits los hace una persona. |

**Resultado: todas las puertas pasan. Sin violaciones que justificar.**

### Reevaluación (después del diseño de la Fase 1)

| Principio | Qué se revisó en el diseño | Resultado |
| --- | --- | --- |
| I | El modelo de datos tiene 3 tablas y ninguna columna sin requisito que la pida. Se descartaron `issued_at`, `occurred_at` y una columna de correo sin normalizar por duplicar datos. Las interfaces de repositorio declaran 7 operaciones, todas usadas; sin CRUD especulativo. | **PASA** |
| II | Los 26 escenarios se mapean a pruebas nombradas (tabla de trazabilidad más abajo). Las pruebas unitarias obligatorias son las de validaciones y reglas de negocio RN-01 a RN-09, que es exactamente lo que exige el Principio II para esta feature (no hay cálculo de métricas ni estimación todavía). | **PASA** |
| III | `domain/` no importa a ninguna otra capa. Los dos paquetes de `platform/` no son módulos de feature y existen porque `cmd/app/` solo puede componer dependencias y las pruebas no pueden importar un paquete `main`. | **PASA** |
| IV | Las 13 dependencias de Go y las 11 de npm están todas en la tabla autorizada o cubiertas por la cláusula de la guía oficial de Vitest de Next.js. `postgres:18.6-alpine` fija el patch, no una etiqueta móvil. Se descartó TypeScript 7 por riesgo sin requisito. | **PASA** |
| V | Un solo `docker-compose.yml` con dos nombres de proyecto y dos archivos de entorno da contenedores y volúmenes disjuntos. Las pruebas cargan `backend/.env.test` y la guía incluye una comprobación de que apuntan a `smye_test`. | **PASA** |
| VI | `JWT_SECRET` validado al arrancar con un mínimo de 32 bytes; `password_hash` es la única forma en que existe la contraseña; `.gitignore` con secciones `#Frontend` y `#Backend` que excluyen `.env`, `node_modules/`, `.next/`, `backend/docs/` y los artefactos de cobertura. | **PASA** |
| VII | Los cuatro endpoints validan antes de la capa de servicio. `gorm.ErrDuplicatedKey` se traduce a 409 en lugar de propagarse como error interno. El límite de 72 bytes se rechaza como validación y no como `panic` de bcrypt. Sin `panic` en el camino de ninguna petición. | **PASA** |
| VIII | El contrato OpenAPI está en español; las anotaciones `swag` también. Identificadores en inglés en los dos lenguajes. `any` prohibido: los tipos del frontend son explícitos. | **PASA** |
| IX | Los artefactos generados son archivos de la copia de trabajo. No hay ninguna operación de Git en el plan. | **PASA** |

**Resultado: todas las puertas siguen pasando después del diseño. La sección de seguimiento de
complejidad queda vacía porque no hay ninguna violación que justificar.**

#### Dos decisiones que la constitución obliga a documentar

Ninguna es una violación, pero las dos son concesiones conscientes y quedan registradas:

1. **Una lectura a la base de datos por petición protegida** (verificación del `jti`). Es el costo de
   cumplir FR-026 con JWT. No se añade caché (Principio I).
2. **`backend/docs/` no se versiona.** El Principio VI obliga a excluir todo artefacto reproducible
   por comandos, y Swagger generado lo es. La contrapartida es que un clon recién hecho no compila
   hasta ejecutar `swag init`; se mitiga documentándolo en el `README.md` y ejecutándolo dentro del
   `Dockerfile`. La alternativa, versionarlo, contradice el Principio VI, que prevalece.

---

## Estructura del Proyecto

### Documentación de esta feature

```text
specs/001-user-auth/
├── spec.md              # Especificación (entrada)
├── plan.md              # Este archivo (salida de /speckit-plan)
├── research.md           # Salida de la Fase 0
├── data-model.md         # Salida de la Fase 1
├── quickstart.md         # Salida de la Fase 1
├── contracts/
│   └── openapi.yaml      # Salida de la Fase 1
├── checklists/
│   └── requirements.md   # Checklist de calidad de la spec (17/17)
└── tasks.md              # Salida de /speckit-tasks — NO lo crea /speckit-plan
```

### Código fuente (raíz del repositorio)

```text
backend/
├── cmd/
│   └── app/
│       └── main.go                    # Composición de dependencias y arranque; sin lógica
├── internal/
│   ├── auth/                          # Módulo único de la feature
│   │   ├── domain/
│   │   │   ├── user.go                # Entidad User
│   │   │   ├── session.go             # Entidad Session y sus reglas de vigencia
│   │   │   ├── auth_event.go          # Entidad AuthEvent y los cuatro tipos
│   │   │   ├── errors.go              # Errores de dominio (correo ocupado, credenciales, sesión)
│   │   │   └── repository.go          # Interfaces de repositorio (solo lo que se usa)
│   │   ├── repository/
│   │   │   ├── user_repository.go     # GORM; traduce ErrDuplicatedKey
│   │   │   ├── session_repository.go  # GORM; creación y revocación transaccionales
│   │   │   └── auth_event_repository.go
│   │   ├── service/
│   │   │   ├── auth_service.go        # Registro, inicio de sesión, cierre y validación de sesión
│   │   │   ├── password.go            # Hash y comparación con bcrypt
│   │   │   └── token.go               # Emisión y parseo del JWT (jti, sub, exp, iat)
│   │   └── delivery/
│   │       ├── handler.go             # 4 manejadores HTTP con anotaciones swag en español
│   │       ├── dto.go                 # Peticiones y respuestas; sin campos de contraseña
│   │       ├── validation.go          # Validación y normalización de entrada
│   │       ├── middleware.go          # Exigencia de sesión válida (FR-019, FR-020)
│   │       └── routes.go              # Registro de rutas públicas y protegidas
│   └── platform/
│       ├── config/
│       │   └── config.go              # Variables de entorno con la biblioteca estándar
│       ├── database/
│       │   └── postgres.go            # Conexión GORM (TranslateError) y AutoMigrate
│       └── testsupport/
│           └── database.go            # Carga .env.test, verifica smye_test y vacía las tablas
├── postgres/
│   ├── docker-compose.yml             # Un archivo, dos entornos por nombre de proyecto
│   ├── .env.example                   # Versionado
│   ├── .env                           # NO versionado — desarrollo, puerto 5432
│   └── .env.test                      # NO versionado — test, puerto 5433
├── .env.example                       # Versionado
├── .env                               # NO versionado
├── .env.test                          # NO versionado
├── Dockerfile                         # Ejecuta swag init durante la construcción
├── go.mod
└── go.sum

frontend/
├── src/
│   ├── app/
│   │   ├── layout.tsx                 # Layout raíz con el proveedor de sesión
│   │   ├── page.tsx                   # Portada pública
│   │   ├── register/page.tsx          # US1
│   │   ├── login/page.tsx             # US2
│   │   └── profile/page.tsx           # US4 y US5; protegida en el cliente
│   ├── features/
│   │   └── auth/
│   │       ├── components/            # RegisterForm, LoginForm, LogoutButton, ProfileCard
│   │       ├── api.ts                 # Llamadas a los 4 endpoints
│   │       └── validation.ts          # Reglas de RC-02 en el formulario
│   ├── components/                    # Componentes de UI reutilizables (Button, Input, Alert)
│   ├── config/
│   │   └── env.ts                     # NEXT_PUBLIC_API_BASE_URL tipada
│   ├── hooks/
│   │   └── use-session.ts             # Acceso al estado de sesión y guardia de ruta
│   ├── lib/
│   │   ├── api-client.ts              # fetch con Authorization: Bearer; sin cliente de terceros
│   │   └── session-storage.ts         # Lectura y escritura del token en sessionStorage
│   ├── providers/
│   │   └── session-provider.tsx       # Contexto de sesión
│   ├── styles/
│   │   └── globals.css                # Tailwind
│   ├── types/
│   │   └── auth.ts                    # Tipos de las peticiones y respuestas; sin `any`
│   └── utils/
│       └── normalize-email.ts         # Recorte y minúsculas, igual que el backend
├── .env.example                       # Versionado
├── .env.local                         # NO versionado
├── .env.test                          # NO versionado
├── Dockerfile
├── next.config.ts                     # agentRules: false (Principio IX, ámbito f)
├── vitest.config.ts
├── package.json
└── tsconfig.json

.gitignore                             # Secciones #Frontend y #Backend
README.md                              # Actualizado con el paso a paso de ejecución
```

**Pruebas**: en el backend, junto al código del paquete que prueban, según la convención de Go
(`auth_service_test.go`, `handler_test.go`, `validation_test.go`). En el frontend, junto al
componente o módulo (`register-form.test.tsx`). No se crea un árbol `tests/` separado: el layout
estándar de Go no lo usa y el Principio III no lo pide.

La única excepción es el **ayudante de aislamiento** de las pruebas de integración, que vive en
`internal/platform/testsupport/database.go` como archivo normal y no como `*_test.go`: el contenido
de un `_test.go` solo existe para su propio paquete, y las pruebas que lo necesitan están en
`internal/auth/delivery/`. El paquete `testsupport` no lo importa el código de producción, solo las
pruebas, y queda disponible para las features siguientes.

**Decisión de estructura**: aplicación web con `backend/` y `frontend/` como directorios de primer
nivel, sin mezcla de artefactos (Principio III). El backend usa organización vertical por módulo:
`internal/auth/` contiene sus cuatro capas y las dependencias apuntan hacia `domain/`. Se eligió un
único módulo de feature en lugar de separar `user` y `auth` porque ninguna funcionalidad pedida
atraviesa esa frontera (ver [research.md](./research.md), sección 11). El frontend declara las diez
capas que exige el Principio III bajo `src/`, cada una con su responsabilidad.

---

## Trazabilidad de escenarios a pruebas (Principio II)

Cada escenario Given/When/Then de la spec se implementa como al menos una prueba cuyo nombre
identifica la historia y el escenario. Buscar `001/US1` a `001/US5` en el código de pruebas devuelve
la trazabilidad completa.

**Backend (Go)**: función `TestUSn_<Escenario>` en el paquete del módulo, precedida por el
comentario `// Escenario: <nombre del escenario en spec.md> (001/USn)`. Las variantes de un mismo
escenario se agrupan como subtests con `t.Run`.

**Frontend (Vitest)**: `describe("001/USn - <título de la historia>")` con un
`it("<nombre del escenario en spec.md>")` por escenario.

| Historia | Escenarios | Pruebas de backend previstas |
| --- | --- | --- |
| US1 Registro | 8 | `TestUS1_AltaExitosa`, `TestUS1_CorreoNormalizado`, `TestUS1_LimitesDeContrasena`, `TestUS1_LimitesDeNombre`, `TestUS1_RegistroConcurrente`, `TestUS1_RegistroNoEmiteSesion`, `TestUS1_CorreoDuplicado`, `TestUS1_EntradaInvalida` |
| US2 Inicio de sesión | 5 | `TestUS2_InicioExitoso`, `TestUS2_CorreoConEspaciosYMayusculas`, `TestUS2_SesionesSimultaneas`, `TestUS2_CorreoInexistente`, `TestUS2_ContrasenaIncorrecta` |
| US3 Protección | 4 | `TestUS3_AccesoConSesionVigente`, `TestUS3_RutasPublicas`, `TestUS3_SesionVencida`, `TestUS3_SesionAdulterada` |
| US4 Perfil | 4 | `TestUS4_PerfilPropio`, `TestUS4_CorreoNormalizadoEnPerfil`, `TestUS4_RespuestaSinContrasena`, `TestUS4_PerfilSinSesion` |
| US5 Cierre de sesión | 5 | `TestUS5_CierreExitoso`, `TestUS5_NuevoInicioTrasCierre`, `TestUS5_TokenCerradoNoSirve`, `TestUS5_CierreNoAfectaOtrasSesiones`, `TestUS5_CierreSinSesion` |

Además, pruebas unitarias **obligatorias** por el Principio II (validaciones y reglas de negocio),
que cubren SC-008:

| Objeto de prueba | Reglas cubiertas |
| --- | --- |
| Normalización de correo | RN-01, RN-02 |
| Validación del nombre | RN-03 |
| Validación de la contraseña (rango, composición, límite de bytes) | RN-04 |
| Hash y comparación bcrypt; ausencia de la contraseña en toda salida | RN-05 |
| Mensaje genérico de credenciales inválidas | RN-06 |
| Vigencia de la sesión: vencida, revocada, inexistente | RN-07 |
| Clasificación de rutas públicas y protegidas | RN-08 |
| Atribución de la acción al titular de la sesión | RN-09 |

Comparación con los criterios de éxito: SC-003 se verifica con `TestUS3_*` sobre cada endpoint
protegido; SC-004 comparando byte a byte las respuestas de `TestUS2_CorreoInexistente` y
`TestUS2_ContrasenaIncorrecta`; SC-005 con `TestUS4_RespuestaSinContrasena` más la revisión del
registro de eventos; SC-006 con `TestUS1_RegistroConcurrente`; SC-009 con las aserciones sobre
`auth_events` de cada historia.

---

## Secuencia de trabajo prevista

El orden lo fija `/speckit-tasks`; esto es la dependencia entre bloques, no la lista de tareas.

1. **Fase técnica de arranque** (sin prueba asociada: infraestructura y configuración).
   Instalar Go 1.27.1. Crear `backend/postgres/` con el `docker-compose.yml` y los tres archivos de
   entorno; levantar los dos contenedores. Inicializar el módulo de Go con las dependencias fijadas.
   Crear el proyecto Next.js con `create-next-app --no-agents-md` y fijar `agentRules: false`.
   `.gitignore` con sus dos secciones.
2. **Puerta obligatoria**: leer la documentación de Next.js instalada en
   `frontend/node_modules/next/dist/docs/` **antes** de escribir código de Next.js o configurar
   Vitest, y usar su guía oficial de Vitest para fijar el conjunto exacto de paquetes de prueba.
3. **Fase técnica de base** (bloquea a todas las historias): configuración y conexión de
   `platform/`, las **tres entidades** `User`, `Session` y `AuthEvent` con sus errores de dominio y
   sus interfaces de repositorio, `AutoMigrate`, el arranque del servidor y el ayudante de
   aislamiento de las pruebas en `platform/testsupport/`. Las tres entidades van juntas acá, y no
   dentro de la historia que primero las usa, porque `AutoMigrate` y el ayudante de pruebas las
   necesitan a las tres para poder aislar cualquier prueba de integración.
4. **US1 Registro** (P1): validación, hash bcrypt, repositorio con traducción de
   `ErrDuplicatedKey`, servicio, endpoint, pantalla de registro.
5. **US2 Inicio de sesión** (P1): emisión del JWT con `jti`, repositorios de sesión y de eventos,
   servicio, endpoint, pantalla de inicio de sesión.
6. **US3 Protección** (P2): reglas de vigencia de la sesión, middleware y rutas protegidas.
7. **US4 Perfil** (P3): endpoint `GET /api/v1/users/me` y pantalla de perfil.
8. **US5 Cierre de sesión** (P3): revocación, endpoint y botón de cierre.
9. **Cierre**: Swagger comparado con el contrato, `README.md`, comandos de cobertura,
   `govulncheck` y la verificación transversal de trazabilidad y puertas de seguridad
   (SC-003, SC-004, SC-005, SC-008).

En cada historia el ciclo es el del Principio II: escribir la prueba, **verificar que falla y
detenerse** para que el desarrollador registre el commit RED con `/redactar-commit`, implementar,
verificar que pasa, refactorizar. Un agente no avanza de RED a GREEN sin ese alto.

---

## Seguimiento de Complejidad

> Se completa SOLO si la verificación de la constitución tiene violaciones que justificar.

**Sin violaciones.** Las dos puertas de evaluación pasan en su totalidad. Las dos concesiones
documentadas (una lectura de base de datos por petición protegida y `backend/docs/` sin versionar)
son consecuencias necesarias de requisitos de la spec y de la propia constitución, no desvíos de
ella; están justificadas en la sección "Verificación de la Constitución" y en
[research.md](./research.md).
