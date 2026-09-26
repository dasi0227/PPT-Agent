import {
  ChevronLeft, ChevronRight, LayoutGrid, MonitorPlay, MousePointer2,
  PanelLeftOpen, PanelRightOpen, Scan, ZoomIn, ZoomOut,
} from 'lucide-react';
import type { ExportFormat } from '../../api/exports';
import { IconButton } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';
import type { ContentMode, PageView } from '../../stores/deckStore';
import { ExportButton } from '../export/ExportButton';
import { ThemeSelector } from './ThemeSelector';
import './previewControls.css';

export interface PreviewSidebarControls {
  leftHidden: boolean;
  rightHidden: boolean;
  canExpandLeft: boolean;
  canExpandRight: boolean;
  onExpandLeft: () => void;
  onExpandRight: () => void;
}

type SelectionMode = 'element' | 'region' | 'none';

function StatusModeTabs({ label, value, options, onValueChange, disabled }: {
  label: string;
  value: string;
  options: readonly { value: string; label: string }[];
  onValueChange: (value: string) => void;
  disabled: boolean;
}) {
  return (
    <div role="group" aria-label={label} className="preview-mode-tabs">
      {options.map(option => (
        <button
          key={option.value}
          type="button"
          className="preview-mode-tab"
          aria-pressed={value === option.value}
          disabled={disabled}
          onClick={() => {
            if (value !== option.value) onValueChange(option.value);
          }}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

export function PreviewToolbar({
  projectId, contentMode, hasPages, overview, onToggleOverview,
  canPresent, onPresent, exportDisabled, exportDisabledReason, onExport,
  selectionMode, selectionEnabled, onSelectionModeChange, sidebarControls,
  pageControlsDisabled,
}: {
  projectId: string | null;
  contentMode: ContentMode;
  hasPages: boolean;
  overview: boolean;
  onToggleOverview: () => void;
  canPresent: boolean;
  onPresent: () => void;
  exportDisabled: boolean;
  exportDisabledReason?: string;
  onExport: (format: ExportFormat) => void;
  selectionMode: SelectionMode;
  selectionEnabled: boolean;
  onSelectionModeChange: (mode: SelectionMode) => void;
  sidebarControls: PreviewSidebarControls;
  pageControlsDisabled: boolean;
}) {
  return (
    <header className="preview-toolbar border-b border-border bg-panel" aria-label="演示操作与选择">
      <div className="flex shrink-0 items-center gap-2">
        {sidebarControls.leftHidden && (
          <IconButton
            label="展开左侧目录"
            onClick={sidebarControls.onExpandLeft}
            disabled={!sidebarControls.canExpandLeft}
            title={sidebarControls.canExpandLeft ? '展开左侧目录' : '加宽窗口后可展开左侧目录'}
          >
            <PanelLeftOpen className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
        )}
        <ThemeSelector key={projectId} projectId={projectId} />
      </div>
      <div className="ml-auto flex shrink-0 items-center gap-2">
        <div
          role="group"
          aria-label="选区工具"
          className="flex items-center gap-1"
        >
          <IconButton
            label="选择元素"
            aria-pressed={selectionEnabled && selectionMode === 'element'}
            onClick={() => onSelectionModeChange(selectionMode === 'element' ? 'none' : 'element')}
            disabled={!selectionEnabled || pageControlsDisabled || contentMode === 'source'}
            className={selectionEnabled && selectionMode === 'element' ? 'bg-accent-soft text-accent' : undefined}
          >
            <MousePointer2 className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
          <IconButton
            label="框选区域"
            aria-pressed={selectionEnabled && selectionMode === 'region'}
            onClick={() => onSelectionModeChange(selectionMode === 'region' ? 'none' : 'region')}
            disabled={!selectionEnabled || pageControlsDisabled || contentMode === 'source'}
            className={selectionEnabled && selectionMode === 'region' ? 'bg-accent-soft text-accent' : undefined}
          >
            <Scan className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
        </div>
        <span aria-hidden="true" className="h-4 w-px shrink-0 bg-border" />
        <div role="group" aria-label="总览与交付" className="flex items-center gap-1">
          <IconButton
            label="总览"
            title={overview ? '返回单页视图' : '查看全部页面'}
            aria-pressed={overview}
            onClick={onToggleOverview}
            disabled={!hasPages}
            className={hasPages && overview ? 'bg-accent-soft text-accent' : undefined}
          >
            <LayoutGrid className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
          <IconButton label="全屏放映" onClick={onPresent} disabled={!canPresent}>
            <MonitorPlay className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
          <ExportButton disabled={exportDisabled} reason={exportDisabledReason} onExport={onExport} />
        </div>
        {sidebarControls.rightHidden && (
          <IconButton
            label="展开右侧对话"
            onClick={sidebarControls.onExpandRight}
            disabled={!sidebarControls.canExpandRight}
            title={sidebarControls.canExpandRight ? '展开右侧对话' : '加宽窗口后可展开右侧对话'}
          >
            <PanelRightOpen className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
        )}
      </div>
    </header>
  );
}

export function PreviewStatusBar({
  view, onViewChange, contentMode, onContentModeChange, pageControlsDisabled,
  pageIndex, pageCount, onPrevious, onNext,
  zoom, zoomMin, zoomMax, zoomEnabled, onZoomOut, onZoomIn,
  documentOpen, sourceToggleVisible = false,
}: {
  documentOpen: boolean;
  sourceToggleVisible?: boolean;
  view: PageView;
  onViewChange: (view: PageView) => void;
  contentMode: ContentMode;
  onContentModeChange: (mode: ContentMode) => void;
  pageControlsDisabled: boolean;
  pageIndex: number;
  pageCount: number;
  onPrevious: () => void;
  onNext: () => void;
  zoom: number;
  zoomMin: number;
  zoomMax: number;
  zoomEnabled: boolean;
  onZoomOut: () => void;
  onZoomIn: () => void;
}) {
  const hasPages = pageCount > 0;
  const zoomPercent = Math.round(zoom * 100);

  return (
    <footer className="preview-status-bar border-t border-border bg-panel" aria-label="画布展示控制">
      <div className="preview-status-bar-content">
        <div role="group" aria-label="画布展示模式" className="preview-status-modes">
          <StatusModeTabs
            label="画布视图"
            value={view}
            disabled={pageControlsDisabled}
            options={[
              { value: 'outline', label: '规格要求' },
              { value: 'html', label: '幻灯片' },
            ]}
            onValueChange={value => onViewChange(value === 'html' ? 'html' : 'outline')}
          />
          {sourceToggleVisible && (
            <button
              type="button"
              role="switch"
              aria-label="显示源码"
              aria-checked={contentMode === 'source'}
              className="preview-source-switch"
              disabled={pageControlsDisabled}
              onClick={() => onContentModeChange(contentMode === 'source' ? 'preview' : 'source')}
            >
              <span aria-hidden="true">源码</span>
              <span className="preview-source-switch-track" aria-hidden="true"><span className="preview-source-switch-thumb" /></span>
            </button>
          )}
        </div>

        <div role="group" aria-label="翻页" className="flex items-center gap-0.5 justify-self-center">
          <IconButton label="上一页" onClick={onPrevious} disabled={documentOpen || !hasPages || pageIndex === 0}>
            <ChevronLeft className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
          <span
            className="flex h-8 min-w-10 shrink-0 items-center justify-center gap-1 px-1 text-xs tabular-nums"
            aria-label={hasPages ? `第 ${pageIndex + 1} 页，共 ${pageCount} 页` : '暂无页面'}
          >
            <span aria-hidden="true" className={cn('font-semibold', !documentOpen && hasPages ? 'text-text-900' : 'text-text-400')}>{hasPages ? pageIndex + 1 : 0}</span>
            <span aria-hidden="true" className={!documentOpen && hasPages ? 'text-text-600' : 'text-text-400'}>/ {pageCount}</span>
          </span>
          <IconButton label="下一页" onClick={onNext} disabled={documentOpen || !hasPages || pageIndex >= pageCount - 1}>
            <ChevronRight className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
        </div>

        <div
          role="group"
          aria-label="画布缩放"
          className="flex items-center gap-1 justify-self-end"
        >
          <IconButton label="缩小画布" onClick={onZoomOut} disabled={documentOpen || !zoomEnabled || zoom <= zoomMin}>
            <ZoomOut className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
          <span
            className={cn('min-w-11 text-center text-xs tabular-nums', zoomEnabled ? 'text-text-700' : 'text-text-400')}
            aria-label={zoomEnabled ? `画布缩放 ${zoomPercent}%` : '当前视图不支持画布缩放'}
            title={zoomEnabled ? '相对于适应画布的缩放比例' : '当前视图不支持画布缩放'}
          >
            {zoomPercent}%
          </span>
          <IconButton label="放大画布" onClick={onZoomIn} disabled={documentOpen || !zoomEnabled || zoom >= zoomMax}>
            <ZoomIn className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
        </div>
      </div>
    </footer>
  );
}
