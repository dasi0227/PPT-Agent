import { describe, expect, it } from 'vitest';
import { formatHTMLSource } from './htmlSourceFormatter';

describe('HTML source display formatter', () => {
  it('indents markup and embedded CSS and JavaScript', async () => {
    const source = '<!doctype html><html><head><style>body{color:red}</style></head><body><main><h1>Hello</h1></main><script>const options={enabled:true};</script></body></html>';
    const formatted = await formatHTMLSource(source);

    expect(formatted).toContain('\n  ><head');
    expect(formatted).toContain('<main><h1>Hello</h1></main');
    expect(formatted).toContain('body {\n');
    expect(formatted).toContain('color: red;');
    expect(formatted).toContain('const options = { enabled: true };');
    expect(formatted).not.toContain('\r');
  });

  it('preserves adjacent inline text when producing a display copy', async () => {
    const source = '<main><span>Hello</span><span>world</span></main>';
    const formatted = await formatHTMLSource(source);
    const parser = new DOMParser();
    expect(parser.parseFromString(formatted, 'text/html').querySelector('main')?.textContent)
      .toBe(parser.parseFromString(source, 'text/html').querySelector('main')?.textContent);
  });
});
