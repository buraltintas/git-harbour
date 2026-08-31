import {useMemo,useState} from 'react';
import type {Day,FleetCell} from '../types';

type Props={days?:Day[];cells?:FleetCell[];weeks?:number;onCell?:(x:number,y:number)=>void;interactive?:boolean;selectable?:boolean;inspectable?:boolean;label:string;busy?:boolean};

export function ContributionGrid({days,cells,weeks=10,onCell,interactive,selectable,inspectable,label,busy}:Props){
  const[active,setActive]=useState(0);
  const items=useMemo<FleetCell[]>(()=>cells||days?.map((d,i)=>({x:Math.floor(i/7),y:i%7,state:d.contributionCount>0?'eligible':'empty',...d}))||[],[cells,days]);
  const operable=!!interactive||!!selectable;
  const navigable=operable||!!inspectable;
  const available=(i:number)=>operable&&!busy&&(selectable?['eligible','empty','deployed','reserve','exposed','selected'].includes(items[i]?.state):(['unknown','exposed'].includes(items[i]?.state)&&!items[i]?.targeted));
  const firstAvailable=items.findIndex((_,i)=>available(i));
  const focusIndex=inspectable?active:available(active)?active:Math.max(0,firstAvailable);
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
    const level=cell.combatLevel??cell.contributionLevel??0;
    const coordinate=`Week ${cell.x+1}, weekday ${cell.y+1}`;
    const detail=cell.date?`${cell.date}, ${cell.contributionCount||0} contributions${cell.unitKind?`, ${cell.unitKind} unit, combat level ${cell.combatLevel}`:''}${state==='eliminated'?', eliminated':state==='exposed'?', exposed':state==='miss'?', targeted miss':''}`:state==='unknown'?'unexplored':state==='miss'?'targeted miss':state==='eliminated'?'eliminated unit':state==='exposed'?`exposed unit, combat level ${cell.combatLevel}`:'empty water';
    const title=cell.date?`${cell.date}: ${cell.contributionCount||0} contributions`:`${coordinate}: ${detail}`;
    const enabled=available(i);
    return <button key={`${cell.x}:${cell.y}`} type="button" role="gridcell" className={`cell target-cell level-${level} ${state}${inspectable?' inspectable':''}`} aria-label={`${coordinate}, ${detail}`} aria-disabled={!enabled} aria-selected={state==='selected'||undefined} title={title} onFocus={()=>setActive(i)} onKeyDown={e=>move(e,i)} tabIndex={navigable&&(inspectable||enabled)?(i===focusIndex?0:-1):-1} onClick={()=>enabled&&onCell?.(cell.x,cell.y)} disabled={!enabled&&!inspectable}><span aria-hidden="true">{state==='eliminated'?'×':state==='deployed'||state==='reserve'||state==='selected'?'◆':state==='exposed'?'!':state==='miss'?'•':''}</span></button>
  })}</div></div>
}
