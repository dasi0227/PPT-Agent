import { ExternalLink, Search } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { repositoriesApi } from '../../api/repositories';
import { skillsApi } from '../../api/skills';
import type { Skill } from '../../api/types';
import { showGlobalError } from '../../stores/toastStore';
import { RepositoryShell } from './RepositoryShell';

type SkillFilter = 'all' | 'enabled' | 'disabled';

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
    if (!selectedId) return;
    void repositoriesApi.getSkill(selectedId).then((detail) => {
      setSkills((current) => current.map((skill) => skill.id === detail.id ? { ...skill, ...detail } : skill));
    }).catch(() => {});
  }, [selectedId]);

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
    <RepositoryShell section="skill" title="技能" onRefresh={() => void load()}>
      <div className="flex h-full min-h-0 flex-col">
        <header className="flex h-14 shrink-0 items-center justify-between gap-3 border-b border-border bg-panel px-4">
          <h1 className="text-base font-semibold">技能</h1>
          <label className="flex h-8 w-full max-w-64 items-center gap-2 rounded-md border border-border bg-surface px-2 text-text-400">
            <Search className="h-4 w-4" /><input value={query} onChange={(event) => setQuery(event.target.value)} className="min-w-0 flex-1 bg-transparent text-sm text-text-900 outline-none" placeholder="搜索技能" aria-label="搜索技能" />
          </label>
        </header>
        {loading ? <RepositoryState text="正在加载技能" /> : error ? <RepositoryState text={error} error /> : (
          <div className="grid min-h-0 flex-1 grid-cols-1 md:grid-cols-[320px_minmax(0,1fr)]">
            <aside className="flex min-h-0 flex-col border-b border-border bg-panel md:border-b-0 md:border-r">
              <div className="flex h-11 shrink-0 items-center gap-1 border-b border-border px-3">
                {(['all', 'enabled', 'disabled'] as SkillFilter[]).map((value) => <button key={value} type="button" onClick={() => setFilter(value)} className={`h-7 rounded-md px-2 text-xs ${filter === value ? 'bg-surface text-text-900 shadow-sm ring-1 ring-border' : 'text-text-600'}`}>{value === 'all' ? '全部' : value === 'enabled' ? '已启用' : '已关闭'}</button>)}
              </div>
              <div className="min-h-0 flex-1 overflow-y-auto p-2">
                {visible.length === 0 ? <RepositoryState text="没有匹配的技能" /> : visible.map((skill) => (
                  <button key={skill.id} type="button" onClick={() => setSelectedId(skill.id)} className={`mb-1 grid w-full grid-cols-[8px_minmax(0,1fr)] gap-3 rounded-md px-3 py-2.5 text-left ${selected?.id === skill.id ? 'bg-accent-soft' : 'hover:bg-panel-muted'} ${skill.disabled ? 'text-text-400' : ''}`}>
                    <span className={`mt-1.5 h-2 w-2 rounded-full ${skill.disabled ? 'bg-text-400' : 'bg-success'}`} aria-label={skill.disabled ? '已关闭' : '已启用'} />
                    <span className="min-w-0"><span className="block truncate text-sm font-semibold text-text-900">{skill.name}</span><span className="mt-0.5 block line-clamp-2 text-xs">{skill.description}</span></span>
                  </button>
                ))}
              </div>
            </aside>
            {selected && <section className="flex min-h-0 flex-col bg-surface">
              <header className="flex h-14 shrink-0 items-center gap-2 border-b border-border px-4">
                <h2 className="min-w-0 truncate text-sm font-semibold">{selected.name}</h2>
                {selected.open_url && <a href={selected.open_url} title="查看文件" aria-label="查看文件"><ExternalLink className="h-3.5 w-3.5 text-text-500" /></a>}
                <button type="button" role="switch" aria-checked={!selected.disabled} aria-label="切换技能状态" disabled={pending === selected.id} onClick={() => void toggle(selected)} className={`ml-auto h-5 w-9 rounded-full p-0.5 transition-colors ${selected.disabled ? 'bg-border-strong' : 'bg-success'} disabled:opacity-50`}>
                  <span className={`block h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${selected.disabled ? '' : 'translate-x-4'}`} />
                </button>
              </header>
              <article className="prose prose-sm min-h-0 max-w-none flex-1 overflow-y-auto p-6 text-text-900 prose-headings:text-text-900 prose-p:text-text-600">
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{selected.content ?? selected.description}</ReactMarkdown>
              </article>
            </section>}
          </div>
        )}
      </div>
    </RepositoryShell>
  );
}

function RepositoryState({ text, error = false }: { text: string; error?: boolean }) {
  return <div role={error ? 'alert' : 'status'} className={`grid min-h-24 flex-1 place-items-center px-4 text-center text-sm ${error ? 'text-danger' : 'text-text-400'}`}>{text}</div>;
}
