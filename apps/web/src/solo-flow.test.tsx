import {fireEvent,render,screen} from '@testing-library/react';
import {describe,expect,it,vi} from 'vitest';
import {dayPower,fleetCapacity,playableWindows} from './solo';
import {ContributionGrid} from './components/ContributionGrid';
import type {Day,FleetCell} from './types';

function days(count=84):Day[]{
  const start=Date.UTC(2025,0,5);
  return Array.from({length:count},(_,i)=>({date:new Date(start+i*86400000).toISOString().slice(0,10),weekday:i%7,contributionCount:i%9===0?2:0,contributionLevel:i%9===0?1:0}));
}

describe('contribution fleet Solo UI model',()=>{
  it('offers week-aligned frozen ten-week player windows',()=>{
    const choices=playableWindows(days());
    expect(choices).toHaveLength(3);
    expect(choices[0].cells).toHaveLength(70);
    expect(choices[1].start).toBe(days()[7].date);
    expect(choices[0].activeDays).toBeGreaterThan(0);
    expect(choices[0].fleetCapacity).toBe(fleetCapacity(choices[0].activeDays));
  });

  it('finds Sunday-aligned windows when imported history begins midweek',()=>{
    const start=Date.UTC(2025,0,2); // Thursday
    const history=Array.from({length:84},(_,i):Day=>({
      date:new Date(start+i*86400000).toISOString().slice(0,10),
      weekday:(4+i)%7,
      contributionCount:i%9===0?2:0,
      contributionLevel:i%9===0?1:0,
    }));
    const choices=playableWindows(history);
    expect(choices).toHaveLength(2);
    expect(choices[0].start).toBe('2025-01-05');
    expect(choices[0].cells[0].weekday).toBe(0);
  });

  it('keeps zero-contribution periods playable and power monotonic',()=>{
    const zero=days().map(day=>({...day,contributionCount:0,contributionLevel:0}));
    expect(playableWindows(zero)[0].fleetCapacity).toBe(3);
    expect(dayPower(600)).toBeGreaterThan(dayPower(10));
  });

  it('allows only untargeted enemy cells and supports deployment selection',()=>{
    const fire=vi.fn();
    const enemy:FleetCell[]=[{x:0,y:0,state:'unknown'},{x:0,y:1,state:'unknown',targeted:true}];
    const own:FleetCell[]=[{x:0,y:0,state:'eligible',date:'2025-01-05',weekday:0,contributionCount:4,contributionLevel:2,combatLevel:1}];
    const{rerender}=render(<ContributionGrid cells={enemy} interactive onCell={fire} label="enemy"/>);
    fireEvent.click(screen.getByRole('gridcell',{name:/Week 1, weekday 1/}));
    expect(fire).toHaveBeenCalledWith(0,0);
    expect(screen.getByRole('gridcell',{name:/weekday 2/})).toBeDisabled();
    rerender(<ContributionGrid cells={own} selectable onCell={fire} label="own"/>);
    const ownCell=screen.getByRole('gridcell',{name:/4 contributions/});
    expect(ownCell).not.toBeDisabled();
    expect(ownCell).toHaveAttribute('aria-disabled','false');
  });
});
