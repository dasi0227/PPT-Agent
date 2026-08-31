import { ChevronRight } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { repositoriesApi } from '../../api/repositories';
import { skillsApi } from '../../api/skills';
import type { Skill } from '../../api/types';
import { cn } from '../../lib/utils';
import { showGlobalError } from '../../stores/toastStore';
import {
  RepositoryFileLink,
  RepositoryPageHeader,
  RepositoryState,
  RepositoryToolbar,
} from './RepositoryPrimitives';
import { RepositoryShell } from './RepositoryShell';

type SkillFilter = 'all' | 'enabled' | 'disabled';

const skillFilters: Array<{ value: SkillFilter; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'enabled', label: '已启用' },
  { value: 'disabled', label: '已关闭' },
];

function SkillLoading() {
  return (
    <div role="status" aria-label="正在加载技能" className="grid min-h-0 flex-1 grid-cols-1 md:grid-cols-[360px_minmax(0,1fr)]">
      <div className="space-y-2 border-r border-border bg-panel p-3">
        {Array.from({ length: 6 }).map((_, index) => (
          <div key={index} className="flex gap-3 rounded-lg p-3">
            <span className="mt-1 h-2.5 w-2.5 animate-pulse rounded-full bg-border-strong" />
            <div className="flex-1 space-y-2"><div className="h-4 w-1/2 animate-pulse rounded bg-border/70" /><div className="h-3 w-full animate-pulse rounded bg-border/60" /></div>
          </div>
        ))}
      </div>
      <div className="bg-[#f3f6fa] p-8">
        <div className="mx-auto max-w-3xl space-y-5 rounded-lg border border-border bg-surface p-8 shadow-[0_10px_30px_rgba(51,65,85,0.08)]">
          <div className="h-7 w-2/5 animate-pulse rounded bg-border/70" />
          <div className="h-4 w-4/5 animate-pulse rounded bg-border/60" />
          <div className="h-px bg-border" />
          <div className="h-4 w-full animate-pulse rounded bg-border/60" />
          <div className="h-4 w-5/6 animate-pulse rounded bg-border/60" />
          <div className="h-28 w-full animate-pulse rounded bg-border/50" />
        </div>
      </div>
    </div>
  );
}

export function SkillRepositoryPage() {
  const [skills, setSkills] = useState<Skill[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<SkillFilter>('all');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [pending, setPending] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const response = await skillsApi.list();
      setSkills(response.skills);
      setSelectedId((value) => value || response.skills[0]?.id || '');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '技能仓库加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const visible = useMemo(() => skills.filter((skill) => {
    const enabled = !skill.disabled;
    return (filter === 'all' || (filter === 'enabled' && enabled) || (filter === 'disabled' && !enabled)) &&
      `${skill.name} ${skill.description}`.toLowerCase().includes(query.toLowerCase());
  }), [filter, query, skills]);
  const selected = visible.find((skill) => skill.id === selectedId) ?? visible[0];

  useEffect(() => {
    if (!selected?.id) return;
    void repositoriesApi.getSkill(selected.id).then((detail) => {
      setSkills((current) => current.map((skill) => skill.id === detail.id ? { ...skill, ...detail } : skill));
    }).catch(() => {});
  }, [selected?.id]);

  const toggle = async (skill: Skill) => {
    const disabled = !skill.disabled;
    setPending(skill.id);
    setSkills((current) => current.map((value) => value.id === skill.id ? { ...value, disabled } : value));
    try {
      const updated = await repositoriesApi.setSkillDisabled(skill.id, disabled);
      setSkills((current) => current.map((value) => value.id === skill.id ? { ...value, ...updated } : value));
    } catch (cause) {
      setSkills((current) => current.map((value) => value.id === skill.id ? { ...value, disabled: skill.disabled } : value));
      showGlobalError(cause instanceof Error ? cause.message : '技能状态更新失败');
    } finally {
      setPending(null);
    }
  };

  return (
    <RepositoryShell section="skill" onRefresh={() => void load()}>
      <div className="flex h-full min-h-0 flex-col">
        <RepositoryPageHeader title="技能" query={query} onQueryChange={setQuery} searchLabel="搜索技能" />
        {loading ? <SkillLoading /> : error ? <RepositoryState text={error} error /> : (
          <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden md:grid-cols-[360px_minmax(0,1fr)] xl:grid-cols-[390px_minmax(0,1fr)]">
            <aside className="flex max-h-[340px] min-h-0 flex-col border-b border-border bg-panel md:max-h-none md:border-b-0 md:border-r" aria-label="技能列表">
              <div className="flex h-11 shrink-0 items-center gap-1 border-b border-border px-3">
                {skillFilters.map((item) => (
                  <button
                    key={item.value}
                    type="button"
                    onClick={() => setFilter(item.value)}
                    className={cn(
                      'h-7 rounded-md px-2.5 text-xs font-semibold transition-all active:translate-y-px focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-1',
                      filter === item.value
                        ? 'bg-surface text-text-900 shadow-[0_1px_3px_rgba(51,65,85,0.12)] ring-1 ring-border'
                        : 'text-text-600 hover:bg-panel-muted hover:text-text-900',
                    )}
                  >
                    {item.label}
                  </button>
                ))}
              </div>
              <div className="min-h-0 flex-1 overflow-y-auto p-2">
                {visible.length === 0 ? <RepositoryState text="没有匹配的技能" className="min-h-40" /> : visible.map((skill) => {
                  const active = selected?.id === skill.id;
                  return (
                    <button
                      key={skill.id}
                      type="button"
                      onClick={() => setSelectedId(skill.id)}
                      className={cn(
                        'group mb-1 grid w-full grid-cols-[14px_minmax(0,1fr)_16px] items-start gap-2 rounded-lg border px-2.5 py-2.5 text-left transition-all active:translate-y-px focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-inset',
                        active
                          ? 'border-accent/30 bg-accent-soft shadow-[0_2px_8px_rgba(47,103,246,0.07)]'
                          : 'border-transparent hover:border-border hover:bg-surface',
                        skill.disabled && 'text-text-400',
                      )}
                    >
                      <span
                        className={cn(
                          'mt-1.5 h-2 w-2 rounded-full',
                          skill.disabled ? 'bg-text-400' : 'bg-success shadow-[0_0_0_3px_rgba(47,125,101,0.10)]',
                        )}
                        aria-label={skill.disabled ? '已关闭' : '已启用'}
                      />
                      <span className="min-w-0">
                        <span className={cn('block truncate text-[13px] font-bold', skill.disabled ? 'text-text-400' : 'text-text-900')}>{skill.name}</span>
                        <span className="mt-1 block line-clamp-2 text-[11px] leading-[1.45] text-text-600">{skill.description}</span>
                      </span>
                      <ChevronRight className={cn('mt-1.5 h-3.5 w-3.5 text-text-400 transition-transform group-hover:translate-x-0.5', active && 'text-accent')} strokeWidth={1.75} />
                    </button>
                  );
                })}
              </div>
            </aside>
            {selected && (
              <section className="flex min-h-[420px] min-w-0 flex-col bg-surface md:min-h-0" aria-label="技能说明">
                <RepositoryToolbar>
                  <div className="flex min-w-0 items-center gap-1">
                    <h2 className="truncate text-sm font-bold text-text-900">{selected.name}</h2>
                    <RepositoryFileLink href={selected.open_url} />
                  </div>
                  <div className="flex shrink-0 items-center gap-2.5">
                    <span className={cn('text-xs font-semibold', selected.disabled ? 'text-text-600' : 'text-success')}>
                      {selected.disabled ? '已关闭' : '已启用'}
                    </span>
                    <button
                      type="button"
                      role="switch"
                      aria-checked={!selected.disabled}
                      aria-label="切换技能状态"
                      disabled={pending === selected.id}
                      onClick={() => void toggle(selected)}
                      className={cn(
                        'h-5 w-9 rounded-full p-0.5 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 disabled:opacity-50',
                        selected.disabled ? 'bg-border-strong' : 'bg-success',
                      )}
                    >
                      <span className={cn('block h-4 w-4 rounded-full bg-white shadow-[0_1px_3px_rgba(23,32,43,0.25)] transition-transform', !selected.disabled && 'translate-x-4')} />
                    </button>
                  </div>
                </RepositoryToolbar>
                <div className="min-h-0 flex-1 overflow-y-auto bg-[#f3f6fa] px-5 py-7 md:px-8 md:py-9">
                  <article className="prose prose-sm mx-auto max-w-3xl rounded-lg border border-border bg-surface px-7 py-8 text-text-900 shadow-[0_12px_34px_rgba(51,65,85,0.08)] prose-headings:font-bold prose-headings:tracking-[-0.015em] prose-headings:text-text-900 prose-h1:mb-3 prose-h1:text-2xl prose-h2:mb-3 prose-h2:mt-8 prose-h2:text-base prose-p:text-[13px] prose-p:leading-7 prose-p:text-[#354150] prose-li:text-[13px] prose-li:leading-6 prose-blockquote:border-accent prose-blockquote:bg-accent-soft/60 prose-blockquote:px-4 prose-blockquote:py-1 prose-blockquote:not-italic prose-code:rounded prose-code:bg-panel-muted prose-code:px-1 prose-code:py-0.5 prose-code:text-[12px] prose-pre:border prose-pre:border-border prose-pre:bg-text-900">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{selected.content ?? selected.description}</ReactMarkdown>
                  </article>
                </div>
              </section>
            )}
          </div>
        )}
      </div>
    </RepositoryShell>
  );
}
