<!--
Título del PR: tipo(ámbito): descripción de la historia (NNN/USx)
Ejemplos:
  feat(auth): registro de cuenta con correo y contraseña (001/US1)
  chore(infra): base técnica del proyecto (001/setup)
Máximo 72 caracteres. Al mergear, el título pasa a ser el mensaje del merge commit.
Los comentarios como este no se ven en el PR publicado.
-->

## Historia de usuario

Closes #<!-- número del issue de la historia o fase técnica -->

- Especificación: `specs/NNN-nombre/spec.md`
- Historia: NNN/USx - <!-- título de la historia -->
- Prioridad: <!-- P1, P2 o P3 -->

## Resumen

<!-- Qué puede hacer el usuario a partir de este PR, en 2 a 4 oraciones. Sin detalles de implementación. -->

## Tareas incluidas

<!-- Una fila por sub-issue. Cada tarea debe tener un commit con "Closes #N". -->

| Tarea | Sub-issue | Descripción | Commits |
| --- | --- | --- | --- |
| T000 | #0 | | `0000000` |

## Criterios de aceptación

<!-- Un ítem por escenario Given/When/Then de la historia en spec.md, con la prueba que lo cubre. -->

- [ ] Escenario 1: <!-- nombre --> - cubierto por `<!-- prueba -->`

## Evidencia de TDD

<!-- Commits con pie "TDD:" para reglas de negocio, cálculos y validaciones. -->

| Tarea | RED | GREEN | REFACTOR |
| --- | --- | --- | --- |
| T000 | `0000000` | `0000000` | - |

## Pruebas

<!-- Comandos ejecutados por el autor y su resultado. -->

| Tipo | Comando | Resultado |
| --- | --- | --- |
| Unitarias | | |
| Integración | | |
| BDD | | |

Cobertura del módulo: <!-- porcentaje -->

## Cambios técnicos relevantes

<!-- Completar solo lo que aplique; escribir "Ninguno" en lo demás. -->

- Endpoints nuevos o modificados:
- Migraciones o cambios de esquema:
- Dependencias nuevas (indicar autorización):
- Variables de entorno nuevas:

## Cómo probarlo

<!-- Pasos para que el revisor verifique la historia en el entorno local. -->

1.

## Pendientes y fuera de alcance

<!-- Tareas no completadas, decisiones diferidas o defectos conocidos. "Ninguno" si no hay. -->

## Checklist del autor

- [ ] Todas las sub-issues tienen un commit con `Closes #N`.
- [ ] Todos los commits siguen el formato de commits de la constitución (Principio IX, ámbito b) y tienen el pie `Spec:`.
- [ ] La rama está actualizada con `main`.
- [ ] Las pruebas pasan en el entorno local.
- [ ] No se versionaron secretos ni archivos `.env` reales.
- [ ] Swagger y README están actualizados, si corresponde.
- [ ] Las dependencias nuevas están autorizadas.

## Notas para el revisor

<!-- Decisiones de diseño, dudas o puntos que conviene revisar con atención. -->
