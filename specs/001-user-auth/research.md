# Investigación (Fase 0): Autenticación y Cuentas de Usuario

**Feature**: `specs/001-user-auth` | **Fecha**: 2026-10-05 | **Spec**: [spec.md](./spec.md)

Este documento resuelve las incógnitas técnicas de la [spec](./spec.md) antes del diseño. Cada
entrada registra la decisión, su justificación y las alternativas descartadas. Las versiones se
consultaron contra `proxy.golang.org`, `registry.npmjs.org`, `go.dev/dl` y Docker Hub.

---

## 0. Contexto: esta es la primera feature del repositorio

El repositorio contiene hoy solo `specs/`, `.specify/`, `.claude/`, `.github/`, `CLAUDE.md` y
`README.md`. No existen `backend/` ni `frontend/`. Por lo tanto esta feature incluye el arranque del
proyecto: módulo de Go, aplicación Next.js, PostgreSQL de desarrollo y de test, Dockerfiles y
Swagger. Ese trabajo de infraestructura es condición necesaria para la primera prueba de integración
y no es alcance añadido (Principio I): sin él no hay dónde ejecutar US1.

### Prerrequisitos faltantes en la máquina (bloqueantes)

| Prerrequisito | Estado verificado | Acción requerida |
| --- | --- | --- |
| Go | **No instalado** | Instalar Go 1.27.1 antes de la primera tarea de backend |
| Node.js | v24.21.0 | — |
| npm | 11.4.2 | — |
| Docker | 29.7.2 | — |
| Docker Compose | v5.5.1 | — |

---

## 1. Versiones del stack

### Decisión

| Ámbito | Dependencia | Versión fijada |
| --- | --- | --- |
| Runtime backend | Go | `1.27.1` |
| Framework backend | github.com/gin-gonic/gin | `v1.12.0` |
| ORM | gorm.io/gorm | `v1.31.2` |
| Driver PostgreSQL | gorm.io/driver/postgres | `v1.6.3` |
| JWT | github.com/golang-jwt/jwt/v5 | `v5.3.1` |
| bcrypt | golang.org/x/crypto | `v0.57.0` |
| UUID | github.com/google/uuid | `v1.6.0` |
| CORS | github.com/gin-contrib/cors | `v1.7.9` |
| Swagger (CLI) | github.com/swaggo/swag | `v1.16.6` |
| Swagger (Gin) | github.com/swaggo/gin-swagger | `v1.6.1` |
| Swagger (assets) | github.com/swaggo/files | `v1.0.1` |
| Pruebas backend | github.com/stretchr/testify | `v1.12.1` |
| Vulnerabilidades | golang.org/x/vuln/cmd/govulncheck | `v1.8.0` |
| Base de datos | Imagen `postgres` | `18.6-alpine` |
| Framework frontend | next | `16.3.8` |
| UI | react / react-dom | `19.3.0` |
| Estilos | tailwindcss | `4.3.3` |
| Formateo frontend | prettier | `3.9.9` |
| Pruebas frontend | vitest | `5.0.3` |
| Pruebas frontend | @vitejs/plugin-react | `6.1.1` |
| Pruebas frontend | jsdom | `30.1.2` |
| Pruebas frontend | @testing-library/react | `16.3.3` |
| Pruebas frontend | @testing-library/dom | `10.4.2` |
| Pruebas frontend | @testing-library/jest-dom | `7.0.1` |
| Pruebas frontend | vite-tsconfig-paths | `6.1.1` |

**Rationale**: todas son versiones estables publicadas, sin etiquetas alpha, beta, rc ni móviles
(Principio IV). La imagen de PostgreSQL se fija con patch exacto (`18.6-alpine`) y no como `18`,
`18-alpine` ni `latest`, porque los dos primeros son etiquetas móviles.

**Alternativas consideradas**:

- PostgreSQL `17.11-alpine`: descartada; 18 es la rama estable vigente y no hay requisito que ate a 17.
- Imagen `postgres:18.6` (Debian) en lugar de Alpine: descartada por tamaño; no se usa ninguna
  extensión que requiera la variante Debian.

### TypeScript: no se adopta la rama 7.x en esta feature

**Decisión**: usar la versión de TypeScript que fije `create-next-app@16.3.8` (rama 5.x). No se
actualiza manualmente a `typescript@7.0.2`, que es la última publicada.

**Rationale**: TypeScript 7 es la reimplementación nativa del compilador; adoptarla fuera de la
versión que Next.js 16.3.8 declara soportada introduce un riesgo de incompatibilidad sin ningún
requisito que lo justifique (Principio I). La prohibición de `any` (Principio VIII) se cumple igual
en 5.x.

**Alternativas consideradas**: fijar `typescript@7.0.2` — descartada por el riesgo descrito.

### Ninguna dependencia requiere autorización nueva

Todas las dependencias listadas están en la tabla de dependencias autorizadas del Principio IV.
`@testing-library/jest-dom` y `vite-tsconfig-paths` quedan cubiertos por la cláusula "y los paquetes
que indique la guía oficial de Next.js para Vitest". **No se solicita ninguna autorización
adicional.** Si al leer la guía oficial de Vitest de la versión instalada apareciera un paquete
fuera de esa lista, el trabajo se detiene y se pide autorización (Principio IV, paso 6 del flujo).

---

## 2. Invalidación inmediata de sesión con JWT (FR-026 + FR-017)

### El conflicto

El Principio IV obliga a usar JWT. Un JWT es autovalidante: cualquier portador con firma válida y
sin vencer obtiene acceso. Pero FR-026 exige que el cierre de sesión **invalide esa sesión de
inmediato en el sistema**, incluso si alguien la copió antes, y FR-017 exige que las demás sesiones
de la misma cuenta sigan vigentes. Un JWT puramente sin estado no puede cumplir FR-026.

### Decisión

**JWT con identificador de sesión persistido (`jti`) respaldado por una tabla `sessions`.**

1. El inicio de sesión crea una fila en `sessions` con un UUID propio y emite un JWT cuyo claim
   `jti` es ese UUID, `sub` es el ID del usuario y `exp` es la emisión más 8 horas.
2. El middleware de autorización valida firma y `exp`, y a continuación lee la fila `sessions` por
   `jti`. Rechaza si la fila no existe, si `revoked_at` no es nulo o si `expires_at` ya pasó.
3. El cierre de sesión marca `revoked_at` en esa única fila. Las demás filas de la cuenta no se
   tocan (FR-017).

**Rationale**:

- Es la única forma de cumplir FR-026 sin abandonar JWT.
- Reproduce exactamente la entidad **Sesión** que la spec declara en "Entidades Clave" (usuario,
  momento de emisión, momento de vencimiento, si fue cerrada por su titular). El modelo de datos no
  inventa nada.
- Unifica los cuatro casos de FR-015 (vencida, cerrada, ausente, adulterada) en una sola decisión
  del middleware: cualquiera de ellos devuelve la misma respuesta.

**Costo aceptado**: una lectura a la base de datos por cada petición protegida. No se añade caché
ni almacén en memoria (Principio I); si el volumen lo exigiera, se tratará como feature propia.

**Alternativas consideradas**:

- **JWT sin estado y sin revocación**: descartada, incumple FR-026.
- **Lista de denegación de `jti` revocados**: cumple FR-026 con menos escrituras, pero necesita una
  tarea de limpieza de entradas vencidas y no modela la entidad Sesión de la spec, que pide también
  el momento de emisión y el de vencimiento. Más piezas móviles para menos información (KISS).
- **Tokens de refresco con acceso de vida corta**: la ventana de validez residual contradice el
  "de inmediato" de FR-026, y los refresh tokens están fuera del alcance de la spec.

### Algoritmo y secreto de firma

**Decisión**: `HS256` con un secreto leído de `JWT_SECRET` (Principio VI). Longitud mínima exigida
al arrancar: 32 bytes; si falta o es más corto, la aplicación no arranca y lo informa.

**Rationale**: HS256 es simétrico y el emisor y el validador son el mismo servicio, así que no hay
caso de uso para claves asimétricas (YAGNI). Validar el secreto al arrancar evita descubrir en
producción que se está firmando con un valor vacío.

**Alternativas consideradas**: RS256 o ES256 — descartadas, no hay verificadores externos.

---

## 3. Normalización y unicidad del correo (FR-004, FR-005, FR-007)

### Decisión

- Se almacena **una sola** columna `email` con el valor ya normalizado: espacios extremos recortados
  (`strings.TrimSpace`) y pasado a minúsculas (`strings.ToLower`).
- La normalización ocurre en la capa de validación de entrada, antes de la capa de servicio
  (Principio VII), tanto en registro como en inicio de sesión.
- La unicidad la garantiza un **índice único de base de datos** sobre `email`, no una consulta
  previa.
- GORM se configura con `TranslateError: true`, de modo que la violación del índice único llega al
  servicio como `gorm.ErrDuplicatedKey` y se traduce a la respuesta "el correo no está disponible".

**Rationale**:

- Una sola columna normalizada satisface a la vez FR-005 (unicidad), FR-012 (comparación al iniciar
  sesión) y el escenario US4-2 (el perfil muestra el correo tal como quedó normalizado). No hace
  falta guardar además el correo tal como se tecleó: ningún requisito lo pide (Principio I).
- El índice único es la **única** defensa correcta contra FR-007 (doble envío simultáneo): un
  "consultar y después insertar" tiene una ventana de carrera entre ambas operaciones y permitiría
  las dos cuentas. Delegar en la restricción de la base de datos y traducir el error es lo que hace
  que SC-006 (cero duplicados con 10 solicitudes simultáneas) sea cierto y no probable.
- `TranslateError` de GORM evita acoplar la capa de servicio al código de error `23505` de
  PostgreSQL, respetando la dependencia hacia el dominio (Principio III).

**Alternativas consideradas**:

- **Verificar existencia y luego insertar**: descartada, pierde FR-007 por condición de carrera.
- **Columna separada `email_normalized` junto al correo original**: descartada por YAGNI; duplica el
  dato y abre la pregunta de cuál mostrar, que US4-2 ya responde.
- **Inspeccionar el código `23505` de `pgconn.PgError`**: descartada, acopla el servicio al driver.
- **Normalizar alias con punto o con signo más**: prohibida explícitamente por el supuesto
  "Unicidad sensible a la cuenta completa" de la spec.

### Validación del formato de correo

**Decisión**: `net/mail.ParseAddress` de la biblioteca estándar, más el límite de 254 caracteres de
FR-003, más el rechazo de direcciones con nombre para mostrar (`"Ana" <ana@example.com>` se
rechaza).

**Rationale**: la biblioteca estándar cubre el requisito de FR-003 ("parte local, arroba y dominio")
sin sumar una dependencia de validación (Principio IV).

**Alternativas consideradas**: expresión regular propia — descartada, es más frágil que el parser de
la biblioteca estándar y nadie la mantiene.

---

## 4. Contraseña: bcrypt y el límite de 72 (RN-04, FR-006, FR-010)

### Decisión

- Hash con `golang.org/x/crypto/bcrypt` y `bcrypt.DefaultCost` (10).
- Validación de entrada: entre 8 y 72 **runas**, con al menos una letra (`unicode.IsLetter`) y al
  menos un dígito (`unicode.IsDigit`).
- **Además**, se rechaza con el mismo error de validación descriptivo toda contraseña cuya
  codificación UTF-8 supere los **72 bytes**.

### Desvío que requiere confirmación del desarrollador

FR-006 y RN-04 dicen "entre 8 y 72 **caracteres**". bcrypt opera sobre bytes y falla por encima de
72 bytes: `bcrypt.GenerateFromPassword` devuelve `ErrPasswordTooLong`. Con caracteres multibyte
(acentos, emoji) una contraseña de 72 caracteres puede superar los 72 bytes, y entonces hay dos
salidas posibles y ninguna cumple todo:

1. Rechazarla como entrada inválida — se aparta de la letra de RN-04 para un subconjunto de
   contraseñas.
2. Dejar que bcrypt falle — produciría un error interno en el camino de la petición, lo que
   **viola el Principio VII** y la condición de error de la spec, que exige rechazo descriptivo.

Se elige (1) porque el Principio VII no admite excepción y porque RC-02 obliga a comunicar el
límite en el formulario en lugar de descubrirlo al fallar. El mensaje usa el mismo texto del rango
permitido, sin repetir la contraseña (FR-011). **Se señala explícitamente al desarrollador**: si
prefiere otra resolución, corresponde ajustar FR-006 y RN-04 en la spec, que es la fuente de verdad;
un agente no modifica la spec por su cuenta.

**Rationale del coste**: `DefaultCost` (10) es el valor recomendado por el paquete. No se expone un
parámetro de entorno para bajarlo en pruebas: sumaría configuración no solicitada (Principio I) y
el tiempo agregado por prueba es del orden de los 100 ms.

**Alternativas consideradas**:

- **Pre-hash SHA-256 antes de bcrypt** para eludir el límite de 72 bytes: descartada; añade un paso
  criptográfico no pedido y RC-02 ya establece el límite como restricción del producto.
- **Coste configurable por entorno**: descartada por YAGNI.

### Sobre el canal lateral de tiempo en el inicio de sesión

FR-014 exige un mensaje genérico idéntico. **No** se añade un cálculo bcrypt simulado para igualar
los tiempos de respuesta entre "correo inexistente" y "contraseña incorrecta".

**Rationale**: FR-014 habla del mensaje, no del tiempo, y la spec ya acepta explícitamente el
riesgo de prueba automática de contraseñas al descartar el límite de intentos fallidos
(clarificación del 2026-09-20). Igualar tiempos sin limitar intentos sería proteger una rendija
dejando la puerta abierta. Se deja constancia del riesgo, coherente con el supuesto aceptado.

---

## 5. Registro de eventos de autenticación (FR-028 a FR-031)

### Decisión

Tabla `auth_events` con tipo de evento, cuenta involucrada (anulable) y correo intentado
(anulable). Cuatro tipos: `account_created`, `login_succeeded`, `login_failed`, `logout`.

- `user_id` se completa cuando la cuenta existe; queda nulo en el intento fallido contra un correo
  no registrado (FR-029: "sin inventar una cuenta").
- `attempted_email` se completa solo en `login_failed`, con el correo **normalizado** que se
  intentó, para que el intento quede rastreable aunque no haya cuenta.
- **Nunca** se guarda la contraseña ni nada derivado de ella (FR-030). El modelo no tiene ninguna
  columna donde pudiera caber.
- **No se expone ningún endpoint ni pantalla** de consulta (FR-031): la entidad no tiene capa
  `delivery`.

**Escritura transaccional**: cada evento se escribe en la **misma transacción** que el efecto que
registra.

| Operación | Transacción |
| --- | --- |
| Registro | `INSERT users` + `INSERT auth_events(account_created)` |
| Inicio de sesión exitoso | `INSERT sessions` + `INSERT auth_events(login_succeeded)` |
| Inicio de sesión fallido | `INSERT auth_events(login_failed)` |
| Cierre de sesión | `UPDATE sessions.revoked_at` + `INSERT auth_events(logout)` |

**Rationale**: FR-028 usa "DEBE", así que el registro no es best-effort. Compartir transacción con
el efecto principal hace imposible el estado incoherente en las dos direcciones: no hay cuenta
creada sin evento ni evento sin cuenta. Además da gratis la atomicidad que FR-008 exige ("sin crear
ninguna cuenta parcial").

**Alternativas consideradas**:

- **Escritura asíncrona o best-effort**: descartada; permitiría perder eventos que FR-028 exige y
  SC-009 mide al 100 %.
- **Escribir en un archivo de log en lugar de una tabla**: descartada; la spec declara "Evento de
  autenticación" como entidad y FR-029 fija sus atributos, lo que pide una tabla consultable.

---

## 6. Momento de creación y columnas duplicadas

### Decisión

Las restricciones técnicas de la constitución exigen que todo modelo persistido registre fecha y
hora de creación y de última actualización. Se usan `CreatedAt` y `UpdatedAt` de GORM, y **se
reutiliza `created_at` como el momento que la spec llama de otra forma**:

| Atributo de la spec | Columna |
| --- | --- |
| Sesión, "momento de emisión" | `created_at` |
| Evento de autenticación, "momento del evento" | `created_at` |

**Rationale**: una sesión se emite en el instante en que se crea su fila y un evento se registra en
el instante en que ocurre. Añadir `issued_at` y `occurred_at` junto a `created_at` sería duplicar el
mismo dato en dos columnas que nunca pueden diferir (DRY, Principio I).

**Alternativas consideradas**: columnas `issued_at` y `occurred_at` explícitas — descartadas por la
duplicación, que además abre la pregunta de cuál es la autoritativa.

---

## 7. Migraciones y esquema

**Decisión**: `AutoMigrate` de GORM, invocado desde `cmd/app` al arrancar y desde el ayudante de
preparación de las pruebas de integración. El índice único de `email` se declara con la etiqueta
`uniqueIndex` en el modelo.

**Rationale**: la constitución autoriza `AutoMigrate` como mecanismo por defecto y excluye las
herramientas de migración externas salvo justificación en el plan. Esta feature crea tres tablas
nuevas y no altera ninguna existente, así que no hay ningún caso que `AutoMigrate` no cubra.

**Alternativas consideradas**: golang-migrate o Atlas — descartadas, dependencia no autorizada sin
necesidad demostrada.

---

## 8. Entornos de desarrollo y de test (Principio V)

**Decisión**: un único `backend/postgres/docker-compose.yml` parametrizado por archivo de entorno,
levantado con nombre de proyecto distinto en cada entorno.

| Entorno | Comando | Puerto en el host | Base |
| --- | --- | --- | --- |
| Desarrollo | `docker compose --env-file .env -p smye-dev up -d` | `5432` | `smye_dev` |
| Test | `docker compose --env-file .env.test -p smye-test up -d` | `5433` | `smye_test` |

El nombre de proyecto distinto da contenedores y **volúmenes distintos**; el puerto distinto hace
imposible que una conexión de test alcance la base de desarrollo.

**Rationale**: cumple "un contenedor de test equivalente por cada base de datos" y "distintos
contenedores, distintas bases y distintos archivos de entorno" con un solo archivo de composición
en lugar de dos casi idénticos (DRY).

**Alternativas consideradas**:

- **Dos archivos de composición**: descartada, duplica la definición del servicio.
- **Un solo contenedor con dos bases**: descartada, la constitución pide contenedores disjuntos.
- **Testcontainers u otro contenedor programático**: prohibido por el Principio IV.

### Lectura de `.env.test` en las pruebas de Go

**Decisión**: un ayudante propio de unas veinte líneas en `internal/platform/config` que lee un
archivo `KEY=VALUE` usando solo `bufio` y `strings` de la biblioteca estándar, y lo carga en el
entorno del proceso de prueba.

**Rationale**: el Principio IV excluye explícitamente los cargadores de variables de entorno de
terceros ("se usa la librería estándar"), pero `go test ./...` tiene que poder encontrar el DSN de
la base de test sin que la persona exporte variables a mano. El ayudante es la pieza mínima que
cierra ese hueco sin dependencia nueva.

**Alternativas consideradas**: `godotenv` — prohibido; exigir variables exportadas a mano —
descartado, rompe la ejecución reproducible de las pruebas.

### Aislamiento entre pruebas

**Decisión**: un ayudante de preparación que, antes de cada prueba de integración, vacía las tres
tablas (`TRUNCATE ... CASCADE`) sobre la base de test.

**Rationale**: es más simple y más fiel al comportamiento real que envolver cada prueba en una
transacción con rollback, que falsearía el escenario de concurrencia de US1-5.

---

## 9. Transporte del token en el frontend

### Decisión

El backend devuelve el token en el **cuerpo** de la respuesta de inicio de sesión. El frontend lo
guarda en `sessionStorage` y lo envía en la cabecera `Authorization: Bearer <token>`.

**Rationale**:

- Un único mecanismo de transporte mantiene la API documentable en Swagger y las pruebas de
  integración directas (KISS). Soportar a la vez cabecera y cookie serían dos caminos de
  autorización que mantener y probar.
- `sessionStorage` en lugar de `localStorage`: el token no sobrevive al cierre del navegador, lo que
  acompaña el motivo que la propia spec da para US5 ("equipos que comparten equipos de trabajo").
- Sobrevive a un refresco de página dentro de la misma pestaña, así que no hace falta nada más.

### Compromiso explícito, señalado al desarrollador

Un token en `sessionStorage` es legible por JavaScript y por lo tanto accesible a un ataque de XSS,
a diferencia de una cookie `HttpOnly`. Ningún requisito de la spec fija el transporte, y no hay
ninguna feature de cookies en el alcance, así que se elige la opción de menos piezas móviles. **Se
deja constancia para que la revisión de seguridad lo evalúe**: si se decide cambiar a cookie
`HttpOnly` con `SameSite`, el impacto es el middleware del backend, la configuración de CORS con
credenciales y el cliente HTTP del frontend.

**Alternativas consideradas**:

- **Cookie `HttpOnly` fijada por el backend**: más resistente a XSS, pero exige CORS con
  credenciales entre dos puertos distintos y complica Swagger y las pruebas. Diferida.
- **Solo en memoria, sin almacenamiento**: cierra la sesión en cada refresco de página; mala
  experiencia sin ningún requisito que lo pida.

### Protección de rutas del frontend

**Decisión**: guardia del lado del cliente. Como el token vive en `sessionStorage`, el middleware
de Next.js no puede verlo, así que la página de perfil comprueba la sesión en el cliente y redirige
a inicio de sesión si falta o si la API responde 401.

**Rationale**: la barrera real y autoritativa es el middleware del backend (FR-019, FR-020). La
guardia del frontend es solo experiencia de usuario; duplicarla en el servidor de Next.js no añade
seguridad y sí piezas móviles.

---

## 10. Documentación de la API con Swagger

**Decisión**: anotaciones `swag` en español sobre los manejadores HTTP; `swag init` genera
`backend/docs/`; se publica en `/swagger/index.html` con `gin-swagger`. El directorio generado
`backend/docs/` **se excluye del control de versiones** y el `Dockerfile` del backend ejecuta
`swag init` durante la construcción.

**Rationale**: el Principio VI obliga a que `.gitignore` excluya "todo artefacto reproducible
mediante comandos", y `backend/docs/` lo es. La contrapartida es que `go build` falla en una copia
recién clonada hasta que se ejecuta `swag init`; se resuelve documentando ese paso en el `README.md`
(cuyo criterio de aceptación es precisamente que alguien que no conoce el proyecto pueda ejecutarlo)
y ejecutándolo dentro del `Dockerfile`, de modo que el camino contenerizado no requiere pasos
manuales.

**Señalado al desarrollador**: la alternativa es versionar `backend/docs/`, que haría compilable el
clon recién hecho a cambio de contradecir el Principio VI. Se eligió respetar el principio, que
prevalece.

---

## 11. Organización en módulos (Principio III)

**Decisión**: un único módulo de feature, `backend/internal/auth/`, con sus cuatro capas
(`domain/`, `repository/`, `service/`, `delivery/`). Las tres entidades (`User`, `Session`,
`AuthEvent`) viven en su `domain/` porque las tres son parte de la misma feature y ninguna se usa
fuera de ella todavía.

Además, dos paquetes de infraestructura mínimos fuera de los módulos de feature:
`internal/platform/config` (lectura de variables de entorno) e `internal/platform/database`
(apertura de la conexión GORM y `AutoMigrate`).

**Rationale**:

- Separar `user` y `auth` en dos módulos crearía una frontera que ninguna funcionalidad pedida
  atraviesa: el registro, el inicio de sesión y el perfil son la misma feature (Principio I).
- Los paquetes de `platform/` existen porque el Principio III exige que `cmd/app/` "solo componga
  dependencias y arranque el servidor". Leer la configuración y abrir la conexión son justamente
  dependencias que componer, y además las pruebas de integración necesitan reusarlas, lo que el
  paquete `main` no permite.
- Las interfaces de repositorio declaran **solo** las operaciones que esta feature usa; no se genera
  CRUD especulativo (Principio III).

**Alternativas consideradas**:

- **Módulos `user` y `auth` separados**: descartada por abstracción prematura.
- **Configuración y conexión dentro de `cmd/app/`**: descartada, las pruebas de integración no
  pueden importar un paquete `main`.

---

## 12. Lectura obligatoria de la documentación de Next.js

La guía operativa del repositorio exige leer la documentación de la versión instalada de Next.js en
`frontend/node_modules/next/dist/docs/` **antes** de escribir código de Next.js. Hoy ese directorio
no existe porque el proyecto no está creado.

**Decisión**: la lectura queda como puerta obligatoria del plan, inmediatamente después de crear el
proyecto con `create-next-app` y antes de la primera línea de código de Next.js o de la
configuración de Vitest. La guía oficial de Vitest de esa versión es también la que determina el
conjunto exacto de paquetes de prueba (ver sección 1).

**Decisión asociada**: el proyecto se crea con `--no-agents-md` y `next.config.ts` fija
`agentRules: false`, para que ninguna herramienta genere `AGENTS.md` (Principio IX, ámbito f). Si
pese a ello apareciera un `AGENTS.md`, se avisa al desarrollador y no se borra por cuenta propia.

---

## 13. Duplicación de validaciones entre backend y frontend

**Decisión**: las reglas de RN-03 y RN-04 se implementan **dos veces**, en Go y en TypeScript.

**Rationale**: no es una violación de DRY. El backend es la autoridad (Principio VII: toda entrada
validada antes de la capa de servicio) y su validación no es opcional. La del frontend existe para
cumplir RC-02 ("el límite debe comunicarse a la persona en el formulario, no descubrirse al
fallar"). Son dos responsabilidades distintas en dos lenguajes distintos; compartir el código
exigiría un esquema común o generación de código, es decir más piezas móviles que las que evita.

Los valores límite (2, 100, 8, 72, 254) se declaran como constantes nombradas en un único lugar por
lado, no repetidos en cada punto de uso.

---

## Resumen: incógnitas resueltas

| Incógnita | Resolución |
| --- | --- |
| Versiones exactas de todo el stack | Sección 1; ninguna autorización nueva necesaria |
| Cómo revocar un JWT de inmediato (FR-026) | Sección 2; `jti` más tabla `sessions` |
| Cómo garantizar unicidad bajo concurrencia (FR-007) | Sección 3; índice único y `ErrDuplicatedKey` |
| Límite de contraseña frente a bcrypt | Sección 4; desvío documentado, requiere confirmación |
| Dónde y cómo registrar los eventos (FR-028) | Sección 5; tabla `auth_events`, misma transacción |
| Aislamiento dev/test (Principio V) | Sección 8; un compose, dos nombres de proyecto y puertos |
| Transporte del token | Sección 9; `Bearer` y `sessionStorage`, compromiso señalado |
| Swagger generado y `.gitignore` | Sección 10; excluido, `swag init` documentado |
| Organización en módulos | Sección 11; un módulo `auth` más `platform/` |

### Puntos abiertos para el desarrollador (no bloquean el diseño)

1. **Instalar Go 1.27.1** (prerrequisito bloqueante de la implementación).
2. **Confirmar el tratamiento del límite de 72** (sección 4): si la resolución elegida no convence,
   la spec debe ajustarse antes de implementar.
3. **Revisar el transporte del token** (sección 9) desde la óptica de seguridad.
