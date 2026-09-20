# Especificación de Feature: Autenticación y Cuentas de Usuario

**Directorio de feature**: `specs/001-user-auth`

**Rama**: `001-user-auth`

**Creada**: 2026-09-19

**Estado**: Clarificada — sin preguntas abiertas (sesión de clarificación del 2026-09-20)

**Entrada**: Descripción del usuario: "Autenticación y cuentas de usuario para Software Metrics &
Estimation, un sistema web multiusuario para estimar, planificar, seguir y medir proyectos de
software con Scrum."

---

## Objetivo

Permitir que cualquier persona cree una cuenta, inicie sesión y acceda a las funciones protegidas
del sistema únicamente mientras su sesión sea válida.

Esta feature es la base de todas las demás del producto: el sistema registra estimaciones, esfuerzo
invertido, votos y defectos, y cada uno de esos registros debe poder atribuirse a una persona
identificada. Sin identidad verificada, las métricas que produce el sistema no son trazables ni
auditables.

---

## Clarifications

### Session 2026-09-20

- Q: ¿Cuánto tiempo transcurre desde que alguien inicia sesión hasta que su sesión vence y debe volver a autenticarse? (FR-018) → A: 8 horas desde el inicio de sesión, sin renovación automática.
- Q: Cuando alguien cierra sesión, ¿la sesión que tenía deja de servir inmediatamente en el sistema, o solo se descarta en su dispositivo? (FR-026) → A: invalidación inmediata de esa sesión en el sistema; las demás sesiones de la misma cuenta siguen vigentes.
- Q: ¿El sistema debe limitar la cantidad de intentos fallidos de inicio de sesión sobre una misma cuenta? → A: no; intentos ilimitados, con el riesgo de prueba automática de contraseñas documentado y aceptado.
- Q: ¿El sistema debe dejar registro de los eventos de autenticación (altas, inicios exitosos, intentos fallidos y cierres de sesión)? → A: sí, registro para diagnóstico con fecha, tipo y cuenta involucrada; sin pantalla de consulta dentro de la aplicación.
- Q: Al terminar el registro, ¿la persona queda con la sesión ya iniciada o debe iniciar sesión con las credenciales recién creadas? → A: debe iniciar sesión; el registro no emite sesión.

---

## Entradas y Salidas Esperadas

### Registro de cuenta

| Entradas | Salidas esperadas |
| --- | --- |
| Nombre completo, correo electrónico, contraseña | **Éxito**: confirmación de cuenta creada; la persona puede iniciar sesión con ese correo |
| | **Error de validación**: detalle de qué campo es inválido y por qué, sin exponer la contraseña |
| | **Correo ya registrado**: aviso de que el correo no está disponible |

### Inicio de sesión

| Entradas | Salidas esperadas |
| --- | --- |
| Correo electrónico, contraseña | **Éxito**: sesión vigente con vencimiento y acceso a las funciones protegidas |
| | **Fallo**: mensaje genérico de credenciales inválidas, idéntico para correo inexistente y contraseña incorrecta |

### Consulta del propio perfil

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión válida | Nombre completo y correo electrónico del titular de la sesión. Nunca la contraseña ni ninguna forma derivada de ella |
| Sesión ausente, vencida o adulterada | Rechazo con indicación de que se requiere iniciar sesión |

### Cierre de sesión

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión válida y acción de cierre | Confirmación de cierre; esa sesión queda invalidada de inmediato y las funciones protegidas vuelven a exigir inicio de sesión |

### Acceso a cualquier función protegida

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión válida | La función se ejecuta y queda atribuida al titular de la sesión |
| Sesión ausente, vencida o adulterada | Rechazo explícito con indicación de volver a iniciar sesión; la acción no produce ningún efecto |

---

## Escenarios de Usuario y Pruebas *(obligatorio)*

### Historia de Usuario 1 (US1) — Registro de cuenta (Prioridad: P1)

Una persona que todavía no usa el sistema ingresa su nombre completo, su correo electrónico y una
contraseña, y obtiene una cuenta con la que podrá identificarse en adelante.

**Por qué esta prioridad**: sin cuentas no existe ninguna otra funcionalidad del producto. Es la
primera porción que entrega valor observable.

**Prueba independiente**: se puede probar completa registrando una cuenta nueva y comprobando que
el correo queda ocupado para un segundo registro, sin necesitar ninguna otra historia.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** que no existe ninguna cuenta con el correo `ana@example.com`, **cuando**
   una persona se registra con nombre "Ana Pérez", correo `ana@example.com` y contraseña
   `Proyecto2026`, **entonces** la cuenta queda creada y el sistema confirma el alta.
2. *(Caso alternativo)* **Dado** que no existe ninguna cuenta con el correo `ana@example.com`,
   **cuando** una persona se registra con el correo `  Ana@Example.COM  `, **entonces** la cuenta
   queda creada con el correo normalizado (sin espacios extremos) y a partir de allí puede iniciar
   sesión escribiéndolo con cualquier combinación de mayúsculas y minúsculas.
3. *(Caso límite)* **Dado** un registro válido en todo lo demás, **cuando** la contraseña tiene
   exactamente 8 caracteres con al menos una letra y un número, **entonces** la cuenta se crea; y lo
   mismo ocurre cuando la contraseña tiene exactamente 72 caracteres.
4. *(Caso límite)* **Dado** un registro válido en todo lo demás, **cuando** el nombre tiene
   exactamente 2 caracteres, **entonces** la cuenta se crea; y **cuando** tiene exactamente 100
   caracteres, también se crea.
5. *(Caso límite — concurrencia)* **Dado** que el mismo formulario de registro se envía dos veces de
   forma simultánea con el mismo correo, **cuando** ambos envíos se procesan, **entonces** se crea
   exactamente una cuenta y el segundo envío se rechaza por correo ya registrado.
6. *(Caso alternativo)* **Dado** un registro recién completado con éxito, **cuando** la persona
   intenta acceder a una función protegida sin iniciar sesión, **entonces** el acceso se rechaza: el
   registro no deja la sesión iniciada.
7. *(Caso de error)* **Dado** que ya existe una cuenta con el correo `ana@example.com`, **cuando**
   una persona intenta registrarse con `ANA@example.com`, **entonces** el registro se rechaza
   informando que el correo no está disponible y no se crea una segunda cuenta.
8. *(Caso de error)* **Dado** un intento de registro, **cuando** falta el nombre, falta el correo,
   el correo no tiene formato válido, la contraseña tiene 7 caracteres, tiene 73 caracteres, no
   contiene ninguna letra o no contiene ningún número, **entonces** el registro se rechaza indicando
   qué campo corregir y no se crea ninguna cuenta.

---

### Historia de Usuario 2 (US2) — Inicio de sesión (Prioridad: P1)

Una persona con cuenta ingresa su correo y su contraseña y obtiene una sesión vigente que le permite
operar en el sistema durante un tiempo acotado.

**Por qué esta prioridad**: es la puerta de entrada a todo el producto. Sin inicio de sesión, una
cuenta registrada no sirve para nada.

**Prueba independiente**: se puede probar completa partiendo de una cuenta ya existente, iniciando
sesión y verificando que la sesión obtenida permite acceder a una función protegida.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** que existe una cuenta con correo `ana@example.com` y contraseña
   `Proyecto2026`, **cuando** la persona inicia sesión con esas credenciales, **entonces** obtiene
   una sesión vigente con vencimiento y puede acceder a las funciones protegidas.
2. *(Caso alternativo)* **Dado** que existe esa cuenta, **cuando** la persona inicia sesión
   escribiendo `  ANA@Example.com  `, **entonces** el inicio de sesión es exitoso porque el correo se
   compara sin distinguir mayúsculas y sin espacios al inicio o al final.
3. *(Caso límite)* **Dado** que existe esa cuenta, **cuando** la persona inicia sesión
   correctamente dos veces seguidas desde dos dispositivos, **entonces** ambas sesiones son válidas
   y ninguna interfiere con la otra.
4. *(Caso de error)* **Dado** que no existe ninguna cuenta con el correo `nadie@example.com`,
   **cuando** alguien intenta iniciar sesión con ese correo y cualquier contraseña, **entonces** el
   sistema responde con un mensaje genérico de credenciales inválidas.
5. *(Caso de error)* **Dado** que existe la cuenta `ana@example.com`, **cuando** alguien intenta
   iniciar sesión con la contraseña incorrecta, **entonces** el sistema responde con **el mismo**
   mensaje genérico del escenario anterior, sin revelar que el correo sí existe.

---

### Historia de Usuario 3 (US3) — Protección de las funciones del sistema (Prioridad: P2)

Cualquier funcionalidad del producto distinta de registro e inicio de sesión exige una sesión
válida; quien no la tenga es rechazado sin que su acción produzca efecto alguno.

**Por qué esta prioridad**: es la razón de ser de la feature. Sin esta barrera, registrarse e
iniciar sesión serían trámites decorativos y los datos de los proyectos quedarían expuestos.

**Prueba independiente**: se puede probar completa intentando acceder a una función protegida sin
sesión, con una sesión vencida y con una sesión adulterada, y verificando el rechazo en los tres
casos.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un usuario con sesión vigente, **cuando** accede a una función
   protegida, **entonces** la función se ejecuta y la acción queda atribuida a ese usuario.
2. *(Caso alternativo)* **Dado** un visitante sin sesión, **cuando** accede al registro o al inicio
   de sesión, **entonces** puede usarlos con normalidad: son las únicas funciones públicas.
3. *(Caso límite)* **Dado** un usuario que está operando en la aplicación, **cuando** su sesión vence
   entre una acción y la siguiente, **entonces** la siguiente acción se rechaza de forma explícita
   indicando que debe volver a iniciar sesión, sin ejecutarse a medias ni fallar en silencio.
4. *(Caso de error)* **Dado** un intento de acceso con una sesión adulterada o inventada, **cuando**
   se solicita una función protegida, **entonces** el sistema la trata exactamente igual que a la
   ausencia de sesión y rechaza el acceso.

---

### Historia de Usuario 4 (US4) — Consulta del propio perfil (Prioridad: P3)

Una persona con sesión vigente consulta los datos de su cuenta para confirmar con qué identidad está
operando.

**Por qué esta prioridad**: aporta confianza y contexto, pero el sistema es usable sin esta pantalla;
por eso va después de las anteriores.

**Prueba independiente**: se puede probar completa iniciando sesión y consultando el perfil, sin
depender de ninguna otra funcionalidad del producto.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un usuario con sesión vigente, **cuando** consulta su perfil,
   **entonces** ve su nombre completo y su correo electrónico.
2. *(Caso alternativo)* **Dado** un usuario que se registró con el correo `  Ana@Example.COM  `,
   **cuando** consulta su perfil, **entonces** ve el correo tal como quedó normalizado, de forma
   consistente con el que usa para iniciar sesión.
3. *(Caso límite)* **Dado** un usuario cuyo perfil se consulta, **cuando** se inspecciona la
   respuesta completa, **entonces** no aparece la contraseña ni ninguna representación derivada de
   ella.
4. *(Caso de error)* **Dado** un visitante sin sesión válida, **cuando** consulta el perfil,
   **entonces** el acceso se rechaza y no se revela ningún dato de ninguna cuenta.

---

### Historia de Usuario 5 (US5) — Cierre de sesión (Prioridad: P3)

Una persona con sesión vigente cierra su sesión desde la interfaz y deja de tener acceso a las
funciones protegidas.

**Por qué esta prioridad**: es necesario para el uso en equipos que comparten equipos de trabajo,
pero no bloquea la operación diaria del producto.

**Prueba independiente**: se puede probar completa iniciando sesión, cerrándola y verificando que una
función protegida vuelve a exigir credenciales.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un usuario con sesión vigente, **cuando** cierra su sesión, **entonces**
   el sistema confirma el cierre y la interfaz vuelve al estado de visitante.
2. *(Caso alternativo)* **Dado** un usuario que cerró su sesión, **cuando** vuelve a iniciar sesión
   con credenciales correctas, **entonces** obtiene una sesión nueva y vigente.
3. *(Caso límite)* **Dado** un usuario que cerró su sesión, **cuando** se intenta usar esa misma
   sesión para acceder a una función protegida —desde su dispositivo o desde cualquier otro—,
   **entonces** el acceso se rechaza aunque no hayan pasado las 8 horas de vigencia.
4. *(Caso límite)* **Dado** un usuario con sesiones vigentes en dos dispositivos, **cuando** cierra
   la sesión en uno, **entonces** la del otro dispositivo sigue funcionando con normalidad.
5. *(Caso de error)* **Dado** un visitante sin sesión, **cuando** solicita cerrar sesión,
   **entonces** la operación no produce ningún efecto ni revela información de ninguna cuenta.

---

### Casos Límite

- **Correo equivalente con otra capitalización o con espacios extremos**: `  ANA@Example.COM  ` y
  `ana@example.com` son el mismo correo, tanto para la unicidad del registro como para el inicio de
  sesión.
- **Contraseña de exactamente 8 caracteres**: válida.
- **Contraseña de exactamente 72 caracteres**: válida.
- **Contraseña de 73 caracteres**: rechazada.
- **Contraseña de 8 caracteres solo con letras o solo con números**: rechazada por no cumplir la
  composición mínima.
- **Nombre de exactamente 2 y de exactamente 100 caracteres**: válidos; el conteo se hace después de
  recortar los espacios al inicio y al final.
- **Nombre compuesto solo por espacios**: equivale a nombre ausente y se rechaza.
- **Sesión que vence mientras la persona está usando la aplicación**: como el plazo de 8 horas es
  fijo y no se renueva por actividad, una jornada continua más larga que eso corta la sesión en
  pleno uso. La siguiente acción se rechaza con un aviso claro de que debe volver a iniciar sesión;
  nada queda a medio ejecutar.
- **Doble envío simultáneo del mismo registro**: se crea una única cuenta; el envío perdedor recibe
  el error de correo ya registrado.
- **Sesión adulterada o fabricada**: se trata igual que la ausencia de sesión.
- **Sesión copiada antes de cerrarla**: una vez cerrada, deja de servir de inmediato en cualquier
  dispositivo, aunque le quedaran horas de vigencia.

---

## Requisitos *(obligatorio)*

### Requisitos Funcionales

**Registro de cuenta**

- **FR-001**: El sistema DEBE permitir crear una cuenta a partir de nombre completo, correo
  electrónico y contraseña.
- **FR-002**: El sistema DEBE exigir un nombre de entre 2 y 100 caracteres, contados después de
  recortar los espacios al inicio y al final; un nombre ausente o compuesto solo por espacios se
  rechaza.
- **FR-003**: El sistema DEBE exigir un correo electrónico con formato válido (parte local, arroba y
  dominio) y de hasta 254 caracteres.
- **FR-004**: El sistema DEBE normalizar el correo antes de usarlo: recortar los espacios al inicio y
  al final y compararlo sin distinguir mayúsculas de minúsculas.
- **FR-005**: El sistema DEBE garantizar que el correo normalizado sea único en todo el sistema.
- **FR-006**: El sistema DEBE exigir una contraseña de entre 8 y 72 caracteres que contenga al menos
  una letra y al menos un número.
- **FR-007**: El sistema DEBE crear una única cuenta cuando se reciben dos solicitudes de registro
  simultáneas con el mismo correo normalizado, y rechazar la restante por correo ya registrado.
- **FR-008**: El sistema DEBE rechazar un registro inválido indicando qué campo debe corregirse y por
  qué, sin crear ninguna cuenta parcial.
- **FR-009**: El registro NO DEBE emitir sesión: al completarlo, la persona queda sin sesión y debe
  iniciarla con las credenciales recién creadas. La emisión de sesiones ocurre únicamente en el
  inicio de sesión.

**Tratamiento de la contraseña**

- **FR-010**: El sistema NUNCA DEBE almacenar la contraseña en forma legible: solo se conserva una
  representación que no permite recuperar el texto original.
- **FR-011**: El sistema NUNCA DEBE incluir la contraseña, ni ninguna representación derivada de
  ella, en respuestas al usuario, mensajes de error, pantallas, registros de actividad o reportes de
  diagnóstico.

**Inicio de sesión y sesión**

- **FR-012**: El sistema DEBE permitir iniciar sesión con correo electrónico y contraseña, aplicando
  al correo la misma normalización definida en FR-004.
- **FR-013**: El sistema DEBE entregar, ante un inicio de sesión exitoso, una sesión con un momento
  de vencimiento determinado.
- **FR-014**: El sistema DEBE responder a cualquier credencial inválida con un único mensaje
  genérico, idéntico e indistinguible tanto si el correo no existe como si la contraseña es
  incorrecta, de modo que no se pueda averiguar qué correos están registrados.
- **FR-015**: El sistema DEBE tratar una sesión vencida, cerrada, ausente o adulterada exactamente
  igual: como falta de sesión.
- **FR-016**: El sistema DEBE informar de forma explícita, cuando rechaza una acción por sesión
  inválida, que la persona debe volver a iniciar sesión.
- **FR-017**: El sistema DEBE admitir que una misma cuenta tenga sesiones vigentes simultáneas en
  distintos dispositivos sin que unas invaliden a otras.
- **FR-018**: La sesión DEBE vencer a las 8 horas contadas desde el inicio de sesión que la emitió.
  El plazo es fijo: la sesión no se renueva ni se extiende por actividad de la persona.

**Protección de acceso**

- **FR-019**: El sistema DEBE exigir una sesión válida para toda funcionalidad, con la única
  excepción del registro y del inicio de sesión, que son públicos.
- **FR-020**: El sistema DEBE rechazar sin efectos secundarios cualquier acción protegida solicitada
  sin sesión válida: la acción no se ejecuta ni total ni parcialmente.
- **FR-021**: El sistema DEBE atribuir toda acción ejecutada sobre una función protegida al titular
  de la sesión con la que se ejecutó.

**Perfil propio**

- **FR-022**: El sistema DEBE permitir a una persona con sesión válida consultar su nombre completo y
  su correo electrónico.
- **FR-023**: El sistema DEBE devolver únicamente los datos del titular de la sesión: no existe forma
  de consultar el perfil de otra cuenta.

**Cierre de sesión**

- **FR-024**: El sistema DEBE permitir cerrar la sesión desde la interfaz.
- **FR-025**: Tras el cierre de sesión, el sistema DEBE exigir un nuevo inicio de sesión para
  cualquier función protegida.
- **FR-026**: El cierre de sesión DEBE invalidar esa sesión de inmediato en el sistema: deja de
  otorgar acceso aunque alguien la haya copiado antes del cierre y aunque todavía no hayan pasado
  las 8 horas. Las demás sesiones vigentes de la misma cuenta no se ven afectadas (coherente con
  FR-017).

**Identidad**

- **FR-027**: Cada cuenta DEBE tener un identificador propio, estable e inmutable, que permita
  atribuirle registros a lo largo del tiempo aunque cambien sus datos visibles.

**Registro de eventos de autenticación**

- **FR-028**: El sistema DEBE dejar registro de cada alta de cuenta, cada inicio de sesión exitoso,
  cada intento de inicio de sesión fallido y cada cierre de sesión.
- **FR-029**: Cada evento registrado DEBE incluir el momento en que ocurrió, el tipo de evento y la
  cuenta involucrada; cuando el intento fallido corresponde a un correo no registrado, se deja
  constancia del intento sin inventar una cuenta.
- **FR-030**: El registro de eventos NUNCA DEBE contener la contraseña ingresada ni ninguna
  representación derivada de ella (aplicación de FR-011).
- **FR-031**: El registro de eventos es para diagnóstico: no se expone ninguna pantalla de consulta
  dentro de la aplicación.

---

### Reglas de Negocio

| ID | Regla |
| --- | --- |
| RN-01 | El correo electrónico identifica de forma única a una persona en todo el sistema. |
| RN-02 | La comparación de correos ignora mayúsculas/minúsculas y espacios al inicio y al final. |
| RN-03 | El nombre completo es obligatorio y mide entre 2 y 100 caracteres. |
| RN-04 | La contraseña mide entre 8 y 72 caracteres e incluye al menos una letra y un número. |
| RN-05 | Las contraseñas no se almacenan, muestran ni registran en texto legible bajo ninguna circunstancia. |
| RN-06 | El mensaje ante credenciales inválidas es genérico y no revela si el correo existe. |
| RN-07 | Una sesión vencida o adulterada equivale a no tener sesión. |
| RN-08 | Registro e inicio de sesión son las únicas funciones públicas; todo lo demás exige sesión válida. |
| RN-09 | Toda acción del sistema se atribuye a un usuario identificado. |

---

### Restricciones

- **RC-01**: La feature no depende de ningún servicio externo de identidad: el sistema es la única
  fuente de verdad de las cuentas.
- **RC-02**: El límite superior de 72 caracteres de contraseña es una restricción del producto y debe
  comunicarse a la persona en el formulario, no descubrirse al fallar.
- **RC-03**: No existe ningún mecanismo para recuperar una contraseña olvidada dentro de esta
  feature; una persona que la pierda queda sin acceso hasta que se especifique esa funcionalidad.
- **RC-04**: Todas las demás features del producto se construyen sobre esta: cualquier cambio en las
  reglas de sesión impacta en la totalidad del sistema.

---

### Condiciones de Error

| Condición | Comportamiento esperado |
| --- | --- |
| Campo obligatorio faltante (nombre, correo o contraseña) | Rechazo con indicación del campo faltante; no se crea ni modifica nada. |
| Correo con formato inválido | Rechazo indicando que el formato del correo no es válido. |
| Nombre fuera del rango de 2 a 100 caracteres | Rechazo indicando el rango permitido. |
| Contraseña fuera del rango de 8 a 72 caracteres | Rechazo indicando el rango permitido, sin repetir la contraseña en el mensaje. |
| Contraseña sin al menos una letra y un número | Rechazo indicando la composición requerida. |
| Correo ya registrado | Rechazo informando que el correo no está disponible. |
| Credenciales incorrectas (correo inexistente o contraseña errónea) | Un único mensaje genérico, idéntico en ambos casos. |
| Acceso a función protegida sin sesión | Rechazo con indicación de iniciar sesión; la acción no se ejecuta. |
| Acceso a función protegida con sesión vencida | Idéntico al caso anterior. |
| Acceso a función protegida con sesión adulterada | Idéntico al caso anterior. |
| Acceso a función protegida con una sesión ya cerrada por su titular | Idéntico al caso anterior, aunque no hayan pasado las 8 horas. |
| Doble envío simultáneo del mismo registro | Una sola cuenta creada; el envío restante recibe el error de correo ya registrado. |

---

### Entidades Clave

- **Usuario**: representa a una persona que usa el sistema. Atributos relevantes: identificador
  propio estable, nombre completo, correo electrónico normalizado (único), verificador de contraseña
  no reversible, momento de creación y momento de última actualización. Es la entidad a la que se
  atribuyen todas las acciones del producto.
- **Sesión**: representa el permiso temporal de una persona identificada para operar en el sistema.
  Atributos relevantes: usuario al que pertenece, momento de emisión, momento de vencimiento (8
  horas después de la emisión) y si fue cerrada por su titular. Una sesión inexistente, vencida,
  cerrada o adulterada no otorga ningún permiso.
- **Evento de autenticación**: representa algo que ocurrió con las cuentas y que conviene poder
  revisar después. Atributos relevantes: momento del evento, tipo (alta de cuenta, inicio de sesión
  exitoso, intento fallido o cierre de sesión) y cuenta involucrada. Nunca contiene contraseñas.

---

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **SC-001**: Una persona nueva completa su registro en menos de 2 minutos y el 95 % lo logra en el
  primer intento, sin ayuda externa.
- **SC-002**: Una persona con cuenta inicia sesión y llega a una pantalla protegida en menos de 30
  segundos.
- **SC-003**: El 100 % de las funciones del sistema distintas de registro e inicio de sesión rechaza
  el acceso sin sesión válida, verificado con una prueba por cada función.
- **SC-004**: Ante credenciales inválidas, el mensaje devuelto es idéntico en el 100 % de los casos,
  sea el correo inexistente o la contraseña incorrecta: una persona ajena no puede determinar qué
  correos están registrados.
- **SC-005**: Cero apariciones de contraseñas en texto legible en almacenamiento, respuestas,
  mensajes de error y registros de actividad, verificado mediante revisión de todas las salidas de la
  feature.
- **SC-006**: Cero cuentas duplicadas con el mismo correo normalizado, incluso al enviar 10
  solicitudes de registro simultáneas con el mismo correo.
- **SC-007**: Una persona cuya sesión vence mientras opera entiende qué pasó y vuelve a estar
  operativa en menos de 1 minuto.
- **SC-008**: El 100 % de las reglas de negocio (RN-01 a RN-09) tiene al menos una prueba automatizada
  asociada que falla si la regla se rompe.
- **SC-009**: El 100 % de las altas de cuenta, inicios de sesión exitosos, intentos fallidos y
  cierres de sesión queda registrado con fecha, tipo y cuenta involucrada, verificable revisando el
  registro después de ejecutar cada uno de esos eventos.

---

## Fuera de Alcance

- Recuperación de contraseña olvidada.
- Verificación del correo electrónico (confirmación por enlace o código).
- Inicio de sesión con proveedores externos (redes sociales, cuentas corporativas, inicio de sesión
  único).
- Límite de intentos fallidos, demoras progresivas y bloqueo de cuentas.
- Pantalla de consulta del historial de accesos dentro de la aplicación (los eventos se registran,
  pero no se exponen en la interfaz).
- Roles globales de administración del sistema.
- Edición del perfil propio (cambio de nombre, correo o contraseña).
- Eliminación o desactivación de la cuenta.

---

## Supuestos

- **Registro abierto**: cualquier persona puede crear una cuenta sin invitación ni aprobación previa;
  no hay lista blanca de dominios de correo.
- **Sin inicio de sesión automático tras el registro**: decisión confirmada el 2026-09-20 y
  formalizada en FR-009. Evita duplicar la lógica de emisión de sesión en dos flujos (Principio I de
  la constitución).
- **Sin límite de intentos fallidos (riesgo aceptado)**: decisión confirmada el 2026-09-20. No se
  limita la cantidad de intentos de inicio de sesión, no se aplican demoras y no se bloquean
  cuentas. Se acepta de forma explícita que una cuenta con contraseña débil queda expuesta a la
  prueba automática de contraseñas; la protección se difiere a una feature propia si el riesgo se
  vuelve relevante.
- **Longitud máxima de correo**: se adopta 254 caracteres, el máximo habitual de una dirección de
  correo electrónico, por no haberse especificado un límite.
- **Contraseña sin requisitos adicionales**: no se exigen mayúsculas, símbolos ni ausencia de
  patrones comunes más allá de lo indicado en RN-04.
- **Unicidad sensible a la cuenta completa**: la normalización aplica al correo entero; no se
  interpretan alias con punto o con signo más como equivalentes (`a.n.a@example.com` y
  `ana@example.com` son cuentas distintas).
- **Idioma de la interfaz**: los mensajes que ve la persona están en español.
- **Dependencia hacia adelante**: las features siguientes del producto (proyectos, backlog, sprints,
  estimaciones, esfuerzo, defectos y métricas) asumen que existe un usuario identificado y que toda
  acción puede atribuírsele.

---

## Preguntas Abiertas

Ninguna. Las cinco decisiones pendientes se resolvieron en la sesión de clarificación del
2026-09-20 (ver la sección Clarifications).
