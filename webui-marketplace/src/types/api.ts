import type {
  Money,
  Rating,
  RatingDirection,
  RegistrationChannel,
  ReputationSummary,
  TaskDetail,
  TaskStatus,
  TaskSummary,
  TraceNode,
  User,
  UserRole,
  WalletBalance,
  WalletTransaction,
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

/** GET /api/wallet */
export interface GetWalletResponse {
  balance: WalletBalance;
  transactions: WalletTransaction[];
}

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
