import {
  AlertTriangle,
  CheckCircle2,
  ClipboardList,
  XCircle,
} from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import { cn } from '../../../lib/cn';
import type { GetTraceResponse } from '../../../types/api';
import type { TraceNode } from '../../../types/domain';
import type { ChainVerification, ComplianceCheck } from '../types';
import { formatDateTime } from '../utils';

export interface ComplianceScanReportProps {
  trace: GetTraceResponse | null;
  verification: ChainVerification;
}

const CHECK_TONE: Record<
  ComplianceCheck['status'],
  { label: string; icon: typeof CheckCircle2; className: string }
> = {
  PASS: {
    label: '通过',
    icon: CheckCircle2,
    className: 'text-clear bg-clear/10 ring-clear/20',
  },
  WARN: {
    label: '关注',
    icon: AlertTriangle,
    className: 'text-amber-600 bg-amber-50 ring-amber-100',
  },
  FAIL: {
    label: '失败',
    icon: XCircle,
    className: 'text-boss bg-boss/10 ring-boss/20',
  },
};

function buildChecks(
  nodes: TraceNode[],
  verification: ChainVerification,
): ComplianceCheck[] {
  const modelCalls = nodes.filter((node) => node.type === 'MODEL_CALL');
  const validations = nodes.filter((node) => node.type === 'VALIDATION');
  const missingPayloadRef = nodes.filter(
    (node) => !node.evidence.payloadRef,
  ).length;
  const incompleteModelCalls = modelCalls.filter(
    (node) => !node.evidence.model || !Number.isFinite(node.evidence.cost),
  ).length;

  return [
    {
      key: 'HASH_CHAIN',
      label: '封印链连续性',
      status:
        verification.state === 'VALID'
          ? 'PASS'
          : verification.state === 'EMPTY'
            ? 'WARN'
            : 'FAIL',
      detail:
        verification.state === 'VALID'
          ? `${verification.nodeCount} 个战报节点 prevHash 与 headHash 全部匹配`
          : verification.state === 'EMPTY'
            ? '暂无战报节点，无法执行封印链校验'
            : `${verification.issues.length} 处封印破损，首个破损节点 ${verification.brokenNodeId ?? '—'}`,
    },
    {
      key: 'EVIDENCE_REF',
      label: '战报引用完整性',
      status: missingPayloadRef === 0 && nodes.length > 0 ? 'PASS' : 'WARN',
      detail:
        nodes.length === 0
          ? '暂无可检查战报节点'
          : missingPayloadRef === 0
            ? '全部战报节点均携带 payloadRef，证据可回指原始载荷'
            : `${missingPayloadRef} 个战报节点缺少 payloadRef，证据不可回指`,
    },
    {
      key: 'MODEL_COST',
      label: '模型调用可归集',
      status:
        modelCalls.length > 0 && incompleteModelCalls === 0 ? 'PASS' : 'WARN',
      detail:
        modelCalls.length === 0
          ? '未发现模型调用战报节点'
          : incompleteModelCalls === 0
            ? `${modelCalls.length} 次模型调用均记录模型标识与成本`
            : `${incompleteModelCalls} 次模型调用缺少模型标识或成本`,
    },
    {
      key: 'CROSS_CHECK',
      label: '平台连携验证',
      status: validations.length > 0 ? 'PASS' : 'WARN',
      detail:
        validations.length > 0
          ? `发现 ${validations.length} 个 VALIDATION 战报节点，平台复核已留痕`
          : '未发现 VALIDATION 战报节点，建议人工复核',
    },
  ];
}

export function ComplianceScanReport({
  trace,
  verification,
}: ComplianceScanReportProps) {
  const nodes = trace?.nodes ?? [];
  const checks = buildChecks(nodes, verification);
  const validations = nodes.filter((node) => node.type === 'VALIDATION');
  const latestValidation = validations[validations.length - 1] ?? null;

  return (
    <Card className="space-y-4 p-5">
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <ClipboardList
            className="h-4 w-4 text-indigo-500"
            aria-hidden="true"
          />
          <h2 className="text-sm font-semibold text-ink">副本规则报告</h2>
        </div>
        <p className="text-xs text-ink-muted">
          数据来源：战报节点本地推导 · 非平台规则引擎回执
        </p>
      </div>

      <ul className="space-y-2">
        {checks.map((check) => {
          const tone = CHECK_TONE[check.status];
          const Icon = tone.icon;
          return (
            <li
              key={check.key}
              className="rounded-lg border border-slate-100 bg-slate-50/60 px-3 py-2.5"
            >
              <div className="flex items-start gap-2">
                <Icon
                  className={cn(
                    'mt-0.5 h-4 w-4 shrink-0 rounded-full p-0.5 ring-1',
                    tone.className,
                  )}
                  aria-hidden="true"
                />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-xs font-semibold text-slate-700">
                      {check.label}
                    </p>
                    <span
                      className={cn(
                        'rounded-full px-2 py-0.5 text-[10px] font-medium ring-1 ring-inset',
                        tone.className,
                      )}
                    >
                      {tone.label}
                    </span>
                  </div>
                  <p className="mt-1 text-[11px] leading-5 text-slate-500">
                    {check.detail}
                  </p>
                </div>
              </div>
            </li>
          );
        })}
      </ul>

      <div className="rounded-lg border border-slate-200 bg-panel px-3 py-2.5">
        <p className="text-[11px] font-medium text-slate-400">
          最新平台复核结论
        </p>
        {latestValidation ? (
          <div className="mt-1 space-y-1">
            <p className="text-xs leading-5 text-slate-700">
              {latestValidation.summary}
            </p>
            <p className="text-[11px] text-slate-400">
              {latestValidation.actor.label} ·{' '}
              {formatDateTime(latestValidation.at)}
            </p>
          </div>
        ) : (
          <p className="mt-1 text-xs text-slate-500">
            暂无 VALIDATION 战报节点，报告仅基于本地推导。
          </p>
        )}
      </div>
    </Card>
  );
}
