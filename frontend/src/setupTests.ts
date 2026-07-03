import '@testing-library/jest-dom';

// Mock scrollIntoView
window.HTMLElement.prototype.scrollIntoView = function() {};

// Mock fetch globally
globalThis.fetch = async (input: RequestInfo | URL) => {
  const url = input.toString();
  if (url.includes('/projects')) {
    if (url.includes('/slides')) {
      return { ok: true, status: 200, json: async () => [
        { id: 's1', project_id: 'p1', idx: 0, layout: 'title', title: 'Slide 1', html_path: '/slides/p1/s1.html', json_path: '/slides/p1/s1.json', current_version: 1 },
        { id: 's2', project_id: 'p1', idx: 1, layout: 'content', title: 'Slide 2', html_path: '/slides/p1/s2.html', json_path: '/slides/p1/s2.json', current_version: 1 }
      ] } as unknown as Response;
    }
    if (url.includes('/threads')) return { ok: true, status: 200, json: async () => [] } as unknown as Response;
    return { ok: true, status: 200, json: async () => [
      { id: 'p1', title: 'Project 1', work_dir: '.ppt-workspace/p1', theme: 'default', status: 'draft', design_path: 'projects/p1/design/tokens.css', created_at: '', updated_at: '' },
      { id: 'p2', title: 'Project 2', work_dir: '.ppt-workspace/p2', theme: 'default', status: 'ready', design_path: 'projects/p2/design/tokens.css', created_at: '', updated_at: '' }
    ] } as unknown as Response;
  }
  return { ok: true, status: 200, json: async () => ({}) } as unknown as Response;
};
