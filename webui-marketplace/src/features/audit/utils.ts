import type { GetTraceResponse } from '../../types/api';
import type { TaskStatus, TraceNode } from '../../types/domain';
import type { BadgeVariant } from '../../components/ui/Badge';
import type { ChainIssue, ChainVerification } from './types';

/** 空 / 未加载状态下的校验结果，避免各组件重复构造 */
export const EMPTY_VERIFICATION: ChainVerification = {
  state: 'EMPTY',
  nodeCount: 0,
  headHash: null,
  issues: [],
  brokenAt: null,
  brokenNodeId: null,
  reason: '暂无证据链',
};

/** 哈希展示统一截断：前 10 位 + 后 6 位 */
export function truncateHash(hash: string): string {
  if (!hash) {
    return '—';
  }
  if (hash.length <= 18) {
    return hash;
  }
  return `${hash.slice(0, 10)}…${hash.slice(-6)}`;
}

const dateTimeFormatter = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
});

/** ISO 时间 → 本地化展示，非法输入原样返回 */
export function formatDateTime(value?: string | null): string {
  if (!value) {
    return '—';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return dateTimeFormatter.format(date);
}

/** 模型成本（演示计价单位：美元），0 / 非法值显示占位符 */
export function formatCost(cost: number): string {
  if (!Number.isFinite(cost) || cost <= 0) {
    return '—';
  }
  return `$${cost.toFixed(2)}`;
}

/** 最近节点时间：按 at 取最大值，缺失时退回最后一个节点 */
export function latestNodeAt(nodes: TraceNode[]): string | null {
  if (nodes.length === 0) {
    return null;
  }
  const fallback = nodes[nodes.length - 1]?.at ?? null;
  let latestAt = fallback;
  let latestTime = fallback ? new Date(fallback).getTime() : Number.NEGATIVE_INFINITY;
  for (const node of nodes) {
    const time = new Date(node.at).getTime();
    if (Number.isFinite(time) && time > latestTime) {
      latestTime = time;
      latestAt = node.at;
    }
  }
  return latestAt;
}

/**
 * 逐节点校验哈希链。
 * 规则：
 * 1. 首节点 prevHash 必须为空；
 * 2. 其余节点 prevHash 必须等于上一节点 hash；
 * 3. 节点 evidence.hash 必须与节点 hash 一致（防止证据与链记录脱钩）；
 * 4. 末节点 hash 必须等于接口 headHash。
 */
export function verifyTrace(
  trace: GetTraceResponse | null | undefined,
): ChainVerification {
  const nodes = trace?.nodes ?? [];

  if (!trace || nodes.length === 0) {
    return {
      ...EMPTY_VERIFICATION,
      headHash: trace?.headHash ?? null,
      reason: trace ? '暂无证据链节点' : '证据链加载失败',
    };
  }

  const issues: ChainIssue[] = [];

  nodes.forEach((node, index) => {
    if (index === 0) {
      if (node.prevHash !== null) {
        issues.push({
          index,
          nodeId: node.id,
          reason: '首节点 prevHash 应为空',
        });
      }
    } else {
      const previous = nodes[index - 1];
      if (node.prevHash !== previous.hash) {
        issues.push({
          index,
          nodeId: node.id,
          reason: `prevHash 未指向上一节点（期望 ${truncateHash(previous.hash)}）`,
        });
      }
    }

    if (node.evidence?.hash && node.evidence.hash !== node.hash) {
      issues.push({
        index,
        nodeId: node.id,
        reason: 'evidence.hash 与节点 hash 不一致',
      });
    }
  });

  const lastNode = nodes[nodes.length - 1];
  if (trace.headHash && lastNode.hash !== trace.headHash) {
    issues.push({
      index: nodes.length - 1,
      nodeId: lastNode.id,
      reason: 'headHash 与末节点 hash 不一致',
    });
  }

  const firstIssue = issues[0] ?? null;
  return {
    state: issues.length > 0 ? 'INVALID' : 'VALID',
    nodeCount: nodes.length,
    headHash: trace.headHash || lastNode.hash,
    issues,
    brokenAt: firstIssue?.index ?? null,
    brokenNodeId: firstIssue?.nodeId ?? null,
    reason: firstIssue?.reason ?? null,
  };
}

/** 任务状态中文标签与徽标色调 */
export const TASK_STATUS_META: Record<
  TaskStatus,
  { label: string; variant: BadgeVariant }
> = {
  DRAFT: { label: '草稿', variant: 'outline' },
  PUBLISHED: { label: '已发布', variant: 'info' },
  ACCEPTED: { label: '已接单', variant: 'info' },
  IN_PROGRESS: { label: '进行中', variant: 'info' },
  SUBMITTED: { label: '已提交', variant: 'warning' },
  AI_VALIDATING: { label: 'AI 复核中', variant: 'warning' },
  VALIDATION_FAILED: { label: '复核未通过', variant: 'danger' },
  COMPLIANCE_SCAN: { label: '合规扫描', variant: 'warning' },
  SETTLING: { label: '结算中', variant: 'warning' },
  COMPLETED: { label: '已完成', variant: 'success' },
  DISPUTED: { label: '争议中', variant: 'danger' },
  REFUNDED: { label: '已退款', variant: 'default' },
  CANCELLED: { label: '已取消', variant: 'default' },
};

const FNV_OFFSET = 0x811c9dc5;
const FNV_PRIME = 0x01000193;

function fnv1a(input: string, seed: number): number {
  let hash = (FNV_OFFSET ^ seed) >>> 0;
  for (let index = 0; index < input.length; index += 1) {
    hash ^= input.charCodeAt(index);
    hash = Math.imul(hash, FNV_PRIME) >>> 0;
  }
  return hash >>> 0;
}

/**
 * 演示用确定性签名哈希：同一任务 + 角色 + 条款勾选组合始终生成同一 sha256 形态字符串。
 * 注意：这不是真实密码学签名，仅用于前端演示与视觉一致性。
 */
export function demoSignatureHash(
  taskId: string,
  signerRole: string,
  clausesAccepted: boolean[],
): string {
  const payload = `${taskId}:${signerRole}:${clausesAccepted
    .map((accepted) => (accepted ? '1' : '0'))
    .join('')}`;
  let digest = '';
  for (let block = 0; block < 8; block += 1) {
    digest += fnv1a(`${payload}:${block}`, Math.imul(block + 1, 0x9e3779b1))
      .toString(16)
      .padStart(8, '0');
  }
  return `sha256:${digest}`;
}
