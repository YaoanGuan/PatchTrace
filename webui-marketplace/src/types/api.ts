import type {
  ApiGatewayOverview,
  ExpertOverview,
  Money,
  Rating,
  RatingDirection,
  RegistrationChannel,
  ReputationSummary,
  TaskDetail,
  TaskStatus,
  TaskSummary,
  TaskTrust,
  TraceNode,
  User,
  UserRole,
  WalletOverview,
} from './domain';

/**
 * 第二轮迭代新增的领域类型统一从 `types/api` 再导出，
 * 既有页面/store 直接 `import type { ... } from '.../types/api'` 即可。
 */
export type {
  AiToolCategory,
  AiToolGrade,
  AiToolIntegration,
  AiToolStatus,
  ApiGatewayOverview,
  ApiKeyRecord,
  ApiUsage,
  ComplianceFinding,
  ComplianceSeverity,
  CreateApiKeyInput,
  CreateApiKeyResponse,
  CrossValidation,
  ExpertOverview,
  ModelCatalogItem,
  RechargeInput,
  RiskAssessment,
  RiskLevel,
  SubscribeVipInput,
  TaskTrust,
  ValidationVerdict,
  WalletOverview,
  WalletVip,
} from './domain';

/** GET /api/me */
export type GetMeResponse = User;

/** GET /api/tasks 的查询参数 */
export interface GetTasksQuery {
  status?: TaskStatus;
  keyword?: string;
  rewardMin?: number;
  rewardMax?: number;
  page?: number;
}

/** GET /api/tasks */
export interface GetTasksResponse {
  items: TaskSummary[];
  total: number;
  page: number;
  pageSize: number;
}

/** POST /api/tasks */
export interface PublishTaskInput {
  title: string;
  description: string;
  category: string;
  reward: Money;
  deadline: string;
  acceptanceCriteria: string[];
  requiredSkills: string[];
  complianceAck: boolean;
}

/** POST /api/tasks/:taskId/accept */
export interface AcceptTaskResponse {
  taskId: string;
  status: TaskStatus;
  leaseExpiresAt: string;
}

/** GET /api/tasks/:taskId/trace */
export interface GetTraceResponse {
  taskId: string;
  headHash: string;
  nodes: TraceNode[];
}

/** GET /api/wallet（向后兼容：保留别名，结构扩展 vip 字段） */
export type GetWalletResponse = WalletOverview;

/** POST /api/wallet/recharge */
export type RechargeWalletResponse = WalletOverview;

/** POST /api/wallet/vip */
export type SubscribeVipResponse = WalletOverview;

/** GET /api/expert/overview */
export type GetExpertOverviewResponse = ExpertOverview;

/** GET /api/tasks/:taskId/trust */
export type GetTaskTrustResponse = TaskTrust;

/** GET /api/gateway */
export type GetApiGatewayResponse = ApiGatewayOverview;

/** GET /api/reputation/:userId */
export type GetReputationResponse = ReputationSummary;

/** POST /api/ratings */
export interface SubmitRatingInput {
  taskId: string;
  direction: RatingDirection;
  score: number;
  tags: string[];
  comment: string;
}

export type SubmitRatingResponse = Rating;

export interface RegisterEvidence {
  studentId?: string;
  schoolEmail?: string;
  workEmail?: string;
  realName?: string;
}

/** POST /api/auth/register/:channel */
export interface RegisterChannelInput {
  channel: RegistrationChannel;
  evidence: RegisterEvidence;
}

export interface RegisterChannelResponse {
  channel: RegistrationChannel;
  status: 'PENDING' | 'VERIFIED';
}

/** POST /api/tasks/:taskId/contracts/sign */
export interface SignContractInput {
  taskId: string;
  signerRole: UserRole;
  clausesAccepted: boolean[];
}

export interface SignContractResponse {
  taskId: string;
  signerRole: UserRole;
  clausesAccepted: boolean[];
  signatureHash: string;
  signedAt: string;
}
