import type { BadgeVariant } from '../../components/ui/Badge';
import {
  TASK_STATUS_LABELS as STATUS_LABEL,
  TASK_STATUS_VARIANTS as STATUS_BADGE_VARIANT,
} from '../../lib/taskStatus';
import type {
  EscrowStatus,
  PipelineStageKey,
  PipelineStageStatus,
  TaskDetail,
  WalletTransaction,
  WalletTransactionType,
} from '../../types/domain';
import type { CategoryOption, EscrowRowModel, SkillOption } from './types';

// 状态文案与色调的唯一权威源在 src/lib/taskStatus.ts，这里保留旧导出名兼容现有调用方。
export { STATUS_LABEL, STATUS_BADGE_VARIANT };

export const STAGE_LABEL: Record<PipelineStageKey, string> = {
  AI_GENERATION: 'AI 生成',
  AI_REVIEW: 'AI 复核',
  COMPLIANCE_SCAN: '合规扫描',
  SETTLEMENT: '结算',
};

export const STAGE_DESCRIPTION: Record<PipelineStageKey, string> = {
  AI_GENERATION: '指挥官调度 Agent 产出可复现的交付物与执行证据。',
  AI_REVIEW: '平台审核 AI 独立交叉复核，不直接采信执行方的结论。',
  COMPLIANCE_SCAN: '校验授权边界、敏感数据与法律责任条款。',
  SETTLEMENT: '托管资金释放，结果写入可追溯哈希链。',
};

export const STAGE_STATUS_LABEL: Record<PipelineStageStatus, string> = {
  PENDING: '待执行',
  RUNNING: '进行中',
  DONE: '已完成',
  FAILED: '未通过',
};

export const STAGE_STATUS_VARIANT: Record<PipelineStageStatus, BadgeVariant> = {
  PENDING: 'outline',
  RUNNING: 'info',
  DONE: 'success',
  FAILED: 'danger',
};

export const STAGE_ORDER: PipelineStageKey[] = [
  'AI_GENERATION',
  'AI_REVIEW',
  'COMPLIANCE_SCAN',
  'SETTLEMENT',
];

export const CATEGORY_OPTIONS: CategoryOption[] = [
  { value: 'ALL', label: '全部类别' },
  { value: 'SECURITY', label: '安全审计' },
  { value: 'DATA', label: '数据智能' },
  { value: 'DEVELOPMENT', label: '研发交付' },
  { value: 'RESEARCH', label: '安全研究' },
];

export const SKILL_OPTIONS: SkillOption[] = [
  { value: 'AGENT_ORCHESTRATION', label: 'Agent 编排' },
  { value: 'FUZZING', label: '模糊测试' },
  { value: 'DATA_PIPELINE', label: '数据管线' },
  { value: 'ANOMALY_DETECTION', label: '异常检测' },
  { value: 'TEST_AUTOMATION', label: '测试自动化' },
  { value: 'ACCESS_CONTROL', label: '访问控制' },
  { value: 'KERNEL', label: '内核研究' },
  { value: 'EXPLOIT_REPRODUCTION', label: '漏洞复现' },
  { value: 'NLP_EXTRACTION', label: '语义抽取' },
  { value: 'DATA_QUALITY', label: '数据质量' },
  { value: 'SBOM', label: 'SBOM 分析' },
  { value: 'SUPPLY_CHAIN', label: '供应链安全' },
  { value: 'FORENSICS', label: '电子取证' },
  { value: 'HASH_CHAIN', label: '哈希链' },
];

export const TRANSACTION_TYPE_LABEL: Record<WalletTransactionType, string> = {
  ESCROW_FREEZE: '托管冻结',
  ESCROW_RELEASE: '托管释放',
  REFUND: '退款',
  DEPOSIT: '充值',
  WITHDRAW: '提现',
  FEE: '平台服务费',
};

const DEBIT_TRANSACTIONS: WalletTransactionType[] = [
  'ESCROW_FREEZE',
  'WITHDRAW',
  'FEE',
];

export function formatMoney(amount: number): string {
  const value = Number.isFinite(amount) ? amount : 0;
  return `¥${value.toLocaleString('zh-CN', { maximumFractionDigits: 2 })}`;
}

export function formatDateTime(value?: string | null): string {
  if (!value) {
    return '—';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString('zh-CN');
}

export function categoryLabel(value: string): string {
  return CATEGORY_OPTIONS.find((option) => option.value === value)?.label ?? value;
}

export function skillLabel(value: string): string {
  return SKILL_OPTIONS.find((option) => option.value === value)?.label ?? value;
}

export function transactionSign(type: WalletTransactionType): '+' | '-' {
  return DEBIT_TRANSACTIONS.includes(type) ? '-' : '+';
}

export function transactionTone(type: WalletTransactionType): string {
  return DEBIT_TRANSACTIONS.includes(type) ? 'text-rose-600' : 'text-emerald-600';
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/**
 * 由任务状态推导四阶段流水线的展示状态。
 * 任务状态机是唯一事实来源；mock 中的 pipeline.stages 只用于补充 at/by/summary。
 */
export function deriveStageStatuses(
  task: TaskDetail,
): Record<PipelineStageKey, PipelineStageStatus> {
  const pending = (): Record<PipelineStageKey, PipelineStageStatus> => ({
    AI_GENERATION: 'PENDING',
    AI_REVIEW: 'PENDING',
    COMPLIANCE_SCAN: 'PENDING',
    SETTLEMENT: 'PENDING',
  });

  switch (task.status) {
    case 'DRAFT':
    case 'PUBLISHED':
    case 'ACCEPTED':
    case 'CANCELLED':
      return pending();
    case 'IN_PROGRESS':
      return { ...pending(), AI_GENERATION: 'RUNNING' };
    case 'SUBMITTED':
      return { ...pending(), AI_GENERATION: 'DONE' };
    case 'AI_VALIDATING':
      return { ...pending(), AI_GENERATION: 'DONE', AI_REVIEW: 'RUNNING' };
    case 'VALIDATION_FAILED':
    case 'DISPUTED':
      return {
        ...pending(),
        AI_GENERATION: 'DONE',
        AI_REVIEW: 'FAILED',
      };
    case 'COMPLIANCE_SCAN':
      return {
        ...pending(),
        AI_GENERATION: 'DONE',
        AI_REVIEW: 'DONE',
        COMPLIANCE_SCAN: 'RUNNING',
      };
    case 'SETTLING':
      return {
        ...pending(),
        AI_GENERATION: 'DONE',
        AI_REVIEW: 'DONE',
        COMPLIANCE_SCAN: 'DONE',
        SETTLEMENT: 'RUNNING',
      };
    case 'COMPLETED':
      return {
        AI_GENERATION: 'DONE',
        AI_REVIEW: 'DONE',
        COMPLIANCE_SCAN: 'DONE',
        SETTLEMENT: 'DONE',
      };
    case 'REFUNDED':
      return {
        AI_GENERATION: 'DONE',
        AI_REVIEW: 'DONE',
        COMPLIANCE_SCAN: 'DONE',
        SETTLEMENT: 'FAILED',
      };
    default:
      return task.pipeline.stages.reduce(
        (acc, stage) => ({ ...acc, [stage.key]: stage.status }),
        pending(),
      );
  }
}

/**
 * 钱包流水按 taskId 聚合出托管任务行。
 * 冻结金额 = 冻结流水 - 释放流水 - 退款流水。
 */
export function deriveEscrowRows(
  transactions: WalletTransaction[],
): EscrowRowModel[] {
  interface Acc extends EscrowRowModel {
    released: number;
    refunded: number;
  }

  const map = new Map<string, Acc>();

  for (const tx of transactions) {
    if (!tx.taskId) {
      continue;
    }
    const row: Acc = map.get(tx.taskId) ?? {
      taskId: tx.taskId,
      amount: 0,
      state: 'FROZEN',
      at: undefined,
      released: 0,
      refunded: 0,
    };

    if (tx.type === 'ESCROW_FREEZE') {
      row.amount += tx.amount;
      row.at = tx.at;
    }
    if (tx.type === 'ESCROW_RELEASE') {
      row.released += tx.amount;
    }
    if (tx.type === 'REFUND') {
      row.refunded += tx.amount;
    }

    map.set(tx.taskId, row);
  }

  return [...map.values()]
    .map<EscrowRowModel>(({ released, refunded, ...row }) => ({
      ...row,
      state: (refunded > 0
        ? 'REFUNDED'
        : released > 0
          ? 'RELEASED'
          : 'FROZEN') as EscrowStatus,
      amount: row.amount,
    }))
    .sort((a, b) => (b.at ?? '').localeCompare(a.at ?? ''));
}
