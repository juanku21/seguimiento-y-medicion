"use client";

import { useEffect, useState } from "react";
import { useRequireSession } from "@/hooks/use-session";
import type { ApiResult } from "@/lib/api-client";

// Estado de una consulta a la API: todavía sin respuesta, los datos recibidos o el mensaje de error.
export type QueryState<T> =
  | { status: "loading" }
  | { status: "loaded"; data: T }
  | { status: "error"; message: string };

// Consulta de lectura protegida por la guardia de sesión: ejecuta load con un token vigente y cada
// vez que cambia load, que el llamador estabiliza con useCallback según sus parámetros. La bandera
// descarta la respuesta si llegó otra consulta o el componente se desmontó.
export function useAuthorizedQuery<T>(
  load: () => Promise<ApiResult<T>>,
): QueryState<T> {
  const token = useRequireSession();
  const [state, setState] = useState<QueryState<T>>({ status: "loading" });

  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    load().then((result) => {
      if (!isCurrent) {
        return;
      }
      setState(
        result.ok
          ? { status: "loaded", data: result.data }
          : { status: "error", message: result.error.message },
      );
    });
    return () => {
      isCurrent = false;
    };
  }, [token, load]);

  return state;
}
