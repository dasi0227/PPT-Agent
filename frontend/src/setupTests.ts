import '@testing-library/jest-dom';

// Mock scrollIntoView
window.HTMLElement.prototype.scrollIntoView = function() {};

// Mock localStorage
const localStorageMock = (function() {
  let store: Record<string, string> = {};
  return {
    getItem: function(key: string) { return store[key] || null; },
    setItem: function(key: string, value: string) { store[key] = value.toString(); },
    removeItem: function(key: string) { delete store[key]; },
    clear: function() { store = {}; }
  };
})();
Object.defineProperty(window, 'localStorage', { value: localStorageMock });
Object.defineProperty(window, 'sessionStorage', { value: localStorageMock });

// Mock fetch globally
globalThis.fetch = async (input: RequestInfo | URL) => {
  const url = input.toString();
  if (url.includes('/api/v1/llm/profiles')) {
    const body = {
      default: 'Kimi K3',
      profiles: [
        {
          name: 'Kimi K3',
          model: 'kimi-k3',
          capabilities: { vision: true, tool_calls: true, multiple_tool_calls: true },
        },
        {
          name: 'DeepSeek V4 Pro',
          model: 'deepseek-v4-pro',
          capabilities: { vision: false, tool_calls: true, multiple_tool_calls: true },
        },
      ],
    };
    return {
      ok: true,
      status: 200,
      json: async () => body,
      text: async () => JSON.stringify(body),
    } as unknown as Response;
  }
  if (/\/api\/v1\/slides\/[^/]+\/render$/.test(url)) {
    return {
      ok: true,
      status: 200,
      text: async () => '<!doctype html><html><body>mock slide</body></html>',
    } as unknown as Response;
  }
  if (url.includes('/projects')) {
    if (url.includes('/slides')) {
      return { ok: true, status: 200, json: async () => [
        { id: 's1', project_id: 'p1', position: 0, layout: 'title', title: 'Slide 1', html_path: '/slides/p1/s1.html', spec_path: '/slides/p1/s1.json', current_version: 1 },
        { id: 's2', project_id: 'p1', position: 1, layout: 'content', title: 'Slide 2', html_path: '/slides/p1/s2.html', spec_path: '/slides/p1/s2.json', current_version: 1 }
      ] } as unknown as Response;
    }
    if (url.includes('/threads')) return { ok: true, status: 200, json: async () => [] } as unknown as Response;
    return { ok: true, status: 200, json: async () => [
      { id: 'p1', title: 'Project 1', work_dir: '', theme: 'default', status: 'draft', design_path: '', created_at: 0, updated_at: 0 },
      { id: 'p2', title: 'Project 2', work_dir: '', theme: 'default', status: 'draft', design_path: '', created_at: 0, updated_at: 0 }
    ] } as unknown as Response;
  }
  return { ok: true, status: 200, json: async () => ({}) } as unknown as Response;
};
