---

description: "Lista de tareas de la feature 001 — Autenticación y Cuentas de Usuario"
---

# Tareas: Autenticación y Cuentas de Usuario

**Entrada**: documentos de diseño de `specs/001-user-auth/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/openapi.yaml](./contracts/openapi.yaml),
[quickstart.md](./quickstart.md)

**Pruebas**: **incluidas y obligatorias**. El Principio II de la constitución es NO NEGOCIABLE: la
prueba se escribe antes del código de producción. Las pruebas unitarias son obligatorias para
validaciones y reglas de negocio (RN-01 a RN-09); las de integración son el mecanismo por defecto.

**Organización**: las tareas se agrupan por historia de usuario para que cada una se implemente y
se pruebe de forma independiente.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[USn]**: historia de usuario a la que pertenece la tarea
- Cada descripción incluye la ruta exacta del archivo

## Convenciones de ruta

- **Backend**: `backend/cmd/app/`, `backend/internal/<módulo>/{domain,repository,service,delivery}/`
- **Frontend**: `frontend/src/{app,features,components,config,hooks,lib,providers,styles,types,utils}/`
- **Pruebas**: junto al código que prueban (Go: `*_test.go` en el mismo paquete; Vitest:
  `*.test.ts`/`*.test.tsx` junto al módulo). No se crea un árbol `tests/` separado.

## Ciclo obligatorio en cada tarea

Según el Principio II y la skill `/speckit-implement`:

1. **Tarea de pruebas** → se escribe la prueba, se verifica que **falla** y se **detiene**. El
   desarrollador registra el commit con `/redactar-commit`, pie `TDD: red`.
2. **Tarea de implementación** → código mínimo que hace pasar la prueba (`TDD: green`), pausa, y
   refactor si corresponde (`TDD: refactor`).
3. **Tarea técnica** → sin prueba asociada, sin pie TDD.

Ningún agente avanza de RED a GREEN sin ese alto.

---

## Fase 1: Setup (Infraestructura compartida)

**Propósito**: arranque del repositorio. Hoy no existen `backend/` ni `frontend/`; sin esta fase no
hay dónde ejecutar la primera prueba (research.md, sección 0). Todas las tareas son **técnicas**.

**Rama**: `fase/001-setup`

- [X] T001 Instalar Go `1.27.1` y verificar con `go version` (prerrequisito bloqueante del
      desarrollador según research.md sección 0; no produce archivos en el repositorio)
- [X] T002 [P] Crear `backend/postgres/docker-compose.yml` con el servicio PostgreSQL en la imagen
      `postgres:18.6-alpine` (patch exacto, nunca `18`, `18-alpine` ni `latest`), parametrizado por
      `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` y `POSTGRES_HOST_PORT`, con volumen
      nombrado para los datos
- [X] T003 [P] Crear `backend/postgres/.env.example` (versionado) con las claves
      `POSTGRES_USER=smye`, `POSTGRES_PASSWORD=cambiar_este_valor`, `POSTGRES_DB=smye_dev`,
      `POSTGRES_HOST_PORT=5432`, y a partir de él `backend/postgres/.env` (desarrollo) y
      `backend/postgres/.env.test` (test, con `POSTGRES_DB=smye_test` y `POSTGRES_HOST_PORT=5433`),
      ambos **no versionados**
- [X] T004 Levantar los dos entornos con `backend/postgres/docker-compose.yml` usando
      `docker compose --env-file .env -p smye-dev up -d` y
      `docker compose --env-file .env.test -p smye-test up -d`, y verificar el aislamiento del
      Principio V con `psql -U smye -d smye_test -c "SELECT current_database(), inet_server_port();"`
      (debe responder `smye_test`; si responde `smye_dev`, detenerse) — depende de T002 y T003
- [X] T005 [P] Crear `backend/.env.example` (versionado) con `DB_HOST`, `DB_PORT`, `DB_USER`,
      `DB_PASSWORD`, `DB_NAME`, `JWT_SECRET`, `SERVER_PORT=8080` y
      `CORS_ALLOWED_ORIGIN=http://localhost:3000`, y a partir de él `backend/.env` y
      `backend/.env.test` (con `DB_PORT=5433` y `DB_NAME=smye_test`), ambos **no versionados**
- [X] T006 Inicializar el módulo de Go en `backend/go.mod` y fijar las versiones exactas de
      research.md sección 1: `gin v1.12.0`, `gorm.io/gorm v1.31.2`,
      `gorm.io/driver/postgres v1.6.3`, `golang-jwt/jwt/v5 v5.3.1`, `golang.org/x/crypto v0.57.0`,
      `google/uuid v1.6.0`, `gin-contrib/cors v1.7.9`, `swaggo/swag v1.16.6`,
      `swaggo/gin-swagger v1.6.1`, `swaggo/files v1.0.1`, `stretchr/testify v1.12.1` — depende de T001
- [X] T007 [P] Instalar las herramientas de Go fuera del módulo:
      `go install github.com/swaggo/swag/cmd/swag@v1.16.6` y
      `go install golang.org/x/vuln/cmd/govulncheck@v1.8.0` (sin archivos en el repositorio) —
      depende de T001
- [X] T008 Crear la aplicación Next.js en `frontend/` con `create-next-app` fijando `next 16.3.8`,
      `react`/`react-dom 19.3.0`, `tailwindcss 4.3.3` y TypeScript (rama 5.x que fije
      `create-next-app`, **no** 7.x), pasando `--no-agents-md` (Principio IX, ámbito f)
- [X] T009 **PUERTA OBLIGATORIA**: leer la documentación de la versión instalada de Next.js en
      `frontend/node_modules/next/dist/docs/`, en particular su guía oficial de Vitest, **antes** de
      escribir cualquier código de Next.js o configurar las pruebas. Si la guía exige un paquete
      fuera de la lista autorizada del Principio IV, detenerse y pedir autorización — depende de T008
- [X] T010 Fijar `agentRules: false` en `frontend/next.config.ts` y verificar que no exista ningún
      `AGENTS.md` en el repositorio; si apareciera, avisar al desarrollador sin borrarlo
      (Principio IX, ámbito f) — depende de T009
- [X] T011 [P] Crear `frontend/.env.example` (versionado) con
      `NEXT_PUBLIC_API_BASE_URL=http://localhost:8080`, y a partir de él `frontend/.env.local` y
      `frontend/.env.test`, ambos **no versionados** — depende de T008
- [X] T012 Configurar Vitest en `frontend/vitest.config.mts` y los scripts `test` (`vitest run`),
      `test:watch` (`vitest`) y `test:coverage` (`vitest run --coverage`) en `frontend/package.json`,
      con `vitest 5.0.3`, `@vitejs/plugin-react 6.1.1`, `jsdom 30.1.2`,
      `@testing-library/react 16.3.3`, `@testing-library/dom 10.4.2`, `vite-tsconfig-paths 6.1.1` y
      `@vitest/coverage-v8 5.0.3`, previa fijación de `@types/node 24.19.1`, exactamente como indique la guía leída en T009 — depende de T009
- [X] T013 [P] Configurar `prettier 3.9.9` en `frontend/` (archivo de configuración y script de
      formateo en `frontend/package.json`) — depende de T008
- [X] T014 [P] Crear `.gitignore` en la raíz con las secciones `#Frontend` y `#Backend`
      (Principio VI), excluyendo `.env`, `.env.test`, `.env.local`, `node_modules/`, `.next/`,
      `backend/docs/`, `coverage.out` y `coverage.html`

**Checkpoint**: Go instalado, los dos contenedores de PostgreSQL arriba en puertos disjuntos, módulo
de Go y proyecto Next.js creados, Vitest configurado y `.gitignore` en su sitio.

---

## Fase 2: Foundational (Prerrequisitos bloqueantes)

**Propósito**: infraestructura de código que TODA historia necesita: configuración, conexión a la
base, las tres entidades persistidas, los errores de dominio, las interfaces de repositorio, el
arranque del servidor y el ayudante de aislamiento de las pruebas.

**⚠️ CRÍTICO**: ninguna historia de usuario puede empezar hasta que esta fase esté completa.

**Rama**: `fase/001-foundational`

- [X] T015 Implementar la lectura de variables de entorno con la biblioteca estándar en
      `backend/internal/platform/config/config.go`, incluida la validación al arrancar de que
      `JWT_SECRET` tenga **al menos 32 bytes** (si falta o es más corto, la aplicación no arranca y
      lo informa; research.md sección 2) — prohibido usar cargadores de terceros (Principio IV)
- [X] T016 Implementar el ayudante lector de archivos `KEY=VALUE` en
      `backend/internal/platform/config/dotenv.go` usando solo `bufio` y `strings`, para que
      `go test ./...` cargue `backend/.env.test` sin exportar variables a mano (research.md
      sección 8)
- [X] T017 Abrir la conexión GORM con `TranslateError: true` y ejecutar `AutoMigrate` de los tres
      modelos en `backend/internal/platform/database/postgres.go` (research.md sección 7; sin
      herramientas de migración externas) — depende de T018, T019 y T020
- [X] T018 [P] Declarar la entidad `User` en `backend/internal/auth/domain/user.go` con los campos
      de data-model.md sección 1: `ID uuid` PK generado con `google/uuid`,
      `FullName varchar(100) NOT NULL`, `Email varchar(254) NOT NULL` con **índice único**,
      `PasswordHash varchar(60) NOT NULL` (largo fijo de bcrypt), `CreatedAt` y `UpdatedAt`
      `timestamptz NOT NULL`. El modelo no declara colecciones ni `Preload` hacia `sessions` ni
      `auth_events`
- [X] T019 [P] Declarar la entidad `Session` en `backend/internal/auth/domain/session.go` con los
      campos de data-model.md sección 2: `ID uuid` PK (es el claim `jti`),
      `UserID uuid NOT NULL` con FK a `users.id` e indexada, `ExpiresAt timestamptz NOT NULL`,
      `RevokedAt timestamptz` **anulable**, `CreatedAt timestamptz NOT NULL` (**es el momento de
      emisión**) y `UpdatedAt timestamptz NOT NULL`. Sin columna de estado: los tres estados se
      derivan de los datos
- [X] T020 [P] Declarar la entidad `AuthEvent` en `backend/internal/auth/domain/auth_event.go` con
      los campos de data-model.md sección 3: `ID uuid` PK,
      `EventType varchar(32) NOT NULL`, `UserID uuid` **anulable** con FK a `users.id` e indexada,
      `AttemptedEmail varchar(254)` **anulable**, `CreatedAt timestamptz NOT NULL` (**es el momento
      del evento**) y `UpdatedAt timestamptz NOT NULL`; más las cuatro constantes de Go tipadas
      `account_created`, `login_succeeded`, `login_failed` y `logout` (no `enum` de PostgreSQL).
      **Ninguna columna admite contraseñas** (FR-030)
- [X] T021 [P] Declarar los errores de dominio en `backend/internal/auth/domain/errors.go`: correo
      no disponible (FR-005), credenciales inválidas (FR-014, mensaje único) y sesión inválida
      (FR-015)
- [X] T022 Declarar en `backend/internal/auth/domain/repository.go` las tres interfaces con
      **exactamente** las siete operaciones de data-model.md sección 5 —
      `UserRepository`: `Create(ctx, user, event) error`, `FindByEmail(ctx, email) (*User, error)`,
      `FindByID(ctx, id) (*User, error)`; `SessionRepository`: `Create(ctx, session, event) error`,
      `FindByID(ctx, id) (*Session, error)`, `Revoke(ctx, id, at, event) error`;
      `AuthEventRepository`: `Create(ctx, event) error`. **Prohibido** declarar `Update`, `Delete`,
      `List` ni búsquedas por otros campos (Principio III) — depende de T018, T019, T020
- [X] T023 Componer dependencias y arrancar el servidor en `backend/cmd/app/main.go` (sin lógica de
      negocio, Principio III), con el grupo de rutas `/api/v1` vacío en
      `backend/internal/auth/delivery/routes.go`, el middleware de CORS leyendo
      `CORS_ALLOWED_ORIGIN` y la publicación de Swagger en `/swagger/index.html` con `gin-swagger` —
      depende de T015 y T017
- [X] T024 Implementar el ayudante de pruebas de integración en
      `backend/internal/platform/testsupport/database.go`: carga `backend/.env.test` con T016,
      abre la conexión, **verifica que la base sea `smye_test`** antes de tocar nada, y vacía las
      tres tablas con `TRUNCATE users, sessions, auth_events CASCADE` antes de cada caso
      (research.md sección 8). Va en un **archivo normal de un paquete propio**, no en un
      `*_test.go`: el contenido de un `_test.go` solo existe para su propio paquete, y las pruebas
      de integración de esta feature viven en `internal/auth/delivery/`, así que desde ahí no
      podrían usarlo. El paquete `testsupport` no se importa desde el código de producción, solo
      desde las pruebas — depende de T016 y T017

**Checkpoint**: la base de test migrada, el servidor arranca, las pruebas de integración pueden
aislarse. Las historias de usuario pueden empezar.

---

## Fase 3: Historia de Usuario 1 — Registro de cuenta (Prioridad: P1) 🎯 MVP

**Objetivo**: una persona crea una cuenta con nombre completo, correo y contraseña; el correo queda
ocupado para un segundo registro y el registro **no** emite sesión (FR-009).

**Rama**: `hu/001-us1-registro-de-cuenta`

**Prueba independiente**: registrar una cuenta nueva con `POST /api/v1/auth/register` y comprobar
que un segundo registro con el mismo correo en otra capitalización responde `409`, sin necesitar
ninguna otra historia.

### Pruebas de la Historia 1 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [ ] T025 [P] [US1] Pruebas unitarias **obligatorias** de validación y normalización en
      `backend/internal/auth/delivery/validation_test.go`, cubriendo RN-01 a RN-04: correo
      normalizado con `TrimSpace` + `ToLower` (RN-01, RN-02); nombre de **entre 2 y 100 runas tras
      `TrimSpace`**, con nombre vacío o solo de espacios rechazado (RN-03); contraseña de **entre 8
      y 72 runas, al menos una letra y un dígito, y como máximo 72 bytes en UTF-8** (RN-04);
      correo con formato válido vía `net/mail.ParseAddress`, sin nombre para mostrar y **hasta 254
      caracteres** (FR-003)
- [ ] T026 [P] [US1] Pruebas unitarias **obligatorias** de hash de contraseña en
      `backend/internal/auth/service/password_test.go`, cubriendo RN-05: el hash no es reversible,
      mide 60 caracteres, dos hashes de la misma contraseña difieren y la contraseña en claro no
      aparece en ninguna salida
- [ ] T027 [US1] Pruebas de integración de la historia en
      `backend/internal/auth/delivery/handler_test.go`, una función por escenario de spec.md,
      precedida por `// Escenario: <nombre del escenario> (001/US1)`: `TestUS1_AltaExitosa`,
      `TestUS1_CorreoNormalizado`, `TestUS1_LimitesDeContrasena` (8 y 72 caracteres),
      `TestUS1_LimitesDeNombre` (2 y 100 caracteres), `TestUS1_RegistroConcurrente` (**10
      solicitudes simultáneas con el mismo correo dejan exactamente una cuenta**, SC-006),
      `TestUS1_RegistroNoEmiteSesion`, `TestUS1_CorreoDuplicado` (otra capitalización → `409`) y
      `TestUS1_EntradaInvalida` (subtests con `t.Run` para nombre ausente, correo ausente, correo
      con formato inválido, contraseña de 7 y de 73 caracteres, sin letra y sin dígito). Cada
      escenario asserta además la fila `account_created` en `auth_events` con `user_id`
      (FR-028, SC-009) — depende de T024
- [ ] T028 [P] [US1] Pruebas de frontend en
      `frontend/src/features/auth/components/register-form.test.tsx` y
      `frontend/src/features/auth/validation.test.ts`, con
      `describe("001/US1 - Registro de cuenta")` y un `it` por escenario, incluido que el formulario
      **muestra el límite de 8 a 72 caracteres antes de enviar, no al fallar** (RC-02) — depende
      de T012

### Implementación de la Historia 1

- [ ] T029 [US1] Implementar normalización y validación de entrada en
      `backend/internal/auth/delivery/validation.go`, con los valores límite 2, 100, 8, 72 y 254
      declarados como **constantes nombradas en un único lugar** (research.md sección 13). La
      validación ocurre antes de la capa de servicio (Principio VII) y el límite de 72 bytes se
      rechaza como entrada inválida, nunca como `panic` de bcrypt
- [ ] T030 [US1] Implementar el hash bcrypt en `backend/internal/auth/service/password.go` con
      `golang.org/x/crypto/bcrypt` (FR-010, RN-05)
- [ ] T031 [US1] Implementar `Create(ctx, user, event)` en
      `backend/internal/auth/repository/user_repository.go`: `INSERT users` +
      `INSERT auth_events(account_created)` en **una sola transacción** (research.md sección 5), y
      traducir `gorm.ErrDuplicatedKey` al error de correo no disponible, sin "consultar y después
      insertar" (FR-007, SC-006) — depende de T029
- [ ] T032 [US1] Implementar `Register` en `backend/internal/auth/service/auth_service.go`: hash,
      construcción del `User` con UUID y del `AuthEvent` `account_created`, y delegación al
      repositorio. **No emite sesión** (FR-009) — depende de T030 y T031
- [ ] T033 [US1] Declarar en `backend/internal/auth/delivery/dto.go` los esquemas de
      contracts/openapi.yaml: `RegisterRequest` (`fullName`, `email`, `password`), `UserProfile`
      (`id`, `fullName`, `email`), `ErrorResponse` (`error`) , `FieldError` (`field`, `message`) y
      `ValidationErrorResponse` (`error`, `fields` con `minItems: 1`). **Ningún DTO de salida tiene
      campo de contraseña ni nada derivado de ella** (FR-011, SC-005)
- [ ] T034 [US1] Implementar el manejador `registerUser` en
      `backend/internal/auth/delivery/handler.go` con anotaciones `swag` en español, registrado como
      ruta **pública** `POST /api/v1/auth/register` en
      `backend/internal/auth/delivery/routes.go`, devolviendo `201` con `UserProfile`, `400` con
      `ValidationErrorResponse` y `409` con `ErrorResponse` (`El correo electrónico no está
      disponible.`) — depende de T032 y T033
- [ ] T035 [P] [US1] Declarar los tipos de petición y respuesta en `frontend/src/types/auth.ts` y la
      variable `NEXT_PUBLIC_API_BASE_URL` tipada en `frontend/src/config/env.ts`. **Prohibido el
      tipo `any`** (Principio VIII) — depende de T009
- [ ] T036 [US1] Implementar el cliente HTTP con `fetch` nativo en
      `frontend/src/lib/api-client.ts` (sin clientes de terceros, Principio IV) y la llamada de
      registro en `frontend/src/features/auth/api.ts` — depende de T035
- [ ] T037 [US1] Implementar `frontend/src/utils/normalize-email.ts` (recorte y minúsculas, igual
      que el backend) y las reglas de RC-02 en `frontend/src/features/auth/validation.ts`, con los
      límites 2, 100, 8, 72 y 254 como constantes nombradas en un único lugar — depende de T035
- [ ] T038 [US1] Implementar los componentes de UI reutilizables `Button`, `Input` y `Alert` en
      `frontend/src/components/` y el `RegisterForm` en
      `frontend/src/features/auth/components/register-form.tsx`, con mensajes en español — depende
      de T036 y T037
- [ ] T039 [US1] Crear `frontend/src/app/layout.tsx`, la portada pública
      `frontend/src/app/page.tsx`, la pantalla `frontend/src/app/register/page.tsx` y
      `frontend/src/styles/globals.css` con Tailwind — depende de T038

**Checkpoint**: US1 funciona y se prueba sola. El MVP está entregable.

---

## Fase 4: Historia de Usuario 2 — Inicio de sesión (Prioridad: P1)

**Objetivo**: una persona con cuenta obtiene una sesión vigente con vencimiento a las 8 horas, con
sesiones simultáneas independientes y un mensaje de credenciales inválidas idéntico en todos los
casos.

**Rama**: `hu/001-us2-inicio-de-sesion`

**Prueba independiente**: partiendo de una cuenta existente, `POST /api/v1/auth/login` devuelve
`200` con `token` y `expiresAt`, y dos inicios seguidos crean dos filas vigentes en `sessions`.

### Pruebas de la Historia 2 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T040 [P] [US2] Pruebas unitarias de emisión y parseo del JWT en
      `backend/internal/auth/service/token_test.go`: `HS256` con el secreto de `JWT_SECRET`, claims
      `jti` (= `sessions.id`), `sub` (= `users.id`), `exp` (= `sessions.expires_at`) e `iat`
      (= `sessions.created_at`); el vencimiento es **8 horas fijas desde la emisión y no se renueva
      por actividad** (FR-018); un token con firma inválida se rechaza
- [ ] T041 [P] [US2] Pruebas unitarias **obligatorias** de la regla RN-06 en
      `backend/internal/auth/service/auth_service_test.go`: la comparación bcrypt de contraseña y el
      **mensaje genérico idéntico** (`Correo electrónico o contraseña incorrectos.`) tanto para
      correo inexistente como para contraseña incorrecta (FR-014, SC-004)
- [ ] T042 [US2] Pruebas de integración de la historia en
      `backend/internal/auth/delivery/handler_test.go`, una función por escenario precedida por
      `// Escenario: <nombre del escenario> (001/US2)`: `TestUS2_InicioExitoso` (`200` con `token` y
      `expiresAt` a 8 horas), `TestUS2_CorreoConEspaciosYMayusculas`,
      `TestUS2_SesionesSimultaneas` (dos sesiones vigentes que no interfieren, FR-017),
      `TestUS2_CorreoInexistente` y `TestUS2_ContrasenaIncorrecta`, comparando **byte a byte** que
      las dos últimas respuestas sean idénticas (SC-004). Se asserta la fila `login_succeeded` con
      `user_id` y, en los fallos, `login_failed` con `attempted_email` normalizado y `user_id`
      **nulo cuando el correo no existe** (FR-029, SC-009) — depende de T024 y T027
- [ ] T043 [P] [US2] Pruebas de frontend en
      `frontend/src/features/auth/components/login-form.test.tsx` con
      `describe("001/US2 - Inicio de sesión")` y un `it` por escenario — depende de T012

### Implementación de la Historia 2

- [ ] T044 [US2] Implementar la emisión y el parseo del token en
      `backend/internal/auth/service/token.go` con `golang-jwt/jwt/v5`, `HS256` y los claims `jti`,
      `sub`, `exp` e `iat` (data-model.md sección 2) — depende de T015
- [ ] T045 [US2] Añadir la comparación de contraseña con bcrypt en
      `backend/internal/auth/service/password.go` (FR-012)
- [ ] T046 [US2] Implementar `Create(ctx, session, event)` en
      `backend/internal/auth/repository/session_repository.go`: `INSERT sessions` +
      `INSERT auth_events(login_succeeded)` en **una sola transacción**, con
      `expires_at = created_at` más 8 horas (FR-013, FR-018)
- [ ] T047 [US2] Implementar `Create(ctx, event)` en
      `backend/internal/auth/repository/auth_event_repository.go` para el evento `login_failed`
      (FR-029)
- [ ] T048 [US2] Implementar `Login` en `backend/internal/auth/service/auth_service.go`: normaliza
      el correo igual que el registro (FR-012), busca con `FindByEmail`, compara la contraseña,
      crea la `Session`, emite el JWT y devuelve **el mismo error genérico** en los dos caminos de
      fallo, registrando `login_failed` — depende de T044, T045, T046 y T047
- [ ] T049 [US2] Añadir `LoginRequest` (`email`, `password`) y `LoginResponse` (`token`,
      `expiresAt`) a `backend/internal/auth/delivery/dto.go`, el manejador `loginUser` con
      anotaciones `swag` en español en `backend/internal/auth/delivery/handler.go` y la ruta
      **pública** `POST /api/v1/auth/login` en `backend/internal/auth/delivery/routes.go`, con
      `200`, `400` y `401` — depende de T048
- [ ] T050 [US2] Implementar la lectura y escritura del token en `sessionStorage` en
      `frontend/src/lib/session-storage.ts` (research.md sección 9) — depende de T035
- [ ] T051 [US2] Implementar el contexto de sesión en
      `frontend/src/providers/session-provider.tsx`, montarlo en `frontend/src/app/layout.tsx` y
      exponer el acceso al estado en `frontend/src/hooks/use-session.ts` — depende de T050
- [ ] T052 [US2] Implementar la llamada de inicio de sesión en
      `frontend/src/features/auth/api.ts`, el `LoginForm` en
      `frontend/src/features/auth/components/login-form.tsx` y la pantalla
      `frontend/src/app/login/page.tsx` — depende de T051

**Checkpoint**: US1 y US2 funcionan de forma independiente.

---

## Fase 5: Historia de Usuario 3 — Protección de las funciones del sistema (Prioridad: P2)

**Objetivo**: toda funcionalidad distinta de registro e inicio de sesión exige una sesión válida;
sesión ausente, vencida, cerrada o adulterada reciben **exactamente la misma** respuesta `401` y la
acción no produce ningún efecto.

**Rama**: `hu/001-us3-proteccion-de-funciones`

**Prueba independiente**: montar el middleware sobre un manejador de prueba y verificar el rechazo
sin cabecera, con sesión vencida y con token adulterado, más que registro e inicio de sesión sigan
siendo públicos.

### Pruebas de la Historia 3 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [ ] T053 [P] [US3] Pruebas unitarias **obligatorias** de la regla RN-07 en
      `backend/internal/auth/domain/session_test.go`: una sesión es vigente **solo si**
      `revoked_at IS NULL AND expires_at > now()`; `CERRADA` y `VENCIDA` son estados **terminales**
      y una sesión no vuelve a ser vigente (data-model.md sección 2)
- [ ] T054 [P] [US3] Pruebas unitarias **obligatorias** de RN-08 y RN-09 en
      `backend/internal/auth/delivery/routes_test.go`: registro e inicio de sesión son las **únicas**
      rutas públicas y todas las demás exigen sesión (FR-019); la acción se atribuye al titular
      identificado por el claim `sub` (FR-021, RN-09)
- [ ] T055 [US3] Pruebas de integración de la historia en
      `backend/internal/auth/delivery/middleware_test.go`, montando el middleware con `httptest`
      sobre un manejador protegido de prueba, una función por escenario precedida por
      `// Escenario: <nombre del escenario> (001/US3)`: `TestUS3_AccesoConSesionVigente`,
      `TestUS3_RutasPublicas`, `TestUS3_SesionVencida` y `TestUS3_SesionAdulterada`, comparando que
      las respuestas de sesión ausente, vencida y adulterada sean **idénticas** (`401` con
      `Se requiere iniciar sesión.`) y que la acción protegida **no se ejecute ni parcialmente**
      (FR-015, FR-016, FR-020) — depende de T024
- [ ] T056 [P] [US3] Pruebas de frontend en `frontend/src/hooks/use-session.test.tsx` con
      `describe("001/US3 - Protección de las funciones del sistema")`: sin token la guardia redirige
      a `/login`, y un `401` de la API también — depende de T012

### Implementación de la Historia 3

- [ ] T057 [US3] Implementar las reglas de vigencia de la sesión en
      `backend/internal/auth/domain/session.go` como predicado derivado de los datos, sin columna de
      estado (RN-07) — depende de T019
- [ ] T058 [US3] Implementar `FindByID(ctx, id)` en
      `backend/internal/auth/repository/session_repository.go` para la verificación del `jti` en
      cada petición protegida (FR-026) — depende de T046
- [ ] T059 [US3] Implementar `ValidateSession` en
      `backend/internal/auth/service/auth_service.go`: valida la firma y el `exp` del token antes de
      tocar la base, y después exige que la fila exista con `revoked_at` nulo y `expires_at` futuro.
      Fila inexistente, cerrada, vencida y firma inválida se tratan de forma **idéntica** (FR-015,
      RN-07) — depende de T057 y T058
- [ ] T060 [US3] Implementar el middleware de exigencia de sesión en
      `backend/internal/auth/delivery/middleware.go`, dejando en el contexto el titular para la
      atribución (FR-021), y aplicarlo al grupo de rutas protegidas en
      `backend/internal/auth/delivery/routes.go`, manteniendo públicas solo registro e inicio de
      sesión (FR-019) — depende de T059
- [ ] T061 [US3] Implementar la guardia de ruta del lado del cliente en
      `frontend/src/hooks/use-session.ts` y el manejo del `401` en
      `frontend/src/lib/api-client.ts`, que limpia el token y redirige a `/login` con un aviso
      explícito de volver a iniciar sesión (FR-016, research.md sección 9) — depende de T051

**Checkpoint**: US1, US2 y US3 funcionan de forma independiente. La barrera de acceso está activa.

---

## Fase 6: Historia de Usuario 4 — Consulta del propio perfil (Prioridad: P3)

**Objetivo**: una persona con sesión vigente ve su nombre completo y su correo normalizado, y la
respuesta no contiene la contraseña ni nada derivado de ella.

**Rama**: `hu/001-us4-consulta-del-propio-perfil`

**Prueba independiente**: iniciar sesión y pedir `GET /api/v1/users/me`; sin sesión válida responde
`401` sin revelar datos de ninguna cuenta.

### Pruebas de la Historia 4 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [ ] T062 [US4] Pruebas de integración de la historia en
      `backend/internal/auth/delivery/handler_test.go`, una función por escenario precedida por
      `// Escenario: <nombre del escenario> (001/US4)`: `TestUS4_PerfilPropio`,
      `TestUS4_CorreoNormalizadoEnPerfil`, `TestUS4_RespuestaSinContrasena` (inspección del **cuerpo
      completo**: ni `password`, ni `passwordHash`, ni ninguna representación derivada; FR-011,
      SC-005) y `TestUS4_PerfilSinSesion` (`401`, SC-003). El endpoint **no acepta ningún
      identificador como parámetro**: no hay forma de consultar el perfil de otra cuenta (FR-023) —
      depende de T055
- [ ] T063 [P] [US4] Pruebas de frontend en
      `frontend/src/features/auth/components/profile-card.test.tsx` con
      `describe("001/US4 - Consulta del propio perfil")` y un `it` por escenario — depende de T012

### Implementación de la Historia 4

- [ ] T064 [US4] Implementar `FindByID(ctx, id)` en
      `backend/internal/auth/repository/user_repository.go` — depende de T031
- [ ] T065 [US4] Implementar la obtención del perfil del titular en
      `backend/internal/auth/service/auth_service.go`, resolviendo la identidad desde el claim `sub`
      que dejó el middleware (FR-022, FR-023) — depende de T060 y T064
- [ ] T066 [US4] Implementar el manejador `getOwnProfile` en
      `backend/internal/auth/delivery/handler.go` con anotaciones `swag` en español y registrarlo
      como ruta **protegida** `GET /api/v1/users/me` en
      `backend/internal/auth/delivery/routes.go`, devolviendo `200` con `UserProfile` y `401` con la
      respuesta `SesionInvalida` — depende de T065
- [ ] T067 [US4] Implementar la llamada de perfil en `frontend/src/features/auth/api.ts`, el
      `ProfileCard` en `frontend/src/features/auth/components/profile-card.tsx` y la pantalla
      protegida `frontend/src/app/profile/page.tsx` con la guardia de T061 — depende de T061

**Checkpoint**: US1 a US4 funcionan de forma independiente.

---

## Fase 7: Historia de Usuario 5 — Cierre de sesión (Prioridad: P3)

**Objetivo**: cerrar la sesión la invalida **de inmediato** en el sistema, aunque le queden horas de
vigencia y aunque alguien haya copiado el token, sin afectar a las demás sesiones de la misma cuenta.

**Rama**: `hu/001-us5-cierre-de-sesion`

**Prueba independiente**: iniciar sesión, cerrar con `POST /api/v1/auth/logout` y comprobar que el
mismo token responde `401` en una función protegida.

### Pruebas de la Historia 5 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [ ] T068 [US5] Pruebas de integración de la historia en
      `backend/internal/auth/delivery/handler_test.go`, una función por escenario precedida por
      `// Escenario: <nombre del escenario> (001/US5)`: `TestUS5_CierreExitoso` (`204`),
      `TestUS5_NuevoInicioTrasCierre` (crea una **fila nueva** en `sessions`),
      `TestUS5_TokenCerradoNoSirve` (`401` **aunque no hayan pasado las 8 horas**, FR-026),
      `TestUS5_CierreNoAfectaOtrasSesiones` (dos dispositivos, FR-017) y `TestUS5_CierreSinSesion`
      (`401`, sin efecto ni información de ninguna cuenta). Se asserta la fila `logout` con
      `user_id` (FR-028, SC-009) — depende de T062
- [ ] T069 [P] [US5] Pruebas de frontend en
      `frontend/src/features/auth/components/logout-button.test.tsx` con
      `describe("001/US5 - Cierre de sesión")` y un `it` por escenario, incluido que la interfaz
      vuelve al estado de visitante — depende de T012

### Implementación de la Historia 5

- [ ] T070 [US5] Implementar `Revoke(ctx, id, at, event)` en
      `backend/internal/auth/repository/session_repository.go`: `UPDATE sessions.revoked_at` +
      `INSERT auth_events(logout)` en **una sola transacción**, afectando **solo** a esa fila
      (FR-017, FR-026) — depende de T058
- [ ] T071 [US5] Implementar `Logout` en `backend/internal/auth/service/auth_service.go`, revocando
      la sesión del `jti` con el que se hizo la petición, sin tocar `expires_at` (FR-018, FR-024) —
      depende de T070
- [ ] T072 [US5] Implementar el manejador `logoutUser` en
      `backend/internal/auth/delivery/handler.go` con anotaciones `swag` en español y registrarlo
      como ruta **protegida** `POST /api/v1/auth/logout` en
      `backend/internal/auth/delivery/routes.go`, devolviendo `204` y `401` — depende de T071
- [ ] T073 [US5] Implementar la llamada de cierre en `frontend/src/features/auth/api.ts`, el
      `LogoutButton` en `frontend/src/features/auth/components/logout-button.tsx` montado en
      `frontend/src/app/profile/page.tsx`, y la limpieza del token en
      `frontend/src/lib/session-storage.ts` con el estado de visitante en el proveedor de sesión —
      depende de T067

**Checkpoint**: las cinco historias funcionan de forma independiente.

---

## Fase 8: Polish (Cierre y asuntos transversales)

**Propósito**: documentación viva, contenerización, cobertura y verificación de las puertas de
calidad. Todas las tareas son **técnicas**.

**Rama**: `fase/001-polish`

- [ ] T074 Generar la documentación con `swag init -g cmd/app/main.go -o docs` y verificar que
      `backend/docs/` coincida con [contracts/openapi.yaml](./contracts/openapi.yaml) en rutas,
      códigos de respuesta y esquemas (Principio VIII); corregir las anotaciones si difieren —
      depende de T072
- [ ] T075 [P] Crear `backend/Dockerfile` con una versión estable de la imagen base de Go, que
      ejecute `swag init` durante la construcción para que `backend/docs/` no haga falta versionado
      (research.md sección 10) — depende de T074
- [ ] T076 [P] Crear `frontend/Dockerfile` con una versión estable de la imagen base de Node.js
      (Principio V)
- [ ] T077 [P] Documentar y verificar los comandos de cobertura exigidos por el Principio II:
      `go test ./... -coverprofile=coverage.out` más `go tool cover` en el backend, y el script
      `test:coverage` de `frontend/package.json` en el frontend
- [ ] T078 Ejecutar `govulncheck ./...` en `backend/` y resolver o justificar por escrito cada
      vulnerabilidad reportada
- [ ] T079 Actualizar `README.md` de la raíz en español con la sección introductoria del proyecto y
      el paso a paso completo de ejecución, incluido el paso de `swag init` que un clon recién hecho
      necesita para compilar. Criterio de aceptación: alguien que no conoce el proyecto puede
      ejecutarlo siguiendo el documento (Principio VIII) — depende de T074
- [ ] T080 Revisar las puertas de calidad en `.gitignore` y en los archivos de entorno: las
      secciones `#Frontend` y `#Backend` excluyen `.env`, `node_modules/`, `.next/`,
      `backend/docs/`, `coverage.out` y `coverage.html`; cada `.env.example` está sincronizado con su
      `.env`; no hay secretos versionados; no existe ningún `AGENTS.md` (Principios VI y IX)
- [ ] T081 Ejecutar de punta a punta la [guía de validación](./quickstart.md) completa, incluidas la
      comprobación de `auth_events` en la base y la verificación de que las pruebas corrieron contra
      `smye_test` en el puerto 5433 y no contra desarrollo — depende de T079
- [ ] T082 [P] Verificar la trazabilidad y las puertas de seguridad recorriendo
      `backend/internal/auth/` y `frontend/src/features/auth/`: que buscar `001/US1` a `001/US5` en
      el código de pruebas devuelva los **26** escenarios; que las **9** reglas RN-01 a RN-09 tengan
      prueba (**SC-008**); que **cada** endpoint protegido tenga su prueba de rechazo sin sesión —
      `GET /api/v1/users/me` y `POST /api/v1/auth/logout` (**SC-003**); que las respuestas de correo
      inexistente y de contraseña incorrecta sean idénticas byte a byte (**SC-004**); y que ninguna
      salida de la feature —respuestas HTTP, mensajes de error, pantallas ni filas de `auth_events`—
      contenga la contraseña ni ninguna representación derivada de ella (**SC-005**) — depende
      de T073

> **Nota sobre la numeración**: T082 verifica el trabajo de las historias, así que
> conceptualmente va antes de T081, pero se numeró al final a propósito. Los 81 issues de
> `T001`–`T081` ya están publicados en GitHub y renumerar los dejaría desalineados; agregar al final
> solo suma un issue nuevo.

---

## Fase 9: Convergence

**Propósito**: cerrar las brechas que `/speckit-converge` encontró entre lo que piden la spec, el
plan y la constitución y lo que ya está implementado en las fases 1 y 2. Todas las tareas son
**técnicas** y no tienen prueba asociada.

**Rama**: cada tarea se hace en la rama del trabajo al que corrige: T083 en
`hu/001-us2-inicio-de-sesion` y T084 en `fase/001-foundational`.

- [X] T083 Fijar `github.com/golang-jwt/jwt/v5 v5.3.1` en `backend/go.mod` con
      `go get github.com/golang-jwt/jwt/v5@v5.3.1` y verificar con
      `go list -m github.com/golang-jwt/jwt/v5` que resuelve `v5.3.1`, porque `go mod tidy` lo quitó
      en T023 al no haber todavía ningún import (research.md sección 1, T006). Se hace **antes de
      T040**, la primera tarea que importa el paquete, para que ese import tome la versión fijada y
      no la última publicada — depende de T023
- [ ] T084 Corregir el comentario de `OpenDatabase` en
      `backend/internal/platform/testsupport/database.go`: sobra el primer "abre" y una línea supera
      el ancho del resto del archivo (Principio VIII) — depende de T024

**Checkpoint**: `go list -m github.com/golang-jwt/jwt/v5` responde `v5.3.1` y el comentario del
ayudante de pruebas describe exactamente lo que hace la función.

---

## Dependencias y Orden de Ejecución

### Dependencias entre fases

- **Fase 1 Setup**: sin dependencias. Puede empezar de inmediato.
- **Fase 2 Foundational**: depende de la Fase 1. **BLOQUEA** todas las historias.
- **Fases 3 a 7 (historias)**: todas dependen de la Fase 2.
  - Pueden avanzar en paralelo si hay personas disponibles, o en orden de prioridad
    (P1 → P1 → P2 → P3 → P3).
- **Fase 8 Polish**: depende de que estén completas las historias que se quieran entregar.

### Dependencias entre historias

- **US1 Registro (P1)**: puede empezar después de la Fase 2. Sin dependencias de otras historias.
- **US2 Inicio de sesión (P1)**: puede empezar después de la Fase 2. Sus pruebas de integración
  necesitan una cuenta, que puede crearse en la propia preparación de la prueba sin depender del
  endpoint de US1.
- **US3 Protección (P2)**: puede empezar después de la Fase 2. Sus pruebas montan el middleware
  sobre un manejador de prueba, así que **no** dependen de que exista un endpoint protegido real.
- **US4 Perfil (P3)**: necesita el middleware de US3 (T060) para su ruta protegida.
- **US5 Cierre de sesión (P3)**: necesita el middleware de US3 (T060) y la creación de sesiones de
  US2 (T046).

### Dentro de cada historia

- Las pruebas se escriben y **fallan** antes de la implementación (Principio II).
- Dominio antes que repositorio; repositorio antes que servicio; servicio antes que `delivery`.
- Backend antes que la pantalla de frontend que lo consume.
- Una historia se termina antes de pasar a la siguiente prioridad.

### Oportunidades de paralelismo

- Fase 1: T002, T003 y T005 en paralelo; T007, T011, T013 y T014 en paralelo.
- Fase 2: T018, T019, T020 y T021 en paralelo (archivos distintos del mismo paquete `domain`).
- Dentro de cada historia: las pruebas unitarias y las de frontend marcadas `[P]` en paralelo entre
  sí; las de integración del mismo `handler_test.go` **no** son paralelizables entre historias
  porque comparten archivo.
- Fase 8: T075, T076, T077 y T082 en paralelo.
- Con varias personas: una por historia después de la Fase 2.

---

## Ejemplo de paralelismo: Historia de Usuario 1

```text
# Las tres tareas de pruebas marcadas [P] pueden escribirse a la vez:
T025  backend/internal/auth/delivery/validation_test.go
T026  backend/internal/auth/service/password_test.go
T028  frontend/src/features/auth/components/register-form.test.tsx

# T027 (handler_test.go) va aparte: ese archivo lo comparten US1, US2, US4 y US5.

# En la implementación, estas dos no se pisan:
T035  frontend/src/types/auth.ts + frontend/src/config/env.ts
T030  backend/internal/auth/service/password.go
```

---

## Estrategia de Implementación

### MVP primero (solo US1)

1. Completar la Fase 1 (Setup).
2. Completar la Fase 2 (Foundational) — **CRÍTICO**: bloquea todas las historias.
3. Completar la Fase 3 (US1 Registro).
4. **DETENERSE y VALIDAR**: probar US1 de forma independiente con la guía de validación.
5. Abrir el pull request de la historia con `/redactar-pr` y demostrar el incremento.

### Entrega incremental

1. Setup + Foundational → base lista (dos fases técnicas, dos pull requests).
2. US1 Registro → probar sola → entregar (**MVP**).
3. US2 Inicio de sesión → probar sola → entregar.
4. US3 Protección → probar sola → entregar.
5. US4 Perfil → probar sola → entregar.
6. US5 Cierre de sesión → probar sola → entregar.
7. Polish → cerrar la feature.

Cada historia añade valor sin romper las anteriores.

### Estrategia con varias personas

1. El equipo completa Setup y Foundational en conjunto.
2. Terminada la Fase 2: una persona por historia, con la salvedad de que US4 y US5 esperan a que
   T060 (middleware) esté en `main`.
3. Las historias se integran de forma independiente, cada una por su pull request hacia `main`.

---

## Notas

- Las tareas `[P]` usan archivos distintos y no tienen dependencias pendientes.
- La etiqueta `[USn]` traza la tarea hasta su historia; buscar `001/US1` a `001/US5` en el código de
  pruebas devuelve la trazabilidad completa exigida por el Principio II.
- Cada tarea se ejecuta con `/speckit-implement Txxx`, una por invocación.
- Cada commit corresponde a una sola tarea y lleva el pie `Spec: 001/Txxx` más `Closes #N` o
  `Refs #N` (Principio IX, ámbito b). Los commits los registra **una persona**, nunca un agente.
- Las pruebas corren siempre contra `smye_test` en el puerto 5433, jamás contra desarrollo
  (Principio V).
- Tres puntos del plan esperan decisión del desarrollador y conviene resolverlos antes de las tareas
  que los tocan: la instalación de Go (T001), el límite de 72 **bytes** de la contraseña frente a
  RN-04 expresado en caracteres (T025, T029) y el transporte del token en `sessionStorage` frente a
  una cookie `HttpOnly` (T050), que conviene que evalúe `/revisar-seguridad`.
- **El límite de la contraseña bloquea a T025.** FR-006 y RN-04 lo expresan en caracteres y el
  diseño lo aplica en bytes de UTF-8: para una contraseña con acentos los dos límites no coinciden
  (40 caracteres acentuados son 80 bytes). Hay que decidirlo **antes** de escribir T025, porque la
  prueba y la implementación se escriben contra una regla o contra la otra. Si se confirma el límite
  en bytes, hay que enmendar FR-006 y RN-04 en la spec, que es la fuente de verdad.
- **Criterios de éxito no automatizables.** SC-001 (registro en menos de 2 minutos, 95 % al primer
  intento), SC-002 (inicio de sesión y pantalla protegida en menos de 30 segundos) y SC-007 (volver
  a estar operativo en menos de 1 minuto tras el vencimiento) son métricas de experiencia de uso: no
  tienen tarea asociada porque ninguna prueba automatizada puede hacerlas fallar. Se validan a mano
  siguiendo la sección "Flujo en la interfaz" de [quickstart.md](./quickstart.md).
