import {useMemo,useState} from 'react';
import type {Day,TargetCell} from '../types';

type Props={days?:Day[];cells?:TargetCell[];weeks?:number;onCell?:(x:number,y:number)=>void;interactive?:boolean;label:string;busy?:boolean};

export function ContributionGrid({days,cells,weeks=10,onCell,interactive,label,busy}:Props){
  const[active,setActive]=useState(0);
  const items=useMemo<TargetCell[]>(()=>cells||days?.map((d,i)=>({x:Math.floor(i/7),y:i%7,state:d.contributionCount>0?'hit':'empty',...d}))||[],[cells,days]);
  const operable=!!interactive;
  const available=(i:number)=>operable&&items[i]?.state==='unknown'&&!busy;
  const firstAvailable=items.findIndex((_,i)=>available(i));
  const focusIndex=available(active)?active:Math.max(0,firstAvailable);
  function move(event:React.KeyboardEvent<HTMLButtonElement>,i:number){
    const x=Math.floor(i/7),y=i%7;
    let next=i;
    if(event.key==='ArrowRight')next=Math.min(items.length-1,(x+1)*7+y);
    else if(event.key==='ArrowLeft')next=Math.max(0,(x-1)*7+y);
    else if(event.key==='ArrowDown')next=Math.min(items.length-1,i+1);
    else if(event.key==='ArrowUp')next=Math.max(0,i-1);
    else return;
    event.preventDefault();
    const direction=event.key==='ArrowRight'||event.key==='ArrowDown'?1:-1;
    while(operable&&!available(next)&&next!==i){
      const candidate=next+direction;
      if(candidate<0||candidate>=items.length)break;
      next=candidate;
    }
    setActive(next);
    const target=event.currentTarget.parentElement?.children[next] as HTMLElement|undefined;
    target?.focus();target?.scrollIntoView({block:'nearest',inline:'nearest'});
  }
  return <div className="calendar-wrap"><div className={`contribution-grid weeks-${weeks}`} role="grid" aria-label={label} aria-busy={busy} style={{gridTemplateColumns:`repeat(${Math.max(1,Math.ceil(items.length/7))}, minmax(0, 1fr))`}}>{items.map((cell,i)=>{
    const state=cell.state;
    const level=state==='hit'?(cell.contributionLevel||1):0;
    const coordinate=`Week ${cell.x+1}, weekday ${cell.y+1}`;
    const detail=cell.date?`${cell.date}, ${cell.contributionCount||0} contributions${state==='hit'?`, level ${cell.contributionLevel||1}`:''}`:state==='unknown'?'unexplored':state==='miss'?'quiet day':`contribution found, ${cell.contributionCount||0} contributions, level ${cell.contributionLevel||1}`;
    const title=cell.date?`${cell.date}: ${cell.contributionCount||0} contributions`:`${coordinate}: ${detail}`;
    const enabled=available(i);
    return <button key={`${cell.x}:${cell.y}`} type="button" role="gridcell" className={`cell target-cell level-${level} ${state}`} aria-label={`${coordinate}, ${detail}`} title={title} onFocus={()=>setActive(i)} onKeyDown={e=>move(e,i)} tabIndex={operable&&enabled?(i===focusIndex?0:-1):-1} onClick={()=>enabled&&onCell?.(cell.x,cell.y)} disabled={!enabled}><span aria-hidden="true">{state==='hit'?'✓':state==='miss'?'•':''}</span></button>
  })}</div></div>
}
