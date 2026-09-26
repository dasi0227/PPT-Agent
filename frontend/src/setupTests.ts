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
  if (url.includes('/api/v1/snippets') || url.includes('/api/v1/components')) {
    const body = url.includes('/snippets') ? { snippets: [] } : { components: [] };
    return { ok: true, status: 200, json: async () => body } as unknown as Response;
  }
  if (url.includes('/api/v1/skills')) {
    const body = {
      skills: [
        { id: 'story', name: '演示叙事', description: '梳理页面叙事。', tags: [], disabled: false, content_state: 'ready', open_url: '' },
        { id: 'visual', name: '视觉层级', description: '优化页面信息层级。', tags: [], disabled: false, content_state: 'ready', open_url: '' },
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
        project_id: 'p1',
        theme: 'clean',
        hashes: { outline: "outline-hash" },
        manifest: { title: 'Project 1', goal: '', audience: '', language: 'zh-CN', pages: '待明确', requirements: [], prohibitions: [] },
        outline: { sections: [{ id: 'sec_test', title: 'Section', purpose: '', slides: [{ slide_id: 's1', title: 'Slide 1' }, { slide_id: 's2', title: 'Slide 2' }], subsections: [] }] },
        design: { direction: 'minimal', layout_preferences: [], decorations: { page_number: 'bottom-right', deck_title: 'none', section_title: 'top-left', key_message: 'none' } },
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
