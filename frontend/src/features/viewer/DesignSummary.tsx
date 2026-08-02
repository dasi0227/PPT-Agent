import type { Design } from '../../api/types';

export function DesignSummary({ design }: { design: Design }) {
  return (
    <section className="rounded-xl border border-border bg-surface p-5 shadow-sm">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold text-text-900">全局视觉规范</h3>
        <span className="text-xs text-text-400">rev {design.revision}</span>
      </div>
      <div className="mt-3 flex gap-2">
        {design.palette.map((color) => <span key={color} className="h-6 w-6 rounded-full border border-black/10" style={{ background: color }} title={color} />)}
      </div>
      <p className="mt-3 text-sm text-text-600">{design.signature}</p>
      <p className="mt-2 text-xs text-text-400">{String(design.layout_system.grid ?? '')} · {String(design.layout_system.density ?? '')}</p>
    </section>
  );
}
