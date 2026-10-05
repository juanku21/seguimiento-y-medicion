# Escribir buenas pruebas

**Cargá esta referencia cuando:** escribas o modifiques pruebas, agregues mocks o
agregues métodos de limpieza o de apoyo para las pruebas.

## Resumen

Una prueba existe para atrapar una rotura concreta. Dos principios gobiernan
todo lo que sigue:

```
1. Toda prueba nombra la rotura que atrapa
2. Toda prueba ejercita la cosa real
```

El TDD estricto produce las dos cosas de forma natural: una prueba escrita
primero y vista fallar contra código real ya demostró que puede fallar, y solo
se gana un mock cuando la dependencia real resulta lenta o externa.

## Principio 1: nombrar la rotura

Antes de escribir el cuerpo de la prueba, respondé: **qué cambio en el código de
producción debería hacer fallar esta prueba, y ese cambio es un error o una
decisión?** Una prueba se gana su lugar atrapando una rama equivocada, un efecto
que falta, un argumento equivocado, un caso límite o un contrato roto.

**Derivá las expectativas de forma independiente.** Usá literales y fixtures
verificados a mano; las pruebas de tabla con valores `want` literales son la
forma preferida. Una expectativa calculada por el código que se está probando,
o por sus helpers, pasa haga lo que haga ese código:

```typescript
// ❌ Aserción espejo: el mismo builder calcula los dos lados, siempre es verdadera
const expected = buildSearchQuery({ tag: 'urgent' });
expect(buildSearchQuery({ tag: 'urgent' })).toBe(expected);

// ✅ Literal derivado a mano
expect(buildSearchQuery({ tag: 'urgent' })).toBe('tag:"urgent"');
```

**Sin detectores de cambios.** Si solo pueden hacer fallar una prueba las
decisiones intencionales (el valor de una constante, la redacción exacta de un
mensaje, una estructura privada), la prueba se dispara en cada rediseño y duerme
ante los errores. Probá el comportamiento que depende de la decisión: no
`expect(MAX_RETRIES).toBe(5)` sino "una llamada que falla se reintenta 5 veces y
el sexto intento nunca ocurre".

**Comportamiento, no texto.** Afirmar que un script, una skill o un archivo de
configuración contiene una línea exacta solo demuestra que la fuente es la
fuente. Corré los scripts contra entradas controladas y afirmá sobre salidas,
efectos o códigos de salida. Los documentos que instruyen a agentes se prueban
por el comportamiento del agente que los consume; la prosa escrita para personas
no lleva prueba alguna.

**Tu código, no el framework.** Probá el contrato que tu código establece en sus
límites: la ruta que registrás, la consulta que emitís, la carga que producís.
La mecánica de las dependencias es responsabilidad de quienes las mantienen (el
caso clásico: afirmar que tu router invoca un handler registrado, que es la
prueba del framework y no la tuya). Cuando el comportamiento de una dependencia
realmente te sorprendió, escribí una única prueba de caracterización acotada que
nombre el supuesto. El mismo límite vale dentro de tu código: los constructores,
los getters, las constantes y el reenvío trivial llevan prueba solo cuando
validan, normalizan, aplican valores por defecto, derivan, imponen una
restricción o producen efectos; si no, afirmá sobre el primer resultado visible
para el consumidor que dependa de ellos.

### Función de compuerta

```
ANTES de escribir el cuerpo de la prueba:
  Nombrá el cambio de producción que haría fallar esta prueba.

  No podés nombrar ninguno        → rediseñá alrededor de un comportamiento observable
  "Cambió el texto de la fuente"  → corré el artefacto y afirmá sobre sus efectos
  Solo decisiones intencionales   → es un detector de cambios; probá el
                                    comportamiento que depende de la decisión

  Confirmá que el valor esperado se deriva sin el código que se está probando.
  SI reutiliza la lógica o los helpers de ese código:
    Reemplazalo por un literal o un fixture verificado a mano
```

## Principio 2: ejercitar la cosa real

En este proyecto la prueba por defecto es de integración contra la base de datos
de test, así que el mock es la excepción: las reglas que siguen aplican cuando
el mock resulta inevitable.

**El mock no se gana aserciones.** Una aserción sobre un mock pasa cuando el
mock está presente y falla cuando no está: no dice nada sobre el componente.
Afirmá sobre el comportamiento del componente real; si lo que estás verificando
es el mock, desmockealo o borrá la aserción.

```typescript
// ✅ Comportamiento real
expect(screen.getByRole('navigation')).toBeInTheDocument();

// ❌ Existencia del mock
expect(screen.getByTestId('sidebar-mock')).toBeInTheDocument();
```

**La corrección del desarrollador:** "¿Estamos probando el comportamiento de un
mock?"

**Mockeá en el nivel correcto.** Aprendé todos los efectos del método real antes
de reemplazarlo; mockeá la operación lenta o externa y dejá real todo aquello de
lo que depende la prueba. Si tenés dudas, corré primero la prueba contra la
implementación real y observá qué necesita ocurrir de verdad.

```typescript
// ❌ El mock se come la escritura de configuración que lee la detección de duplicados
vi.mock('ToolCatalog', () => ({
  discoverAndCacheTools: vi.fn().mockResolvedValue(undefined)
}));

// ✅ Solo se mockea el arranque lento del servidor; la escritura de configuración sigue real
vi.mock('MCPServerManager');
```

**Hacé específicos los dobles.** Cuando los argumentos, la cantidad de llamadas
o el orden son parte del contrato, afirmá sobre ellos: un fake que acepta
cualquier cosa no verifica nada. Dale a cada rama (éxito, error, malformado) su
propio fixture o espía, para que la rama equivocada no pueda satisfacer la
expectativa.

**Espejá los datos reales por completo.** Mockeá la estructura completa tal como
existe en la realidad, con todos los campos documentados, no solo los que lee tu
prueba. Los mocks parciales fallan en silencio cuando el código de más abajo lee
un campo omitido: la prueba pasa mientras la integración se rompe.

**Las clases de producción llevan solo métodos de producción.** La limpieza que
únicamente necesitan las pruebas vive en utilidades de prueba, nunca como un
`destroy()` de la clase de producción. Preguntate: ¿este método se llama solo
desde las pruebas? ¿Esta clase es dueña del ciclo de vida de ese recurso? Si las
respuestas dan mal, va a una utilidad de prueba.

**Preferí componentes reales antes que mocks complejos.** Cuando la preparación
del mock crece más que la lógica de la prueba, cuando a los mocks les faltan
métodos que los componentes reales tienen, o cuando las pruebas se rompen al
cambiar el mock, pasá a una prueba de integración con componentes reales.
**La pregunta del desarrollador:** "¿Hace falta usar un mock acá?"

### Función de compuerta

```
ANTES de agregar un mock o un helper de prueba:
  Enumerá los efectos del método real; dejá reales los que la prueba
  necesita y mockeá el nivel lento o externo que está por debajo.

  Las respuestas mockeadas espejan la estructura real completa.

  Un método que solo llaman las pruebas vive en utilidades de prueba,
  no en producción.

  ¿Estás por afirmar sobre el mock mismo?
    Desmockealo o borrá la aserción.
```

## Las pruebas se entregan con la implementación

El ciclo TDD (prueba que falla, implementación mínima, refactor) es lo que
significa "completo", y cada etapa queda registrada en su propio commit, hecho
por el desarrollador. Entregá las pruebas que el comportamiento necesita y solo
esas: el código trivial y la prosa escrita para personas no llevan ninguna, y
una prueba escrita para cumplir con un proceso cuesta mantenimiento para
siempre.

## La prueba de mutación

Antes de terminar, mutá mentalmente el código de producción; al menos una prueba
debería fallar ante cada mutación realista:

- Constante o argumento equivocado
- Rama equivocada
- Cambio de estado o efecto que falta
- Retorno vacío o por defecto
- Validación ausente para cero, vacío, nil, no autorizado o malformado

Una mutación que nada atrapa marca el comportamiento como desprotegido, o la
prueba como tautológica.

## Referencia rápida

| Cuando... | Hacé |
|-----------|------|
| Escribís cualquier prueba | Nombrá la rotura que atrapa: un error, no una decisión |
| Construís un valor esperado | Derivalo a mano; nunca con el código que se está probando |
| Probás un script o un documento | Corrélo o poné a prueba a su consumidor; nunca busques en su texto |
| Te tienta probar una dependencia | Probá el contrato de tu límite, no la mecánica documentada de otros |
| Querés afirmar sobre un elemento mockeado | Probá el componente real, o desmockealo |
| Estás por mockear un método | Aprendé sus efectos; mockeá el nivel lento o externo |
| Construís una respuesta mockeada | Espejá la estructura real completa |
| Necesitás limpieza que solo usan las pruebas | Ponela en utilidades de prueba |
| Ves inflarse la preparación del mock | Pasá a una prueba de integración con componentes reales |
| Terminás un archivo de pruebas | Corré la prueba de mutación |

## Señales de advertencia

- La preparación y la aserción comparten el mismo objeto, lo que garantiza la igualdad
- La prueba solo puede fallar por un panic, una caída o un selector ausente
- La prueba falla ante cada cambio intencional y nunca ante una rotura accidental
- Los valores esperados están escondidos detrás de bucles, builders o helpers
- La prueba busca en el texto fuente, o afirma que un símbolo eliminado sigue eliminado
- La prueba seguiría importando si solo quedara el framework
- La prueba existe por cobertura y no verifica ningún efecto ni resultado
- Una aserción verifica un ID de prueba `*-mock`, o falla si quitás el mock
- Un método se llama únicamente desde archivos de prueba
- La preparación del mock es más de la mitad de la prueba, o no podés explicar por qué hace falta el mock
- Mockear "por si acaso"
