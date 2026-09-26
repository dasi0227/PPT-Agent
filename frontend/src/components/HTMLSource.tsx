import { useEffect, useRef } from 'react';
import { EditorState } from '@codemirror/state';
import { EditorView, lineNumbers, drawSelection } from '@codemirror/view';
import { defaultHighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { html } from '@codemirror/lang-html';

const theme = EditorView.theme({
  '&': { height: '100%', fontSize: '13px', backgroundColor: 'var(--color-surface, #fff)' },
  '.cm-scroller': { overflow: 'auto', fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', lineHeight: '1.6' },
  '.cm-content': { padding: '14px 0' },
  '.cm-gutters': { backgroundColor: 'var(--color-panel, #fafafa)', color: 'var(--color-text-600, #475569)', borderRight: '1px solid var(--color-border, #e2e8f0)' },
  '&.cm-focused': { outline: 'none' },
  '& .cm-selectionBackground, &.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground': {
    backgroundColor: 'rgb(var(--ui-selected))',
  },
  '& .cm-content ::selection': { backgroundColor: 'rgb(var(--ui-selected))' },
});

export function HTMLSource({ text }: { text: string }) {
  const host = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!host.current) return;
    const view = new EditorView({ parent: host.current, state: EditorState.create({
      doc: text,
      extensions: [lineNumbers(), drawSelection(), html(), syntaxHighlighting(defaultHighlightStyle), theme,
        EditorState.readOnly.of(true), EditorView.editable.of(false),
        EditorView.contentAttributes.of({ tabindex: '0', 'aria-label': 'HTML 只读源码', 'aria-readonly': 'true' })],
    }) });
    return () => view.destroy();
  }, [text]);
  return <div ref={host} className="min-h-0 flex-1 overflow-hidden" />;
}
