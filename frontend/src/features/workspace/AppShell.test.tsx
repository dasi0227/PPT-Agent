import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { AppShell } from './AppShell';
import { useProjectStore } from '../../stores/projectStore';
import { useUIStore } from '../../stores/uiStore';

// Mock react-resizable-panels components for JSDOM
vi.mock('react-resizable-panels', () => ({
  PanelGroup: ({ children }: any) => <div data-testid="panel-group">{children}</div>,
  Panel: ({ children, id }: any) => <div data-testid={`panel-${id}`}>{children}</div>,
  PanelResizeHandle: () => <div data-testid="panel-handle" />
}));

describe('AppShell', () => {
  beforeEach(() => {
    useProjectStore.setState({ activeProjectId: null });
    useUIStore.setState({ leftPanelHidden: false, rightPanelHidden: false });
  });

  it('renders WorkspaceEmptyState when no active project', () => {
    render(<AppShell />);
    expect(screen.getByText('Welcome back')).toBeInTheDocument();
  });

  it('renders PanelGroup when active project exists', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    render(<AppShell />);
    expect(screen.getByTestId('panel-group')).toBeInTheDocument();
    expect(screen.getByTestId('panel-left')).toBeInTheDocument();
    expect(screen.getByTestId('panel-center')).toBeInTheDocument();
    expect(screen.getByTestId('panel-right')).toBeInTheDocument();
  });

  it('hides left panel when leftPanelHidden is true', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useUIStore.setState({ leftPanelHidden: true });
    render(<AppShell />);
    expect(screen.queryByTestId('panel-left')).not.toBeInTheDocument();
    expect(screen.getByTestId('panel-center')).toBeInTheDocument();
  });

  it('hides right panel when rightPanelHidden is true', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useUIStore.setState({ rightPanelHidden: true });
    render(<AppShell />);
    expect(screen.queryByTestId('panel-right')).not.toBeInTheDocument();
    expect(screen.getByTestId('panel-center')).toBeInTheDocument();
  });
});
