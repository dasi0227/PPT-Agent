import { fetchClient } from './client';

export const attachmentsApi = {
  contentUrl: (projectId: string, attachmentId: string, variant: 'thumbnail' | 'original' = 'thumbnail') =>
    `/api/projects/${encodeURIComponent(projectId)}/attachments/${encodeURIComponent(attachmentId)}?variant=${variant}`,
  upload: (projectId: string, file: File) => {
    const body = new FormData();
    body.append('project_id', projectId);
    body.append('file', file, file.name);
    return fetchClient<{ id: string }>('/attachments/upload', {
      method: 'POST', body, reportError: false, timeoutMs: 30_000,
    });
  },
};
