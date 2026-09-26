import * as prettier from 'prettier/standalone';
import * as babelPlugin from 'prettier/plugins/babel';
import * as estreePlugin from 'prettier/plugins/estree';
import * as htmlPlugin from 'prettier/plugins/html';
import * as postcssPlugin from 'prettier/plugins/postcss';

export function formatHTMLSource(source: string): Promise<string> {
  return prettier.format(source, {
    parser: 'html',
    plugins: [htmlPlugin, postcssPlugin, babelPlugin, estreePlugin],
    tabWidth: 2,
    printWidth: 100,
    htmlWhitespaceSensitivity: 'css',
    embeddedLanguageFormatting: 'auto',
  });
}
