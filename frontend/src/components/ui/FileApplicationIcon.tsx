import { useState } from 'react';
import { Box } from 'lucide-react';

const logos: Record<string, string> = {
  finder: 'finder.png', textedit: 'textedit.png', vscode: 'vscode.png',
  obsidian: 'obsidian.svg', typora: 'typora.png', webstorm: 'webstorm.png', cursor: 'cursor.png',
};
const appIds: Record<string, string> = {
  finder: 'finder', textedit: 'textedit', 'visual studio code': 'vscode', 'vs code': 'vscode',
  obsidian: 'obsidian', typora: 'typora', webstorm: 'webstorm', cursor: 'cursor',
};

/** Unknown applications and missing artwork share the same sized Box icon. */
export function FileApplicationIcon({ application, size = 30 }: { application?: string; size?: number }) {
  const id = application ? appIds[application.replace(/\.app$/i, '').trim().toLowerCase()] ?? application : undefined;
  const logo = id ? logos[id] : undefined;
  const [failed, setFailed] = useState<string>();
  return logo && failed !== logo
    ? <img src={`/file-app-icons/${logo}`} alt="" aria-hidden="true" width={size} height={size}
        style={{ width: size, height: size, objectFit: 'contain', flexShrink: 0 }} onError={() => setFailed(logo)} />
    : <Box aria-hidden="true" size={size} strokeWidth={1.75} className="shrink-0 text-text-500" style={{ width: size, height: size }} />;
}
