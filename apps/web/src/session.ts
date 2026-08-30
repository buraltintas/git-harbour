const KEY = 'githarbour.session';
const RETURN_KEY = 'githarbour.auth-return';
const safeReturn = (route: string) => /^#\/(?:challenge\/[A-Za-z0-9_-]{4,128}|battle\/[A-Za-z0-9-]{8,64}|battles|leaderboard)$/.test(route);
let current: string | null = null;
export const session = {
  get() {
    if (current) return current;
    try {
      current = localStorage.getItem(KEY);
    } catch {}
    return current;
  },
  set(token: string) {
    current = token;
    try {
      localStorage.setItem(KEY, token);
    } catch {}
  },
  clear() {
    current = null;
    try {
      localStorage.removeItem(KEY);
      localStorage.removeItem('githarbour.game');
    } catch {}
  },
  setReturnRoute(route: string) {
    if (!safeReturn(route)) return;
    try { sessionStorage.setItem(RETURN_KEY, route); } catch {}
  },
  takeReturnRoute() {
    try {
      const route = sessionStorage.getItem(RETURN_KEY) || '';
      sessionStorage.removeItem(RETURN_KEY);
      return safeReturn(route) ? route : '#/';
    } catch { return '#/'; }
  },
};
