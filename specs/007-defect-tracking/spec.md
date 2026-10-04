# Especificación de Feature: Gestión de Defectos

**Directorio de feature**: `specs/007-defect-tracking`

**Rama**: `007-defect-tracking`

**Creada**: 2026-10-03

**Estado**: Clarificada — sin preguntas abiertas (sesión de clarificación del 2026-10-03)

**Entrada**: Descripción del usuario: "Gestión de defectos para Software Metrics & Estimation, un
sistema web multiusuario para estimar, planificar, seguir y medir proyectos de software con Scrum.
Depende de: gestión de proyectos e integrantes; Product Backlog; gestión de Sprints (sprints
Planificado / Activo / Cerrado)."

---

## Objetivo

Permitir que los integrantes registren los defectos que encuentran en el producto y les den
seguimiento hasta resolverlos, dejando asentado a qué historia afectan, en qué sprint se detectaron y
en cuál se resolvieron, para poder medir la calidad del proyecto.

El resto del producto mide cuánto trabajo se comprometió, cuánto se completó y cuánto esfuerzo costó.
Nada de eso dice si lo entregado funciona. Un sprint que cierra el 100 % de sus Story Points y deja
nueve defectos críticos detrás no es un buen sprint, pero sin registro de defectos el sistema lo
muestra como perfecto. Esta feature aporta la dimensión que falta: la cuenta de lo que salió mal.

Por eso cada defecto lleva dos anclas de sprint y no una. El sprint de detección dice cuándo apareció
el problema; el de resolución, cuándo se arregló. La distancia entre ambos es lo que convierte una
lista de defectos en información útil: permite ver si el equipo arregla dentro del mismo sprint o
arrastra deuda de calidad de un sprint al siguiente. Un único campo de "sprint" perdería esa
distinción y con ella la mitad de lo que se quiere medir.

---

## Clarifications

### Session 2026-10-03

- Q: ¿Se puede corregir el sprint de detección de un defecto ya registrado, del que dependen los conteos de defectos detectados por sprint? (FR-038) → A: sí, pero solo mientras el defecto esté en estado Abierto; apenas alguien lo pasa a En progreso queda congelado. Como un defecto Abierto nunca tiene sprint de resolución, la corrección no puede romper el orden entre detección y resolución.
- Q: ¿Cómo se descarta un defecto que nunca debió registrarse, por ejemplo un duplicado o algo que resultó no ser un defecto? (FR-039) → A: con un cuarto estado, Descartado, terminal, alcanzable desde Abierto y desde En progreso. No se elimina nada y las métricas pueden excluirlo del conteo de defectos detectados.
- Q: Si dos integrantes actúan a la vez sobre el mismo defecto, ¿se rechaza la segunda acción por haberse validado contra un estado que ya cambió? (FR-018, FR-039) → A: sí; cada transición se valida contra el estado vigente al aplicarla y la segunda se rechaza indicando el estado actual. Sin esa garantía, el carácter definitivo de Descartado se puede saltear con una carrera y una resolución se pierde sin rastro.
- Q: ¿La historia relacionada se congela igual que el sprint de detección, es decir, corregible solo mientras el defecto está Abierto? (FR-033, FR-038) → A: sí; los dos vínculos del defecto siguen la misma ventana. Son vínculos elegidos a mano de los que dependen conteos de calidad, y una única regla para ambos es más fácil de recordar y de probar que dos ventanas distintas.
- Q: ¿Los listados de defectos devuelven siempre el conjunto completo, sin paginar? (FR-040, FR-048) → A: sí; la paginación queda fuera de alcance, igual que en `specs/002-project-members` y `specs/003-product-backlog`. Los filtros son la herramienta para acotar.

---

## Entradas y Salidas Esperadas

### Registro de un defecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador del proyecto, descripción, severidad, historia relacionada y sprint de detección | **Éxito**: defecto creado en estado Abierto, sin sprint de resolución, atribuido a quien lo registró |
| | **Error de validación**: detalle de qué campo es inválido y por qué; no se crea nada |
| | **Sprint de detección Planificado**: rechazo indicando que solo se detecta en un sprint Activo o Cerrado |
| | **Historia o sprint de otro proyecto**: respuesta de historia o sprint inexistente |
| | **Proyecto Finalizado**: rechazo por proyecto de solo lectura |
| | **Quien pide no es integrante**: respuesta de proyecto inexistente |

### Cambio de estado de un defecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador del defecto y estado destino | **Éxito al pasar a En progreso**: el defecto queda En progreso; su sprint de detección no cambia |
| Sesión de un integrante, identificador del defecto, estado destino Resuelto y sprint de resolución | **Éxito al resolver**: el defecto queda Resuelto con su sprint de resolución registrado |
| | **Resolución sin sprint de resolución**: rechazo indicando que el campo es obligatorio para resolver |
| | **Sprint de resolución Planificado o anterior al de detección**: rechazo con el motivo; el defecto no cambia |
| | **Transición inválida**: rechazo indicando las transiciones permitidas |

### Reapertura de un defecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador de un defecto Resuelto | **Éxito**: el defecto vuelve a Abierto y su sprint de resolución queda vacío; el de detección no cambia |
| | **Defecto Abierto o En progreso**: rechazo indicando que solo se reabre un defecto Resuelto |
| | **Proyecto Finalizado**: rechazo por proyecto de solo lectura |

### Modificación de un defecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador del defecto y los datos a modificar (descripción, severidad, historia relacionada y, solo si el defecto está Abierto, sprint de detección) | **Éxito**: defecto actualizado; su estado, su sprint de resolución y su autor no cambian |
| | **Error de validación**: rechazo con el motivo; el defecto queda como estaba |
| | **Sprint de detección sobre un defecto que no está Abierto**: rechazo indicando que ya no se corrige |
| | **Defecto Descartado**: rechazo por defecto descartado, que es de solo lectura |
| | **Proyecto Finalizado**: rechazo por proyecto de solo lectura |

### Descarte de un defecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador de un defecto Abierto o En progreso | **Éxito**: el defecto queda Descartado, conserva su sprint de detección y sigue sin sprint de resolución |
| | **Defecto Resuelto o ya Descartado**: rechazo indicando desde qué estados se descarta |
| | **Proyecto Finalizado**: rechazo por proyecto de solo lectura |

### Consulta de defectos

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador del proyecto y filtros opcionales por estado (Abierto, En progreso, Resuelto o Descartado), severidad, historia y sprint | Lista de defectos que cumplen todos los filtros, en orden estable, con descripción, severidad, estado, historia, sprint de detección y sprint de resolución si lo tiene |
| Sesión de un integrante e identificador de una historia | Lista de los defectos de esa historia con los mismos datos |
| Historia o proyecto sin defectos | Lista vacía con la indicación de que no hay defectos registrados, sin que eso sea un error |
| Filtro con un valor inválido o de otro proyecto | Rechazo indicando qué filtro es inválido |

---

## Escenarios de Usuario y Pruebas *(obligatorio)*

### Historia de Usuario 1 (US1) — Registrar un defecto (Prioridad: P1)

Un integrante que encontró un problema en el producto lo deja asentado: qué pasa, qué tan grave es, a
qué historia afecta y en qué sprint apareció.

**Por qué esta prioridad**: es el único punto de entrada de los defectos al sistema. Sin registro no
hay nada que seguir, nada que resolver y nada que medir; todas las demás historias dependen de esta.

**Prueba independiente**: se puede probar completa registrando un defecto sobre una historia de un
proyecto con un sprint Activo y verificando que nace Abierto, sin sprint de resolución, con los datos
ingresados y con las validaciones aplicadas.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un integrante de un proyecto En curso con un sprint Activo y una historia
   del backlog, **cuando** registra un defecto con severidad Alta, la descripción "El total del
   reporte no suma los descuentos" y el sprint Activo como sprint de detección, **entonces** el
   defecto queda creado en estado Abierto, sin sprint de resolución y atribuido a quien lo registró.
2. *(Caso alternativo)* **Dado** un proyecto con un sprint Activo, **cuando** un integrante abre el
   formulario de registro, **entonces** el sprint Activo viene propuesto por defecto como sprint de
   detección, y el integrante puede cambiarlo por uno Cerrado.
3. *(Caso alternativo)* **Dado** una historia en estado Completada, **cuando** un integrante registra
   un defecto sobre ella, **entonces** el registro se acepta, porque un defecto suele aparecer
   justamente después de dar la historia por terminada.
4. *(Caso alternativo)* **Dado** un proyecto sin ningún sprint Activo pero con dos sprints Cerrados,
   **cuando** un integrante registra un defecto eligiendo uno de los Cerrados como sprint de
   detección, **entonces** el registro se acepta y no se propone ningún sprint por defecto.
5. *(Caso alternativo)* **Dado** una historia que ya tiene defectos registrados, **cuando** un
   integrante registra otro, **entonces** se acepta como defecto independiente, sin límite por
   historia.
6. *(Caso límite)* **Dado** un integrante, **cuando** registra un defecto con una descripción de
   exactamente 2000 caracteres, **entonces** el defecto se crea; y **cuando** tiene 2001, se rechaza.
7. *(Caso límite)* **Dado** un integrante, **cuando** registra un defecto con cada una de las cuatro
   severidades (Crítica, Alta, Media y Baja), **entonces** las cuatro se aceptan.
8. *(Caso límite)* **Dado** una historia con 50 defectos registrados, **cuando** un integrante
   registra el número 51, **entonces** se acepta y la consulta de la historia los devuelve todos.
9. *(Caso de error)* **Dado** un integrante, **cuando** intenta registrar un defecto sin descripción
   o con una descripción compuesta solo por espacios, **entonces** el registro se rechaza indicando
   que la descripción es obligatoria.
10. *(Caso de error)* **Dado** un integrante, **cuando** intenta registrar un defecto con una
    severidad que no es ninguna de las cuatro admitidas, **entonces** el registro se rechaza
    indicando las severidades válidas.
11. *(Caso de error)* **Dado** un integrante, **cuando** intenta registrar un defecto sin historia
    relacionada o sin sprint de detección, **entonces** el registro se rechaza indicando el campo
    faltante.
12. *(Caso de error)* **Dado** un sprint en estado Planificado, **cuando** un integrante intenta
    usarlo como sprint de detección, **entonces** el registro se rechaza indicando que un defecto
    solo se detecta en un sprint Activo o Cerrado.
13. *(Caso de error)* **Dado** una historia o un sprint que pertenece a otro proyecto, **cuando** un
    integrante intenta vincularlo al defecto, **entonces** el registro se rechaza con una respuesta
    de historia o sprint inexistente, sin revelar dato alguno del otro proyecto.
14. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
    registrar un defecto, **entonces** el registro se rechaza porque el proyecto es de solo lectura.
15. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
    registrar un defecto en él, **entonces** la respuesta es de proyecto inexistente y no se crea
    nada.
16. *(Caso de error)* **Dado** un proyecto que nunca inició ningún sprint, **cuando** un integrante
    intenta registrar un defecto, **entonces** el registro se rechaza indicando que no hay ningún
    sprint Activo ni Cerrado disponible como sprint de detección.

---

### Historia de Usuario 2 (US2) — Seguir y resolver un defecto (Prioridad: P1)

Un integrante toma un defecto abierto, lo marca como en progreso mientras lo trabaja y, cuando queda
arreglado, lo da por resuelto indicando en qué sprint se arregló.

**Por qué esta prioridad**: es lo que convierte el registro en seguimiento. Sin el ciclo de estados,
la lista de defectos solo crece y nunca se sabe cuáles siguen vivos ni cuándo se arreglaron, que es
el dato del que depende toda medición de calidad.

**Prueba independiente**: se puede probar completa creando un defecto Abierto, moviéndolo a En
progreso y después a Resuelto con su sprint de resolución, y verificando que las transiciones no
permitidas se rechazan.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un defecto Abierto, **cuando** un integrante lo pasa a En progreso y más
   tarde a Resuelto indicando el sprint Activo como sprint de resolución, **entonces** ambas
   transiciones se aceptan y el defecto queda Resuelto con ese sprint registrado.
2. *(Caso alternativo)* **Dado** un defecto detectado en el sprint Activo, **cuando** se resuelve en
   ese mismo sprint, **entonces** la resolución se acepta y el defecto queda con el mismo sprint en
   ambos campos, porque se detectó y se arregló dentro del mismo período.
3. *(Caso alternativo)* **Dado** un defecto detectado en un sprint ya Cerrado, **cuando** se resuelve
   en el sprint Activo posterior, **entonces** la resolución se acepta y los dos sprints quedan
   registrados por separado, dejando visible que el arreglo llevó más de un sprint.
4. *(Caso alternativo)* **Dado** un defecto, **cuando** un integrante distinto del que lo registró lo
   pasa a En progreso o lo resuelve, **entonces** la acción se acepta, porque cualquier integrante
   puede actualizar cualquier defecto del proyecto.
5. *(Caso límite)* **Dado** un defecto detectado en un sprint Cerrado, **cuando** se lo resuelve
   indicando otro sprint Cerrado posterior, **entonces** la resolución se acepta, porque el sprint de
   resolución puede ser Cerrado y no necesita ser el actual.
6. *(Caso límite)* **Dado** un defecto cuyo sprint de detección es el único sprint del proyecto,
   **cuando** se lo resuelve en ese mismo sprint, **entonces** la resolución se acepta.
7. *(Caso de error)* **Dado** un defecto En progreso, **cuando** un integrante intenta resolverlo sin
   indicar sprint de resolución, **entonces** la acción se rechaza indicando que el sprint de
   resolución es obligatorio para resolver, y el defecto sigue En progreso.
8. *(Caso de error)* **Dado** un defecto detectado en el segundo sprint del proyecto, **cuando** un
   integrante intenta resolverlo indicando el primer sprint, **entonces** la acción se rechaza
   indicando que el sprint de resolución no puede empezar antes que el de detección.
9. *(Caso de error)* **Dado** un sprint Planificado, **cuando** un integrante intenta usarlo como
   sprint de resolución, **entonces** la acción se rechaza indicando que solo se resuelve en un
   sprint Activo o Cerrado.
10. *(Caso de error)* **Dado** un defecto Abierto, **cuando** un integrante intenta pasarlo
    directamente a Resuelto, **entonces** la acción se rechaza indicando que primero debe pasar por
    En progreso.
11. *(Caso de error)* **Dado** un defecto En progreso, **cuando** un integrante intenta devolverlo a
    Abierto, **entonces** la acción se rechaza indicando las transiciones permitidas; para volver a
    Abierto hay que resolverlo y después reabrirlo.
12. *(Caso de error)* **Dado** un sprint de resolución de otro proyecto, **cuando** un integrante
    intenta usarlo, **entonces** la acción se rechaza con una respuesta de sprint inexistente.
13. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
    cambiar el estado de un defecto, **entonces** la acción se rechaza porque el proyecto es de solo
    lectura.

---

### Historia de Usuario 3 (US3) — Consultar y filtrar los defectos (Prioridad: P2)

Un integrante recorre los defectos del proyecto y los acota por estado, severidad, historia o sprint
para responder una pregunta concreta, o abre una historia para ver qué defectos tiene.

**Por qué esta prioridad**: es la lectura que hace utilizable todo lo registrado. El equipo puede
cargar y resolver defectos sin ella durante un tiempo muy corto, pero sin esta vista no hay forma de
saber qué queda pendiente ni de encontrar un defecto entre muchos.

**Prueba independiente**: se puede probar completa cargando defectos de distintas severidades,
estados, historias y sprints, y verificando que cada filtro y cada combinación devuelve exactamente
el subconjunto esperado.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto con defectos en los cuatro estados y de las cuatro
   severidades, **cuando** un integrante consulta los defectos del proyecto sin filtros,
   **entonces** ve todos con su descripción, severidad, estado, historia, sprint de detección y
   sprint de resolución cuando lo tengan.
2. *(Caso normal)* **Dado** una historia con cuatro defectos, **cuando** un integrante consulta esa
   historia, **entonces** ve sus cuatro defectos con los mismos datos.
3. *(Caso alternativo)* **Dado** el mismo proyecto, **cuando** un integrante filtra por estado
   Abierto, **entonces** ve únicamente los defectos Abiertos.
4. *(Caso alternativo)* **Dado** el mismo proyecto, **cuando** un integrante filtra por severidad
   Crítica y estado En progreso a la vez, **entonces** ve únicamente los defectos que cumplen ambas
   condiciones.
5. *(Caso alternativo)* **Dado** el mismo proyecto, **cuando** un integrante filtra por un sprint,
   **entonces** ve los defectos detectados en ese sprint y los resueltos en ese sprint, distinguibles
   entre sí por sus campos.
6. *(Caso límite)* **Dado** un proyecto sin ningún defecto, **cuando** un integrante los consulta,
   **entonces** obtiene una lista vacía con la indicación de que no hay defectos registrados, sin que
   eso sea un error.
7. *(Caso límite)* **Dado** una historia sin defectos, **cuando** un integrante la consulta,
   **entonces** obtiene una lista vacía, sin que eso sea un error.
8. *(Caso límite)* **Dado** una combinación de filtros que ningún defecto cumple, **cuando** un
   integrante la aplica, **entonces** obtiene una lista vacía con una indicación clara.
9. *(Caso límite)* **Dado** una historia con 50 defectos, **cuando** un integrante la consulta,
   **entonces** los ve todos en un orden estable y predecible.
10. *(Caso de error)* **Dado** un filtro por una historia o un sprint que pertenece a otro proyecto,
    **cuando** un integrante lo aplica, **entonces** la consulta se rechaza indicando que el filtro
    no corresponde al proyecto, sin revelar dato alguno del otro proyecto.
11. *(Caso de error)* **Dado** un filtro por un estado o una severidad que no existe, **cuando** un
    integrante lo aplica, **entonces** la consulta se rechaza indicando los valores admitidos.
12. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
    consultar sus defectos, **entonces** la respuesta es de proyecto inexistente.

---

### Historia de Usuario 4 (US4) — Corregir los datos de un defecto (Prioridad: P2)

Un integrante que describió mal un defecto, le puso la severidad equivocada o lo vinculó a la
historia que no era, corrige esos datos.

**Por qué esta prioridad**: un defecto mal descripto o mal clasificado ensucia la medición de calidad
y hace perder tiempo a quien lo va a arreglar. Va después de registrar, resolver y consultar porque
las tres funcionan sin ella.

**Prueba independiente**: se puede probar completa creando un defecto, cambiando cada campo editable
y verificando que el estado, los sprints y el autor no se alteran.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un defecto Abierto con severidad Media, **cuando** un integrante corrige
   su descripción y le sube la severidad a Crítica, **entonces** el defecto queda actualizado y su
   estado, sus sprints y su autor no cambian.
2. *(Caso alternativo)* **Dado** un defecto Abierto vinculado a la historia equivocada, **cuando**
   un integrante lo revincula a otra historia del mismo proyecto, **entonces** el cambio se acepta y
   el defecto deja de aparecer en la historia anterior.
3. *(Caso alternativo)* **Dado** un defecto ya Resuelto, **cuando** un integrante corrige su
   descripción, **entonces** el cambio se acepta, porque corregir el texto no altera el resultado del
   seguimiento.
4. *(Caso alternativo)* **Dado** un defecto En progreso, **cuando** un integrante le cambia la
   severidad, **entonces** el cambio se acepta y el defecto sigue En progreso.
5. *(Caso alternativo)* **Dado** un defecto Abierto registrado con el sprint de detección
   equivocado, **cuando** un integrante lo corrige por otro sprint Activo o Cerrado del proyecto,
   **entonces** el cambio se acepta, porque el defecto todavía no empezó a trabajarse.
6. *(Caso alternativo)* **Dado** un defecto Abierto, **cuando** un integrante le corrige la historia
   relacionada y el sprint de detección en la misma operación, **entonces** ambos cambios se
   aceptan, porque los dos vínculos comparten la misma ventana de corrección.
7. *(Caso límite)* **Dado** un defecto, **cuando** un integrante lo deja con exactamente la misma
   descripción y severidad que ya tenía, **entonces** el cambio se acepta sin efectos.
8. *(Caso límite)* **Dado** un defecto, **cuando** un integrante lleva su descripción a exactamente
   2000 caracteres, **entonces** el cambio se acepta.
9. *(Caso límite)* **Dado** un defecto que fue resuelto y después reabierto, **cuando** un integrante
   le corrige el sprint de detección, **entonces** el cambio se acepta, porque el defecto volvió a
   estar Abierto y la ventana de corrección se abre de nuevo.
10. *(Caso de error)* **Dado** un defecto, **cuando** un integrante intenta dejar la descripción vacía
    o poner una severidad inválida, **entonces** el cambio se rechaza con el motivo y el defecto queda
    exactamente como estaba.
11. *(Caso de error)* **Dado** un defecto Abierto, **cuando** un integrante intenta revincularlo a
    una historia de otro proyecto, **entonces** el cambio se rechaza con una respuesta de historia
    inexistente, aunque esté dentro de la ventana de corrección.
12. *(Caso de error)* **Dado** un defecto En progreso o Resuelto, **cuando** un integrante intenta
    corregirle el sprint de detección o la historia relacionada, **entonces** el cambio se rechaza
    indicando que los vínculos del defecto solo se corrigen mientras está Abierto.
13. *(Caso de error)* **Dado** un defecto Abierto, **cuando** un integrante intenta corregir su
    sprint de detección por uno Planificado o por uno de otro proyecto, **entonces** el cambio se
    rechaza con el mismo criterio que en el registro.
14. *(Caso de error)* **Dado** un defecto Descartado, **cuando** un integrante intenta modificar
    cualquiera de sus datos, **entonces** la acción se rechaza porque un defecto descartado es de
    solo lectura.
15. *(Caso de error)* **Dado** un defecto, **cuando** un integrante intenta cambiar su estado o su
    sprint de resolución desde la modificación de datos, **entonces** la acción se rechaza indicando
    que el estado se cambia con las transiciones de US2, US5 y US6.
16. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
    modificar un defecto, **entonces** la acción se rechaza porque el proyecto es de solo lectura.

---

### Historia de Usuario 5 (US5) — Reabrir un defecto resuelto (Prioridad: P3)

Un integrante que comprueba que un defecto dado por resuelto sigue ocurriendo lo devuelve al estado
Abierto para que vuelva a trabajarse.

**Por qué esta prioridad**: es la corrección de un caso que no es el habitual. El equipo puede operar
sin ella registrando un defecto nuevo, aunque eso duplicaría el registro y distorsionaría la cuenta
de defectos detectados.

**Prueba independiente**: se puede probar completa resolviendo un defecto, reabriéndolo y verificando
que vuelve a Abierto, que pierde el sprint de resolución y que conserva el de detección.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un defecto Resuelto con su sprint de resolución registrado, **cuando** un
   integrante lo reabre, **entonces** el defecto vuelve a Abierto, su sprint de resolución queda
   vacío y su sprint de detección no cambia.
2. *(Caso alternativo)* **Dado** un defecto reabierto, **cuando** un integrante lo vuelve a pasar por
   En progreso y lo resuelve en un sprint posterior, **entonces** la segunda resolución se acepta y
   el sprint de resolución queda con el sprint nuevo.
3. *(Caso alternativo)* **Dado** un defecto que se reabrió y se resolvió dos veces, **cuando** un
   integrante lo consulta, **entonces** ve su estado actual y el sprint de resolución vigente, que es
   siempre el de la última resolución.
4. *(Caso límite)* **Dado** un defecto detectado en el primer sprint y resuelto en el segundo,
   **cuando** se lo reabre y se lo resuelve de nuevo en el segundo sprint, **entonces** la resolución
   se acepta, porque la regla compara contra el sprint de detección, que no cambió.
5. *(Caso límite)* **Dado** un defecto Resuelto en un proyecto cuyo único sprint ya está Cerrado,
   **cuando** un integrante lo reabre, **entonces** la reapertura se acepta y el defecto queda
   Abierto, aunque no haya ningún sprint Activo.
6. *(Caso de error)* **Dado** un defecto Abierto o En progreso, **cuando** un integrante intenta
   reabrirlo, **entonces** la acción se rechaza indicando que solo se reabre un defecto Resuelto.
7. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
   reabrir un defecto, **entonces** la acción se rechaza porque el proyecto es de solo lectura.

---

### Historia de Usuario 6 (US6) — Descartar un defecto que no corresponde (Prioridad: P3)

Un integrante que detecta que un defecto registrado es un duplicado, o que lo reportado no era en
realidad un defecto, lo descarta para que deje de figurar como trabajo pendiente y no distorsione la
medición de calidad.

**Por qué esta prioridad**: sin esta historia, un duplicado queda vivo para siempre y engorda el
conteo de defectos detectados, que es justamente lo que la feature quiere medir bien. Va última
porque el equipo puede convivir un tiempo con defectos espurios en la lista.

**Prueba independiente**: se puede probar completa registrando un defecto, descartándolo y
verificando que queda en estado Descartado, que conserva su sprint de detección, que no admite más
cambios y que los filtros por estado lo distinguen del resto.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un defecto Abierto que resultó ser un duplicado, **cuando** un
   integrante lo descarta, **entonces** el defecto queda en estado Descartado, conserva su sprint de
   detección y sigue sin sprint de resolución.
2. *(Caso alternativo)* **Dado** un defecto En progreso que al investigarlo resultó no ser un
   defecto, **cuando** un integrante lo descarta, **entonces** el descarte se acepta, porque también
   se llega a Descartado desde En progreso.
3. *(Caso alternativo)* **Dado** un defecto Descartado, **cuando** un integrante consulta los
   defectos del proyecto sin filtros, **entonces** lo ve listado con su estado Descartado,
   distinguible de los demás.
4. *(Caso alternativo)* **Dado** un proyecto con defectos en los cuatro estados, **cuando** un
   integrante filtra por estado Descartado, **entonces** ve únicamente los descartados.
5. *(Caso límite)* **Dado** una historia cuyos únicos defectos fueron todos descartados, **cuando**
   un integrante intenta eliminar esa historia, **entonces** la eliminación se rechaza igual, porque
   la historia tuvo defectos asociados y eso no se revierte.
6. *(Caso límite)* **Dado** un defecto Descartado, **cuando** un integrante consulta su detalle,
   **entonces** ve su sprint de detección y el campo de sprint de resolución vacío, porque el
   defecto nunca se resolvió.
7. *(Caso de error)* **Dado** un defecto Resuelto, **cuando** un integrante intenta descartarlo,
   **entonces** la acción se rechaza indicando que solo se descarta un defecto Abierto o En
   progreso; para sacarlo de circulación hay que reabrirlo primero.
8. *(Caso de error)* **Dado** un defecto Descartado, **cuando** un integrante intenta descartarlo de
   nuevo, pasarlo a En progreso, resolverlo o reabrirlo, **entonces** la acción se rechaza porque
   Descartado es un estado definitivo.
9. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
   descartar un defecto, **entonces** la acción se rechaza porque el proyecto es de solo lectura.

---

### Casos Límite

- **Defecto detectado y resuelto en el mismo sprint**: válido; ambos campos quedan con el mismo
  sprint y la distancia entre detección y resolución es cero.
- **Defecto resuelto en un sprint posterior**: válido; los dos sprints quedan registrados por
  separado y dejan visible que el arreglo cruzó el límite del sprint.
- **Defecto que se reabre y se resuelve en otro sprint**: el sprint de resolución se reemplaza por el
  de la última resolución; el de detección nunca cambia.
- **Defecto reabierto y vuelto a resolver en el mismo sprint que la primera vez**: válido, porque la
  regla de orden compara contra el sprint de detección, no contra la resolución anterior.
- **Defecto registrado cuando ya no hay sprint Activo**: se elige un sprint Cerrado como sprint de
  detección y no se propone ninguno por defecto.
- **Proyecto que todavía no inició ningún sprint**: no se puede registrar ningún defecto, porque no
  existe ningún sprint Activo ni Cerrado que pueda ser sprint de detección.
- **Historia con muchos defectos**: no hay límite por historia; la consulta los devuelve todos en un
  orden estable.
- **Defecto sobre una historia Completada**: válido y esperable, porque un defecto suele aparecer
  después de dar la historia por terminada.
- **Defecto sobre una historia que vuelve al backlog al cerrarse su sprint**: el defecto sigue
  vinculado a la historia y conserva su sprint de detección original.
- **Historia con defectos que se intenta eliminar**: la eliminación se rechaza por la regla de
  `specs/003-product-backlog`; tener defectos vuelve a la historia no eliminable para siempre.
- **Defecto de un integrante que después fue quitado del proyecto**: sigue visible con el nombre de
  quien lo registró y cualquier integrante vigente puede seguir trabajándolo.
- **Severidad que cambia durante el seguimiento**: se admite en cualquier estado; el defecto conserva
  la severidad vigente, sin historial de severidades anteriores.
- **Defecto Resuelto en un proyecto que después se finaliza**: queda congelado en su estado; el
  proyecto Finalizado no admite reaperturas ni cambios.
- **Sprint de detección que era Activo y después se cierra**: el defecto no cambia; el sprint de
  detección sigue siendo el mismo.
- **Corrección del sprint de detección de un defecto reabierto**: al volver a Abierto, la ventana de
  corrección se abre de nuevo, porque la regla mira el estado actual y no si el defecto fue
  trabajado alguna vez. Es el efecto deliberado de atar la ventana al estado Abierto.
- **Defecto Abierto cuyo sprint de detección se corrige**: nunca puede quedar en conflicto con el
  sprint de resolución, porque un defecto Abierto no tiene sprint de resolución: o nunca se resolvió
  o la reapertura se lo borró.
- **Defecto descartado desde En progreso**: queda Descartado sin haber pasado nunca por Resuelto y
  sin sprint de resolución; no cuenta ni como resuelto ni como pendiente.
- **Historia cuyos defectos fueron todos descartados**: sigue sin poder eliminarse, porque haber
  tenido defectos no se revierte.
- **Defecto descartado por error**: no tiene vuelta atrás; Descartado es definitivo y la única salida
  es registrar el defecto de nuevo, que nacerá Abierto y con su propio sprint de detección.
- **Dos integrantes que actúan a la vez sobre el mismo defecto En progreso**: si uno lo resuelve y
  el otro lo descarta, se aplica una sola acción y la otra se rechaza indicando el estado vigente;
  el defecto nunca queda Resuelto después de haber sido descartado.
- **Modificación que llega junto con un descarte**: si el descarte se aplica primero, la
  modificación se rechaza porque un defecto Descartado es de solo lectura.

---

## Requisitos *(obligatorio)*

### Requisitos Funcionales

**Registro del defecto**

- **FR-001**: El sistema DEBE permitir a cualquier integrante vigente de un proyecto registrar un
  defecto indicando descripción, severidad, historia relacionada y sprint de detección.
- **FR-002**: El sistema DEBE exigir una descripción de entre 1 y 2000 caracteres, contados después
  de recortar los espacios al inicio y al final; una descripción ausente o compuesta solo por
  espacios se rechaza.
- **FR-003**: El sistema DEBE admitir como severidad únicamente los valores Crítica, Alta, Media y
  Baja, y rechazar cualquier otro indicando los valores admitidos.
- **FR-004**: El sistema DEBE exigir una historia relacionada, que DEBE pertenecer al mismo proyecto
  que el defecto.
- **FR-005**: El sistema DEBE permitir registrar un defecto sobre una historia en cualquier estado,
  incluida Completada.
- **FR-006**: El sistema DEBE exigir un sprint de detección, que DEBE pertenecer al mismo proyecto y
  DEBE estar en estado Activo o Cerrado.
- **FR-007**: El sistema DEBE rechazar un sprint de detección en estado Planificado, indicando que un
  defecto solo se detecta en un sprint que ya arrancó.
- **FR-008**: Cuando el proyecto tiene un sprint Activo, el sistema DEBE proponerlo por defecto como
  sprint de detección, dejando que el integrante lo cambie por cualquier sprint Cerrado del proyecto.
- **FR-009**: Cuando el proyecto no tiene ningún sprint Activo, el sistema NO DEBE proponer ningún
  sprint por defecto y DEBE exigir que el integrante elija uno Cerrado.
- **FR-010**: Cuando el proyecto no tiene ningún sprint Activo ni Cerrado, el sistema DEBE rechazar
  el registro indicando que todavía no hay ningún sprint que pueda ser sprint de detección.
- **FR-011**: Todo defecto DEBE nacer en estado Abierto y sin sprint de resolución.
- **FR-012**: Cada defecto DEBE tener un identificador propio, estable, inmutable y no secuencial.
- **FR-013**: Cada defecto DEBE pertenecer a exactamente un proyecto y NO DEBE poder moverse a otro.
- **FR-014**: El sistema DEBE registrar en cada defecto quién lo creó, su momento de creación y su
  momento de última actualización.
- **FR-015**: El sistema DEBE permitir registrar cualquier cantidad de defectos sobre una misma
  historia, sin límite.
- **FR-016**: El sistema DEBE rechazar un registro inválido indicando qué campo corregir y por qué,
  sin crear el defecto; la validación DEBE ser atómica.

**Estados y transiciones**

- **FR-017**: El sistema DEBE admitir para un defecto exactamente cuatro estados: Abierto, En
  progreso, Resuelto y Descartado.
- **FR-018**: El sistema DEBE admitir únicamente las transiciones Abierto → En progreso, En progreso
  → Resuelto, Resuelto → Abierto, Abierto → Descartado y En progreso → Descartado, y rechazar
  cualquier otra indicando las permitidas. El sistema DEBE validar cada transición contra el estado
  vigente del defecto en el momento de aplicarla, no contra el estado leído antes de pedirla. Cuando
  dos acciones concurrentes sobre el mismo defecto son incompatibles entre sí, el sistema DEBE
  aplicar una sola y rechazar la otra indicando el estado actual del defecto. La misma garantía rige
  para una modificación de datos que llegue a la vez que un descarte: si el descarte se aplica
  primero, la modificación se rechaza por FR-036.
- **FR-019**: El sistema DEBE rechazar el salto directo de Abierto a Resuelto, indicando que primero
  debe pasar por En progreso.
- **FR-020**: El sistema DEBE rechazar la vuelta de En progreso a Abierto; para devolver un defecto a
  Abierto hay que resolverlo y reabrirlo.
- **FR-021**: El sistema DEBE permitir a cualquier integrante vigente cambiar el estado de cualquier
  defecto del proyecto, sin importar quién lo registró.
- **FR-022**: El sistema DEBE registrar el momento de cada cambio de estado y a quién se atribuye.

**Resolución**

- **FR-023**: El sistema DEBE exigir un sprint de resolución al pasar un defecto a Resuelto, y
  rechazar la transición si no se indica.
- **FR-024**: El sprint de resolución DEBE pertenecer al mismo proyecto que el defecto y DEBE estar
  en estado Activo o Cerrado; un sprint Planificado se rechaza.
- **FR-025**: El sistema DEBE rechazar un sprint de resolución cuya fecha de inicio sea anterior a la
  del sprint de detección. El mismo sprint en ambos campos es válido.
- **FR-026**: El sistema DEBE permitir que el sprint de resolución sea un sprint Cerrado, no
  necesariamente el sprint vigente.
- **FR-027**: El sprint de detección de un defecto NO DEBE cambiar al resolverlo, al reabrirlo ni al
  modificar sus datos.

**Reapertura**

- **FR-028**: El sistema DEBE permitir a cualquier integrante vigente reabrir un defecto en estado
  Resuelto, que vuelve a Abierto.
- **FR-029**: Al reabrir un defecto, el sistema DEBE borrar su sprint de resolución y DEBE conservar
  su sprint de detección.
- **FR-030**: El sistema DEBE rechazar la reapertura de un defecto en estado Abierto o En progreso,
  indicando que solo se reabre un defecto Resuelto.
- **FR-031**: El sistema DEBE permitir reabrir y volver a resolver un defecto cuantas veces haga
  falta; el sprint de resolución vigente DEBE ser siempre el de la última resolución.
- **FR-032**: El sistema DEBE permitir reabrir un defecto aunque el proyecto no tenga ningún sprint
  Activo en ese momento.

**Modificación de datos**

- **FR-033**: El sistema DEBE permitir a cualquier integrante vigente modificar la descripción y la
  severidad de un defecto en cualquier estado salvo Descartado, y sus dos vínculos —la historia
  relacionada y el sprint de detección— únicamente dentro de la ventana de corrección de FR-038.
- **FR-034**: El sistema DEBE aplicar en la modificación exactamente las mismas validaciones de
  descripción, severidad y pertenencia de la historia al proyecto que en la creación (FR-002 a
  FR-004).
- **FR-035**: La modificación de datos NO DEBE alterar el estado del defecto, su sprint de
  resolución ni su autor; el estado se cambia únicamente con las transiciones de FR-018.
- **FR-036**: El sistema DEBE permitir modificar un defecto en estado Abierto, En progreso o
  Resuelto, y DEBE rechazar toda modificación de un defecto Descartado.
- **FR-037**: La modificación DEBE ser atómica: si algún dato es inválido, ningún cambio se aplica y
  el defecto queda exactamente como estaba.
- **FR-038**: El sistema DEBE permitir corregir los dos vínculos de un defecto —su historia
  relacionada y su sprint de detección— únicamente mientras esté en estado Abierto. Esa es la
  ventana de corrección, y es la misma para ambos. Las correcciones DEBEN aplicar las mismas
  validaciones que en el registro: la historia debe pertenecer al mismo proyecto (FR-004) y el
  sprint debe pertenecer al mismo proyecto y estar Activo o Cerrado, nunca Planificado (FR-006,
  FR-007). El sistema DEBE rechazar la corrección de cualquiera de los dos vínculos sobre un defecto
  En progreso, Resuelto o Descartado, indicando que ya quedaron fijos. Como un defecto Abierto nunca
  tiene sprint de resolución, corregir el sprint de detección no puede dejar los dos sprints en un
  orden inválido.
- **FR-039**: El sistema DEBE permitir a cualquier integrante vigente descartar un defecto en estado
  Abierto o En progreso, que pasa a Descartado. El descarte DEBE conservar el sprint de detección y
  DEBE dejar el defecto sin sprint de resolución, porque nunca se resolvió. El estado Descartado
  DEBE ser definitivo: el sistema rechaza volver a descartarlo, pasarlo a En progreso, resolverlo y
  reabrirlo. El sistema DEBE rechazar el descarte de un defecto Resuelto, indicando que primero hay
  que reabrirlo.

**Consulta de defectos**

- **FR-040**: El sistema DEBE permitir a cualquier integrante listar los defectos de un proyecto con
  filtros opcionales y combinables por estado, por severidad, por historia y por sprint. El filtro
  por estado DEBE admitir los cuatro estados, incluido Descartado. Los listados sin filtro de estado
  DEBEN incluir los defectos Descartados, con su estado visible, para que no desaparezcan sin dejar
  rastro; quien quiera excluirlos filtra por los otros tres estados. El listado DEBE devolver el
  conjunto completo de defectos que cumplen los filtros, sin paginación y sin tope de resultados.
- **FR-041**: El sistema DEBE combinar los filtros aplicados de forma conjuntiva: un defecto aparece
  solo si cumple todos los filtros indicados.
- **FR-042**: El filtro por sprint DEBE devolver tanto los defectos detectados en ese sprint como los
  resueltos en él, y el resultado DEBE permitir distinguir unos de otros.
- **FR-043**: El sistema DEBE permitir consultar los defectos de una historia determinada.
- **FR-044**: Cada defecto listado DEBE mostrar al menos su descripción, severidad, estado, historia
  relacionada, sprint de detección y, cuando exista, sprint de resolución.
- **FR-045**: Cuando ningún defecto cumple los filtros, o cuando el proyecto o la historia no tienen
  defectos, el sistema DEBE devolver una lista vacía con una indicación clara, no un error.
- **FR-046**: El sistema DEBE rechazar un filtro por un estado o una severidad inexistente,
  indicando los valores admitidos.
- **FR-047**: El sistema DEBE rechazar un filtro por una historia o un sprint que no pertenece al
  proyecto consultado, sin revelar ningún dato del proyecto ajeno.
- **FR-048**: Los listados DEBEN presentarse en un orden estable y predecible: severidad descendente
  (Crítica, Alta, Media, Baja) y, a igual severidad, por momento de creación descendente.
- **FR-049**: El sistema DEBE permitir consultar el detalle de un defecto con todos sus datos, su
  estado y sus dos sprints.

**Autorización y visibilidad**

- **FR-050**: El sistema DEBE exigir una sesión válida para toda acción de esta feature, sin
  excepciones.
- **FR-051**: El sistema DEBE restringir toda acción sobre los defectos de un proyecto a sus
  integrantes vigentes.
- **FR-052**: El sistema DEBE permitir a cualquier integrante vigente registrar, modificar, cambiar
  de estado y reabrir cualquier defecto del proyecto; no hay propiedad individual sobre un defecto.
- **FR-053**: Ante una solicitud sobre un proyecto, una historia, un sprint o un defecto a los que
  quien pide no tiene acceso, el sistema DEBE responder exactamente igual que ante un identificador
  inexistente, sin revelar ningún dato ni la existencia del recurso.
- **FR-054**: Cuando el proyecto está en estado Finalizado, el sistema DEBE tratar sus defectos como
  de solo lectura: rechaza registrar, modificar, cambiar de estado, reabrir y descartar, y permite
  consultar.
- **FR-055**: El sistema DEBE conservar los defectos registrados por una persona que después fue
  quitada del proyecto, mostrando su nombre; cualquier integrante vigente puede seguir trabajándolos.
- **FR-056**: Toda creación, modificación, cambio de estado y reapertura DEBE quedar atribuida al
  titular de la sesión que la ejecutó.

---

### Reglas de Negocio

| ID | Regla |
| --- | --- |
| RN-01 | Cualquier integrante vigente puede registrar y actualizar cualquier defecto del proyecto; no hay propiedad individual. |
| RN-02 | En un proyecto Finalizado los defectos son de solo lectura. |
| RN-03 | La descripción es obligatoria y mide hasta 2000 caracteres. |
| RN-04 | Las severidades admitidas son exactamente Crítica, Alta, Media y Baja. |
| RN-05 | Todo defecto nace en estado Abierto y sin sprint de resolución. |
| RN-06 | La historia relacionada es obligatoria y pertenece al mismo proyecto que el defecto. |
| RN-07 | Un defecto puede registrarse sobre una historia en cualquier estado, incluida Completada. |
| RN-08 | El sprint de detección es obligatorio, pertenece al mismo proyecto y está Activo o Cerrado, nunca Planificado. |
| RN-09 | Si hay un sprint Activo, se propone por defecto como sprint de detección; si no lo hay, no se propone ninguno. |
| RN-10 | Sin ningún sprint Activo ni Cerrado en el proyecto, no se puede registrar un defecto. |
| RN-11 | Los estados del defecto son Abierto, En progreso, Resuelto y Descartado. |
| RN-12 | Las únicas transiciones admitidas son Abierto → En progreso, En progreso → Resuelto, Resuelto → Abierto, Abierto → Descartado y En progreso → Descartado. |
| RN-13 | El sprint de resolución es obligatorio al pasar a Resuelto, pertenece al mismo proyecto y está Activo o Cerrado. |
| RN-14 | El sprint de resolución no puede empezar antes que el de detección; el mismo sprint en ambos campos es válido. |
| RN-15 | Reabrir un defecto lo devuelve a Abierto y borra su sprint de resolución. |
| RN-16 | Los dos vínculos del defecto —historia relacionada y sprint de detección— solo se corrigen a mano mientras está Abierto, y ninguna transición de estado los cambia. |
| RN-17 | Un defecto puede reabrirse y resolverse cuantas veces haga falta; vale siempre la última resolución. |
| RN-18 | La modificación de datos no cambia el estado ni los sprints del defecto. |
| RN-19 | Una historia con defectos registrados no puede eliminarse, aunque todos sus defectos estén Descartados. |
| RN-20 | Los defectos de una persona quitada del proyecto se conservan y siguen mostrando su nombre. |
| RN-21 | Un defecto Abierto o En progreso puede descartarse; uno Resuelto, no, y uno Descartado no vuelve atrás. |
| RN-22 | Un defecto Descartado es de solo lectura: no se modifica, no cambia de estado y sigue visible en los listados. |
| RN-23 | Un defecto Descartado nunca tiene sprint de resolución, porque no se lo resolvió. |
| RN-24 | Dos acciones concurrentes e incompatibles sobre el mismo defecto nunca se aplican las dos: la segunda se rechaza contra el estado vigente. |

---

### Restricciones

- **RC-01**: Esta feature depende de la feature de autenticación y cuentas de usuario
  (`specs/001-user-auth`): solo usuarios registrados con sesión válida operan sobre los defectos.
- **RC-02**: Esta feature depende de la feature de gestión de proyectos e integrantes
  (`specs/002-project-members`): el defecto existe dentro de un proyecto, el permiso de acceso es la
  membresía vigente y el estado Finalizado vuelve los defectos de solo lectura.
- **RC-03**: `specs/002-project-members` (FR-027) establece que quitar a un integrante conserva los
  defectos que registró y sigue mostrando su nombre. Esta feature lo cumple en FR-055.
- **RC-04**: Esta feature depende de la feature de Product Backlog (`specs/003-product-backlog`): las
  historias y sus estados se definen allí. Esta feature las consume y no modifica ninguna historia.
- **RC-05**: `specs/003-product-backlog` (FR-043, RN-16) impide eliminar una historia que tiene
  defectos asociados. Esa condición queda satisfecha para siempre desde el primer defecto registrado
  sobre la historia, y descartar todos sus defectos no la revierte: un defecto Descartado sigue
  siendo un defecto asociado.
- **RC-06**: Esta feature depende de la feature de gestión de Sprints
  (`specs/004-sprint-management`): los sprints, sus estados y sus fechas se definen allí. Esta
  feature los consume como anclas de detección y resolución; no crea, no modifica y no cierra
  sprints, y el estado de un defecto no condiciona el cierre de ningún sprint.
- **RC-07**: El orden entre dos sprints, que FR-025 necesita para comparar detección y resolución, se
  establece por su fecha de inicio prevista. `specs/004-sprint-management` (RN-05) garantiza que los
  períodos de dos sprints del mismo proyecto no se superponen, así que esas fechas definen un orden
  total sin ambigüedad.
- **RC-08**: Esta feature no condiciona la eliminación de un sprint. `specs/004-sprint-management`
  (FR-048) solo permite eliminar un sprint Planificado que nunca fue iniciado, y un sprint
  Planificado no puede ser sprint de detección ni de resolución (FR-007, FR-024), así que ningún
  defecto puede quedar apuntando a un sprint eliminable.
- **RC-09**: El conteo de defectos detectados y resueltos por sprint, la densidad de defectos por
  Story Point y cualquier otro indicador agregado de calidad corresponden a la feature de métricas.
  Esta feature produce los defectos con sus dos anclas de sprint que esos cálculos consumen, pero no
  realiza las agregaciones. La feature de métricas DEBE excluir los defectos Descartados de todo
  conteo de defectos detectados y resueltos: esa exclusión es la razón de ser del estado, que existe
  para que un duplicado deje de contar sin borrar el registro.
- **RC-10**: La severidad no tiene historial: el defecto conserva solo la severidad vigente. Un
  cambio de severidad no deja rastro consultable más allá del momento de última actualización.

---

### Condiciones de Error

| Condición | Comportamiento esperado |
| --- | --- |
| Acción de la feature sin sesión válida | Rechazo con indicación de iniciar sesión; la acción no se ejecuta. |
| Descripción ausente o compuesta solo por espacios | Rechazo indicando que la descripción es obligatoria. |
| Descripción de más de 2000 caracteres | Rechazo indicando el máximo permitido. |
| Severidad ausente o fuera de las cuatro admitidas | Rechazo indicando las severidades válidas. |
| Historia relacionada ausente | Rechazo indicando que el campo es obligatorio. |
| Historia de otro proyecto | Respuesta de historia inexistente, sin revelar dato alguno del otro proyecto. |
| Sprint de detección ausente | Rechazo indicando que el campo es obligatorio. |
| Sprint de detección en estado Planificado | Rechazo indicando que un defecto solo se detecta en un sprint Activo o Cerrado. |
| Sprint de detección de otro proyecto | Respuesta de sprint inexistente. |
| Registro en un proyecto sin ningún sprint Activo ni Cerrado | Rechazo indicando que todavía no hay sprint que pueda ser sprint de detección. |
| Resolución sin sprint de resolución | Rechazo indicando que el sprint de resolución es obligatorio para resolver; el defecto no cambia. |
| Sprint de resolución en estado Planificado | Rechazo indicando que solo se resuelve en un sprint Activo o Cerrado. |
| Sprint de resolución anterior al de detección | Rechazo indicando que el arreglo no puede ser anterior a la detección; el defecto no cambia. |
| Salto directo de Abierto a Resuelto | Rechazo indicando que primero debe pasar por En progreso. |
| Vuelta de En progreso a Abierto | Rechazo indicando las transiciones permitidas. |
| Reapertura de un defecto Abierto o En progreso | Rechazo indicando que solo se reabre un defecto Resuelto. |
| Descarte de un defecto Resuelto | Rechazo indicando que solo se descarta un defecto Abierto o En progreso, y que primero hay que reabrirlo. |
| Cualquier cambio de estado o de datos sobre un defecto Descartado | Rechazo indicando que Descartado es un estado definitivo y de solo lectura. |
| Transición válida contra el estado leído pero inválida contra el estado vigente, por una acción concurrente de otro integrante | Rechazo indicando el estado actual del defecto; solo una de las dos acciones se aplica. |
| Corrección de la historia relacionada o del sprint de detección sobre un defecto que no está Abierto | Rechazo indicando que los vínculos del defecto solo se corrigen mientras está Abierto. |
| Corrección del sprint de detección por uno Planificado o de otro proyecto | Rechazo con el mismo criterio que en el registro; el defecto no cambia. |
| Intento de cambiar el estado o los sprints desde la modificación de datos | Rechazo indicando que el estado se cambia con las transiciones correspondientes. |
| Cualquier acción de escritura sobre los defectos de un proyecto Finalizado | Rechazo por proyecto de solo lectura. |
| Filtro por un estado o una severidad inexistente | Rechazo indicando los valores admitidos. |
| Filtro por una historia o un sprint de otro proyecto | Rechazo indicando que el filtro no corresponde al proyecto. |
| Solicitud sobre un proyecto, una historia, un sprint o un defecto de los que quien pide no es integrante | Respuesta idéntica a la de un identificador inexistente, sin revelar ningún dato. |
| Solicitud sobre un identificador de defecto que no existe | Respuesta de defecto inexistente. |
| Consulta de un proyecto o una historia sin defectos | No es un error: lista vacía con indicación clara. |

---

### Entidades Clave

- **Defecto**: representa un problema detectado en el producto. Atributos relevantes: identificador
  propio estable y no secuencial, proyecto al que pertenece, historia relacionada, descripción,
  severidad, estado, sprint de detección, sprint de resolución cuando existe, autor, momento de
  creación y momento de última actualización.
- **Estado del defecto**: representa el punto del ciclo de vida en que está. Valores posibles:
  Abierto, En progreso, Resuelto y Descartado. A diferencia del estado de un sprint, admite volver
  atrás: un defecto Resuelto puede reabrirse y recorrer el ciclo de nuevo. Descartado es la única
  salida definitiva, y se alcanza desde Abierto o desde En progreso.
- **Severidad**: conjunto cerrado de valores que expresa la gravedad del defecto: Crítica, Alta,
  Media y Baja. Ordena los listados y no tiene historial.
- **Sprint de detección**: ancla que indica en qué sprint apareció el defecto. Es obligatoria y es la
  base del conteo de defectos detectados por sprint. Solo se corrige a mano mientras el defecto está
  Abierto; ninguna otra acción la cambia.
- **Sprint de resolución**: ancla que indica en qué sprint se arregló el defecto. Está vacía mientras
  el defecto no esté Resuelto, se completa al resolver, se borra al reabrir y se vuelve a completar
  en la siguiente resolución.
- **Historia de usuario** *(entidad de `specs/003-product-backlog`)*: esta feature la consume para
  vincular el defecto; no la modifica, pero tener defectos la vuelve no eliminable.
- **Sprint** y **Estado del sprint** *(entidades de `specs/004-sprint-management`)*: esta feature los
  consume como anclas y para validar que no estén Planificados; no los modifica.
- **Proyecto** e **Integrante del proyecto** *(entidades de `specs/002-project-members`)*: esta
  feature los consume para ubicar el defecto y decidir quién accede.
- **Usuario** *(entidad de `specs/001-user-auth`)*: esta feature lo consume para atribuir la autoría
  de cada acción; no lo modifica.

---

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **SC-001**: Un integrante registra un defecto en menos de 1 minuto y en no más de 5 pasos, y el
  95 % lo logra en el primer intento sin ayuda externa.
- **SC-002**: Un integrante encuentra todos los defectos Críticos abiertos de un proyecto en menos de
  30 segundos usando los filtros disponibles.
- **SC-003**: Un integrante ve los defectos de una historia desde la propia historia, sin combinar
  información de otros lugares del sistema.
- **SC-004**: El 100 % de los defectos nace en estado Abierto y sin sprint de resolución, verificado
  sobre registros con las cuatro severidades.
- **SC-005**: El 100 % de las transiciones de estado no permitidas se rechaza, verificado probando
  las once combinaciones de estado origen y destino distintas de las cinco válidas, incluidas las
  cuatro salidas desde Descartado.
- **SC-006**: Cero defectos Resueltos sin sprint de resolución, verificado intentando resolver sin
  indicarlo y comprobando que el defecto sigue En progreso.
- **SC-007**: Cero defectos con un sprint de resolución anterior al de detección, verificado
  intentando resolver contra un sprint previo y contra el propio sprint de detección: el primero se
  rechaza y el segundo se acepta.
- **SC-008**: Cero defectos anclados a un sprint Planificado, verificado intentando usarlo como
  sprint de detección y como sprint de resolución.
- **SC-009**: El 100 % de las reaperturas deja el defecto Abierto y sin sprint de resolución, y
  conserva el sprint de detección, verificado comparando los dos campos antes y después.
- **SC-010**: Ni la historia relacionada ni el sprint de detección cambian por efecto de ninguna
  transición de estado, verificado recorriendo el ciclo completo de resolución, reapertura, segunda
  resolución y descarte, y comparando ambos vínculos al principio y al final.
- **SC-011**: La corrección de cada uno de los dos vínculos —historia relacionada y sprint de
  detección— se acepta en el 100 % de los defectos Abiertos y se rechaza en el 100 % de los que
  están En progreso, Resueltos o Descartados, verificado con una prueba por vínculo y por estado.
- **SC-012**: Cero defectos Descartados con sprint de resolución, y cero defectos Descartados que
  admitan una modificación o un cambio de estado posterior, verificado intentando las cuatro
  acciones sobre uno descartado.
- **SC-013**: El 100 % de las combinaciones de filtros devuelve exactamente el subconjunto esperado,
  verificado con una prueba por filtro individual y al menos tres combinaciones de dos o más filtros.
- **SC-014**: Una historia con 50 defectos los devuelve todos, en el mismo orden ante consultas
  repetidas con los mismos filtros.
- **SC-015**: El 100 % de las acciones de escritura sobre los defectos de un proyecto Finalizado se
  rechaza, sin producir ningún cambio.
- **SC-016**: El 100 % de las acciones de esta feature exige sesión válida y membresía vigente en el
  proyecto, verificado con una prueba por acción.
- **SC-017**: Cero filtraciones de datos entre proyectos: en el 100 % de los intentos de acceso o de
  vinculación con historias o sprints ajenos, la respuesta es indistinguible de la de un
  identificador inexistente.
- **SC-018**: Cero defectos perdidos o sin autor identificable después de quitar a un integrante del
  proyecto, verificado contando los defectos de sus historias antes y después de la baja.
- **SC-019**: Cero defectos que terminen en un estado inalcanzable desde su estado previo real,
  verificado enviando de forma simultánea una resolución y un descarte sobre el mismo defecto En
  progreso: exactamente una de las dos se aplica y la otra se rechaza, en 10 intentos de 10.
- **SC-020**: El 100 % de las reglas de negocio (RN-01 a RN-24) tiene al menos una prueba
  automatizada asociada que falla si la regla se rompe.

---

## Fuera de Alcance

- Asignación de responsables: un defecto no se asigna a una persona; cualquier integrante puede
  trabajarlo.
- Archivos adjuntos, capturas de pantalla y pasos de reproducción estructurados: la descripción es
  texto libre.
- Comentarios o hilos de discusión sobre un defecto.
- Prioridad como campo distinto de la severidad: hay un único eje de clasificación.
- Notificaciones o recordatorios por defectos nuevos, reabiertos o sin resolver.
- Conteos de defectos detectados y resueltos por sprint o por proyecto, densidad de defectos y
  cualquier otro indicador agregado de calidad, que corresponden a la feature de métricas. Esta
  feature produce los defectos con sus dos anclas de sprint que esos cálculos consumen.
- Gráficos de cualquier tipo.
- Búsqueda por texto y paginación de los listados de defectos, en línea con
  `specs/002-project-members` y `specs/003-product-backlog`, que también las dejan fuera. Los
  filtros de FR-040 son la única herramienta para acotar un listado.
- Vinculación de un defecto con más de una historia, o con un sprint sin historia.
- Relaciones entre defectos (duplicados, bloqueos, dependencias).
- Historial de auditoría consultable de los cambios de un defecto, más allá de su momento de última
  actualización y de la atribución de cada acción.
- Registro de esfuerzo en horas sobre los defectos, que `specs/006-effort-tracking` define
  únicamente sobre historias de usuario.

---

## Supuestos

- **Los defectos no tienen dueño**: el enunciado dice que cualquier integrante puede registrar y
  actualizar defectos, sin reservar ninguna acción al autor. A diferencia del registro de esfuerzo,
  donde cada persona solo toca lo suyo, acá la escritura es abierta dentro del proyecto, porque un
  defecto es un problema del equipo y no un testimonio personal.
- **Los campos editables son descripción y severidad siempre, y los dos vínculos solo con el defecto
  Abierto**: el enunciado dice "modificar un defecto" sin precisar qué. Se excluyen el estado y el
  sprint de resolución, que se cambian con las transiciones.
- **Una sola ventana de corrección para los dos vínculos**: decisión confirmada el 2026-10-03
  (FR-033, FR-038, RN-16). La historia relacionada y el sprint de detección son vínculos elegidos a
  mano, fáciles de errar al cargar, y de los dos dependen conteos de calidad: defectos por historia
  y defectos detectados por sprint. Por eso comparten regla en vez de tener cada uno la suya: una
  única frase —"los vínculos se corrigen mientras el defecto está Abierto"— es más fácil de
  recordar, de probar y de explicar en una revisión que dos ventanas distintas.
- **La ventana se ata al estado Abierto, no a si el defecto fue trabajado alguna vez**: un defecto
  reabierto vuelve a estar Abierto y por lo tanto vuelve a admitir la corrección de ambos vínculos.
  Es un efecto deliberado y conocido de la regla elegida.
- **Un vínculo mal cargado que ya pasó a En progreso no se arregla**: hay que descartar el defecto y
  registrarlo de nuevo. Es el costo aceptado de congelar temprano.
- **Descartado es definitivo y de solo lectura**: decisión confirmada el 2026-10-03 (FR-039). Un
  defecto descartado por error no se recupera; la salida es registrarlo de nuevo. Se eligió así
  porque un estado terminal sin vuelta atrás es la pieza más simple que resuelve el problema del
  duplicado, y el costo de equivocarse es bajo: volver a cargar un defecto es barato.
- **Descartar no pide motivo**: el enunciado deja los comentarios fuera de alcance, así que el
  descarte no lleva texto explicativo. Si más adelante hiciera falta saber por qué se descartó algo,
  haría falta una enmienda.
- **El orden entre sprints se mide por fecha de inicio prevista**: es la única fecha que todo sprint
  tiene desde que se crea, mientras que el momento real de inicio solo existe desde que arranca. Como
  los períodos no se superponen, ambas producen el mismo orden en la práctica.
- **El mismo sprint en detección y resolución es válido**: la regla dice que el de resolución no
  puede ser anterior, y un sprint no es anterior a sí mismo. Es además el caso deseable: el equipo
  arregla dentro del mismo sprint en que detectó.
- **La reapertura no conserva historial de resoluciones**: el defecto guarda solo el sprint de la
  última resolución. El enunciado pide que reabrir borre el sprint de resolución, lo que implica que
  no se acumulan. Si más adelante las métricas necesitaran contar cuántas veces se reabrió un
  defecto, haría falta una enmienda.
- **No hay propiedad entre integrantes, pero sí consistencia bajo concurrencia**: cualquiera puede
  trabajar cualquier defecto, sin bloqueos ni reservas. Lo que el sistema sí garantiza, por decisión
  del 2026-10-03 (FR-018, RN-24), es que dos acciones incompatibles simultáneas no se aplican las
  dos: la segunda se valida contra el estado vigente y se rechaza. La invariante que se protege no
  es numérica como el tope diario de `specs/006-effort-tracking`, sino la máquina de estados, y en
  particular el carácter definitivo de Descartado.
- **Modificaciones concurrentes de datos**: dos correcciones simultáneas de descripción o severidad
  sí se resuelven por última escritura, porque no hay ninguna invariante que puedan romper entre
  ellas. La garantía de FR-018 cubre las transiciones de estado y la combinación de una modificación
  con un descarte.
- **La severidad se puede cambiar en cualquier estado**: un defecto puede resultar más grave de lo
  que parecía, incluso después de resuelto, y el enunciado no restringe cuándo se corrige.
- **Sin límite de defectos por historia ni por proyecto y sin paginación**: no se define un máximo
  de defectos ni un tope de resultados, porque no se pidió y porque las specs 002 y 003 ya dejaron
  la paginación fuera de alcance. El caso límite de la historia con muchos defectos se resuelve con
  un orden estable, no con un tope.
- **Orden por severidad y después por fecha**: el enunciado no fija orden. Se eligió severidad
  descendente porque el caso de uso dominante es encontrar lo más grave primero, y la fecha como
  segundo criterio para que el orden sea determinista.
- **Idioma de la interfaz**: los mensajes que ve la persona están en español.

---

## Preguntas Abiertas

Ninguna. Las dos decisiones que quedaron abiertas al redactar la especificación se resolvieron el
2026-10-03 y están registradas en la sección Clarifications: la mutabilidad del sprint de detección,
que se puede corregir solo mientras el defecto está Abierto (FR-038, RN-16), y el descarte de un
defecto registrado por error, que se resuelve con el estado terminal Descartado (FR-039, RN-21 a
RN-23, US6).

Las demás decisiones que podrían haber quedado abiertas se resolvieron con supuestos explícitos,
documentados en la sección Supuestos.
