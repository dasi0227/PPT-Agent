import '@testing-library/jest-dom';

// Mock scrollIntoView
window.HTMLElement.prototype.scrollIntoView = function() {};

// Mock fetch globally
globalThis.fetch = async (input: RequestInfo | URL) => {
  const url = input.toString();
  if (url.includes('/projects')) {
    if (url.includes('/slides')) {
      return { ok: true, status: 200, json: async () => [
        { id: 's1', project_id: 'p1', idx: 0, html_path: '/slides/p1/s1.html', notes: '', current_version: 1, created_at: '', updated_at: '' },
        { id: 's2', project_id: 'p1', idx: 1, html_path: '/slides/p1/s2.html', notes: '', current_version: 1, created_at: '', updated_at: '' }
      ] } as unknown as Response;
    }
    if (url.includes('/threads')) return { ok: true, status: 200, json: async () => [] } as unknown as Response;
    return { ok: true, status: 200, json: async () => [
      { id: 'p1', topic: 'Project 1', brief: '', theme: 'default', language: 'zh', slide_count: 2, created_at: '', updated_at: '' },
      { id: 'p2', topic: 'Project 2', brief: '', theme: 'default', language: 'zh', slide_count: 1, created_at: '', updated_at: '' }
    ] } as unknown as Response;
  }
  return { ok: true, status: 200, json: async () => ({}) } as unknown as Response;
};
