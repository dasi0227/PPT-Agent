import '@testing-library/jest-dom';

// Mock scrollIntoView
window.HTMLElement.prototype.scrollIntoView = function() {};

// Mock fetch globally
global.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
  const url = input.toString();
  if (url.includes('/projects')) {
    if (url.includes('/slides')) {
      return { ok: true, status: 200, json: async () => [
        { id: 's1', project_id: 'p1', page_index: 0, content_html: '<h1>P1S1</h1>', version_no: 1, updated_at: '' },
        { id: 's2', project_id: 'p1', page_index: 1, content_html: '<h1>P1S2</h1>', version_no: 1, updated_at: '' }
      ] } as Response;
    }
    if (url.includes('/threads')) return { ok: true, status: 200, json: async () => [] } as Response;
    return { ok: true, status: 200, json: async () => [
      { id: 'p1', title: 'Project 1', theme: 'default', slide_count: 2, created_at: '', updated_at: '' },
      { id: 'p2', title: 'Project 2', theme: 'default', slide_count: 1, created_at: '', updated_at: '' }
    ] } as Response;
  }
  return { ok: true, status: 200, json: async () => ({}) } as Response;
};
