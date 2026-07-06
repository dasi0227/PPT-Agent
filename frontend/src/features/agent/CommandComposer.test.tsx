import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it } from 'vitest';
import { CommandComposer } from './CommandComposer';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { useComposerStore } from '../../stores/composerStore';

class MockEventSource {
  onerror: ((event: Event) => void) | null = null;
  addEventListener() {}
  close() {}
}

function setupStores(slides: Array<{ html_path?: string }> = []) {
  useProjectStore.setState({
    projects: [],
    activeProjectId: 'p1',
    slidesByProjectId: {
      p1: slides.map((s, i) => ({
        id: `s${i}`, project_id: 'p1', idx: i, layout: 'bullets',
        title: `Slide ${i}`, html_path: s.html_path ?? '', json_path: '', current_version: 1,
      })),
    },
    loadingProjects: false,
  });
  useThreadStore.setState({
    threadsByProjectId: { p1: [{ id: 't1', project_id: 'p1', title: 'Thread', created_at: 0, updated_at: 0 }] },
    openThreadIdsByProjectId: { p1: ['t1'] },
    activeThreadIdByProjectId: { p1: 't1' },
  });
  useDeckStore.setState({ currentPage: 0, previewMode: 'main' });
  useRunStore.setState({ sessions: {} });
  useComposerStore.setState({
    interactionMode: 'outline',
    subMode: 'normal',
    userTouchedMode: false,
    focusNonce: 0,
  });
}

function mockFetch() {
  const requests: Array<{ url: string; body: any }> = [];
  globalThis.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    requests.push({
      url: input.toString(),
      body: init?.body ? JSON.parse(init.body.toString()) : undefined,
    });
    return { ok: true, status: 200, json: async () => ({ id: 'r1' }) } as unknown as Response;
  };
  return requests;
}

describe('CommandComposer', () => {
  beforeEach(() => {
    globalThis.EventSource = MockEventSource as unknown as typeof EventSource;
  });

  it('empty project smart-defaults to Outline and sends kind=outline', async () => {
    setupStores([]);
    const user = userEvent.setup();
    const requests = mockFetch();

    render(<CommandComposer />);

    await user.type(screen.getByRole('textbox'), '给投资人讲我们的 AI 产品');
    await user.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(requests).toHaveLength(1));
    expect(requests[0].url).toBe('/api/v1/threads/t1/runs');
    expect(requests[0].body).toMatchObject({
      kind: 'outline',
      scope: 'current',
      mode: 'normal',
      instruction: '给投资人讲我们的 AI 产品',
    });
  });

  it('slash command is passed through raw without local scope/mode rewrite', async () => {
    setupStores([{ html_path: 'slides/000/index.html' }]);
    const user = userEvent.setup();
    const requests = mockFetch();

    render(<CommandComposer />);

    await user.type(screen.getByRole('textbox'), '/repo 把卡片圆角调大');
    await user.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(requests).toHaveLength(1));
    expect(requests[0].body).toMatchObject({
      kind: 'edit',
      instruction: '/repo 把卡片圆角调大',
    });
    // 不本地写入 scope/mode，交后端解析
    expect(requests[0].body.scope).toBeUndefined();
    expect(requests[0].body.mode).toBeUndefined();
  });

  it('Page mode with existing html sends kind=edit scope=current', async () => {
    setupStores([{ html_path: 'slides/000/index.html' }]);
    useComposerStore.setState({ interactionMode: 'page', userTouchedMode: true });
    const user = userEvent.setup();
    const requests = mockFetch();

    render(<CommandComposer />);

    await user.type(screen.getByRole('textbox'), '标题改大');
    await user.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(requests).toHaveLength(1));
    expect(requests[0].body).toMatchObject({
      kind: 'edit',
      scope: 'current',
      mode: 'normal',
      page_index: 0,
    });
  });
});
