import type {Day} from './types';

export function playableWindows(days:Day[]){
  const out:{start:string;end:string;cells:Day[];targets:number}[]=[];
  for(let i=0;i+70<=days.length;i+=7){
    const cells=days.slice(i,i+70);
    if(cells[0].weekday!==0||cells.some((day,index)=>day.weekday!==index%7||index>0&&Date.parse(day.date)-Date.parse(cells[index-1].date)!==86400000))continue;
    const targets=cells.filter(day=>day.contributionCount>0).length;
    if(targets>0)out.push({start:cells[0].date,end:cells[69].date,cells,targets});
  }
  const distance=(targets:number)=>targets<10?10-targets:targets>45?targets-45:0;
  const best=Math.min(...out.map(window=>distance(window.targets)));
  return out.filter(window=>distance(window.targets)===best);
}
