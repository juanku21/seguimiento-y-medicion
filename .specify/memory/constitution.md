<!--
SYNC IMPACT REPORT
==================
Cambio de versión: 2.2.0 → 2.3.0
Tipo de bump: MINOR (las reglas de formato de los mensajes de commit pasan a
estar contenidas en la constitución y se elimina la dependencia de guías
externas al repositorio; no se elimina ni se redefine ninguna regla existente).

Historial:
  - 1.0.0 (2026-09-19): ratificación original del documento.
  - 1.1.0 (2026-09-19): enmienda de los Principios III y VIII.
  - 1.2.0 (2026-09-20): enmienda del Principio IX (gestión de issues vía MCP de
    GitHub bajo condiciones estrictas), del árbol de directorios y del Flujo de
    Trabajo de Desarrollo.
  - 2.0.0 (2026-10-04): reorganización del Principio IX en seis ámbitos; se
    permite proponer mensajes de commit y abrir pull requests con confirmación
    humana, y se exige que toda escritura hacia GitHub pase por una skill
    declarada.
  - 2.1.0 (2026-10-04): obligatoriedad de pruebas unitarias en métricas,
    estimación, reglas de negocio y validaciones; evidencia del ciclo TDD en
    commits separados; delimitación de la prohibición del ámbito a) al
    repositorio Git con autorización explícita sobre la copia de trabajo local;
    reglas de incorporación de skills de terceros.
  - 2.2.0 (2026-10-04): convención de BDD que liga cada escenario Given/When/Then
    de una spec a una prueba automatizada con nombre trazable, y tabla de
    dependencias autorizadas con sus exclusiones.
  - 2.3.0 (2026-10-05): el formato de los mensajes de commit se incorpora
    completo al Principio IX, ámbito b); se eliminan las referencias a la guía
    externa de commits y la entrada `docs/` del árbol de directorios, y el árbol
    incorpora `.specify/` y `specs/`.

Principios modificados en 2.3.0:
  - II. Desarrollo Guiado por Pruebas (NO NEGOCIABLE):
    - La regla de evidencia del ciclo TDD ya no remite a una guía externa: los
      commits de RED, GREEN y REFACTOR se registran según el formato de commits
      del Principio IX, ámbito b). El resto de la regla no cambia.
  - IX. Gobierno del Repositorio y Límites de Agentes de IA:
    - Ámbito b), punto 3: la remisión a una guía externa de commits se reemplaza
      por el formato completo de los mensajes de commit, que pasa a vivir en la
      constitución: Conventional Commits con primera línea
      `tipo(ámbito): descripción` de 72 caracteres o menos; tipos `feat`, `fix`,
      `test`, `refactor`, `docs`, `style` y `chore`; ámbito del módulo de la spec
      o transversal (`infra`, `deps`, `repo`); descripción en español, en
      minúsculas, sin punto final y con el verbo en presente; pie
      `Spec: NNN/Txxx` en todo commit derivado de una tarea de `tasks.md`, con
      `Closes #N` si la termina o `Refs #N` si es intermedio, más `TDD: red`,
      `TDD: green` o `TDD: refactor` en reglas de negocio, cálculos y
      validaciones; ámbito `repo` y sin esos pies para la configuración del
      repositorio que no surge de una spec; y un commit por tarea. Los puntos 1
      y 2 del ámbito b) no cambian.

Secciones modificadas en 2.3.0:
  - Restricciones Técnicas, "Estructura del repositorio": se elimina del árbol de
    directorios la entrada `docs/` con su archivo de guía de commits, y se
    incorporan `.specify/` (configuración de Spec Kit, con `memory/`, donde vive
    este documento) y `specs/` (una carpeta por feature), que el documento ya
    referenciaba sin listarlas.

Secciones añadidas: ninguna
Secciones eliminadas: ninguna

Principios modificados en 2.2.0:
  - II. Desarrollo Guiado por Pruebas (NO NEGOCIABLE):
    - Nueva viñeta de BDD: cada escenario Given/When/Then de una spec DEBE
      implementarse como al menos una prueba automatizada cuyo nombre identifique
      la historia y el escenario, sin incorporar herramientas de BDD. Fija la
      convención por stack: en Go, función `TestUSn_<Escenario>` en el paquete
      del módulo precedida por el comentario
      `// Escenario: <nombre> (NNN/USn)`, con las variantes agrupadas como
      subtests; en Vitest, un `describe("NNN/USn - <título>")` con un `it` por
      escenario. La trazabilidad DEBE poder verificarse buscando `NNN/USn` en el
      código de pruebas.
    - Justificación ampliada con la exigencia de BDD de la cátedra.
  - IV. Stack Fijo y Versiones Estables:
    - Nueva subsección "Dependencias autorizadas": tabla de once entradas que
      concede la autorización previa que el principio exige, por nombre y
      propósito, dejando la versión exacta al plan de cada feature bajo la regla
      de versiones estables. Cubre backend (driver de PostgreSQL para GORM, JWT,
      bcrypt, UUID, Swagger, CORS), pruebas de backend (testify), frontend
      (TypeScript, ESLint, Prettier), pruebas de frontend (vitest y la cadena de
      Testing Library que indica la guía oficial de Next.js) y la herramienta de
      desarrollo govulncheck.
    - Dos viñetas de cierre: exclusiones por aplicación del Principio I (clientes
      HTTP de terceros, cargadores de variables de entorno, contenedores de
      prueba programáticos y herramientas de migración externas) y diferimiento
      de las dependencias de WebSockets (spec 005), gráficos (spec 009) y
      generación de PDF (spec 010) al momento de planificar esas specs.
    - La tabla del stack obligatorio, la regla de versiones estables y el párrafo
      de autorización previa no cambian.

Secciones modificadas en 2.2.0: ninguna
Secciones añadidas: ninguna
Secciones eliminadas: ninguna

Principios modificados en 2.1.0:
  - II. Desarrollo Guiado por Pruebas (NO NEGOCIABLE):
    - Las pruebas de integración se definen como verificación de punta a punta
      (entrada HTTP, servicio y persistencia contra la base de test).
    - Las pruebas unitarias pasan de excepcionales a OBLIGATORIAS en cuatro
      dominios: cálculo de métricas, estimación (conversión de story points a
      horas, velocidad y proyecciones), reglas de negocio (transiciones de
      estado, permisos por rol y restricciones de dominio) y validaciones de
      entrada. Fuera de esos casos se mantiene el criterio anterior de lógica
      considerablemente compleja.
    - Nueva regla de evidencia: cada etapa del ciclo (RED, GREEN, REFACTOR) se
      registra en un commit separado hecho por una persona según
      `docs/guia-commits.md`, y el agente que aplica el ciclo se detiene tras
      verificar que la prueba falla.
    - Justificación ampliada con el criterio de evaluación de la cátedra.
  - IX. Gobierno del Repositorio y Límites de Agentes de IA:
    - Ámbito a) renombrado: "Código e historial de Git" → "Repositorio Git e
      historial". La prohibición se delimita al repositorio Git y su historial
      (registrar cambios, crear o modificar ramas, integrar código a `main`) y
      se agrega un párrafo que autoriza a los agentes a crear, modificar y
      revisar archivos en la copia de trabajo local del desarrollador y a
      ejecutar pruebas, formateadores, compilaciones y el entorno de test; esos
      cambios llegan al repositorio solo cuando una persona los registra en un
      commit. La lista de comandos prohibidos, las operaciones de lectura
      permitidas y la aplicación técnica en `.claude/settings.json` no cambian.
    - Ámbito f) ampliado con tres reglas: incorporación de skills de terceros
      (copia de carpetas seleccionadas en `.claude/skills/` desde un commit fijo
      del repositorio de origen, con su licencia, revisión humana previa del
      contenido y registro del origen y las adaptaciones en el mensaje del
      commit que la incorpora o la actualiza; prohibidos los plugins y las ramas
      móviles por aplicación del Principio IV); precedencia de esta constitución
      ante cualquier skill, propia o de terceros; y obligación de configurar las
      herramientas para no generar archivos de instrucciones agénticas
      (`agentRules: false` en `next.config` y `--no-agents-md` al crear el
      proyecto Next.js).
    - Justificación: "el código, el índice y el historial" → "el repositorio, su
      índice y su historial", para no contradecir la autorización nueva.

Secciones modificadas en 2.1.0: ninguna
Secciones añadidas: ninguna
Secciones eliminadas: ninguna

Principios modificados en 1.1.0:
  - III. Arquitectura por Capas: el backend fija `cmd/app/` como punto de
    entrada e `internal/<módulo>/` como raíz de los módulos con sus capas.
  - VIII. Estándares de Código y Documentación Viva: se retira la regla de
    indentación de dos tabulaciones; el formateo pasa a regirse por el
    estándar oficial de cada lenguaje (`gofmt` en Go, convenciones oficiales
    de TypeScript en el frontend).

Principios definidos en 1.0.0 (ninguno previo existía):
  - I. Simplicidad Deliberada (YAGNI, KISS, DRY, SOLID)
  - II. Desarrollo Guiado por Pruebas (NO NEGOCIABLE)
  - III. Arquitectura por Capas
  - IV. Stack Fijo y Versiones Estables
  - V. Contenerización y Aislamiento de Entornos
  - VI. Seguridad y Gestión de Secretos
  - VII. Robustez y Validación de Entradas
  - VIII. Estándares de Código y Documentación Viva
  - IX. Gobierno del Repositorio y Límites de Agentes de IA

Plantillas dependientes: no modificadas (leen la constitución en tiempo de
ejecución).

Seguimiento manual requerido:
  - `CLAUDE.md` de la raíz referencia "Principio IX, ámbitos b y d" para la
    exclusividad de la revisión y el merge: desde 2.0.0 esa regla vive en el
    ámbito c. La referencia a "ámbito a" sigue siendo correcta. La corrección la
    hace una persona.
  - `CLAUDE.md` describe la prohibición del ámbito a) como escritura "en el
    repositorio de código o en su índice"; conviene alinearla con la
    delimitación de 2.1.0 (repositorio Git e historial) para que no se lea como
    una prohibición de editar archivos de la copia de trabajo.
  - El formato de commits quedó centralizado en el ámbito b) del Principio IX,
    pero cuatro artefactos siguen remitiendo a una guía externa del equipo:
    `CLAUDE.md` de la raíz (sección "Commits"),
    `.claude/skills/redactar-commit/SKILL.md`,
    `.claude/skills/redactar-pr/SKILL.md` y
    `.github/pull_request_template.md`. Deben apuntar al Principio IX, ámbito
    b). La corrección la hace una persona.
  - `specs/001-user-auth/plan.md` y `specs/001-user-auth/research.md` registran
    la ausencia de la guía de commits como bloqueo del primer commit RED; ese
    bloqueo desaparece con 2.3.0.
  - El rol "Agile Enabler" queda nombrado en el ámbito c) como responsable
    humano de la revisión y el merge.
  - `.specify/templates/plan-template.md` no menciona la tabla de dependencias
    autorizadas que incorpora 2.2.0, y es el plan de cada feature el que fija la
    versión exacta de cada dependencia. Conviene revisar si el plan debe
    referenciarla de forma explícita.

TODO pendientes: ninguno
-->

# Constitución de Software Metrics & Estimation

## Principios Fundamentales

### I. Simplicidad Deliberada (YAGNI, KISS, DRY, SOLID)

El alcance implementado DEBE limitarse a las funcionalidades explícitamente solicitadas: no se
construye infraestructura, abstracción ni configuración "por si acaso" (YAGNI). La lógica DEBE
resolverse con la solución más simple que satisfaga el requisito (KISS); si existen dos soluciones
correctas, se elige la que tenga menos piezas móviles. La duplicación de lógica DEBE eliminarse
extrayendo la responsabilidad compartida a un único lugar (DRY), sin crear abstracciones
prematuras que violen YAGNI. La organización del proyecto y del código DEBE respetar los
principios SOLID, en particular responsabilidad única y dependencia hacia abstracciones.

**Justificación**: el proyecto es un sistema de medición cuyo valor está en los indicadores que
produce, no en su andamiaje. Cada pieza no solicitada es deuda que hay que testear, documentar y
mantener.

### II. Desarrollo Guiado por Pruebas (NO NEGOCIABLE)

Para cada funcionalidad, la prueba DEBE escribirse antes que el código de producción. El ciclo
obligatorio es: escribir la prueba → verificar que falla → implementar → verificar que pasa →
refactorizar. Está prohibido escribir código de producción sin una prueba que lo justifique.

- Las pruebas de integración son el mecanismo por defecto para verificar cada funcionalidad de
  punta a punta (entrada HTTP, servicio y persistencia contra la base de test).
- Las pruebas unitarias son OBLIGATORIAS para el cálculo de métricas; la estimación (conversión de
  story points a horas, velocidad y proyecciones); las reglas de negocio (transiciones de estado,
  permisos por rol y restricciones de dominio) y las validaciones de entrada. Fuera de esos casos,
  se escriben solo ante lógica considerablemente compleja que requiera verificación aislada.
- Las pruebas DEBEN ejecutarse contra servicios y bases de datos de test, nunca contra el entorno
  de desarrollo (ver Principio V).
- El backend y el frontend DEBEN exponer cada uno un comando documentado para generar el reporte
  de cobertura de pruebas.
- Cada etapa del ciclo con evidencia (RED, GREEN, REFACTOR) queda registrada en un commit
  separado, hecho por una persona según el formato de commits del Principio IX, ámbito b). Un
  agente que aplica el ciclo se detiene después de verificar que la prueba falla, para que se
  registre el commit RED antes de implementar.
- Cada escenario Given/When/Then de una spec DEBE implementarse como al menos una prueba
  automatizada cuyo nombre identifique la historia y el escenario, sin herramientas de BDD
  adicionales:
  - Backend (Go): función `TestUSn_<Escenario>` en el paquete del módulo (por ejemplo,
    `TestUS1_CorreoDuplicado`), precedida por el comentario
    `// Escenario: <nombre del escenario en spec.md> (NNN/USn)`. Las variantes de un mismo escenario
    se agrupan como subtests (`t.Run`).
  - Frontend (Vitest): bloque `describe("NNN/USn - <título de la historia>")` con un
    `it("<nombre del escenario en spec.md>")` por escenario.
  - La trazabilidad escenario → prueba DEBE poder verificarse buscando `NNN/USn` en el código de
    pruebas.

**Justificación**: un sistema que mide la calidad de otros proyectos pierde credibilidad si no
puede demostrar la suya. La cobertura medible es el registro de qué se está probando realmente. La
cátedra evalúa pruebas unitarias en métricas, estimación, reglas de negocio y validaciones, y la
evidencia de TDD en el historial del repositorio. La cátedra exige BDD; la convención de nombres
convierte cada escenario de la spec en una prueba ejecutable y rastreable sin sumar dependencias.

### III. Arquitectura por Capas

Backend y frontend DEBEN separarse en directorios de primer nivel en la raíz del repositorio
(`backend/` y `frontend/`), sin mezcla de artefactos entre ambos.

- **Backend**: organización vertical por módulo o feature siguiendo el layout estándar de Go.
  - `backend/cmd/app/` contiene el archivo que es el punto de entrada de la aplicación. Este
    paquete solo compone dependencias y arranca el servidor: no contiene lógica de negocio.
  - `backend/internal/<módulo>/` es la raíz de cada módulo o feature. Cada módulo DEBE contener
    sus propias capas: `delivery/` (entrada HTTP), `service/` (lógica de negocio), `repository/`
    (persistencia) y `domain/` (entidades e interfaces).
  - Las dependencias apuntan hacia `domain/`, nunca al revés.
- **Frontend**: bajo `src/` DEBEN existir las capas `app/`, `features/`, `components/`, `config/`,
  `hooks/`, `lib/`, `providers/`, `styles/`, `types/` y `utils/`, cada una con una responsabilidad
  única y explícita.
- Las interfaces de repositorio de la capa de dominio DEBEN declarar únicamente las operaciones
  que se usan. Está prohibido generar CRUD completo especulativo (aplicación directa del
  Principio I).

**Justificación**: la separación por responsabilidades hace que el impacto de un cambio sea
localizable y que las pruebas de integración puedan sustituir capas concretas.

### IV. Stack Fijo y Versiones Estables

El stack tecnológico está definido y no se sustituye sin enmienda de esta constitución:

| Ámbito | Tecnología obligatoria |
| --- | --- |
| Framework backend | Gin (Go) |
| ORM backend | GORM |
| Base de datos relacional | PostgreSQL |
| Framework frontend | Next.js |
| Estilos frontend | Tailwind CSS |
| Autenticación | JWT |
| Hash de contraseñas | bcrypt |
| Orquestación de servicios | Docker / Docker Compose |
| Documentación de API | Swagger |

Toda imagen de Docker, librería, dependencia, runtime o framework DEBE fijarse en una versión
estable publicada; están prohibidas las versiones alpha, beta, release candidate y las etiquetas
móviles del tipo `latest`.

Instalar una dependencia externa no listada en esta tabla ni exigida explícitamente por el
requisito en curso REQUIERE autorización previa y explícita del responsable humano, acompañada de
una justificación que indique por qué se necesita y para qué se usará.

**Dependencias autorizadas**

Las siguientes dependencias cuentan con la autorización previa exigida por este principio. Se
autoriza el nombre y el propósito; la versión exacta la fija el plan de cada feature, respetando la
regla de versiones estables.

| Ámbito | Dependencia | Propósito |
| --- | --- | --- |
| Backend | gorm.io/driver/postgres | Conexión de GORM con PostgreSQL |
| Backend | github.com/golang-jwt/jwt/v5 | Emisión y validación de JWT |
| Backend | golang.org/x/crypto (bcrypt) | Hash de contraseñas |
| Backend | github.com/google/uuid | Identificadores UUID |
| Backend | github.com/swaggo/swag, github.com/swaggo/gin-swagger, github.com/swaggo/files | Generación y publicación de Swagger |
| Backend | github.com/gin-contrib/cors | CORS entre frontend y API |
| Backend (pruebas) | github.com/stretchr/testify | Aserciones y mocks en pruebas |
| Frontend | TypeScript y ESLint (incluidos por create-next-app) | Tipado y análisis estático |
| Frontend | Prettier | Formateo del código |
| Frontend (pruebas) | vitest, @vitejs/plugin-react, jsdom, @testing-library/react, @testing-library/dom y los paquetes que indique la guía oficial de Next.js para Vitest | Pruebas de componentes |
| Herramienta de desarrollo | golang.org/x/vuln/cmd/govulncheck | Detección de vulnerabilidades conocidas en dependencias de Go |

- Quedan excluidas por el Principio I, salvo nueva autorización: clientes HTTP de terceros (se usa
  `fetch`), cargadores de variables de entorno (se usa la librería estándar), contenedores de prueba
  programáticos (se usa Docker Compose, Principio V) y herramientas de migración externas (se usa
  `AutoMigrate` de GORM, salvo justificación en el plan).
- Las dependencias de WebSockets (spec 005), gráficos (spec 009) y generación de PDF (spec 010) se
  autorizan al planificar esas specs.

**Justificación**: fijar el stack y las versiones elimina decisiones repetidas, evita
incompatibilidades sorpresivas y mantiene reproducible cualquier entorno.

### V. Contenerización y Aislamiento de Entornos

Toda base de datos y todo servicio externo de calibre equivalente DEBE levantarse con Docker
Compose usando una versión estable; no se admiten instalaciones manuales en la máquina anfitriona.

- Cada servicio externo DEBE tener su propio directorio dentro del proyecto que lo consume, con
  su propio `docker-compose`, `.env`, `.env.example` y `.env.test`. Ejemplo: `backend/postgres/`.
- Por cada base de datos del proyecto DEBE existir un contenedor de test equivalente, levantado
  también mediante Docker Compose.
- Los entornos de test se configuran exclusivamente con archivos `.env.test` separados, tanto para
  los servicios como para las aplicaciones backend y frontend, de modo que el entorno de pruebas
  jamás toque datos ni puertos del entorno de desarrollo.
- Las aplicaciones backend y frontend DEBEN encapsularse cada una en su `Dockerfile` versionado,
  de forma que puedan ejecutarse de manera portable en cualquier entorno.

**Justificación**: un entorno reproducible con un comando elimina la clase entera de fallos "en mi
máquina funciona" y permite que las pruebas de integración corran contra datos desechables.

### VI. Seguridad y Gestión de Secretos

Contraseñas de bases de datos, claves de encriptación, secretos de firma JWT, puertos y cualquier
otro dato sensible DEBEN residir en archivos `.env`, nunca embebidos en el código ni versionados.
Cada `.env` DEBE tener su `.env.example` con las mismas claves y valores de ejemplo no sensibles.

- La autenticación de usuarios se resuelve con JWT.
- Las contraseñas almacenadas en base de datos DEBEN estar hasheadas con bcrypt; está prohibido
  guardar o registrar contraseñas en texto plano.
- El archivo `.gitignore` DEBE excluir archivos `.env`, dependencias, configuraciones innecesarias
  y todo artefacto reproducible mediante comandos. Sus entradas DEBEN estar agrupadas con
  comentarios `#Frontend` y `#Backend`, y DEBEN actualizarse cada vez que aparezca un nuevo
  artefacto no versionable.

**Justificación**: un secreto filtrado en el historial de Git es irrecuperable; la separación
entre configuración y código es la única barrera efectiva.

### VII. Robustez y Validación de Entradas

Todo error en tiempo de ejecución DEBE manejarse explícitamente para que la aplicación nunca caiga
por un fallo no controlado. En Go, cada `error` devuelto DEBE inspeccionarse y propagarse o
traducirse a una respuesta HTTP adecuada; están prohibidos los `panic` no recuperados en el camino
de una petición.

Toda entrada que llegue al backend DEBE pasar por validación de tipos de datos y de reglas de
negocio antes de alcanzar la capa de servicio. Una entrada inválida produce una respuesta de error
descriptiva, nunca un fallo interno.

**Justificación**: el sistema es de registro y seguimiento; una caída no controlada puede hacer
perder esfuerzo ya cargado por el equipo.

### VIII. Estándares de Código y Documentación Viva

- **Idioma del código**: todo identificador (nombres de variables, funciones, tipos, archivos,
  endpoints) DEBE estar escrito en inglés.
- **Idioma de la documentación**: comentarios, `README.md` y documentación Swagger DEBEN estar en
  español.
- **Comentarios**: cada bloque de código DEBE llevar un comentario breve en español explicando qué
  hace o cómo funciona. Se comenta el bloque, NO cada línea.
- **Formateo**: el formateo del código (indentación incluida) lo determina el estándar oficial de
  cada lenguaje y su herramienta de formateo automático; no se definen reglas propias que compitan
  con ellas. El código entregado DEBE estar formateado con esa herramienta.
- **Go**: se adoptan las convenciones oficiales del lenguaje (camelCase, nombres cortos para
  variables de ámbito reducido, acrónimos con capitalización uniforme en todo el nombre como `ID`,
  `HTTP`, `URL`).
- **TypeScript**: se adoptan las convenciones oficiales del lenguaje. El tipo `any` está
  terminantemente prohibido; se usan tipos explícitos, genéricos o `unknown` con reducción.
- **Swagger**: toda la API del backend DEBE estar documentada con Swagger, en español, y la
  documentación se actualiza en el mismo cambio que modifica el endpoint.
- **README**: el `README.md` de la raíz DEBE mantenerse actualizado con el paso a paso para
  ejecutar la aplicación. La sección introductoria que explica el proyecto NO se elimina: se
  actualiza a medida que el proyecto crece, igual que los pasos de ejecución. El criterio de
  aceptación es que alguien que no conoce el proyecto pueda ejecutarlo siguiendo el documento.

**Justificación**: el código en inglés es portable entre herramientas y colaboradores; la
documentación en español es la lengua de trabajo del equipo. Una documentación que se actualiza en
el mismo cambio que el código nunca miente.

### IX. Gobierno del Repositorio y Límites de Agentes de IA

**a) Repositorio Git e historial — prohibición total para agentes**

Ningún agente de IA (no humano) puede ejecutar operación de escritura alguna sobre el repositorio
Git ni sobre su historial: registrar cambios, crear o modificar ramas, ni integrar código a `main`.
La prohibición se mantiene aunque una persona la pida: no es una preferencia revocable en
conversación, es un límite del proyecto. Alcanza por igual a:

- Los comandos de Git que escriben: `add`, `commit`, `branch` con argumentos, `checkout`, `switch`,
  `merge`, `rebase`, `push`, `pull`, `fetch`, `stash`, `tag`, `reset`, `restore`, `revert`,
  `cherry-pick` y cualquier otro equivalente.
- Las APIs y herramientas MCP que producen el mismo efecto: crear ramas; crear, modificar o
  eliminar archivos remotos; hacer push; y mergear, aprobar, cerrar o modificar pull requests.

Un agente SOLO puede ejecutar operaciones de lectura sobre el repositorio: `status`, `diff`, `log`,
`rev-parse`, `rev-list`, `branch --show-current` y `remote get-url`.

Esta frontera DEBE estar además aplicada técnicamente: `.claude/settings.json` deniega los comandos
de escritura de Git, de modo que la regla no dependa únicamente de la obediencia del agente.

Los agentes PUEDEN crear, modificar y revisar archivos en la copia de trabajo local del
desarrollador, y ejecutar pruebas, formateadores, compilaciones y el entorno de test. Esos cambios
llegan al repositorio solo cuando una persona los revisa y los registra en un commit.

**b) Mensajes de commit — propuesta sin ejecución**

Un agente PUEDE redactar y proponer mensajes de commit, y solo eso, cumpliendo TODAS las
condiciones siguientes, que son acumulativas:

1. La propuesta se produce únicamente a pedido de una persona y a través de la skill dedicada
   (`/redactar-commit`); un agente no redacta mensajes de commit por iniciativa propia.
2. La propuesta se entrega como texto en la conversación. La persona la revisa, la modifica si hace
   falta y ejecuta el commit ella misma.
3. El formato de los mensajes de commit, que es la referencia obligatoria de toda propuesta, es el
   siguiente:
   - Los commits siguen Conventional Commits: la primera línea es `tipo(ámbito): descripción` y
     DEBE tener 72 caracteres o menos.
   - Los tipos permitidos son `feat`, `fix`, `test`, `refactor`, `docs`, `style` y `chore`.
   - El ámbito es el módulo de la spec (`auth`, `proyectos`, `backlog`, `sprints`, `poker`,
     `esfuerzo`, `defectos`, `metricas`, `dashboard`, `reportes`) o uno transversal (`infra`,
     `deps`, `repo`).
   - La descripción se escribe en español, en minúsculas, sin punto final y con el verbo en
     presente.
   - Todo commit derivado de una tarea de `tasks.md` DEBE llevar el pie `Spec: NNN/Txxx` y, además,
     `Closes #N` si termina la tarea o `Refs #N` si es intermedio, donde `#N` es la sub-issue de la
     tarea. En reglas de negocio, cálculos y validaciones DEBE llevar también `TDD: red`,
     `TDD: green` o `TDD: refactor`.
   - Los cambios de configuración del repositorio que no surgen de una spec usan el ámbito `repo` y
     no llevan esos pies.
   - Cada commit corresponde a una sola tarea.

**c) Pull requests — apertura con confirmación; revisión y merge humanos**

Un agente PUEDE redactar y abrir pull requests cumpliendo TODAS las condiciones siguientes, que son
acumulativas:

1. La redacción y la apertura se producen únicamente a pedido de una persona y a través de la skill
   dedicada (`/redactar-pr`).
2. El pull request se abre desde la rama de la historia de usuario o de la fase técnica y siempre
   hacia `main`.
3. Antes de abrirlo, el agente DEBE mostrar el título y el cuerpo completos y esperar la
   confirmación explícita de una persona.
4. Está prohibido mergear, aprobar, cerrar o modificar un pull request, y también solicitar
   revisores. La revisión, la aprobación y el merge son exclusivos del Agile Enabler, el rol humano
   responsable de la integración en `main`.

**d) Gestión de issues del tablero — permitida bajo condiciones**

Un agente PUEDE crear, editar, etiquetar, comentar y cerrar issues, vincular sub-issues,
reordenarlos o cambiarlos de padre, y crear etiquetas del repositorio, únicamente a través del
servidor MCP de GitHub y cumpliendo TODAS las condiciones siguientes, que son acumulativas:

1. La fuente de verdad son los archivos de `specs/`. La sincronización es unidireccional, de
   `specs/` hacia GitHub: un agente jamás modifica una spec, un plan ni un `tasks.md` a partir del
   contenido de un issue.
2. Antes de escribir en GitHub, el agente DEBE presentar la lista completa de cambios y esperar la
   aprobación explícita de una persona.
3. Está prohibido eliminar issues, comentarios o etiquetas. Un issue obsoleto se cierra como "no
   planificado" con un comentario que explica el motivo.
4. Los campos Valor, Prioridad y Estimación, los milestones y las asignaciones los gestiona
   exclusivamente el equipo humano; el agente no los completa ni los modifica.
5. El servidor MCP de GitHub DEBE configurarse exponiendo solo las herramientas de issues, de
   etiquetas y de apertura de pull requests, y sus credenciales nunca se versionan (aplicación del
   Principio VI).

**e) Canal único de escritura: skills y subagentes**

Toda escritura permitida hacia GitHub pasa por un punto de entrada declarado y auditable:

- La publicación o actualización de issues a partir de `specs/` se hace únicamente con la skill
  `/sincronizar-github`, que delega en el subagente `sync-github`.
- La conversación principal no invoca las herramientas de escritura de issues ni de etiquetas del
  servidor MCP de GitHub. Su única herramienta de escritura es la de apertura de pull requests
  (`create_pull_request`), y solo dentro de la skill `/redactar-pr`.
- Si una persona pide en lenguaje natural redactar un commit, abrir un pull request o sincronizar
  con GitHub, el agente no lo hace por cuenta propia: indica qué skill corresponde ejecutar
  (`/redactar-commit`, `/redactar-pr` o `/sincronizar-github`).

**f) Artefactos de instrucciones y configuración agéntica**

- Los únicos artefactos agénticos permitidos son el archivo `CLAUDE.md` de la raíz y el directorio
  `.claude/` de la raíz (skills, subagentes y configuración de Claude Code), dado que el stack
  agéntico del proyecto es Claude.
- Está prohibido crear archivos `AGENTS.md` en cualquier ubicación del repositorio, incluidas la
  raíz, `frontend/` y `backend/`.
- Las skills de terceros se incorporan copiando en `.claude/skills/` solo las carpetas
  seleccionadas, desde un commit fijo de su repositorio de origen y con su archivo de licencia.
  Está prohibido instalarlas como plugins o desde ramas móviles (aplicación del Principio IV).
  Antes de incorporarlas, una persona revisa su contenido (instrucciones, scripts y referencias a
  otras skills). El repositorio y el commit de origen de cada skill, y sus adaptaciones, se
  registran en el mensaje del commit que la incorpora o la actualiza.
- Ante conflicto entre una skill, propia o de terceros, y esta constitución, prevalece la
  constitución.
- Las herramientas que generan archivos de instrucciones agénticas DEBEN configurarse para no
  generarlos. En particular, en Next.js se desactiva la generación de `AGENTS.md`
  (`agentRules: false` en `next.config` y `--no-agents-md` al crear el proyecto).

**Justificación**: en un trabajo colaborativo evaluado, cada commit y cada integración en `main`
necesitan un responsable humano identificable; por eso el repositorio, su índice y su historial
siguen siendo territorio exclusivamente humano, y la revisión y el merge quedan en manos del Agile
Enabler. Lo que sí se delega es el trabajo mecánico que rodea a esa autoría: redactar un mensaje de
commit, transcribir a issues lo que ya está decidido en `specs/`, preparar la descripción de un
pull request. En los tres casos el agente produce texto derivado de archivos versionados por
personas y una persona confirma antes de que ese texto llegue a GitHub, así que no se cede ninguna
decisión ni ninguna línea de código. Canalizar cada escritura por una skill nombrada, en lugar de
permitirla en cualquier punto de la conversación, hace que la frontera sea verificable: basta mirar
qué skill se invocó.

## Restricciones Técnicas

**Estructura del repositorio**

```
/
├── backend/          # Aplicación Go (Gin + GORM) + Dockerfile
│   ├── cmd/
│   │   └── app/      # Punto de entrada de la aplicación backend
│   ├── internal/
│   │   └── <módulo>/ # delivery/ service/ repository/ domain/
│   └── postgres/     # docker-compose, .env, .env.example, .env.test
├── frontend/         # Aplicación Next.js + Tailwind CSS + Dockerfile
│   └── src/          # app/ features/ components/ config/ hooks/
│                     # lib/ providers/ styles/ types/ utils/
├── .claude/          # Configuración agéntica de Claude Code
│   ├── skills/
│   ├── agents/
│   └── settings.json
├── .github/
│   └── ISSUE_TEMPLATE/  # Plantillas de issues: historia de usuario, fase técnica y tarea
├── .specify/         # Configuración de Spec Kit
│   ├── memory/       # constitution.md: este documento
│   └── templates/    # Plantillas de spec, plan y tasks
├── specs/            # Una carpeta por feature: spec.md, plan.md, tasks.md
├── README.md         # Documentación en español, siempre actualizada
├── CLAUDE.md         # Instrucciones agénticas de la raíz (junto con .claude/)
└── .gitignore        # Con secciones #Frontend y #Backend
```

**Modelo de datos (obligatorio para todo modelo persistido)**

- La clave primaria DEBE ser de tipo UUID. Están prohibidos los enteros secuenciales como
  identificador primario o público.
- Todo modelo DEBE registrar fecha y hora de creación y fecha y hora de última actualización.
- Las contraseñas se persisten únicamente como hash bcrypt.

**Entornos**

- Desarrollo y test son entornos disjuntos: distintos contenedores, distintas bases de datos y
  distintos archivos de variables de entorno (`.env` frente a `.env.test`).

## Flujo de Trabajo de Desarrollo

1. **Especificar**: la funcionalidad se define antes de escribirse; el alcance queda acotado a lo
   pedido (Principio I).
2. **Publicar el trabajo en el tablero**: las historias de usuario y las tareas de `specs/` se
   publican como issues de GitHub mediante la skill de sincronización, que presenta el plan
   completo de cambios y espera la aprobación explícita de una persona antes de escribir
   (Principio IX, ámbito d).
3. **Probar primero**: se escribe la prueba de integración de la funcionalidad y se verifica que
   falla (Principio II). Las pruebas unitarias se añaden solo ante lógica compleja aislable.
4. **Levantar el entorno de test**: los servicios de test se arrancan con Docker Compose usando
   `.env.test` (Principio V).
5. **Implementar**: se escribe el código mínimo que hace pasar la prueba, respetando la
   arquitectura por capas, el manejo de errores y la validación de entradas.
6. **Autorizar dependencias**: si la implementación requiere una dependencia externa no prevista,
   se detiene el trabajo y se solicita autorización con justificación (Principio IV).
7. **Refactorizar**: se eliminan duplicaciones y se simplifica sin romper pruebas.
8. **Documentar**: se actualizan Swagger, el `README.md` y el `.gitignore` en el mismo cambio.
9. **Verificar cobertura**: se ejecuta el comando de cobertura de backend y/o frontend.
10. **Entregar**: si se le pide, el agente propone el mensaje de commit con `/redactar-commit`,
    pero el commit lo ejecuta una persona; ningún agente escribe en el código, en el índice ni en
    el historial del repositorio (Principio IX, ámbitos a y b).
11. **Abrir el pull request**: desde la rama de la historia o de la fase técnica hacia `main`, con
    `/redactar-pr` y previa confirmación explícita del título y el cuerpo. La revisión, la
    aprobación y el merge son exclusivos del Agile Enabler (Principio IX, ámbito c).

**Puertas de calidad (todas deben pasar antes de dar por terminado un cambio)**

- [ ] Existe prueba escrita antes del código y ahora pasa.
- [ ] Las pruebas corrieron contra el entorno de test, no contra el de desarrollo.
- [ ] No hay secretos fuera de `.env`; `.env.example` está sincronizado.
- [ ] No hay tipo `any` en el frontend ni errores de Go sin manejar en el backend.
- [ ] Cada bloque de código nuevo tiene su comentario en español y los identificadores están en
      inglés.
- [ ] Swagger y `README.md` reflejan el estado actual.
- [ ] Las versiones añadidas son estables y las dependencias no previstas fueron autorizadas.

## Gobernanza

Esta constitución tiene precedencia sobre cualquier otra práctica, preferencia o convención
adoptada en el proyecto. Ante conflicto entre este documento y una decisión puntual, prevalece
este documento.

**Procedimiento de enmienda**

1. La propuesta de enmienda se documenta indicando el principio afectado, el texto nuevo y la
   razón del cambio.
2. Requiere aprobación explícita del responsable humano del proyecto.
3. Si la enmienda invalida trabajo existente, DEBE acompañarse de un plan de migración.
4. La enmienda aprobada se aplica a `.specify/memory/constitution.md` actualizando la versión y la
   fecha de última modificación.

**Política de versionado (semántico)**

- **MAJOR**: eliminación o redefinición incompatible de un principio o de una regla de gobernanza.
- **MINOR**: incorporación de un principio o sección nueva, o ampliación material de una guía.
- **PATCH**: aclaraciones, correcciones de redacción y refinamientos sin cambio semántico.

**Revisión de cumplimiento**

- Toda revisión de cambios DEBE verificar el cumplimiento de las puertas de calidad de este
  documento.
- Cualquier desviación DEBE justificarse explícitamente por escrito; una desviación no justificada
  bloquea la entrega.
- Toda complejidad añadida DEBE justificarse frente al Principio I.
- `CLAUDE.md` en la raíz es la guía operativa en tiempo de desarrollo y DEBE mantenerse alineado
  con esta constitución; ante discrepancia, manda la constitución.

**Version**: 2.3.0 | **Ratified**: 2026-09-19 | **Last Amended**: 2026-10-05
