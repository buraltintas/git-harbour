import type {Day} from './types';

export const reservePower=1;
export function dayPower(count:number){return count>0?1+Math.log2(1+count):0}
export function combatLevel(power:number){return power<=reservePower?0:power<4?1:power<7?2:power<10?3:4}
export function fleetCapacity(activeDays:number){return Math.max(3,Math.min(14,Math.round(3+Math.sqrt(12*Math.max(0,Math.min(70,activeDays))/7))))}

export function playableWindows(days:Day[]){
  const out:{start:string;end:string;cells:Day[];totalContributions:number;activeDays:number;contributionPower:number;fleetCapacity:number;peakCount:number;peakDate:string;maxDeployedPower:number}[]=[];
  for(let i=0;i+70<=days.length;i+=7){
    const cells=days.slice(i,i+70);
    if(cells[0].weekday!==0||cells.some((day,index)=>day.weekday!==index%7||index>0&&Date.parse(day.date)-Date.parse(cells[index-1].date)!==86400000))continue;
    const active=cells.filter(day=>day.contributionCount>0);
    const capacity=fleetCapacity(active.length);
    const powers=active.map(day=>dayPower(day.contributionCount)).sort((a,b)=>b-a);
    const peak=active.reduce<Day|undefined>((best,day)=>!best||day.contributionCount>best.contributionCount?day:best,undefined);
    out.push({start:cells[0].date,end:cells[69].date,cells,totalContributions:cells.reduce((sum,day)=>sum+day.contributionCount,0),activeDays:active.length,contributionPower:powers.reduce((a,b)=>a+b,0),fleetCapacity:capacity,peakCount:peak?.contributionCount||0,peakDate:peak?.date||'',maxDeployedPower:Array.from({length:capacity},(_,n)=>powers[n]??reservePower).reduce((a,b)=>a+b,0)});
  }
  return out;
}
