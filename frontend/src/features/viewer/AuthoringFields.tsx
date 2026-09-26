import { useId } from 'react';
import type { DecorationPlacement, DecorationType, Design, Manifest, SlideRole, SlideSpec } from '../../api/types';
import { Select } from '../../components/ui/select';
import { Button } from '../../components/ui/primitives';
import { decorationPlacementLabel, decorationTypeLabel, elementTypeLabel, slideRoleOptions } from './semanticLabels';
import { ItemActions, TextField, TextListField } from './ManagementEditor';
import { moveItem } from '../../lib/utils';

export function ManifestFields({ value, onChange }: { value: Manifest; onChange: (value: Manifest) => void }) {
  return <>
    <TextField label="演示标题" value={value.title} minLength={1} maxLength={200} onChange={title => onChange({ ...value, title })} />
    <TextField label="演示语言" value={value.language} minLength={2} maxLength={32} onChange={language => onChange({ ...value, language })} />
    <TextField label="演示目标" value={value.goal} minLength={1} maxLength={1200} multiline onChange={goal => onChange({ ...value, goal })} />
    <TextField label="目标受众" value={value.audience} minLength={1} maxLength={600} multiline onChange={audience => onChange({ ...value, audience })} />
    <TextListField label="内容要求" values={value.requirements} maxLength={300} onChange={requirements => onChange({ ...value, requirements })} />
    <TextListField label="限制与禁忌" values={value.prohibitions} maxLength={300} onChange={prohibitions => onChange({ ...value, prohibitions })} />
  </>;
}

const placements: (DecorationPlacement | 'none')[] = ['none', 'top-left', 'top-center', 'top-right', 'bottom-left', 'bottom-center', 'bottom-right', 'left-edge', 'right-edge'];
const decorations: DecorationType[] = ['page_number', 'section_title', 'deck_title', 'key_message'];

export function DesignFields({ value, onChange }: { value: Design; onChange: (value: Design) => void }) {
  return <>
    <TextField label="视觉方向" value={value.direction} maxLength={600} multiline onChange={direction => onChange({ ...value, direction })} />
    <TextListField label="排版偏好" values={value.layout_preferences} maxLength={600} onChange={layout_preferences => onChange({ ...value, layout_preferences })} />
    <section className="space-y-3" aria-label="页面装饰">
      <h3 className="text-sm font-semibold text-text-900">页面装饰</h3>
      {decorations.map(type => <div key={type} className="flex flex-wrap items-center justify-between gap-3 border-b border-border py-3">
        <span className="text-sm text-text-700">{decorationTypeLabel(type)}</span>
        <Select className="max-w-56" aria-label={`${decorationTypeLabel(type)}位置`} value={value.decorations[type]}
          options={placements.filter(p => type !== 'page_number' || p !== 'none').map(p => ({ value: p, label: decorationPlacementLabel(p) }))}
          onValueChange={placement => onChange({ ...value, decorations: { ...value.decorations, [type]: placement } })} />
      </div>)}
    </section>
  </>;
}

const elementTypes: SlideSpec['elements'][number]['type'][] = ['text', 'list', 'metric', 'quote', 'table', 'chart', 'diagram', 'code', 'asset'];

export function SpecFields({ value, onChange }: { value: SlideSpec; onChange: (value: SlideSpec) => void }) {
  const roleId = useId();
  return <>
    <div className="space-y-2">
      <label htmlFor={roleId} className="block text-sm font-medium text-text-700">页面角色（选填）</label>
      <Select id={roleId} value={value.role ?? ''} options={[{ value: '', label: '未设置' }, ...slideRoleOptions]}
        onValueChange={role => {
          const next = { ...value }; if (role) next.role = role as SlideRole; else delete next.role; onChange(next);
        }} />
    </div>
    <TextField label="核心信息" value={value.key_message} minLength={1} maxLength={500} multiline onChange={key_message => onChange({ ...value, key_message })} />
    <TextField label="布局建议（选填）" value={value.layout ?? ''} maxLength={80} onChange={layout => {
      const next = { ...value }; if (layout.trim()) next.layout = layout; else delete next.layout; onChange(next);
    }} />
    <section className="space-y-4" aria-label="内容元素">
      <h3 className="text-sm font-semibold text-text-900">内容元素</h3>
      {value.elements.length === 0 && <p className="text-sm text-text-600">暂无内容元素</p>}
      {value.elements.map((element, index) => <div key={index} className="space-y-3 border-b border-border pb-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <Select className="max-w-48" aria-label={`元素 ${index + 1} 类型`} value={element.type} options={elementTypes.map(type => ({ value: type, label: elementTypeLabel(type) }))}
            onValueChange={type => onChange({ ...value, elements: value.elements.map((item, i) => i === index ? { ...item, type: type as typeof element.type } : item) })} />
          <ItemActions label={`元素 ${index + 1}`} index={index} count={value.elements.length} onMove={to => onChange({ ...value, elements: moveItem(value.elements, index, to) })} onRemove={() => onChange({ ...value, elements: value.elements.filter((_, i) => i !== index) })} />
        </div>
        <TextField label={`元素 ${index + 1} 内容意图`} value={element.intent} minLength={1} maxLength={1200} multiline onChange={intent => onChange({ ...value, elements: value.elements.map((item, i) => i === index ? { ...item, intent } : item) })} />
      </div>)}
      <Button type="button" variant="secondary" disabled={value.elements.length >= 32} onClick={() => onChange({ ...value, elements: [...value.elements, { type: 'text', intent: '' }] })}>添加内容元素</Button>
    </section>
  </>;
}
