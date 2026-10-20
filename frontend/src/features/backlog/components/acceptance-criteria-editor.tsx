"use client";

import { useId, useRef } from "react";
import { Input } from "@/components/input";
import { MAX_ACCEPTANCE_CRITERION_LENGTH } from "@/features/backlog/validation";

// Criterio de aceptación en edición. El id es estable mientras el criterio existe, para que
// reordenar o quitar no confunda el campo ni el error que le corresponden.
export type CriterionItem = {
  id: string;
  text: string;
};

// Propiedades del editor: la lista en orden, los errores por id de criterio y el aviso de cambios.
type AcceptanceCriteriaEditorProps = {
  items: CriterionItem[];
  errors: Partial<Record<string, string>>;
  onChange: (items: CriterionItem[]) => void;
};

// Indicación del límite de cada criterio, visible desde el inicio (FR-007).
const CRITERION_HINT = `Hasta ${MAX_ACCEPTANCE_CRITERION_LENGTH} caracteres.`;

const SECONDARY_BUTTON_CLASS_NAME =
  "rounded-md border border-zinc-300 px-3 py-1.5 text-xs font-medium text-zinc-800 hover:bg-zinc-100 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-zinc-900 disabled:cursor-not-allowed disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800";

// Devuelve una copia de la lista con los elementos de las posiciones from y to intercambiados.
function swap(items: CriterionItem[], from: number, to: number) {
  const next = [...items];
  [next[from], next[to]] = [next[to], next[from]];
  return next;
}

// Editor de la lista ordenada de criterios de aceptación (003/US1): permite agregar, quitar y
// reordenar criterios. Cada campo se llama "Criterio N" según su posición en la lista.
export function AcceptanceCriteriaEditor({
  items,
  errors,
  onChange,
}: AcceptanceCriteriaEditorProps) {
  const idPrefix = useId();
  const nextIdRef = useRef(0);

  // Agrega un criterio vacío al final de la lista con un id nuevo.
  function addItem() {
    nextIdRef.current += 1;
    onChange([...items, { id: `${idPrefix}-${nextIdRef.current}`, text: "" }]);
  }

  // Reemplaza el texto del criterio indicado.
  function updateItem(id: string, text: string) {
    onChange(items.map((item) => (item.id === id ? { ...item, text } : item)));
  }

  return (
    <fieldset className="flex flex-col gap-3">
      <legend className="mb-1 text-sm font-medium">
        Criterios de aceptación
      </legend>
      <ol className="flex flex-col gap-4">
        {items.map((item, index) => {
          const position = index + 1;
          return (
            <li key={item.id} className="flex flex-col gap-2">
              <Input
                label={`Criterio ${position}`}
                autoComplete="off"
                value={item.text}
                onChange={(event) => updateItem(item.id, event.target.value)}
                hint={CRITERION_HINT}
                error={errors[item.id]}
              />
              <div className="flex gap-2">
                <button
                  type="button"
                  className={SECONDARY_BUTTON_CLASS_NAME}
                  aria-label={`Subir criterio ${position}`}
                  disabled={index === 0}
                  onClick={() => onChange(swap(items, index, index - 1))}
                >
                  Subir
                </button>
                <button
                  type="button"
                  className={SECONDARY_BUTTON_CLASS_NAME}
                  aria-label={`Bajar criterio ${position}`}
                  disabled={index === items.length - 1}
                  onClick={() => onChange(swap(items, index, index + 1))}
                >
                  Bajar
                </button>
                <button
                  type="button"
                  className={SECONDARY_BUTTON_CLASS_NAME}
                  aria-label={`Quitar criterio ${position}`}
                  onClick={() =>
                    onChange(items.filter((other) => other.id !== item.id))
                  }
                >
                  Quitar
                </button>
              </div>
            </li>
          );
        })}
      </ol>
      <button
        type="button"
        className={`self-start ${SECONDARY_BUTTON_CLASS_NAME}`}
        onClick={addItem}
      >
        Agregar criterio
      </button>
    </fieldset>
  );
}
