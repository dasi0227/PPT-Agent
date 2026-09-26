import { useId, type ReactNode } from 'react';
import { Pencil, Plus, Trash2 } from 'lucide-react';
import { IconButton } from '../../components/ui/primitives';
import { InlineTextEditor, type ManagementController } from './ManagementEditor';

export function ManagementSection({ title, action, children, className = '' }: {
  title: string; action?: ReactNode; children: ReactNode; className?: string;
}) {
  return <section className={`management-section ${className}`} aria-label={title}>
    <div className="management-section-heading"><h2>{title}</h2>{action}</div>
    {children}
  </section>;
}

function ReadableValue({ id, value, label, disabled, onEdit }: { id: string; value: string; label: string; disabled: boolean; onEdit: () => void }) {
  return <div className="management-value ui-interactive" aria-disabled={disabled}><span className={!value ? 'text-text-600' : undefined}>{value || '待填写'}</span>
    <IconButton id={id} data-edit-id={id} label={`编辑${label}`} disabled={disabled} onClick={onEdit} className="management-edit-action">
      <Pencil className="h-4 w-4" aria-hidden="true" />
    </IconButton>
  </div>;
}

export function TextProperty<T>({ editor, id, label, value, displayValue, update, minLength = 0, maxLength, multiline = false }: {
  editor: ManagementController<T>; id: string; label: string; value: string; displayValue?: string;
  update: (current: T, text: string) => T; minLength?: number; maxLength: number; multiline?: boolean;
}) {
  const buttonId = useId();
  return <div className="management-field"><div className="management-label">{label}</div><div className="min-w-0">
    {editor.draft?.id === id ? <InlineTextEditor editor={editor} hideLabel /> : <ReadableValue id={buttonId} value={displayValue ?? value} label={label} disabled={editor.disabled}
      onEdit={() => editor.start({ id, label, value, update, minLength, maxLength, multiline })} />}
  </div></div>;
}

export function ListProperty<T, Item>({ editor, id, label, items, maxLength, getText, withText, createItem, update, leading }: {
  editor: ManagementController<T>; id: string; label: string; items: Item[]; maxLength: number;
  getText: (item: Item) => string; withText: (item: Item, text: string) => Item; createItem: (text: string) => Item;
  update: (current: T, items: Item[]) => T;
  leading?: (item: Item, index: number) => ReactNode;
}) {
  const listId = useId();
  const adding = editor.draft?.id === `${id}:new`;
  return <ManagementSection title={label} action={
    <button type="button" className="management-add ui-interactive" id={`${listId}-add`} aria-label={`新增${label}`} disabled={editor.disabled || items.length >= 32}
      onClick={() => editor.start({ id: `${id}:new`, label: `新增${label}`, value: '', minLength: 1, maxLength, multiline: true,
        update: (current, text) => update(current, [...items, createItem(text)]) })}><Plus aria-hidden="true" />新增</button>
  }>
    {items.length || adding ? <ul className="management-list" role="list">
      {items.map((item, index) => {
        const itemId = `${id}:${index}`;
        const itemLabel = `${label} ${index + 1}`;
        const editing = editor.draft?.id === itemId;
        return <li key={itemId} aria-disabled={editor.disabled} className={`management-item ${editing ? '' : 'ui-interactive'} ${leading ? 'management-element' : ''} ${editing ? 'management-item-editing' : ''}`}>
          {editing ? <InlineTextEditor editor={editor} /> : <>
            {leading ? leading(item, index) : <span className="management-bullet" aria-hidden="true" />}
            <div className="min-w-0"><ReadableValue id={`${listId}-edit-${index}`} value={getText(item)} label={itemLabel} disabled={editor.disabled}
              onEdit={() => editor.start({ id: itemId, label: itemLabel, value: getText(item), minLength: 1, maxLength, multiline: true,
                update: (current, text) => update(current, items.map((value, i) => i === index ? withText(value, text) : value)) })} /></div>
            <IconButton data-focus-fallback={`${listId}-add`} label={`删除${itemLabel}`} className="management-delete-action ui-danger" disabled={editor.disabled}
              onClick={() => { void editor.commit(current => update(current, items.filter((_, i) => i !== index)), true); }}><Trash2 className="h-4 w-4" aria-hidden="true" /></IconButton>
          </>}
        </li>;
      })}
      {adding && <li className="management-item management-item-editing"><InlineTextEditor editor={editor} /></li>}
    </ul> : <p className="management-empty">暂无条目</p>}
  </ManagementSection>;
}

export function TextListProperty<T>(props: {
  editor: ManagementController<T>; id: string; label: string; items: string[]; maxLength: number;
  update: (current: T, items: string[]) => T;
}) {
  return <ListProperty {...props} getText={text => text} withText={(_, text) => text} createItem={text => text} />;
}
