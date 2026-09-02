import { act, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { useUIStore } from '../../stores/uiStore';
import { AppShell } from './AppShell';

vi.mock('react-resizable-panels', () => ({
  PanelGroup: ({ children, autoSaveId }: { children: React.ReactNode; autoSaveId: string }) => (
    <div data-testid="panel-group" data-autosave-id={autoSaveId}>{children}</div>
  ),
  Panel: ({
    children,
    id,
    defaultSize,
    minSize,
    maxSize,
  }: {
    children: React.ReactNode;
    id: string;
    defaultSize: number;
    minSize: number;
    maxSize?: number;
  }) => (
    <div
      data-testid={`panel-${id}`}
      data-default-size={defaultSize}
      data-min-size={minSize}
      data-max-size={maxSize}
    >{children}</div>
  ),
  PanelResizeHandle: ({ 'aria-label': label }: { 'aria-label': string }) => <div role="separator" aria-label={label} />,
}));

vi.mock('./ProjectTabs', () => ({ ProjectTabs: () => <nav data-testid="project-tabs">项目标签</nav> }));
vi.mock('../deck/DeckNavigator', () => ({ DeckNavigator: () => <aside>幻灯片导航</aside> }));
vi.mock('../viewer/PreviewWorkspace', () => ({ PreviewWorkspace: () => <main>预览区</main> }));
vi.mock('../agent/AgentPanel', () => ({ AgentPanel: () => <aside>Agent 对话</aside> }));

describe('AppShell layout contract', () => {
  beforeEach(() => {
    useProjectStore.setState({ activeProjectId: null });
    useUIStore.setState({ leftPanelHidden: false, rightPanelHidden: false });
  });

  it('hides the top project tabs on the home empty state', () => {
    render(<AppShell />);
    expect(screen.queryByTestId('project-tabs')).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Dasi PPT Agent' })).toBeInTheDocument();
    expect(screen.getByText('Agent')).toBeInTheDocument();
    expect(screen.getByText('PPT')).toBeInTheDocument();
    expect(screen.getByText('Dasi')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '新建 / 打开项目' })).toBeInTheDocument();
    expect(screen.queryByTestId('lucide-layers')).not.toBeInTheDocument();
  });

  it('keeps the left, center, and right columns with protected size bounds', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    render(<AppShell />);
    expect(screen.getByTestId('project-tabs')).toBeInTheDocument();
    expect(screen.getByTestId('panel-left')).toHaveAttribute('data-default-size', '22');
    expect(screen.getByTestId('panel-left')).toHaveAttribute('data-min-size', '16');
    expect(screen.getByTestId('panel-left')).toHaveAttribute('data-max-size', '32');
    expect(screen.getByTestId('panel-center')).toHaveAttribute('data-min-size', '30');
    expect(screen.getByTestId('panel-right')).toHaveAttribute('data-default-size', '40');
    expect(screen.getByTestId('panel-right')).toHaveAttribute('data-min-size', '40');
    expect(screen.getByTestId('panel-right')).toHaveAttribute('data-max-size', '40');
    expect(screen.getAllByRole('separator')).toHaveLength(2);
  });

  it('hides and restores each sidebar independently with a stable persistence key', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    render(<AppShell />);
    const expandedKey = screen.getByTestId('panel-group').getAttribute('data-autosave-id');

    act(() => useUIStore.getState().toggleLeftPanel());
    expect(screen.queryByTestId('panel-left')).not.toBeInTheDocument();
    expect(screen.getByTestId('panel-right')).toBeInTheDocument();
    expect(screen.getByTestId('panel-center')).toHaveAttribute('data-default-size', '60');

    act(() => useUIStore.getState().toggleRightPanel());
    expect(screen.queryByTestId('panel-right')).not.toBeInTheDocument();
    expect(screen.getByTestId('panel-center')).toBeInTheDocument();
    expect(screen.getByTestId('panel-center')).toHaveAttribute('data-default-size', '100');

    act(() => {
      useUIStore.getState().toggleLeftPanel();
      useUIStore.getState().toggleRightPanel();
    });
    expect(screen.getByTestId('panel-left')).toBeInTheDocument();
    expect(screen.getByTestId('panel-right')).toBeInTheDocument();
    expect(screen.getByTestId('panel-center')).toHaveAttribute('data-default-size', '38');
    expect(screen.getByTestId('panel-group')).toHaveAttribute('data-autosave-id', expandedKey);
  });

  it('does not mix panel visibility when switching projects', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useUIStore.setState({ leftPanelHidden: true, rightPanelHidden: false });
    render(<AppShell />);
    act(() => useProjectStore.setState({ activeProjectId: 'p2' }));
    expect(screen.queryByTestId('panel-left')).not.toBeInTheDocument();
    expect(screen.getByTestId('panel-right')).toBeInTheDocument();
  });
});
