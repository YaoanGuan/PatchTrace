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
  'bg-panel-soft text-ink-muted ring-1 ring-inset ring-panel-line';

const CONSENSUS_LABEL: Record<ValidationVerdict, string> = {
  PASS: '连携成功',
  FAIL: '连携失败',
  PENDING: '连携待裁决',
};

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
  labels,
}: {
  label: string;
  verdict: ValidationVerdict | undefined;
  labels?: Record<ValidationVerdict, string>;
}) {
  const safeVerdict: ValidationVerdict =
    verdict === 'PASS' || verdict === 'FAIL' || verdict === 'PENDING'
      ? verdict
      : 'PENDING';
  return (
    <div className="rounded-lg bg-panel-soft px-3 py-2 ring-1 ring-inset ring-panel-line">
      <p className="text-[11px] text-ink-muted">{label}</p>
      <span
        className={cn(
          'mt-1 inline-flex rounded-full px-2 py-0.5 text-[11px] font-semibold',
          VERDICT_CLASS[safeVerdict] ?? VERDICT_FALLBACK_CLASS,
        )}
      >
        {(labels ?? VERDICT_LABEL)[safeVerdict] ?? safeVerdict}
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
 * 双人连携验证面板：Claude 出招（生成 PoC）→ Codex 拆招（复核）→ 平台连携裁决。
 * 包含判定、连携、置信度、成本、耗时与时间。
 */
export function CrossValidationPanel({
  validation,
  className,
}: CrossValidationPanelProps) {
  if (!validation) {
    return (
      <EmptyState
        className={className}
        title="暂无连携验证结果"
        description="任务进入双人连携阶段后，将展示出招、拆招与连携三步判定。"
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
          <h2 className="flex items-center gap-1.5 text-sm font-semibold text-ink">
            <GitCompareArrows className="h-4 w-4 text-arcane" />
            双人连携验证
          </h2>
          <p className="mt-0.5 text-xs text-ink-muted">
            Claude 出招（生成 PoC）· Codex 拆招（复核）· 平台连携裁决：任一环节未通关都不会直接放款。
          </p>
        </div>
        <span className="rounded-full bg-panel-soft px-2 py-0.5 font-mono text-[10px] text-ink-muted ring-1 ring-inset ring-panel-line">
          {validation.taskId || '任务未标注'}
        </span>
      </div>

      <div className="mt-3">
        <PipelineStatus validation={validation} />
      </div>

      <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
        <VerdictMetric label="出招判定" verdict={validation.generatorVerdict} />
        <VerdictMetric label="拆招判定" verdict={validation.reviewerVerdict} />
        <VerdictMetric
          label="连携"
          verdict={validation.consensus}
          labels={CONSENSUS_LABEL}
        />
        <div className="rounded-lg bg-panel-soft px-3 py-2 ring-1 ring-inset ring-panel-line">
          <p className="text-[11px] text-ink-muted">置信度</p>
          <p className="mt-1 font-mono text-sm font-semibold text-arcane">
            {confidenceText(validation.confidence)}
          </p>
        </div>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] text-ink-muted">
        <span>
          成本{' '}
          <span className="font-mono font-medium text-ink">
            {formatCost(validation.cost)}
          </span>
        </span>
        <span>
          耗时{' '}
          <span className="font-mono font-medium text-ink">
            {formatElapsed(validation.elapsedMs)}
          </span>
        </span>
        <span>
          记录时间{' '}
          <span className="font-mono font-medium text-ink">
            {formatAt(validation.at)}
          </span>
        </span>
      </div>

      {pass && (
        <p className="mt-3 rounded-lg bg-clear/10 px-3 py-2 text-xs text-clear ring-1 ring-inset ring-clear/30">
          连携成功：出招与拆招结论一致，进入副本规则扫描。
        </p>
      )}
      {fail && (
        <p className="mt-3 rounded-lg bg-boss/10 px-3 py-2 text-xs text-boss ring-1 ring-inset ring-boss/30">
          连携失败：任务将被退回或触发关卡封锁，不进入结算。
        </p>
      )}
      {!pass && !fail && (
        <p className="mt-3 rounded-lg bg-panel-soft px-3 py-2 text-xs text-ink-muted ring-1 ring-inset ring-panel-line">
          连携进行中：等待出招与拆招双方给出判定。
        </p>
      )}
    </Card>
  );
}
