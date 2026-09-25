# Especificación de Feature: Product Backlog

**Directorio de feature**: `specs/003-product-backlog`

**Rama**: `003-product-backlog`

**Creada**: 2026-09-25

**Estado**: Clarificada — sin preguntas abiertas (sesión de clarificación del 2026-09-25; enmendada el 2026-09-25 por `specs/004-sprint-management`)

**Entrada**: Descripción del usuario: "Product Backlog para Software Metrics & Estimation, un sistema
web multiusuario para estimar, planificar, seguir y medir proyectos de software con Scrum. Depende
de: autenticación y cuentas de usuario; gestión de proyectos e integrantes (propietario e
integrantes, proyecto con estados Planificado / En curso / Finalizado)."

---

## Objetivo

Permitir que los integrantes de un proyecto administren su Product Backlog: registrar, refinar,
priorizar y estimar en Story Points las historias de usuario que más adelante se planifican en
sprints.

El backlog es la lista viva de todo lo que el equipo se propone construir dentro de un proyecto. Es
la pieza que convierte una intención ("queremos exportar reportes") en una unidad de trabajo
priorizada, estimada y verificable contra criterios de aceptación explícitos. Sin backlog no hay qué
planificar en un sprint, no hay Story Points que comparar contra el esfuerzo real y, por lo tanto, no
hay ninguna métrica que producir: la velocidad, el burndown y el desvío de estimación se calculan
todos a partir de las historias que viven acá.

---

## Clarifications

### Session 2026-09-25

- Q: ¿Quién produce el paso de una historia de Pendiente a En progreso? (FR-027) → A: la feature de sprints, igual que el paso a Completada; el backlog registra y muestra el estado pero nunca lo cambia.
- Q: ¿Quién puede eliminar una historia del backlog? (FR-042) → A: cualquier integrante del proyecto, igual que crear, modificar y estimar.
- Q: ¿Qué pasa cuando dos integrantes modifican la misma historia al mismo tiempo? (FR-025) → A: gana la última escritura campo por campo; dos personas que tocan campos distintos no chocan y solo se pisan si editan el mismo campo.
- Q: ¿Puede una historia ya comprometida en un sprint quedar sin Story Points o sin criterios de aceptación? (FR-018, FR-022) → A: no; mientras esté comprometida en un sprint abierto el cambio se rechaza y hay que quitarla del sprint primero. Decisión tomada al especificar `specs/004-sprint-management` y reflejada acá.

---

## Entradas y Salidas Esperadas

### Creación de una historia

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador del proyecto, título, descripción (opcional), prioridad, Story Points (opcional) y criterios de aceptación (opcionales) | **Éxito**: historia creada en estado Pendiente, atribuida a quien la creó, con Story Points o con la marca "sin estimar" |
| | **Error de validación**: detalle de qué campo es inválido y por qué; no se crea nada |
| | **Proyecto Finalizado**: rechazo por backlog de solo lectura |
| | **Quien pide no es integrante**: respuesta de proyecto inexistente |

### Modificación de una historia

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador de la historia y los datos a modificar (título, descripción, prioridad, criterios de aceptación) | **Éxito**: historia actualizada; su identificador, su estado, su autor y su momento de creación no cambian |
| | **Error de validación**: rechazo con el motivo; la historia queda como estaba, sin cambios parciales |
| | **Historia Completada**: rechazo por historia cerrada |
| | **Proyecto Finalizado**: rechazo por backlog de solo lectura |
| | **Quien pide no es integrante**: respuesta de historia inexistente |

### Estimación de una historia

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador de la historia y un valor de la escala de Story Points o la marca "sin estimar" | **Éxito**: la historia queda con ese valor; si además tiene al menos un criterio de aceptación, pasa a estar lista para planificar |
| | **Valor fuera de la escala**: rechazo indicando los valores admitidos |
| | **Historia Completada**: rechazo por historia cerrada |
| | **Proyecto Finalizado**: rechazo por backlog de solo lectura |

### Listado del backlog

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante, identificador del proyecto y, opcionalmente, filtros por estado y por prioridad | Lista de historias del proyecto ordenada por prioridad y luego por fecha de creación, cada una con su identificador, título, prioridad, estado, Story Points o "sin estimar", cantidad de criterios de aceptación y si está lista para planificar |
| Proyecto sin historias | Lista vacía con la indicación de que el backlog todavía no tiene historias |
| Filtros que no coinciden con ninguna historia | Lista vacía con la indicación de que ninguna historia coincide con los filtros, y el detalle de los filtros aplicados |
| Filtro con un valor que no existe | Rechazo indicando los valores admitidos para ese filtro |

### Consulta del detalle de una historia

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador de la historia | Título, descripción, prioridad, estado, Story Points o "sin estimar", criterios de aceptación en su orden, si está lista para planificar, quién la creó, momento de creación y de última actualización, y si está o estuvo asignada a un sprint |
| Historia sin criterios de aceptación | Los mismos datos, con la lista de criterios vacía y la indicación de que todavía no está lista para planificar |
| Sesión de quien no es integrante | Respuesta de historia inexistente, idéntica a la de un identificador que no existe |

### Eliminación de una historia

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador de la historia | **Éxito**: la historia deja de existir y desaparece del backlog |
| | **Historia con historial**: rechazo indicando que estuvo asignada a un sprint o que tiene esfuerzo o defectos asociados; la historia no se toca |
| | **Proyecto Finalizado**: rechazo por backlog de solo lectura |
| | **Quien pide no es integrante**: respuesta de historia inexistente |

---

## Escenarios de Usuario y Pruebas *(obligatorio)*

### Historia de Usuario 1 (US1) — Registrar una historia en el backlog (Prioridad: P1)

Un integrante del proyecto anota algo que el equipo quiere construir: le pone un título, explica de
qué se trata, le asigna una prioridad y, si ya lo sabe, escribe sus criterios de aceptación y sus
Story Points.

**Por qué esta prioridad**: sin historias registradas el backlog está vacío y no hay nada que
priorizar, estimar ni planificar. Es la primera porción que entrega valor observable sobre la
gestión de proyectos ya construida.

**Prueba independiente**: se puede probar completa creando una historia en un proyecto y verificando
que queda en estado Pendiente, atribuida a quien la creó y con los datos ingresados, sin necesitar
ninguna otra historia.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un integrante de un proyecto En curso, **cuando** crea la historia
   "Exportar el reporte de velocidad a PDF" con prioridad Alta, 5 Story Points y dos criterios de
   aceptación, **entonces** la historia queda creada en estado Pendiente, con esos datos, atribuida a
   quien la creó y marcada como lista para planificar.
2. *(Caso alternativo)* **Dado** un integrante de un proyecto, **cuando** crea una historia indicando
   solo el título y la prioridad, **entonces** la historia se crea igual, queda "sin estimar", sin
   descripción, sin criterios de aceptación y marcada como no lista para planificar.
3. *(Caso alternativo)* **Dado** un proyecto en estado Planificado, **cuando** un integrante crea una
   historia, **entonces** la creación se acepta, porque el backlog se carga antes de que el proyecto
   arranque.
4. *(Caso límite)* **Dado** un integrante de un proyecto, **cuando** crea una historia con exactamente
   0 Story Points, **entonces** la historia se crea con la estimación 0, que es un valor válido y
   distinto de "sin estimar", y queda lista para planificar si tiene al menos un criterio.
5. *(Caso límite)* **Dado** un integrante de un proyecto, **cuando** crea una historia cuyo título
   tiene exactamente 150 caracteres, **entonces** la historia se crea; y **cuando** tiene 151, se
   rechaza.
6. *(Caso límite)* **Dado** un integrante de un proyecto, **cuando** crea una historia con un criterio
   de aceptación de exactamente 500 caracteres y otro de 1 carácter, **entonces** ambos se aceptan.
7. *(Caso límite)* **Dado** un integrante de un proyecto, **cuando** crea dos historias con el mismo
   título, **entonces** ambas se crean, porque el título no es único dentro del backlog.
8. *(Caso de error)* **Dado** un integrante de un proyecto, **cuando** intenta crear una historia sin
   título, con un título compuesto solo por espacios o sin prioridad, **entonces** la creación se
   rechaza indicando qué campo corregir y no se crea ninguna historia.
9. *(Caso de error)* **Dado** un integrante de un proyecto, **cuando** intenta crear una historia con
   4 Story Points, con 7, con un valor negativo o con un decimal, **entonces** la creación se rechaza
   indicando los valores admitidos de la escala.
10. *(Caso de error)* **Dado** un integrante de un proyecto, **cuando** intenta crear una historia con
    un criterio de aceptación vacío o compuesto solo por espacios, **entonces** la creación se rechaza
    indicando que cada criterio debe tener contenido y no se crea la historia.
11. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
    crear una historia, **entonces** la creación se rechaza porque el backlog es de solo lectura.
12. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta crear
    una historia en él, **entonces** la respuesta es de proyecto inexistente y no se crea nada.

---

### Historia de Usuario 2 (US2) — Ver el backlog ordenado y filtrado (Prioridad: P1)

Un integrante abre el backlog del proyecto y ve todas sus historias en el orden en que conviene
atacarlas: primero las de prioridad Alta y, dentro de cada prioridad, las más antiguas. Si el backlog
es grande, filtra por estado o por prioridad para concentrarse en una parte.

**Por qué esta prioridad**: es la contracara imprescindible del registro. Sin listado el equipo no
puede volver sobre lo que anotó, y sin orden por prioridad el backlog no cumple su función de decir
qué va primero.

**Prueba independiente**: se puede probar completa creando varias historias con distintas prioridades
y fechas y verificando el orden del listado y el efecto de cada filtro.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto con historias de prioridad Alta, Media y Baja creadas en
   distinto orden, **cuando** un integrante consulta el backlog, **entonces** ve primero todas las
   Altas, después las Medias y después las Bajas, y dentro de cada prioridad las más antiguas
   primero.
2. *(Caso alternativo)* **Dado** un backlog con historias en los tres estados, **cuando** un
   integrante filtra por estado Pendiente, **entonces** ve únicamente las historias Pendientes, en el
   mismo orden de prioridad y fecha.
3. *(Caso alternativo)* **Dado** un backlog con historias de las tres prioridades, **cuando** un
   integrante filtra por prioridad Alta y por estado Pendiente a la vez, **entonces** ve únicamente
   las historias que cumplen ambas condiciones.
4. *(Caso límite)* **Dado** un proyecto recién creado sin ninguna historia, **cuando** un integrante
   consulta el backlog, **entonces** obtiene una lista vacía con la indicación de que el backlog
   todavía no tiene historias, sin que eso sea un error.
5. *(Caso límite)* **Dado** un backlog que solo tiene historias de prioridad Baja, **cuando** un
   integrante filtra por prioridad Alta, **entonces** obtiene una lista vacía con la indicación de
   que ninguna historia coincide con los filtros aplicados, sin que eso sea un error.
6. *(Caso límite)* **Dado** dos historias de la misma prioridad creadas en el mismo instante,
   **cuando** un integrante consulta el backlog dos veces seguidas, **entonces** ambas aparecen
   siempre en el mismo orden.
7. *(Caso de error)* **Dado** un integrante de un proyecto, **cuando** consulta el backlog filtrando
   por una prioridad o un estado que no existe, **entonces** la consulta se rechaza indicando los
   valores admitidos para ese filtro.
8. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
   consultar su backlog, **entonces** la respuesta es de proyecto inexistente y no revela ninguna
   historia.

---

### Historia de Usuario 3 (US3) — Consultar el detalle de una historia (Prioridad: P2)

Un integrante abre una historia del backlog y ve todo lo que se sabe de ella: de qué se trata, qué
prioridad tiene, cuánto se estimó y, sobre todo, bajo qué criterios se va a dar por terminada.

**Por qué esta prioridad**: el listado muestra un resumen, pero los criterios de aceptación —que son
lo que hace verificable a una historia— solo caben en el detalle. Va después del listado porque se
llega desde él.

**Prueba independiente**: se puede probar completa creando una historia con criterios de aceptación y
verificando que el detalle los devuelve completos y en orden.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una historia con descripción, prioridad Media, 8 Story Points y tres
   criterios de aceptación, **cuando** un integrante consulta su detalle, **entonces** obtiene todos
   esos datos, los criterios en el orden en que se cargaron, su estado, quién la creó, cuándo se creó
   y cuándo se actualizó por última vez, y la indicación de que está lista para planificar.
2. *(Caso alternativo)* **Dado** una historia sin estimar y sin criterios de aceptación, **cuando** un
   integrante consulta su detalle, **entonces** ve la marca "sin estimar", la lista de criterios
   vacía y la indicación de que todavía no está lista para planificar, con el motivo.
3. *(Caso alternativo)* **Dado** una historia que ya fue asignada a un sprint, **cuando** un
   integrante consulta su detalle, **entonces** además de sus datos ve que está o estuvo asignada a un
   sprint.
4. *(Caso límite)* **Dado** una historia estimada en 0 Story Points, **cuando** un integrante consulta
   su detalle, **entonces** ve la estimación 0, claramente distinguible de "sin estimar".
5. *(Caso de error)* **Dado** una historia de un proyecto del que no soy integrante, **cuando** intento
   consultar su detalle, **entonces** la respuesta es de historia inexistente y no revela ningún dato,
   ni siquiera su título.
6. *(Caso de error)* **Dado** un identificador de historia que no existe, **cuando** un integrante
   intenta consultarlo, **entonces** obtiene una respuesta de historia inexistente idéntica a la del
   caso anterior.

---

### Historia de Usuario 4 (US4) — Estimar una historia en Story Points (Prioridad: P2)

Un integrante le asigna a una historia un valor de la escala de Story Points, o corrige la estimación
anterior cuando el equipo entiende mejor el trabajo.

**Por qué esta prioridad**: la estimación es la materia prima de todas las métricas del producto
(velocidad, burndown, desvío entre lo estimado y lo real). Sin Story Points ninguna historia puede
planificarse en un sprint.

**Prueba independiente**: se puede probar completa creando una historia sin estimar, asignándole un
valor de la escala y verificando que queda registrado y que la historia pasa a estar lista para
planificar si ya tenía criterios.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una historia sin estimar con dos criterios de aceptación, **cuando** un
   integrante le asigna 13 Story Points, **entonces** la historia queda estimada en 13 y pasa a estar
   lista para planificar.
2. *(Caso alternativo)* **Dado** una historia ya estimada en 5 Story Points, **cuando** un integrante
   la reestima en 8, **entonces** el valor anterior se reemplaza por 8 y queda registrado el momento
   de la última actualización.
3. *(Caso alternativo)* **Dado** una historia estimada que no está comprometida en ningún sprint,
   **cuando** un integrante la devuelve al estado "sin estimar", **entonces** el cambio se acepta y la
   historia deja de estar lista para planificar.
4. *(Caso límite)* **Dado** una historia sin criterios de aceptación, **cuando** un integrante le
   asigna 3 Story Points, **entonces** la estimación se registra pero la historia sigue sin estar
   lista para planificar, porque le falta al menos un criterio.
5. *(Caso límite)* **Dado** una historia sin estimar, **cuando** un integrante le asigna los valores
   extremos de la escala, 0 y 100, **entonces** ambos se aceptan.
6. *(Caso de error)* **Dado** una historia estimada y comprometida en un sprint abierto, **cuando**
   un integrante intenta devolverla a "sin estimar", **entonces** el cambio se rechaza indicando que
   primero debe quitarla del sprint, y la estimación no cambia.
7. *(Caso de error)* **Dado** una historia del backlog, **cuando** un integrante intenta estimarla en
   50, en 4 o en 6,5, **entonces** la estimación se rechaza indicando la escala admitida y el valor
   anterior no cambia.
8. *(Caso de error)* **Dado** una historia en estado Completada, **cuando** un integrante intenta
   reestimarla, **entonces** la acción se rechaza porque una historia completada no se reestima.
9. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
   estimar una historia de su backlog, **entonces** la acción se rechaza porque el backlog es de solo
   lectura.

---

### Historia de Usuario 5 (US5) — Refinar una historia (Prioridad: P3)

Un integrante mejora una historia existente: le corrige el título, amplía la descripción, le cambia
la prioridad o reescribe sus criterios de aceptación a medida que el equipo entiende mejor qué hay
que construir.

**Por qué esta prioridad**: es un ajuste sobre algo que ya funciona. El backlog es utilizable sin
esta historia, aunque obligaría a registrar todo perfecto la primera vez.

**Prueba independiente**: se puede probar completa creando una historia, modificando cada uno de sus
campos y verificando que el detalle refleja los nuevos valores y que el estado no cambió.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una historia Pendiente, **cuando** un integrante le cambia el título, la
   descripción, la prioridad y sus criterios de aceptación por valores válidos, **entonces** la
   historia queda actualizada y su identificador, su estado, su autor y su momento de creación no
   cambian.
2. *(Caso alternativo)* **Dado** una historia creada por otro integrante, **cuando** la refino,
   **entonces** el cambio se acepta, porque cualquier integrante del proyecto puede refinar cualquier
   historia del backlog.
3. *(Caso alternativo)* **Dado** una historia con tres criterios de aceptación, **cuando** un
   integrante envía la lista con los mismos tres criterios en otro orden, **entonces** la historia
   queda con ese nuevo orden.
4. *(Caso límite)* **Dado** una historia con dos criterios de aceptación que no está comprometida en
   ningún sprint, **cuando** un integrante envía una lista de criterios vacía, **entonces** la
   historia queda sin criterios y deja de estar lista para planificar.
5. *(Caso límite)* **Dado** una historia En progreso dentro de un sprint activo, **cuando** un
   integrante corrige su descripción, **entonces** el cambio se acepta, porque solo el estado
   Completada cierra la historia a modificaciones.
6. *(Caso límite)* **Dado** una historia que dos integrantes editan a la vez, uno cambiando la
   prioridad y el otro la descripción, **cuando** ambas modificaciones se aplican, **entonces** la
   historia queda con la prioridad nueva y la descripción nueva, sin que una pise a la otra.
7. *(Caso límite)* **Dado** una historia que dos integrantes editan a la vez, ambos reescribiendo
   sus criterios de aceptación, **cuando** ambas modificaciones se aplican, **entonces** la historia
   queda con la lista de criterios del que escribió último, sin fusionar las dos listas y sin avisar
   al primero.
8. *(Caso de error)* **Dado** una historia comprometida en un sprint abierto, **cuando** un
   integrante intenta dejarla sin criterios de aceptación, **entonces** el cambio se rechaza
   indicando que primero debe quitarla del sprint, y los criterios no cambian.
9. *(Caso de error)* **Dado** una historia del backlog, **cuando** un integrante intenta dejar el
   título vacío, poner un título de 151 caracteres, una descripción de más de 2000 caracteres, una
   prioridad que no existe o un criterio de aceptación de más de 500 caracteres, **entonces** la
   modificación se rechaza indicando el motivo y no se aplica ningún cambio parcial.
10. *(Caso de error)* **Dado** una historia en estado Completada, **cuando** un integrante intenta
    modificarla, **entonces** la acción se rechaza porque una historia completada no se modifica.
11. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
    modificar una historia de su backlog, **entonces** la acción se rechaza porque el backlog es de
    solo lectura.
12. *(Caso de error)* **Dado** una historia de un proyecto del que no soy integrante, **cuando** intento
    modificarla, **entonces** la respuesta es de historia inexistente y la historia no cambia.
13. *(Caso de error)* **Dado** una historia Pendiente que no pertenece a ningún sprint, **cuando** un
    integrante intenta pasarla a En progreso o a Completada desde el backlog, **entonces** la acción
    no está disponible en esta feature y el estado de la historia no cambia.

---

### Historia de Usuario 6 (US6) — Eliminar una historia que nunca se trabajó (Prioridad: P3)

Un integrante borra del backlog una historia que se cargó por error o que el equipo descartó, siempre
que nunca haya entrado en un sprint ni tenga trabajo registrado detrás.

**Por qué esta prioridad**: mantiene el backlog limpio, pero el producto funciona sin ella: una
historia descartada puede quedar en prioridad Baja sin estorbar. La restricción existe para que
eliminar nunca destruya historial de medición.

**Prueba independiente**: se puede probar completa creando una historia, eliminándola y verificando
que desaparece del backlog, y creando otra asignada a un sprint para verificar que su eliminación se
rechaza.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** una historia recién creada que nunca estuvo en un sprint y no tiene
   esfuerzo ni defectos asociados, **cuando** un integrante la elimina, **entonces** la historia deja
   de existir y ya no aparece en el backlog.
2. *(Caso alternativo)* **Dado** una historia estimada y con criterios de aceptación pero nunca
   asignada a un sprint, **cuando** un integrante la elimina, **entonces** la eliminación se acepta,
   porque estar lista para planificar no es lo mismo que haberse planificado.
3. *(Caso alternativo)* **Dado** una historia creada por otro integrante y sin historial asociado,
   **cuando** la elimino sin ser el propietario del proyecto, **entonces** la eliminación se acepta,
   porque cualquier integrante puede eliminar cualquier historia del backlog.
4. *(Caso límite)* **Dado** una historia que fue asignada a un sprint y luego quitada de él,
   **cuando** un integrante intenta eliminarla, **entonces** la eliminación se rechaza, porque la
   condición es no haber estado nunca en un sprint.
5. *(Caso límite)* **Dado** un backlog con una sola historia, **cuando** un integrante la elimina,
   **entonces** el backlog queda vacío y el listado lo informa como backlog sin historias, no como
   error.
6. *(Caso de error)* **Dado** una historia que tiene esfuerzo registrado o defectos asociados,
   **cuando** un integrante intenta eliminarla, **entonces** la eliminación se rechaza indicando qué
   historial se lo impide y la historia no se modifica.
7. *(Caso de error)* **Dado** un proyecto en estado Finalizado, **cuando** un integrante intenta
   eliminar una historia de su backlog, **entonces** la acción se rechaza porque el backlog es de solo
   lectura.
8. *(Caso de error)* **Dado** una historia de un proyecto del que no soy integrante, **cuando** intento
   eliminarla, **entonces** la respuesta es de historia inexistente y la historia no se toca.
9. *(Caso de error)* **Dado** una historia ya eliminada, **cuando** un integrante intenta eliminarla de
   nuevo, **entonces** obtiene una respuesta de historia inexistente.

---

### Casos Límite

- **Historia estimada en 0 Story Points**: válida y distinta de "sin estimar". Una historia de 0
  puntos está estimada y puede estar lista para planificar; una "sin estimar" nunca lo está.
- **Título de exactamente 150 caracteres**: válido; 151 se rechaza. El conteo se hace después de
  recortar los espacios al inicio y al final.
- **Título compuesto solo por espacios**: equivale a título ausente y se rechaza.
- **Criterio de aceptación de exactamente 1 y de exactamente 500 caracteres**: ambos válidos; 0 y 501
  se rechazan. El conteo se hace después de recortar los espacios extremos.
- **Historia sin criterios de aceptación**: estado válido y esperable en un backlog sin refinar; la
  historia existe pero no está lista para planificar.
- **Historia con Story Points pero sin criterios, y viceversa**: ninguna de las dos está lista para
  planificar; hacen falta las dos condiciones a la vez.
- **Backlog vacío**: el listado devuelve una lista vacía con una indicación explícita; no es un error.
- **Filtros que no devuelven resultados**: lista vacía con la indicación de que ninguna historia
  coincide, junto con los filtros aplicados; no es un error y se distingue del backlog vacío.
- **Dos historias con el mismo título en el mismo backlog**: permitido; el título no es único.
- **Dos historias de la misma prioridad creadas en el mismo instante**: el orden entre ellas es
  siempre el mismo entre consultas, desempatado por su identificador.
- **Dos integrantes editando la misma historia a la vez**: si tocan campos distintos, los dos cambios
  se conservan. Si tocan el mismo campo, queda el del que escribió último y el otro se pierde sin
  aviso. La lista de criterios de aceptación cuenta como un solo campo.
- **Reestimar una historia que ya está en un sprint activo**: permitido mientras no esté Completada;
  el impacto sobre las métricas del sprint lo define la feature de sprints.
- **Historia asignada a un sprint y luego quitada de él**: ya no se puede eliminar, porque la
  condición es no haber estado nunca en un sprint.
- **Historia que nunca entró en un sprint**: permanece siempre en estado Pendiente, porque el backlog
  no cambia estados por su cuenta.
- **Vaciar la lista de criterios de aceptación de una historia ya lista**: permitido mientras la
  historia no esté comprometida en un sprint abierto; entonces deja de estar lista para planificar. Si
  está comprometida, se rechaza hasta que se la quite del sprint.
- **Historia comprometida en un sprint abierto**: conserva sus Story Points y al menos un criterio de
  aceptación hasta que se la quite del sprint. Cambiar un valor de Story Points por otro sigue
  permitido, porque la historia no deja de estar lista para planificar.
- **Proyecto Finalizado**: el backlog admite consulta por parte de cualquier integrante y rechaza toda
  acción de escritura, incluida la eliminación.
- **Integrante quitado del proyecto**: pierde el acceso al backlog de inmediato; las historias que
  creó se conservan y siguen mostrando su nombre.

---

## Requisitos *(obligatorio)*

### Requisitos Funcionales

**Creación de historias**

- **FR-001**: El sistema DEBE permitir a cualquier integrante de un proyecto crear una historia en su
  backlog a partir de título, descripción, prioridad, Story Points y criterios de aceptación.
- **FR-002**: El sistema DEBE exigir un título de entre 1 y 150 caracteres, contados después de
  recortar los espacios al inicio y al final; un título ausente o compuesto solo por espacios se
  rechaza.
- **FR-003**: El sistema DEBE tratar la descripción como opcional y admitir hasta 2000 caracteres.
- **FR-004**: El sistema DEBE exigir una prioridad y admitir únicamente los valores Alta, Media y
  Baja.
- **FR-005**: El sistema DEBE tratar los Story Points como opcionales en la creación; si no se
  indican, la historia nace "sin estimar".
- **FR-006**: El sistema DEBE tratar los criterios de aceptación como opcionales en la creación y
  admitirlos como una lista ordenada de enunciados de texto.
- **FR-007**: El sistema DEBE exigir que cada criterio de aceptación tenga entre 1 y 500 caracteres,
  contados después de recortar los espacios extremos; un criterio vacío o compuesto solo por espacios
  se rechaza.
- **FR-008**: Toda historia DEBE nacer en estado Pendiente.
- **FR-009**: Cada historia DEBE tener un identificador propio, estable, inmutable y no secuencial,
  distinto del título.
- **FR-010**: Cada historia DEBE pertenecer a exactamente un proyecto y NO DEBE poder moverse a otro.
- **FR-011**: El sistema DEBE registrar en cada historia quién la creó, su momento de creación y su
  momento de última actualización.
- **FR-012**: El sistema DEBE permitir que dos historias del mismo backlog tengan el mismo título.
- **FR-013**: El sistema DEBE rechazar una creación inválida indicando qué campo corregir y por qué,
  sin crear la historia ni ninguno de sus criterios de aceptación.

**Estimación en Story Points**

- **FR-014**: El sistema DEBE admitir como Story Points únicamente los valores de la escala 0, 1, 2,
  3, 5, 8, 13, 20, 40 y 100, o la marca "sin estimar".
- **FR-015**: El sistema DEBE tratar el valor 0 como una estimación válida y distinguible de "sin
  estimar".
- **FR-016**: El sistema DEBE permitir a cualquier integrante asignar directamente un valor de la
  escala a una historia.
- **FR-017**: Una nueva estimación DEBE reemplazar a la anterior y actualizar el momento de última
  actualización de la historia.
- **FR-018**: El sistema DEBE permitir devolver una historia estimada al estado "sin estimar",
  excepto cuando la historia esté comprometida en un sprint abierto (Planificado o Activo): en ese
  caso el cambio se rechaza indicando que primero hay que quitarla del sprint (ver RC-09).
- **FR-019**: El sistema DEBE rechazar todo valor de Story Points fuera de la escala, incluidos los
  números intermedios, los negativos y los decimales, indicando los valores admitidos y sin alterar el
  valor vigente.

**Refinamiento de historias**

- **FR-020**: El sistema DEBE permitir a cualquier integrante modificar el título, la descripción, la
  prioridad y los criterios de aceptación de una historia. Una solicitud de modificación DEBE aplicar
  únicamente los campos que incluye: un campo ausente deja su valor vigente sin cambios, y un campo
  enviado vacío borra el valor cuando ese campo admite estar vacío.
- **FR-021**: El sistema DEBE aplicar en la modificación exactamente las mismas validaciones de
  título, descripción, prioridad, Story Points y criterios de aceptación que en la creación (FR-002 a
  FR-007 y FR-014).
- **FR-022**: La modificación de los criterios de aceptación DEBE reemplazar la lista completa,
  de modo que agregar, quitar y reordenar criterios se resuelven por la misma vía. El sistema DEBE
  rechazar dejar sin criterios a una historia comprometida en un sprint abierto, indicando que
  primero hay que quitarla del sprint (ver RC-09).
- **FR-023**: La modificación DEBE ser atómica: si algún dato es inválido, ningún cambio se aplica.
- **FR-024**: La modificación NO DEBE alterar el identificador de la historia, su estado, el proyecto
  al que pertenece, quién la creó ni su momento de creación.
- **FR-025**: Ante dos modificaciones simultáneas de la misma historia, el sistema DEBE conservar
  ambos cambios cuando afectan campos distintos, y DEBE dejar prevalecer la última en aplicarse
  cuando afectan el mismo campo, sin avisar a quien lo había modificado antes. A estos efectos, la
  lista de criterios de aceptación se considera un único campo: dos listas enviadas a la vez no se
  fusionan, prevalece la última.

**Estado y disponibilidad para planificar**

- **FR-026**: Toda historia DEBE tener exactamente uno de estos tres estados: Pendiente, En progreso o
  Completada.
- **FR-027**: El sistema NO DEBE ofrecer ninguna acción de cambio de estado de una historia dentro de
  esta feature: tanto el paso a En progreso como el paso a Completada los produce la feature de
  sprints. El backlog registra y muestra el estado vigente, pero nunca lo modifica.
- **FR-028**: El sistema DEBE considerar una historia "lista para planificar" cuando tiene Story
  Points asignados (incluido el valor 0) y al menos un criterio de aceptación.
- **FR-029**: El sistema DEBE exponer la condición de lista para planificar tanto en el listado del
  backlog como en el detalle de la historia, y en el detalle DEBE indicar qué condición falta cuando
  no se cumple.
- **FR-030**: Una historia en estado Completada NO DEBE poder modificarse ni reestimarse.

**Listado del backlog**

- **FR-031**: El sistema DEBE permitir a cualquier integrante listar las historias del backlog de un
  proyecto.
- **FR-032**: El listado DEBE ordenarse por prioridad, de Alta a Baja, y dentro de cada prioridad por
  fecha de creación ascendente; ante igual prioridad y fecha, el orden DEBE desempatarse por el
  identificador de la historia, de modo que sea estable entre consultas.
- **FR-033**: Cada elemento del listado DEBE incluir al menos el identificador, el título, la
  prioridad, el estado, los Story Points o la marca "sin estimar", la cantidad de criterios de
  aceptación y si la historia está lista para planificar.
- **FR-034**: El sistema DEBE permitir filtrar el listado por estado y por prioridad, admitiendo
  varios valores en cada filtro y combinando ambos filtros de forma que una historia se incluya solo
  si cumple las dos condiciones.
- **FR-035**: Cuando el backlog no tiene ninguna historia, el sistema DEBE devolver un listado vacío
  con una indicación clara, no un error.
- **FR-036**: Cuando los filtros aplicados no coinciden con ninguna historia, el sistema DEBE devolver
  un listado vacío indicando los filtros aplicados, de forma distinguible del backlog vacío, y no un
  error.
- **FR-037**: El sistema DEBE rechazar un filtro cuyo valor no pertenezca a los estados o prioridades
  admitidos, indicando los valores válidos.
- **FR-038**: El listado NO DEBE incluir historias de ningún otro proyecto.

**Consulta del detalle**

- **FR-039**: El sistema DEBE permitir a cualquier integrante consultar el detalle de una historia y
  devolver: identificador, título, descripción, prioridad, estado, Story Points o la marca "sin
  estimar", criterios de aceptación en su orden, condición de lista para planificar, quién la creó,
  momento de creación, momento de última actualización y si está o estuvo asignada a un sprint.
- **FR-040**: Cuando la historia no tiene criterios de aceptación, el sistema DEBE devolver la lista
  vacía, sin tratarlo como error.

**Eliminación**

- **FR-041**: El sistema DEBE permitir eliminar una historia únicamente cuando nunca fue asignada a un
  sprint y no tiene esfuerzo registrado ni defectos asociados.
- **FR-042**: El sistema DEBE permitir eliminar una historia a cualquier integrante del proyecto, sin
  importar quién la creó ni si es el propietario del proyecto.
- **FR-043**: El sistema DEBE rechazar la eliminación de una historia con historial asociado,
  indicando cuál de las tres condiciones se lo impide, sin modificar la historia.
- **FR-044**: Una historia eliminada DEBE desaparecer del backlog y de toda consulta posterior; un
  intento de operar sobre ella se responde como historia inexistente.

**Autorización y visibilidad**

- **FR-045**: El sistema DEBE exigir una sesión válida para toda acción de esta feature, sin
  excepciones.
- **FR-046**: El sistema DEBE restringir toda acción sobre el backlog de un proyecto a los integrantes
  vigentes de ese proyecto.
- **FR-047**: Ante una solicitud sobre un proyecto o una historia a los que quien pide no tiene
  acceso, el sistema DEBE responder exactamente igual que ante un identificador inexistente, sin
  revelar ningún dato ni la existencia del recurso.
- **FR-048**: Cuando el proyecto está en estado Finalizado, el sistema DEBE tratar su backlog como de
  solo lectura: rechaza crear, modificar, estimar y eliminar historias, y permite listar y consultar.
- **FR-049**: Toda creación, modificación, estimación y eliminación DEBE quedar atribuida al titular
  de la sesión que la ejecutó.
- **FR-050**: Las historias creadas por una persona que luego deja de ser integrante DEBEN conservarse
  y seguir mostrando su nombre como autora.

---

### Reglas de Negocio

| ID | Regla |
| --- | --- |
| RN-01 | El backlog pertenece a un proyecto; cualquier integrante del proyecto puede consultarlo y trabajarlo. |
| RN-02 | Cualquier integrante puede crear, modificar, estimar y eliminar historias, sin importar quién las creó. |
| RN-03 | El título es obligatorio y mide hasta 150 caracteres; no es único dentro del backlog. |
| RN-04 | La descripción es opcional y mide hasta 2000 caracteres. |
| RN-05 | La prioridad es obligatoria y solo puede ser Alta, Media o Baja. |
| RN-06 | Los Story Points solo pueden ser 0, 1, 2, 3, 5, 8, 13, 20, 40, 100 o "sin estimar". |
| RN-07 | Una historia estimada en 0 está estimada; "sin estimar" es la ausencia de estimación. |
| RN-08 | Los criterios de aceptación forman una lista ordenada y cada uno mide entre 1 y 500 caracteres. |
| RN-09 | Toda historia nace Pendiente. |
| RN-10 | Los cambios de estado de una historia ocurren dentro de un sprint y los regula la feature de sprints; el backlog nunca cambia el estado de una historia. |
| RN-11 | El paso de una historia a Completada solo ocurre dentro de un sprint activo. |
| RN-12 | Una historia está lista para planificar cuando tiene Story Points y al menos un criterio de aceptación. |
| RN-13 | Solo las historias listas para planificar pueden asignarse a un sprint. |
| RN-14 | Una historia Completada no se modifica ni se reestima. |
| RN-15 | En un proyecto Finalizado el backlog es de solo lectura. |
| RN-16 | Una historia solo se elimina si nunca estuvo en un sprint y no tiene esfuerzo ni defectos asociados. |
| RN-17 | El backlog se ordena por prioridad de Alta a Baja y, a igual prioridad, por fecha de creación ascendente. |
| RN-18 | Una historia comprometida en un sprint abierto no puede quedar sin Story Points ni sin criterios de aceptación; para eso hay que quitarla antes del sprint. |

---

### Restricciones

- **RC-01**: Esta feature depende de la feature de autenticación y cuentas de usuario
  (`specs/001-user-auth`): solo usuarios registrados con sesión válida operan sobre el backlog.
- **RC-02**: Esta feature depende de la feature de gestión de proyectos e integrantes
  (`specs/002-project-members`): el backlog existe dentro de un proyecto, el permiso de acceso es la
  membresía vigente y el estado Finalizado del proyecto vuelve el backlog de solo lectura.
- **RC-03**: La condición "lista para planificar" se define acá, pero quien la hace cumplir al armar
  un sprint es la feature de sprints. Esta feature no asigna historias a sprints.
- **RC-04**: Todos los cambios de estado de una historia (a En progreso y a Completada) los produce la
  feature de sprints. Esta feature registra y muestra el estado, y bloquea la modificación de las
  historias Completadas, pero no provoca ninguna transición. En consecuencia, una historia que no
  pertenece a ningún sprint permanece siempre en Pendiente.
- **RC-05**: Las tres condiciones que bloquean la eliminación (haber estado en un sprint, tener
  esfuerzo registrado, tener defectos asociados) son datos que producen las features de sprints,
  esfuerzo y defectos. Esta feature consulta su existencia; no las genera.
- **RC-06**: La estimación colaborativa por Planning Poker es una feature propia que actualiza el
  mismo valor de Story Points definido acá, respetando la misma escala y las mismas restricciones de
  estado.
- **RC-07**: La cantidad de historias del backlog por estado y los Story Points totales que muestra la
  vista de estado del proyecto (`specs/002-project-members`) se calculan sobre las historias definidas
  en esta feature; los estados de historia que esa vista informa son Pendiente, En progreso y
  Completada.
- **RC-08**: No existe forma de archivar una historia ni de recuperarla después de eliminarla; para
  descartar una historia que ya tiene historial, la única vía disponible es bajarle la prioridad.
- **RC-09**: Una historia comprometida en un sprint abierto (Planificado o Activo) no puede dejar de
  estar lista para planificar: no se le pueden quitar los Story Points ni su último criterio de
  aceptación hasta que se la quite del sprint. La condición la impone
  `specs/004-sprint-management` (su FR-018 y RN-22) para que ningún sprint llegue al cierre con
  historias a medio definir, y esta spec la refleja en FR-018 y FR-022. Saber si una historia está
  comprometida es un dato que produce la feature de sprints.
---

### Condiciones de Error

| Condición | Comportamiento esperado |
| --- | --- |
| Acción de la feature sin sesión válida | Rechazo con indicación de iniciar sesión; la acción no se ejecuta. |
| Título ausente o compuesto solo por espacios | Rechazo indicando que el título es obligatorio. |
| Título de más de 150 caracteres | Rechazo indicando el máximo permitido. |
| Descripción de más de 2000 caracteres | Rechazo indicando el máximo permitido. |
| Prioridad ausente o fuera de Alta, Media y Baja | Rechazo indicando los valores admitidos. |
| Story Points fuera de la escala admitida | Rechazo indicando la escala completa; el valor vigente no cambia. |
| Criterio de aceptación vacío o compuesto solo por espacios | Rechazo indicando que cada criterio debe tener contenido; no se guarda ninguno. |
| Criterio de aceptación de más de 500 caracteres | Rechazo indicando el máximo permitido. |
| Modificación o reestimación de una historia Completada | Rechazo indicando que una historia completada no se modifica ni se reestima. |
| Quitar los Story Points o el último criterio de aceptación a una historia comprometida en un sprint abierto | Rechazo indicando que primero hay que quitarla del sprint; la historia no cambia. |
| Eliminación de una historia que estuvo en un sprint | Rechazo indicando que tiene historial de planificación. |
| Eliminación de una historia con esfuerzo registrado o defectos asociados | Rechazo indicando qué historial se lo impide. |
| Cualquier acción de escritura sobre el backlog de un proyecto Finalizado | Rechazo por backlog de solo lectura. |
| Filtro de estado o prioridad con un valor inexistente | Rechazo indicando los valores admitidos para ese filtro. |
| Filtros válidos que no coinciden con ninguna historia | Listado vacío con los filtros aplicados; no es un error. |
| Backlog sin historias | Listado vacío con la indicación correspondiente; no es un error. |
| Solicitud sobre un proyecto o una historia de los que quien pide no es integrante | Respuesta idéntica a la de un identificador inexistente, sin revelar ningún dato. |
| Solicitud sobre un identificador de historia que no existe | Respuesta de historia inexistente. |

---

### Entidades Clave

- **Historia de usuario**: representa una unidad de trabajo que el equipo quiere construir y medir.
  Atributos relevantes: identificador propio estable y no secuencial, proyecto al que pertenece,
  título, descripción, prioridad, estado, Story Points (o la ausencia de estimación), criterios de
  aceptación, autor, momento de creación y momento de última actualización. Es la unidad a la que se
  refieren la planificación de sprints, el esfuerzo registrado y los defectos.
- **Criterio de aceptación**: enunciado de texto que define cuándo una historia se considera
  terminada. Pertenece a una historia, ocupa una posición dentro de su lista ordenada y mide entre 1 y
  500 caracteres. Tener al menos uno es condición para que la historia esté lista para planificar.
- **Prioridad**: representa la urgencia relativa de una historia dentro del backlog. Valores posibles:
  Alta, Media y Baja, con ese orden de precedencia en el listado.
- **Estado de la historia**: representa el punto del ciclo de trabajo en que está la historia. Valores
  posibles: Pendiente, En progreso y Completada. Toda historia nace Pendiente y sus transiciones las
  produce exclusivamente la feature de sprints.
- **Escala de Story Points**: conjunto cerrado de valores admitidos (0, 1, 2, 3, 5, 8, 13, 20, 40,
  100) más la marca "sin estimar", que expresa que la historia todavía no fue estimada.
- **Proyecto** e **Integrante del proyecto** *(entidades de `specs/002-project-members`)*: esta
  feature los consume para ubicar el backlog, decidir quién accede y saber si el proyecto está
  Finalizado; no los modifica.
- **Usuario** *(entidad de `specs/001-user-auth`)*: esta feature lo consume para atribuir la autoría
  de cada historia y de cada cambio; no lo modifica.
- **Sprint**, **Registro de esfuerzo** y **Defecto** *(entidades de features propias)*: esta feature
  solo consulta su existencia para informar si una historia estuvo planificada y para decidir si puede
  eliminarse.

---

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **SC-001**: Un integrante registra una historia con título, prioridad y criterios de aceptación en
  menos de 2 minutos, y el 95 % lo logra en el primer intento, sin ayuda externa.
- **SC-002**: Un integrante estima una historia existente en menos de 30 segundos y en no más de 3
  pasos.
- **SC-003**: Un integrante identifica las historias de mayor prioridad todavía sin estimar en menos
  de 15 segundos desde que abre el backlog, sin combinar información de otros lugares del sistema.
- **SC-004**: El 100 % de las consultas del backlog devuelve las historias en el orden definido
  (prioridad y luego fecha de creación), verificado con un backlog de al menos 50 historias que cubra
  las tres prioridades.
- **SC-005**: El 100 % de las acciones de esta feature exige sesión válida y membresía vigente en el
  proyecto, verificado con una prueba por acción.
- **SC-006**: Cero filtraciones de datos entre proyectos: en el 100 % de los intentos de acceso de
  quien no es integrante, la respuesta es indistinguible de la de un identificador inexistente,
  verificado comparando ambas respuestas para cada acción de la feature.
- **SC-007**: El 100 % de los valores de Story Points fuera de la escala se rechaza, verificado
  probando al menos los valores 4, 6, 7, 50, 101, un negativo y un decimal.
- **SC-008**: Cero historias cuyo título supere los 150 caracteres o cuyos criterios de aceptación
  superen los 500, verificado con pruebas en el límite exacto y un carácter por encima.
- **SC-009**: El 100 % de las historias marcadas como listas para planificar tiene Story Points y al
  menos un criterio de aceptación, y ninguna historia que cumpla ambas condiciones queda sin marcar.
- **SC-010**: Cero historias Completadas modificadas o reestimadas, verificado intentando cada
  operación de escritura sobre una historia Completada.
- **SC-011**: Cero registros de esfuerzo, defectos o historial de sprints perdidos por la eliminación
  de una historia, verificado intentando eliminar historias con cada uno de los tres tipos de
  historial.
- **SC-012**: El 100 % de las acciones de escritura sobre el backlog de un proyecto Finalizado se
  rechaza, sin producir ningún cambio.
- **SC-013**: En el 100 % de las parejas de modificaciones simultáneas sobre campos distintos de la
  misma historia, los dos cambios quedan aplicados, verificado enviando ambas modificaciones a la vez
  y comparando el detalle resultante.
- **SC-014**: El 100 % de las reglas de negocio (RN-01 a RN-18) tiene al menos una prueba automatizada
  asociada que falla si la regla se rompe.

---

## Fuera de Alcance

- Tareas dentro de las historias (descomposición en subunidades de trabajo).
- Épicas y cualquier otra agrupación jerárquica de historias.
- Etiquetas y categorías libres sobre las historias.
- Adjuntos y archivos asociados a una historia.
- Comentarios y discusión dentro de una historia.
- Reordenamiento manual del backlog por arrastre; el orden lo determinan la prioridad y la fecha de
  creación.
- Planning Poker y toda forma de estimación colaborativa, que se especifican en una feature propia.
- Asignación de historias a sprints y cambio de estado dentro del sprint, que se especifican en la
  feature de sprints.
- Registro de esfuerzo y gestión de defectos sobre las historias, que se especifican en features
  propias.
- Métricas y gráficos calculados a partir del backlog (velocidad, burndown, desvío de estimación).
- Búsqueda por texto y paginación del listado del backlog.
- Historial de auditoría consultable de los cambios de una historia (quién cambió qué y cuándo).
- Asignación de una historia a una persona responsable.

---

## Supuestos

- **Criterios de aceptación opcionales al crear**: el enunciado los incluye entre los datos de
  creación pero también admite explícitamente la historia sin criterios como caso límite válido; se
  adoptan como opcionales, y su ausencia es lo que impide que la historia esté lista para planificar.
- **Prioridad obligatoria y sin valor por defecto**: el enunciado marca como opcionales solo los Story
  Points, así que la prioridad se exige en la creación en lugar de asumir un valor.
- **Título no único**: no se pidió unicidad y dos historias pueden describir trabajos distintos con el
  mismo título corto, así que no se impone esa restricción.
- **Orden dentro de la misma prioridad**: se adopta fecha de creación ascendente, de más antigua a más
  reciente, porque lo que se anotó primero se refina y se planifica primero. El desempate por
  identificador existe para que el orden sea reproducible en las pruebas.
- **Filtros combinables y multivalor**: los filtros por estado y por prioridad admiten varios valores
  cada uno y se combinan de forma restrictiva, que es el comportamiento esperable de un tablero de
  backlog.
- **Devolver una historia a "sin estimar"**: se admite porque una reestimación puede revelar que el
  equipo todavía no entiende el trabajo; no hacerlo obligaría a dejar un número inventado. La única
  excepción es que la historia esté comprometida en un sprint abierto (RC-09).
- **Reemplazo completo de la lista de criterios**: agregar, editar, quitar y reordenar criterios se
  resuelven enviando la lista entera, para no multiplicar operaciones sobre una lista corta.
- **Eliminación definitiva**: como solo se puede eliminar una historia sin ningún historial asociado,
  el borrado es definitivo y no se conserva copia; no hay nada que auditar ni que restaurar.
- **Modificación de historias En progreso**: se permite, porque el enunciado solo cierra a
  modificaciones las historias Completadas. Refinar criterios durante el sprint es una práctica
  habitual.
- **Sin aviso de conflicto de edición**: decisión confirmada el 2026-09-25 (FR-025). El sistema
  resuelve la edición simultánea campo por campo y no avisa a quien perdió su cambio sobre un mismo
  campo. Mostrar un aviso de conflicto o una vista de comparación queda fuera de alcance.
- **Sin límite de historias ni de criterios**: no se define un máximo de historias por backlog ni de
  criterios de aceptación por historia, porque no se pidió.
- **Idioma de la interfaz**: los mensajes que ve la persona están en español.
- **Autoría conservada**: si una persona deja de ser integrante del proyecto, las historias que creó
  se conservan con su nombre, igual que el resto de sus registros en `specs/002-project-members`.

---

## Preguntas Abiertas

Ninguna. Las tres decisiones que quedaron abiertas al redactar la especificación se resolvieron el
2026-09-25 y están registradas en la sección Clarifications: quién cambia el estado de una historia
(FR-027), quién puede eliminarla (FR-042) y cómo se resuelve la edición simultánea (FR-025).
