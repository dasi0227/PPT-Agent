import { useState } from 'react';
import { Blocks, Box, ChevronRight, ExternalLink } from 'lucide-react';
import type { PublicLoadedResource, Skill } from '../../api/types';
import { TimelineDisclosure } from './TimelineDisclosure';

export function SkillActivity({ skills }: { skills: Skill[] }) {
  const [expanded, setExpanded] = useState(true);
  if (skills.length === 0) return null;

  return (
    <div>
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className="flex min-h-7 w-full items-center gap-2 rounded-md px-1.5 py-0.5 text-left text-[13px] text-text-600 hover:bg-panel-muted"
      >
        <Blocks className="h-4 w-4 shrink-0 text-success" strokeWidth={1.75} />
        <span className="min-w-0 flex-1 truncate">已启用 {skills.length} 个技能</span>
        <ChevronRight
          className={`h-3.5 w-3.5 shrink-0 text-text-400 transition-transform duration-200 ${expanded ? 'rotate-90' : ''}`}
          strokeWidth={1.75}
        />
      </button>
      <TimelineDisclosure open={expanded}>
        <ul className="m-0 space-y-0 py-0.5 pl-8 pr-1 text-xs text-text-600">
          {skills.map((skill) => (
            <li key={skill.id} className="list-disc marker:text-accent">
              {skill.open_url ? (
                <a
                  href={skill.open_url}
                  title="查看文件"
                  className="flex min-h-5 items-center gap-1 rounded px-1 hover:bg-accent-soft hover:text-text-900"
                >
                  <span className="min-w-0 flex-1 truncate">{skill.name}</span>
                  <ExternalLink className="h-3.5 w-3.5 shrink-0 text-text-500" strokeWidth={1.75} />
                </a>
              ) : (
                <span className="flex min-h-5 items-center px-1">{skill.name}</span>
              )}
            </li>
          ))}
        </ul>
      </TimelineDisclosure>
    </div>
  );
}

export function ComponentActivity({ components }: { components: PublicLoadedResource[] }) {
  const [expanded, setExpanded] = useState(true);
  if (components.length === 0) return null;

  return (
    <div>
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
        className="flex min-h-7 w-full items-center gap-2 rounded-md px-1.5 py-0.5 text-left text-[13px] text-text-600 hover:bg-panel-muted"
      >
        <Box className="h-4 w-4 shrink-0 text-accent" strokeWidth={1.75} />
        <span className="min-w-0 flex-1 truncate">已引用 {components.length} 个组件</span>
        <ChevronRight
          className={`h-3.5 w-3.5 shrink-0 text-text-400 transition-transform duration-200 ${expanded ? 'rotate-90' : ''}`}
          strokeWidth={1.75}
        />
      </button>
      <TimelineDisclosure open={expanded}>
        <ul className="m-0 space-y-0 py-0.5 pl-8 pr-1 text-xs text-text-600">
          {components.map((component) => (
            <li key={component.id} className="list-disc marker:text-accent">
              {component.open_url ? (
                <a
                  href={component.open_url}
                  title="查看文件"
                  className="flex min-h-5 items-center gap-1 rounded px-1 hover:bg-accent-soft hover:text-text-900"
                >
                  <span className="min-w-0 flex-1 truncate">{component.name}</span>
                  <ExternalLink className="h-3.5 w-3.5 shrink-0 text-text-500" strokeWidth={1.75} />
                </a>
              ) : (
                <span className="flex min-h-5 items-center px-1">{component.name}</span>
              )}
            </li>
          ))}
        </ul>
      </TimelineDisclosure>
    </div>
  );
}
