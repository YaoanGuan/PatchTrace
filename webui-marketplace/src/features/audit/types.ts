import type { GetTraceResponse } from '../../types/api';
import type { TaskSummary } from '../../types/domain';

/** 封印链校验状态：待校验 / 封印完好 / 封印破损 / 暂无战报 */
export type ChainState = 'PENDING' | 'VALID' | 'INVALID' | 'EMPTY';

/** 列表页封印链筛选条件 */
export type ChainFilter = 'ALL' | 'VALID' | 'INVALID';

/** 单条封印破损记录 */
export interface ChainIssue {
  /** 异常节点在节点数组中的下标 */
  index: number;
  nodeId: string;
  reason: string;
}

/** verifyTrace 的统一输出，供列表、详情与汇总组件复用 */
export interface ChainVerification {
  state: ChainState;
  nodeCount: number;
  /** 封印链头哈希（优先取接口 headHash，缺失时退化为末节点 hash） */
  headHash: string | null;
  issues: ChainIssue[];
  brokenAt: number | null;
  brokenNodeId: string | null;
  reason: string | null;
}

/** 战报列表的一行数据：副本摘要 + 封印链 + 现算校验结果 */
export interface AuditTaskEntry {
  task: TaskSummary;
  trace: GetTraceResponse | null;
  chain: ChainVerification;
  lastNodeAt: string | null;
}

/** 三方责任主体 */
export type LiabilityParty = 'EMPLOYER' | 'COMMANDER' | 'PLATFORM';

/** 战报契约条款 */
export interface LiabilityClause {
  id: string;
  party: LiabilityParty;
  title: string;
  detail: string;
}

export type ComplianceCheckStatus = 'PASS' | 'WARN' | 'FAIL';

/** 副本规则报告中的单项检查 */
export interface ComplianceCheck {
  key: string;
  label: string;
  status: ComplianceCheckStatus;
  detail: string;
}
