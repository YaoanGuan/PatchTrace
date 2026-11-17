/** 平台身份视角 */
export type UserRole = 'EMPLOYER' | 'COMMANDER';

/** 准入通道 */
export type Channel = 'UNIVERSITY' | 'ENTERPRISE' | 'FREELANCER';

/** 注册通道与准入通道同构 */
export type RegistrationChannel = Channel;

/** 任务状态机 */
export type TaskStatus =
  | 'DRAFT'
  | 'PUBLISHED'
  | 'ACCEPTED'
  | 'IN_PROGRESS'
  | 'SUBMITTED'
  | 'AI_VALIDATING'
  | 'VALIDATION_FAILED'
  | 'COMPLIANCE_SCAN'
  | 'SETTLING'
  | 'COMPLETED'
  | 'DISPUTED'
  | 'REFUNDED'
  | 'CANCELLED';

/**
 * 唯一合法状态迁移表。
 * advanceStage 必须先查这张表，非法迁移直接抛错，禁止静默写入。
 */
export const ALLOWED_TRANSITIONS: Record<TaskStatus, TaskStatus[]> = {
  DRAFT: ['PUBLISHED'],
  PUBLISHED: ['ACCEPTED', 'DISPUTED', 'CANCELLED'],
  ACCEPTED: ['IN_PROGRESS', 'DISPUTED'],
  IN_PROGRESS: ['SUBMITTED', 'DISPUTED'],
  SUBMITTED: ['AI_VALIDATING'],
  AI_VALIDATING: ['COMPLIANCE_SCAN', 'VALIDATION_FAILED'],
  VALIDATION_FAILED: ['IN_PROGRESS'],
  COMPLIANCE_SCAN: ['SETTLING'],
  SETTLING: ['COMPLETED'],
  COMPLETED: [],
  DISPUTED: ['COMPLETED', 'REFUNDED'],
  REFUNDED: [],
  CANCELLED: [],
};

export interface Money {
  amount: number;
  currency: 'CNY';
}

export interface UserSummary {
  id: string;
  displayName: string;
  reputationScore: number;
}

export type PublisherSummary = UserSummary;

export interface TaskFilters {
  status: TaskStatus | 'ALL';
  keyword: string;
  rewardMin?: number;
  rewardMax?: number;
  page: number;
}

export interface TaskSummary {
  id: string;
  title: string;
  category: string;
  status: TaskStatus;
  reward: Money;
  deadline: string;
  publisher: PublisherSummary;
  requiredSkills: string[];
  acceptCount: number;
  validationScore: number;
}

export type PipelineStageKey =
  | 'AI_GENERATION'
  | 'AI_REVIEW'
  | 'COMPLIANCE_SCAN'
  | 'SETTLEMENT';

export type PipelineStageStatus = 'PENDING' | 'RUNNING' | 'DONE' | 'FAILED';

export interface PipelineStage {
  key: PipelineStageKey;
  status: PipelineStageStatus;
  at?: string;
  by?: string;
  summary?: string;
}

export interface Pipeline {
  currentStage: PipelineStageKey;
  stages: PipelineStage[];
}

export type EscrowStatus = 'NONE' | 'FROZEN' | 'RELEASED' | 'REFUNDED';

export interface EscrowState {
  state: EscrowStatus;
  amount: number;
  frozenAt?: string;
}

export interface TaskDetail {
  id: string;
  title: string;
  description: string;
  category: string;
  status: TaskStatus;
  publisher: PublisherSummary;
  assignee: UserSummary | null;
  reward: Money;
  escrow: EscrowState;
  pipeline: Pipeline;
  requiredSkills: string[];
  acceptanceCriteria: string[];
  createdAt: string;
  deadline: string;
}

export type WalletTransactionType =
  | 'ESCROW_FREEZE'
  | 'ESCROW_RELEASE'
  | 'REFUND'
  | 'DEPOSIT'
  | 'WITHDRAW'
  | 'FEE';

export type WalletTransactionStatus = 'PENDING' | 'SETTLED' | 'FAILED';

export interface WalletTransaction {
  id: string;
  type: WalletTransactionType;
  amount: number;
  taskId?: string;
  at: string;
  status: WalletTransactionStatus;
}

export interface WalletBalance {
  available: number;
  frozen: number;
  currency: 'CNY';
}

export type TraceNodeType =
  | 'REQUIREMENT'
  | 'ACCEPTANCE'
  | 'MODEL_CALL'
  | 'ACTION'
  | 'OUTPUT'
  | 'VALIDATION';

export type TraceActorKind = 'USER' | 'AGENT' | 'SYSTEM';

export interface TraceActor {
  kind: TraceActorKind;
  id: string;
  label: string;
}

export interface TraceEvidence {
  hash: string;
  payloadRef: string | null;
  model: string | null;
  cost: number;
}

export interface TraceNode {
  id: string;
  type: TraceNodeType;
  actor: TraceActor;
  at: string;
  summary: string;
  evidence: TraceEvidence;
  prevHash: string | null;
  hash: string;
}

export type SkillDimension =
  | 'INSTRUCTION_PRECISION'
  | 'TASK_CLOSURE'
  | 'COST_EFFICIENCY'
  | 'RISK_CONTROL'
  | 'EVIDENCE_QUALITY';

export interface ReputationDimension {
  key: string;
  label: string;
  score: number;
}

export interface SkillScore extends ReputationDimension {
  key: SkillDimension;
}

export type ExpertiseKind = 'AGENT' | 'DATA_ASSET' | 'PROMPT_TEMPLATE';

export interface ExpertiseItem {
  id: string;
  kind: ExpertiseKind;
  title: string;
  level: string;
  usageCount: number;
  updatedAt: string;
}

export type TrustBadgeType = 'UNIVERSITY' | 'ENTERPRISE' | 'TASK_CLEARANCE';

export type TrustBadgeStatus = 'VERIFIED' | 'PENDING' | 'REVOKED';

export interface TrustBadge {
  id: string;
  type: TrustBadgeType;
  label: string;
  issuer: string;
  verifiedAt: string;
  status: TrustBadgeStatus;
}

export interface UserReputation {
  score: number;
  completedTasks: number;
  onTimeRate: number;
  disputes: number;
}

export type RatingDirection = 'EMPLOYER_TO_COMMANDER' | 'COMMANDER_TO_EMPLOYER';

export interface Rating {
  id: string;
  taskId: string;
  direction: RatingDirection;
  score: number;
  tags: string[];
  comment: string;
  createdAt: string;
}

/** GET /api/reputation/:userId 的响应体 */
export interface ReputationSummary {
  userId: string;
  score: number;
  dimensions: ReputationDimension[];
  ratings: Rating[];
}

export interface User {
  id: string;
  displayName: string;
  avatarUrl: string;
  activeRole: UserRole;
  roles: UserRole[];
  verificationChannel: Channel;
  reputation: UserReputation;
  skills: SkillScore[];
  expertise: ExpertiseItem[];
  badges: TrustBadge[];
}

export type ToastVariant = 'info' | 'success' | 'warning' | 'error';

export interface Toast {
  id: string;
  title: string;
  description?: string;
  variant?: ToastVariant;
}

/* ------------------------------------------------------------------ */
/* M2 AI 专家（信任收费站 / Trust Layer）                              */
/* ------------------------------------------------------------------ */

export type AiToolCategory = 'CLOUD_COMPUTER' | 'AI_CLI' | 'AI_IDE' | 'CHAT_UI';

export type AiToolGrade = 'S' | 'A' | 'B' | 'C';

export type AiToolStatus = 'CONNECTED' | 'PENDING' | 'SANDBOX';

export interface AiToolIntegration {
  id: string;
  name: string;
  vendor: string;
  category: AiToolCategory;
  protocol: string;
  grade: AiToolGrade;
  status: AiToolStatus;
  roles: UserRole[];
  permissions: string[];
  compliance: string[];
}

export type ValidationVerdict = 'PASS' | 'FAIL' | 'PENDING';

export interface CrossValidation {
  taskId: string;
  generator: string;
  generatorModel: string;
  generatorVerdict: ValidationVerdict;
  reviewer: string;
  reviewerModel: string;
  reviewerVerdict: ValidationVerdict;
  consensus: ValidationVerdict;
  confidence: number;
  cost: number;
  elapsedMs: number;
  at: string;
}

export type ComplianceSeverity = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';

export interface ComplianceFinding {
  id: string;
  rule: string;
  severity: ComplianceSeverity;
  status: 'PASS' | 'WARN' | 'BLOCK';
  detail: string;
}

export type RiskLevel = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';

export interface RiskAssessment {
  score: number;
  level: RiskLevel;
  threshold: number;
  blocked: boolean;
  reasons: string[];
}

export interface ExpertOverview {
  tools: AiToolIntegration[];
  validation: CrossValidation;
  compliance: ComplianceFinding[];
  risk: RiskAssessment;
  pricing: { service: string; unit: string; price: number }[];
}

export interface TaskTrust {
  taskId: string;
  validation: CrossValidation;
  compliance: ComplianceFinding[];
  risk: RiskAssessment;
}

/* ------------------------------------------------------------------ */
/* 钱包 VIP 与 API 中转站                                              */
/* ------------------------------------------------------------------ */

export interface WalletVip {
  level: 'FREE' | 'PRO' | 'ENTERPRISE';
  expiresAt: string | null;
  feeRate: number;
}

export interface WalletOverview {
  balance: WalletBalance;
  transactions: WalletTransaction[];
  vip: WalletVip;
}

export interface RechargeInput {
  amount: number;
}

export interface SubscribeVipInput {
  level: 'PRO' | 'ENTERPRISE';
}

export interface ModelCatalogItem {
  id: string;
  provider: string;
  name: string;
  contextWindow: number;
  inputPricePerMTok: number;
  outputPricePerMTok: number;
  latencyMs: number;
  status: 'ONLINE' | 'DEGRADED';
  capabilities: string[];
}

export interface ApiKeyRecord {
  id: string;
  label: string;
  maskedKey: string;
  createdAt: string;
  lastUsedAt: string | null;
  scopes: string[];
  status: 'ACTIVE' | 'REVOKED';
}

export interface ApiUsage {
  period: string;
  tokensIn: number;
  tokensOut: number;
  requests: number;
  spend: number;
}

export interface ApiGatewayOverview {
  models: ModelCatalogItem[];
  keys: ApiKeyRecord[];
  usage: ApiUsage;
}

export interface CreateApiKeyInput {
  label: string;
  scopes: string[];
}

export interface CreateApiKeyResponse {
  key: ApiKeyRecord;
  plaintextKey: string;
}
