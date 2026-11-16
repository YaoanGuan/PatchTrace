import type { GetTraceResponse } from '../../types/api';
import type { TaskSummary } from '../../types/domain';

/** 哈希链校验状态：待校验 / 通过 / 异常 / 暂无证据链 */
export type ChainState = 'PENDING' | 'VALID' | 'INVALID' | 'EMPTY';

/** 列表页哈希链筛选条件 */
export type ChainFilter = 'ALL' | 'VALID' | 'INVALID';

/** 单条断链 / 异常记录 */
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
  /** 证据链头哈希（优先取接口 headHash，缺失时退化为末节点 hash） */
  headHash: string | null;
  issues: ChainIssue[];
  brokenAt: number | null;
  brokenNodeId: string | null;
  reason: string | null;
}

/** 审计列表的一行数据：任务摘要 + 证据链 + 现算校验结果 */
export interface AuditTaskEntry {
  task: TaskSummary;
  trace: GetTraceResponse | null;
  chain: ChainVerification;
  lastNodeAt: string | null;
}

/** 三方责任主体 */
export type LiabilityParty = 'EMPLOYER' | 'COMMANDER' | 'PLATFORM';

/** 免责契约条款 */
export interface LiabilityClause {
  id: string;
  party: LiabilityParty;
  title: string;
  detail: string;
}

export type ComplianceCheckStatus = 'PASS' | 'WARN' | 'FAIL';

/** 合规扫描报告中的单项检查 */
export interface ComplianceCheck {
  key: string;
  label: string;
  status: ComplianceCheckStatus;
  detail: string;
}
