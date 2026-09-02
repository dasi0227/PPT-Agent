import { Check, Pause } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { repositoriesApi } from '../../api/repositories';
import { skillsApi } from '../../api/skills';
import type { Skill, SkillTag } from '../../api/types';
import { cn } from '../../lib/utils';
import { showGlobalError } from '../../stores/toastStore';
import {
  RepositoryCatalog,
  RepositoryDetail,
  RepositoryDirectoryItem,
  RepositoryFilterButton,
  RepositoryLoading,
  RepositoryPageHeader,
  RepositoryState,
  RepositoryTagList,
  RepositoryWorkspace,
} from './RepositoryPrimitives';
import { RepositoryEditDialog } from './RepositoryEditDialog';
import { RepositoryShell } from './RepositoryShell';

const skillTagLabels: Record<SkillTag, string> = {
  workflow: '工作流',
  methodology: '方法论',
  manual: '操作手册',
  experience: '开发经验',
  other: '其它',
};

const skillTagOrder = Object.keys(skillTagLabels) as SkillTag[];

export function SkillRepositoryPage() {
  const [skills, setSkills] = useState<Skill[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<SkillTag | 'all'>('all');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [pending, setPending] = useState<string | null>(null);
  const [editOpen, setEditOpen] = useState(false);

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
    const tags = skill.tags ?? [];
    return (filter === 'all' || tags.includes(filter)) &&
      `${skill.name} ${skill.description} ${tags.flatMap((tag) => [tag, skillTagLabels[tag]]).join(' ')}`
        .toLowerCase().includes(query.toLowerCase());
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

  const deleteSkill = async (skill: Skill) => {
    try {
      await repositoriesApi.deleteSkill(skill.id);
      setSkills((current) => current.filter((value) => value.id !== skill.id));
      setSelectedId('');
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '技能删除失败');
      throw cause;
    }
  };
  const updateSkill = async (value: { name: string; description: string; tags: SkillTag[] }) => {
    if (!selected) return;
    try {
      const updated = await repositoriesApi.updateSkill(selected.id, value);
      setSkills((current) => current.map((skill) => skill.id === updated.id ? { ...skill, ...updated } : skill));
    } catch (cause) {
      showGlobalError(cause instanceof Error ? cause.message : '技能更新失败');
      throw cause;
    }
  };

  return (
    <RepositoryShell section="skill" onRefresh={() => void load()}>
      <div className="flex h-full min-h-0 flex-col">
        <RepositoryPageHeader title="技能" query={query} onQueryChange={setQuery} searchLabel="搜索技能" />
        {loading ? <RepositoryLoading aside /> : error ? <RepositoryState text={error} error /> : (
          <RepositoryWorkspace>
            <RepositoryCatalog
              label="技能列表"
              controls={(['all', ...skillTagOrder] as const).map((tag) => (
                  <RepositoryFilterButton
                    key={tag}
                    active={filter === tag}
                    onClick={() => setFilter(tag)}
                  >
                    {tag === 'all' ? '全部' : skillTagLabels[tag]}
                  </RepositoryFilterButton>
                ))}
            >
              {visible.length === 0 ? <RepositoryState text="没有匹配的技能" className="min-h-40" /> : visible.map((skill) => {
                  const active = selected?.id === skill.id;
                  return (
                    <RepositoryDirectoryItem
                      key={skill.id}
                      active={active}
                      disabled={skill.disabled}
                      name={skill.name}
                      description={skill.description}
                      visual={skill.disabled
                        ? <Pause className="h-4 w-4" strokeWidth={1.75} />
                        : <Check className="h-4 w-4" strokeWidth={1.75} />}
                      visualClassName={cn(
                        'h-9 w-9 rounded-full border-0',
                        skill.disabled ? 'bg-panel-muted text-text-400' : 'bg-success-soft text-success',
                      )}
                      onClick={() => setSelectedId(skill.id)}
                    />
                  );
                })}
            </RepositoryCatalog>
            {selected && (
              <RepositoryDetail
                label="技能说明"
                title={selected.name}
                description={selected.description}
                openUrl={selected.open_url}
                properties={<RepositoryTagList tags={(selected.tags ?? []).map((tag) => skillTagLabels[tag])} />}
                actions={(
                  <div className="flex items-center gap-2.5">
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
                )}
                contentClassName="px-5 py-7 md:px-8 md:py-9"
                deleteNoun="技能"
                onEdit={() => setEditOpen(true)}
                onDelete={() => deleteSkill(selected)}
              >
                <article className="prose prose-sm mx-auto max-w-3xl rounded-lg border border-border bg-surface px-7 py-8 text-text-900 shadow-[0_12px_34px_rgba(51,65,85,0.08)] prose-headings:font-bold prose-headings:tracking-[-0.015em] prose-headings:text-text-900 prose-h1:mb-3 prose-h1:text-2xl prose-h2:mb-3 prose-h2:mt-8 prose-h2:text-base prose-p:text-[13px] prose-p:leading-7 prose-p:text-[#354150] prose-li:text-[13px] prose-li:leading-6 prose-blockquote:border-accent prose-blockquote:bg-accent-soft/60 prose-blockquote:px-4 prose-blockquote:py-1 prose-blockquote:not-italic prose-code:rounded prose-code:bg-panel-muted prose-code:px-1 prose-code:py-0.5 prose-code:text-[12px] prose-pre:border prose-pre:border-border prose-pre:bg-text-900">
                  <ReactMarkdown remarkPlugins={[remarkGfm]}>{selected.content ?? selected.description}</ReactMarkdown>
                </article>
              </RepositoryDetail>
            )}
            {!selected && <RepositoryState text="请选择一个技能" className="min-h-[420px] bg-canvas/70" />}
          </RepositoryWorkspace>
        )}
      </div>
      {selected && (
        <RepositoryEditDialog
          open={editOpen}
          title="编辑技能"
          value={{ name: selected.name, description: selected.description, tags: selected.tags ?? [] }}
          tagOptions={skillTagOrder.map((tag) => ({ value: tag, label: skillTagLabels[tag] }))}
          onOpenChange={setEditOpen}
          onSave={updateSkill}
        />
      )}
    </RepositoryShell>
  );
}
