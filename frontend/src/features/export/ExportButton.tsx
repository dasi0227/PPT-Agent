import { Download } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '../../components/ui/dropdown-menu';
import type { ExportFormat } from '../../api/exports';

export function ExportButton({ disabled, reason, onExport }: { disabled: boolean; reason?: string; onExport: (format: ExportFormat) => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button type="button" disabled={disabled} title={disabled ? reason : '导出完整演示文稿'} className="inline-flex h-8 items-center justify-center gap-1.5 rounded-md px-2 text-sm font-medium text-text-600 hover:bg-panel-muted hover:text-text-900 disabled:cursor-not-allowed disabled:opacity-45">
          <Download className="h-4 w-4" strokeWidth={1.75} />导出
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        <DropdownMenuItem onSelect={() => onExport('png')}>导出 PNG</DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onExport('pdf')}>导出 PDF</DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onExport('html')}>导出 HTML</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
