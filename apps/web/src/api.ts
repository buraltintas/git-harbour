import type { BattleEvent, Battles, Challenge, Coord, Day, DeploymentChoice, Game, LeaderboardEntry, PublicUser, User } from './types';
import { session } from './session';
export const apiBase = import.meta.env.VITE_API_URL || 'http://localhost:8080';
const emptyStats={games:0,wins:0,losses:0,rating:1200,shots:0,hits:0,currentStreak:0,longestStreak:0,winRate:0,accuracy:0,averageShotsPerWin:0,rank:'Officer'};
const identity=(value:any)=>({login:value?.login||value?.Login||'',name:value?.name||value?.Name||'',avatarUrl:value?.avatarUrl||value?.AvatarURL||'',pvp:value?.pvp||emptyStats});
function normalizeBattles(raw:any):Battles {const map=(rows:any[]=[])=>rows.map(x=>({...x,opponent:identity(x.opponent),winner:x.winner==='you'||x.winner==='opponent'?x.winner:undefined}));return{yourTurn:map(raw.yourTurn),waiting:map(raw.waiting),finished:map(raw.finished),challenges:raw.challenges||[]}}
export class ApiError extends Error {
  constructor(public status:number, public code:string, message:string) { super(message); }
}
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
    const error=body.error;
    throw new ApiError(response.status,typeof error==='object'&&error?.code?error.code:'request_failed',typeof error==='string'?error:error?.message||'Request failed');
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
  devSession: (user = 'octocat') =>
    request<{ token: string; user: User }>(
      `/v1/dev/session?user=${encodeURIComponent(user)}`,
      { method: 'POST' },
      false,
    ),
  contributions: () => request<{ days: Day[] }>('/v1/me/contributions'),
  me: () => request<User>('/v1/me'),
  create: (playerStart: string) =>
    request<Game>('/v1/games/solo', {
      method: 'POST',
      body: JSON.stringify({playerStart}),
    }),
  game: (id: string) => request<Game>(`/v1/games/${id}`),
  deploy: (id:string,units:DeploymentChoice[])=>request<Game>(`/v1/games/${id}/deployment`,{method:'POST',body:JSON.stringify({units})}),
  shot: (id: string, target:Coord) =>
    request<{ game: Game; events: BattleEvent[] }>(`/v1/games/${id}/shots`, {
      method: 'POST',
      body: JSON.stringify({ target }),
    }),
  publicUser: async (login: string) => {
    const u=await request<any>(
      `/v1/public/users/${encodeURIComponent(login)}`,
      undefined,
      false,
    );
    return {...u,pvpHistory:(u.pvpHistory||[]).map((x:any)=>({id:x.gameId,opponent:identity(x.opponent),result:x.won?'victory':'defeat',shots:x.shots,ratingDelta:x.ratingDelta,completedAt:x.completedAt,shareId:x.shareId||x.gameId}))} as PublicUser;
  },
  publicChallenge:(code:string)=>request<Challenge>(`/v1/public/challenges/${encodeURIComponent(code)}`),
  createChallenge:()=>request<{challenge:Challenge;challengeUrl:string}>('/v1/challenges',{method:'POST'}),
  acceptChallenge:(code:string)=>request<{challenge:Challenge;game?:unknown}>(`/v1/challenges/${encodeURIComponent(code)}/accept`,{method:'POST'}),
  cancelChallenge:(code:string)=>request<{challenge:Challenge}>(`/v1/challenges/${encodeURIComponent(code)}/cancel`,{method:'POST'}),
  battles:async()=>normalizeBattles(await request<any>('/v1/battles')),
  rematch:(id:string)=>request<{challenge:Challenge;challengeUrl:string}>(`/v1/games/${encodeURIComponent(id)}/rematch`,{method:'POST'}),
  arcadeLeaderboard:()=>request<{entries:LeaderboardEntry[]}>('/v1/public/leaderboards/solo',undefined,false),
  pvpLeaderboard:()=>request<{entries:LeaderboardEntry[]}>('/v1/public/leaderboards/pvp',undefined,false),
};

const friendly:Record<string,string>={pvp_refit:'Developer vs Developer is not part of the current ruleset yet.',history_not_playable:'Your imported history has no distinct second ten-week period. This fallback needs a product decision.',invalid_player_harbour:'Choose a valid contiguous ten-week contribution harbour.',legacy_game_retired:'That earlier battle cannot continue under the current ruleset.',deployment_rejected:'Choose the required contribution units and place every required Reserve.',setup_locked:'This deployment is already locked.',shot_rejected:'Choose one untargeted enemy coordinate.',rate_limited:'Too many shots arrived at once. Wait a moment and continue.',game_not_found:'This battle is unavailable.',game_complete:'This battle is already complete.'};
export function friendlyError(error:unknown,fallback='Something went wrong. Try again.') { return error instanceof ApiError?(friendly[error.code]||fallback):fallback; }
