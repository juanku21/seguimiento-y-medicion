# Especificación de Feature: Gestión de Proyectos e Integrantes

**Directorio de feature**: `specs/002-project-members`

**Rama**: `002-project-members`

**Creada**: 2026-09-25

**Estado**: Clarificada — sin preguntas abiertas (sesión de clarificación del 2026-09-25)

**Entrada**: Descripción del usuario: "Gestión de proyectos e integrantes para Software Metrics &
Estimation, un sistema web multiusuario para estimar, planificar, seguir y medir proyectos de
software con Scrum. Depende de la feature de autenticación y cuentas de usuario (usuarios
registrados con sesión)."

---

## Objetivo

Permitir que un usuario registrado cree y administre proyectos de software, sume a otros usuarios
registrados como integrantes y consulte en cualquier momento el estado del proyecto.

El proyecto es la unidad de trabajo alrededor de la cual gira todo el producto: el backlog, los
sprints, las estimaciones, el esfuerzo registrado, los defectos y las métricas existen siempre
dentro de un proyecto y se atribuyen a sus integrantes. Sin proyectos con integrantes definidos no
hay ámbito al que referir ninguna medición, ni forma de decidir quién puede ver o registrar qué. El
factor de horas por Story Point se define en el proyecto porque es el traductor entre la estimación
relativa del equipo (Story Points) y el esfuerzo en horas que la organización necesita para
planificar.

---

## Clarifications

### Session 2026-09-25

- Q: ¿Se permite finalizar un proyecto que nunca pasó por "En curso", es decir, el salto directo de Planificado a Finalizado? (FR-037, FR-038) → A: no; solo se admiten las transiciones consecutivas Planificado → En curso → Finalizado.
- Q: ¿Se puede finalizar un proyecto mientras tiene un sprint activo (abierto)? (FR-041) → A: no; la finalización se rechaza mientras haya un sprint activo y hay que cerrarlo primero.
- Q: ¿La vista de estado del proyecto debe mostrar el total de horas estimadas del backlog (Story Points × factor)? (FR-042, FR-045) → A: sí; muestra el total de Story Points estimados del backlog y su equivalente en horas con el factor vigente; las historias sin estimar no suman.
- Q: ¿Según qué zona horaria se determina la "fecha del cambio" que queda como fecha de finalización real? (FR-039) → A: una zona horaria única del sistema, configurable (por ejemplo, America/Argentina/Buenos_Aires).
- Q: En la lista de integrantes del proyecto, ¿qué datos de cada persona ven los demás integrantes? (FR-030) → A: todos ven nombre completo, marca de propietario y fecha de incorporación; solo el propietario ve además el correo electrónico.

---

## Entradas y Salidas Esperadas

### Creación de un proyecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión válida, nombre, descripción (opcional), fecha de inicio, fecha de finalización prevista, factor de horas por Story Point | **Éxito**: proyecto creado en estado Planificado, con quien lo creó como propietario y único integrante |
| | **Error de validación**: detalle de qué campo es inválido y por qué; no se crea nada |
| | **Nombre repetido**: aviso de que ya existe un proyecto propio con ese nombre |

### Modificación de un proyecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión del propietario, identificador del proyecto y los datos a modificar (nombre, descripción, fechas, factor) | **Éxito**: proyecto actualizado con los nuevos datos; el estado y la fecha de finalización real no cambian por esta vía |
| | **Error de validación o nombre repetido**: rechazo con el motivo; el proyecto queda como estaba |
| | **Quien pide no es el propietario**: rechazo por falta de permiso |
| | **Proyecto Finalizado**: rechazo por proyecto de solo lectura |
| | **Quien pide no es integrante**: respuesta de proyecto inexistente |

### Alta de un integrante

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión del propietario, identificador del proyecto y correo electrónico de un usuario registrado | **Éxito**: el usuario queda integrante y desde ese momento puede consultar el proyecto |
| | **Correo sin cuenta**: aviso de que no existe un usuario registrado con ese correo |
| | **Ya es integrante**: aviso de que el usuario ya forma parte del proyecto |
| | **Quien pide no es el propietario, o el proyecto está Finalizado**: rechazo con el motivo |

### Baja de un integrante

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión del propietario, identificador del proyecto e identificación del integrante a quitar | **Éxito**: el integrante deja de tener acceso; lo que registró en el proyecto se conserva con su nombre |
| | **Intento de quitar al propietario**: rechazo indicando que el propietario no puede quitarse |
| | **La persona no es integrante**: aviso de que no forma parte del proyecto |
| | **Quien pide no es el propietario, o el proyecto está Finalizado**: rechazo con el motivo |

### Listado de proyectos propios

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión válida | Lista de los proyectos de los que la persona es integrante, cada uno con nombre, estado, fechas, cantidad de integrantes y si la persona es su propietario. Ningún proyecto ajeno aparece |
| Sesión válida sin proyectos | Lista vacía con indicación de que todavía no participa en ningún proyecto |

### Cambio de estado del proyecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión del propietario, identificador del proyecto y estado destino | **Éxito**: el proyecto pasa al estado destino; al pasar a Finalizado se registra la fecha de finalización real |
| | **Transición inválida** (retroceso, salto de estado o estado actual): rechazo indicando la transición permitida |
| | **Finalización con sprint activo**: rechazo indicando que primero hay que cerrar el sprint activo |
| | **Quien pide no es el propietario**: rechazo por falta de permiso |

### Consulta del estado de un proyecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de cualquier integrante e identificador del proyecto | Datos generales (nombre, descripción, propietario, factor de horas por Story Point), estado, fecha de inicio, fecha de finalización prevista, fecha de finalización real si existe, lista de integrantes, sprint activo si existe, cantidad de historias del backlog por estado, y total de Story Points estimados del backlog con su equivalente en horas |
| Proyecto sin backlog ni sprints | Los mismos datos, con los contadores del backlog, el total de Story Points y las horas estimadas en cero, y la indicación explícita de que no hay sprint activo |
| Sesión de quien no es integrante | Respuesta de proyecto inexistente, idéntica a la de un identificador que no existe |

---

## Escenarios de Usuario y Pruebas *(obligatorio)*

### Historia de Usuario 1 (US1) — Crear un proyecto (Prioridad: P1)

Una persona con cuenta crea un proyecto indicando su nombre, una descripción, cuándo empieza, cuándo
prevé terminar y cuántas horas equivale un Story Point en su equipo. Al crearlo queda como
propietario y como su primer integrante.

**Por qué esta prioridad**: sin proyecto no existe ningún otro concepto del producto (backlog,
sprints, esfuerzo, métricas). Es la primera porción que entrega valor observable sobre la feature de
autenticación ya construida.

**Prueba independiente**: se puede probar completa creando un proyecto y verificando que queda en
estado Planificado, con quien lo creó como propietario e integrante, sin necesitar ninguna otra
historia.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un usuario con sesión válida que no tiene proyectos, **cuando** crea el
   proyecto "Portal de Clientes" con fecha de inicio 2026-10-01, fecha de finalización prevista
   2026-12-20 y factor 6,5, **entonces** el proyecto queda creado en estado Planificado, con ese
   usuario como propietario y como único integrante, y sin fecha de finalización real.
2. *(Caso alternativo)* **Dado** un usuario con sesión válida, **cuando** crea un proyecto sin
   escribir descripción, **entonces** el proyecto se crea igual, porque la descripción es opcional.
3. *(Caso alternativo)* **Dado** que otro usuario ya tiene un proyecto llamado "Portal de Clientes",
   **cuando** este usuario crea su propio proyecto "Portal de Clientes", **entonces** el proyecto se
   crea, porque la unicidad del nombre solo aplica entre los proyectos de un mismo propietario.
4. *(Caso límite)* **Dado** un usuario con sesión válida, **cuando** crea un proyecto cuya fecha de
   finalización prevista es igual a la fecha de inicio, **entonces** el proyecto se crea.
5. *(Caso límite)* **Dado** un usuario con sesión válida, **cuando** crea un proyecto con factor
   0,01 y luego otro con factor 40, **entonces** ambos se crean, porque son el mínimo y el máximo
   admitidos.
6. *(Caso límite)* **Dado** un usuario con sesión válida, **cuando** crea un proyecto cuyo nombre
   tiene exactamente 100 caracteres, **entonces** el proyecto se crea; y **cuando** tiene 101, se
   rechaza.
7. *(Caso de error)* **Dado** un usuario con sesión válida, **cuando** intenta crear un proyecto sin
   nombre, sin fecha de inicio, sin fecha de finalización prevista o sin factor, **entonces** la
   creación se rechaza indicando qué campo corregir y no se crea ningún proyecto.
8. *(Caso de error)* **Dado** un usuario con sesión válida, **cuando** intenta crear un proyecto con
   factor 0, con factor negativo, con factor 40,01 o con factor 6,555 (tres decimales), **entonces**
   la creación se rechaza indicando el rango y la precisión admitidos.
9. *(Caso de error)* **Dado** un usuario con sesión válida, **cuando** intenta crear un proyecto cuya
   fecha de finalización prevista es anterior a la fecha de inicio, **entonces** la creación se
   rechaza indicando que la fecha de finalización prevista no puede ser anterior a la de inicio.
10. *(Caso de error)* **Dado** un usuario que ya tiene un proyecto llamado "Portal de Clientes",
    **cuando** intenta crear otro llamado "  portal de clientes  ", **entonces** la creación se
    rechaza por nombre repetido y no se crea un segundo proyecto.
11. *(Caso de error)* **Dado** alguien sin sesión válida, **cuando** intenta crear un proyecto,
    **entonces** la acción se rechaza pidiendo iniciar sesión y no se crea nada.

---

### Historia de Usuario 2 (US2) — Ver mis proyectos y quedar aislado de los ajenos (Prioridad: P1)

Una persona con cuenta entra al sistema y ve la lista de los proyectos de los que participa, sin ver
ni poder alcanzar los proyectos de otras personas.

**Por qué esta prioridad**: es la contracara imprescindible de la creación. Sin listado la persona no
puede volver a su trabajo, y sin aislamiento el sistema multiusuario no es usable en un entorno
compartido.

**Prueba independiente**: se puede probar completa creando proyectos con dos usuarios distintos y
verificando que cada uno ve solo los suyos y que el identificador del proyecto ajeno se comporta como
inexistente.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un usuario que es propietario de dos proyectos e integrante de un tercero,
   **cuando** consulta su listado, **entonces** ve los tres, cada uno con su nombre, estado, fechas,
   cantidad de integrantes y la indicación de si es su propietario.
2. *(Caso alternativo)* **Dado** un usuario recién registrado que no participa en ningún proyecto,
   **cuando** consulta su listado, **entonces** obtiene una lista vacía con la indicación de que
   todavía no participa en ningún proyecto.
3. *(Caso alternativo)* **Dado** un usuario que era integrante de un proyecto y fue quitado,
   **cuando** consulta su listado, **entonces** ese proyecto ya no aparece.
4. *(Caso límite)* **Dado** un proyecto cuyo único integrante es su propietario, **cuando** el
   propietario consulta su listado, **entonces** el proyecto aparece con un integrante.
5. *(Caso de error)* **Dado** un usuario que no es integrante de un proyecto existente, **cuando**
   intenta consultarlo con su identificador, **entonces** la respuesta es de proyecto inexistente,
   idéntica a la que se obtiene con un identificador que no existe.
6. *(Caso de error)* **Dado** alguien sin sesión válida, **cuando** intenta consultar el listado,
   **entonces** la acción se rechaza pidiendo iniciar sesión.

---

### Historia de Usuario 3 (US3) — Gestionar los integrantes del proyecto (Prioridad: P2)

El propietario suma al proyecto a otros usuarios registrados buscándolos por su correo electrónico y
quita a quienes ya no participan, sin perder lo que esas personas registraron.

**Por qué esta prioridad**: convierte el proyecto en un espacio de equipo, que es la premisa de todo
el producto (votación de estimaciones, registro de esfuerzo, defectos). Depende de que el proyecto
exista, por eso va después de US1 y US2.

**Prueba independiente**: se puede probar completa con dos cuentas: el propietario agrega a la
segunda por correo, verifica que esa persona ya ve el proyecto, la quita y verifica que deja de
verlo.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto del que soy propietario y un usuario registrado con correo
   `bruno@example.com`, **cuando** lo agrego como integrante, **entonces** queda registrado como
   integrante y desde ese momento puede consultar el proyecto y verlo en su listado.
2. *(Caso alternativo)* **Dado** el mismo proyecto y el usuario registrado con correo
   `bruno@example.com`, **cuando** lo agrego escribiendo `  Bruno@Example.COM  `, **entonces** el
   sistema lo encuentra igual, porque el correo se compara sin espacios extremos y sin distinguir
   mayúsculas de minúsculas.
3. *(Caso alternativo)* **Dado** un integrante que fue quitado del proyecto, **cuando** el
   propietario lo vuelve a agregar, **entonces** recupera el acceso y sigue apareciendo una sola vez
   en la lista de integrantes.
4. *(Caso alternativo)* **Dado** un proyecto con propietario y dos integrantes más, **cuando** el
   propietario consulta la lista de integrantes, **entonces** ve de cada persona su nombre completo,
   su correo, la marca de propietario y la fecha de incorporación; y **cuando** la consulta un
   integrante que no es el propietario, **entonces** ve los mismos datos salvo los correos.
5. *(Caso límite)* **Dado** un proyecto cuyo único integrante es su propietario, **cuando** el
   propietario consulta la lista de integrantes, **entonces** se ve a sí mismo marcado como
   propietario y la lista no está vacía.
6. *(Caso límite)* **Dado** un integrante que ya registró esfuerzo, votos de estimación y defectos en
   el proyecto, **cuando** el propietario lo quita, **entonces** esos registros siguen existiendo y
   siguen mostrando su nombre; lo único que cambia es que la persona pierde el acceso.
7. *(Caso de error)* **Dado** un proyecto del que soy propietario, **cuando** intento agregar el
   correo `nadie@example.com`, que no corresponde a ningún usuario registrado, **entonces** el alta
   se rechaza informando que no existe un usuario registrado con ese correo.
8. *(Caso de error)* **Dado** un proyecto en el que `bruno@example.com` ya es integrante, **cuando**
   intento agregarlo otra vez, **entonces** el alta se rechaza informando que ya forma parte del
   proyecto y la lista de integrantes no cambia.
9. *(Caso de error)* **Dado** un proyecto del que soy integrante pero no propietario, **cuando**
   intento agregar o quitar un integrante, **entonces** la acción se rechaza por falta de permiso y
   la lista de integrantes no cambia.
10. *(Caso de error)* **Dado** un proyecto del que soy propietario, **cuando** intento quitarme a mí
    mismo, **entonces** la acción se rechaza indicando que el propietario no puede quitarse del
    proyecto.
11. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** el propietario intenta
    agregar o quitar un integrante, **entonces** la acción se rechaza porque el proyecto es de solo
    lectura.

---

### Historia de Usuario 4 (US4) — Consultar el estado del proyecto (Prioridad: P2)

Cualquier integrante abre el proyecto y ve de un vistazo en qué situación está: sus datos generales,
su estado, sus fechas, quiénes participan, qué sprint está activo y cómo está repartido el backlog.

**Por qué esta prioridad**: es la vista que responde "¿cómo va el proyecto?", el propósito declarado
del producto. Va después de la gestión de integrantes porque necesita mostrarlos.

**Prueba independiente**: se puede probar completa creando un proyecto, agregando un integrante y
verificando que ambos obtienen la vista de estado con los datos correctos, incluso sin backlog ni
sprints.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto En curso con tres integrantes, un sprint activo y un backlog
   con historias en distintos estados, **cuando** un integrante consulta el estado, **entonces**
   obtiene los datos generales, el estado, la fecha de inicio, la fecha de finalización prevista, el
   factor de horas por Story Point, la lista de integrantes con el propietario identificado, el
   sprint activo, la cantidad de historias del backlog por estado y el total de Story Points
   estimados con su equivalente en horas.
2. *(Caso alternativo)* **Dado** un proyecto Finalizado, **cuando** un integrante consulta el estado,
   **entonces** además de los demás datos ve la fecha de finalización real y la indicación de que el
   proyecto es de solo lectura.
3. *(Caso alternativo)* **Dado** un proyecto sin sprint activo (todos cerrados o ninguno creado),
   **cuando** un integrante consulta el estado, **entonces** la vista indica explícitamente que no
   hay sprint activo, sin mostrarse como un error.
4. *(Caso límite)* **Dado** un proyecto recién creado, sin backlog y sin sprints, **cuando** su
   propietario consulta el estado, **entonces** obtiene los datos generales con los contadores de
   historias por estado, el total de Story Points y las horas estimadas en cero, y la indicación de
   que no hay sprint activo.
5. *(Caso límite)* **Dado** un proyecto con factor 6,5 cuyo backlog tiene historias estimadas en 3,
   5 y 8 Story Points y una historia sin estimar, **cuando** un integrante consulta el estado,
   **entonces** ve un total de 16 Story Points y 104 horas estimadas; la historia sin estimar no
   suma.
6. *(Caso de error)* **Dado** un proyecto del que no soy integrante, **cuando** intento consultar su
   estado, **entonces** la respuesta es de proyecto inexistente y no revela ningún dato del proyecto,
   ni siquiera su nombre.
7. *(Caso de error)* **Dado** alguien sin sesión válida, **cuando** intenta consultar el estado de un
   proyecto, **entonces** la acción se rechaza pidiendo iniciar sesión.

---

### Historia de Usuario 5 (US5) — Modificar los datos del proyecto (Prioridad: P3)

El propietario corrige el nombre, la descripción, las fechas o el factor de horas por Story Point a
medida que el proyecto cambia.

**Por qué esta prioridad**: es un ajuste sobre algo que ya funciona. El producto es utilizable sin
esta historia, aunque con datos rígidos.

**Prueba independiente**: se puede probar completa creando un proyecto, modificando cada uno de sus
campos y verificando que la vista de estado refleja los nuevos valores.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto del que soy propietario en estado Planificado, **cuando**
   cambio su nombre, su descripción, sus fechas y su factor por valores válidos, **entonces** el
   proyecto queda actualizado y su estado y su fecha de finalización real no cambian.
2. *(Caso alternativo)* **Dado** un proyecto En curso, **cuando** el propietario corre la fecha de
   finalización prevista a una fecha posterior, **entonces** el cambio se acepta, porque solo el
   estado Finalizado bloquea las modificaciones.
3. *(Caso alternativo)* **Dado** un proyecto del que soy propietario, **cuando** lo renombro con el
   mismo nombre que ya tenía, **entonces** el cambio se acepta y no se rechaza por nombre repetido
   contra sí mismo.
4. *(Caso límite)* **Dado** un proyecto con dos sprints ya cerrados y uno en curso, **cuando** el
   propietario cambia el factor de 6 a 8, **entonces** los sprints cerrados conservan el factor que
   estaba vigente al momento de su cierre y el nuevo factor se aplica al proyecto y a los sprints que
   se cierren en adelante.
5. *(Caso límite)* **Dado** un proyecto del que soy propietario, **cuando** cambio el factor al
   mínimo (0,01) o al máximo (40) admitidos, **entonces** el cambio se acepta.
6. *(Caso de error)* **Dado** un proyecto del que soy propietario y otro proyecto propio llamado
   "Portal de Clientes", **cuando** intento renombrar el primero como "Portal de Clientes",
   **entonces** la modificación se rechaza por nombre repetido y el proyecto queda como estaba.
7. *(Caso de error)* **Dado** un proyecto del que soy propietario, **cuando** intento dejar el nombre
   vacío, poner una fecha de finalización prevista anterior a la de inicio, o un factor fuera del
   rango o con más de dos decimales, **entonces** la modificación se rechaza indicando el motivo y no
   se aplica ningún cambio parcial.
8. *(Caso de error)* **Dado** un proyecto del que soy integrante pero no propietario, **cuando**
   intento modificarlo, **entonces** la acción se rechaza por falta de permiso.
9. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** su propietario intenta
   modificar cualquier dato, **entonces** la acción se rechaza porque el proyecto es de solo lectura.

---

### Historia de Usuario 6 (US6) — Avanzar el estado del proyecto (Prioridad: P3)

El propietario marca el arranque del proyecto y, cuando termina, lo cierra; al cerrarlo queda
registrada la fecha real de finalización para poder compararla con la prevista.

**Por qué esta prioridad**: aporta trazabilidad del ciclo de vida y habilita la comparación entre lo
previsto y lo real, pero el trabajo diario puede registrarse sin ella.

**Prueba independiente**: se puede probar completa creando un proyecto y recorriendo Planificado → En
curso → Finalizado, verificando la fecha de finalización real y el rechazo de las transiciones no
permitidas.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto Planificado del que soy propietario, **cuando** lo paso a En
   curso y más tarde a Finalizado, **entonces** ambas transiciones se aceptan y al finalizar queda
   registrada la fecha de finalización real con la fecha del cambio.
2. *(Caso alternativo)* **Dado** un proyecto que se finaliza antes de la fecha de finalización
   prevista, **cuando** el propietario lo finaliza, **entonces** la transición se acepta y la fecha
   de finalización real queda anterior a la prevista, sin que eso sea un error.
3. *(Caso alternativo)* **Dado** un proyecto Finalizado, **cuando** cualquier integrante lo consulta,
   **entonces** puede verlo completo pero ninguna acción de administración se admite.
4. *(Caso límite)* **Dado** un proyecto que se finaliza el mismo día en que comenzó, **cuando** el
   propietario lo finaliza, **entonces** la transición se acepta y la fecha de finalización real es
   igual a la fecha de inicio.
5. *(Caso límite)* **Dado** un proyecto Planificado con un sprint activo o con backlog cargado,
   **cuando** el propietario lo pasa a En curso, **entonces** la transición se acepta sin exigir
   ninguna condición adicional sobre el backlog ni sobre los sprints.
6. *(Caso de error)* **Dado** un proyecto En curso, **cuando** el propietario intenta devolverlo a
   Planificado, **entonces** la transición se rechaza indicando que los estados solo avanzan.
7. *(Caso de error)* **Dado** un proyecto Planificado, **cuando** el propietario intenta pasarlo
   directamente a Finalizado, **entonces** la transición se rechaza indicando que debe pasar primero
   por En curso.
8. *(Caso de error)* **Dado** un proyecto En curso, **cuando** el propietario intenta pasarlo a En
   curso otra vez, **entonces** la transición se rechaza por inválida y nada cambia.
9. *(Caso de error)* **Dado** un proyecto Finalizado, **cuando** el propietario intenta cambiar su
   estado, **entonces** la acción se rechaza porque el proyecto es de solo lectura y la fecha de
   finalización real no se altera.
10. *(Caso de error)* **Dado** un proyecto del que soy integrante pero no propietario, **cuando**
    intento cambiar su estado, **entonces** la acción se rechaza por falta de permiso.
11. *(Caso de error)* **Dado** un proyecto En curso con un sprint activo, **cuando** el propietario
    intenta pasarlo a Finalizado, **entonces** la transición se rechaza indicando que primero debe
    cerrar el sprint activo, y el proyecto sigue En curso sin fecha de finalización real.

---

### Casos Límite

- **Fecha de finalización prevista igual a la fecha de inicio**: válida; solo se rechaza si es
  anterior.
- **Factor en el mínimo y en el máximo admitidos**: 0,01 y 40 son válidos; 0 y 40,01 se rechazan.
- **Factor con más de dos decimales**: se rechaza, no se redondea, para que el valor que la persona
  ve sea exactamente el que se usa en los cálculos.
- **Correo del integrante con otra capitalización o con espacios extremos**: `  Bruno@Example.COM  `
  y `bruno@example.com` identifican al mismo usuario, igual que en la feature de autenticación.
- **Proyecto sin integrantes además del propietario**: estado válido y esperable; la lista de
  integrantes siempre tiene al menos una persona.
- **Consulta de estado de un proyecto sin backlog ni sprints**: los contadores de historias por
  estado, el total de Story Points y las horas estimadas son cero, y la ausencia de sprint activo se
  informa explícitamente; no es un error.
- **Historias sin estimar en el backlog**: se cuentan en el reparto por estado, pero no suman al
  total de Story Points ni a las horas estimadas.
- **Cambio del factor con sprints ya cerrados**: los sprints cerrados conservan el factor vigente al
  momento de su cierre; el nuevo factor rige para el proyecto y para los cierres posteriores. Las
  métricas históricas no se recalculan hacia atrás.
- **Nombre de exactamente 100 caracteres**: válido; 101 se rechaza. El conteo se hace después de
  recortar los espacios al inicio y al final.
- **Nombre compuesto solo por espacios**: equivale a nombre ausente y se rechaza.
- **Nombre repetido con distinta capitalización o con espacios extremos**: se considera el mismo
  nombre para la unicidad entre los proyectos de un mismo propietario.
- **Mismo nombre de proyecto en propietarios distintos**: permitido; la unicidad es por propietario.
- **Dos altas simultáneas del mismo integrante**: se registra una sola membresía; el intento perdedor
  recibe el error de integrante duplicado.
- **Dos creaciones simultáneas del mismo nombre por el mismo propietario**: se crea un solo proyecto;
  el intento perdedor recibe el error de nombre repetido.
- **Integrante quitado que tenía la vista del proyecto abierta**: su siguiente acción sobre el
  proyecto se rechaza como si el proyecto no existiera.
- **Integrante quitado que había registrado esfuerzo, votos o defectos**: el historial se conserva
  con su nombre; el proyecto nunca queda con registros sin autor identificable.
- **Proyecto Finalizado**: admite consulta por parte de cualquier integrante y rechaza toda acción de
  administración, incluida la baja de integrantes.
- **Finalización cerca de la medianoche**: la fecha de finalización real es la fecha calendario del
  momento del cambio en la zona horaria del sistema; si el proyecto se finaliza a las 23:30 hora del
  sistema, la fecha real es ese mismo día aunque en UTC ya sea el día siguiente.
- **Finalización con un sprint activo**: se rechaza; el propietario debe cerrar antes el sprint
  activo, de modo que ningún proyecto Finalizado queda con un sprint abierto.

---

## Requisitos *(obligatorio)*

### Requisitos Funcionales

**Creación del proyecto**

- **FR-001**: El sistema DEBE permitir a un usuario con sesión válida crear un proyecto a partir de
  nombre, descripción, fecha de inicio, fecha de finalización prevista y factor de horas por Story
  Point.
- **FR-002**: El sistema DEBE exigir un nombre de entre 1 y 100 caracteres, contados después de
  recortar los espacios al inicio y al final; un nombre ausente o compuesto solo por espacios se
  rechaza.
- **FR-003**: El sistema DEBE tratar la descripción como opcional y admitir hasta 1000 caracteres.
- **FR-004**: El sistema DEBE garantizar que el nombre no se repita entre los proyectos de un mismo
  propietario, comparándolo sin distinguir mayúsculas de minúsculas y sin los espacios extremos. El
  mismo nombre en proyectos de propietarios distintos está permitido.
- **FR-005**: El sistema DEBE exigir fecha de inicio y fecha de finalización prevista, y rechazar la
  creación si la fecha de finalización prevista es anterior a la fecha de inicio; ambas iguales es
  válido.
- **FR-006**: El sistema DEBE exigir un factor de horas por Story Point mayor que 0, menor o igual a
  40 y con hasta 2 decimales; un valor con más de 2 decimales se rechaza sin redondear.
- **FR-007**: El sistema DEBE registrar a quien crea el proyecto como su propietario y, en el mismo
  acto, como integrante.
- **FR-008**: Todo proyecto DEBE nacer en estado Planificado y sin fecha de finalización real.
- **FR-009**: Cada proyecto DEBE tener un identificador propio, estable e inmutable, distinto del
  nombre.
- **FR-010**: El sistema DEBE rechazar una creación inválida indicando qué campo corregir y por qué,
  sin crear el proyecto ni ninguna membresía.
- **FR-011**: El sistema DEBE crear un único proyecto cuando el mismo propietario envía dos
  solicitudes simultáneas con el mismo nombre; la solicitud restante se rechaza por nombre repetido.

**Modificación del proyecto**

- **FR-012**: El sistema DEBE permitir al propietario modificar el nombre, la descripción, la fecha
  de inicio, la fecha de finalización prevista y el factor de horas por Story Point.
- **FR-013**: El sistema DEBE aplicar en la modificación exactamente las mismas validaciones de
  nombre, descripción, fechas y factor que en la creación (FR-002 a FR-006).
- **FR-014**: Al validar la unicidad del nombre en una modificación, el sistema DEBE excluir al
  propio proyecto que se está modificando.
- **FR-015**: La modificación DEBE ser atómica: si algún dato es inválido, ningún cambio se aplica.
- **FR-016**: La modificación del proyecto NO DEBE alterar su estado ni su fecha de finalización
  real.
- **FR-017**: Un cambio del factor de horas por Story Point DEBE regir para el proyecto y para los
  sprints que se cierren en adelante; los sprints ya cerrados conservan el factor vigente al momento
  de su cierre y sus métricas no se recalculan.
- **FR-018**: Un proyecto en estado Finalizado DEBE ser de solo lectura: el sistema rechaza toda
  modificación de datos, todo cambio de estado y toda alta o baja de integrantes.

**Gestión de integrantes**

- **FR-019**: El sistema DEBE permitir al propietario agregar como integrante a un usuario registrado
  identificándolo por su correo electrónico.
- **FR-020**: El sistema DEBE normalizar el correo antes de buscar al usuario, recortando los espacios
  extremos y sin distinguir mayúsculas de minúsculas, igual que la feature de autenticación.
- **FR-021**: El sistema DEBE rechazar el alta cuando el correo no corresponde a ningún usuario
  registrado, informando esa situación.
- **FR-022**: El sistema DEBE rechazar el alta cuando el usuario ya es integrante del proyecto, sin
  duplicar la membresía.
- **FR-023**: El sistema DEBE registrar una única membresía cuando se reciben dos altas simultáneas
  del mismo usuario en el mismo proyecto; el intento restante se rechaza por integrante duplicado.
- **FR-024**: El sistema DEBE permitir al propietario quitar a cualquier integrante del proyecto
  excepto a sí mismo.
- **FR-025**: El sistema DEBE rechazar el intento del propietario de quitarse del proyecto,
  indicando el motivo.
- **FR-026**: El sistema DEBE rechazar la baja de una persona que no es integrante del proyecto.
- **FR-027**: Quitar a un integrante DEBE eliminar únicamente su acceso al proyecto: el esfuerzo, los
  votos de estimación, los defectos y cualquier otro registro que haya generado se conservan y siguen
  mostrando su nombre.
- **FR-028**: Una persona quitada del proyecto DEBE perder el acceso de inmediato, y el proyecto DEBE
  comportarse para ella como inexistente.
- **FR-029**: El sistema DEBE permitir volver a agregar a una persona que había sido quitada, sin
  duplicar su membresía ni su historial.
- **FR-030**: El sistema DEBE permitir a cualquier integrante ver la lista de integrantes del
  proyecto con el nombre completo de cada persona, la marca de propietario y la fecha de
  incorporación. El correo electrónico de los integrantes DEBE mostrarse únicamente al propietario;
  a los demás integrantes el sistema NO DEBE revelarlo por ninguna vía de esta feature.

**Listado de proyectos**

- **FR-031**: El sistema DEBE permitir a un usuario con sesión válida listar los proyectos de los que
  es integrante, incluidos los propios.
- **FR-032**: Cada elemento del listado DEBE incluir al menos nombre, estado, fecha de inicio, fecha
  de finalización prevista, cantidad de integrantes y si quien consulta es el propietario.
- **FR-033**: El listado NO DEBE incluir ningún proyecto del que quien consulta no sea integrante.
- **FR-034**: Cuando la persona no participa en ningún proyecto, el sistema DEBE devolver un listado
  vacío con una indicación clara, no un error.
- **FR-035**: El listado DEBE presentarse en un orden estable y predecible: fecha de inicio
  descendente y, a igual fecha, nombre ascendente.

**Ciclo de vida del proyecto**

- **FR-036**: El proyecto DEBE tener exactamente uno de estos tres estados: Planificado, En curso o
  Finalizado.
- **FR-037**: El sistema DEBE admitir únicamente las transiciones Planificado → En curso y En curso →
  Finalizado.
- **FR-038**: El sistema DEBE rechazar cualquier otra transición, incluidos el retroceso a un estado
  anterior, el salto de Planificado a Finalizado y la transición al estado actual, indicando la
  transición permitida.
- **FR-039**: Al pasar el proyecto a Finalizado, el sistema DEBE registrar la fecha de finalización
  real con la fecha en que ocurre el cambio, calculada en la zona horaria única y configurable del
  sistema, independientemente de la zona horaria del dispositivo de quien finaliza.
- **FR-040**: La fecha de finalización real NO DEBE poder editarse ni borrarse por ninguna vía de
  esta feature.
- **FR-041**: El sistema DEBE permitir la transición Planificado → En curso sin exigir condiciones
  sobre el backlog ni sobre los sprints. La transición En curso → Finalizado DEBE rechazarse mientras
  el proyecto tenga un sprint activo, indicando que primero hay que cerrarlo; no se exige ninguna
  otra condición sobre el backlog ni sobre los sprints.

**Consulta del estado del proyecto**

- **FR-042**: El sistema DEBE permitir a cualquier integrante consultar el estado del proyecto y
  devolver: nombre, descripción, propietario, factor de horas por Story Point, estado, fecha de
  inicio, fecha de finalización prevista, fecha de finalización real cuando exista, lista de
  integrantes, sprint activo cuando exista, cantidad de historias del backlog por estado, total de
  Story Points estimados del backlog y su equivalente en horas estimadas.
- **FR-043**: Cuando el proyecto no tiene sprint activo, el sistema DEBE informarlo de forma
  explícita, sin tratarlo como error.
- **FR-044**: Cuando el proyecto no tiene historias en el backlog, el sistema DEBE devolver los
  contadores por estado, el total de Story Points y las horas estimadas en cero.
- **FR-045**: El sistema DEBE calcular las horas estimadas del proyecto multiplicando el total de
  Story Points estimados del backlog por el factor vigente del proyecto; las historias sin estimar
  no suman al total.

**Autorización y visibilidad**

- **FR-046**: El sistema DEBE exigir una sesión válida para toda acción de esta feature, sin
  excepciones.
- **FR-047**: El sistema DEBE restringir la modificación del proyecto, el cambio de estado y el alta
  y baja de integrantes al propietario del proyecto.
- **FR-048**: El sistema DEBE permitir la consulta del proyecto, de su estado y de sus integrantes a
  cualquier integrante.
- **FR-049**: Ante una solicitud sobre un proyecto del que quien pide no es integrante, el sistema
  DEBE responder exactamente igual que ante un identificador inexistente, sin revelar ningún dato del
  proyecto ni su existencia.
- **FR-050**: Ante una acción de administración solicitada por un integrante que no es el
  propietario, el sistema DEBE responder por falta de permiso, y la acción no DEBE producir ningún
  efecto.
- **FR-051**: Toda creación, modificación, cambio de estado y alta o baja de integrantes DEBE quedar
  atribuida al titular de la sesión que la ejecutó.

---

### Reglas de Negocio

| ID | Regla |
| --- | --- |
| RN-01 | Quien crea el proyecto es su propietario y queda automáticamente registrado como integrante. |
| RN-02 | Solo el propietario modifica el proyecto, cambia su estado y agrega o quita integrantes. |
| RN-03 | Cualquier integrante puede consultar el proyecto; quien no es integrante no puede distinguirlo de un proyecto inexistente. |
| RN-04 | El nombre del proyecto es obligatorio, mide hasta 100 caracteres y no se repite entre los proyectos de un mismo propietario. |
| RN-05 | La comparación de nombres de proyecto y de correos de integrantes ignora mayúsculas/minúsculas y espacios extremos. |
| RN-06 | La fecha de finalización prevista nunca es anterior a la fecha de inicio. |
| RN-07 | El factor de horas por Story Point es obligatorio, mayor que 0, menor o igual a 40 y con hasta 2 decimales. |
| RN-08 | Las horas estimadas del proyecto son Story Points × factor de horas por Story Point. |
| RN-09 | Los estados solo avanzan: Planificado → En curso → Finalizado. No hay retrocesos ni saltos. |
| RN-10 | Al finalizar un proyecto se registra la fecha de finalización real, que después no se modifica. |
| RN-11 | Un proyecto Finalizado es de solo lectura. |
| RN-12 | El propietario no puede quitarse a sí mismo del proyecto. |
| RN-13 | Un usuario no puede ser integrante dos veces del mismo proyecto. |
| RN-14 | Quitar a un integrante no borra lo que registró; el historial se conserva con su nombre. |
| RN-15 | Un sprint cerrado conserva el factor de horas por Story Point vigente al momento de su cierre. |
| RN-16 | Un proyecto no puede finalizarse mientras tenga un sprint activo. |

---

### Restricciones

- **RC-01**: Esta feature depende de la feature de autenticación y cuentas de usuario
  (`specs/001-user-auth`): solo usuarios registrados con sesión válida operan, y los integrantes se
  buscan por el correo normalizado de una cuenta existente.
- **RC-02**: Solo se agregan al proyecto usuarios que ya tienen cuenta. No existe forma de sumar a
  alguien que no se registró todavía.
- **RC-03**: La respuesta ante un correo sin cuenta registrada revela que ese correo no está
  registrado en el sistema. Es una exposición aceptada de forma explícita, porque sin ese mensaje el
  propietario no puede distinguir un correo mal escrito de un problema de permisos. La feature de
  autenticación mantiene su mensaje genérico en el inicio de sesión.
- **RC-04**: El propietario es único y no se transfiere dentro de esta feature: si la persona
  propietaria deja de participar, el proyecto queda sin quien lo administre hasta que se especifique
  la transferencia de propiedad.
- **RC-05**: No existe forma de eliminar ni archivar un proyecto en esta feature; el único cierre
  posible es la transición a Finalizado.
- **RC-06**: El sprint activo, las cantidades de historias del backlog y los Story Points de cada
  historia que usa la consulta de estado son datos que producen las features de backlog y sprints; esta feature define qué se muestra
  y con qué criterio de visibilidad, no cómo se generan.
- **RC-07**: El cambio del factor de horas por Story Point altera las horas estimadas que se
  muestran en adelante; las métricas de sprints ya cerrados quedan congeladas, de modo que dos
  sprints del mismo proyecto pueden haberse calculado con factores distintos.

---

### Condiciones de Error

| Condición | Comportamiento esperado |
| --- | --- |
| Acción de la feature sin sesión válida | Rechazo con indicación de iniciar sesión; la acción no se ejecuta. |
| Nombre de proyecto ausente o compuesto solo por espacios | Rechazo indicando que el nombre es obligatorio. |
| Nombre de proyecto de más de 100 caracteres | Rechazo indicando el máximo permitido. |
| Descripción de más de 1000 caracteres | Rechazo indicando el máximo permitido. |
| Fecha de inicio o fecha de finalización prevista ausente | Rechazo indicando el campo faltante. |
| Fecha de finalización prevista anterior a la fecha de inicio | Rechazo indicando que no puede ser anterior a la de inicio. |
| Factor ausente, igual a 0, negativo o mayor que 40 | Rechazo indicando el rango permitido. |
| Factor con más de 2 decimales | Rechazo indicando la precisión permitida, sin redondear el valor. |
| Nombre repetido entre los proyectos del mismo propietario | Rechazo informando que ya existe un proyecto propio con ese nombre. |
| Dos creaciones simultáneas del mismo nombre por el mismo propietario | Un solo proyecto creado; la solicitud restante recibe el error de nombre repetido. |
| Correo que no corresponde a ningún usuario registrado | Rechazo informando que no existe un usuario registrado con ese correo. |
| Usuario que ya es integrante del proyecto | Rechazo informando que ya forma parte del proyecto; la membresía no se duplica. |
| Dos altas simultáneas del mismo integrante | Una sola membresía; el intento restante recibe el error de integrante duplicado. |
| Baja de una persona que no es integrante | Rechazo informando que no forma parte del proyecto. |
| Intento del propietario de quitarse a sí mismo | Rechazo indicando que el propietario no puede quitarse del proyecto. |
| Acción de administración ejecutada por un integrante que no es el propietario | Rechazo por falta de permiso; ningún efecto sobre el proyecto. |
| Transición de estado inválida (retroceso, salto o estado actual) | Rechazo indicando la transición permitida; el estado no cambia. |
| Finalización de un proyecto con un sprint activo | Rechazo indicando que primero hay que cerrar el sprint activo; el proyecto sigue En curso. |
| Cualquier modificación, cambio de estado o alta/baja de integrantes sobre un proyecto Finalizado | Rechazo por proyecto de solo lectura. |
| Solicitud sobre un proyecto del que quien pide no es integrante | Respuesta idéntica a la de un proyecto inexistente, sin revelar ningún dato. |
| Solicitud sobre un identificador de proyecto que no existe | Respuesta de proyecto inexistente. |

---

### Entidades Clave

- **Proyecto**: representa un emprendimiento de software que el equipo estima, planifica, sigue y
  mide. Atributos relevantes: identificador propio estable, nombre (único por propietario),
  descripción, fecha de inicio, fecha de finalización prevista, fecha de finalización real (solo
  cuando está Finalizado), factor de horas por Story Point vigente, estado, propietario, momento de
  creación y momento de última actualización. Es el ámbito al que pertenecen el backlog, los sprints
  y todas las mediciones.
- **Integrante del proyecto (membresía)**: representa la participación de un usuario en un proyecto y
  es lo que habilita su acceso. Atributos relevantes: proyecto, usuario, si es el propietario y
  momento en que se incorporó. Un mismo usuario tiene como máximo una membresía vigente por
  proyecto; quitarla retira el acceso sin borrar lo que la persona registró.
- **Estado del proyecto**: representa el punto del ciclo de vida en que está el proyecto. Valores
  posibles: Planificado, En curso y Finalizado, recorridos en ese orden y sin vuelta atrás.
- **Usuario** *(entidad de la feature de autenticación)*: persona identificada por su correo
  normalizado. Esta feature lo consume para buscar integrantes y para atribuir acciones; no lo
  modifica.
- **Sprint** e **Historia del backlog** *(entidades de features propias)*: esta feature solo las
  consulta para informar el sprint activo y la cantidad de historias por estado, y para calcular las
  horas estimadas con el factor del proyecto.

---

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **SC-001**: Una persona con cuenta crea su primer proyecto en menos de 2 minutos y el 95 % lo
  logra en el primer intento, sin ayuda externa.
- **SC-002**: El propietario suma un integrante conociendo su correo en menos de 30 segundos y en no
  más de 3 pasos.
- **SC-003**: Un integrante obtiene la vista de estado del proyecto (datos generales, estado, fechas,
  integrantes, sprint activo, reparto del backlog y horas estimadas) en una sola pantalla y en menos de 3 segundos,
  sin tener que combinar información de otros lugares del sistema.
- **SC-004**: El 100 % de las acciones de esta feature exige sesión válida, verificado con una prueba
  por acción.
- **SC-005**: Cero filtraciones de datos entre proyectos: en el 100 % de los intentos de acceso de
  quien no es integrante, la respuesta es indistinguible de la de un proyecto inexistente, verificado
  comparando ambas respuestas para cada acción de la feature.
- **SC-006**: El 100 % de las acciones de administración (modificar, cambiar de estado, agregar y
  quitar integrantes) se rechaza cuando la pide un integrante que no es el propietario, sin producir
  ningún cambio.
- **SC-007**: Cero proyectos con nombre repetido para un mismo propietario, incluso al enviar 10
  solicitudes de creación simultáneas con el mismo nombre.
- **SC-008**: Cero membresías duplicadas, incluso al enviar 10 altas simultáneas del mismo usuario en
  el mismo proyecto.
- **SC-009**: El 100 % de las transiciones de estado no permitidas se rechaza, verificado probando
  las seis combinaciones de estado origen y destino distintas de las dos transiciones válidas.
- **SC-010**: El 100 % de los proyectos finalizados tiene fecha de finalización real registrada, y
  ninguna de esas fechas cambia después de registrarse.
- **SC-011**: Cero registros de esfuerzo, votos o defectos perdidos o sin autor identificable después
  de quitar a un integrante, verificado contando los registros antes y después de la baja.
- **SC-012**: El 100 % de las reglas de negocio (RN-01 a RN-16) tiene al menos una prueba
  automatizada asociada que falla si la regla se rompe.

---

## Fuera de Alcance

- Invitaciones por correo electrónico a personas que todavía no tienen cuenta.
- Transferencia de la propiedad del proyecto a otro integrante.
- Roles adicionales dentro del proyecto (Scrum Master, Product Owner, permisos por rol).
- Eliminación y archivado de proyectos.
- Métricas y gráficos del proyecto (velocidad, burndown, desvíos), que se especifican en features
  propias.
- Gestión del backlog, de los sprints, del esfuerzo, de las estimaciones y de los defectos, que se
  especifican en features propias. Esta feature solo consulta sus resultados para la vista de estado.
- Historial de auditoría consultable de los cambios del proyecto (quién cambió qué y cuándo).
- Búsqueda, filtrado y paginación del listado de proyectos.
- Notificaciones al integrante cuando se lo agrega o se lo quita.
- Adjuntos, enlaces externos y documentación cargada dentro del proyecto.

---

## Supuestos

- **Descripción opcional con tope de 1000 caracteres**: la descripción no se declaró obligatoria ni
  se le fijó longitud; se adopta opcional y con un máximo razonable para un texto de contexto.
- **Nombre de 1 a 100 caracteres**: el máximo lo fija el enunciado; no se define un mínimo mayor que
  1 porque no se pidió, pero un nombre en blanco se rechaza.
- **Unicidad de nombre insensible a capitalización y espacios extremos**: se aplica el mismo criterio
  de normalización que la feature de autenticación usa para los correos, para que dos proyectos que
  una persona lee como iguales no coexistan.
- **Fechas sin hora**: la fecha de inicio, la de finalización prevista y la de finalización real son
  fechas de calendario, sin hora. La fecha de finalización real se deriva del momento del cambio
  usando la zona horaria única y configurable del sistema (confirmado el 2026-09-25, FR-039).
- **Fecha de inicio libre**: puede ser pasada, presente o futura; el sistema no la compara con la
  fecha actual.
- **Fecha de finalización real = fecha del cambio de estado**: se toma la fecha en que el propietario
  finaliza el proyecto; no se pide ni se admite ingresarla a mano.
- **Transiciones solo consecutivas**: decisión confirmada el 2026-09-25 y formalizada en FR-037 y
  FR-038. "Los estados solo avanzan" significa la cadena Planificado → En curso → Finalizado paso a
  paso; el salto directo de Planificado a Finalizado se rechaza, porque un proyecto que nunca estuvo
  en curso no tiene esfuerzo que medir. Cerrar un proyecto que nunca arrancó es una cancelación, un
  concepto distinto que hoy está fuera de alcance.
- **Finalizar exige no tener sprint activo**: decisión confirmada el 2026-09-25 y formalizada en
  FR-041 y RN-16. Como un proyecto Finalizado es de solo lectura, finalizarlo con un sprint abierto
  dejaría un sprint que ya nadie puede cerrar. Esta feature solo consulta si existe un sprint activo;
  cómo se cierra un sprint corresponde a la feature de sprints.
- **Sin límite de integrantes ni de proyectos**: no se define un máximo de integrantes por proyecto
  ni de proyectos por usuario, porque no se pidió.
- **Factor almacenado tal como se ingresa**: el factor se guarda con su precisión de hasta 2
  decimales y se usa sin redondeos intermedios en el cálculo de horas estimadas.
- **Estados del backlog definidos en su propia feature**: la consulta de estado informa la cantidad
  de historias por cada estado que la feature de backlog defina; esta spec no fija ese conjunto de
  estados.
- **Idioma de la interfaz**: los mensajes que ve la persona están en español.
- **Dependencia hacia adelante**: las features siguientes (backlog, sprints, estimaciones, esfuerzo,
  defectos y métricas) asumen que todo su contenido pertenece a un proyecto y que quien opera es un
  integrante vigente de ese proyecto.

---

## Preguntas Abiertas

Ninguna. Las decisiones no especificadas en la descripción de entrada se resolvieron con supuestos
explícitos (ver la sección Supuestos); las que más impactan en el alcance son la interpretación de
las transiciones como consecutivas y el rechazo de la finalización mientras haya un sprint activo,
ambas confirmadas en la sección Clarifications.
