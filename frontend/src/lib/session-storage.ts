// Persistencia del token de sesión en sessionStorage (research.md, sección 9): el token sobrevive al
// refresco de la pestaña pero no al cierre del navegador.
const TOKEN_KEY = "sessionToken";

// Marca que deja un rechazo de la API por sesión inválida, para que la guardia sepa que debe avisar
// que hay que volver a iniciar sesión (FR-016). Vive en sessionStorage para no depender de quién la
// lea primero y se borra al iniciar una sesión nueva.
const REJECTED_KEY = "sessionRejected";

// Suscriptores que se vuelven a renderizar cuando cambia el token. sessionStorage no emite eventos
// en la misma pestaña, así que el aviso lo hacen las funciones que escriben o borran el token.
const listeners = new Set<() => void>();

// Indica si hay sessionStorage disponible. Next.js también prerenderiza los Client Components en el
// servidor, donde no existe window, y ahí no hay sesión que leer ni guardar.
function hasSessionStorage(): boolean {
  return typeof window !== "undefined";
}

// Avisa a todos los suscriptores que el token cambió.
function notifyListeners(): void {
  listeners.forEach((listener) => listener());
}

// Registra un suscriptor y devuelve la función que lo da de baja, como pide useSyncExternalStore.
export function subscribeToken(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

// Devuelve el token guardado en la pestaña, o null si no hay sesión o si se ejecuta en el servidor.
export function readToken(): string | null {
  if (!hasSessionStorage()) {
    return null;
  }
  return window.sessionStorage.getItem(TOKEN_KEY);
}

// Guarda el token recibido al iniciar sesión, descarta un rechazo anterior y avisa a los
// suscriptores; en el servidor no hace nada.
export function writeToken(token: string): void {
  if (!hasSessionStorage()) {
    return;
  }
  window.sessionStorage.setItem(TOKEN_KEY, token);
  window.sessionStorage.removeItem(REJECTED_KEY);
  notifyListeners();
}

// Borra el token porque la API rechazó la sesión, deja la marca del rechazo y avisa a los
// suscriptores; en el servidor no hace nada.
export function rejectSession(): void {
  if (!hasSessionStorage()) {
    return;
  }
  window.sessionStorage.removeItem(TOKEN_KEY);
  window.sessionStorage.setItem(REJECTED_KEY, "true");
  notifyListeners();
}

// Borra el token porque la persona cerró su sesión (US5) y avisa a los suscriptores. A diferencia de
// rejectSession no deja la marca del rechazo: el cierre es voluntario y la guardia no debe mostrar
// el aviso de volver a iniciar sesión. En el servidor no hace nada.
export function clearSession(): void {
  if (!hasSessionStorage()) {
    return;
  }
  window.sessionStorage.removeItem(TOKEN_KEY);
  window.sessionStorage.removeItem(REJECTED_KEY);
  notifyListeners();
}

// Indica si la última sesión de la pestaña terminó por un rechazo de la API.
export function wasSessionRejected(): boolean {
  if (!hasSessionStorage()) {
    return false;
  }
  return window.sessionStorage.getItem(REJECTED_KEY) !== null;
}
