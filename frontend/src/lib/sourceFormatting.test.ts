import { describe, expect, it } from 'vitest';
import { formatStrictJSON } from './sourceFormatting';

describe('source JSON formatting', () => {
  it('formats a valid object without silently accepting duplicate nested fields', async () => {
    await expect(formatStrictJSON('{"a":{"b":1}}')).resolves.toBe('{\n  "a": {\n    "b": 1\n  }\n}\n');
    await expect(formatStrictJSON('{"a":{"b":1,"b":2}}')).rejects.toThrow('JSON 字段重复：b');
    await expect(formatStrictJSON('{"a":1,}')).rejects.toThrow();
  });
});
