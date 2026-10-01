import { format } from 'prettier';
const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const source = Buffer.concat(chunks).toString('utf8');
process.stdout.write(await format(source, {
  parser: 'html', tabWidth: 2, printWidth: 100,
  htmlWhitespaceSensitivity: 'strict', embeddedLanguageFormatting: 'auto',
  endOfLine: 'lf',
}));
