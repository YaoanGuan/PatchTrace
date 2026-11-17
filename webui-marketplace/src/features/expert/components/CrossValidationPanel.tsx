import { GitCompareArrows } from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import { EmptyState } from '../../../components/ui/EmptyState';
import { cn } from '../../../lib/cn';
import type { CrossValidation, ValidationVerdict } from '../../../types/domain';
import { VERDICT_CLASS, VERDICT_LABEL } from '../labels';
import { PipelineStatus } from './PipelineStatus';

export interface CrossValidationPanelProps {
  validation?: CrossValidation | null;
  className?: string;
}

const VERDICT_FALLBACK_CLASS =
  'bg-slate-100 text-slate-600 ring-1 ring-inset ring-slate-500/20';

function formatCost(cost: number | undefined): string {
  if (typeof cost !== 'number' || !Number.isFinite(cost)) {
    return '未记录';
  }
  return `¥${cost.toFixed(2)}`;
}

function formatElapsed(elapsedMs: number | undefined): string {
  if (typeof elapsedMs !== 'number' || !Number.isFinite(elapsedMs)) {
    return '未记录';
  }
  return elapsedMs >= 1000
    ? `${(elapsedMs / 1000).toFixed(1)} 秒`
    : `${Math.round(elapsedMs)} 毫秒`;
}

function formatAt(at: string | undefined): string {
  if (!at) {
    return '时间未记录';
  }
  const date = new Date(at);
  if (Number.isNaN(date.getTime())) {
    return at;
  }
  return date.toLocaleString('zh-CN', { hour12: false });
}

function VerdictMetric({
  label,
  verdict,
}: {
  label: string;
  verdict: ValidationVerdict | undefined;
}) {
  const safeVerdict: ValidationVerdict =
    verdict === 'PASS' || verdict === 'FAIL' || verdict === 'PENDING'
      ? verdict
      : 'PENDING';
  return (
    <div className="rounded-lg bg-slate-50 px-3 py-2">
      <p className="text-[11px] text-slate-500">{label}</p>
      <span
        className={cn(
          'mt-1 inline-flex rounded-full px-2 py-0.5 text-[11px] font-semibold',
          VERDICT_CLASS[safeVerdict] ?? VERDICT_FALLBACK_CLASS,
        )}
      >
        {VERDICT_LABEL[safeVerdict] ?? safeVerdict}
      </span>
    </div>
  );
}

function confidenceText(confidence: number | undefined): string {
  if (typeof confidence !== 'number' || !Number.isFinite(confidence)) {
    return '未记录';
  }
  const percentage = confidence <= 1 ? confidence * 100 : confidence;
  return `${Math.round(percentage)}%`;
}

/**
 * 多 Agent 交叉验证面板：Claude 生成 → Codex 复核 → 平台共识。
 * 包含判定、共识、置信度、成本、耗时与时间。
 */
export function CrossValidationPanel({
  validation,
  className,
}: CrossValidationPanelProps) {
  if (!validation) {
    return (
      <EmptyState
        className={className}
        title="暂无交叉验证结果"
        description="任务进入 AI 验证阶段后，将展示生成、复核与共识三步判定。"
      />
    );
  }

  const consensus = validation.consensus;
  const pass = consensus === 'PASS';
  const fail = consensus === 'FAIL';

  return (
    <Card className={cn('p-4', className)}>
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h2 className="flex items-center gap-1.5 text-sm font-semibold text-slate-900">
            <GitCompareArrows className="h-4 w-4 text-indigo-600" />
            多 Agent 交叉验证
          </h2>
          <p className="mt-0.5 text-xs text-slate-500">
            Claude 生成 · Codex 复核 · 平台共识：任一环节未通过都不会直接放款。
          </p>
        </div>
        <span className="rounded-full bg-slate-100 px-2 py-0.5 font-mono text-[10px] text-slate-500">
          {validation.taskId || '任务未标注'}
        </span>
      </div>

      <div className="mt-3">
        <PipelineStatus validation={validation} />
      </div>

      <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
        <VerdictMetric label="生成判定" verdict={validation.generatorVerdict} />
        <VerdictMetric label="复核判定" verdict={validation.reviewerVerdict} />
        <VerdictMetric label="共识" verdict={validation.consensus} />
        <div className="rounded-lg bg-slate-50 px-3 py-2">
          <p className="text-[11px] text-slate-500">置信度</p>
          <p className="mt-1 text-sm font-semibold text-slate-800">
            {confidenceText(validation.confidence)}
          </p>
        </div>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] text-slate-500">
        <span>
          成本 <span className="font-medium text-slate-700">{formatCost(validation.cost)}</span>
        </span>
        <span>
          耗时{' '}
          <span className="font-medium text-slate-700">
            {formatElapsed(validation.elapsedMs)}
          </span>
        </span>
        <span>
          记录时间{' '}
          <span className="font-medium text-slate-700">
            {formatAt(validation.at)}
          </span>
        </span>
      </div>

      {pass && (
        <p className="mt-3 rounded-lg bg-emerald-50 px-3 py-2 text-xs text-emerald-700">
          交叉验证通过：生成与复核结论一致，进入合规扫描。
        </p>
      )}
      {fail && (
        <p className="mt-3 rounded-lg bg-rose-50 px-3 py-2 text-xs text-rose-700">
          交叉验证未通过：任务将被退回或阻断，不进入结算。
        </p>
      )}
      {!pass && !fail && (
        <p className="mt-3 rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-600">
          交叉验证进行中：等待生成与复核双方给出判定。
        </p>
      )}
    </Card>
  );
}
