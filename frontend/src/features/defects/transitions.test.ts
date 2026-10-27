import { describe, expect, it } from "vitest";
import { allowedTransitions } from "@/features/defects/transitions";

// Pruebas unitarias de la copia en el cliente del mapa de transiciones del backend (T030;
// research.md sección 8, RN-12). La interfaz decide qué acciones de estado ofrece con este mapa y
// el backend lo vuelve a validar.
//
// Solo se fijan las transiciones de esta historia: Abierto → En progreso y En progreso → Resuelto,
// y las que la historia rechaza. Las aserciones usan pertenencia y no igualdad exacta para que
// US5 (reapertura) y US6 (descarte) puedan sumar destinos sin romper estas pruebas.

describe("007/US2 - Seguir y resolver un defecto", () => {
  // Caso normal: el ciclo Abierto → En progreso → Resuelto está admitido (US2, escenario 1).
  it("Abierto pasa a En progreso y más tarde a Resuelto", () => {
    expect(allowedTransitions("open")).toContain("in_progress");
    expect(allowedTransitions("in_progress")).toContain("resolved");
  });

  // Caso de error: un Abierto no salta a Resuelto ni se queda en Abierto (US2, escenario 10).
  it("un defecto Abierto no pasa directamente a Resuelto", () => {
    const allowed = allowedTransitions("open");
    expect(allowed).not.toContain("resolved");
    expect(allowed).not.toContain("open");
  });

  // Caso de error: un En progreso no vuelve a Abierto ni se queda en En progreso (US2,
  // escenario 11).
  it("un defecto En progreso no vuelve a Abierto", () => {
    const allowed = allowedTransitions("in_progress");
    expect(allowed).not.toContain("open");
    expect(allowed).not.toContain("in_progress");
  });

  // Los estados posteriores no vuelven a En progreso ni se resuelven otra vez (RN-12): un Resuelto
  // solo se reabre y un Descartado es definitivo.
  it("un defecto Resuelto o Descartado no pasa a En progreso ni a Resuelto", () => {
    for (const current of ["resolved", "discarded"] as const) {
      const allowed = allowedTransitions(current);
      expect(allowed).not.toContain("in_progress");
      expect(allowed).not.toContain("resolved");
    }
  });
});

describe("007/US5 - Reabrir un defecto resuelto", () => {
  // Caso normal: un defecto Resuelto puede volver a Abierto (US5, RN-15).
  it("un defecto Resuelto puede reabrirse a Abierto", () => {
    expect(allowedTransitions("resolved")).toContain("open");
  });

  // Caso de error: solo lo Resuelto se reabre; un Abierto, un En progreso y un Descartado no pasan
  // a Abierto (US5, RN-12).
  it("un defecto que no está Resuelto no pasa a Abierto", () => {
    for (const current of ["open", "in_progress", "discarded"] as const) {
      expect(allowedTransitions(current)).not.toContain("open");
    }
  });
});
