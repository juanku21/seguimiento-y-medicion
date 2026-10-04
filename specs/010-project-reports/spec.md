# Especificación de Feature: Reportes de Proyecto y de Sprint

**Directorio de feature**: `specs/010-project-reports`

**Rama**: `010-project-reports`

**Creada**: 2026-10-04

**Estado**: Clarificada — sin preguntas abiertas (sesión de clarificación del 2026-10-04)

**Entrada**: Descripción del usuario: "Reportes de proyecto y de sprint con exportación a PDF para
Software Metrics & Estimation, un sistema web multiusuario para estimar, planificar, seguir y medir
proyectos de software con Scrum. Depende de: gestión de proyectos; Product Backlog; gestión de
Sprints; Planning Poker (historial de estimación); registro de esfuerzo; gestión de defectos;
cálculo de métricas (única fuente de las cifras)."

---

## Objetivo

Permitir que un integrante genere un reporte consolidado de un proyecto completo o de un sprint, lo
revise en pantalla y lo exporte a un archivo PDF para compartirlo o archivarlo.

El dashboard responde preguntas del momento: cómo venimos, qué está pasando ahora. El reporte
responde otra cosa, y por eso no alcanza con una pantalla: qué pasó, con el detalle suficiente para
defenderlo frente a alguien que no estuvo. Una retrospectiva, una entrega a un cliente, una revisión
de cátedra o una auditoría interna necesitan un documento que se pueda leer completo, fuera del
sistema y sin credenciales.

De ahí las dos propiedades que lo definen. La primera es la consolidación: el reporte junta en un
solo lugar lo que hoy vive en seis features distintas —historias, estimaciones, esfuerzo, métricas y
defectos— porque el valor está justamente en verlas juntas. La segunda es la fidelidad: el PDF tiene
que decir exactamente lo mismo que la pantalla. Un documento que se comparte y que no coincide con
el sistema del que salió es peor que no tener documento, porque discutir sobre dos versiones
distintas de los mismos hechos cuesta más que no tener ninguna.

Y como todo lo que muestra cifras en este producto, el reporte no calcula: las pide. La feature de
métricas sigue siendo la única fuente. Un reporte que sumara por su cuenta sería la tercera versión
posible de la velocidad del equipo, después del tablero y del módulo de métricas.

---

## Clarifications

### Session 2026-10-04

- Q: Un reporte de sprint cerrado puede contradecirse: las métricas salen de la instantánea y los datos de la historia, del backlog, que pudo cambiar. ¿Qué versión se usa? (FR-021) → A: la instantánea, ampliada. Se enmienda `specs/004-sprint-management` para que congele también el título y la prioridad de cada historia involucrada, de modo que un reporte de un sprint cerrado sea idéntico siempre y nunca se contradiga consigo mismo.
- Q: ¿El sistema guarda los reportes generados? (FR-034) → A: guarda solo el registro de generación —quién, cuándo y con qué alcance—, no el documento. Queda el rastro de auditoría de lo que se emitió, sin el costo de almacenar archivos.
- Q: ¿Qué pasa con las historias que nunca estuvieron en un sprint, dado que el reporte de proyecto agrupa por sprint? (FR-036) → A: se listan en un grupo propio de "historias no planificadas" al final, para que el reporte refleje todo el backlog. Es la misma solución simétrica que `specs/008-metrics-calculation` adoptó para el esfuerzo fuera de sprint.

---

## Entradas y Salidas Esperadas

### Generación del reporte de un sprint

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador de un sprint Cerrado | Reporte en pantalla con las seis secciones: encabezado, historias, estimaciones, esfuerzo, métricas y defectos |
| Sesión de un integrante e identificador del sprint Activo | El mismo reporte, marcado de forma visible como **parcial** |
| Sprint Planificado | Rechazo indicando que un sprint que todavía no arrancó no tiene nada que reportar |
| Sprint inexistente o de otro proyecto | Respuesta de sprint inexistente, sin revelar ningún dato |
| Quien pide no es integrante | Respuesta de proyecto inexistente |

### Generación del reporte de un proyecto

| Entradas | Salidas esperadas |
| --- | --- |
| Sesión de un integrante e identificador del proyecto | Reporte en pantalla con las mismas seis secciones, cubriendo todos los sprints Cerrados y el Activo si existe, con las historias agrupadas por sprint |
| Proyecto con sprint Activo | El reporte completo se marca como **parcial**, porque incluye un sprint en curso |
| Proyecto sin sprints cerrados ni activo | Reporte con el encabezado y las secciones en estado "sin datos", no un error |
| Proyecto inexistente o ajeno | Respuesta de proyecto inexistente |

### Exportación a PDF

| Entradas | Salidas esperadas |
| --- | --- |
| Reporte ya generado en pantalla y la orden de exportarlo | Archivo PDF descargable con el mismo contenido que la pantalla, con numeración de páginas y un nombre que incluye proyecto, alcance y fecha de generación |
| Falla durante la generación del archivo | Se informa el problema y el reporte en pantalla se conserva intacto, para poder reintentar sin regenerarlo |
| Reporte con muchas historias | PDF de varias páginas, todas numeradas, sin contenido cortado ni perdido |
| Nombre de proyecto con acentos o caracteres especiales | El contenido los muestra correctamente y el nombre del archivo resulta válido y legible |

---

## Escenarios de Usuario y Pruebas *(obligatorio)*

### Historia de Usuario 1 (US1) — Generar y ver el reporte de un sprint (Prioridad: P1)

Un integrante que termina un sprint genera su reporte y lo revisa en pantalla antes de llevarlo a la
retrospectiva.

**Por qué esta prioridad**: es el reporte más pedido y el que da valor con menos datos: basta un
sprint cerrado. Todo lo demás de esta feature se construye sobre él.

**Prueba independiente**: se puede probar completa cerrando un sprint con historias, esfuerzo y
defectos, generando su reporte y verificando que las seis secciones aparecen con los datos
correctos.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un sprint Cerrado con 4 historias planificadas, esfuerzo registrado y 3
   defectos, **cuando** un integrante genera su reporte, **entonces** ve las seis secciones:
   encabezado con el proyecto, el alcance "sprint", el período cubierto, la fecha y hora de
   generación y su propio nombre; historias con título, prioridad, Story Points y estado;
   estimaciones; esfuerzo por historia y por integrante con sus totales; métricas del sprint; y el
   listado de defectos con sus totales.
2. *(Caso alternativo)* **Dado** el sprint Activo del proyecto, **cuando** un integrante genera su
   reporte, **entonces** el reporte se genera igual y viene marcado de forma visible como
   **parcial**, porque el sprint todavía está en curso.
3. *(Caso alternativo)* **Dado** una historia del sprint que tuvo una sesión de Planning Poker
   Finalizada con 3 rondas y valor acordado 8, **cuando** un integrante genera el reporte,
   **entonces** la sección de estimaciones muestra esos 8 Story Points junto con el resumen: 3
   rondas y valor acordado 8.
4. *(Caso alternativo)* **Dado** una historia del sprint que nunca pasó por Planning Poker,
   **cuando** un integrante genera el reporte, **entonces** la sección de estimaciones muestra sus
   Story Points e indica que no hubo sesión de estimación, sin dejar la fila vacía.
5. *(Caso límite)* **Dado** un sprint Cerrado sin ninguna historia completada, **cuando** un
   integrante genera su reporte, **entonces** el reporte se genera con las historias planificadas
   listadas, ninguna en estado Completada, y las métricas correspondientes.
6. *(Caso límite)* **Dado** un sprint Cerrado sin ninguna historia, **cuando** un integrante genera
   su reporte, **entonces** todas las secciones que dependen de historias muestran **"sin datos"** y
   el encabezado y las métricas se muestran igual.
7. *(Caso límite)* **Dado** un sprint cuyas métricas incluyen un valor "no calculable", **cuando**
   un integrante genera el reporte, **entonces** ese valor aparece como "no calculable" con su
   explicación, nunca como 0.
8. *(Caso de error)* **Dado** un sprint en estado Planificado, **cuando** un integrante intenta
   generar su reporte, **entonces** la acción se rechaza indicando que un sprint que todavía no
   arrancó no tiene nada que reportar.
9. *(Caso de error)* **Dado** un sprint de otro proyecto, **cuando** un integrante intenta generar
   su reporte, **entonces** la respuesta es de sprint inexistente, sin revelar ningún dato del otro
   proyecto.
10. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
    generar el reporte de uno de sus sprints, **entonces** la respuesta es de proyecto inexistente.

---

### Historia de Usuario 2 (US2) — Exportar el reporte a PDF (Prioridad: P1)

Un integrante descarga el reporte que está viendo como un archivo PDF, para adjuntarlo a un correo,
subirlo a un repositorio de documentación o archivarlo.

**Por qué esta prioridad**: es la mitad del motivo por el que existe la feature. Un reporte que solo
vive en pantalla no se comparte con quien no tiene acceso al sistema, que es el caso de uso central.

**Prueba independiente**: se puede probar completa generando un reporte y exportándolo, verificando
que el archivo se descarga, que su contenido coincide con la pantalla y que su nombre cumple la
regla.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un reporte de sprint generado en pantalla, **cuando** un integrante lo
   exporta, **entonces** obtiene un archivo PDF descargable cuyo contenido coincide sección por
   sección con la pantalla, con las páginas numeradas, y cuyo nombre incluye el nombre del
   proyecto, el alcance y la fecha de generación.
2. *(Caso alternativo)* **Dado** un reporte marcado como parcial, **cuando** un integrante lo
   exporta, **entonces** el PDF también lleva esa marca de forma visible, para que no se comparta
   un documento que parezca definitivo.
3. *(Caso alternativo)* **Dado** un reporte de proyecto, **cuando** un integrante lo exporta,
   **entonces** el nombre del archivo refleja el alcance "proyecto" y lo distingue de un reporte de
   sprint del mismo proyecto y la misma fecha.
4. *(Caso límite)* **Dado** un reporte de un proyecto con 80 historias, **cuando** un integrante lo
   exporta, **entonces** el PDF abarca varias páginas, todas numeradas, sin contenido cortado,
   superpuesto ni perdido, y las tablas que cruzan de página repiten sus encabezados.
5. *(Caso límite)* **Dado** un proyecto cuyo nombre lleva acentos, eñes y caracteres especiales,
   **cuando** un integrante exporta su reporte, **entonces** el contenido del PDF los muestra
   correctamente y el nombre del archivo resulta válido para el sistema de archivos y sigue siendo
   legible.
6. *(Caso límite)* **Dado** dos reportes del mismo proyecto, el mismo alcance y el mismo día,
   **cuando** un integrante exporta los dos, **entonces** puede distinguirlos sin tener que abrirlos.
7. *(Caso de error)* **Dado** un reporte en pantalla, **cuando** falla la generación del archivo,
   **entonces** se informa el problema y el reporte en pantalla se conserva intacto, de modo que se
   pueda reintentar sin volver a generarlo.
8. *(Caso de error)* **Dado** un usuario que no es integrante del proyecto, **cuando** intenta
   exportar uno de sus reportes, **entonces** la respuesta es de proyecto inexistente.

---

### Historia de Usuario 3 (US3) — Generar y ver el reporte de un proyecto completo (Prioridad: P2)

Un integrante genera el reporte de todo el proyecto, con sus sprints uno tras otro, para una entrega
o un cierre.

**Por qué esta prioridad**: es el documento de cierre y el más completo, pero el equipo puede
trabajar sprint a sprint sin él durante buena parte del proyecto.

**Prueba independiente**: se puede probar completa con un proyecto de tres sprints cerrados y uno
activo, verificando el agrupamiento por sprint y la marca de parcialidad.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un proyecto con 3 sprints cerrados, **cuando** un integrante genera su
   reporte, **entonces** ve las historias agrupadas por sprint, en orden cronológico, y las
   secciones de estimaciones, esfuerzo, métricas y defectos del proyecto completo.
2. *(Caso alternativo)* **Dado** un proyecto con 3 sprints cerrados y uno Activo, **cuando** un
   integrante genera su reporte, **entonces** el reporte incluye los 4 sprints y se marca como
   **parcial**, porque contiene un sprint en curso.
3. *(Caso alternativo)* **Dado** un proyecto con un sprint Planificado además de los cerrados,
   **cuando** un integrante genera su reporte, **entonces** el sprint Planificado no aparece, igual
   que no se puede reportar por separado.
4. *(Caso alternativo)* **Dado** un proyecto Finalizado, **cuando** un integrante genera su reporte,
   **entonces** el reporte se genera completo y sin marca de parcial, porque ya no hay sprint en
   curso.
5. *(Caso límite)* **Dado** una historia que estuvo planificada en dos sprints, **cuando** un
   integrante genera el reporte del proyecto, **entonces** aparece en los dos grupos, con el estado
   que tuvo en cada uno, de modo que se entienda que se replanificó.
6. *(Caso alternativo)* **Dado** un proyecto con 12 historias en el backlog que nunca entraron a un
   sprint, **cuando** un integrante genera su reporte, **entonces** ve un grupo final de
   **historias no planificadas** con esas 12, después de los grupos por sprint.
7. *(Caso límite)* **Dado** un proyecto cuyas historias estuvieron todas en algún sprint, **cuando**
   un integrante genera su reporte, **entonces** el grupo de historias no planificadas se muestra
   igual, con el mensaje "sin datos".
8. *(Caso límite)* **Dado** un reporte de un sprint Cerrado cuyas historias fueron reestimadas y
   renombradas después del cierre, **cuando** un integrante lo genera, **entonces** ve el título,
   la prioridad y los Story Points que tenían al cerrarse, de modo que la sección de historias
   cuadra con la de métricas.
9. *(Caso límite)* **Dado** el mismo sprint Cerrado, **cuando** un integrante genera su reporte hoy
   y otro integrante lo genera dentro de un mes, **entonces** los dos documentos son idénticos en
   todo salvo la fecha, la hora y el nombre de quien lo generó.
10. *(Caso límite)* **Dado** un proyecto sin ningún sprint cerrado ni activo, **cuando** un
    integrante genera su reporte, **entonces** obtiene el encabezado y todas las secciones en estado
    "sin datos", sin que eso sea un error.
11. *(Caso de error)* **Dado** un identificador de proyecto que no existe, **cuando** un integrante
    intenta generar su reporte, **entonces** obtiene una respuesta de proyecto inexistente.

---

### Historia de Usuario 4 (US4) — Leer un reporte con secciones sin datos (Prioridad: P3)

Un integrante genera un reporte de algo que todavía está incompleto y entiende qué falta, en lugar
de encontrarse con un documento mutilado.

**Por qué esta prioridad**: es lo que hace que un reporte temprano siga siendo útil y creíble. Va
último porque solo tiene sentido una vez que las secciones existen.

**Prueba independiente**: se puede probar completa generando el reporte de un sprint sin esfuerzo y
sin defectos, y verificando que esas secciones aparecen con "sin datos" en lugar de desaparecer.

**Escenarios de aceptación**:

1. *(Caso normal)* **Dado** un sprint sin ninguna hora de esfuerzo registrada, **cuando** un
   integrante genera su reporte, **entonces** la sección de esfuerzo aparece con el mensaje **"sin
   datos"** y no se omite, de modo que se entienda que nadie cargó horas y no que la sección no
   existe.
2. *(Caso alternativo)* **Dado** un sprint sin defectos, **cuando** un integrante genera su
   reporte, **entonces** la sección de defectos aparece con "sin datos" y sus totales en 0.
3. *(Caso alternativo)* **Dado** un sprint cuyas historias están todas sin estimar, **cuando** un
   integrante genera su reporte, **entonces** la sección de estimaciones las lista marcadas como
   "sin estimar", y las métricas que dependen de los Story Points muestran lo que la feature de
   métricas informe.
4. *(Caso límite)* **Dado** un reporte en el que las seis secciones están sin datos, **cuando** un
   integrante lo genera, **entonces** las seis aparecen con su título y su mensaje, y el encabezado
   se muestra completo.
5. *(Caso límite)* **Dado** un reporte con secciones sin datos, **cuando** un integrante lo exporta,
   **entonces** el PDF conserva esas secciones con su mensaje, igual que la pantalla.
6. *(Caso de error)* **Dado** que una de las features de origen no puede entregar los datos de su
   sección, **cuando** un integrante genera el reporte, **entonces** esa sección informa que no
   pudo obtenerse, de forma distinguible de "sin datos", porque no tener horas cargadas y no poder
   leerlas son cosas distintas.

---

### Casos Límite

- **Sprint sin historias completadas**: el reporte se genera con las historias planificadas y
  ninguna en Completada; las métricas reflejan ese resultado.
- **Sprint sin ninguna historia**: las secciones dependientes muestran "sin datos" y el encabezado y
  las métricas se muestran igual.
- **Proyecto sin sprints cerrados**: el reporte de proyecto se genera con las secciones en "sin
  datos"; no es un error.
- **Historias sin estimar**: se listan marcadas como tales, y las métricas que dependen de los Story
  Points muestran lo que la feature de métricas informe.
- **Historias sin Planning Poker**: la sección de estimaciones indica que no hubo sesión, en lugar
  de dejar la fila vacía.
- **Historia con varias sesiones de Planning Poker**: se informa el resumen de la última sesión
  Finalizada, que es la que fijó el valor vigente.
- **Historia con una sesión de Planning Poker Cancelada y ninguna Finalizada**: se informa que no
  hay valor acordado por estimación colaborativa, aunque la historia tenga Story Points cargados a
  mano.
- **Sección sin datos**: se muestra con su título y el mensaje "sin datos", nunca se omite, para que
  quien lee sepa que la sección existe y está vacía.
- **Sección que no pudo obtenerse**: se distingue de "sin datos", porque no tener datos y no poder
  leerlos significan cosas distintas.
- **Proyecto con muchas historias**: el PDF abarca varias páginas numeradas, sin contenido cortado,
  y las tablas que cruzan de página repiten sus encabezados.
- **Nombres con acentos o caracteres especiales**: el contenido los muestra correctamente y el
  nombre del archivo resulta válido y legible.
- **Dos reportes del mismo proyecto, alcance y día**: se pueden distinguir sin abrirlos.
- **Historia planificada en dos sprints**: en el reporte de proyecto aparece en los dos grupos, con
  el estado que tuvo en cada uno.
- **Reporte de un sprint Activo**: se genera marcado como parcial, y la marca viaja al PDF.
- **Reporte de un proyecto Finalizado**: se genera completo y sin marca de parcial.
- **Falla al generar el archivo**: el reporte en pantalla se conserva intacto para reintentar.

---

## Requisitos *(obligatorio)*

### Requisitos Funcionales

**Generación del reporte**

- **FR-001**: El sistema DEBE permitir a cualquier integrante vigente generar el reporte de un
  sprint del proyecto.
- **FR-002**: El sistema DEBE permitir a cualquier integrante vigente generar el reporte del
  proyecto completo.
- **FR-003**: El sistema DEBE mostrar el reporte generado en pantalla antes de cualquier
  exportación.
- **FR-004**: El sistema DEBE rechazar la generación del reporte de un sprint en estado Planificado,
  indicando que un sprint que todavía no arrancó no tiene nada que reportar.
- **FR-005**: El reporte de un sprint Activo DEBE generarse y DEBE marcarse de forma visible como
  **parcial**.
- **FR-006**: El reporte de proyecto DEBE abarcar todos los sprints Cerrados y el Activo si existe,
  y DEBE marcarse como **parcial** cuando incluya al Activo.
- **FR-007**: El reporte de proyecto NO DEBE incluir los sprints Planificados.
- **FR-008**: El sistema NO DEBE recalcular ninguna cifra: todas provienen de
  `specs/008-metrics-calculation`.

**Contenido del reporte**

- **FR-009**: El **encabezado** DEBE incluir el nombre del proyecto, el alcance (proyecto completo o
  sprint, identificando cuál), el período cubierto, la fecha y hora de generación y el nombre del
  integrante que lo generó.
- **FR-010**: La sección de **historias** DEBE listar las historias planificadas y las completadas
  con su título, su prioridad, sus Story Points y su estado.
- **FR-011**: En el reporte de proyecto, las historias DEBEN agruparse por sprint, en orden
  cronológico.
- **FR-012**: Una historia planificada en más de un sprint DEBE aparecer en cada grupo, con el
  estado que tuvo en ese sprint.
- **FR-013**: La sección de **estimaciones** DEBE mostrar los Story Points de cada historia y,
  cuando exista una sesión de Planning Poker Finalizada, su resumen: la cantidad de rondas y el
  valor acordado.
- **FR-014**: Cuando una historia tuvo varias sesiones de Planning Poker Finalizadas, el sistema
  DEBE informar el resumen de la última, que es la que fijó el valor vigente.
- **FR-015**: Cuando una historia no tuvo ninguna sesión de Planning Poker Finalizada, el sistema
  DEBE indicarlo de forma explícita, sin dejar la fila vacía.
- **FR-016**: La sección de **esfuerzo** DEBE mostrar las horas registradas por historia y por
  integrante, con sus totales.
- **FR-017**: La sección de **métricas** DEBE incluir las métricas del sprint o del proyecto que
  define `specs/008-metrics-calculation`, con sus estados de valor intactos.
- **FR-018**: La sección de **defectos** DEBE listar los defectos con su severidad, su estado, su
  historia relacionada, su sprint de detección y su sprint de resolución, y DEBE incluir los totales
  de detectados y resueltos.
- **FR-019**: El sistema DEBE presentar los valores "no calculable" como tales, con una explicación
  breve, y NUNCA como 0.
- **FR-020**: El sistema DEBE conservar las marcas de estado que entrega la feature de métricas
  —parcial, provisoria, definitiva, no calculable y no aplicable— sin aplanarlas.
- **FR-021**: Para un sprint **Cerrado**, el sistema DEBE tomar la sección de historias
  exclusivamente de la instantánea de cierre del sprint: título, prioridad, Story Points y
  resultado de cada historia involucrada, tal como estaban al cerrarse. NO DEBE tomar ningún dato
  de esa sección del backlog vigente. Así la sección de historias y la de métricas, que también
  sale de la instantánea, nunca pueden contradecirse, y dos reportes del mismo sprint cerrado
  generados en fechas distintas son idénticos.
- **FR-022**: Para el sprint **Activo** y para las historias no planificadas, el sistema DEBE tomar
  los datos del backlog vigente, porque no existe ninguna instantánea que los congele. Esos datos
  pueden cambiar entre un reporte y el siguiente, lo que es coherente con la marca de parcial.

**Secciones sin datos y fallas parciales**

- **FR-023**: Cuando una sección no tiene datos, el sistema DEBE mostrarla con su título y el
  mensaje **"sin datos"**, y NUNCA DEBE omitirla.
- **FR-024**: Cuando una sección no puede obtenerse, el sistema DEBE informarlo de forma
  distinguible de "sin datos", porque no tener datos y no poder leerlos significan cosas distintas.
- **FR-025**: Una sección sin datos o con falla NO DEBE impedir la generación del resto del reporte.

**Exportación a PDF**

- **FR-026**: El sistema DEBE permitir exportar a un archivo PDF descargable el reporte que está en
  pantalla.
- **FR-027**: El PDF DEBE contener exactamente el mismo contenido que el reporte en pantalla,
  sección por sección, incluidas las secciones sin datos y las marcas de parcialidad.
- **FR-028**: El PDF DEBE llevar numeración de páginas.
- **FR-029**: El nombre del archivo DEBE incluir el nombre del proyecto, el alcance y la fecha de
  generación, y DEBE permitir distinguir dos reportes del mismo proyecto, el mismo alcance y el
  mismo día sin necesidad de abrirlos.
- **FR-030**: El nombre del archivo DEBE resultar válido para el sistema de archivos aun cuando el
  nombre del proyecto tenga acentos, eñes o caracteres especiales, y DEBE seguir siendo legible.
- **FR-031**: El contenido del PDF DEBE mostrar correctamente los acentos, las eñes y los
  caracteres especiales que aparezcan en los datos.
- **FR-032**: Cuando el contenido excede una página, el PDF DEBE paginarlo sin cortar, superponer ni
  perder contenido, y las tablas que cruzan de página DEBEN repetir sus encabezados.
- **FR-033**: Cuando falla la generación del archivo, el sistema DEBE informarlo y DEBE conservar
  intacto el reporte en pantalla, de modo que se pueda reintentar sin volver a generarlo.
- **FR-034**: El sistema DEBE registrar cada generación de reporte con quién la hizo, en qué momento
  y con qué alcance (proyecto completo o qué sprint). Ese **registro de generación** es lo único que
  persiste: el sistema NO DEBE almacenar el documento ni el archivo exportado. El registro DEBE
  conservarse aunque la persona que generó el reporte deje de ser integrante del proyecto, y DEBE
  seguir mostrando su nombre.
- **FR-035**: El registro de generación NO DEBE considerarse un dato del proyecto a efectos del
  estado Finalizado: generar un reporte de un proyecto Finalizado es una lectura del proyecto y
  deja su rastro de auditoría, sin que eso vulnere la condición de solo lectura que establece
  `specs/002-project-members`.

**Alcance del reporte de proyecto**

- **FR-036**: El reporte de proyecto DEBE incluir, después de los grupos por sprint, un grupo final
  de **historias no planificadas**: las historias del backlog que nunca estuvieron en ningún
  sprint, con su título, su prioridad, sus Story Points y su estado, tomados del backlog vigente
  (FR-022). Así el reporte refleja todo el backlog del proyecto y no solo lo que se trabajó. Cuando
  no hay ninguna, el grupo DEBE mostrarse con el mensaje "sin datos", igual que cualquier otra
  sección.

**Autorización y visibilidad**

- **FR-037**: El sistema DEBE exigir una sesión válida para generar y para exportar reportes.
- **FR-038**: El sistema DEBE restringir la generación y la exportación a los integrantes vigentes
  del proyecto.
- **FR-039**: Ante una solicitud sobre un proyecto o un sprint a los que quien pide no tiene acceso,
  el sistema DEBE responder exactamente igual que ante un identificador inexistente, sin revelar
  ningún dato ni la existencia del recurso.
- **FR-040**: El sistema DEBE permitir generar y exportar reportes de proyectos en cualquiera de sus
  tres estados, incluido Finalizado, porque generar un reporte no escribe nada en el proyecto.
- **FR-041**: El sistema DEBE atribuir cada reporte al integrante que lo generó y DEBE mostrar su
  nombre en el encabezado.

---

### Reglas de Negocio

| ID | Regla |
| --- | --- |
| RN-01 | Las cifras del reporte provienen de la feature de cálculo de métricas; el reporte no recalcula nada. |
| RN-02 | Generar o exportar un reporte no modifica ningún dato del proyecto; lo único que escribe es su propio registro de generación. |
| RN-03 | El reporte de un sprint Activo se marca como parcial. |
| RN-04 | El reporte de un sprint Planificado no se puede generar. |
| RN-05 | El reporte de proyecto abarca los sprints cerrados y el activo si existe, y no los planificados. |
| RN-06 | El reporte de proyecto se marca como parcial cuando incluye al sprint activo. |
| RN-07 | En el reporte de proyecto las historias se agrupan por sprint, en orden cronológico. |
| RN-08 | Una historia planificada en varios sprints aparece en cada grupo, con el estado que tuvo en cada uno. |
| RN-09 | El resumen de Planning Poker de una historia es el de su última sesión Finalizada. |
| RN-10 | Una historia sin sesión de Planning Poker Finalizada lo indica de forma explícita. |
| RN-11 | Una sección sin datos se muestra con su título y el mensaje "sin datos"; nunca se omite. |
| RN-12 | Una sección que no pudo obtenerse se distingue de una sección sin datos. |
| RN-13 | Un valor "no calculable" se muestra como tal con su explicación, nunca como 0. |
| RN-14 | Las marcas de estado que entrega la feature de métricas se conservan sin aplanarse. |
| RN-15 | El PDF tiene exactamente el mismo contenido que el reporte en pantalla. |
| RN-16 | El PDF lleva numeración de páginas y no corta ni pierde contenido al paginar. |
| RN-17 | El nombre del archivo incluye proyecto, alcance y fecha, es válido para el sistema de archivos y es legible. |
| RN-18 | Una falla al generar el archivo no destruye el reporte en pantalla. |
| RN-19 | Solo los integrantes vigentes generan y exportan reportes del proyecto. |
| RN-20 | Los reportes se pueden generar para proyectos en cualquier estado, incluido Finalizado. |
| RN-21 | Cada reporte queda atribuido a quien lo generó, y su nombre figura en el encabezado. |
| RN-22 | La sección de historias de un sprint cerrado sale íntegramente de su instantánea; dos reportes del mismo sprint cerrado son idénticos. |
| RN-23 | Los datos del sprint activo y de las historias no planificadas salen del backlog vigente y pueden cambiar entre un reporte y el siguiente. |
| RN-24 | De cada generación de reporte se guarda quién, cuándo y con qué alcance; el documento no se almacena. |
| RN-25 | El reporte de proyecto incluye un grupo final de historias no planificadas, para reflejar todo el backlog. |

---

### Restricciones

- **RC-01**: Esta feature depende de `specs/001-user-auth`: solo usuarios con sesión válida generan
  reportes, y el nombre que figura en el encabezado es el del titular de la sesión.
- **RC-02**: Esta feature depende de `specs/002-project-members`: el proyecto, sus fechas y la
  membresía vigente, que es el permiso de acceso.
- **RC-03**: Esta feature depende de `specs/003-product-backlog`: el título, la prioridad, los Story
  Points, el estado y la marca "sin estimar" de cada historia.
- **RC-04**: Esta feature depende de `specs/004-sprint-management`: los sprints, sus estados, sus
  fechas y la instantánea congelada al cierre, que es la que vuelve reproducible un reporte de un
  sprint cerrado. La instantánea se amplió el 2026-10-04 a pedido de esta feature para que congele
  también el título y la prioridad de cada historia involucrada: sin esos dos datos, la sección de
  historias habría tenido que leerlos del backlog vigente y podría contradecir a la de métricas
  dentro del mismo documento.
- **RC-05**: Esta feature depende de `specs/005-planning-poker`: el historial de sesiones de
  estimación, su cantidad de rondas y su valor acordado. Solo las sesiones Finalizadas tienen valor
  acordado; las Canceladas no.
- **RC-06**: Esta feature depende de `specs/006-effort-tracking`: las horas por historia y por
  integrante. Los registros de esfuerzo conservan el nombre de quien los cargó aunque esa persona ya
  no sea integrante, de modo que un reporte histórico sigue siendo legible.
- **RC-07**: Esta feature depende de `specs/007-defect-tracking`: los defectos con su severidad, su
  estado y sus dos sprints.
- **RC-08**: Esta feature depende de `specs/008-metrics-calculation` como **única fuente de las
  cifras**, igual que `specs/009-project-dashboard`. El reporte no reimplementa ninguna fórmula: si
  lo hiciera, el producto tendría tres versiones posibles de la misma métrica.
- **RC-09**: `specs/007-defect-tracking` y `specs/008-metrics-calculation` excluyen los defectos
  Descartados de todo conteo. Para que el listado de defectos del reporte cuadre con sus totales, el
  listado DEBE excluirlos también.
- **RC-10**: La presentación visual concreta del reporte y del PDF —tipografías, márgenes, colores,
  disposición de las tablas— se resuelve en la etapa de diseño. Esta especificación fija qué
  información aparece y con qué garantías, no cómo se ve.
- **RC-11**: Esta feature produce documentos destinados a salir del sistema. Un PDF descargado no
  tiene control de acceso: quien lo recibe lo lee completo. Por eso el reporte no incluye nada que
  no sea visible dentro del proyecto para cualquiera de sus integrantes.

---

### Condiciones de Error

| Condición | Comportamiento esperado |
| --- | --- |
| Generación o exportación sin sesión válida | Rechazo con indicación de iniciar sesión. |
| Proyecto inexistente | Respuesta de proyecto inexistente. |
| Sprint inexistente | Respuesta de sprint inexistente. |
| Sprint o proyecto de los que quien pide no es integrante | Respuesta idéntica a la de un identificador inexistente, sin revelar ningún dato. |
| Reporte de un sprint en estado Planificado | Rechazo indicando que un sprint que todavía no arrancó no tiene nada que reportar. |
| Falla al generar el archivo PDF | Se informa el problema y el reporte en pantalla se conserva intacto para reintentar. |
| Falla al obtener los datos de una sección | La sección informa que no pudo obtenerse, de forma distinguible de "sin datos"; el resto del reporte se genera. |
| Sección sin datos | No es un error: se muestra con su título y el mensaje "sin datos". |
| Métrica con valor "no calculable" | No es un error: se muestra ese estado con su explicación. |
| Proyecto sin sprints cerrados ni activo | No es un error: el reporte se genera con las secciones en "sin datos". |

---

### Entidades Clave

- **Reporte**: documento consolidado y derivado, compuesto por seis secciones —encabezado,
  historias, estimaciones, esfuerzo, métricas y defectos— con un alcance que es un sprint o el
  proyecto completo. Lleva la marca de parcial cuando incluye un sprint en curso.
- **Alcance del reporte**: valor que distingue un reporte de sprint de uno de proyecto. Determina
  qué sprints abarca, si las historias se agrupan y qué métricas se incluyen.
- **Sección del reporte**: unidad de contenido que puede estar completa, sin datos o no obtenible.
  Los tres estados son distinguibles entre sí y ninguno hace desaparecer la sección.
- **Archivo exportado**: representación del reporte en PDF, con el mismo contenido que la pantalla,
  numeración de páginas y un nombre que identifica proyecto, alcance y fecha.
- **Resumen de estimación** *(derivado de `specs/005-planning-poker`)*: cantidad de rondas y valor
  acordado de la última sesión Finalizada de una historia. No existe para historias sin sesiones
  Finalizadas.
- **Métricas** *(entidad de `specs/008-metrics-calculation`)*: el reporte las consume con sus
  estados de valor; no las calcula ni las reinterpreta.
- **Proyecto**, **Sprint**, **Historia de usuario**, **Registro de esfuerzo** y **Defecto**
  *(entidades de las specs 002, 003, 004, 006 y 007)*: el reporte las consume para armar sus
  secciones; no modifica ninguna.

---

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **SC-001**: Un integrante genera el reporte de un sprint y lo tiene en pantalla en menos de 30
  segundos desde que decide hacerlo, en no más de 3 pasos.
- **SC-002**: Un integrante exporta el reporte que está viendo en 1 paso.
- **SC-003**: El contenido del PDF coincide con el de la pantalla en el 100 % de las secciones,
  verificado comparando las seis secciones de un reporte con datos en todas ellas.
- **SC-004**: Cero cifras del reporte difieren de las que entrega la feature de métricas, verificado
  comparando cada métrica contra la fuente.
- **SC-005**: Cero valores "no calculable" mostrados como 0, verificado forzando cada caso de
  división por cero que define la feature de métricas.
- **SC-006**: El 100 % de los reportes que incluyen un sprint Activo viene marcado como parcial,
  tanto en pantalla como en el PDF.
- **SC-007**: Cero reportes generados para sprints Planificados.
- **SC-008**: El 100 % de las secciones sin datos aparece con su título y su mensaje, y ninguna se
  omite, verificado generando un reporte con las seis secciones vacías.
- **SC-009**: Una sección que no pudo obtenerse se distingue de una sección sin datos en el 100 % de
  los casos.
- **SC-010**: Un PDF de un proyecto con 80 historias conserva el 100 % del contenido, con todas las
  páginas numeradas y sin texto cortado ni superpuesto.
- **SC-011**: El 100 % de los acentos, eñes y caracteres especiales del contenido se muestra
  correctamente en el PDF, verificado con un proyecto cuyo nombre y cuyas historias los incluyan.
- **SC-012**: El nombre del archivo es válido y legible en el 100 % de los casos, incluido un
  proyecto cuyo nombre tenga caracteres especiales, y permite distinguir dos reportes del mismo
  proyecto, alcance y día.
- **SC-013**: Una falla al generar el archivo conserva el reporte en pantalla en el 100 % de los
  casos, permitiendo reintentar sin regenerarlo.
- **SC-014**: El 100 % de las generaciones y exportaciones exige sesión válida y membresía vigente.
- **SC-015**: Cero filtraciones de datos entre proyectos: en el 100 % de los intentos de quien no es
  integrante, la respuesta es indistinguible de la de un identificador inexistente.
- **SC-016**: El listado de defectos cuadra con sus totales en el 100 % de los casos, verificado en
  un proyecto que tenga defectos Descartados.
- **SC-017**: Dos reportes del mismo sprint cerrado generados en fechas distintas son idénticos en
  todo salvo la fecha, la hora y el nombre de quien los generó, verificado reestimando y
  renombrando sus historias entre una generación y la otra.
- **SC-018**: El 100 % de las generaciones de reporte queda registrada con quién, cuándo y con qué
  alcance, y cero documentos se almacenan.
- **SC-019**: El 100 % de las reglas de negocio (RN-01 a RN-25) tiene al menos una prueba
  automatizada asociada que falla si la regla se rompe.

---

## Fuera de Alcance

- Otros formatos de exportación: Excel, CSV o cualquiera que no sea PDF.
- Envío del reporte por correo electrónico desde el sistema.
- Reportes programados o generados automáticamente en fechas o eventos.
- Plantillas de reporte personalizables, con secciones que el usuario elija incluir o excluir.
- Reportes que comparen dos o más proyectos entre sí.
- Gráficos dentro del reporte, que pertenecen a la feature de dashboard.
- Firma digital, marca de agua o protección con contraseña del PDF.
- Edición del reporte antes de exportarlo.
- Vista o pantalla que permita consultar el historial de reportes generados: el registro de FR-034
  existe como rastro de auditoría, pero esta feature no define cómo se lo lee.
- Recuperación de un reporte ya emitido: el documento no se almacena, así que no se puede volver a
  descargar el archivo exacto que alguien compartió.
- Cálculo de métricas nuevas, que corresponde a `specs/008-metrics-calculation`.

---

## Supuestos

- **El reporte se genera a pedido y no en segundo plano**: el integrante lo pide, lo ve y decide si
  lo exporta. No hay colas ni avisos de "tu reporte está listo", porque no se pidieron.
- **El PDF se produce desde el reporte que ya está en pantalla**: por eso una falla al generar el
  archivo no obliga a regenerar el reporte, y por eso la fidelidad entre ambos es verificable.
- **El período cubierto de un reporte de sprint son sus fechas**; el de un reporte de proyecto va
  desde el inicio del primer sprint abarcado hasta el fin del último.
- **Solo las sesiones Finalizadas de Planning Poker aportan resumen**: una sesión Cancelada no fijó
  ningún valor, así que informarla como estimación sería engañoso. Si una historia solo tuvo
  sesiones Canceladas, se informa que no hay valor acordado por estimación colaborativa.
- **El listado de defectos excluye los Descartados**: es la única forma de que el listado cuadre con
  los totales, que la feature de métricas calcula ya excluidos.
- **El esfuerzo conserva el nombre de quien lo cargó aunque ya no sea integrante**: así lo garantiza
  `specs/006-effort-tracking`, y es lo que hace que un reporte histórico siga siendo legible.
- **Un reporte de proyecto Finalizado no es parcial**: no hay sprint en curso, así que todas sus
  cifras son definitivas.
- **El reporte no incluye información que un integrante no vería en el sistema**: un PDF descargado
  no tiene control de acceso, así que no se aprovecha el formato para mostrar de más.
- **Sin paginación ni recorte de contenido en pantalla**: el reporte se muestra completo, en línea
  con el criterio del resto del producto.
- **Idioma**: los títulos de las secciones, los mensajes de "sin datos" y las explicaciones de los
  valores no calculables están en español.

---

## Preguntas Abiertas

Ninguna. Las tres decisiones que quedaron abiertas al redactar la especificación se resolvieron el
2026-10-04 y están registradas en la sección Clarifications:

1. **Un reporte de sprint cerrado sale íntegramente de la instantánea** (FR-021, FR-022, RN-22,
   RN-23, RC-04). Se enmendó `specs/004-sprint-management` para que la instantánea congele también
   el título y la prioridad de cada historia.
2. **Se guarda el registro de generación, no el documento** (FR-034, FR-035, RN-02, RN-24).
3. **El reporte de proyecto incluye un grupo de historias no planificadas** (FR-036, RN-25).

Las demás decisiones que podrían haber quedado abiertas se resolvieron con supuestos explícitos,
documentados en la sección Supuestos.
