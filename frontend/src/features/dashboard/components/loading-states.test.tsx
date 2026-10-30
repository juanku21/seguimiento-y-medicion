import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { KeyIndicators } from "@/features/dashboard/components/key-indicators";
import { OpenDefectsDistribution } from "@/features/dashboard/components/open-defects-distribution";
import { SeriesSection } from "@/features/dashboard/components/series-section";

// Pruebas de los estados de carga de los componentes del dashboard (T045, per T040): cada uno
// recibe la consulta aún sin respuesta y debe mostrar solo el aviso de carga.

// Verifica el mensaje de carga de las secciones mientras la consulta no responde.
describe("dashboard - estados de carga (T045)", () => {
  // Desmonta el componente renderizado tras cada prueba.
  afterEach(() => {
    cleanup();
  });

  it("KeyIndicators muestra Cargando en ambos grupos mientras las consultas no responden", () => {
    render(
      <KeyIndicators
        projectMetrics={{ status: "loading" }}
        activeSprintMetrics={{ status: "loading" }}
      />,
    );
    expect(screen.getAllByRole("status")).toHaveLength(2);
    expect(screen.getAllByText("Cargando…")).toHaveLength(2);
    expect(screen.queryByText("Velocidad del equipo")).toBeNull();
  });

  it("OpenDefectsDistribution muestra Cargando y no monta la región mientras carga", () => {
    render(<OpenDefectsDistribution projectMetrics={{ status: "loading" }} />);
    expect(screen.getByRole("status").textContent).toBe("Cargando…");
    expect(screen.queryByText("Defectos abiertos por severidad")).toBeNull();
  });

  it("SeriesSection muestra Cargando sin invocar render mientras la consulta no responde", () => {
    let called = false;
    render(
      <>
        {SeriesSection({
          series: { status: "loading" },
          errorMessage: "Falla",
          render: () => {
            called = true;
            return null;
          },
        })}
      </>,
    );
    expect(screen.getByRole("status").textContent).toBe("Cargando…");
    expect(called).toBe(false);
  });
});
