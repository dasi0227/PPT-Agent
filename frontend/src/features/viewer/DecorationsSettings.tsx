import { useEffect, useRef, useState } from 'react';
import { ImageOff, Loader2, PanelsTopLeft, Trash2, Upload } from 'lucide-react';
import { RequestCanceledError } from '../../api/client';
import { projectsApi } from '../../api/projects';
import type { Project } from '../../api/types';
import {
  AnchoredPopover, AnchoredPopoverContent, AnchoredPopoverTitle, AnchoredPopoverTrigger,
} from '../../components/ui/anchored-popover';
import { floatingPanelTitleClassName } from '../../components/ui/dropdown-styles';
import { Button, IconButton } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';
import { useProjectStore } from '../../stores/projectStore';
import { showGlobalError } from '../../stores/toastStore';
import { DecorationPlacementEditor } from './DecorationPlacementEditor';
import { useAuthoringBlock } from './useAuthoringBlock';

type SaveOperation = 'positions' | 'upload' | 'delete';

export function DecorationsSettings({ projectId }: { projectId: string | null }) {
  const project = useProjectStore(state => state.projects.find(value => value.id === projectId));
  const loadAppearance = useProjectStore(state => state.loadProjectAppearance);
  const setDecorations = useProjectStore(state => state.setProjectDecorations);
  const uploadLogo = useProjectStore(state => state.uploadProjectLogo);
  const deleteLogo = useProjectStore(state => state.deleteProjectLogo);
  const blocked = useAuthoringBlock(projectId);
  const anchorRef = useRef<HTMLButtonElement>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const savingRef = useRef(false);
  const mountedRef = useRef(false);
  const activeProjectIdRef = useRef(projectId);
  activeProjectIdRef.current = projectId;
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [saving, setSaving] = useState<SaveOperation | null>(null);
  const [fileDragging, setFileDragging] = useState(false);
  const [logoError, setLogoError] = useState(false);
  const logo = project?.logo;
  const disabled = !project || !projectId || loading || loadError || Boolean(blocked) || Boolean(saving);

  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);
  useEffect(() => { setLogoError(false); }, [logo?.hash]);
  useEffect(() => {
    if (!open || !projectId) return;
    const controller = new AbortController();
    setLoading(true);
    setLoadError(false);
    void loadAppearance(projectId, controller.signal).catch(error => {
      if (!controller.signal.aborted && !(error instanceof RequestCanceledError)) setLoadError(true);
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => { controller.abort(); };
  }, [open, projectId, loadAppearance, loadAttempt]);

  const save = async (operation: SaveOperation, write: () => Promise<Project>): Promise<boolean> => {
    if (disabled || savingRef.current) return false;
    savingRef.current = true;
    setSaving(operation);
    try {
      await write();
      return true;
    } catch (error) {
      if (!(error instanceof RequestCanceledError)) {
        showGlobalError(error instanceof Error ? error.message : '页面装饰保存失败');
        if (projectId) {
          try {
            await loadAppearance(projectId);
          } catch {
            if (mountedRef.current && activeProjectIdRef.current === projectId) setLoadError(true);
          }
        }
      }
      return false;
    } finally {
      savingRef.current = false;
      if (mountedRef.current) setSaving(null);
    }
  };
  const uploadFiles = (files: File[]) => {
    if (!projectId || disabled || files.length === 0) return;
    if (files.length !== 1) {
      showGlobalError('请选择一张 PNG 或 JPEG 图片');
      return;
    }
    const file = files[0];
    const supported = ['image/png', 'image/jpeg'].includes(file.type)
      || (!file.type && /\.(png|jpe?g)$/i.test(file.name));
    if (!supported) {
      showGlobalError('仅支持 PNG 和 JPEG 图片');
      return;
    }
    if (file.size > 10 * 1024 * 1024) {
      showGlobalError('图片超过 10 MiB，请压缩后重试');
      return;
    }
    void save('upload', () => uploadLogo(projectId, file));
  };

  return (
    <AnchoredPopover open={open} onOpenChange={value => {
      setOpen(value);
      setFileDragging(false);
      if (value) { setLoading(true); setLoadError(false); }
    }}>
      <AnchoredPopoverTrigger asChild>
        <button ref={anchorRef} type="button" disabled={!projectId || !project}
          aria-label="页面装饰" aria-busy={Boolean(saving)} title="页面装饰"
          className="flex h-8 shrink-0 items-center gap-1 whitespace-nowrap rounded-md bg-transparent px-2 text-sm font-medium text-text-600 transition-colors ui-interactive disabled:cursor-not-allowed disabled:opacity-50">
          {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" /> : <PanelsTopLeft className="h-3.5 w-3.5" strokeWidth={1.75} />}
          <span>页面装饰</span>
        </button>
      </AnchoredPopoverTrigger>
      {open && project && (
        <AnchoredPopoverContent anchorRef={anchorRef} side="bottom" sideOffset={6} showArrow={false}
          className="w-[400px] rounded-[var(--radius-panel)]">
          {loadError ? (
            <>
              <header className="decorations-editor-header">
                <AnchoredPopoverTitle className={floatingPanelTitleClassName}>页面装饰</AnchoredPopoverTitle>
              </header>
              <button type="button" onClick={() => setLoadAttempt(value => value + 1)} className="mx-4 mb-3 rounded-md px-2 py-1.5 text-xs text-text-600 ui-interactive">
                加载失败，点击重试
              </button>
            </>
          ) : (
            <DecorationPlacementEditor key={project.id} value={project.decorations} disabled={disabled} loading={loading} blocked={blocked}
              onChange={positions => save('positions', () => setDecorations(project.id, positions))} />
          )}
          <div className={cn('decorations-upload border-t border-border transition-colors', fileDragging && !disabled && 'bg-hover')}
            onDragOver={event => {
              if (!event.dataTransfer.types.includes('Files')) return;
              event.preventDefault();
              event.dataTransfer.dropEffect = disabled ? 'none' : 'copy';
              if (!disabled) setFileDragging(true);
            }}
            onDragLeave={event => {
              if (!(event.relatedTarget instanceof Node) || !event.currentTarget.contains(event.relatedTarget)) setFileDragging(false);
            }}
            onDrop={event => {
              if (!event.dataTransfer.types.includes('Files')) return;
              event.preventDefault();
              setFileDragging(false);
              uploadFiles(Array.from(event.dataTransfer.files));
            }}>
            <input ref={fileRef} type="file" accept="image/png,image/jpeg" disabled={disabled} className="hidden"
              onChange={event => { uploadFiles(Array.from(event.currentTarget.files ?? [])); event.currentTarget.value = ''; }} />
            {logo ? (
              <div className="flex min-h-[34px] items-center gap-2.5">
                <div className="flex h-[30px] w-11 shrink-0 items-center justify-center rounded-md border border-border bg-panel p-1">
                  {logoError ? <ImageOff className="h-4 w-4 text-text-400" aria-label="标识图片加载失败" /> : (
                    <img key={logo.hash} src={projectsApi.logoContentUrl(project.id, logo.hash)} alt="演示标识" draggable={false}
                      className="h-full w-full object-contain" onError={() => setLogoError(true)} />
                  )}
                </div>
                <span className="min-w-0 flex-1 truncate text-xs text-text-700" title={logo.original_name}>{logo.original_name}</span>
                <IconButton label="更换标识" disabled={disabled} title={blocked ?? '更换标识'} onClick={() => fileRef.current?.click()} className="h-7 w-7">
                  {saving === 'upload' ? <Loader2 className="h-[15px] w-[15px] animate-spin motion-reduce:animate-none" /> : <Upload className="h-[15px] w-[15px]" strokeWidth={1.75} />}
                </IconButton>
                <IconButton label="清除标识" disabled={disabled} title={blocked ?? '清除标识'}
                  onClick={() => { if (projectId) void save('delete', () => deleteLogo(projectId)); }} className="h-7 w-7 ui-danger">
                  {saving === 'delete' ? <Loader2 className="h-[15px] w-[15px] animate-spin motion-reduce:animate-none" /> : <Trash2 className="h-[15px] w-[15px]" strokeWidth={1.75} />}
                </IconButton>
              </div>
            ) : (
              <Button variant="ghost" type="button" disabled={disabled} title={blocked} onClick={() => fileRef.current?.click()}
                className="h-[34px] w-full border border-dashed border-border-strong text-xs font-normal">
                {saving === 'upload' ? <Loader2 className="h-4 w-4 animate-spin motion-reduce:animate-none" /> : <Upload className="h-4 w-4" strokeWidth={1.75} />}上传标识
              </Button>
            )}
          </div>
        </AnchoredPopoverContent>
      )}
    </AnchoredPopover>
  );
}
