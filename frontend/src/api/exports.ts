import { fetchClient } from './client';

export type ExportFormat = 'png' | 'pdf' | 'html';

function readFilename(disposition: string): string | undefined {
  const encoded = disposition.match(/(?:^|;)\s*filename\*=UTF-8'[^']*'([^;]+)/i);
  if (encoded) {
    try { return decodeURIComponent(encoded[1].trim()); } catch { /* Use the plain filename if supplied. */ }
  }
  const plain = disposition.match(/(?:^|;)\s*filename\s*=\s*(?:"((?:[^"\\]|\\.)*)"|([^;]+))/i);
  return plain?.[1]?.replace(/\\(.)/g, '$1') ?? plain?.[2]?.trim();
}

export const exportsApi = {
  async create(project_id: string, format: ExportFormat) {
    const response = await fetchClient<{ blob: Blob; headers: Headers }>('/exports/create', {
      method: 'POST',
      body: JSON.stringify({ project_id, format }),
      responseType: 'blob',
      timeoutMs: 0,
      reportError: false,
    });
    const filename = readFilename(response?.headers.get('Content-Disposition') ?? '');
    if (!filename) throw new Error('导出响应缺少文件名');
    return { blob: response.blob, filename };
  },
};
