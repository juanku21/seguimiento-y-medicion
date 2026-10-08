# Modelo de Datos (Fase 1): Autenticación y Cuentas de Usuario

**Feature**: `specs/001-user-auth` | **Fecha**: 2026-10-05

Entidades derivadas de la sección "Entidades Clave" de [spec.md](./spec.md). Las decisiones de
diseño que las justifican están en [research.md](./research.md).

Reglas transversales obligatorias (restricciones técnicas de la constitución):

- La clave primaria de los tres modelos es **UUID**; prohibidos los enteros secuenciales.
- Los tres modelos registran **fecha y hora de creación y de última actualización**
  (`created_at`, `updated_at`, gestionadas por GORM).
- La contraseña se persiste **únicamente** como hash bcrypt.
- El esquema se crea con `AutoMigrate` de GORM.
- Los identificadores del código están en inglés (Principio VIII).

**Glosario**: donde la spec dice "caracteres", este documento dice **runas** (`rune` de Go, un punto
de código Unicode), que es la unidad en la que se cuenta al validar y la que también cuenta
`varchar(n)` de PostgreSQL. Las dos coinciden en todos los límites de esta feature, con **una
excepción**: el máximo de la contraseña, que bcrypt impone en **bytes** y no en runas (ver la
sección 1 y [research.md](./research.md), sección 4). Esa diferencia está señalada en el
[plan](./plan.md) como punto a confirmar con el desarrollador.

---

## 1. `User` — tabla `users`

Representa a una persona que usa el sistema. Es la entidad a la que se atribuyen todas las acciones
del producto (FR-021, RN-09).

| Campo (Go) | Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- | --- |
| `ID` | `id` | `uuid` | PK, generado en la aplicación con `google/uuid` | FR-027 |
| `FullName` | `full_name` | `varchar(100)` | `NOT NULL` | FR-002, RN-03 |
| `Email` | `email` | `varchar(254)` | `NOT NULL`, **índice único** | FR-003, FR-004, FR-005, RN-01 |
| `PasswordHash` | `password_hash` | `varchar(60)` | `NOT NULL` | FR-010, RN-05 |
| `CreatedAt` | `created_at` | `timestamptz` | `NOT NULL` | Constitución |
| `UpdatedAt` | `updated_at` | `timestamptz` | `NOT NULL` | Constitución |

### Reglas de validación

| Regla | Detalle | Requisito |
| --- | --- | --- |
| Nombre obligatorio | Tras `TrimSpace`, entre 2 y 100 runas. Un nombre vacío o solo de espacios equivale a ausente y se rechaza. | FR-002, RN-03 |
| Correo con formato válido | `net/mail.ParseAddress`, sin nombre para mostrar, hasta 254 caracteres. | FR-003 |
| Correo normalizado | `TrimSpace` + `ToLower` **antes** de llegar al servicio. La columna almacena el valor normalizado. | FR-004, RN-02 |
| Correo único | Garantizado por el índice único, no por consulta previa. | FR-005, FR-007, SC-006 |
| Contraseña | Entre 8 y 72 runas, al menos una letra y un dígito, y como máximo 72 bytes en UTF-8. | FR-006, RN-04 |

### Notas de diseño

- **Una sola columna de correo**: almacena el valor normalizado y es la que se muestra en el perfil
  (escenario US4-2). No se guarda el correo tal como se tecleó.
- `password_hash` mide 60 caracteres porque es el largo fijo de la salida de bcrypt.
- `PasswordHash` **nunca** se serializa: el modelo de dominio no se devuelve por HTTP; las
  respuestas se construyen con DTO propios de la capa `delivery` (FR-011, SC-005).
- La unicidad bajo concurrencia se resuelve traduciendo `gorm.ErrDuplicatedKey` al error "correo no
  disponible" (FR-007).

---

## 2. `Session` — tabla `sessions`

Representa el permiso temporal de una persona identificada para operar en el sistema. Es la pieza
que hace posible la invalidación inmediata exigida por FR-026 sobre un JWT (ver
[research.md](./research.md), sección 2).

| Campo (Go) | Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- | --- |
| `ID` | `id` | `uuid` | PK; es el claim `jti` del JWT | FR-026 |
| `UserID` | `user_id` | `uuid` | `NOT NULL`, FK a `users.id`, indexada | Entidad Sesión |
| `ExpiresAt` | `expires_at` | `timestamptz` | `NOT NULL`; `created_at` más 8 horas | FR-013, FR-018 |
| `RevokedAt` | `revoked_at` | `timestamptz` | **anulable**; no nulo si el titular la cerró | FR-026 |
| `CreatedAt` | `created_at` | `timestamptz` | `NOT NULL`; **es el momento de emisión** | FR-013, Constitución |
| `UpdatedAt` | `updated_at` | `timestamptz` | `NOT NULL` | Constitución |

### Estados y transiciones

Una sesión tiene tres estados observables, derivados de los datos (no hay columna de estado):

```text
                 inicio de sesión exitoso
                           |
                           v
                      [ VIGENTE ]
                 revoked_at IS NULL
                 AND expires_at > now()
                     /           \
       cierre de    /             \   paso del tiempo
       sesión      /               \  (8 h desde created_at)
                  v                 v
            [ CERRADA ]         [ VENCIDA ]
         revoked_at NOT NULL   expires_at <= now()
```

- `VIGENTE` es el único estado que otorga acceso.
- `CERRADA` y `VENCIDA` son terminales: una sesión no vuelve a ser vigente. Para volver a operar hay
  que iniciar sesión, lo que crea una **fila nueva** (escenario US5-2).
- El middleware trata `CERRADA`, `VENCIDA`, fila inexistente (sesión inventada) y token con firma
  inválida (sesión adulterada) de forma **idéntica**: falta de sesión (FR-015, RN-07).
- Cerrar una sesión **no** afecta a las otras filas del mismo `user_id` (FR-017, escenario US5-4).
- El plazo de 8 horas es fijo: ninguna operación modifica `expires_at` (FR-018).

### Correspondencia entre el JWT y la fila

| Claim del JWT | Valor | Verificación del middleware |
| --- | --- | --- |
| `jti` | `sessions.id` | Debe existir la fila, con `revoked_at` nulo y `expires_at` futuro |
| `sub` | `users.id` | Identidad a la que se atribuye la acción (FR-021) |
| `exp` | `sessions.expires_at` | Validado por la librería JWT antes de tocar la base |
| `iat` | `sessions.created_at` | Informativo |

Validar `exp` en el token **y** `expires_at` en la fila es redundante por diseño: el token corta sin
coste de base de datos y la fila es la autoridad.

---

## 3. `AuthEvent` — tabla `auth_events`

Representa algo que ocurrió con las cuentas y que conviene poder revisar después (FR-028 a FR-031).

| Campo (Go) | Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- | --- |
| `ID` | `id` | `uuid` | PK | Constitución |
| `EventType` | `event_type` | `varchar(32)` | `NOT NULL`; uno de los cuatro valores de abajo | FR-029 |
| `UserID` | `user_id` | `uuid` | **anulable**, FK a `users.id`, indexada | FR-029 |
| `AttemptedEmail` | `attempted_email` | `varchar(254)` | **anulable**; correo normalizado del intento | FR-029 |
| `CreatedAt` | `created_at` | `timestamptz` | `NOT NULL`; **es el momento del evento** | FR-029, Constitución |
| `UpdatedAt` | `updated_at` | `timestamptz` | `NOT NULL` | Constitución |

### Tipos de evento y campos que se completan

| `event_type` | Cuándo | `user_id` | `attempted_email` |
| --- | --- | --- | --- |
| `account_created` | Alta de cuenta exitosa | ID de la cuenta creada | nulo |
| `login_succeeded` | Inicio de sesión exitoso | ID del titular | nulo |
| `login_failed` | Credenciales inválidas | ID si el correo existe, **nulo** si no | correo normalizado intentado |
| `logout` | Cierre de sesión | ID del titular | nulo |

### Notas de diseño

- Para un intento fallido contra un correo **no registrado**, `user_id` queda nulo y el intento
  queda rastreable por `attempted_email`: se deja constancia "sin inventar una cuenta" (FR-029).
- **No existe ninguna columna donde quepa una contraseña** (FR-030, RN-05). Es una garantía
  estructural, no una convención.
- La tabla **no tiene capa `delivery`**: ningún endpoint ni pantalla la consulta (FR-031). Se lee
  solo desde las pruebas de integración, que es lo que verifica SC-009.
- El evento se escribe en la **misma transacción** que el efecto que registra (ver
  [research.md](./research.md), sección 5).
- `event_type` se modela como constantes de Go tipadas, no como `enum` de PostgreSQL: añadir un tipo
  en la base obligaría a una migración por cada valor nuevo sin aportar nada que `AutoMigrate` y la
  validación en el dominio no cubran.

---

## 4. Relaciones

```text
   users
     | id (PK)
     |
     +----< sessions.user_id      NOT NULL   (una cuenta, N sesiones simultáneas — FR-017)
     |
     +----< auth_events.user_id   NULL       (nulo en intento fallido con correo no registrado)
```

- `sessions.user_id`: obligatoria. Toda sesión pertenece a una cuenta.
- `auth_events.user_id`: opcional, por el caso explícito de FR-029.
- Ninguna de las dos relaciones se navega desde `User`: las interfaces de repositorio declaran solo
  las operaciones que esta feature usa y no se genera CRUD especulativo (Principio III). No hay
  campos de precarga (`Preload`) ni colecciones en el modelo `User`.
- No se definen borrados en cascada porque esta feature no borra cuentas (está fuera de alcance).

---

## 5. Interfaces de repositorio (capa `domain`)

Declaran exclusivamente lo que esta feature necesita. Las dependencias apuntan hacia `domain`
(Principio III).

| Interfaz | Operación | Usada por |
| --- | --- | --- |
| `UserRepository` | `Create(ctx, user, event) error` | Registro (transacción cuenta + evento) |
| `UserRepository` | `FindByEmail(ctx, email) (*User, error)` | Inicio de sesión |
| `UserRepository` | `FindByID(ctx, id) (*User, error)` | Consulta del perfil |
| `SessionRepository` | `Create(ctx, session, event) error` | Inicio de sesión (transacción sesión + evento) |
| `SessionRepository` | `FindByID(ctx, id) (*Session, error)` | Middleware de autorización |
| `SessionRepository` | `Revoke(ctx, id, at, event) error` | Cierre de sesión (transacción revocación + evento) |
| `AuthEventRepository` | `Create(ctx, event) error` | Intento de inicio de sesión fallido |

Las operaciones que agrupan dos escrituras reciben el evento como parámetro para poder resolverlas
en una sola transacción, en lugar de exponer la transacción al servicio.

**No se declaran**: `Update`, `Delete`, `List`, ni búsquedas por otros campos. Ninguna
funcionalidad de la spec las pide.

---

## 6. Trazabilidad: requisitos cubiertos por el modelo

| Requisito | Cómo lo cubre el modelo |
| --- | --- |
| FR-002, RN-03 | `full_name varchar(100)` más validación de 2 a 100 runas |
| FR-003 | `email varchar(254)` más `net/mail.ParseAddress` |
| FR-004, RN-02 | La columna `email` guarda el valor normalizado |
| FR-005, FR-007, SC-006 | Índice único sobre `email` |
| FR-006, RN-04 | Validación de 8 a 72 runas, letra y dígito, 72 bytes máximo |
| FR-010, FR-011, RN-05, SC-005 | Solo `password_hash`; DTO de salida sin ese campo |
| FR-013, FR-018 | `sessions.expires_at` fijo a `created_at` más 8 h |
| FR-015, RN-07 | Los tres estados no vigentes dan la misma respuesta |
| FR-017 | Varias filas `sessions` por `user_id`; revocación de una sola |
| FR-021, RN-09 | `sub` del JWT identifica al titular de toda acción |
| FR-026 | `sessions.revoked_at` consultada en cada petición protegida |
| FR-027 | `users.id` UUID, estable e inmutable |
| FR-028, FR-029, SC-009 | Tabla `auth_events` con los cuatro tipos |
| FR-030 | Ninguna columna admite contraseñas |
| FR-031 | La tabla no tiene capa `delivery` |
