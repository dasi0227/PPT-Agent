import { useEffect, useRef } from 'react';
import { EditorState } from '@codemirror/state';
import { EditorView, lineNumbers, drawSelection } from '@codemirror/view';
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { tags } from '@lezer/highlight';
import { html } from '@codemirror/lang-html';

const theme = EditorView.theme({
  '&': { height: '100%', fontSize: 'var(--source-font-size)', backgroundColor: 'rgb(var(--ui-surface))', color: 'rgb(var(--ui-foreground))' },
  '.cm-scroller': { overflow: 'auto', fontFamily: 'var(--source-font)', lineHeight: 'var(--source-line-height)' },
  '.cm-content': { padding: 'var(--source-padding-y) 0', caretColor: 'rgb(var(--ui-foreground))' },
  '.cm-gutters': { backgroundColor: 'rgb(var(--ui-panel))', color: 'rgb(var(--ui-text-600))', borderRight: '1px solid rgb(var(--ui-border))' },
  '.cm-lineNumbers .cm-gutterElement': { boxSizing: 'border-box', minWidth: 'calc(var(--source-gutter-width) - 1px)', padding: '0 8px 0 4px' },
  '&.cm-focused': { outline: 'none' },
  '& .cm-selectionBackground, &.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground': {
    backgroundColor: 'rgb(var(--ui-selected))',
  },
  '& .cm-content ::selection': { backgroundColor: 'rgb(var(--ui-selected))' },
});

// CSS variables recolor syntax in place; toggling appearance never resets scroll or selection.
const highlight = HighlightStyle.define([
  { tag: [tags.keyword, tags.attributeName, tags.propertyName], color: 'rgb(var(--ui-code-key))' },
  { tag: [tags.string, tags.attributeValue], color: 'rgb(var(--ui-code-string))' },
  { tag: [tags.number, tags.bool, tags.null], color: 'rgb(var(--ui-code-literal))' },
  { tag: [tags.comment, tags.meta], color: 'rgb(var(--ui-code-comment))' },
  { tag: [tags.tagName, tags.typeName], color: 'rgb(var(--ui-code-tag))' },
]);

export function HTMLSource({ text }: { text: string }) {
  const host = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!host.current) return;
    const view = new EditorView({ parent: host.current, state: EditorState.create({
      doc: text,
      extensions: [lineNumbers(), drawSelection(), html(), syntaxHighlighting(highlight), theme,
        EditorState.readOnly.of(true), EditorView.editable.of(false),
        EditorView.contentAttributes.of({ tabindex: '0', 'aria-label': 'HTML 只读源码', 'aria-readonly': 'true' })],
    }) });
    return () => view.destroy();
  }, [text]);
  return <div ref={host} className="min-h-0 min-w-0 flex-1 overflow-hidden" />;
}
