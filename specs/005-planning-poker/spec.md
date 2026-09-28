# Especificación de Feature: Planning Poker en Tiempo Real

**Directorio de feature**: `specs/005-planning-poker`

**Rama**: `005-planning-poker`

**Creada**: 2026-09-26

**Estado**: Clarificada — sin preguntas abiertas (sesión de clarificación del 2026-09-26)

**Entrada**: Descripción del usuario: "Planning Poker en tiempo real para Software Metrics &
Estimation, un sistema web multiusuario para estimar, planificar, seguir y medir proyectos de software
con Scrum. Depende de: gestión de proyectos e integrantes; Product Backlog (historias con Story Points
en la escala 0, 1, 2, 3, 5, 8, 13, 20, 40, 100 o "sin estimar")."

---

## Objetivo

Permitir que los integrantes de un proyecto estimen colaborativamente una historia del backlog en
Story Points, cada uno desde su propia sesión y en tiempo real, evitando el sesgo de anclaje: los
votos permanecen ocultos hasta que se revelan, las diferencias se hacen visibles y se repiten rondas
hasta acordar una estimación que queda registrada en la historia.

La estimación individual es fácil de conseguir y difícil de confiar: quien habla primero fija el
número y el resto ajusta su opinión a ese ancla, de modo que el equipo termina con una cifra que nadie
revisó de verdad. El Planning Poker existe para romper ese efecto. Cada persona se compromete con un
número antes de conocer el de los demás, y las diferencias que aparecen al revelar no son un problema
a promediar sino la señal más valiosa de la reunión: cuando alguien vota 3 y otro vota 20, están
entendiendo cosas distintas por la misma historia, y esa conversación vale más que la estimación. Como
todas las métricas del producto —velocidad, burndown, desvío entre lo estimado y lo real— se calculan
sobre los Story Points de las historias, la calidad de ese número es el techo de la calidad de todo lo
que después se mide. El historial de rondas y votos que queda guardado es, además, el registro de cómo
el equipo llegó a cada cifra y de cuánto fue mejorando su capacidad de estimar.

---

## Clarifications

### Session 2026-09-26

- Q: ¿Qué pasa con una sesión Abierta que queda abandonada sin cerrarse ni cancelarse, si una historia
  admite una sola sesión abierta a la vez? (FR-063) → A: caduca automáticamente por inactividad; la
  sesión se cancela sola después de 60 minutos sin ningún hecho registrado (incorporaciones,
  desconexiones, votos, revelaciones, rondas nuevas y cambios de facilitador), y cada hecho reinicia
  la cuenta.
- Q: ¿Qué pasa si alguien reestima la historia directamente desde el backlog mientras hay una sesión
  Abierta sobre ella? (FR-075) → A: el cambio directo se acepta, la sesión sigue su curso y al cerrarse
  el valor acordado reemplaza el valor vigente de la historia: gana la última escritura. No hace falta
  enmendar `specs/003-product-backlog`.
- Q: ¿Un participante puede retirar su voto y volver a quedar "sin voto" antes de la revelación?
  (FR-022) → A: no; un voto emitido solo se cambia por otra carta, incluida "?" para decir que no sabe.
  Retirarlo le daría a cualquiera la posibilidad de frenar indefinidamente la revelación automática.
- Q: Si dos participantes piden tomar el rol de facilitador en el mismo instante, ¿cómo se resuelve?
  (FR-072) → A: prospera solo el primer pedido que el sistema procesa; el segundo se rechaza indicando
  quién es el facilitador vigente, y todos los participantes conectados ven un único cambio de
  facilitador. Es el mismo criterio de resolución hacia un único resultado que FR-029 aplica al voto
  simultáneo con la revelación.
- Q: Si una misma persona abre la sesión en dos pestañas o dispositivos a la vez, ¿cuenta como un
  participante o como dos, y qué pasa cuando cierra solo una? (FR-013, FR-014) → A: cuenta como un único
  participante con varias conexiones: figura como conectado mientras le quede al menos una conexión viva,
  su voto es el mismo en todas sus pantallas y solo queda desconectado cuando cae la última.
- Q: ¿Cuántos participantes simultáneos por sesión y cuántas sesiones en paralelo hay que sostener con
  el límite de 2 segundos? (FR-073, RC-14) → A: hasta 10 participantes conectados por sesión y hasta 10
  sesiones en paralelo en todo el sistema, es decir 100 conexiones concurrentes. Es una capacidad
  objetivo, no un tope: el sistema no rechaza al participante 11, pero el límite de 2 segundos se
  compromete hasta esa escala.
- Q: ¿Se puede eliminar del backlog una historia que ya tuvo sesiones de Planning Poker, y qué pasa con
  su historial de votos? (FR-089) → A: no se puede; el historial de estimación se suma como cuarta
  condición que bloquea la eliminación, junto con haber estado en un sprint, tener esfuerzo registrado y
  tener defectos asociados. La decisión enmienda `specs/003-product-backlog` (FR-041, FR-043, RN-16 y
  RC-05) y vale tanto para las sesiones Finalizadas como para las Canceladas.

---

## Entradas y Salidas Esperadas

### Apertura de una sesión de estimación

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador de una historia del backlog | **Éxito**: sesión de estimación Abierta con una primera ronda En votación, con quien la abrió como facilitador y primer participante conectado |
| | **La historia ya tiene una sesión Abierta**: rechazo indicando que ya hay una sesión en curso, con el identificador de esa sesión para poder unirse; no se crea una segunda |
| | **Historia Completada**: rechazo porque una historia completada no se estima |
| | **Proyecto Finalizado**: rechazo porque el proyecto es de solo lectura |
| | **Quien pide no es integrante**: respuesta de historia inexistente |

### Unirse a una sesión

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador de la sesión de estimación | **Éxito**: el integrante queda conectado como participante y recibe el título, la descripción y los criterios de aceptación de la historia, la escala de cartas, la lista de participantes conectados con quién ya votó, el número de ronda y el estado de la ronda |
| Integrante que ya había participado en esta sesión y se reconecta | Los mismos datos, y además su propio voto de la ronda vigente si lo había emitido |
| Sesión Finalizada o Cancelada | Rechazo indicando que la sesión ya está cerrada, con el resultado registrado; no se admiten nuevos participantes |
| Quien pide no es integrante del proyecto | Respuesta de sesión inexistente, idéntica a la de un identificador que no existe |

### Emisión y cambio de voto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un participante conectado e identificador de una carta de la escala o de la carta "?" | **Éxito**: el voto queda registrado y oculto; el resto ve que esa persona ya votó, sin ver el valor; quien vota ve su propia carta elegida |
| Participante que vuelve a votar antes de la revelación | **Éxito**: el nuevo voto reemplaza al anterior y sigue oculto |
| Valor fuera de la escala y distinto de "?" | Rechazo indicando las cartas admitidas; el voto anterior, si existía, no cambia |
| Ronda ya revelada | Rechazo indicando que la ronda está revelada y que hay que esperar una nueva ronda |
| Sesión Finalizada o Cancelada | Rechazo por sesión cerrada |
| Quien vota no es participante conectado de la sesión | Rechazo por no participar de la sesión |

### Revelación de los votos

| Entradas | Salidas esperadas |
| --- | --- |
| Todos los participantes conectados de la ronda emitieron su voto (revelación automática) | Ronda pasa a Revelada y todos ven, en menos de 2 segundos, el voto de cada participante, si hay consenso o divergencia y —cuando hay al menos un voto numérico— el mínimo y el máximo con sus autores |
| Sesión del facilitador con al menos un voto emitido en la ronda (revelación manual) | El mismo resultado; los participantes conectados que no votaron figuran como "sin voto" |
| Sesión del facilitador sin ningún voto emitido en la ronda | Rechazo indicando que hace falta al menos un voto para revelar |
| Acción de revelación pedida por un participante que no es el facilitador | Rechazo por acción reservada al facilitador; la ronda sigue En votación |
| Ronda ya revelada | Rechazo indicando que la ronda ya fue revelada |

### Inicio de una nueva ronda

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión del facilitador y una ronda Revelada | **Éxito**: la ronda revelada se cierra y queda en el historial de la sesión con sus votos; se abre una ronda nueva En votación, sin votos, con el número siguiente; todos los participantes conectados ven la nueva ronda en menos de 2 segundos |
| Ronda todavía En votación | Rechazo indicando que primero hay que revelar la ronda vigente |
| Pedido de un participante que no es el facilitador | Rechazo por acción reservada al facilitador |
| Sesión Finalizada o Cancelada | Rechazo por sesión cerrada |

### Registro de la estimación acordada

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión del facilitador, identificador de la sesión y un valor numérico de la escala de Story Points | **Éxito**: la sesión pasa a Finalizada con ese valor acordado, los Story Points de la historia quedan en ese valor y todos los participantes conectados ven el cierre y el valor registrado en menos de 2 segundos |
| Valor fuera de la escala numérica, o la carta "?", o la marca "sin estimar" | Rechazo indicando los valores admitidos; la sesión sigue Abierta y la historia no cambia |
| Sesión sin ninguna ronda revelada | Rechazo indicando que primero hay que revelar al menos una ronda o cancelar la sesión |
| Pedido de un participante que no es el facilitador | Rechazo por acción reservada al facilitador |
| Sesión Finalizada o Cancelada | Rechazo por sesión cerrada; el valor ya registrado no se modifica |

### Cancelación de la sesión

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión del facilitador e identificador de la sesión | **Éxito**: la sesión pasa a Cancelada, los Story Points de la historia quedan como estaban y todos los participantes conectados ven la cancelación en menos de 2 segundos; las rondas y los votos se conservan |
| Pedido de un participante que no es el facilitador | Rechazo por acción reservada al facilitador |
| Sesión Finalizada o Cancelada | Rechazo por sesión cerrada |
| Sesión Abierta sin ningún hecho durante 60 minutos (cancelación automática por inactividad) | La sesión pasa a Cancelada con ese motivo registrado, los Story Points de la historia quedan como estaban, se avisa a los participantes conectados y las rondas y los votos se conservan |

### Toma del rol de facilitador

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un participante conectado, con el facilitador desconectado desde hace 5 minutos o más | **Éxito**: quien lo pide pasa a ser el facilitador; todos los participantes conectados ven el cambio en menos de 2 segundos |
| Facilitador conectado, o desconectado desde hace menos de 5 minutos | Rechazo indicando cuánto falta para poder tomar el rol; el facilitador no cambia |
| Pedido de quien no es participante conectado de la sesión | Rechazo por no participar de la sesión |

### Consulta del estado de la sesión y del historial de estimación

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador de una historia | Si hay una sesión Abierta, su identificador, su facilitador, su número de ronda y el estado de la ronda; y en todos los casos la lista de sesiones anteriores de esa historia con su resultado |
| Sesión de un integrante e identificador de una sesión cerrada | Momento de apertura y de cierre, quién la abrió, quién la cerró, estado final (Finalizada o Cancelada), valor acordado si lo hubo, y cada ronda con su número, sus votos por participante y su resultado de consenso o divergencia |
| Historia que nunca se estimó por Planning Poker | Historial vacío con la indicación correspondiente; no es un error |
| Sesión de quien no es integrante del proyecto | Respuesta de historia o sesión inexistente, sin revelar ningún dato |

---

## Escenarios de Usuario y Pruebas *(obligatorio)*

### Historia de Usuario 1 (US1) — Abrir una sesión y reunir al equipo (Prioridad: P1)

Un integrante elige una historia del backlog que hay que estimar y abre una sesión de Planning Poker.
Queda como facilitador y comparte la sesión; los demás integrantes se unen desde sus propias
computadoras y todos ven la misma historia y la misma lista de participantes conectados, que se
actualiza a medida que la gente entra y sale.

**Por qué esta prioridad**: sin sesión y sin participantes no hay nada que votar. Es la base sobre la
que se apoyan todas las demás historias de esta feature.

**Prueba independiente**: se puede probar completa abriendo una sesión para una historia del backlog
desde la cuenta de un integrante, uniéndose desde las cuentas de otros dos y verificando que los tres
ven la misma historia y la misma lista de participantes conectados, y que la lista se actualiza cuando
uno de ellos cierra su sesión.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una historia Pendiente de un proyecto En curso, **cuando** un integrante
   abre una sesión de Planning Poker sobre ella, **entonces** la sesión queda Abierta con una primera
   ronda En votación, con quien la abrió como facilitador y como primer participante conectado.
2. *(Caso normal)* **Dado** una sesión Abierta con un participante, **cuando** otros dos integrantes
   del proyecto se unen, **entonces** los tres ven el título, la descripción y los criterios de
   aceptación de la historia, y la lista de los tres participantes conectados, en menos de 2 segundos
   desde cada incorporación y sin recargar la pantalla.
3. *(Caso alternativo)* **Dado** una sesión Abierta con tres participantes conectados, **cuando** uno
   de ellos abandona la sesión, **entonces** los otros dos ven en menos de 2 segundos que ya no está
   conectado.
4. *(Caso alternativo)* **Dado** una historia sin criterios de aceptación, **cuando** un integrante
   abre una sesión sobre ella, **entonces** la sesión se abre igual y la historia se muestra con la
   lista de criterios vacía.
5. *(Caso límite)* **Dado** una sesión Abierta cuyo único participante conectado es el facilitador,
   **cuando** nadie más se une, **entonces** la sesión es válida y el facilitador puede votar y revelar
   por sí solo.
6. *(Caso límite)* **Dado** una historia con dos sesiones anteriores ya cerradas, **cuando** un
   integrante abre una nueva sesión sobre ella, **entonces** la apertura se acepta y las sesiones
   anteriores se conservan en el historial.
7. *(Caso de error)* **Dado** una historia que ya tiene una sesión Abierta, **cuando** otro integrante
   intenta abrir una segunda sesión sobre la misma historia, **entonces** la acción se rechaza
   indicando que ya hay una sesión en curso y ofreciendo unirse a ella.
8. *(Caso de error)* **Dado** una historia en estado Completada, **cuando** un integrante intenta abrir
   una sesión sobre ella, **entonces** la acción se rechaza porque una historia completada no se
   estima.
9. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta abrir
   una sesión sobre una de sus historias, **entonces** la acción se rechaza porque el proyecto es de
   solo lectura.
10. *(Caso de error)* **Dado** una sesión Abierta de un proyecto, **cuando** un usuario que no es
    integrante de ese proyecto intenta unirse, **entonces** recibe una respuesta de sesión inexistente,
    idéntica a la de un identificador que no existe.

---

### Historia de Usuario 2 (US2) — Votar en secreto (Prioridad: P1)

Cada participante elige la carta que representa su estimación —un valor de la escala o "?" si no sabe—
y la juega. Todos ven quién ya votó, pero nadie ve el valor ajeno. Quien se arrepiente puede cambiar
su carta tantas veces como quiera mientras la ronda siga en votación.

**Por qué esta prioridad**: el secreto del voto es la razón de existir de la feature. Es lo que evita
el sesgo de anclaje; sin él, esto sería una encuesta abierta.

**Prueba independiente**: se puede probar completa con una sesión de tres participantes, votando desde
cada cuenta y verificando que cada persona ve solo su propia carta, que las demás aparecen como "ya
votó" sin valor, y que un cambio de carta reemplaza el voto anterior.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una ronda En votación con tres participantes conectados, **cuando** uno
   juega la carta 5, **entonces** su voto queda registrado y los otros dos ven en menos de 2 segundos
   que esa persona ya votó, sin ver el valor.
2. *(Caso normal)* **Dado** un participante que ya jugó la carta 5, **cuando** consulta su pantalla,
   **entonces** ve su propia carta elegida y puede distinguirla del resto de las cartas.
3. *(Caso alternativo)* **Dado** un participante que jugó la carta 8, **cuando** juega la carta 13
   antes de la revelación, **entonces** su voto pasa a ser 13, el 8 se descarta y para los demás sigue
   figurando solamente como "ya votó".
4. *(Caso alternativo)* **Dado** un participante que no tiene idea de cuánto cuesta la historia,
   **cuando** juega la carta "?", **entonces** su voto se registra como emitido igual que cualquier
   otro.
5. *(Caso límite)* **Dado** una ronda En votación, **cuando** un participante juega la carta 0 y otro
   la carta 100, **entonces** ambos votos se aceptan porque son los extremos válidos de la escala.
6. *(Caso límite)* **Dado** una ronda en la que dos participantes de tres ya votaron, **cuando** el
   tercero cambia su carta en el mismo instante en que el facilitador revela, **entonces** el
   resultado revelado es consistente para todos: o se revela el voto nuevo, o se revela el anterior,
   pero todos los participantes ven exactamente el mismo conjunto de votos.
7. *(Caso de error)* **Dado** una ronda En votación, **cuando** un participante intenta votar el valor
   4, el 50 o un decimal, **entonces** el voto se rechaza indicando las cartas admitidas y su voto
   anterior, si lo tenía, no cambia.
8. *(Caso de error)* **Dado** una ronda ya Revelada, **cuando** un participante intenta votar o
   cambiar su voto, **entonces** la acción se rechaza indicando que la ronda está revelada y que debe
   esperar una ronda nueva; los votos revelados no cambian.
9. *(Caso de error)* **Dado** una sesión Abierta, **cuando** un integrante del proyecto que no se unió
   a la sesión intenta votar, **entonces** la acción se rechaza porque no participa de la sesión.
10. *(Caso de error)* **Dado** un participante que ya jugó la carta 5, **cuando** intenta retirar su
    voto para volver a quedar sin votar, **entonces** la acción se rechaza indicando que solo puede
    cambiarla por otra carta, incluida "?", y su voto sigue siendo 5.

---

### Historia de Usuario 3 (US3) — Revelar los votos y ver las diferencias (Prioridad: P1)

Cuando todos los conectados votaron, las cartas se dan vuelta solas; si alguien no vota, el facilitador
puede darlas vuelta a mano. Todos ven al mismo tiempo qué votó cada uno y, si los números no
coinciden, el sistema señala el voto más bajo y el más alto con sus autores para que expliquen por qué.

**Por qué esta prioridad**: la revelación simultánea y la exposición de las diferencias son el
resultado que el equipo viene a buscar. Sin esto, votar en secreto no sirve para nada.

**Prueba independiente**: se puede probar completa con una sesión de tres participantes que votan
valores distintos, verificando que la revelación ocurre sola al completarse el último voto, que los
tres ven los mismos valores y que el sistema marca divergencia con el mínimo y el máximo y sus autores.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una ronda con tres participantes conectados que votaron 3, 5 y 13,
   **cuando** se emite el último voto, **entonces** la ronda se revela automáticamente y los tres ven,
   en menos de 2 segundos, el voto de cada participante, la marca de divergencia, el mínimo 3 y el
   máximo 13 con sus autores.
2. *(Caso normal)* **Dado** una ronda con cuatro participantes conectados en la que tres votaron 8 y
   uno no votó, **cuando** el facilitador revela manualmente, **entonces** todos ven los tres votos
   revelados, el cuarto participante figura como "sin voto" y la ronda se marca con consenso en 8.
3. *(Caso alternativo)* **Dado** una ronda en la que dos participantes votaron 5 y un tercero votó
   "?", **cuando** se revela, **entonces** el "?" se muestra como tal y la ronda se marca con consenso
   en 5, porque el "?" no participa del cálculo.
4. *(Caso alternativo)* **Dado** una ronda en la que tres participantes votaron 20, 20 y 20, **cuando**
   se revela, **entonces** la ronda se marca con consenso en 20 y no se señala mínimo ni máximo, porque
   no hay diferencias que explicar.
5. *(Caso límite)* **Dado** una ronda de un único participante conectado, **cuando** ese participante
   vota, **entonces** la ronda se revela automáticamente y se marca con consenso en su valor.
6. *(Caso límite)* **Dado** una ronda en la que los tres participantes conectados votaron "?",
   **cuando** se revela, **entonces** no hay consenso, no se señalan mínimo ni máximo y el resultado
   indica que la ronda no tuvo ningún voto numérico.
7. *(Caso límite)* **Dado** una ronda con cuatro participantes conectados de los cuales tres ya
   votaron, **cuando** el cuarto se desconecta sin votar, **entonces** la revelación automática se
   dispara porque todos los que siguen conectados ya votaron.
8. *(Caso límite)* **Dado** una ronda en la que dos participantes votaron 3 y otros dos votaron 40,
   **cuando** se revela, **entonces** se señalan los dos autores del mínimo y los dos autores del
   máximo.
9. *(Caso de error)* **Dado** una ronda En votación sin ningún voto emitido, **cuando** el facilitador
   intenta revelar, **entonces** la acción se rechaza indicando que hace falta al menos un voto.
10. *(Caso de error)* **Dado** una ronda En votación con dos votos emitidos, **cuando** un participante
    que no es el facilitador intenta revelar, **entonces** la acción se rechaza por estar reservada al
    facilitador y la ronda sigue En votación con los votos ocultos.

---

### Historia de Usuario 4 (US4) — Repetir la ronda hasta acercar posiciones (Prioridad: P2)

Después de discutir por qué alguien votó 3 y alguien votó 20, el facilitador abre una ronda nueva. Las
cartas de la ronda anterior se levantan de la mesa, todos vuelven a votar desde cero y la ronda vieja
queda guardada en el historial de la sesión.

**Por qué esta prioridad**: sin rondas nuevas la sesión solo sirve cuando el equipo acierta a la
primera. Es la mecánica que convierte una votación aislada en una conversación que converge, pero el
producto ya entrega valor con una sola ronda.

**Prueba independiente**: se puede probar completa revelando una ronda con votos divergentes, abriendo
una ronda nueva y verificando que nadie tiene voto en la ronda nueva, que el número de ronda avanzó y
que la ronda anterior sigue consultable con sus votos.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una ronda 1 Revelada con votos 3, 5 y 13, **cuando** el facilitador inicia
   una ronda nueva, **entonces** se abre la ronda 2 En votación, ningún participante tiene voto, los
   votos de la ronda 1 desaparecen de la vista y los tres participantes ven el cambio en menos de 2
   segundos.
2. *(Caso normal)* **Dado** una ronda 2 en curso, **cuando** cualquier participante consulta el
   historial de la sesión, **entonces** ve la ronda 1 con el voto de cada participante y su resultado
   de divergencia.
3. *(Caso alternativo)* **Dado** una ronda 1 Revelada con consenso, **cuando** el facilitador inicia
   una ronda nueva de todos modos, **entonces** la ronda 2 se abre igual, porque el consenso no obliga
   a cerrar la sesión.
4. *(Caso alternativo)* **Dado** una ronda 2 en curso, **cuando** un integrante que no había
   participado se une a la sesión, **entonces** queda como participante conectado de la ronda 2 y puede
   votar en ella, aunque no tenga voto en la ronda 1.
5. *(Caso límite)* **Dado** una sesión que ya lleva diez rondas sin consenso, **cuando** el facilitador
   inicia la ronda once, **entonces** la acción se acepta porque no hay límite de rondas, y las diez
   rondas anteriores siguen completas en el historial.
6. *(Caso límite)* **Dado** una ronda Revelada en la que todos los participantes se desconectaron
   menos el facilitador, **cuando** el facilitador inicia una ronda nueva, **entonces** la ronda se
   abre con un único participante conectado.
7. *(Caso de error)* **Dado** una ronda En votación todavía sin revelar, **cuando** el facilitador
   intenta iniciar una ronda nueva, **entonces** la acción se rechaza indicando que primero hay que
   revelar la ronda vigente, para que ningún voto emitido se descarte sin haberse mostrado.
8. *(Caso de error)* **Dado** una ronda Revelada, **cuando** un participante que no es el facilitador
   intenta iniciar una ronda nueva, **entonces** la acción se rechaza por estar reservada al
   facilitador.

---

### Historia de Usuario 5 (US5) — Registrar la estimación acordada (Prioridad: P2)

Cuando el equipo llega a un número, el facilitador lo registra: elige el valor final de la escala —que
puede no coincidir con ningún voto emitido— y con eso la sesión se cierra y los Story Points de la
historia quedan actualizados para todo el equipo.

**Por qué esta prioridad**: es el punto donde la conversación se convierte en un dato que el resto del
sistema usa. Sin registrar, la sesión fue una charla; con registrar, el backlog queda estimado.

**Prueba independiente**: se puede probar completa revelando una ronda, registrando un valor final
desde la cuenta del facilitador y verificando que la sesión queda Finalizada con ese valor y que la
historia del backlog muestra esos Story Points.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una ronda Revelada con consenso en 8, **cuando** el facilitador registra 8
   como estimación acordada, **entonces** la sesión pasa a Finalizada con valor 8, la historia queda
   estimada en 8 Story Points y todos los participantes conectados ven el cierre en menos de 2
   segundos.
2. *(Caso normal)* **Dado** una ronda Revelada con votos 5, 8 y 13, **cuando** el facilitador registra
   8 después de la discusión, **entonces** la sesión se cierra con 8 aunque no haya habido consenso.
3. *(Caso alternativo)* **Dado** una ronda Revelada con votos 5 y 8, **cuando** el facilitador registra
   13, un valor que nadie votó, **entonces** la acción se acepta porque el valor acordado no tiene que
   coincidir con ningún voto.
4. *(Caso alternativo)* **Dado** una historia que ya estaba estimada en 3 Story Points, **cuando** la
   sesión se cierra con 20, **entonces** la historia queda estimada en 20 y el valor anterior se
   reemplaza.
5. *(Caso alternativo)* **Dado** una sesión Abierta sobre una historia sin estimar, **cuando** un
   integrante la reestima directamente en 3 desde el backlog mientras la ronda está en votación,
   **entonces** la sesión sigue su curso sin interrupción y, al cerrarse con 8, la historia queda
   estimada en 8.
6. *(Caso límite)* **Dado** una ronda Revelada en la que todos votaron "?", **cuando** el facilitador
   registra 40 de todos modos, **entonces** la acción se acepta, porque el valor acordado lo decide el
   facilitador y no se calcula a partir de los votos.
7. *(Caso límite)* **Dado** una historia sin criterios de aceptación estimada por una sesión que se
   cierra con 5, **entonces** la historia queda estimada en 5 pero sigue sin estar lista para
   planificar, porque le falta al menos un criterio.
8. *(Caso límite)* **Dado** una sesión cuyo facilitador registra la estimación acordada, **cuando** la
   sesión se cierra, **entonces** las rondas y los votos de la sesión siguen consultables, porque
   forman parte de los reportes de estimación.
9. *(Caso de error)* **Dado** una ronda Revelada, **cuando** el facilitador intenta registrar 7, un
   decimal o la marca "sin estimar" como valor final, **entonces** la acción se rechaza indicando los
   valores admitidos, la sesión sigue Abierta y los Story Points de la historia no cambian.
10. *(Caso de error)* **Dado** una sesión Abierta cuya primera ronda todavía no se reveló, **cuando** el
    facilitador intenta registrar la estimación acordada, **entonces** la acción se rechaza indicando
    que primero hay que revelar al menos una ronda o cancelar la sesión.
11. *(Caso de error)* **Dado** una ronda Revelada, **cuando** un participante que no es el facilitador
    intenta registrar la estimación acordada, **entonces** la acción se rechaza por estar reservada al
    facilitador y los Story Points de la historia no cambian.

---

### Historia de Usuario 6 (US6) — Sobrevivir a las desconexiones y relevar al facilitador (Prioridad: P2)

A alguien se le corta internet en medio de la ronda. La sesión no se cae: quien se va deja de contarse
para la revelación automática y, si vuelve en la misma ronda, recupera su lugar y su voto. Si el que
se cayó es el facilitador, la sesión sigue abierta esperándolo, y si no vuelve en cinco minutos
cualquier otro participante puede tomar el rol y seguir adelante.

**Por qué esta prioridad**: una reunión distribuida sin tolerancia a cortes se interrumpe todo el
tiempo, y una sesión que solo el facilitador puede cerrar se convierte en una historia bloqueada si esa
persona desaparece.

**Prueba independiente**: se puede probar completa con una sesión de tres participantes, cortando la
conexión de uno en medio de la ronda y verificando que la revelación automática se evalúa sobre los
que quedan, que al reconectar recupera su voto, y cortando la del facilitador para verificar que la
sesión sigue abierta y que a los cinco minutos otro participante puede tomar el rol.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una ronda con tres participantes conectados de los cuales dos votaron,
   **cuando** el tercero pierde la conexión, **entonces** los otros dos lo ven como desconectado en
   menos de 2 segundos y la ronda se revela automáticamente porque todos los conectados ya votaron.
2. *(Caso normal)* **Dado** un participante que votó 13 y se desconectó, **cuando** se reconecta en la
   misma ronda todavía En votación, **entonces** vuelve a figurar como conectado, su voto 13 sigue
   contado y él ve su propia carta.
3. *(Caso alternativo)* **Dado** un participante que se desconectó sin votar, **cuando** se reconecta
   en la misma ronda todavía En votación, **entonces** vuelve a contarse para la revelación automática
   y puede votar.
4. *(Caso alternativo)* **Dado** un participante que se desconectó durante la ronda 1, **cuando** se
   reconecta cuando la sesión ya está en la ronda 3, **entonces** se incorpora a la ronda 3 sin voto y
   sus votos de las rondas anteriores siguen en el historial.
5. *(Caso alternativo)* **Dado** el facilitador desconectado hace dos minutos, **cuando** se reconecta,
   **entonces** retoma su rol y puede revelar, iniciar rondas, registrar la estimación y cancelar.
6. *(Caso límite)* **Dado** una ronda con dos participantes conectados que no votaron, **cuando** los
   dos se desconectan, **entonces** no se revela nada y la sesión queda Abierta en la misma ronda,
   esperando que alguien vuelva.
7. *(Caso límite)* **Dado** el facilitador desconectado hace más de 5 minutos, **cuando** otro
   participante conectado toma el rol y el facilitador original se reconecta después, **entonces** el
   facilitador es quien tomó el rol y el original queda como participante común.
8. *(Caso límite)* **Dado** el facilitador desconectado hace más de 5 minutos y un único participante
   conectado, **cuando** ese participante toma el rol, **entonces** pasa a ser facilitador y puede
   cerrar o cancelar la sesión por sí solo.
9. *(Caso límite)* **Dado** un participante que abrió la sesión en dos pestañas y votó 8, **cuando**
   cierra una de las dos, **entonces** sigue figurando como conectado para el resto del equipo, su voto
   8 sigue contado y en la pestaña que queda abierta ve su propia carta.
10. *(Caso de error)* **Dado** el facilitador conectado, **cuando** otro participante intenta tomar el
    rol de facilitador, **entonces** la acción se rechaza porque el facilitador está presente.
11. *(Caso de error)* **Dado** el facilitador desconectado hace dos minutos, **cuando** otro
    participante intenta tomar el rol, **entonces** la acción se rechaza indicando cuánto falta para
    que el rol quede disponible.
12. *(Caso de error)* **Dado** el facilitador desconectado hace más de 5 minutos, **cuando** dos
    participantes piden tomar el rol en el mismo instante, **entonces** uno solo lo obtiene, al otro se
    le rechaza el pedido indicando quién es el facilitador vigente y todos ven un único cambio de
    facilitador.

---

### Historia de Usuario 7 (US7) — Cancelar una sesión sin registrar estimación (Prioridad: P3)

El equipo se da cuenta de que la historia está mal definida y que no tiene sentido seguir estimándola.
El facilitador cancela la sesión: no se registra ningún valor, los Story Points de la historia quedan
como estaban y lo que se votó queda guardado igual.

**Por qué esta prioridad**: es la salida limpia de una sesión que no debe terminar en un número.
Importa porque una historia solo admite una sesión abierta a la vez, pero el valor central de la
feature se entrega sin ella.

**Prueba independiente**: se puede probar completa abriendo una sesión, cancelándola desde la cuenta
del facilitador y verificando que la historia conserva su estimación anterior, que la sesión figura
como Cancelada y que se puede abrir una sesión nueva sobre la misma historia.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una sesión Abierta sobre una historia estimada en 5 Story Points,
   **cuando** el facilitador la cancela, **entonces** la sesión pasa a Cancelada, la historia sigue
   estimada en 5 y todos los participantes conectados ven la cancelación en menos de 2 segundos.
2. *(Caso alternativo)* **Dado** una sesión Cancelada, **cuando** un integrante abre una sesión nueva
   sobre la misma historia, **entonces** la apertura se acepta porque la historia ya no tiene ninguna
   sesión Abierta.
3. *(Caso límite)* **Dado** una sesión Abierta con tres rondas reveladas, **cuando** el facilitador la
   cancela, **entonces** las tres rondas y todos sus votos siguen consultables en el historial de
   estimación de la historia.
4. *(Caso límite)* **Dado** una sesión Abierta en la que nadie votó nunca, **cuando** el facilitador la
   cancela, **entonces** la cancelación se acepta y la sesión queda registrada sin ninguna ronda
   revelada.
5. *(Caso límite)* **Dado** una sesión Abierta que el equipo abandonó sin cerrarla, **cuando** pasan 60
   minutos sin ningún hecho en la sesión, **entonces** la sesión queda Cancelada por inactividad con
   ese motivo registrado, la historia conserva su estimación anterior y se puede abrir una sesión nueva
   sobre ella.
6. *(Caso límite)* **Dado** una sesión Abierta cuyo último hecho ocurrió hace 59 minutos, **cuando** un
   participante emite un voto, **entonces** la cuenta de inactividad se reinicia y la sesión no se
   cancela.
7. *(Caso de error)* **Dado** una sesión Abierta, **cuando** un participante que no es el facilitador
   intenta cancelarla, **entonces** la acción se rechaza por estar reservada al facilitador y la sesión
   sigue Abierta.
8. *(Caso de error)* **Dado** una sesión ya Finalizada con valor acordado, **cuando** el facilitador
   intenta cancelarla, **entonces** la acción se rechaza por sesión cerrada y el valor registrado no se
   modifica.
9. *(Caso de error)* **Dado** una sesión Cancelada por inactividad, **cuando** un participante intenta
   votar o el facilitador intenta registrar una estimación, **entonces** la acción se rechaza indicando
   que la sesión se cerró por inactividad.

---

### Historia de Usuario 8 (US8) — Consultar el historial de estimación de una historia (Prioridad: P3)

Cualquier integrante abre una historia y ve cómo se estimó: cuántas sesiones hubo, cuántas rondas tuvo
cada una, qué votó cada persona en cada ronda y con qué valor terminó. Sirve para entender de dónde
salió un número y para mirar hacia atrás cuánto se desviaron las estimaciones del equipo.

**Por qué esta prioridad**: es el aprovechamiento posterior del dato. No hace falta para estimar, pero
es la razón por la que las rondas se conservan después de cerrar la sesión.

**Prueba independiente**: se puede probar completa cerrando dos sesiones sobre la misma historia con
valores distintos y verificando que el historial muestra ambas, con sus rondas, sus votos por
participante y sus valores acordados.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una historia con una sesión Finalizada de tres rondas, **cuando** un
   integrante consulta su historial de estimación, **entonces** ve la sesión con su momento de apertura
   y de cierre, quién la abrió, quién la cerró, el valor acordado y las tres rondas con el voto de cada
   participante y el resultado de consenso o divergencia de cada una.
2. *(Caso normal)* **Dado** una historia estimada dos veces por Planning Poker, **cuando** un
   integrante consulta el historial, **entonces** ve las dos sesiones ordenadas de la más reciente a la
   más antigua, cada una con su valor acordado.
3. *(Caso alternativo)* **Dado** una historia con una sesión Cancelada, **cuando** un integrante
   consulta el historial, **entonces** la sesión figura como Cancelada, sin valor acordado, y con sus
   rondas completas.
4. *(Caso alternativo)* **Dado** una historia con una sesión Abierta en este momento, **cuando** un
   integrante consulta el historial, **entonces** ve que hay una sesión en curso, con su facilitador y
   su número de ronda, y sin los votos de la ronda vigente si todavía no se reveló.
5. *(Caso límite)* **Dado** una historia que nunca se estimó por Planning Poker, **cuando** un
   integrante consulta su historial, **entonces** recibe un historial vacío con la indicación de que la
   historia no tiene sesiones de estimación; no es un error.
6. *(Caso límite)* **Dado** una sesión cerrada en la que participó alguien que después dejó de ser
   integrante del proyecto, **cuando** un integrante consulta el historial, **entonces** los votos de
   esa persona siguen figurando con su nombre.
7. *(Caso límite)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante consulta el
   historial de estimación de una de sus historias, **entonces** la consulta se responde, porque un
   proyecto Finalizado admite lectura.
8. *(Caso de error)* **Dado** una historia de un proyecto, **cuando** un usuario que no es integrante
   de ese proyecto consulta su historial de estimación, **entonces** recibe una respuesta de historia
   inexistente, sin revelar ningún dato.
9. *(Caso de error)* **Dado** una historia con una sesión de estimación Finalizada y otra Cancelada,
   **cuando** un integrante intenta eliminarla del backlog, **entonces** la eliminación se rechaza
   indicando que tiene historial de estimación asociado, y el historial sigue consultable.

---

### Casos Límite

- **Sesión con un único participante**: válida de punta a punta. Al votar, la ronda se revela
  automáticamente porque todos los conectados votaron, y el mismo participante puede registrar la
  estimación acordada si es el facilitador.
- **Consenso en la primera ronda**: no cierra la sesión por sí mismo; el facilitador puede registrar el
  valor directamente o abrir otra ronda si quiere.
- **Todos votan "?"**: la ronda se revela, no hay consenso ni divergencia y no se señalan mínimo ni
  máximo. El resultado informa que no hubo ningún voto numérico.
- **Un único voto numérico y el resto "?"**: hay consenso en ese valor y no se señala mínimo ni máximo,
  porque no hay diferencias numéricas.
- **Cambio de voto en el mismo instante de la revelación**: se resuelve hacia un único resultado y
  todos los participantes ven el mismo conjunto de votos; no hay dos versiones de la misma ronda.
- **El último participante sin votar se desconecta**: dispara la revelación automática, porque la
  condición se evalúa sobre los que siguen conectados.
- **Todos los participantes se desconectan sin votar**: no se revela nada y la sesión queda Abierta en
  la misma ronda hasta que alguien vuelva.
- **La historia se modifica mientras hay una sesión abierta**: el cambio de título, descripción o
  criterios de aceptación se refleja en la sesión en menos de 2 segundos, para que nadie siga votando
  contra un texto viejo.
- **La historia se reestima directamente desde el backlog mientras hay una sesión abierta**: el cambio
  directo se acepta y la sesión sigue su curso sin enterarse; cuando la sesión se cierra con un valor
  acordado, ese valor reemplaza al que había quedado. Gana la última escritura.
- **La historia pasa a Completada o el proyecto pasa a Finalizado con una sesión abierta**: la sesión se
  cancela automáticamente indicando el motivo, y su historial de rondas se conserva.
- **Muchas rondas sin consenso**: sin límite de rondas; todas quedan en el historial de la sesión.
- **Sesión abandonada sin cerrar ni cancelar**: se cancela automáticamente por inactividad después de
  60 minutos sin ningún hecho, de modo que la historia vuelva a admitir una sesión nueva. El historial
  de rondas se conserva y el motivo de la cancelación queda registrado.
- **Sesión larga con pausas cortas**: no caduca; cualquier hecho de la sesión reinicia la cuenta de
  inactividad, así que una discusión de media hora entre dos rondas no la cancela.
- **Un integrante se une cuando la ronda ya está en votación**: queda como participante conectado de
  esa ronda, puede votar y se cuenta para la revelación automática.
- **Un integrante se une cuando la ronda ya está Revelada**: ve los votos revelados, no puede votar en
  esa ronda y participa de la siguiente.
- **Un participante deja de ser integrante del proyecto en medio de la sesión**: pierde el acceso a la
  sesión y deja de contarse para la revelación automática; sus votos ya emitidos se conservan en el
  historial.
- **Dos sesiones abiertas al mismo tiempo sobre historias distintas del mismo proyecto**: permitido; el
  límite de una sola sesión abierta aplica por historia, no por proyecto.
- **Una misma persona participando de dos sesiones a la vez**: permitido; cada sesión lleva su
  presencia y sus votos por separado.
- **Una misma persona con dos pestañas abiertas en la misma sesión**: es un único participante con dos
  conexiones. Cuenta una sola vez para la revelación automática, ve su propio voto en las dos pantallas
  y solo queda desconectada cuando cae la última conexión.
- **Sesión abierta sobre una historia comprometida en un sprint abierto**: permitida; registrar la
  estimación acordada cambia un valor de Story Points por otro, que es una operación admitida durante
  el sprint.

---

## Requisitos *(obligatorio)*

### Requisitos Funcionales

**Apertura de la sesión**

- **FR-001**: El sistema DEBE permitir a cualquier integrante de un proyecto abrir una sesión de
  Planning Poker sobre una historia del backlog de ese proyecto.
- **FR-002**: El sistema DEBE registrar como facilitador de la sesión al integrante que la abre, y
  sumarlo a la sesión como participante conectado.
- **FR-003**: Toda sesión DEBE nacer en estado Abierta y con una primera ronda numerada 1 en estado En
  votación y sin votos.
- **FR-004**: Cada sesión DEBE tener un identificador propio, estable, inmutable y no secuencial.
- **FR-005**: Cada sesión DEBE pertenecer a exactamente una historia y NO DEBE poder moverse a otra.
- **FR-006**: El sistema DEBE rechazar la apertura de una segunda sesión sobre una historia que ya
  tiene una sesión Abierta, indicando el identificador de la sesión en curso para poder unirse a ella.
- **FR-007**: El sistema DEBE rechazar la apertura de una sesión sobre una historia en estado
  Completada, indicando que una historia completada no se estima.
- **FR-008**: El sistema DEBE rechazar toda apertura de sesión en un proyecto en estado Finalizado,
  indicando que el proyecto es de solo lectura.
- **FR-009**: El sistema DEBE permitir abrir una sesión sobre una historia sin criterios de aceptación
  y sobre una historia ya estimada, sin exigir ninguna condición previa más allá de FR-007 y FR-008.
- **FR-010**: El sistema DEBE registrar en cada sesión quién la abrió, su momento de apertura, quién la
  cerró y su momento de cierre.

**Participación y presencia**

- **FR-011**: El sistema DEBE permitir a cualquier integrante del proyecto unirse a una sesión Abierta
  de una historia de ese proyecto.
- **FR-012**: El sistema DEBE entregar a cada participante que se une el título, la descripción y los
  criterios de aceptación de la historia en su orden, la escala de cartas disponible, la lista de
  participantes conectados con la indicación de quién ya votó, el número de ronda y el estado de la
  ronda.
- **FR-013**: El sistema DEBE mantener, para cada sesión, la lista de participantes con su estado de
  presencia (conectado o desconectado). Un usuario DEBE contar como un único participante de la sesión
  aunque tenga varias conexiones abiertas a la vez (varias pestañas o varios dispositivos), y DEBE
  figurar como conectado mientras le quede al menos una conexión viva.
- **FR-014**: El sistema DEBE considerar desconectado al participante que se queda sin ninguna conexión
  viva —porque cerró la sesión en todas sus pantallas o perdió la conexión— y no restablece su presencia
  dentro de 30 segundos.
- **FR-015**: El sistema DEBE permitir que un participante desconectado se reincorpore a la misma
  sesión conservando su identidad de participante.
- **FR-016**: Al reincorporarse durante la misma ronda, el participante DEBE recuperar su estado y su
  voto de esa ronda si lo había emitido.
- **FR-017**: El sistema DEBE permitir unirse a una ronda ya en curso; quien se une queda como
  participante conectado de esa ronda y puede votar mientras la ronda esté En votación.
- **FR-018**: El sistema DEBE rechazar la incorporación a una sesión Finalizada o Cancelada, indicando
  que la sesión está cerrada y con qué resultado.
- **FR-019**: El sistema DEBE excluir de la sesión, y del recuento de la revelación automática, al
  participante que deja de ser integrante del proyecto, conservando sus votos ya emitidos en el
  historial.

**Votación y secreto del voto**

- **FR-020**: El sistema DEBE admitir como carta de voto únicamente los valores 0, 1, 2, 3, 5, 8, 13,
  20, 40 y 100 y la carta "?".
- **FR-021**: El sistema DEBE permitir a cada participante conectado emitir un voto por ronda mientras
  la ronda esté En votación.
- **FR-022**: El sistema DEBE permitir a un participante cambiar su voto tantas veces como quiera
  mientras la ronda esté En votación; el voto nuevo reemplaza al anterior y el anterior no queda
  registrado. El sistema NO DEBE permitir retirar un voto ya emitido para volver al estado sin voto: el
  intento se rechaza indicando que un voto emitido solo se cambia por otra carta, incluida "?".
- **FR-023**: Antes de la revelación, el sistema DEBE mostrar a todos los participantes quién emitió su
  voto y NO DEBE exponer el valor votado por otra persona por ningún medio.
- **FR-024**: Antes de la revelación, cada participante DEBE poder ver su propio voto, y DEBE verlo
  igual en todas sus conexiones abiertas.
- **FR-025**: El sistema DEBE rechazar todo voto con un valor que no sea una carta admitida, incluidos
  los números intermedios de la escala, los negativos y los decimales, indicando las cartas admitidas y
  sin alterar el voto vigente de quien lo intentó.
- **FR-026**: El sistema DEBE rechazar todo voto emitido sobre una ronda ya Revelada, indicando que hay
  que esperar una ronda nueva, y sin alterar los votos revelados.
- **FR-027**: El sistema DEBE rechazar todo voto emitido sobre una sesión Finalizada o Cancelada.
- **FR-028**: El sistema DEBE rechazar el voto de quien no es participante conectado de la sesión.
- **FR-029**: El sistema DEBE resolver un cambio de voto simultáneo con la revelación hacia un único
  resultado, de modo que todos los participantes vean el mismo conjunto de votos revelados para esa
  ronda.

**Revelación y detección de diferencias**

- **FR-030**: El sistema DEBE revelar la ronda automáticamente en cuanto haya al menos un participante
  conectado y todos los participantes conectados hayan emitido su voto.
- **FR-031**: El sistema DEBE evaluar la condición de revelación automática cada vez que se emite un
  voto, que un participante se conecta y que un participante se desconecta.
- **FR-032**: El sistema DEBE permitir al facilitador revelar la ronda manualmente cuando exista al
  menos un voto emitido en ella, incluso si hay participantes conectados que no votaron.
- **FR-033**: El sistema DEBE rechazar la revelación manual cuando la ronda no tenga ningún voto
  emitido, indicando que hace falta al menos uno.
- **FR-034**: El sistema DEBE rechazar toda revelación manual pedida por un participante que no es el
  facilitador, dejando la ronda En votación y los votos ocultos.
- **FR-035**: Al revelar, el sistema DEBE mostrar a todos los participantes el voto de cada
  participante que votó y marcar como "sin voto" a los participantes conectados que no votaron.
- **FR-036**: El sistema DEBE marcar la ronda con consenso cuando exista al menos un voto numérico y
  todos los votos numéricos de la ronda sean iguales.
- **FR-037**: El sistema DEBE marcar la ronda con divergencia cuando existan al menos dos votos
  numéricos distintos.
- **FR-038**: El sistema DEBE excluir del cálculo de consenso, mínimo y máximo los votos "?" y los
  participantes sin voto.
- **FR-039**: El sistema DEBE informar, en una ronda con divergencia, el voto numérico mínimo y el
  máximo junto con todos sus autores.
- **FR-040**: El sistema NO DEBE informar mínimo ni máximo en una ronda con consenso ni en una ronda
  sin ningún voto numérico.
- **FR-041**: El sistema DEBE informar explícitamente que una ronda no tuvo ningún voto numérico cuando
  todos los votos emitidos sean "?", y esa ronda NO DEBE marcarse con consenso.
- **FR-042**: El sistema NO DEBE calcular ni mostrar promedio, mediana ni ninguna otra agregación de
  los votos más allá del consenso, el mínimo y el máximo.

**Nueva ronda**

- **FR-043**: El sistema DEBE permitir al facilitador iniciar una ronda nueva cuando la ronda vigente
  esté Revelada.
- **FR-044**: Al iniciar una ronda nueva, el sistema DEBE cerrar la ronda revelada conservándola en el
  historial de la sesión con sus votos y su resultado, y abrir una ronda En votación sin votos, con el
  número inmediatamente siguiente.
- **FR-045**: Al iniciar una ronda nueva, el sistema DEBE dejar de mostrar los votos de la ronda
  anterior en la vista de votación, sin borrarlos del historial.
- **FR-046**: El sistema DEBE rechazar el inicio de una ronda nueva mientras la ronda vigente esté En
  votación, indicando que primero hay que revelarla.
- **FR-047**: El sistema DEBE rechazar el inicio de una ronda nueva pedido por un participante que no
  es el facilitador.
- **FR-048**: El sistema NO DEBE limitar la cantidad de rondas de una sesión.

**Registro de la estimación acordada**

- **FR-049**: El sistema DEBE permitir al facilitador registrar la estimación acordada eligiendo un
  valor numérico de la escala 0, 1, 2, 3, 5, 8, 13, 20, 40 o 100, sin exigir que coincida con ningún
  voto emitido.
- **FR-050**: El sistema DEBE rechazar como valor acordado la carta "?", la marca "sin estimar" y todo
  valor fuera de la escala numérica, indicando los valores admitidos y sin cerrar la sesión.
- **FR-051**: El sistema DEBE exigir al menos una ronda Revelada en la sesión para registrar la
  estimación acordada; si no hay ninguna, la acción se rechaza indicando que hay que revelar una ronda
  o cancelar la sesión.
- **FR-052**: Al registrarse la estimación acordada, el sistema DEBE pasar la sesión a Finalizada,
  guardar el valor acordado y actualizar los Story Points de la historia con ese valor, reemplazando el
  valor anterior.
- **FR-053**: El sistema DEBE registrar quién cerró la sesión y en qué momento.
- **FR-054**: El sistema DEBE rechazar el registro de la estimación acordada pedido por un participante
  que no es el facilitador, sin alterar los Story Points de la historia.
- **FR-055**: El sistema DEBE rechazar el registro de la estimación acordada sobre una sesión
  Finalizada o Cancelada, sin modificar el valor ya registrado.
- **FR-056**: Una sesión Finalizada NO DEBE admitir cambios de ningún tipo: ni en su valor acordado, ni
  en sus rondas, ni en sus votos.

**Cancelación y cierre sin estimación**

- **FR-057**: El sistema DEBE permitir al facilitador cancelar la sesión en cualquier momento mientras
  esté Abierta, con la ronda vigente En votación o Revelada.
- **FR-058**: Al cancelarse la sesión, el sistema DEBE pasarla a Cancelada, dejar los Story Points de
  la historia exactamente como estaban y conservar todas sus rondas y votos.
- **FR-059**: El sistema DEBE rechazar la cancelación pedida por un participante que no es el
  facilitador.
- **FR-060**: El sistema DEBE rechazar la cancelación de una sesión Finalizada o Cancelada.
- **FR-061**: El sistema DEBE cancelar automáticamente la sesión Abierta de una historia cuando esa
  historia pase a Completada o cuando su proyecto pase a Finalizado, registrando el motivo de la
  cancelación automática y avisando a los participantes conectados.
- **FR-062**: El sistema DEBE permitir abrir una sesión nueva sobre una historia cuya sesión anterior
  quedó Finalizada o Cancelada.
- **FR-063**: El sistema DEBE cancelar automáticamente por inactividad toda sesión Abierta que pase 60
  minutos sin registrar ningún hecho, entendiendo por hecho la incorporación o la desconexión de un
  participante, la emisión o el cambio de un voto, una revelación, el inicio de una ronda nueva y el
  cambio de facilitador. Cada hecho DEBE reiniciar la cuenta de inactividad. La cancelación por
  inactividad DEBE registrar ese motivo, avisar a los participantes conectados y conservar las rondas
  y los votos, igual que la cancelación automática de FR-061.

**Rol de facilitador y continuidad**

- **FR-064**: El sistema DEBE reservar al facilitador las acciones de revelar manualmente, iniciar una
  ronda nueva, registrar la estimación acordada y cancelar la sesión.
- **FR-065**: El sistema DEBE permitir al facilitador votar como cualquier otro participante, y su voto
  DEBE contarse igual que el de los demás para la revelación automática y para el cálculo de consenso,
  mínimo y máximo.
- **FR-066**: El sistema DEBE mantener la sesión Abierta cuando el facilitador se desconecta, sin
  interrumpir la ronda en curso.
- **FR-067**: El sistema DEBE devolver al facilitador sus atribuciones cuando se reconecta, siempre que
  el rol no haya sido tomado por otro participante.
- **FR-068**: El sistema DEBE permitir a cualquier participante conectado tomar el rol de facilitador
  cuando el facilitador vigente lleve 5 minutos o más desconectado de forma continua.
- **FR-069**: El sistema DEBE rechazar la toma del rol de facilitador mientras el facilitador vigente
  esté conectado o lleve menos de 5 minutos desconectado, indicando cuánto falta para que el rol quede
  disponible.
- **FR-070**: Al tomarse el rol, el participante que lo toma DEBE pasar a ser el único facilitador de la
  sesión y el anterior DEBE quedar como participante común, incluso si después se reconecta.
- **FR-071**: El sistema DEBE informar a todos los participantes conectados el cambio de facilitador.
- **FR-072**: Una sesión DEBE tener exactamente un facilitador en todo momento mientras esté Abierta.
  Ante dos o más pedidos simultáneos de tomar el rol, DEBE prosperar únicamente el primero que el sistema
  procesa; los demás DEBEN rechazarse indicando quién es el facilitador vigente, y los participantes
  conectados DEBEN ver un único cambio de facilitador.

**Actualización en tiempo real**

- **FR-073**: El sistema DEBE reflejar en la vista de todos los participantes conectados, sin que
  recarguen la pantalla y en menos de 2 segundos, cada uno de estos hechos: incorporación de un
  participante, salida o desconexión de un participante, emisión de un voto (sin su valor), cambio de
  ronda, revelación, cambio de facilitador, cierre con estimación acordada y cancelación. El límite de 2
  segundos DEBE cumplirse en las condiciones de escala de RC-14.
- **FR-074**: El sistema DEBE reflejar en la sesión, en menos de 2 segundos, los cambios de título,
  descripción y criterios de aceptación que la historia reciba mientras la sesión está Abierta.
- **FR-075**: El sistema DEBE aceptar la reestimación directa de los Story Points de una historia desde
  el backlog aunque tenga una sesión de estimación Abierta, sin cancelar la sesión ni interrumpir la
  ronda en curso. El valor acordado que la sesión registre al cerrarse DEBE reemplazar el valor vigente
  de la historia, cualquiera sea su origen: gana la última escritura. La reestimación directa NO DEBE
  mostrarse como un hecho de la sesión ni alterar los votos ya emitidos.
- **FR-076**: El sistema DEBE dejar a todos los participantes conectados con la misma vista del estado
  de la sesión: número de ronda, estado de la ronda, quién votó, votos revelados y facilitador vigente.
- **FR-077**: El sistema DEBE permitir que una persona recupere el estado completo y vigente de la
  sesión al reconectarse, sin depender de los hechos ocurridos mientras estuvo desconectada.

**Historial y consulta**

- **FR-078**: El sistema DEBE conservar las rondas y los votos de una sesión después de que la sesión
  quede Finalizada o Cancelada.
- **FR-079**: El sistema DEBE permitir a cualquier integrante del proyecto consultar el historial de
  estimación de una historia: sus sesiones, con su estado final, su valor acordado, sus rondas y el
  voto de cada participante en cada ronda.
- **FR-080**: El sistema DEBE devolver las sesiones del historial ordenadas de la más reciente a la más
  antigua por su momento de apertura.
- **FR-081**: El sistema DEBE informar, en la consulta de una historia, si tiene una sesión Abierta en
  ese momento, con su identificador, su facilitador, su número de ronda y el estado de la ronda.
- **FR-082**: El sistema NO DEBE exponer en ninguna consulta los votos de una ronda En votación que
  todavía no fue revelada.
- **FR-083**: El sistema DEBE devolver un historial vacío, con la indicación correspondiente y sin
  tratarlo como error, cuando la historia nunca tuvo una sesión de estimación.
- **FR-084**: El sistema DEBE conservar en el historial los votos de las personas que dejaron de ser
  integrantes del proyecto, atribuidos a su nombre.
- **FR-085**: El sistema DEBE permitir la consulta del historial de estimación en un proyecto
  Finalizado.

**Acceso y visibilidad**

- **FR-086**: El sistema DEBE exigir sesión válida para toda acción y toda consulta de esta feature.
- **FR-087**: El sistema DEBE exigir membresía vigente en el proyecto para abrir una sesión, unirse a
  ella, votar, ejecutar acciones de facilitador y consultar el historial.
- **FR-088**: El sistema DEBE responder a quien no es integrante del proyecto de forma indistinguible
  de un identificador inexistente, sin revelar la existencia de la historia ni de la sesión.

**Conservación del historial frente a la eliminación de la historia**

- **FR-089**: El sistema DEBE impedir la eliminación de una historia que tenga al menos una sesión de
  estimación, sin importar si quedó Finalizada o Cancelada, indicando que tiene historial de estimación
  asociado. Es una cuarta condición que se suma a las tres que ya bloquean la eliminación en
  `specs/003-product-backlog` (FR-041 y FR-043): haber estado en un sprint, tener esfuerzo registrado y
  tener defectos asociados.

---

### Reglas de Negocio

| ID | Regla |
| --- | --- |
| RN-01 | Solo los integrantes del proyecto pueden abrir una sesión de estimación, unirse a ella y votar. |
| RN-02 | Cualquier integrante puede abrir una sesión, sin importar quién creó la historia. |
| RN-03 | Quien abre la sesión es su facilitador. |
| RN-04 | Una historia admite como máximo una sesión Abierta a la vez; puede tener cuantas sesiones cerradas haga falta. |
| RN-05 | No se abren sesiones sobre historias Completadas ni sobre historias de un proyecto Finalizado. |
| RN-06 | Las cartas admitidas son 0, 1, 2, 3, 5, 8, 13, 20, 40, 100 y "?"; no hay otras. |
| RN-07 | Un participante tiene como máximo un voto por ronda y puede cambiarlo mientras la ronda esté En votación. |
| RN-08 | Antes de la revelación se ve quién votó, nunca qué votó. |
| RN-09 | La revelación automática ocurre cuando hay al menos un participante conectado y todos los conectados votaron. |
| RN-10 | La revelación manual la ejecuta solo el facilitador y exige al menos un voto emitido en la ronda. |
| RN-11 | Una ronda Revelada no admite votos nuevos ni cambios de voto. |
| RN-12 | El voto "?" cuenta como voto emitido para la revelación automática, pero no participa del cálculo de consenso, mínimo ni máximo. |
| RN-13 | Hay consenso cuando existe al menos un voto numérico y todos los votos numéricos son iguales; si todos votan "?", no hay consenso. |
| RN-14 | En una ronda con divergencia se señalan el voto mínimo y el máximo con todos sus autores. |
| RN-15 | Solo el facilitador revela manualmente, inicia rondas nuevas, registra la estimación acordada y cancela la sesión. |
| RN-16 | El facilitador vota como cualquier otro participante. |
| RN-17 | Una ronda nueva solo se inicia sobre una ronda Revelada, y no hay límite de rondas por sesión. |
| RN-18 | El valor acordado es un valor numérico de la escala, elegido por el facilitador, y puede no coincidir con ningún voto. |
| RN-19 | Registrar la estimación acordada finaliza la sesión y actualiza los Story Points de la historia. |
| RN-20 | Registrar la estimación acordada exige al menos una ronda revelada en la sesión. |
| RN-21 | Cancelar la sesión no modifica los Story Points de la historia. |
| RN-22 | Una sesión Finalizada o Cancelada es definitiva: no se reabre, ni se modifica, ni admite participantes nuevos. |
| RN-23 | Un participante desconectado deja de contarse para la revelación automática; si se reconecta en la misma ronda, recupera su estado y su voto. |
| RN-24 | Si el facilitador se desconecta, la sesión sigue Abierta y él retoma el rol al reconectarse, salvo que otro ya lo haya tomado. |
| RN-25 | Cualquier participante conectado puede tomar el rol de facilitador si el facilitador vigente lleva 5 minutos o más desconectado. |
| RN-26 | Mientras la sesión está Abierta hay exactamente un facilitador. |
| RN-27 | Todos los participantes conectados ven cada cambio de la sesión en menos de 2 segundos y sin recargar. |
| RN-28 | El historial de rondas y votos se conserva después de cerrada la sesión, porque forma parte de los reportes de estimación. |
| RN-29 | El historial de estimación de una historia lo consulta cualquier integrante del proyecto, incluso con el proyecto Finalizado. |
| RN-30 | Una sesión Abierta se cancela automáticamente si su historia pasa a Completada o su proyecto pasa a Finalizado. |
| RN-31 | Una sesión Abierta que pasa 60 minutos sin ningún hecho se cancela automáticamente por inactividad; cada hecho de la sesión reinicia la cuenta. |
| RN-32 | La reestimación directa de una historia desde el backlog no se bloquea ni interrumpe su sesión Abierta; al cerrarse la sesión, el valor acordado reemplaza el valor vigente. |
| RN-33 | Una historia con al menos una sesión de estimación, Finalizada o Cancelada, no se elimina del backlog, porque su historial de rondas y votos es parte de los reportes de estimación. |
| RN-34 | Un usuario es un único participante por sesión aunque abra varias conexiones; está conectado mientras le quede al menos una y su voto es el mismo en todas sus pantallas. |
| RN-35 | Ante pedidos simultáneos de tomar el rol de facilitador prospera solo el primero que se procesa; los demás se rechazan y la sesión nunca queda con dos facilitadores. |
| RN-36 | Un voto emitido no se retira: solo se cambia por otra carta, incluida "?". Nadie vuelve al estado sin voto dentro de la misma ronda. |

---

### Restricciones

- **RC-01**: Esta feature depende de la feature de autenticación y cuentas de usuario
  (`specs/001-user-auth`): solo usuarios registrados con sesión válida participan de una sesión de
  estimación, y su identidad es la que se muestra en la lista de participantes y en el historial de
  votos.
- **RC-02**: Esta feature depende de la feature de gestión de proyectos e integrantes
  (`specs/002-project-members`): la sesión existe dentro de un proyecto, el permiso de acceso es la
  membresía vigente y el estado Finalizado del proyecto impide abrir y operar sesiones.
- **RC-03**: Esta feature depende de la feature de Product Backlog (`specs/003-product-backlog`): la
  historia, sus criterios de aceptación, su estado y su valor de Story Points están definidos allí.
  Esta feature no crea, modifica ni elimina historias: el único campo de la historia que escribe son
  sus Story Points, al registrarse la estimación acordada.
- **RC-04**: La escala de Story Points es la definida en `specs/003-product-backlog` (RN-06): 0, 1, 2,
  3, 5, 8, 13, 20, 40 y 100. Esta feature no la extiende ni la configura, y agrega la carta "?" como
  carta de votación que no es un valor de Story Points.
- **RC-05**: La marca "sin estimar" de `specs/003-product-backlog` no es un resultado posible de una
  sesión: una sesión termina con un valor numérico de la escala (Finalizada) o sin tocar la estimación
  (Cancelada).
- **RC-06**: El estado de la historia (Pendiente, En progreso, Completada) lo produce la feature de
  sprints (`specs/004-sprint-management`). Esta feature lo consulta para impedir estimar historias
  Completadas, pero nunca lo cambia.
- **RC-07**: La condición "lista para planificar" de `specs/003-product-backlog` (RN-12) se recalcula
  como consecuencia de que la sesión escriba los Story Points de la historia; esta feature no la
  gestiona directamente.
- **RC-08**: Una historia comprometida en un sprint abierto puede estimarse por Planning Poker, porque
  registrar la estimación acordada reemplaza un valor de Story Points por otro, operación admitida por
  `specs/003-product-backlog` (FR-018) y `specs/004-sprint-management`. Lo que esas features prohíben
  es dejar la historia sin estimación, y una sesión nunca produce ese resultado (ver RC-05).
- **RC-09**: La actualización simultánea en varias pantallas es un requisito de comportamiento
  observable (menos de 2 segundos, sin recargar); esta especificación no fija el mecanismo que lo
  consigue.
- **RC-10**: Los reportes y métricas de estimación que consumen el historial de rondas y votos
  (desvío entre lo estimado y lo real, evolución de la capacidad de estimar) se especifican en features
  propias; esta feature solo garantiza que el dato se conserve y sea consultable.
- **RC-11**: La feature no notifica a los integrantes que se abrió una sesión: quien quiera participar
  se une desde la historia o desde la sesión en curso. Las notificaciones y las invitaciones quedan
  fuera de alcance.
- **RC-12**: La estimación directa de una historia desde el backlog sigue regida por
  `specs/003-product-backlog` (FR-016 y FR-017) y no se restringe por la existencia de una sesión
  Abierta. Decisión tomada el 2026-09-26 (ver Clarifications): en este punto esta feature no enmienda esa
  spec, y ambas escriben el mismo campo de Story Points bajo la regla de última escritura (RN-32).
- **RC-13**: Esta feature sí enmienda `specs/003-product-backlog` en la eliminación de historias: su
  FR-041, su FR-043, su RN-16 y su RC-05 pasan a contemplar el historial de estimación como una cuarta
  condición que la bloquea. Decisión tomada el 2026-09-26 (ver Clarifications). Saber si una historia
  tiene sesiones de estimación es un dato que produce esta feature; aplicar la condición al eliminar es
  responsabilidad de la feature de backlog.
- **RC-14**: La escala que esta feature se compromete a sostener es de hasta 10 participantes conectados
  por sesión y hasta 10 sesiones en paralelo en todo el sistema, es decir 100 conexiones concurrentes.
  Decisión tomada el 2026-09-26 (ver Clarifications). Es una capacidad objetivo y no un tope: el sistema
  no rechaza la incorporación del participante 11 ni la apertura de la sesión 11, pero el límite de 2
  segundos de FR-073 solo se compromete dentro de esa escala.

---

### Condiciones de Error

| Condición | Comportamiento esperado |
| --- | --- |
| Acción o consulta de la feature sin sesión válida | Rechazo con indicación de iniciar sesión; la acción no se ejecuta. |
| Apertura de una segunda sesión sobre una historia con sesión Abierta | Rechazo indicando que ya hay una sesión en curso, con su identificador para unirse; no se crea nada. |
| Apertura de una sesión sobre una historia Completada | Rechazo indicando que una historia completada no se estima. |
| Apertura o cualquier acción de escritura en un proyecto Finalizado | Rechazo por proyecto de solo lectura. |
| Voto con un valor que no es una carta admitida (4, 50, negativo, decimal) | Rechazo indicando las cartas admitidas; el voto vigente no cambia. |
| Voto o cambio de voto sobre una ronda ya Revelada | Rechazo indicando que la ronda está revelada y que hay que esperar una ronda nueva. |
| Voto de un integrante que no se unió a la sesión | Rechazo por no participar de la sesión. |
| Intento de retirar un voto ya emitido | Rechazo indicando que solo se cambia por otra carta, incluida "?"; el voto vigente no cambia. |
| Voto sobre una sesión Finalizada o Cancelada | Rechazo por sesión cerrada. |
| Revelación manual sin ningún voto emitido en la ronda | Rechazo indicando que hace falta al menos un voto. |
| Revelación manual de una ronda ya Revelada | Rechazo indicando que la ronda ya fue revelada. |
| Acción de facilitador (revelar, nueva ronda, registrar, cancelar) ejecutada por otro participante | Rechazo por acción reservada al facilitador; nada cambia. |
| Inicio de una ronda nueva con la ronda vigente En votación | Rechazo indicando que primero hay que revelar la ronda vigente. |
| Registro de un valor final fuera de la escala numérica, "?" o "sin estimar" | Rechazo indicando los valores admitidos; la sesión sigue Abierta y los Story Points no cambian. |
| Registro de la estimación acordada sin ninguna ronda revelada | Rechazo indicando que hay que revelar una ronda o cancelar la sesión. |
| Registro o cancelación sobre una sesión Finalizada o Cancelada | Rechazo por sesión cerrada; el resultado registrado no se modifica. |
| Incorporación a una sesión Finalizada o Cancelada | Rechazo indicando que la sesión está cerrada y con qué resultado. |
| Toma del rol de facilitador con el facilitador conectado o desconectado hace menos de 5 minutos | Rechazo indicando cuánto falta para que el rol quede disponible. |
| Toma del rol de facilitador por quien no es participante conectado | Rechazo por no participar de la sesión. |
| Dos pedidos simultáneos de tomar el rol de facilitador | Prospera el primero que se procesa; el otro se rechaza indicando quién es el facilitador vigente. |
| Pérdida de conexión de un participante en medio de una ronda | No es un error de la sesión: el participante queda desconectado, deja de contarse para la revelación automática y su voto se conserva para cuando vuelva. |
| Cierre de una de las varias conexiones de un mismo participante | No es un error ni una desconexión: sigue conectado con su voto intacto mientras le quede al menos una conexión viva. |
| Pérdida de conexión del facilitador en medio de una ronda | No es un error: la sesión sigue Abierta y el rol queda disponible para otro participante a los 5 minutos. |
| Pérdida de conexión de todos los participantes | La sesión queda Abierta en la misma ronda; no se revela nada. |
| Acción o consulta sobre una historia o una sesión de un proyecto del que quien pide no es integrante | Respuesta idéntica a la de un identificador inexistente, sin revelar ningún dato. |
| Consulta de una sesión o una historia con un identificador que no existe | Respuesta de sesión o historia inexistente. |
| Historia sin ninguna sesión de estimación | Historial vacío con la indicación correspondiente; no es un error. |
| Sesión Abierta sin ningún hecho durante 60 minutos | No es un error: la sesión se cancela automáticamente por inactividad, se avisa a los participantes conectados y el historial de rondas se conserva. |
| Reestimación directa de la historia con una sesión Abierta | No es un error: el cambio directo se acepta, la sesión sigue su curso y el valor acordado al cerrar reemplaza el vigente. |
| Acción sobre una sesión cancelada por inactividad | Rechazo por sesión cerrada, indicando que se canceló por inactividad. |
| Eliminación de una historia con al menos una sesión de estimación | Rechazo indicando que tiene historial de estimación asociado; ni la historia ni su historial se tocan. |

---

### Entidades Clave

- **Sesión de estimación**: representa un encuentro de Planning Poker sobre una única historia.
  Atributos relevantes: identificador propio estable y no secuencial, historia estimada, facilitador
  vigente, estado (Abierta, Finalizada o Cancelada), valor acordado (solo cuando está Finalizada),
  motivo de cancelación cuando la cancela el sistema, quién la abrió, quién la cerró, momento de
  apertura, momento del último hecho registrado (del que depende la caducidad por inactividad) y
  momento de cierre. Es el contenedor del historial de rondas y votos de una estimación.
- **Estado de la sesión**: representa el punto del ciclo de vida de la sesión. Valores posibles:
  Abierta, Finalizada (terminó con un valor acordado) y Cancelada (terminó sin tocar la estimación).
  Desde Abierta se llega a cualquiera de los otros dos y no hay vuelta atrás.
- **Ronda**: representa un intento de estimación dentro de una sesión. Atributos relevantes: sesión a
  la que pertenece, número secuencial dentro de la sesión, estado (En votación o Revelada), momento de
  apertura, momento de revelación, resultado (consenso con su valor, divergencia con su mínimo y su
  máximo, o sin votos numéricos). Las rondas de una sesión se conservan completas después del cierre.
- **Participante**: representa la presencia de un integrante del proyecto en una sesión de estimación.
  Atributos relevantes: sesión, usuario, si es el facilitador vigente, estado de presencia (conectado o
  desconectado), momento en que se incorporó y momento de su última desconexión. Un mismo usuario tiene
  como máximo un participante por sesión, sin importar cuántas conexiones abra a la vez: está conectado
  mientras le quede al menos una.
- **Voto**: representa la carta que un participante jugó en una ronda. Atributos relevantes: ronda,
  participante, carta jugada (un valor de la escala o "?") y momento en que quedó emitido. Un
  participante tiene como máximo un voto por ronda; antes de la revelación su valor solo es visible
  para su autor.
- **Carta**: valor que un participante puede jugar. Conjunto cerrado: los diez valores de la escala de
  Story Points (0, 1, 2, 3, 5, 8, 13, 20, 40, 100) más la carta "?", que expresa que la persona no
  sabe estimar la historia y que no interviene en ningún cálculo.
- **Resultado de la ronda**: representa la lectura que el equipo hace de una ronda revelada. Valores
  posibles: consenso (con el valor acordado por todos los votos numéricos), divergencia (con el mínimo
  y el máximo y sus autores) y sin votos numéricos (cuando todos votaron "?" o nadie votó un número).
- **Historia de usuario** *(entidad de `specs/003-product-backlog`)*: esta feature la consume para
  mostrar su título, su descripción y sus criterios de aceptación, y para verificar que no está
  Completada; el único campo que escribe son sus Story Points, al registrarse la estimación acordada.
  Además, la existencia de sesiones de estimación sobre ella pasa a ser una condición que bloquea su
  eliminación del backlog (RN-33, RC-13).
- **Proyecto** e **Integrante del proyecto** *(entidades de `specs/002-project-members`)*: esta feature
  los consume para ubicar la sesión, decidir quién accede y saber si el proyecto está Finalizado; no los
  modifica.
- **Usuario** *(entidad de `specs/001-user-auth`)*: esta feature lo consume para identificar a los
  participantes y atribuir cada voto; no lo modifica.

---

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **SC-001**: Un integrante abre una sesión de estimación sobre una historia del backlog en menos de 30
  segundos y en no más de 3 pasos.
- **SC-002**: Un participante emite o cambia su voto en menos de 10 segundos desde que ve la historia,
  en un solo paso.
- **SC-003**: Un equipo de 5 personas estima una historia con dos rondas en menos de 5 minutos, sin
  contar la discusión, y el 95 % de los participantes lo logra sin ayuda externa en su primer uso.
- **SC-004**: El 100 % de los cambios de la sesión (incorporación, salida, desconexión, voto emitido,
  revelación, ronda nueva, cambio de facilitador, cierre y cancelación) aparece en la pantalla de todos
  los participantes conectados en menos de 2 segundos, sin que nadie recargue, verificado con 10 sesiones
  simultáneas de 10 participantes cada una, es decir 100 conexiones concurrentes (RC-14).
- **SC-005**: Cero exposiciones de un voto ajeno antes de la revelación: en el 100 % de los intentos de
  obtener los votos de una ronda En votación desde una cuenta distinta de la de su autor, la respuesta
  no contiene ningún valor votado, verificado para cada forma de consulta de la sesión.
- **SC-006**: El 100 % de las rondas en las que todos los participantes conectados votaron se revela
  automáticamente, sin intervención del facilitador, verificado con rondas de 1, 2 y 5 participantes.
- **SC-007**: El 100 % de las rondas reveladas muestra el mismo conjunto de votos a todos los
  participantes conectados, verificado comparando la vista de cada participante en al menos 20
  revelaciones, incluidas revelaciones simultáneas con un cambio de voto.
- **SC-008**: El 100 % de las rondas con al menos dos votos numéricos distintos se marca con
  divergencia e informa el mínimo y el máximo con todos sus autores; el 100 % de las rondas con votos
  numéricos todos iguales se marca con consenso y no informa mínimo ni máximo.
- **SC-009**: El 100 % de las rondas en las que todos los votos emitidos son "?" se marca sin consenso
  y sin mínimo ni máximo.
- **SC-010**: El 100 % de los votos con un valor que no es una carta admitida se rechaza, verificado
  probando al menos los valores 4, 6, 7, 50, 101, un negativo y un decimal.
- **SC-011**: El 100 % de las acciones reservadas al facilitador ejecutadas por otro participante se
  rechaza sin producir ningún efecto, verificado con una prueba por acción (revelar, iniciar ronda,
  registrar estimación, cancelar).
- **SC-012**: Cero segundas sesiones abiertas sobre una misma historia, verificado con 10 intentos
  simultáneos de apertura sobre la misma historia desde cuentas distintas: exactamente una prospera.
- **SC-013**: El 100 % de las sesiones Finalizadas deja los Story Points de su historia iguales al
  valor acordado, y el 100 % de las sesiones Canceladas los deja exactamente como estaban.
- **SC-014**: Un participante que pierde la conexión y vuelve dentro de la misma ronda recupera su
  estado y su voto en el 100 % de los casos, verificado con cortes en los tres momentos de la ronda
  (antes de votar, después de votar y después de la revelación).
- **SC-015**: El 100 % de las sesiones cuyo facilitador se desconecta sigue Abierta, y en el 100 % de
  los casos otro participante puede tomar el rol a partir de los 5 minutos y no antes.
- **SC-016**: Cero rondas o votos perdidos después del cierre de una sesión: el 100 % de las sesiones
  cerradas conserva todas sus rondas y todos sus votos consultables, verificado sobre sesiones
  Finalizadas y Canceladas de 1, 3 y 10 rondas.
- **SC-017**: El 100 % de las acciones y consultas de esta feature exige sesión válida y membresía
  vigente en el proyecto, verificado con una prueba por acción.
- **SC-018**: Cero filtraciones de datos entre proyectos: en el 100 % de los intentos de acceso de quien
  no es integrante, la respuesta es indistinguible de la de un identificador inexistente, verificado
  comparando ambas respuestas para cada acción y cada consulta de la feature.
- **SC-019**: Un integrante encuentra en el historial de una historia cuántas rondas hicieron falta y
  qué votó cada persona en menos de 30 segundos, sin combinar información de otros lugares del sistema.
- **SC-020**: El 100 % de las reglas de negocio (RN-01 a RN-36) tiene al menos una prueba automatizada
  asociada que falla si la regla se rompe.
- **SC-021**: El 100 % de las sesiones Abiertas que pasan 60 minutos sin ningún hecho queda Cancelada
  por inactividad, y cero sesiones con al menos un hecho dentro de esa ventana se cancelan, verificado
  con una prueba por tipo de hecho que reinicia la cuenta.
- **SC-022**: Cero sesiones interrumpidas por una reestimación directa de su historia, y el 100 % de
  las sesiones que cierran con un valor acordado deja ese valor en la historia aunque haya habido una
  reestimación directa mientras la sesión estaba Abierta.
- **SC-023**: Cero historiales de estimación perdidos por la eliminación de una historia: el 100 % de
  los intentos de eliminar una historia con al menos una sesión de estimación se rechaza, verificado con
  una historia cuya única sesión quedó Finalizada y con otra cuya única sesión quedó Cancelada.
- **SC-024**: Cero revelaciones automáticas bloqueadas por conexiones repetidas de la misma persona: en
  el 100 % de las rondas en las que algún participante tiene dos conexiones abiertas, la revelación se
  dispara con un voto por persona, verificado también cerrando una de las dos conexiones a mitad de la
  ronda.
- **SC-025**: Cero sesiones con dos facilitadores: en el 100 % de las tandas de pedidos simultáneos de
  tomar el rol, exactamente uno prospera, verificado con al menos 10 pedidos lanzados a la vez desde
  cuentas distintas.

---

## Fuera de Alcance

- Chat, comentarios o discusión escrita dentro de la sesión de estimación.
- Temporizador de votación y cualquier forma de cierre de ronda por tiempo.
- Escalas configurables: la escala de cartas es fija y es la del Product Backlog más la carta "?".
- Estimación de varias historias en cola dentro de una misma sesión: una sesión estima exactamente una
  historia.
- Votos de personas que no son integrantes del proyecto, en cualquier forma (invitados, observadores
  externos, enlaces públicos).
- Promedio, mediana y cualquier otra estadística de los votos más allá del consenso, el mínimo y el
  máximo.
- Sugerencia automática del valor acordado a partir de los votos emitidos.
- Notificaciones o invitaciones a participar de una sesión (por correo, dentro del producto o por
  cualquier otro medio).
- Roles de participación diferenciados dentro de la sesión (observadores que no votan, votos con peso
  distinto).
- Audio, video y compartir pantalla dentro de la sesión.
- Reapertura de una sesión Finalizada o Cancelada, y edición de votos ya emitidos en rondas cerradas.
- Estimación en horas o en cualquier unidad distinta de los Story Points.
- Reportes y métricas calculados sobre el historial de estimación (desvío entre lo estimado y lo real,
  evolución de la capacidad de estimar del equipo), que se especifican en features propias.
- Exportación del historial de estimación a archivos o a otros sistemas.

---

## Supuestos

- **Cualquier integrante puede abrir la sesión**: se adopta el mismo criterio que el Product Backlog
  para crear y estimar historias (`specs/003-product-backlog`, RN-02), en lugar de reservarlo al
  propietario del proyecto, porque estimar es una actividad de todo el equipo.
- **El facilitador vota**: el enunciado no lo excluye y en la práctica del Planning Poker el
  facilitador es parte del equipo que estima. Su voto cuenta como cualquier otro.
- **El valor acordado es siempre numérico**: el enunciado dice que el facilitador elige "el valor final
  de la escala", así que "sin estimar" no es un resultado posible. Un equipo que no quiere fijar un
  número cancela la sesión.
- **Registrar exige una ronda revelada**: cerrar una sesión con un valor sin haber revelado nunca nada
  sería estimar por decreto y dejaría un historial vacío que no explica de dónde salió el número; para
  ese caso existe la cancelación.
- **La ronda nueva exige revelar primero**: descartar una ronda con votos emitidos sin mostrarlos
  desperdiciaría la información que la gente ya se comprometió a dar y dejaría un hueco inexplicable en
  el historial.
- **Umbral de desconexión de 30 segundos**: hace falta un criterio observable para decidir cuándo
  alguien dejó de contarse para la revelación automática; 30 segundos toleran un corte breve sin
  frenar al resto del equipo.
- **El relevo del facilitador es explícito**: pasados los 5 minutos, el rol queda disponible y alguien
  lo toma; no se asigna automáticamente, para que nadie reciba atribuciones que no pidió.
- **El facilitador relevado no recupera el rol**: si volviera a tomarlo automáticamente, la sesión
  podría quedar con dos personas creyéndose facilitador. El rol se mueve una sola vez por relevo.
- **Quien se une a una ronda en curso puede votar en ella**: es lo que espera alguien que llega tarde a
  una reunión, y se cuenta para la revelación automática desde que se incorpora.
- **Sesión cancelada automáticamente por la historia o el proyecto**: si la historia se completa o el
  proyecto se finaliza, seguir votando su estimación no tiene sentido y dejar la sesión abierta
  bloquearía la historia; se cancela conservando el historial.
- **Umbral de inactividad de 60 minutos**: decisión confirmada el 2026-09-26 (FR-063). La ventana es
  holgada a propósito: tiene que sobrevivir a una discusión larga entre dos rondas y, al mismo tiempo,
  destrabar la historia el mismo día en que el equipo abandonó la sesión. Cualquier hecho reinicia la
  cuenta, así que una sesión con gente trabajando nunca caduca.
- **La reestimación directa no se muestra en la sesión**: decisión confirmada el 2026-09-26 (FR-075).
  Mostrar el número que alguien escribió por afuera mientras los votos están ocultos sería exactamente
  el ancla que la feature existe para evitar.
- **Sin tope de rondas ni de participantes**: no se define un máximo que rechace rondas ni
  incorporaciones; una sesión de Planning Poker se acota por la dinámica del equipo, no por el producto.
  Lo que sí queda declarado es la escala hasta la que se compromete el tiempo real (RC-14).
- **Una persona en varias sesiones a la vez**: se permite, porque cada sesión lleva su propia presencia
  y sus propios votos, y no hay razón para bloquear a alguien que estima dos historias de dos proyectos.
- **Votos de una ronda anterior no se editan**: al iniciar una ronda nueva, los votos de la anterior
  quedan congelados; el historial es un registro, no un borrador.
- **Autoría conservada**: si una persona deja de ser integrante del proyecto, sus votos siguen
  figurando con su nombre en el historial, igual que el resto de sus registros en
  `specs/002-project-members` (RN-14).
- **Idioma de la interfaz**: los mensajes que ve la persona están en español.

---

## Preguntas Abiertas

Ninguna. Las dos decisiones que quedaron abiertas al redactar la especificación se resolvieron el
2026-09-26 y están registradas en la sección Clarifications: la caducidad de una sesión abandonada
(FR-063, RN-31) y el comportamiento de la reestimación directa mientras hay una sesión abierta
(FR-075, RN-32, RC-12).
