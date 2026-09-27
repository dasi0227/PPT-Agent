import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const source = readFileSync('public/interaction-tokens.css', 'utf8');
const readTokens = (css: string) => Object.fromEntries([...css.matchAll(/--ui-([\w-]+):\s*(\d+) (\d+) (\d+);/g)].map((m) => [m[1], m.slice(2).map(Number)]));
const light = readTokens(source.split(":root[data-color-mode='dark']")[0]);
const dark = { ...light, ...readTokens(source.split(":root[data-color-mode='dark']")[1]) };
const luminance = (rgb: number[]) => rgb.map(v => v / 255).map(v => v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4).reduce((sum, v, i) => sum + v * [.2126, .7152, .0722][i], 0);
const contrast = (a: number[], b: number[]) => {
  const values = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (values[0] + .05) / (values[1] + .05);
};

describe.each([['light', light], ['dark', dark]] as const)('%s palette', (_, palette) => {
  it('keeps body, secondary text and selected/semantic labels readable', () => {
    const pairs = [
      ['foreground', 'surface'], ['text-600', 'surface'], ['text-600', 'timeline-card'],
      ['text-400', 'panel'], ['selected-foreground', 'selected'], ['success', 'success-soft'],
      ['warning-foreground', 'warning-soft'], ['danger-hover', 'danger-soft'],
      ['on-solid', 'success-solid'], ['on-solid', 'danger-solid'],
      ['code-key', 'surface'], ['code-string', 'surface'], ['code-comment', 'surface'],
    ];
    for (const [fg, bg] of pairs) expect(contrast(palette[fg], palette[bg]), `${fg} on ${bg}`).toBeGreaterThanOrEqual(4.5);
  });
});
