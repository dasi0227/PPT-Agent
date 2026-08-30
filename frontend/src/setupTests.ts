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
  if (url.includes('/api/v1/skills')) {
    const body = {
      skills: [
        { id: 'story', name: '演示叙事', description: '梳理页面叙事。' },
        { id: 'visual', name: '视觉层级', description: '优化页面信息层级。' },
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
    if (url.includes('/content')) {
      const body = {
        revision: 1,
        manifest: { version: '4.0', revision: 1, project_id: 'p1', title: 'Project 1', goal: '', audience: '', language: 'zh-CN', requirements: [], prohibitions: [], canvas: { aspect_ratio: '16:9' }, numbering: { enabled: true, hidden_roles: ['cover'], format: 'number' }, created_at: 0, updated_at: 0 },
        outline: { version: '4.0', revision: 1, project_id: 'p1', sections: [{ id: 'sec_test', title: 'Section', purpose: '', slides: [{ slide_id: 's1', title: 'Slide 1', role: 'cover' }, { slide_id: 's2', title: 'Slide 2', role: 'content' }], subsections: [] }], created_at: 0, updated_at: 0 },
        design: { version: '4.0', revision: 1, project_id: 'p1', theme: 'clean', direction: 'minimal', density: 'medium', chrome: [{ type: 'page_number', placement: 'bottom-right', style: 'muted' }], created_at: 0, updated_at: 0 },
        slides_by_id: {}, active_run: null,
      };
      return { ok: true, status: 200, json: async () => body, text: async () => JSON.stringify(body) } as unknown as Response;
    }
    if (url.includes('/threads')) return { ok: true, status: 200, json: async () => [] } as unknown as Response;
    return { ok: true, status: 200, json: async () => [
      { id: 'p1', title: 'Project 1', work_dir: '', theme: 'default', status: 'draft', design_path: '', created_at: 0, updated_at: 0 },
      { id: 'p2', title: 'Project 2', work_dir: '', theme: 'default', status: 'draft', design_path: '', created_at: 0, updated_at: 0 }
    ] } as unknown as Response;
  }
  return { ok: true, status: 200, json: async () => ({}) } as unknown as Response;
};
