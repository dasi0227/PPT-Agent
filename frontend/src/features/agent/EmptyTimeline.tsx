import { useEffect, useMemo, useRef, useState, type PointerEvent } from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { getStarterTemplates, type StarterTemplate } from './starterTemplates';
import './emptyTimeline.css';

const messages = [
  '别和空白页对视了，Dasi 在呢。',
  '别让光标一个人跳舞，跟 Dasi 聊两句。',
  '想法不用穿正装，直接说给 Dasi 听。',
  '脑洞先别收起来，Dasi 给它留了一页。',
  '做 PPT 不是 Dasi 的舒适区，是统治区。',
];
const brand = Array.from('Dasi PPT Agent');
type TypingPhase = 'typing' | 'holding' | 'deleting' | 'between';
interface TypingState { index: number; count: number; phase: TypingPhase }

function useReducedMotion() {
  const [reduced, setReduced] = useState(() => typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return;
    const media = window.matchMedia('(prefers-reduced-motion: reduce)');
    const update = () => setReduced(media.matches);
    media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, []);
  return reduced;
}

interface EmptyTimelineProps {
  actionsVisible: boolean;
  onSelect?: (template: StarterTemplate) => void;
}

export function EmptyTimeline({ actionsVisible, onSelect }: EmptyTimelineProps) {
  const projectId = useProjectStore(state => state.activeProjectId);
  const snapshot = useProjectStore(state => projectId ? state.contentByProjectId[projectId] : undefined);
  const templates = useMemo(() => snapshot ? getStarterTemplates(snapshot) : [], [snapshot]);
  const rootRef = useRef<HTMLDivElement>(null);
  const wordRef = useRef<HTMLButtonElement>(null);
  const pointerFrame = useRef(0);
  const animations = useRef<Animation[]>([]);
  const reduced = useReducedMotion();
  const [onScreen, setOnScreen] = useState(true);
  const [pageVisible, setPageVisible] = useState(!document.hidden);
  const [paused, setPaused] = useState(false);
  const [typing, setTyping] = useState<TypingState>({ index: 0, count: 0, phase: 'typing' });
  const phrase = reduced ? messages[0] : messages[typing.index];
  const characters = useMemo(() => Array.from(phrase), [phrase]);
  const running = onScreen && pageVisible && !paused && !reduced;

  useEffect(() => {
    const update = () => setPageVisible(!document.hidden);
    document.addEventListener('visibilitychange', update);
    return () => document.removeEventListener('visibilitychange', update);
  }, []);
  useEffect(() => {
    const root = rootRef.current;
    if (!root || typeof IntersectionObserver === 'undefined') return;
    const observer = new IntersectionObserver(entries => setOnScreen(entries[0].isIntersecting), { threshold: .05 });
    observer.observe(root);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    if (!running) return;
    const punctuation = /[，。！？、,.!?；：]/.test(characters[typing.count - 1] ?? '');
    const delay = typing.phase === 'holding' ? 3000
      : typing.phase === 'deleting' ? 42
      : typing.phase === 'between' ? 450
      : typing.count === 0 ? 420 : punctuation ? 290 : 95;
    const timer = window.setTimeout(() => {
      setTyping(current => {
        if (current.phase === 'typing') {
          const count = current.count + 1;
          return { ...current, count, phase: count >= characters.length ? 'holding' : 'typing' };
        }
        if (current.phase === 'holding') return { ...current, phase: 'deleting' };
        if (current.phase === 'deleting') {
          const count = Math.max(0, current.count - 1);
          return { ...current, count, phase: count === 0 ? 'between' : 'deleting' };
        }
        return { index: (current.index + 1) % messages.length, count: 0, phase: 'typing' };
      });
    }, delay);
    return () => window.clearTimeout(timer);
  }, [running, typing, characters]);

  function resetPointer() {
    cancelAnimationFrame(pointerFrame.current);
    pointerFrame.current = 0;
    for (const glyph of Array.from(wordRef.current?.children ?? [])) {
      (glyph as HTMLElement).style.transform = '';
      (glyph as HTMLElement).style.color = '';
    }
  }
  function movePointer(event: PointerEvent<HTMLDivElement>) {
    if (event.pointerType === 'touch') return;
    const { clientX, clientY } = event;
    cancelAnimationFrame(pointerFrame.current);
    pointerFrame.current = requestAnimationFrame(() => {
      pointerFrame.current = 0;
      const root = rootRef.current;
      const word = wordRef.current;
      if (!root || !word) return;
      const rect = root.getBoundingClientRect();
      const wordRect = word.getBoundingClientRect();
      const dx = clientX - rect.left - rect.width / 2;
      for (const element of Array.from(word.children)) {
        const glyph = element as HTMLElement;
        const x = wordRect.left + glyph.offsetLeft + glyph.offsetWidth / 2;
        const y = wordRect.top + glyph.offsetTop + glyph.offsetHeight / 2;
        const strength = Math.max(0, 1 - Math.hypot(clientX - x, clientY - y) / 72);
        glyph.style.transform = reduced ? '' : `translateY(${-11 * strength}px) rotate(${dx * strength * .035}deg)`;
        glyph.style.color = strength > .28 ? 'rgb(var(--ui-selected-foreground))' : '';
      }
    });
  }
  function greet() {
    animations.current.forEach(animation => animation.cancel());
    animations.current = [];
    const animate = (element: Element, frames: Keyframe[], options: KeyframeAnimationOptions) => {
      const animation = element.animate(frames, options);
      animations.current.push(animation);
      animation.finished.then(() => {
        animations.current = animations.current.filter(item => item !== animation);
      }).catch(() => {});
    };
    for (const [index, glyph] of Array.from(wordRef.current?.children ?? []).entries()) {
      animate(glyph, reduced ? [
        { color: 'rgb(var(--ui-selected-foreground))' }, { color: 'rgb(var(--ui-text-400))' },
      ] : [
        { transform: 'translateY(0)', color: 'rgb(var(--ui-text-400))' },
        { transform: 'translateY(-12px)', color: 'rgb(var(--ui-selected-foreground))', offset: .32 },
        { transform: 'translateY(2px)', offset: .72 },
        { transform: 'translateY(0)', color: 'rgb(var(--ui-text-400))' },
      ], { duration: 520, delay: reduced ? 0 : index * 22, easing: 'cubic-bezier(.2,.7,.25,1)' });
    }
  }
  useEffect(() => () => {
    cancelAnimationFrame(pointerFrame.current);
    animations.current.forEach(animation => animation.cancel());
  }, []);

  return (
    <div className="empty-timeline" ref={rootRef} onPointerMove={movePointer} onPointerLeave={resetPointer}
      data-ambient-running={onScreen && pageVisible && !reduced}>
      <div className="empty-timeline-content">
        <button type="button" className="empty-timeline-mascot" onClick={greet} aria-label="和 Dasi 打招呼">
          <span className="empty-timeline-shadow" aria-hidden="true" />
          <img src="/logo.png" alt="" width={80} height={80} draggable={false} />
        </button>
        <div className="empty-timeline-brand">
          <button ref={wordRef} type="button" className="empty-timeline-wordmark" onClick={greet} aria-label="Dasi PPT Agent">
            {brand.map((char, index) => <span key={index} aria-hidden="true">{char === ' ' ? '\u00a0' : char}</span>)}
          </button>
          <button type="button" className="empty-timeline-tagline" onClick={() => setPaused(value => !value)}
            aria-label={paused ? '继续轮播文案' : '暂停轮播文案'} aria-pressed={paused} disabled={reduced}
            title={paused ? '继续轮播' : '暂停轮播'} data-running={running} data-phase={typing.phase}>
            <span className="sr-only">{phrase}</span>
            <span className="empty-timeline-line" aria-hidden="true">
              <span className="empty-timeline-measure">{phrase}</span>
              <span className="empty-timeline-ink">{reduced ? phrase : characters.slice(0, typing.count).join('')}<span className="empty-timeline-caret" /></span>
            </span>
          </button>
        </div>
        <div className="empty-timeline-actions" data-count={templates.length} data-hidden={!actionsVisible} aria-hidden={!actionsVisible}>
          {templates.map(template => <button key={template.id} type="button" className="empty-timeline-action ui-interactive"
            disabled={!actionsVisible || !onSelect} onClick={() => onSelect?.(template)}>
            <template.icon className="h-[15px] w-[15px] shrink-0" strokeWidth={1.75} />
            <span>{template.label}</span>
          </button>)}
        </div>
      </div>
    </div>
  );
}
