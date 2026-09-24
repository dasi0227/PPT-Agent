import React from 'react';
import { EditorState, Compartment, Transaction } from '@codemirror/state';
import { EditorView, keymap, lineNumbers, highlightActiveLine, drawSelection, highlightSpecialChars } from '@codemirror/view';
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
import { bracketMatching, defaultHighlightStyle, indentOnInput, syntaxHighlighting } from '@codemirror/language';
import { searchKeymap, highlightSelectionMatches } from '@codemirror/search';
import { json, jsonParseLinter } from '@codemirror/lang-json';
import { html } from '@codemirror/lang-html';
import { lintGutter, linter, setDiagnostics } from '@codemirror/lint';
import type { SourceKind } from '../api/slideSources';
import { fixedBindings, matchesShortcut } from '../lib/shortcuts';

const sessions = new Map<string, { state: EditorState; scrollTop: number }>();
const sourceTheme = EditorView.theme({
  '&': { height: '100%', fontSize: '13px', backgroundColor: 'var(--color-surface, #fff)' },
  '.cm-scroller': { overflow: 'auto', scrollbarWidth: 'none', fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', lineHeight: '1.6' },
  '.cm-scroller::-webkit-scrollbar': { display: 'none' },
  '.cm-content': { minHeight: '100%', padding: '14px 0' },
  '.cm-gutters': { backgroundColor: 'var(--color-panel, #fafafa)', color: '#94a3b8', borderRight: '1px solid #e2e8f0' },
  '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: '#f3f7fc' },
  '&.cm-focused': { outline: 'none' },
});

export function SourceEditor({ resourceKey, kind, text, resetVersion, readOnly, selectionAnchor, scrollTop, diagnostics, systemUpdatedAt, onChange, onSave, onFlush }: {
  resourceKey: string; kind: SourceKind; text: string; resetVersion: number; readOnly: boolean;
  selectionAnchor?: number; scrollTop?: number;
  diagnostics?: { from: number; to: number; message: string; severity: 'error' | 'warning' }[];
  systemUpdatedAt?: number;
  onChange: (text: string, selectionAnchor: number, scrollTop: number) => void;
  onSave: () => void;
  onFlush: () => void;
}) {
  const host = React.useRef<HTMLDivElement>(null);
  const view = React.useRef<EditorView | null>(null);
  const sync = React.useRef(false);
  const latest = React.useRef({ onChange, onSave, onFlush, systemUpdatedAt });
  latest.current = { onChange, onSave, onFlush, systemUpdatedAt };
  const permission = React.useRef(new Compartment());

  React.useEffect(() => {
    if (!host.current) return;
    const sessionKey = `${resourceKey}:${resetVersion}`;
    const cached = sessions.get(sessionKey);
    const initial = cached?.state.doc.toString() === text ? cached : undefined;
    const state = initial?.state ?? EditorState.create({
      doc: text,
      selection: { anchor: Math.min(selectionAnchor ?? 0, text.length) },
      extensions: [
        lineNumbers(), highlightActiveLine(), drawSelection(), highlightSpecialChars(),
        history(), indentOnInput(), bracketMatching(), highlightSelectionMatches(), lintGutter(),
        syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
        kind === 'spec' ? json() : html(),
        kind === 'spec' ? linter(jsonParseLinter()) : [],
        sourceTheme,
        permission.current.of([EditorState.readOnly.of(readOnly), EditorView.editable.of(!readOnly)]),
        EditorView.domEventHandlers({ blur() { latest.current.onFlush(); }, keydown(event) {
          if (!matchesShortcut(event, fixedBindings['deck.save'])) return false;
          event.preventDefault(); event.stopPropagation();
          if (!event.repeat) latest.current.onSave();
          return true;
        } }),
        keymap.of([indentWithTab, ...searchKeymap, ...historyKeymap, ...defaultKeymap]),
        EditorView.updateListener.of((update) => {
          if ((!update.docChanged && !update.selectionSet) || sync.current) return;
          if (kind === 'spec' && update.docChanged && update.transactions.some((transaction) => transaction.isUserEvent('undo') || transaction.isUserEvent('redo')) && latest.current.systemUpdatedAt !== undefined) {
            const currentText = update.state.doc.toString();
            const match = /"updated_at"\s*:\s*(\d+)/.exec(currentText);
            if (match && Number(match[1]) !== latest.current.systemUpdatedAt) {
              const from = match.index + match[0].length - match[1].length;
              queueMicrotask(() => {
                if (update.view.state.doc.toString() !== currentText) return;
                sync.current = true;
                update.view.dispatch({ changes: { from, to: from + match[1].length, insert: String(latest.current.systemUpdatedAt) }, annotations: Transaction.addToHistory.of(false) });
                sync.current = false;
                latest.current.onChange(update.view.state.doc.toString(), update.view.state.selection.main.anchor, update.view.scrollDOM.scrollTop);
              });
              return;
            }
          }
          latest.current.onChange(update.state.doc.toString(), update.state.selection.main.anchor, update.view.scrollDOM.scrollTop);
        }),
      ],
    });
    const editor = new EditorView({ state, parent: host.current });
    view.current = editor;
    editor.scrollDOM.scrollTop = initial?.scrollTop ?? scrollTop ?? 0;
    const onScroll = () => latest.current.onChange(editor.state.doc.toString(), editor.state.selection.main.anchor, editor.scrollDOM.scrollTop);
    editor.scrollDOM.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      editor.scrollDOM.removeEventListener('scroll', onScroll);
      sessions.set(sessionKey, { state: editor.state, scrollTop: editor.scrollDOM.scrollTop });
      if (view.current === editor) view.current = null;
      editor.destroy();
    };
    // A new resetVersion intentionally discards the old undo stack.
  }, [resourceKey, resetVersion, kind]);

  React.useEffect(() => {
    const editor = view.current;
    if (!editor) return;
    editor.dispatch({ effects: permission.current.reconfigure([EditorState.readOnly.of(readOnly), EditorView.editable.of(!readOnly)]) });
  }, [readOnly, resourceKey, resetVersion]);

  React.useEffect(() => {
    const editor = view.current;
    if (!editor || editor.state.doc.toString() === text) return;
    sync.current = true;
    editor.dispatch({ changes: { from: 0, to: editor.state.doc.length, insert: text } });
    sync.current = false;
  }, [text, resourceKey, resetVersion]);

  React.useEffect(() => {
    const editor = view.current;
    if (!editor) return;
    editor.dispatch(setDiagnostics(editor.state, (diagnostics ?? []).map((item) => ({
      ...item, from: Math.min(item.from, editor.state.doc.length), to: Math.min(item.to, editor.state.doc.length),
    }))));
  }, [diagnostics, resourceKey, resetVersion]);

  return <div ref={host} className="h-full min-h-0 min-w-0 overflow-hidden" aria-label={`${kind === 'spec' ? '设计稿 JSON' : '幻灯片 HTML'} 源文件编辑器`} />;
}
