import { useEffect, useState } from 'react';
import { ChartColumn, ChevronDown, Code2, Gauge, Image, List, Quote, Table2, Type, Workflow, type LucideIcon } from 'lucide-react';
import type { Design, Manifest, SlideContentType, SlidePurpose, SlideSpec } from '../../api/types';
import { Select } from '../../components/ui/select';
import { DropdownMenu, DropdownMenuContent, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuTrigger } from '../../components/ui/dropdown-menu';
import { decorationPlacementLabel, decorationTypeLabel, elementTypeLabel, slideContentTypeOptions, slidePurposeOptions } from './semanticLabels';
import type { ManagementController } from './ManagementEditor';
import { ListProperty, ManagementSection, TextListProperty, TextProperty } from './ManagementFields';
import { decorationPlacements, decorationTypes, decorationPositionOccupants } from './decorationPositions';

type Element = SlideSpec['elements'][number];
const elementIcons: Record<Element['type'], LucideIcon> = {
  text: Type, list: List, metric: Gauge, quote: Quote, table: Table2, chart: ChartColumn, diagram: Workflow, code: Code2, asset: Image,
};
const elementTypes = Object.keys(elementIcons) as Element['type'][];

function languageLabel(language: string) {
  const labels: Record<string, string> = {
    zh: '中文', 'zh-cn': '简体中文', 'zh-hans': '简体中文', 'zh-tw': '繁体中文', 'zh-hk': '繁体中文', 'zh-hant': '繁体中文',
    en: '英语', 'en-us': '英语（美国）', 'en-gb': '英语（英国）', ja: '日语', ko: '韩语', fr: '法语', de: '德语', es: '西班牙语',
  };
  return labels[language.trim().toLowerCase()] ?? language.trim();
}

export function ManifestFields({ editor }: { editor: ManagementController<Manifest> }) {
  const value = editor.value;
  return <>
    <ManagementSection title="基本信息">
      <TextProperty editor={editor} id="title" label="演示标题" value={value.title} minLength={1} maxLength={200} update={(current, title) => ({ ...current, title })} />
      <TextProperty editor={editor} id="language" label="演示语言" value={value.language} displayValue={languageLabel(value.language)} minLength={2} maxLength={32} update={(current, language) => ({ ...current, language })} />
      <TextProperty editor={editor} id="pages" label="演示页数" value={value.pages} minLength={1} maxLength={32} update={(current, pages) => ({ ...current, pages })} />
      <TextProperty editor={editor} id="audience" label="演示受众" value={value.audience} minLength={1} maxLength={600} multiline update={(current, audience) => ({ ...current, audience })} />
      <TextProperty editor={editor} id="goal" label="演示目标" value={value.goal} minLength={1} maxLength={1200} multiline update={(current, goal) => ({ ...current, goal })} />
    </ManagementSection>
    <TextListProperty editor={editor} id="requirements" label="内容需求" items={value.requirements} maxLength={300} update={(current, requirements) => ({ ...current, requirements })} />
    <TextListProperty editor={editor} id="prohibitions" label="内容限制" items={value.prohibitions} maxLength={300} update={(current, prohibitions) => ({ ...current, prohibitions })} />
  </>;
}

export function DesignFields({ editor }: { editor: ManagementController<Design> }) {
  const value = editor.value;
  return <>
    <TextListProperty editor={editor} id="demands" label="视觉需求" items={value.demands} maxLength={600}
      update={(current, demands) => ({ ...current, demands })} />
    <ManagementSection title="页面装饰" className="management-decorations">
      {decorationTypes.map(type => <div key={type} className="management-field"><div className="management-label">{decorationTypeLabel(type)}</div>
        <Select aria-label={`${decorationTypeLabel(type)}位置`} value={value.decorations[type]} disabled={editor.disabled} className="management-select"
          options={decorationPlacements.filter(p => type !== 'page_number' || p !== 'none').map(p => {
            const occupants = decorationPositionOccupants(value.decorations, type, p);
            return { value: p, label: occupants.length
              ? `${decorationPlacementLabel(p)} · 已被${occupants.map(decorationTypeLabel).join('、')}占用`
              : decorationPlacementLabel(p), disabled: occupants.length > 0 };
          })}
          onValueChange={placement => { void editor.commit(current => ({ ...current, decorations: { ...current.decorations, [type]: placement } })); }} />
      </div>)}
    </ManagementSection>
  </>;
}

function ElementTypeMenu({ type, index, disabled, onChange }: { type: Element['type']; index: number; disabled: boolean; onChange: (type: Element['type']) => void }) {
  const [open, setOpen] = useState(false);
  useEffect(() => { if (disabled) setOpen(false); }, [disabled]);
  const Icon = elementIcons[type];
  return <DropdownMenu open={open && !disabled} onOpenChange={setOpen}>
    <DropdownMenuTrigger asChild><button type="button" disabled={disabled} className="management-element-type ui-interactive"
      aria-label={`元素 ${index + 1} 类型：${elementTypeLabel(type)}`} title={`${elementTypeLabel(type)} · 切换类型`}>
      <Icon aria-hidden="true" /><ChevronDown aria-hidden="true" />
    </button></DropdownMenuTrigger>
    <DropdownMenuContent className="management-type-menu scrollbar-none" align="start" onEscapeKeyDown={event => event.stopPropagation()}>
      <DropdownMenuRadioGroup value={type} onValueChange={next => onChange(next as Element['type'])}>
        {elementTypes.map(value => { const OptionIcon = elementIcons[value]; return <DropdownMenuRadioItem key={value} value={value} textValue={elementTypeLabel(value)}>
          <OptionIcon className="h-4 w-4" aria-hidden="true" /><span>{elementTypeLabel(value)}</span>
        </DropdownMenuRadioItem>; })}
      </DropdownMenuRadioGroup>
    </DropdownMenuContent>
  </DropdownMenu>;
}

export function SpecFields({ editor, title, creating = false }: { editor: ManagementController<SlideSpec>; title: string; creating?: boolean }) {
  const value = editor.value;
  return <>
    <ManagementSection title="页面信息">
      <div className="management-field"><div className="management-label">页面标题</div><div className="management-value"><span>{title}</span></div></div>
      {creating && <p className="management-empty mb-3">设计稿尚未生成。填写核心信息后即可创建，也可以交给 Agent 生成。</p>}
      <div className="management-field"><div className="management-label">页面用途</div><div className="management-select-wrap">
        <Select aria-label="页面用途" value={value.purpose ?? ''} disabled={editor.disabled || creating} className="management-select"
          options={[{ value: '', label: '未设置' }, ...slidePurposeOptions]} onValueChange={purpose => { void editor.commit(current => {
            const next = { ...current };
            if (purpose) next.purpose = purpose as SlidePurpose; else delete next.purpose;
            if (purpose !== 'content') delete next.content_type;
            return next;
          }); }} />
      </div></div>
      {value.purpose === 'content' && <div className="management-field"><div className="management-label">正文类型</div><div className="management-select-wrap">
        <Select aria-label="正文类型" value={value.content_type ?? ''} disabled={editor.disabled || creating} className="management-select"
          options={[{ value: '', label: '未设置' }, ...slideContentTypeOptions]} onValueChange={contentType => { void editor.commit(current => {
            const next = { ...current }; if (contentType) next.content_type = contentType as SlideContentType; else delete next.content_type; return next;
          }); }} />
      </div></div>}
      <TextProperty editor={editor} id="core" label="核心信息" value={value.core} minLength={1} maxLength={500} multiline update={(current, core) => ({ ...current, core })} />
      <TextProperty editor={creating ? { ...editor, disabled: true } : editor} id="layout" label="布局建议" value={value.layout ?? ''} maxLength={80} update={(current, layout) => {
        const next = { ...current }; if (layout) next.layout = layout; else delete next.layout; return next;
      }} />
    </ManagementSection>
    <ListProperty<SlideSpec, Element> editor={creating ? { ...editor, disabled: true } : editor} id="elements" label="内容元素" items={value.elements} maxLength={1200}
      getText={element => element.intent} withText={(element, intent) => ({ ...element, intent })} createItem={intent => ({ type: 'text', intent })}
      update={(current, elements) => ({ ...current, elements })}
      leading={(element, index) => <ElementTypeMenu type={element.type} index={index} disabled={editor.disabled || creating}
        onChange={type => { void editor.commit(current => ({ ...current, elements: current.elements.map((item, i) => i === index ? { ...item, type } : item) })); }} />} />
  </>;
}
