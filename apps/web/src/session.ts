const KEY = 'githarbour.session';
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
};
