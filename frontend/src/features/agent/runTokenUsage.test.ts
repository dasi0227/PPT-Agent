import { describe, expect, it } from 'vitest';
import { formatRunTokenUsage, type RunTokenUsage } from './runTokenUsage';

const usage = (total: number, calls = 1, unknown = 0): RunTokenUsage => ({
  total_tokens: total, input_tokens: total, output_tokens: 0, calls, unknown_calls: unknown,
});
describe('committed token totals', () => {
  it('distinguishes unknown usage from a reported zero', () => {
    expect(formatRunTokenUsage(undefined)).toBeNull();
    expect(formatRunTokenUsage(usage(0, 2, 2))).toBeNull();
    expect(formatRunTokenUsage(usage(0))).toBe('0');
    expect(formatRunTokenUsage(usage(130, 3, 1))).toBe('130');
    expect(formatRunTokenUsage(usage(1472122))).toBe('1.47M');
  });
});
