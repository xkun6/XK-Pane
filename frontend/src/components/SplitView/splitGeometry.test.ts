import { describe, expect, it } from 'vitest';
import { computeSashResize, normalizeSizesToHundred } from './splitGeometry';

describe('computeSashResize', () => {
  it('基本拖拽：把 delta% 从右 pane 移到左 pane', () => {
    expect(computeSashResize([50, 50], 0, 10, 5)).toEqual([60, 40]);
  });

  it('左 pane 触底最小值 → 钳到 min，溢出转嫁右 pane', () => {
    // newLeft=2 < 5 → newRight += 2-5 → 98-3=95，newLeft=5
    expect(computeSashResize([50, 50], 0, -48, 5)).toEqual([5, 95]);
  });

  it('右 pane 触底最小值 → 钳到 min，溢出转嫁左 pane', () => {
    // newRight=2 < 5 → newLeft += 2-5 → 98-3=95，newRight=5
    expect(computeSashResize([50, 50], 0, 48, 5)).toEqual([95, 5]);
  });

  it('多子：只动相邻两 pane，其余不变', () => {
    expect(computeSashResize([33, 33, 34], 1, 10, 5)).toEqual([33, 43, 24]);
  });
});

describe('normalizeSizesToHundred', () => {
  it('已是 100 → 不变', () => {
    expect(normalizeSizesToHundred([50, 50])).toEqual([50, 50]);
    expect(normalizeSizesToHundred([20, 50, 30])).toEqual([20, 50, 30]);
  });

  it('非 100 → 归一化且末位吸收舍入，总和恰为 100', () => {
    const out = normalizeSizesToHundred([60, 30]); // total 90
    expect(out).toEqual([66.7, 33.3]);
    expect(out.reduce((a, b) => a + b, 0)).toBeCloseTo(100, 5);
  });

  it('total<=0 / 空数组 → 原样返回', () => {
    expect(normalizeSizesToHundred([])).toEqual([]);
    expect(normalizeSizesToHundred([0, 0])).toEqual([0, 0]);
  });
});
