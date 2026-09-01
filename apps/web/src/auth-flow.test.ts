import {afterEach,describe,expect,it,vi} from 'vitest';
import {navigateAfterAuth} from './routing';

describe('GitHub authentication callback routing',()=>{
  afterEach(()=>history.replaceState(null,'',location.pathname+location.search+'#/'));

  it('leaves the callback route even when the URL already has the destination hash',()=>{
    history.replaceState(null,'',location.pathname+location.search+'#/');
    const setRoute=vi.fn();

    navigateAfterAuth('#/',setRoute);

    expect(setRoute).toHaveBeenCalledWith('#/');
    expect(location.hash).toBe('#/');
  });
});
