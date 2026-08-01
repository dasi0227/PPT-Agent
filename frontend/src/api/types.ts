export interface Project {
  id: string;
  title: string;
  work_dir: string;
  theme: string;
  status: 'draft' | 'generating' | 'ready';
  design_path: string;
  deck_path?: string;
  deck_revision?: number;
  design_revision?: number;
  created_at: number;
  updated_at: number;
}

export interface SlideBlueprint {
  schema_version: '2.0';
  revision: number;
  slide_id: string;
  section_id: string;
  subsection_id?: string;
  role: string;
  title: string;
  key_message: string;
  content: { summary: string; points: string[] };
  visual_intent: { archetype: string; description: string; asset_queries: string[] };
  speaker_notes: string;
  created_at: number;
  updated_at: number;
}

export interface DeckBlueprint {
  schema_version: '2.0';
  revision: number;
  project_id: string;
  title: string;
  goal: string;
  audience: string;
  language: string;
  core_thesis: string;
  narrative_arc: string;
  sections: Array<{ id: string; number: string; title: string; subsections: Array<{ id: string; number: string; title: string }> }>;
  slide_order: string[];
  created_at: number;
  updated_at: number;
}

export interface DesignSpec {
  schema_version: '2.0';
  revision: number;
  canvas: Record<string, unknown>;
  palette: string[];
  typography: Record<string, unknown>;
  spacing: Record<string, unknown>;
  radius: Record<string, unknown>;
  shadows: Record<string, unknown>;
  layout_system: Record<string, unknown>;
  signature: string;
  motion: Record<string, unknown>;
}

export type MaterializationState = 'not_materialized' | 'fresh' | 'blueprint_stale' | 'design_stale' | 'unknown';
export interface Materialization {
  state: MaterializationState;
  revisions: { presentation: number; source_deck: number; source_blueprint: number; source_design: number };
}

export interface Slide {
  id: string;
  project_id: string;
  position: number;
  layout: string;
  title: string;
  html_path: string;
  json_path: string;
  current_version: number;
  blueprint_revision?: number;
  presentation_revision?: number;
  source_deck_revision?: number;
  source_blueprint_revision?: number;
  source_design_revision?: number;
  blueprint?: SlideBlueprint;
  materialization?: Materialization;
}

export interface Thread {
  id: string;
  project_id: string;
  title: string;
  status: string;
  history_path: string;
  created_at: number;
  updated_at: number;
}

export interface Run {
  id: string;
  thread_id: string;
  project_id: string;
  status: 'pending' | 'running' | 'waiting' | 'done' | 'failed' | 'canceled';
  target: RunTarget;
  interaction: RunInteraction;
  events_url: string;
}

export type Artifact = 'blueprint' | 'presentation';
export type TargetLevel = 'slide' | 'deck';
export type InteractionIntent = 'apply' | 'consult';
export type ClarificationPolicy = 'when_blocked' | 'before_apply' | 'never';
export type ExecutionStrategy = 'respond' | 'direct_action' | 'compact_workflow' | 'full_pev';

export interface RunTarget { artifact: Artifact; level: TargetLevel; slide_id?: string }
export interface RunInteraction { intent: InteractionIntent; clarification: ClarificationPolicy }

export interface CreateRunRequest {
  target: RunTarget;
  interaction: RunInteraction;
  instruction: string;
  options?: { language?: string; theme_id?: string; desired_slide_count?: number };
}

export interface BlueprintProjectView {
  deck: DeckBlueprint;
  slides: Record<string, SlideBlueprint>;
  design_spec: DesignSpec;
  materialization: Record<string, Materialization>;
}

export interface NeedsInputPayload {
  content: string;
  reply_to: string;
}

export type JsonRecord = Record<string, unknown>;

export type SSEEventName =
  | 'run.started'
  | 'context.assembled'
  | 'strategy.selected'
  | 'plan.created'
  | 'stage.started'
  | 'stage.completed'
  | 'step.started'
  | 'step.completed'
  | 'step.failed'
  | 'tool.called'
  | 'tool.completed'
  | 'verification.completed'
  | 'repair.started'
  | 'repair.completed'
  | 'artifact.staged'
  | 'artifact.committed'
  | 'status.summary'
  | 'needs_input'
  | 'run.completed'
  | 'run.failed'
  | 'run.canceled';

export type PlanStepStatus = 'pending' | 'in_progress' | 'completed' | 'failed' | 'skipped';

export interface PlanStep {
  id: string;
  title: string;
  status: PlanStepStatus;
  detail?: string;
}

export interface PlanState {
  id: string;
  title: string;
  steps: PlanStep[];
}

export interface WorkflowIssue {
  code?: string;
  message?: string;
  evidence?: string;
  [key: string]: unknown;
}

export interface ArtifactRef {
  kind?: string;
  id?: string;
  path?: string;
  [key: string]: unknown;
}

export interface StructuredOutcome {
  status?: string;
  strategy?: ExecutionStrategy;
  summary?: string;
  code?: string;
  message?: string;
  target?: RunTarget;
  operation?: string;
  affected?: ArtifactRef[];
  issues?: WorkflowIssue[];
  repair_rounds?: number;
  [key: string]: unknown;
}

interface SSEEventBase<Name extends SSEEventName, Data> {
  id?: string;
  event: Name;
  data: Data;
}

export type SSEEvent =
  | SSEEventBase<'run.started', JsonRecord>
  | SSEEventBase<'context.assembled', JsonRecord & { profile?: string; warnings?: string[]; read_only?: boolean }>
  | SSEEventBase<'strategy.selected', JsonRecord & { strategy: ExecutionStrategy; reason?: string; risk?: string; complexity?: string }>
  | SSEEventBase<'plan.created', JsonRecord & { plan: JsonRecord & { id?: string; goal?: string; steps?: JsonRecord[] } }>
  | SSEEventBase<'stage.started' | 'stage.completed', JsonRecord & { stage?: string; duration_ms?: number }>
  | SSEEventBase<'step.started' | 'step.completed' | 'step.failed', JsonRecord & { step_id: string; summary?: string }>
  | SSEEventBase<'tool.called', JsonRecord & { call_id: string; tool: string; args?: JsonRecord }>
  | SSEEventBase<'tool.completed', JsonRecord & { call_id: string; ok?: boolean; summary?: string; issues?: WorkflowIssue[] }>
  | SSEEventBase<'verification.completed', JsonRecord & { verifier?: string; result?: JsonRecord & { passed?: boolean; issues?: WorkflowIssue[] } }>
  | SSEEventBase<'repair.started' | 'repair.completed', JsonRecord & { round?: number; artifact?: ArtifactRef; improved?: boolean; summary?: string }>
  | SSEEventBase<'artifact.staged' | 'artifact.committed', JsonRecord & { artifact: ArtifactRef; change?: string }>
  | SSEEventBase<'status.summary', JsonRecord & { summary?: string }>
  | SSEEventBase<'needs_input', JsonRecord & { id: string; prompt: string; choices?: string[] }>
  | SSEEventBase<'run.completed' | 'run.failed' | 'run.canceled', JsonRecord & { outcome?: StructuredOutcome }>;
