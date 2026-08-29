import type{Coord,Ship}from'./types';import{fleetSizes}from'./types';
export function placement(kind:string,start:Coord,vertical:boolean):Coord[]{return Array.from({length:fleetSizes[kind]},(_,i)=>({x:start.x+(vertical?0:i),y:start.y+(vertical?i:0)}))}
export function canPlace(cells:Coord[],ships:Ship[]):boolean{const occupied=new Set(ships.flatMap(s=>s.cells.map(c=>`${c.x}:${c.y}`)));return cells.every(c=>c.x>=0&&c.x<10&&c.y>=0&&c.y<7&&!occupied.has(`${c.x}:${c.y}`))}
export function rangeLabel(days:{date:string}[],week:number){const fmt=(v:string)=>new Intl.DateTimeFormat('en',{month:'short',day:'numeric',year:'numeric',timeZone:'UTC'}).format(new Date(v+'T00:00:00Z'));return `${fmt(days[week*7].date)} – ${fmt(days[week*7+69].date)}`}
