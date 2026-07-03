export type TimelineItemType = 'markdown' | 'thought' | 'tool_call' | 'plan' | 'artifact' | 'final_result' | 'needs_input' | 'error';

export interface BaseTimelineItem {
  id: string;
  type: TimelineItemType;
  timestamp: number;
}

export interface MarkdownMessageItem extends BaseTimelineItem {
  type: 'markdown';
  text: string;
}

export interface ThoughtItem extends BaseTimelineItem {
  type: 'thought';
  text: string;
}

export interface ToolCallItem extends BaseTimelineItem {
  type: 'tool_call';
  call_id: string;
  tool: string;
  args: any;
  status: 'running' | 'success' | 'failed';
  observation?: any;
  artifacts: ArtifactItem[];
}

export interface PlanItem extends BaseTimelineItem {
  type: 'plan';
  title: string;
  steps: Array<{ id: string, title: string, status: string, detail?: string }>;
}

export interface ArtifactItem extends BaseTimelineItem {
  type: 'artifact';
  artifact_type: string;
  ref: string;
  page_index?: number;
  delivery?: 'intermediate' | 'final';
}

export interface FinalResultItem extends BaseTimelineItem {
  type: 'final_result';
  result: any;
}

export interface NeedsInputItem extends BaseTimelineItem {
  type: 'needs_input';
  prompt: string;
  schema?: any;
  choices?: string[];
}

export interface ErrorItem extends BaseTimelineItem {
  type: 'error';
  code?: string;
  message: string;
}

export type TimelineItem =
  | MarkdownMessageItem
  | ThoughtItem
  | ToolCallItem
  | PlanItem
  | ArtifactItem
  | FinalResultItem
  | NeedsInputItem
  | ErrorItem;

import { SSEEvent } from '../../api/types';

export function reduceSSEEvent(state: TimelineItem[], event: SSEEvent): TimelineItem[] {
  const timestamp = Date.now();
  const newId = event.id || `evt_${timestamp}_${Math.random().toString(36).substring(7)}`;

  switch (event.event) {
    case 'run.started':
      // Clear timeline on new run? Usually handled in store before reducing
      return state;

    case 'thought':
      return [...state, { id: newId, type: 'thought', text: event.data.text, timestamp }];

    case 'tool_call':
      return [...state, {
        id: newId,
        type: 'tool_call',
        call_id: event.data.call_id,
        tool: event.data.tool,
        args: event.data.args,
        status: 'running',
        artifacts: [],
        timestamp
      }];

    case 'tool_result':
      return state.map(item => {
        if (item.type === 'tool_call' && item.call_id === event.data.call_id) {
          return {
            ...item,
            status: event.data.ok ? 'success' : 'failed',
            observation: event.data.observation
          };
        }
        return item;
      });

    case 'artifact':
      // Try to attach to nearest running/success tool_call that hasn't finished (simplistic heuristic)
      // or just last tool_call
      const lastToolCallIndex = [...state].reverse().findIndex(item => item.type === 'tool_call');
      if (lastToolCallIndex !== -1) {
        const realIndex = state.length - 1 - lastToolCallIndex;
        const newState = [...state];
        const tc = newState[realIndex] as ToolCallItem;
        newState[realIndex] = {
          ...tc,
          artifacts: [...tc.artifacts, {
            id: newId,
            type: 'artifact',
            artifact_type: event.data.artifact_type,
            ref: event.data.ref,
            page_index: event.data.page_index,
            delivery: 'intermediate',
            timestamp
          }]
        };
        return newState;
      }
      return [...state, {
        id: newId,
        type: 'artifact',
        artifact_type: event.data.artifact_type,
        ref: event.data.ref,
        page_index: event.data.page_index,
        delivery: 'intermediate',
        timestamp
      }];

    case 'token':
      // Append to last markdown if it exists, else create new
      const lastItemIndex = state.length - 1;
      if (lastItemIndex >= 0 && state[lastItemIndex].type === 'markdown') {
        const newState = [...state];
        const mdItem = newState[lastItemIndex] as MarkdownMessageItem;
        newState[lastItemIndex] = {
          ...mdItem,
          text: mdItem.text + event.data.text
        };
        return newState;
      }
      return [...state, {
        id: newId,
        type: 'markdown',
        text: event.data.text,
        timestamp
      }];

    case 'info':
      return [...state, {
        id: newId,
        type: 'markdown',
        text: event.data.text,
        timestamp
      }];

    case 'needs_input':
      return [...state, {
        id: event.data.id || newId, // Use event data id for reply_to
        type: 'needs_input',
        prompt: event.data.prompt,
        schema: event.data.schema,
        choices: event.data.choices,
        timestamp
      }];

    case 'done':
      return [...state, {
        id: newId,
        type: 'final_result',
        result: event.data.result,
        timestamp
      }];

    case 'error':
      return [...state, {
        id: newId,
        type: 'error',
        code: event.data.code,
        message: event.data.message,
        timestamp
      }];

    case 'progress':
      // Progress usually handled at run level, but can be added if needed
      return state;

    default:
      return state;
  }
}
