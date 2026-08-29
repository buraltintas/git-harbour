import type { Day, Game, PublicUser, Ship, User } from './types';
import { session } from './session';
export const apiBase = import.meta.env.VITE_API_URL || 'http://localhost:8080';
async function request<T>(
  path: string,
  init?: RequestInit,
  authenticated = true,
): Promise<T> {
  const token = session.get();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((init?.headers as Record<string, string>) || {}),
  };
  if (authenticated && token) headers.Authorization = `Bearer ${token}`;
  const response = await fetch(apiBase + path, { ...init, headers });
  if (!response.ok) {
    const body = await response
      .json()
      .catch(() => ({ error: { message: 'Request failed' } }));
    if (response.status === 401 && authenticated) session.clear();
    throw new Error(
      typeof body.error === 'string'
        ? body.error
        : body.error?.message || 'Request failed',
    );
  }
  if (response.status === 204) return undefined as T;
  return response.json();
}
export const api = {
  authStart: `${apiBase}/auth/github/start`,
  exchange: (code: string) =>
    request<{ token: string; user: User }>(
      '/auth/exchange',
      { method: 'POST', body: JSON.stringify({ code }) },
      false,
    ),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),
  devSession: () =>
    request<{ token: string; user: User }>(
      '/v1/dev/session',
      { method: 'POST' },
      false,
    ),
  contributions: () => request<{ days: Day[] }>('/v1/me/contributions'),
  me: () => request<User>('/v1/me'),
  create: (startDate: string, fleet: Ship[]) =>
    request<Game>('/v1/games/solo', {
      method: 'POST',
      body: JSON.stringify({ startDate, fleet }),
    }),
  game: (id: string) => request<Game>(`/v1/games/${id}`),
  shot: (id: string, x: number, y: number) =>
    request<{ game: Game }>(`/v1/games/${id}/shots`, {
      method: 'POST',
      body: JSON.stringify({ x, y }),
    }),
  publicUser: (login: string) =>
    request<PublicUser>(
      `/v1/public/users/${encodeURIComponent(login)}`,
      undefined,
      false,
    ),
};
