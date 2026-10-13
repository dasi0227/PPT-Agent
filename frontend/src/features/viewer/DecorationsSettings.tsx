import { useEffect, useId, useRef, useState } from 'react';
import { ChevronDown, ImageOff, Loader2, PanelsTopLeft, Trash2, Upload, X } from 'lucide-react';
import { RequestCanceledError } from '../../api/client';
import { projectsApi } from '../../api/projects';
import type { Decorations, DecorationType, Project } from '../../api/types';
import {
  AnchoredPopover, AnchoredPopoverContent, AnchoredPopoverTitle, AnchoredPopoverTrigger,
} from '../../components/ui/anchored-popover';
import { Button, IconButton } from '../../components/ui/primitives';
import { Select, type SelectOption } from '../../components/ui/select';
import { cn } from '../../lib/utils';
import { useProjectStore } from '../../stores/projectStore';
import { showGlobalError } from '../../stores/toastStore';
import { decorationTypeLabel } from './semanticLabels';
import { useAuthoringBlock } from './useAuthoringBlock';

const fields: DecorationType[] = ['page_number', 'deck_title', 'section_title', 'logo'];
const placements: SelectOption[] = [
  { value: 'top-left', label: '左上' },
  { value: 'top-center', label: '上中' },
  { value: 'top-right', label: '右上' },
  { value: 'bottom-left', label: '左下' },
  { value: 'bottom-center', label: '下中' },
  { value: 'bottom-right', label: '右下' },
];
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
  const ownerId = useId();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [saving, setSaving] = useState<SaveOperation | null>(null);
  const [dragging, setDragging] = useState(false);
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

  const save = async (operation: SaveOperation, write: () => Promise<Project>) => {
    if (disabled || savingRef.current) return;
    savingRef.current = true;
    setSaving(operation);
    try {
      await write();
    } catch (error) {
      if (!(error instanceof RequestCanceledError)) {
        showGlobalError(error instanceof Error ? error.message : '页面装饰保存失败');
        if (projectId) {
          try {
            await loadAppearance(projectId);
          } catch {
            if (mountedRef.current) setLoadError(true);
          }
        }
      }
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
      setDragging(false);
      if (value) { setLoading(true); setLoadError(false); }
    }}>
      <AnchoredPopoverTrigger asChild>
        <button ref={anchorRef} type="button" disabled={!projectId || !project}
          aria-label="页面装饰" aria-busy={Boolean(saving)} title="页面装饰"
          className="flex h-8 shrink-0 items-center gap-1 whitespace-nowrap rounded-md bg-transparent px-2 text-xs font-medium text-text-600 transition-colors ui-interactive disabled:cursor-not-allowed disabled:opacity-50">
          {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" /> : <PanelsTopLeft className="h-3.5 w-3.5" strokeWidth={1.75} />}
          <span>页面装饰</span>
          <ChevronDown className={cn('h-3 w-3 transition-transform motion-reduce:transition-none', open && 'rotate-180')} strokeWidth={1.75} />
        </button>
      </AnchoredPopoverTrigger>
      {open && project && (
        <AnchoredPopoverContent anchorRef={anchorRef} side="bottom" sideOffset={6} showArrow={false} className="w-80 rounded-lg"
          onInteractOutside={event => {
            const target = event.target;
            if (target instanceof Element && target.closest('[data-select-owner]')?.getAttribute('data-select-owner') === ownerId) event.preventDefault();
          }}>
          <header className="flex h-12 items-center gap-2 border-b border-border px-4">
            <AnchoredPopoverTitle className="text-[13px] font-semibold">页面装饰</AnchoredPopoverTitle>
            {loading && <Loader2 className="h-3.5 w-3.5 animate-spin text-text-400 motion-reduce:animate-none" aria-label="正在加载页面装饰" />}
            <IconButton label="关闭页面装饰" onClick={() => setOpen(false)} className="ml-auto h-6 w-6">
              <X className="h-3.5 w-3.5" strokeWidth={1.75} />
            </IconButton>
          </header>
          {loadError ? (
            <button type="button" onClick={() => setLoadAttempt(value => value + 1)} className="mx-4 my-3 rounded-md px-2 py-1.5 text-xs text-text-600 ui-interactive">
              加载失败，点击重试
            </button>
          ) : (
            <div className="px-4 py-1.5" aria-busy={Boolean(saving) || loading}>
              {fields.map(key => {
                const label = decorationTypeLabel(key);
                const options = (key === 'page_number' ? placements : [{ value: 'none', label: '不显示' }, ...placements]).map(option => ({
                  ...option,
                  disabled: option.value !== 'none' && fields.some(field => field !== key && project.decorations[field] === option.value),
                }));
                return (
                  <div key={key} className="flex min-h-12 items-center justify-between gap-3 border-b border-border last:border-b-0">
                    <label id={`${ownerId}-${key}`} htmlFor={`${ownerId}-${key}-input`} className="text-xs text-text-700">{label}</label>
                    <Select value={project.decorations[key]} options={options} ownerId={ownerId}
                      id={`${ownerId}-${key}-input`}
                      aria-labelledby={`${ownerId}-${key}`} disabled={disabled} title={blocked}
                      className="h-[30px] w-[151px] rounded-md bg-surface px-2 text-xs"
                      onValueChange={value => {
                        if (!projectId || value === project.decorations[key]) return;
                        const updates = { [key]: value } as Partial<Decorations>;
                        void save('positions', () => setDecorations(projectId, updates));
                      }} />
                  </div>
                );
              })}
            </div>
          )}
          <div className={cn('border-t border-border p-4 transition-colors', dragging && !disabled && 'bg-hover')}
            onDragOver={event => {
              if (!event.dataTransfer.types.includes('Files')) return;
              event.preventDefault();
              event.dataTransfer.dropEffect = disabled ? 'none' : 'copy';
              if (!disabled) setDragging(true);
            }}
            onDragLeave={event => {
              if (!(event.relatedTarget instanceof Node) || !event.currentTarget.contains(event.relatedTarget)) setDragging(false);
            }}
            onDrop={event => {
              if (!event.dataTransfer.types.includes('Files')) return;
              event.preventDefault();
              setDragging(false);
              uploadFiles(Array.from(event.dataTransfer.files));
            }}>
            <input ref={fileRef} type="file" accept="image/png,image/jpeg" disabled={disabled} className="hidden"
              onChange={event => { uploadFiles(Array.from(event.currentTarget.files ?? [])); event.currentTarget.value = ''; }} />
            {logo ? (
              <div className="flex items-center gap-3">
                <div className="flex h-16 w-[108px] shrink-0 items-center justify-center rounded-md border border-border bg-panel p-2">
                  {logoError ? <ImageOff className="h-5 w-5 text-text-400" aria-label="标志图片加载失败" /> : (
                    <img key={logo.hash} src={projectsApi.logoContentUrl(project.id, logo.hash)} alt="演示标志"
                      className="h-full w-full object-contain" onError={() => setLogoError(true)} />
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <span className="mb-1.5 block truncate text-xs text-text-700" title={logo.original_name}>{logo.original_name}</span>
                  <div className="flex items-center gap-1">
                    <Button variant="ghost" type="button" disabled={disabled} title={blocked} onClick={() => fileRef.current?.click()} className="h-7 gap-1 px-1.5 text-xs">
                      {saving === 'upload' ? <Loader2 className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" /> : <Upload className="h-3.5 w-3.5" strokeWidth={1.75} />}更换
                    </Button>
                    <Button variant="ghost" type="button" disabled={disabled} title={blocked}
                      onClick={() => { if (projectId) void save('delete', () => deleteLogo(projectId)); }} className="h-7 gap-1 px-1.5 text-xs text-danger ui-danger">
                      {saving === 'delete' ? <Loader2 className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" /> : <Trash2 className="h-3.5 w-3.5" strokeWidth={1.75} />}清除
                    </Button>
                  </div>
                </div>
              </div>
            ) : (
              <button type="button" disabled={disabled} title={blocked} onClick={() => fileRef.current?.click()}
                className="flex h-[90px] w-full flex-col items-center justify-center gap-1.5 rounded-md border border-dashed border-border-strong bg-panel text-xs text-text-600 ui-interactive disabled:cursor-not-allowed disabled:opacity-50">
                {saving === 'upload' ? <Loader2 className="h-5 w-5 animate-spin motion-reduce:animate-none" /> : <Upload className="h-5 w-5" strokeWidth={1.75} />}上传标志
              </button>
            )}
          </div>
        </AnchoredPopoverContent>
      )}
    </AnchoredPopover>
  );
}
