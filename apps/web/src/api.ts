import type { Battles, Challenge, Day, Game, LeaderboardEntry, PvpGame, PublicUser, Ship, User } from './types';
import { session } from './session';
export const apiBase = import.meta.env.VITE_API_URL || 'http://localhost:8080';
const emptyStats={games:0,wins:0,losses:0,rating:1200,shots:0,hits:0,currentStreak:0,longestStreak:0,winRate:0,accuracy:0,averageShotsPerWin:0,rank:'Officer'};
const identity=(value:any)=>({login:value?.login||value?.Login||'',name:value?.name||value?.Name||'',avatarUrl:value?.avatarUrl||value?.AvatarURL||'',pvp:value?.pvp||emptyStats});
function normalizePvp(raw:any):PvpGame {if(raw?.yourBoard)return raw;const you=raw.you||{},opponent=raw.opponent||{},yourShots=you.shots||[],opponentShots=opponent.shots||[],result=raw.status==='complete'&&raw.result?{winner:raw.result.won?'you':'opponent',yourRating:{before:raw.result.ratingBefore||1200,after:raw.result.ratingAfter||1200,delta:raw.result.ratingDelta||0,rank:raw.result.rank||'Officer'},opponentRating:{before:raw.result.opponentRatingBefore||1200,after:raw.result.opponentRatingAfter||1200,delta:raw.result.opponentRatingDelta||0,rank:raw.result.opponentRank||opponent.user?.pvp?.rank||'Officer'},yourShots:raw.result.shots||yourShots.length,yourHits:raw.result.hits||0,yourAccuracy:(raw.result.shots||0)>0?100*(raw.result.hits||0)/raw.result.shots:0,shipsSunk:raw.result.shipsSunk||0,yourPeriod:{start:you.board?.[0]?.date||'',end:you.board?.at?.(-1)?.date||''},opponentPeriod:{start:opponent.board?.[0]?.date||'',end:opponent.board?.at?.(-1)?.date||''},shareId:raw.shareId||''}:undefined;return{id:raw.id,mode:'pvp',status:raw.status,you:identity(you.user),opponent:identity(opponent.user),yourTurn:!!raw.yourTurn,currentTurnLogin:raw.currentTurnLogin||'',yourBoard:you.board||[],opponentBoard:opponent.board||[],yourFleet:you.fleet||[],yourShots,opponentShots,yourSunkShips:[...new Set(opponentShots.filter((s:any)=>s.result==='sunk').map((s:any)=>s.ship).filter(Boolean))] as string[],opponentSunkShips:[...new Set(yourShots.filter((s:any)=>s.result==='sunk').map((s:any)=>s.ship).filter(Boolean))] as string[],lastMove:raw.lastMove,result:result as PvpGame['result']}}
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
  acceptChallenge:(code:string)=>request<{challenge:Challenge;game?:PvpGame}>(`/v1/challenges/${encodeURIComponent(code)}/accept`,{method:'POST'}),
  cancelChallenge:(code:string)=>request<{challenge:Challenge}>(`/v1/challenges/${encodeURIComponent(code)}/cancel`,{method:'POST'}),
  readyChallenge:(code:string,startDate:string,fleet:Ship[])=>request<{challenge:Challenge;game?:PvpGame}>(`/v1/challenges/${encodeURIComponent(code)}/ready`,{method:'POST',body:JSON.stringify({startDate,fleet})}),
  battles:async()=>normalizeBattles(await request<any>('/v1/battles')),
  pvpGame:async(id:string)=>normalizePvp(await request<any>(`/v1/games/${encodeURIComponent(id)}`)),
  pvpShot:async(id:string,x:number,y:number)=>{const r=await request<any>(`/v1/games/${encodeURIComponent(id)}/shots`,{method:'POST',body:JSON.stringify({x,y})});return{...r,game:normalizePvp(r.game)} as {game:PvpGame;events:unknown[]}},
  rematch:(id:string)=>request<{challenge:Challenge;challengeUrl:string}>(`/v1/games/${encodeURIComponent(id)}/rematch`,{method:'POST'}),
  leaderboard:()=>request<{entries:LeaderboardEntry[]}>('/v1/public/leaderboards/pvp',undefined,false),
};

const friendly:Record<string,string>={challenge_not_found:'This challenge link is not valid.',challenge_expired:'This challenge has expired.',self_challenge:"You can't accept your own challenge.",challenge_taken:'Another developer accepted this challenge first.',challenge_not_open:'This challenge is no longer open.',setup_locked:'Your harbour is already locked for this battle.',invalid_period:'Choose a complete 10-week contribution period.',invalid_fleet:'Place all five ships in bounds without overlap.',game_not_found:'This battle is unavailable.',not_your_turn:"It isn't your turn yet.",duplicate_shot:'That cell has already been targeted.',game_complete:'This battle is already complete.'};
export function friendlyError(error:unknown,fallback='Something went wrong. Try again.') { return error instanceof ApiError?(friendly[error.code]||fallback):fallback; }
