# Guía de Validación (Fase 1): Autenticación y Cuentas de Usuario

**Feature**: `specs/001-user-auth` | **Fecha**: 2026-10-05

Cómo levantar el entorno y comprobar de punta a punta que la feature funciona. Los detalles de
diseño están en [research.md](./research.md), [data-model.md](./data-model.md) y
[contracts/openapi.yaml](./contracts/openapi.yaml).

> Este documento es una guía de ejecución y validación. El código de implementación no va aquí: las
> tareas se generan con `/speckit-tasks` y se implementan siguiendo el ciclo TDD del Principio II.

---

## 1. Prerrequisitos

| Herramienta | Versión exigida | Estado en esta máquina (2026-10-05) |
| --- | --- | --- |
| Go | 1.27.1 | **Falta instalar** |
| Node.js | 24.x | v24.21.0 |
| npm | 11.x | 11.4.2 |
| Docker | 29.x | 29.7.2 |
| Docker Compose | v5.x | v5.5.1 |

Herramientas de Go que se instalan una vez (fuera del módulo, con `go install`):

```powershell
go install github.com/swaggo/swag/cmd/swag@v1.16.6
go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
```

---

## 2. Variables de entorno

Los archivos de ejemplo se versionan; los que llevan valores reales, **no** (Principio VI).

| Archivo | Se versiona | Para qué |
| --- | --- | --- |
| `backend/postgres/.env.example` | sí | Plantilla del servicio PostgreSQL |
| `backend/postgres/.env` | no | PostgreSQL de desarrollo (puerto 5432, base `smye_dev`) |
| `backend/postgres/.env.test` | no | PostgreSQL de test (puerto 5433, base `smye_test`) |
| `backend/.env.example` | sí | Plantilla de la aplicación backend |
| `backend/.env` | no | Backend en desarrollo |
| `backend/.env.test` | no | Backend en pruebas |
| `frontend/.env.example` | sí | Plantilla del frontend |
| `frontend/.env.local` | no | Frontend en desarrollo |
| `frontend/.env.test` | no | Frontend en pruebas |

Claves que debe contener cada plantilla:

```text
# backend/postgres/.env.example
POSTGRES_USER=smye
POSTGRES_PASSWORD=cambiar_este_valor
POSTGRES_DB=smye_dev
POSTGRES_HOST_PORT=5432

# backend/.env.example
DB_HOST=localhost
DB_PORT=5432
DB_USER=smye
DB_PASSWORD=cambiar_este_valor
DB_NAME=smye_dev
JWT_SECRET=cambiar_por_un_secreto_de_al_menos_32_bytes
SERVER_PORT=8080
CORS_ALLOWED_ORIGIN=http://localhost:3000

# frontend/.env.example
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
```

`JWT_SECRET` debe tener al menos 32 bytes: si falta o es más corto, el backend no arranca y lo
informa (ver [research.md](./research.md), sección 2).

Primera puesta en marcha:

```powershell
Copy-Item backend/postgres/.env.example backend/postgres/.env
Copy-Item backend/postgres/.env.example backend/postgres/.env.test
Copy-Item backend/.env.example backend/.env
Copy-Item backend/.env.example backend/.env.test
Copy-Item frontend/.env.example frontend/.env.local
```

Después hay que editar los archivos de test para apuntar al entorno de test, que es **disjunto**
del de desarrollo (Principio V):

- `backend/postgres/.env.test`: `POSTGRES_DB=smye_test` y `POSTGRES_HOST_PORT=5433`
- `backend/.env.test`: `DB_PORT=5433` y `DB_NAME=smye_test`

---

## 3. Levantar los servicios

Los dos entornos usan el **mismo** `docker-compose.yml` con archivos de entorno y nombres de
proyecto distintos, lo que da contenedores, volúmenes y puertos separados.

```powershell
cd backend/postgres

# Desarrollo (puerto 5432, base smye_dev)
docker compose --env-file .env -p smye-dev up -d

# Test (puerto 5433, base smye_test)
docker compose --env-file .env.test -p smye-test up -d

# Comprobar que ambos están arriba y en puertos distintos
docker compose -p smye-dev ps
docker compose -p smye-test ps
```

**Comprobación de aislamiento** — las pruebas nunca deben tocar la base de desarrollo:

```powershell
docker exec smye-test-postgres-1 psql -U smye -d smye_test -c "SELECT current_database(), inet_server_port();"
```

Debe responder `smye_test`. Si responde `smye_dev`, el entorno está mal configurado y hay que
detenerse antes de ejecutar nada.

---

## 4. Ejecutar las pruebas

### Backend

```powershell
cd backend

# Swagger se genera antes de compilar: backend/docs/ no está versionado
swag init -g cmd/app/main.go -o docs

# Todas las pruebas contra la base de TEST
go test ./...

# Solo las pruebas de una historia (trazabilidad del Principio II)
go test ./... -run TestUS1
go test ./... -run TestUS5

# Reporte de cobertura
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
go tool cover -func=coverage.out
```

Las pruebas de integración cargan `backend/.env.test` mediante el ayudante de
`backend/internal/platform/testsupport/` y
vacían las tres tablas antes de cada caso. **No** leen `backend/.env`.

### Frontend

```powershell
cd frontend

npm test                   # Vitest una vez
npm run test:watch         # Vitest en modo observación
npm run test:coverage      # Reporte de cobertura
npm run lint               # ESLint
npx tsc --noEmit           # Comprobación de tipos (prohibido `any`)
```

### Vulnerabilidades

```powershell
cd backend
govulncheck ./...
```

---

## 5. Levantar la aplicación

```powershell
# Terminal 1 — backend en http://localhost:8080
cd backend
swag init -g cmd/app/main.go -o docs
go run ./cmd/app

# Terminal 2 — frontend en http://localhost:3000
cd frontend
npm run dev
```

Swagger queda publicado en <http://localhost:8080/swagger/index.html>.

Alternativa contenerizada (cada aplicación tiene su `Dockerfile`, y el del backend ejecuta
`swag init` durante la construcción):

```powershell
docker build -t smye-backend ./backend
docker build -t smye-frontend ./frontend
```

---

## 6. Escenarios de validación de punta a punta

Cada bloque corresponde a una historia de la [spec](./spec.md). Las pruebas automatizadas que los
cubren se nombran según el Principio II (`TestUSn_<Escenario>` en Go,
`describe("001/USn - ...")` en Vitest), de modo que buscar `001/US1` en el código de pruebas
devuelve la trazabilidad completa.

Las comprobaciones manuales de abajo usan `curl` para no depender del frontend. En PowerShell
conviene usar `curl.exe` explícitamente.

### US1 — Registro de cuenta

```powershell
# Alta exitosa -> 201 con id, fullName y email; sin token (FR-009)
curl.exe -i -X POST http://localhost:8080/api/v1/auth/register `
  -H "Content-Type: application/json" `
  -d '{"fullName":"Ana Perez","email":"ana@example.com","password":"Proyecto2026"}'

# Correo con espacios y mayúsculas -> 201, se guarda normalizado (FR-004)
curl.exe -i -X POST http://localhost:8080/api/v1/auth/register `
  -H "Content-Type: application/json" `
  -d '{"fullName":"Beto Gomez","email":"  Beto@Example.COM  ","password":"Proyecto2026"}'

# Correo ya registrado con otra capitalización -> 409 (FR-005)
curl.exe -i -X POST http://localhost:8080/api/v1/auth/register `
  -H "Content-Type: application/json" `
  -d '{"fullName":"Otra Ana","email":"ANA@example.com","password":"Proyecto2026"}'

# Entrada inválida -> 400 con el detalle por campo, sin crear nada (FR-008)
curl.exe -i -X POST http://localhost:8080/api/v1/auth/register `
  -H "Content-Type: application/json" `
  -d '{"fullName":"A","email":"no-es-un-correo","password":"corta"}'
```

| Resultado esperado | Requisito |
| --- | --- |
| `201` con `id`, `fullName` y `email`; **ningún token en la respuesta** | FR-001, FR-009 |
| El correo guardado es `beto@example.com`, sin espacios y en minúsculas | FR-004 |
| `409` con "El correo electrónico no está disponible" | FR-005 |
| `400` con una entrada por cada campo inválido y **sin la contraseña en el mensaje** | FR-008, FR-011 |
| Contraseñas de exactamente 8 y de exactamente 72 caracteres: aceptadas | Casos límite |
| Contraseñas de 7 y de 73 caracteres, o solo letras, o solo números: rechazadas | FR-006 |
| Nombres de exactamente 2 y de exactamente 100 caracteres: aceptados | FR-002 |
| Nombre compuesto solo por espacios: rechazado como ausente | FR-002 |

**Concurrencia (FR-007, SC-006)** — 10 solicitudes simultáneas con el mismo correo deben dejar
exactamente una cuenta. Es la verificación que cubre la prueba de integración
`TestUS1_RegistroConcurrente`; manualmente se comprueba contando filas:

```powershell
docker exec smye-dev-postgres-1 psql -U smye -d smye_dev -c `
  "SELECT email, count(*) FROM users GROUP BY email HAVING count(*) > 1;"
```

Debe devolver cero filas.

### US2 — Inicio de sesión

```powershell
# Credenciales correctas -> 200 con token y expiresAt
curl.exe -i -X POST http://localhost:8080/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d '{"email":"ana@example.com","password":"Proyecto2026"}'

# Correo con espacios y mayúsculas -> 200 igual (FR-012)
curl.exe -i -X POST http://localhost:8080/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d '{"email":"  ANA@Example.com  ","password":"Proyecto2026"}'

# Correo inexistente -> 401
curl.exe -i -X POST http://localhost:8080/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d '{"email":"nadie@example.com","password":"CualquierCosa1"}'

# Contraseña incorrecta -> 401
curl.exe -i -X POST http://localhost:8080/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d '{"email":"ana@example.com","password":"Incorrecta123"}'
```

| Resultado esperado | Requisito |
| --- | --- |
| `200` con `token` y `expiresAt` 8 horas después de la emisión | FR-013, FR-018 |
| El mensaje de los dos últimos casos es **byte a byte idéntico** | FR-014, RN-06, SC-004 |
| Dos inicios de sesión seguidos dan dos tokens distintos y **ambos válidos** | FR-017 |

La comprobación de SC-004 es el corazón de la historia: comparar los dos cuerpos de respuesta y
verificar que son iguales.

### US3 — Protección de las funciones del sistema

```powershell
$token = "<token obtenido en US2>"

# Con sesión vigente -> 200
curl.exe -i http://localhost:8080/api/v1/users/me -H "Authorization: Bearer $token"

# Sin cabecera -> 401
curl.exe -i http://localhost:8080/api/v1/users/me

# Token adulterado -> 401, idéntico al caso anterior
curl.exe -i http://localhost:8080/api/v1/users/me -H "Authorization: Bearer no.es.un.token"

# Registro e inicio de sesión siguen siendo públicos
curl.exe -i -X POST http://localhost:8080/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d '{"email":"ana@example.com","password":"Proyecto2026"}'
```

| Resultado esperado | Requisito |
| --- | --- |
| `401` con "Se requiere iniciar sesión" para ausente, vencida, cerrada y adulterada | FR-015, FR-016, RN-07 |
| La acción rechazada **no produce ningún efecto**, ni parcial | FR-020 |
| Solo registro e inicio de sesión responden sin sesión | FR-019, RN-08 |

**Sesión vencida**: no se espera 8 horas. La prueba de integración adelanta `expires_at` de la fila
`sessions` a un momento pasado y repite la petición, que debe responder `401`.

### US4 — Consulta del propio perfil

```powershell
curl.exe -s http://localhost:8080/api/v1/users/me -H "Authorization: Bearer $token"
```

| Resultado esperado | Requisito |
| --- | --- |
| `200` con `id`, `fullName` y `email` del titular de la sesión | FR-022 |
| El correo aparece normalizado, igual al que se usa para iniciar sesión | FR-004, US4-2 |
| **La respuesta completa no contiene** `password`, `passwordHash`, `hash` ni nada derivado | FR-011, SC-005 |
| No hay ningún parámetro para pedir el perfil de otra cuenta | FR-023 |

### US5 — Cierre de sesión

```powershell
# Cierre -> 204
curl.exe -i -X POST http://localhost:8080/api/v1/auth/logout -H "Authorization: Bearer $token"

# El mismo token ya no sirve, aunque no hayan pasado las 8 horas -> 401
curl.exe -i http://localhost:8080/api/v1/users/me -H "Authorization: Bearer $token"

# Visitante sin sesión pidiendo cerrar -> 401, sin efecto ni información
curl.exe -i -X POST http://localhost:8080/api/v1/auth/logout
```

| Resultado esperado | Requisito |
| --- | --- |
| `204` y la sesión queda invalidada **de inmediato** | FR-024, FR-026 |
| El token cerrado responde `401` desde cualquier dispositivo | FR-025, US5-3 |
| Con sesiones en dos dispositivos, cerrar una **no** afecta a la otra | FR-017, US5-4 |
| Volver a iniciar sesión da una sesión nueva y vigente | US5-2 |
| El cierre sin sesión no revela nada de ninguna cuenta | US5-5 |

### Registro de eventos de autenticación (FR-028, SC-009)

No hay pantalla ni endpoint de consulta (FR-031): se verifica contra la base de datos.

```powershell
docker exec smye-dev-postgres-1 psql -U smye -d smye_dev -c `
  "SELECT event_type, user_id, attempted_email, created_at FROM auth_events ORDER BY created_at;"
```

| Resultado esperado | Requisito |
| --- | --- |
| Un `account_created` por cada alta, con `user_id` | FR-028 |
| Un `login_succeeded` por cada inicio exitoso, con `user_id` | FR-028 |
| Un `login_failed` por cada intento fallido | FR-028 |
| En el fallido contra un correo no registrado: `user_id` nulo y `attempted_email` con el correo | FR-029 |
| Un `logout` por cada cierre, con `user_id` | FR-028 |
| **Ninguna columna contiene la contraseña ni nada derivado de ella** | FR-030, SC-005 |

### Flujo en la interfaz

1. Abrir <http://localhost:3000/register> y crear una cuenta. Se confirma el alta y **no** queda la
   sesión iniciada: entrar a `/profile` redirige a `/login` (FR-009, escenario US1-6).
2. Iniciar sesión en <http://localhost:3000/login> con esas credenciales y llegar a `/profile`
   (SC-002: menos de 30 segundos).
3. El formulario de registro muestra el límite de 8 a 72 caracteres **antes** de enviar, no al
   fallar (RC-02).
4. Cerrar sesión desde `/profile` (FR-024): la interfaz vuelve al estado de visitante y `/profile`
   redirige a `/login`.
5. Refrescar `/profile` con sesión vigente mantiene la sesión; abrir una pestaña nueva del mismo
   navegador exige iniciar sesión, porque el token vive en `sessionStorage`.

---

## 7. Puertas de calidad antes de dar por terminada la feature

De las puertas de calidad de la constitución, aplicadas a esta feature:

- [ ] Cada prueba se escribió antes de su código de producción y ahora pasa (Principio II).
- [ ] Las pruebas corrieron contra `smye_test` en el puerto 5433, no contra desarrollo.
- [ ] Hay una prueba nombrada por cada escenario de la spec; buscar `001/US1` a `001/US5` en el
      código de pruebas devuelve la trazabilidad completa.
- [ ] Hay pruebas unitarias de las validaciones de entrada y de las reglas de negocio RN-01 a RN-09
      (SC-008).
- [ ] No hay secretos fuera de los `.env`; cada `.env.example` está sincronizado con su `.env`.
- [ ] No hay `any` en el frontend ni errores de Go sin manejar en el backend.
- [ ] Cada bloque nuevo tiene su comentario en español y los identificadores están en inglés.
- [ ] Swagger publicado coincide con [contracts/openapi.yaml](./contracts/openapi.yaml).
- [ ] El `README.md` de la raíz permite ejecutar el proyecto a alguien que no lo conoce, incluido el
      paso de `swag init`.
- [ ] El `.gitignore` tiene secciones `#Frontend` y `#Backend` y excluye `.env`, `node_modules/`,
      `.next/`, `backend/docs/`, `coverage.out` y `coverage.html`.
- [ ] `govulncheck ./...` no reporta vulnerabilidades conocidas.
- [ ] No se generó ningún `AGENTS.md` (Principio IX, ámbito f).
- [ ] Se ejecutaron los comandos de cobertura de backend y de frontend.
