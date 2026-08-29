import { describe, expect, it } from 'vitest';
import { canPlace, placement, rangeLabel } from './logic';
describe('deployment', () => {
  it('builds straight ships and rejects collision or bounds', () => {
    const cells = placement('Destroyer', { x: 8, y: 2 }, false);
    expect(cells).toEqual([
      { x: 8, y: 2 },
      { x: 9, y: 2 },
    ]);
    expect(canPlace(cells, [])).toBe(true);
    expect(canPlace(placement('Destroyer', { x: 9, y: 2 }, false), [])).toBe(
      false,
    );
    expect(
      canPlace(cells, [{ kind: 'Cruiser', cells: [{ x: 8, y: 2 }] }]),
    ).toBe(false);
  });
  it('labels selected range', () => {
    const days = Array.from({ length: 70 }, (_, i) => ({
      date: new Date(Date.UTC(2025, 0, 1 + i)).toISOString().slice(0, 10),
    }));
    expect(rangeLabel(days, 0)).toContain('Jan 1, 2025');
  });
});
