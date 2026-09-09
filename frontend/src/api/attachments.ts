import { fetchClient } from './client';

export interface ImageAttachment {
  id: string;
  project_id: string;
  original_name: string;
  media_type: 'image/png' | 'image/jpeg' | 'image/webp';
  extension: 'png' | 'jpg' | 'webp';
  size_bytes: number;
  width: number;
  height: number;
  created_at: number;
}

export const attachmentsApi = {
  upload: (projectId: string, file: File) => {
    const body = new FormData();
    body.append('file', file, file.name);
    return fetchClient<ImageAttachment>(`/projects/${projectId}/attachments`, {
      method: 'POST', body, reportError: false, timeoutMs: 30_000,
    });
  },
};
