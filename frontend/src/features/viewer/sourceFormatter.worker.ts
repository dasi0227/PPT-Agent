import * as prettier from 'prettier/standalone';
import html from 'prettier/plugins/html';
import babel from 'prettier/plugins/babel';
import estree from 'prettier/plugins/estree';
import postcss from 'prettier/plugins/postcss';

self.onmessage = async (event: MessageEvent<{ id: number; text: string }>) => {
  const { id, text } = event.data;
  try {
    const formatted = await prettier.format(text, {
      parser: 'html', plugins: [html, babel, estree, postcss],
      tabWidth: 2, useTabs: false, endOfLine: 'lf', printWidth: 100,
      embeddedLanguageFormatting: 'auto', htmlWhitespaceSensitivity: 'strict',
    });
    self.postMessage({ id, formatted });
  } catch (error) {
    self.postMessage({ id, error: error instanceof Error ? error.message : String(error) });
  }
};
