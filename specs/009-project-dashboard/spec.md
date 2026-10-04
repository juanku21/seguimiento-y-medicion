# Especificación de Feature: Dashboard del Proyecto

**Directorio de feature**: `specs/009-project-dashboard`

**Rama**: `009-project-dashboard`

**Creada**: 2026-10-03

**Estado**: Clarificada — sin preguntas abiertas (sesión de clarificación del 2026-10-03)

**Entrada**: Descripción del usuario: "Dashboard del proyecto para Software Metrics & Estimation, un
sistema web multiusuario para estimar, planificar, seguir y medir proyectos de software con Scrum.
Depende de: gestión de proyectos e integrantes (consulta de estado); gestión de Sprints; cálculo de
métricas (única fuente de las cifras)."

---

## Objetivo

Ofrecer a los integrantes una vista única que muestre de un vistazo el estado del proyecto y la
evolución de sus métricas principales con gráficos, para detectar desvíos y tomar decisiones sin
tener que recorrer cada módulo por separado.

Las ocho features anteriores construyeron las piezas y la de métricas las convirtió en números. Pero
un número en una pantalla no es lo mismo que una respuesta: para saber si el equipo está mejorando
hay que ver tres sprints seguidos, y para saber si va a llegar hay que comparar lo estimado con lo
real sin abrir cuatro pantallas. Esta feature es el lugar donde las cifras se vuelven una lectura.

De ahí su regla más importante y la que más disciplina exige: **el dashboard no calcula nada**. Cada
cifra que muestra viene de la feature de métricas. Es tentador que una pantalla sume dos columnas
por su cuenta "porque es trivial", y es exactamente así como un producto termina mostrando una
velocidad en el tablero y otra distinta en el reporte. El dashboard presenta; la feature de métricas
decide cuánto vale cada cosa.

La segunda regla en importancia es no mentir con el formato. Un "no calculable" convertido en 0, o
una cifra parcial de un sprint en curso mostrada igual que una definitiva, no son errores de
cálculo: son errores de presentación que producen decisiones equivocadas. La feature de métricas se
tomó el trabajo de distinguir esos estados; el dashboard tiene que mostrarlos, no aplanarlos.

---

## Clarifications

### Session 2026-10-03

- Q: La feature de métricas expone una sola serie por sprint y los gráficos necesitan seis. ¿De dónde salen las cinco que faltan? (FR-026) → A: de una métrica nueva de "series del proyecto" en `specs/008-metrics-calculation`, que entrega las seis juntas en una sola consulta. Esa feature sigue siendo dueña de todo lo que se grafica, incluido el orden. Requiere enmendar la spec 008.
- Q: Los días restantes del sprint activo no son una métrica pero son un cálculo. ¿Los deriva el dashboard? (FR-004) → A: no; los entrega `specs/004-sprint-management` junto con los datos del sprint. La regla "el dashboard no calcula nada" queda sin excepciones. Requiere enmendar la spec 004.
- Q: ¿El dashboard muestra las advertencias que produce la feature de métricas? (FR-032) → A: sí, las dos clases —de planificación y de inconsistencia de datos— y distinguidas entre sí. Esconderlas anularía la decisión de la spec 008 de avisar en vez de calcular en silencio.

---

## Entradas y Salidas Esperadas

### Apertura del dashboard de un proyecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador del proyecto | Dashboard completo: resumen del proyecto, indicadores clave, los tres gráficos de evolución y la distribución de defectos abiertos por severidad |
| Proyecto sin sprints | Estado vacío que explica que todavía no hay datos y qué hacer a continuación, en lugar de gráficos en cero |
| Proyecto sin sprint Activo | Resumen y gráficos normales; la sección del sprint activo informa que no hay ninguno en curso |
| Proyecto Finalizado | Dashboard completo de solo lectura, con el estado del proyecto visible y sin sección de sprint activo |
| Falla de una sección | El resto del dashboard se muestra igual y la sección afectada indica que no pudo obtenerse |
| Proyecto inexistente o de quien no es integrante | Respuesta de proyecto inexistente, sin revelar ningún dato |

### Resumen del proyecto

| Entradas | Salidas esperadas |
| --- | --- |
| Datos del proyecto y de su sprint Activo | Nombre, estado, fecha de inicio, fecha de finalización prevista, lista de integrantes y, si hay sprint Activo, su nombre, Sprint Goal, fechas y días restantes |
| Proyecto Finalizado | Además, su fecha de finalización real |
| Sprint Activo cuya fecha de fin ya pasó | Los días restantes se informan como vencidos, no como un número negativo suelto |

### Indicadores clave

| Entradas | Salidas esperadas |
| --- | --- |
| Métricas del proyecto y del sprint Activo | Velocidad del equipo, Story Points planificados y completados del sprint activo, porcentaje de historias completadas, horas estimadas contra reales con su desviación, y cantidad de defectos abiertos |
| Métrica con valor "no calculable" | Se muestra como "no calculable" con una explicación breve del motivo, nunca como 0 |
| Cifras del sprint Activo | Se muestran identificadas como parciales |

### Gráficos de evolución

| Entradas | Salidas esperadas |
| --- | --- |
| Serie de Story Points planificados y completados por sprint | Gráfico de velocidad en orden cronológico, con título, ejes rotulados y leyenda |
| Serie de horas estimadas y reales por sprint | Gráfico de esfuerzo con el mismo tratamiento |
| Serie de defectos detectados y resueltos por sprint | Gráfico de calidad con el mismo tratamiento |
| Conteo de defectos abiertos por severidad | Distribución por las cuatro severidades, con su total |
| Serie vacía o con un único punto | Se muestra igual, con la indicación de que todavía no hay evolución que comparar |

---

## Escenarios de Usuario y Pruebas *(obligatorio)*

### Historia de Usuario 1 (US1) — Ver el estado del proyecto y del sprint en curso (Prioridad: P1)

Un integrante abre el dashboard y en la primera pantalla sabe en qué proyecto está, en qué estado,
quiénes lo integran y qué se está haciendo ahora mismo.

**Por qué esta prioridad**: es la razón por la que alguien abre un dashboard. Sin esto, la pantalla
es un conjunto de gráficos sin contexto, y el integrante igual tiene que ir a otro módulo para saber
de qué proyecto le están hablando.

**Prueba independiente**: se puede probar completa abriendo el dashboard de un proyecto con sprint
Activo y verificando que el resumen muestra todos los datos esperados y los días restantes
correctos.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto En curso con 4 integrantes y un sprint Activo que termina el
   2026-10-14, **cuando** un integrante abre el dashboard el 2026-10-04, **entonces** ve el nombre
   del proyecto, su estado, sus fechas previstas, los 4 integrantes, y del sprint activo su nombre,
   su Sprint Goal, sus fechas y **10 días restantes**.
2. *(Caso alternativo)* **Dado** un proyecto Finalizado, **cuando** un integrante abre el dashboard,
   **entonces** ve el estado Finalizado y su fecha de finalización real, y la sección del sprint
   activo indica que no hay ninguno en curso.
3. *(Caso alternativo)* **Dado** un proyecto En curso sin sprint Activo, porque el último se cerró y
   no se inició otro, **cuando** un integrante abre el dashboard, **entonces** el resumen se muestra
   igual y la sección del sprint activo indica que no hay ninguno en curso, sin que eso sea un
   error.
4. *(Caso límite)* **Dado** un sprint Activo cuya fecha de fin es hoy y para el que la feature de
   sprints informa 0 días restantes, **cuando** un integrante abre el dashboard, **entonces** ve
   **0 días restantes**, no un número negativo ni un vacío.
5. *(Caso límite)* **Dado** un sprint Activo que la feature de sprints informa como vencido hace 3
   días, **cuando** un integrante abre el dashboard, **entonces** ve esa condición indicando por
   cuánto, en lugar de mostrar "−3" sin contexto.
6. *(Caso límite)* **Dado** un sprint Activo, **cuando** un integrante abre el dashboard,
   **entonces** los días restantes que ve coinciden exactamente con los que entrega la feature de
   sprints, sin que el dashboard reste fechas por su cuenta.
7. *(Caso límite)* **Dado** un proyecto cuyo único integrante es su propietario, **cuando** abre el
   dashboard, **entonces** ve la lista con una sola persona, sin que eso sea un estado especial.
8. *(Caso de error)* **Dado** un identificador de proyecto que no existe, **cuando** un integrante
   intenta abrir su dashboard, **entonces** obtiene una respuesta de proyecto inexistente.
9. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta abrir
   su dashboard, **entonces** la respuesta es idéntica a la de un proyecto inexistente y no revela
   ningún dato.

---

### Historia de Usuario 2 (US2) — Ver los indicadores clave del momento (Prioridad: P1)

Un integrante mira media docena de números y sabe si el equipo está rindiendo como siempre, si el
sprint va encaminado y si hay problemas de calidad acumulándose.

**Por qué esta prioridad**: son las cifras que disparan una conversación o una decisión. Junto con
US1 forman el mínimo útil del dashboard: con las dos, la pantalla ya sirve aunque no tenga ningún
gráfico.

**Prueba independiente**: se puede probar completa con un proyecto que tenga sprints cerrados y uno
activo, verificando que cada indicador coincide con el valor que entrega la feature de métricas.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto cuya feature de métricas informa velocidad 14,00, sprint
   activo con 18 Story Points planificados y 5 completados, 27,78 % de historias completadas, 108
   horas estimadas contra 40 reales con desviación −62,96 %, y 7 defectos abiertos, **cuando** un
   integrante abre el dashboard, **entonces** ve esos seis indicadores con exactamente esos valores,
   sin recalcular ninguno.
2. *(Caso alternativo)* **Dado** el mismo proyecto, **cuando** un integrante mira los indicadores
   del sprint activo, **entonces** los ve identificados como **parciales**, mientras que la
   velocidad no lleva esa marca.
3. *(Caso alternativo)* **Dado** un proyecto cuyo acumulado viene marcado como parcial por incluir
   al sprint activo, **cuando** un integrante lo mira, **entonces** esa marca se muestra también en
   el dashboard.
4. *(Caso límite)* **Dado** un proyecto sin sprints cerrados, cuya velocidad la feature de métricas
   informa como "no calculable", **cuando** un integrante abre el dashboard, **entonces** ve **"no
   calculable"** con una explicación breve —que todavía no hay sprints cerrados— y nunca un 0.
5. *(Caso límite)* **Dado** un sprint activo con 0 horas estimadas, cuya desviación porcentual es
   "no calculable", **cuando** un integrante lo mira, **entonces** ve ese estado con su explicación,
   y la desviación absoluta sí se muestra como número.
6. *(Caso límite)* **Dado** un proyecto sin ningún defecto, **cuando** un integrante abre el
   dashboard, **entonces** ve **0 defectos abiertos** como cifra real, no como "no calculable",
   porque contar no divide.
7. *(Caso de error)* **Dado** que la feature de métricas no puede entregar los indicadores,
   **cuando** un integrante abre el dashboard, **entonces** la sección de indicadores informa que no
   pudo obtenerse y el resto del dashboard se muestra igual.

---

### Historia de Usuario 3 (US3) — Ver la evolución de la velocidad (Prioridad: P2)

Un integrante mira cuántos Story Points planificó y completó el equipo en cada sprint y se da cuenta
de si la velocidad sube, baja o se mantiene.

**Por qué esta prioridad**: es el gráfico que convierte la velocidad de un número suelto en una
tendencia. Es útil desde el segundo sprint cerrado, pero el dashboard ya entrega valor sin él.

**Prueba independiente**: se puede probar completa con cuatro sprints cerrados de resultados
distintos, verificando que el gráfico muestra los cuatro en orden y con los valores correctos.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto cuya métrica de series entrega, para 4 sprints cerrados,
   21, 24, 13 y 25 Story Points planificados y 13, 21, 8 y 20 completados, **cuando** un integrante
   abre el dashboard, **entonces** el gráfico de velocidad muestra esos 4 pares en el orden en que
   la feature de métricas los entregó, con las dos series diferenciadas por leyenda, título y ejes
   rotulados.
2. *(Caso alternativo)* **Dado** un proyecto con 3 sprints cerrados y uno Activo, **cuando** un
   integrante mira el gráfico, **entonces** ve únicamente los 3 cerrados, porque la velocidad se
   mide sobre sprints terminados.
3. *(Caso límite)* **Dado** un proyecto con un único sprint cerrado, **cuando** un integrante mira
   el gráfico, **entonces** ve ese único punto con la indicación de que todavía no hay evolución que
   comparar.
4. *(Caso límite)* **Dado** un proyecto sin ningún sprint cerrado, **cuando** un integrante mira el
   gráfico, **entonces** ve un estado vacío que lo explica, no un gráfico con ejes en cero.
5. *(Caso límite)* **Dado** un proyecto con 20 sprints cerrados, **cuando** un integrante mira el
   gráfico, **entonces** puede identificar sin ambigüedad el valor de cualquiera de los 20 y
   distinguir a qué sprint corresponde.
6. *(Caso límite)* **Dado** un sprint cerrado que completó 0 Story Points, **cuando** un integrante
   mira el gráfico, **entonces** ese sprint aparece con valor 0, que es un dato real y no un hueco.
7. *(Caso de error)* **Dado** que la serie de velocidad no puede obtenerse, **cuando** un integrante
   abre el dashboard, **entonces** ese gráfico informa que no pudo cargarse y los demás se muestran
   igual.

---

### Historia de Usuario 4 (US4) — Ver la evolución del esfuerzo (Prioridad: P2)

Un integrante compara, sprint por sprint, las horas que se habían estimado contra las que realmente
costó, y ve si el equipo estima cada vez mejor o cada vez peor.

**Por qué esta prioridad**: es el gráfico que responde la pregunta de fondo del producto, pero
necesita varios sprints con esfuerzo cargado para decir algo. Va después de la velocidad porque esa
es la métrica que el equipo mira primero.

**Prueba independiente**: se puede probar completa con tres sprints de desviaciones distintas,
verificando que ambas series aparecen y que las cifras coinciden con las de la feature de métricas.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto con 3 sprints cuyas horas estimadas fueron 126, 144 y 108 y
   las reales 142,5, 130 y 95, **cuando** un integrante abre el dashboard, **entonces** el gráfico
   de esfuerzo muestra las dos series por sprint en orden cronológico, con leyenda, título y ejes
   rotulados.
2. *(Caso alternativo)* **Dado** el mismo proyecto con un sprint Activo en curso, **cuando** un
   integrante mira el gráfico, **entonces** el sprint activo aparece identificado como parcial, para
   que su barra más baja no se lea como una mejora de estimación.
3. *(Caso límite)* **Dado** un sprint cuyas horas estimadas son 0 y tiene horas reales registradas,
   **cuando** un integrante mira el gráfico, **entonces** la serie de estimadas muestra 0 en ese
   sprint y la de reales su valor, sin que el punto desaparezca.
4. *(Caso límite)* **Dado** un sprint sin ninguna hora registrada, **cuando** un integrante mira el
   gráfico, **entonces** la serie de reales muestra 0 en ese sprint, no un hueco.
5. *(Caso límite)* **Dado** un proyecto sin sprints con esfuerzo registrado, **cuando** un
   integrante mira el gráfico, **entonces** ve un estado vacío que lo explica.
6. *(Caso de error)* **Dado** que la serie de esfuerzo no puede obtenerse, **cuando** un integrante
   abre el dashboard, **entonces** ese gráfico informa que no pudo cargarse y los demás se muestran
   igual.

---

### Historia de Usuario 5 (US5) — Ver la calidad del producto (Prioridad: P2)

Un integrante mira cuántos defectos aparecieron y cuántos se arreglaron en cada sprint, y cómo se
reparten por gravedad los que siguen abiertos.

**Por qué esta prioridad**: es el eje de calidad, independiente del de esfuerzo. Un equipo puede ir
rapidísimo y estar acumulando defectos críticos, y sin esta sección eso no se ve.

**Prueba independiente**: se puede probar completa con defectos de distintas severidades repartidos
entre sprints, verificando el gráfico por sprint y la distribución por severidad.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto con 3 sprints en los que se detectaron 7, 4 y 9 defectos y
   se resolvieron 4, 6 y 2, **cuando** un integrante abre el dashboard, **entonces** el gráfico de
   calidad muestra las dos series por sprint en orden cronológico, con leyenda, título y ejes
   rotulados.
2. *(Caso normal)* **Dado** un proyecto con 7 defectos abiertos repartidos en 2 Críticos, 3 Altos, 1
   Medio y 1 Bajo, **cuando** un integrante abre el dashboard, **entonces** ve la distribución con
   esas cuatro cantidades y el total de 7.
3. *(Caso alternativo)* **Dado** un sprint en el que se resolvieron más defectos de los que se
   detectaron, porque venían de sprints anteriores, **cuando** un integrante mira el gráfico,
   **entonces** las dos series lo reflejan sin tratarlo como un error.
4. *(Caso límite)* **Dado** un proyecto sin ningún defecto, **cuando** un integrante mira la
   sección, **entonces** ve la distribución con 0 en las cuatro severidades y un mensaje que lo
   celebra o lo explica, no un estado de error.
5. *(Caso límite)* **Dado** un proyecto cuyos defectos abiertos son todos de una sola severidad,
   **cuando** un integrante mira la distribución, **entonces** ve esa severidad con su cantidad y
   las otras tres en 0.
6. *(Caso límite)* **Dado** un proyecto con defectos Descartados, **cuando** un integrante mira la
   sección, **entonces** esos defectos no aparecen en ningún conteo, porque la feature de métricas
   ya los excluye.
7. *(Caso de error)* **Dado** que las métricas de defectos no pueden obtenerse, **cuando** un
   integrante abre el dashboard, **entonces** la sección de calidad informa que no pudo cargarse y
   el resto se muestra igual.

---

### Historia de Usuario 6 (US6) — Usar el dashboard cuando falta información o falla una sección (Prioridad: P3)

Un integrante abre el dashboard de un proyecto recién creado, o en un momento en que una parte del
sistema no responde, y aun así entiende qué está pasando y qué puede hacer.

**Por qué esta prioridad**: es lo que distingue un dashboard usable de uno que asusta. Va último
porque solo tiene sentido una vez que las secciones existen, pero sin esta historia el primer día de
un proyecto la pantalla se ve rota.

**Prueba independiente**: se puede probar completa abriendo el dashboard de un proyecto sin sprints
y después forzando la falla de una sección, verificando que el resto sigue en pie.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto recién creado, sin sprints ni historias, **cuando** un
   integrante abre el dashboard, **entonces** ve un estado vacío que explica que todavía no hay
   datos y qué conviene hacer a continuación, en lugar de gráficos y cifras en cero.
2. *(Caso alternativo)* **Dado** un proyecto con historias en el backlog pero sin ningún sprint,
   **cuando** un integrante abre el dashboard, **entonces** el resumen y la lista de integrantes se
   muestran normalmente y las secciones que dependen de sprints indican que todavía no hay ninguno.
3. *(Caso alternativo)* **Dado** un proyecto cuyo backlog tiene 6 historias sin estimar, **cuando**
   un integrante abre el dashboard, **entonces** ve la advertencia de planificación
   correspondiente, para saber que las cifras pueden quedar cortas.
4. *(Caso alternativo)* **Dado** un proyecto en el que la feature de métricas detectó una
   inconsistencia de datos, **cuando** un integrante abre el dashboard, **entonces** ve esa
   advertencia presentada de forma distinguible de la de planificación, porque significa que hay
   información rota que alguien debe revisar, y las métricas se muestran igual.
5. *(Caso límite)* **Dado** que falla la obtención del gráfico de calidad, **cuando** un integrante
   abre el dashboard, **entonces** el resumen, los indicadores y los otros dos gráficos se muestran
   completos, y solo la sección de calidad indica que no pudo cargarse y cuál es.
6. *(Caso límite)* **Dado** que fallan dos secciones distintas, **cuando** un integrante abre el
   dashboard, **entonces** ambas lo informan por separado y las demás se muestran igual.
7. *(Caso límite)* **Dado** que una sección falló, **cuando** el integrante actualiza la pantalla y
   esa sección ya responde, **entonces** se muestra con sus datos, sin necesidad de ninguna acción
   especial.
8. *(Caso límite)* **Dado** un dashboard abierto y que mientras tanto alguien cierra el sprint
   activo, **cuando** el integrante actualiza la pantalla, **entonces** ve los datos vigentes, con
   el sprint ya cerrado y sin sprint activo.
9. *(Caso de error)* **Dado** que falla la obtención del resumen del proyecto, que es la sección que
   da contexto a todo lo demás, **cuando** un integrante abre el dashboard, **entonces** se le
   informa que el dashboard no puede mostrarse y se lo invita a reintentar, en lugar de presentar
   gráficos sin saber de qué proyecto son.

---

### Casos Límite

- **Proyecto recién creado sin sprints**: estado vacío con una indicación de qué hacer a
  continuación, en lugar de gráficos con ejes en cero que sugieren fracaso donde no hubo nada que
  hacer.
- **Proyecto con un único sprint cerrado**: los gráficos muestran ese único punto, con la indicación
  de que todavía no hay evolución que comparar.
- **Proyecto sin sprint activo**: el resumen y los gráficos se muestran normalmente; la sección del
  sprint activo informa que no hay ninguno en curso.
- **Proyecto Finalizado**: el dashboard se muestra completo, con la fecha de finalización real y sin
  sección de sprint activo. Es el momento en que más sentido tiene mirarlo.
- **Proyecto con muchos sprints**: el gráfico sigue permitiendo identificar el valor de cualquier
  sprint y a cuál corresponde, sin que las etiquetas se superpongan.
- **Sprint activo cuya fecha de fin ya pasó**: los días restantes se informan como vencidos, no como
  un número negativo suelto.
- **Sprint activo que termina hoy**: muestra 0 días restantes.
- **Valor "no calculable" en cualquier indicador**: se muestra como tal con una explicación breve,
  nunca como 0 ni como un espacio en blanco.
- **Cifra parcial del sprint activo**: se identifica como parcial, para que no se lea como un
  resultado cerrado.
- **Sprint cerrado que completó 0 Story Points**: aparece en el gráfico con valor 0, que es un dato
  real y distinto de un hueco.
- **Sprint sin horas registradas**: la serie de horas reales muestra 0 en ese sprint.
- **Proyecto sin defectos**: la distribución por severidad muestra 0 en las cuatro, no un estado de
  error ni un "no calculable".
- **Sprint con más defectos resueltos que detectados**: es válido, porque se arreglaron defectos de
  sprints anteriores; el gráfico lo muestra sin tratarlo como anomalía.
- **Falla de una sección**: el resto del dashboard se muestra completo y la sección afectada indica
  que no pudo obtenerse.
- **Falla del resumen del proyecto**: es la única falla que impide mostrar el dashboard, porque sin
  saber de qué proyecto se trata el resto no tiene sentido.
- **Cambio de datos mientras el dashboard está abierto**: la pantalla no se actualiza sola; al
  volver a ella o recargarla muestra los datos vigentes.

---

## Requisitos *(obligatorio)*

### Requisitos Funcionales

**Alcance y rol de la feature**

- **FR-001**: El dashboard DEBE presentar en una única vista el resumen del proyecto, los
  indicadores clave, los tres gráficos de evolución y la distribución de defectos abiertos por
  severidad.
- **FR-002**: El dashboard NO DEBE calcular ninguna métrica por su cuenta: toda cifra mostrada DEBE
  provenir de la feature de cálculo de métricas (`specs/008-metrics-calculation`).
- **FR-003**: El dashboard NO DEBE modificar ningún dato del producto: es una vista de solo lectura.
- **FR-004**: El dashboard NO DEBE derivar ningún valor por su cuenta, ni siquiera aritmética de
  calendario. La prohibición de FR-002 no tiene excepciones: todo dato que el dashboard muestra
  viene ya resuelto por la feature que lo posee. En particular, los días restantes del sprint activo
  los entrega `specs/004-sprint-management` junto con los demás datos del sprint, y el dashboard los
  presenta tal cual los recibe.

**Resumen del proyecto**

- **FR-005**: El dashboard DEBE mostrar el nombre del proyecto, su estado, su fecha de inicio y su
  fecha de finalización prevista.
- **FR-006**: Cuando el proyecto está Finalizado, el dashboard DEBE mostrar además su fecha de
  finalización real.
- **FR-007**: El dashboard DEBE mostrar la lista de integrantes vigentes del proyecto.
- **FR-008**: Cuando existe un sprint Activo, el dashboard DEBE mostrar su nombre, su Sprint Goal,
  sus fechas de inicio y fin previstas y sus días restantes, todos tomados de
  `specs/004-sprint-management`.
- **FR-009**: El dashboard DEBE presentar los días restantes exactamente como los entrega
  `specs/004-sprint-management`, sin recalcularlos ni reinterpretarlos. Esa feature es la que define
  que el conteo llega hasta la fecha de fin inclusive, de modo que el último día del sprint vale 0.
- **FR-010**: Cuando `specs/004-sprint-management` informa que el sprint está vencido, el dashboard
  DEBE mostrar esa condición y por cuánto, en lugar de un número negativo suelto.
- **FR-011**: Cuando el proyecto no tiene sprint Activo, el dashboard DEBE indicarlo de forma
  explícita en esa sección, sin tratarlo como un error.

**Indicadores clave**

- **FR-012**: El dashboard DEBE mostrar como indicadores clave: la velocidad del equipo, los Story
  Points planificados y completados del sprint activo, el porcentaje de historias completadas, las
  horas estimadas contra las reales con su desviación, y la cantidad de defectos abiertos.
- **FR-013**: Cada indicador DEBE mostrar exactamente el valor que entrega la feature de métricas,
  sin redondeos, recortes ni transformaciones propias.
- **FR-014**: Cuando la feature de métricas devuelve **"no calculable"**, el dashboard DEBE mostrar
  ese estado acompañado de una explicación breve del motivo, y NUNCA DEBE mostrar 0, un guion, un
  espacio en blanco ni ocultar el indicador.
- **FR-015**: Cuando la feature de métricas marca una cifra como **parcial**, el dashboard DEBE
  mostrar esa marca de forma visible junto a la cifra.
- **FR-016**: Cuando la feature de métricas marca una cifra de un sprint cerrado como
  **provisoria**, el dashboard DEBE distinguirla de una definitiva, sin confundir ese estado con
  "parcial".
- **FR-017**: El dashboard DEBE presentar un 0 real, un "no calculable" y un "no aplicable" de forma
  distinguible entre sí.

**Gráficos de evolución**

- **FR-018**: El dashboard DEBE mostrar un **gráfico de velocidad** con los Story Points
  planificados y completados de cada sprint cerrado.
- **FR-019**: El dashboard DEBE mostrar un **gráfico de esfuerzo** con las horas estimadas y las
  horas reales de cada sprint.
- **FR-020**: El dashboard DEBE mostrar un **gráfico de calidad** con los defectos detectados y los
  resueltos de cada sprint.
- **FR-021**: Los tres gráficos DEBEN ordenar los sprints cronológicamente y DEBEN incluir título,
  ejes rotulados y leyenda que identifique cada serie.
- **FR-022**: El dashboard DEBE mostrar la distribución de los defectos abiertos por severidad, con
  las cuatro severidades y el total.
- **FR-023**: Cuando una serie tiene un único punto, el gráfico DEBE mostrarlo igual, acompañado de
  la indicación de que todavía no hay evolución que comparar.
- **FR-024**: Cuando una serie está vacía, el dashboard DEBE mostrar un estado vacío explicativo en
  lugar de un gráfico con los ejes en cero.
- **FR-025**: Un valor 0 dentro de una serie DEBE representarse como el dato real que es, y DEBE ser
  distinguible de la ausencia de dato.
- **FR-026**: El dashboard DEBE obtener las seis series de sus gráficos de una única métrica de
  **series del proyecto** que entrega `specs/008-metrics-calculation`, ya armadas y ordenadas
  cronológicamente: Story Points planificados y completados por sprint, horas estimadas y reales por
  sprint, y defectos detectados y resueltos por sprint. El dashboard NO DEBE armar ninguna serie
  consultando sprint por sprint, ni reordenarlas, ni completar los puntos faltantes: la feature de
  métricas es dueña de todo lo que se grafica, incluido su orden.

**Estados vacíos, fallas y actualización**

- **FR-027**: Cuando el proyecto no tiene ningún sprint, el dashboard DEBE mostrar un estado vacío
  que explique que todavía no hay datos e indique qué hacer a continuación.
- **FR-028**: Cuando falla la obtención de una sección, el dashboard DEBE mostrar las demás
  completas y DEBE indicar en la sección afectada que no pudo obtenerse, identificándola.
- **FR-029**: Cuando fallan varias secciones, cada una DEBE informarlo por separado.
- **FR-030**: Cuando falla la obtención del resumen del proyecto, el dashboard NO DEBE mostrarse:
  sin saber de qué proyecto se trata, el resto de las cifras carece de contexto. El sistema DEBE
  informarlo e invitar a reintentar.
- **FR-031**: El dashboard NO DEBE actualizarse solo. Al volver a la pantalla o recargarla, DEBE
  reflejar los datos vigentes en ese momento.
- **FR-032**: El dashboard DEBE mostrar las advertencias que entrega
  `specs/008-metrics-calculation` (su FR-038), en sus dos clases y distinguidas entre sí:
  - **De planificación**, como la cantidad de historias del backlog sin estimar, presentada como un
    aviso dirigido al equipo sobre datos válidos pero incompletos.
  - **De inconsistencia de datos**, presentada de forma que se entienda que hay información rota que
    alguien debe revisar, no una situación normal del proyecto.
  Una advertencia NUNCA DEBE impedir que se muestren las métricas que la acompañan.

**Autorización y visibilidad**

- **FR-033**: El dashboard DEBE exigir una sesión válida.
- **FR-034**: El dashboard DEBE restringirse a los integrantes vigentes del proyecto.
- **FR-035**: Ante una solicitud sobre un proyecto al que quien pide no tiene acceso, el sistema
  DEBE responder exactamente igual que ante un identificador inexistente, sin revelar ningún dato ni
  la existencia del proyecto.
- **FR-036**: El dashboard DEBE estar disponible para los proyectos en cualquiera de sus tres
  estados, incluido Finalizado, porque consultar no es una escritura.

---

### Reglas de Negocio

| ID | Regla |
| --- | --- |
| RN-01 | El dashboard no calcula ni deriva nada: toda cifra y todo valor que muestra vienen ya resueltos por la feature que los posee. |
| RN-02 | El dashboard es de solo lectura: no modifica ningún dato del producto. |
| RN-03 | Un valor "no calculable" se muestra como tal con una explicación breve, nunca como 0. |
| RN-04 | Las cifras del sprint activo se identifican como parciales. |
| RN-05 | Una cifra provisoria de un sprint cerrado se distingue de una definitiva y no se confunde con una parcial. |
| RN-06 | Un 0 real, un "no calculable" y un "no aplicable" se presentan de forma distinguible. |
| RN-07 | Los gráficos se ordenan cronológicamente por sprint. |
| RN-08 | Todo gráfico lleva título, ejes rotulados y leyenda. |
| RN-09 | Una serie con un único punto se muestra igual, indicando que no hay evolución que comparar. |
| RN-10 | Una serie vacía se muestra como estado vacío explicativo, no como un gráfico en cero. |
| RN-11 | Un proyecto sin sprints muestra un estado vacío que indica qué hacer a continuación. |
| RN-12 | Un proyecto sin sprint activo lo informa en esa sección, sin que sea un error. |
| RN-13 | Los días restantes y la condición de vencido los entrega la feature de sprints; el dashboard los presenta tal cual. |
| RN-14 | Un sprint vencido se muestra como tal y por cuánto, en lugar de un número negativo. |
| RN-15 | La falla de una sección no impide mostrar las demás; la afectada informa que no pudo obtenerse. |
| RN-16 | La falla del resumen del proyecto sí impide mostrar el dashboard, porque es lo que da contexto al resto. |
| RN-17 | El dashboard no se actualiza solo; refleja los datos vigentes al volver a la pantalla o recargarla. |
| RN-18 | Solo los integrantes vigentes acceden al dashboard de un proyecto. |
| RN-19 | El dashboard está disponible para proyectos en cualquier estado, incluido Finalizado. |
| RN-20 | Las seis series de los gráficos llegan ya armadas y ordenadas desde la feature de métricas; el dashboard no las construye ni las reordena. |
| RN-21 | El dashboard muestra las advertencias de la feature de métricas en sus dos clases, distinguidas entre sí, y ninguna impide mostrar las métricas. |

---

### Restricciones

- **RC-01**: Esta feature depende de `specs/001-user-auth`: solo usuarios con sesión válida abren el
  dashboard.
- **RC-02**: Esta feature depende de `specs/002-project-members`: el nombre, el estado, las fechas y
  la lista de integrantes del resumen salen de allí, y la membresía vigente es el permiso de acceso.
- **RC-03**: Esta feature depende de `specs/004-sprint-management`: el sprint Activo, su Sprint
  Goal, sus fechas previstas, sus días restantes y su condición de vencido se definen y se calculan
  allí. El dashboard no crea, no modifica y no cierra sprints, y tampoco deriva ninguno de esos
  valores. Los días restantes se incorporaron a esa spec el 2026-10-03 a pedido de esta feature,
  para que la regla "el dashboard no calcula nada" no tuviera excepciones.
- **RC-04**: Esta feature depende de `specs/008-metrics-calculation` como **única fuente de las
  cifras**. El dashboard no reimplementa ninguna fórmula, ni siquiera una suma que parezca trivial:
  si lo hiciera, el producto podría mostrar una velocidad en el tablero y otra distinta en un
  reporte, que es exactamente lo que la feature de métricas existe para evitar.
- **RC-05**: El dashboard DEBE respetar los cinco estados de valor que define
  `specs/008-metrics-calculation`: parcial, provisoria, definitiva, no calculable y no aplicable.
  Aplanarlos en la presentación anularía el trabajo que esa feature hizo para distinguirlos.
- **RC-06**: Las seis series que los gráficos necesitan las entrega `specs/008-metrics-calculation`
  como una métrica propia de **series del proyecto**, incorporada a esa spec el 2026-10-03 a pedido
  de esta feature. Antes expondría una sola, la de Story Points completados, y el dashboard habría
  tenido que armar las otras cinco, que es exactamente la clase de trabajo que RC-04 busca evitar.
- **RC-07**: `specs/007-defect-tracking` excluye los defectos Descartados de todo conteo, exclusión
  que `specs/008-metrics-calculation` ya aplica. El dashboard muestra lo que recibe y no vuelve a
  filtrar.
- **RC-08**: La exportación del dashboard a PDF o a cualquier otro formato corresponde a la feature
  de reportes. Esta feature muestra en pantalla.
- **RC-09**: El dashboard no define métricas nuevas. Si hiciera falta una cifra que
  `specs/008-metrics-calculation` no produce, corresponde enmendar esa spec y no calcularla acá.
- **RC-10**: La presentación visual concreta —colores, tipos de gráfico, disposición de las
  secciones— se resuelve en la etapa de diseño. Esta especificación fija qué información se muestra
  y con qué garantías, no cómo se ve.

---

### Condiciones de Error

| Condición | Comportamiento esperado |
| --- | --- |
| Apertura del dashboard sin sesión válida | Rechazo con indicación de iniciar sesión. |
| Proyecto inexistente | Respuesta de proyecto inexistente. |
| Proyecto del que quien pide no es integrante | Respuesta idéntica a la de un proyecto inexistente, sin revelar ningún dato. |
| Falla al obtener los indicadores clave | La sección informa que no pudo obtenerse; el resto del dashboard se muestra completo. |
| Falla al obtener uno de los gráficos | Ese gráfico informa que no pudo cargarse; los demás y el resto del dashboard se muestran completos. |
| Falla al obtener varias secciones | Cada una lo informa por separado; las que sí respondieron se muestran. |
| Falla al obtener el resumen del proyecto | El dashboard no se muestra; se informa el problema y se invita a reintentar. |
| Proyecto sin sprints | No es un error: estado vacío con indicación de qué hacer a continuación. |
| Proyecto sin sprint activo | No es un error: la sección correspondiente lo informa. |
| Serie vacía o con un único punto | No es un error: se muestra el estado vacío o el punto único con su indicación. |
| Métrica con valor "no calculable" | No es un error: se muestra ese estado con su explicación breve. |

---

### Entidades Clave

- **Dashboard del proyecto**: vista compuesta y derivada, sin estado propio ni datos almacenados.
  Está formada por seis secciones: resumen del proyecto, indicadores clave, gráfico de velocidad,
  gráfico de esfuerzo, gráfico de calidad y distribución de defectos abiertos por severidad.
- **Sección del dashboard**: unidad de presentación que se obtiene y falla de forma independiente de
  las demás. Todas son degradables salvo el resumen del proyecto, que es el que da contexto al resto.
- **Serie por sprint**: secuencia ordenada cronológicamente de pares sprint-valor, que alimenta cada
  gráfico. El dashboard la ordena y la presenta; no la calcula.
- **Estado de valor** *(concepto de `specs/008-metrics-calculation`)*: parcial, provisoria,
  definitiva, no calculable o no aplicable. El dashboard los recibe y los presenta de forma
  distinguible; no los reinterpreta.
- **Proyecto**, **Integrante del proyecto** y **Sprint** *(entidades de `specs/002-project-members`
  y `specs/004-sprint-management`)*: el dashboard los consume para el resumen; no los modifica.

---

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **SC-001**: Un integrante identifica el estado del sprint activo —su objetivo, sus fechas y cuánto
  falta para que termine— en menos de 10 segundos desde que abre el dashboard.
- **SC-002**: Un integrante responde "¿el equipo va más rápido o más lento que el sprint pasado?" en
  menos de 15 segundos mirando únicamente el dashboard.
- **SC-003**: Un integrante obtiene las seis cifras clave del proyecto sin abrir ninguna otra
  pantalla, en el 100 % de los casos.
- **SC-004**: Cero cifras del dashboard difieren del valor que entrega la feature de métricas,
  verificado comparando cada indicador y cada punto de cada serie contra la fuente.
- **SC-005**: Cero valores "no calculable" mostrados como 0, como guion, como espacio en blanco o
  como indicador oculto, verificado forzando cada uno de los tres casos de división por cero que
  define la feature de métricas.
- **SC-006**: El 100 % de las cifras del sprint activo se muestra identificado como parcial, y
  ninguna cifra de un sprint cerrado se muestra con esa marca.
- **SC-007**: El 100 % de los gráficos incluye título, ejes rotulados y leyenda, verificado con una
  prueba por gráfico.
- **SC-008**: El 100 % de los gráficos presenta los sprints en orden cronológico, verificado con un
  proyecto cuyos sprints se crearon en un orden distinto al de sus fechas.
- **SC-009**: Con 20 sprints cerrados, un integrante identifica sin ambigüedad el valor de cualquier
  sprint del gráfico y a cuál corresponde, en el 100 % de los intentos.
- **SC-010**: El 100 % de las secciones distintas del resumen degrada de forma independiente,
  verificado forzando la falla de cada una por separado y comprobando que las demás se muestran
  completas.
- **SC-011**: La falla del resumen del proyecto impide mostrar el dashboard en el 100 % de los
  casos, con un mensaje que invita a reintentar.
- **SC-012**: Un proyecto sin sprints muestra un estado vacío con indicación de qué hacer, y cero
  gráficos con los ejes en cero.
- **SC-013**: Tras recargar la pantalla, el dashboard refleja los datos vigentes en el 100 % de los
  casos, verificado cerrando el sprint activo desde otra sesión y recargando.
- **SC-014**: El 100 % de los accesos exige sesión válida y membresía vigente.
- **SC-015**: Cero filtraciones de datos entre proyectos: en el 100 % de los intentos de quien no es
  integrante, la respuesta es indistinguible de la de un proyecto inexistente.
- **SC-016**: El 100 % de las reglas de negocio (RN-01 a RN-19) tiene al menos una prueba
  automatizada asociada que falla si la regla se rompe.

---

## Fuera de Alcance

- Dashboards personalizables: la composición de secciones es fija e igual para todos los
  integrantes.
- Comparación de métricas entre proyectos distintos.
- Exportación del dashboard a PDF o a cualquier otro formato, que corresponde a la feature de
  reportes.
- Gráfico de burndown.
- Actualización en tiempo real: el dashboard no se refresca solo ni avisa de cambios mientras está
  abierto.
- Definición o cálculo de métricas nuevas, que corresponde a `specs/008-metrics-calculation`.
- Alertas, umbrales o semáforos sobre los valores mostrados.
- Filtros interactivos sobre los gráficos, como acotar a un rango de sprints.
- Dashboards por integrante o métricas de rendimiento individual.
- Navegación desde el dashboard hacia el detalle de cada módulo, más allá de mostrar la información.

---

## Supuestos

- **El dashboard no deriva absolutamente nada**: decisión confirmada el 2026-10-03 (FR-004, RN-01).
  Ni siquiera la aritmética de fechas. La regla vale justamente por ser absoluta: una excepción
  "porque es trivial" es la grieta por la que después entra una suma de dos columnas, y de ahí a que
  el tablero y un reporte muestren cifras distintas hay un paso. El costo asumido fue enmendar
  `specs/004-sprint-management` para que entregue los días restantes.
- **Las series llegan armadas desde la feature de métricas**: decisión confirmada el 2026-10-03
  (FR-026, RN-20). Se incorporó a `specs/008-metrics-calculation` una métrica de series del
  proyecto que entrega las seis juntas y ordenadas. La alternativa era que el dashboard consultara
  sprint por sprint, lo que lo habría vuelto dueño del orden de los gráficos.
- **El resumen del proyecto es la única sección no degradable**: sin saber de qué proyecto son las
  cifras, mostrarlas es peor que no mostrar nada. Todas las demás secciones degradan de forma
  independiente.
- **El estado vacío de un proyecto sin sprints indica qué hacer**: el enunciado lo pide
  explícitamente. El dashboard de un proyecto recién creado es la primera pantalla que muchos
  integrantes van a ver, y una pantalla vacía sin instrucciones se interpreta como un error.
- **La composición es fija**: todos los integrantes ven las mismas seis secciones en el mismo orden,
  porque la personalización está fuera de alcance.
- **El dashboard muestra lo que recibe**: no vuelve a filtrar defectos descartados ni a excluir
  sprints, porque la feature de métricas ya aplicó esas reglas. Volver a aplicarlas sería duplicar
  lógica y arriesgarse a que las dos versiones diverjan.
- **Sin paginación ni recorte de sprints en los gráficos**: se muestran todos los sprints del
  proyecto, en línea con el criterio del resto del producto, donde la paginación está fuera de
  alcance. La legibilidad con muchos sprints se resuelve en el diseño, no recortando datos.
- **Idioma de la interfaz**: los textos, los títulos de los gráficos y las explicaciones de los
  valores no calculables están en español.

---

## Preguntas Abiertas

Ninguna. Las tres decisiones que quedaron abiertas al redactar la especificación se resolvieron el
2026-10-03 y están registradas en la sección Clarifications:

1. **El dashboard no deriva nada** (FR-004, FR-008 a FR-010, RN-01, RC-03). Los días restantes los
   entrega `specs/004-sprint-management`, que se enmendó ese mismo día.
2. **Las series llegan armadas** (FR-026, RN-20, RC-06). Las entrega una métrica de series del
   proyecto incorporada a `specs/008-metrics-calculation`, también enmendada ese día.
3. **Las advertencias se muestran** (FR-032, RN-21), en sus dos clases y distinguidas entre sí.

Las demás decisiones que podrían haber quedado abiertas se resolvieron con supuestos explícitos,
documentados en la sección Supuestos.
