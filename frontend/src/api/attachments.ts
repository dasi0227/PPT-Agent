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
}

export const attachmentsApi = {
  contentUrl: (projectId: string, attachmentId: string, variant: 'thumbnail' | 'original' = 'thumbnail') =>
    `/api/v1/projects/${encodeURIComponent(projectId)}/attachments/${encodeURIComponent(attachmentId)}/content?variant=${variant}`,
  upload: (projectId: string, file: File) => {
    const body = new FormData();
    body.append('file', file, file.name);
    return fetchClient<ImageAttachment>(`/projects/${projectId}/attachments`, {
      method: 'POST', body, reportError: false, timeoutMs: 30_000,
    });
  },
};
