<!--
SYNC IMPACT REPORT
==================
Cambio de versión: 1.1.0 → 1.2.0
Tipo de bump: MINOR (se amplía materialmente la guía del Principio IX y la
estructura del repositorio; no se elimina ninguna prohibición existente sobre
el código ni el historial de Git)

Historial:
  - 1.0.0 (2026-09-19): ratificación original del documento.
  - 1.1.0 (2026-09-19): enmienda de los Principios III y VIII.
  - 1.2.0 (2026-09-20): enmienda del Principio IX (gestión de issues por
    agentes vía MCP de GitHub bajo condiciones estrictas y ampliación de los
    artefactos agénticos permitidos), del árbol de directorios y del Flujo de
    Trabajo de Desarrollo.

Principios modificados en 1.2.0:
  - IX. Gobierno del Repositorio y Límites de Agentes de IA: se separa el
    ámbito de código e historial de Git (prohibición total para agentes, sin
    cambios de fondo) del ámbito de gestión de issues del tablero (permitido
    vía servidor MCP de GitHub bajo cinco condiciones acumulativas). Los
    artefactos agénticos permitidos pasan a ser `CLAUDE.md` y el directorio
    `.claude/` de la raíz; se mantiene la prohibición de `AGENTS.md`.
    Justificación reescrita.

Secciones modificadas en 1.2.0:
  - Restricciones Técnicas: el árbol de directorios incorpora `.claude/`
    (skills, agentes y configuración) y `.github/ISSUE_TEMPLATE/`.
  - Flujo de Trabajo de Desarrollo: nuevo paso 2 (publicación de historias y
    tareas en GitHub con aprobación humana del plan); pasos 2 a 9 renumerados
    a 3 a 10, y el paso de entrega precisa que la prohibición alcanza al
    código y al historial, no al tablero de issues.

Principios modificados en 1.1.0:
  - III. Arquitectura por Capas: el backend fija `cmd/app/` como punto de
    entrada e `internal/<módulo>/` como raíz de los módulos con sus capas.
  - VIII. Estándares de Código y Documentación Viva: se retira la regla de
    indentación de dos tabulaciones; el formateo pasa a regirse por el
    estándar oficial de cada lenguaje (`gofmt` en Go, convenciones oficiales
    de TypeScript en el frontend).

Secciones modificadas en 1.1.0:
  - Restricciones Técnicas: árbol de directorios actualizado con `cmd/app/`
    e `internal/`.

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

Secciones añadidas:
  - Restricciones Técnicas (stack, estructura de directorios, modelo de datos)
  - Flujo de Trabajo de Desarrollo (ciclo TDD, autorizaciones, puertas de calidad)
  - Gobernanza

Secciones eliminadas: ninguna (el documento previo era la plantilla vacía)

Plantillas dependientes: no modificadas (leen la constitución en tiempo de
ejecución). La skill de sincronización de issues y el `CLAUDE.md` de la raíz
quedan alcanzados por el Principio IX enmendado y DEBEN revisarse contra él.

TODO pendientes: ninguno (TODO(INDENTACION_GO) queda resuelto al adoptar el
estándar de formateo oficial de cada lenguaje).
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

- Las pruebas de integración son el mecanismo por defecto para verificar funcionalidades.
- Las pruebas unitarias SOLO se escriben cuando exista lógica considerablemente compleja
  (cálculos, gran cantidad de operaciones lógicas) que requiera verificación aislada.
- Las pruebas DEBEN ejecutarse contra servicios y bases de datos de test, nunca contra el entorno
  de desarrollo (ver Principio V).
- El backend y el frontend DEBEN exponer cada uno un comando documentado para generar el reporte
  de cobertura de pruebas.

**Justificación**: un sistema que mide la calidad de otros proyectos pierde credibilidad si no
puede demostrar la suya. La cobertura medible es el registro de qué se está probando realmente.

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

**a) Código e historial de Git — prohibición total para agentes**

Ningún agente de IA (no humano) puede ejecutar operación de escritura alguna sobre el código ni
sobre el historial del repositorio colaborativo. La prohibición alcanza por igual a:

- Los comandos de Git que escriben: `commit`, `branch`, `checkout -b`, `merge`, `rebase`, `push`,
  `stash`, `tag`, `reset` y cualquier otro equivalente.
- Las APIs y herramientas MCP que producen el mismo efecto: crear ramas, crear, modificar o
  eliminar archivos remotos, hacer push, y abrir o mergear pull requests.

Estas operaciones son exclusivamente humanas. Los agentes solo pueden leer el repositorio.

**b) Gestión de issues del tablero — permitida bajo condiciones**

Un agente PUEDE crear, editar, etiquetar, comentar y cerrar issues, vincular sub-issues y crear
etiquetas del repositorio, únicamente a través del servidor MCP de GitHub y cumpliendo TODAS las
condiciones siguientes, que son acumulativas:

1. La fuente de verdad son los archivos de `specs/`. La sincronización es unidireccional, de
   `specs/` hacia GitHub: un agente jamás modifica una spec, un plan ni un `tasks.md` a partir del
   contenido de un issue.
2. Antes de escribir en GitHub, el agente DEBE presentar el plan completo de cambios y esperar la
   aprobación explícita de una persona.
3. Está prohibido eliminar issues, comentarios o etiquetas. Un issue obsoleto se cierra como "no
   planificado" con un comentario que explica el motivo.
4. Los campos Valor, Prioridad y Estimación, los milestones y las asignaciones los gestiona
   exclusivamente el equipo humano; el agente no los completa ni los modifica.
5. El servidor MCP de GitHub DEBE configurarse exponiendo solo las herramientas de issues y
   etiquetas, y sus credenciales nunca se versionan (aplicación del Principio VI).

**c) Archivos de instrucciones y configuración agéntica**

- Los únicos artefactos agénticos permitidos son el archivo `CLAUDE.md` de la raíz y el directorio
  `.claude/` de la raíz (skills, subagentes y configuración de Claude Code), dado que el stack
  agéntico del proyecto es Claude.
- Está prohibido crear archivos `AGENTS.md` en cualquier ubicación del repositorio, incluidas la
  raíz, `frontend/` y `backend/`.

**Justificación**: la trazabilidad de la autoría en un trabajo colaborativo evaluado depende de
que cada commit tenga un responsable humano identificable, y por eso el código y el historial
siguen siendo territorio exclusivamente humano. Los issues son otra cosa: no son autoría, son el
reflejo organizacional de lo que ya está decidido en `specs/`. Como su contenido se deriva de
archivos versionados por personas y cada escritura requiere aprobación humana previa, delegar esa
transcripción en un agente ahorra trabajo mecánico sin ceder ni una decisión ni una línea de
código.

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
   (Principio IX, ámbito b).
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
10. **Entregar**: el commit lo realiza una persona; ningún agente escribe en el código ni en el
    historial del repositorio (Principio IX, ámbito a).

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

**Version**: 1.2.0 | **Ratified**: 2026-09-19 | **Last Amended**: 2026-09-20
