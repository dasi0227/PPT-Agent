import { ClipboardList, LayoutTemplate, List, ListChecks, MessageCircleQuestion, MessagesSquare, NotebookPen, Paintbrush, Type, type LucideIcon } from 'lucide-react';
import type { ProjectContentSnapshot, RunMode } from '../../api/types';
import { ContentRequirementsIcon } from '../../components/ui/ContentRequirementsIcon';
import { GlobalConstraintsIcon } from '../../components/ui/GlobalConstraintsIcon';
import { orderedSlides } from '../deck/selectors';

export type StarterActionId = 'content' | 'outline' | 'constraints' | 'clarify' | 'brainstorm' | 'plan' | 'specification' | 'describe' | 'copy' | 'layout' | 'color';
export interface StarterTemplate {
  id: StarterActionId;
  label: string;
  icon: LucideIcon;
  mode: RunMode;
  scope: 'all' | 'page';
  text: string;
}

const templates: Record<StarterActionId, StarterTemplate> = {
  content: { id: 'content', label: '阐述内容', icon: ContentRequirementsIcon, mode: 'create', scope: 'all', text: "请根据以下信息整理这份 PPT 的基本信息：\n- 主题：【主题】\n- 受众：【目标受众】\n- 目标：【演示目标】\n- 语言：【语言】\n- 页数：【预计页数】\n\n只需整理基本信息，暂时不需要生成设计稿或幻灯片。" },
  outline: { id: 'outline', label: '生成目录', icon: List, mode: 'create', scope: 'all', text: "请根据以下信息整理这份 PPT 的目录结构：\n- 主题：【主题】\n- 章节数量：【章节数】\n- 总页数：【总页数】\n\n只需整理目录结构，暂时不需要生成设计稿或幻灯片。" },
  constraints: { id: 'constraints', label: '规定约束', icon: GlobalConstraintsIcon, mode: 'create', scope: 'all', text: "请根据以下信息设置整份 PPT 的全局约束：\n- 内容要求：【内容要求】\n- 内容禁止项：【内容禁止项】\n- 视觉要求：【视觉要求】\n- 视觉禁止项：【视觉禁止项】\n\n只需设置全局约束，暂时不需要生成设计稿或幻灯片。" },
  clarify: { id: 'clarify', label: '理清需求', icon: MessageCircleQuestion, mode: 'ask', scope: 'all', text: "我准备做一份关于【主题或想法】的 PPT，请先通过提问帮我理清需求。" },
  brainstorm: { id: 'brainstorm', label: '聊聊思路', icon: MessagesSquare, mode: 'chat', scope: 'all', text: "我想做一份关于【主题】的 PPT，一起聊聊【想讨论的问题】。" },
  plan: { id: 'plan', label: '规划制作', icon: ListChecks, mode: 'plan', scope: 'all', text: "我想完成【制作目标】，请结合当前项目制定一份制作计划，先交给我确认。" },
  specification: { id: 'specification', label: '框定规格', icon: ClipboardList, mode: 'create', scope: 'page', text: "请先为当前页框定设计稿：核心信息是【核心信息】，需要呈现【内容元素】，布局上希望【布局要求】。" },
  describe: { id: 'describe', label: '描述页面', icon: NotebookPen, mode: 'create', scope: 'page', text: "这一页我想讲【页面内容】，希望呈现出【视觉效果】。请据此完善设计稿并生成幻灯片。" },
  copy: { id: 'copy', label: '精简文案', icon: Type, mode: 'create', scope: 'page', text: "请把当前页的【需要精简的内容】改得更简洁，保留原有事实、数据和核心结论。" },
  layout: { id: 'layout', label: '优化排版', icon: LayoutTemplate, mode: 'create', scope: 'page', text: "请调整当前页的排版，重点改善【排版问题】，我希望【预期效果】。" },
  color: { id: 'color', label: '修改颜色', icon: Paintbrush, mode: 'create', scope: 'page', text: "请把当前页的【调整对象】改为【目标颜色】，保持其余内容和配色不变。" },
};

export function getStarterTemplates(snapshot: ProjectContentSnapshot): StarterTemplate[] {
  const slides = orderedSlides(snapshot);
  const ids: StarterActionId[] = slides.length === 0
    ? ['content', 'constraints', 'outline', 'clarify', 'brainstorm', 'plan']
    : slides.every(slide => snapshot.slides_by_id[slide.id]?.spec_state === 'ready' && slide.html_state === 'available')
      ? ['copy', 'layout', 'color']
      : ['specification', 'describe'];
  return ids.map(id => templates[id]);
}

export function getStarterTarget(snapshot: ProjectContentSnapshot, currentSlideId: string | null): string | undefined {
  const slides = orderedSlides(snapshot);
  return slides.find(slide => slide.id === currentSlideId)?.id
    ?? slides.find(slide => snapshot.slides_by_id[slide.id]?.spec_state !== 'ready' || slide.html_state !== 'available')?.id
    ?? slides[0]?.id;
}

export function starterTokens(text: string): string[] {
  return [...new Set(Array.from(text.matchAll(/【[^】]+】/g), match => match[0]))];
}

export interface StarterSlot { start: number; end: number }
export function pendingStarterSlots(text: string, tokens: readonly string[]): StarterSlot[] {
  if (tokens.length === 0) return [];
  const slots: StarterSlot[] = [];
  for (const token of new Set(tokens)) {
    let from = 0;
    let index: number;
    while ((index = text.indexOf(token, from)) !== -1) {
      slots.push({ start: index, end: index + token.length });
      from = index + token.length;
    }
  }
  for (const match of text.matchAll(/【\s*】/g)) slots.push({ start: match.index!, end: match.index! + match[0].length });
  return slots.sort((a, b) => a.start - b.start);
}
