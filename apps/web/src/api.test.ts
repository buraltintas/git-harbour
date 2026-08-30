import{afterEach,describe,expect,it,vi}from'vitest';
import{api,friendlyError}from'./api';import{session}from'./session';

describe('API session boundary',()=>{
  afterEach(()=>{vi.unstubAllGlobals();session.clear()});
  it('exchanges the one-time login code without an existing bearer',async()=>{
    const fetchMock=vi.fn().mockResolvedValue(new Response(JSON.stringify({token:'app-token',user:{login:'alice'}}),{status:200,headers:{'Content-Type':'application/json'}}));
    vi.stubGlobal('fetch',fetchMock);await api.exchange('one-time');
    const init=fetchMock.mock.calls[0][1];
    expect(init.body).toContain('one-time');expect(init.headers.Authorization).toBeUndefined();
  });
  it('adds only the GitHarbour session to private requests',async()=>{
    session.set('app-token');
    const fetchMock=vi.fn().mockResolvedValue(new Response(JSON.stringify({login:'alice'}),{status:200,headers:{'Content-Type':'application/json'}}));
    vi.stubGlobal('fetch',fetchMock);await api.me();
    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe('Bearer app-token');
  });
  it('creates Solo without client board data and submits only shot coordinates',async()=>{
    session.set('app-token');
    const fetchMock=vi.fn().mockImplementation(()=>Promise.resolve(new Response(JSON.stringify({id:'g',game:{id:'g'}}),{status:200,headers:{'Content-Type':'application/json'}})));
    vi.stubGlobal('fetch',fetchMock);
    await api.create();
    expect(fetchMock.mock.calls[0][0]).toContain('/v1/games/solo');
    expect(fetchMock.mock.calls[0][1].body).toBe('{}');
    await api.shot('game-1',4,3);
    expect(fetchMock.mock.calls[1][0]).toContain('/v1/games/game-1/shots');
    expect(fetchMock.mock.calls[1][1].body).toBe('{"x":4,"y":3}');
  });
  it('maps the PvP refit response to intentional copy',async()=>{
    const fetchMock=vi.fn().mockResolvedValue(new Response(JSON.stringify({error:{code:'pvp_refit',message:'raw'}}),{status:503,headers:{'Content-Type':'application/json'}}));
    vi.stubGlobal('fetch',fetchMock);let message='';
    try{await api.acceptChallenge('mine')}catch(error){message=friendlyError(error)}
    expect(message).toContain('contribution-target');
  });
});
