import { ArrowRight, CheckCircle2, Clock3, XCircle } from 'lucide-react';
import { cn } from '../../../lib/cn';
import type { CrossValidation, ValidationVerdict } from '../../../types/domain';
import {
  VERDICT_CLASS,
  VERDICT_LABEL,
  VERDICT_STEP_CLASS,
} from '../labels';

export interface PipelineStatusProps {
  validation?: CrossValidation | null;
  compact?: boolean;
  className?: string;
}

const FALLBACK_STEP_CLASS = 'border-panel-line bg-panel-soft';
const FALLBACK_BADGE_CLASS =
  'bg-panel-soft text-ink-muted ring-1 ring-inset ring-panel-line';

function VerdictIcon({ verdict }: { verdict: ValidationVerdict }) {
  if (verdict === 'PASS') {
    return <CheckCircle2 className="h-3 w-3" />;
  }
  if (verdict === 'FAIL') {
    return <XCircle className="h-3 w-3" />;
  }
  return <Clock3 className="h-3 w-3" />;
}

function VerdictBadge({ verdict }: { verdict: ValidationVerdict }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold',
        VERDICT_CLASS[verdict] ?? FALLBACK_BADGE_CLASS,
      )}
    >
      <VerdictIcon verdict={verdict} />
      {VERDICT_LABEL[verdict] ?? verdict}
    </span>
  );
}

function normalizeVerdict(value: ValidationVerdict | undefined): ValidationVerdict {
  return value === 'PASS' || value === 'FAIL' || value === 'PENDING'
    ? value
    : 'PENDING';
}

/**
 * 出招 → 拆招 → 连携 三步关卡进度条，风格对齐副本战斗流水线。
 * validation 缺失时展示降级占位，不会抛错。
 */
export function PipelineStatus({
  validation,
  compact = false,
  className,
}: PipelineStatusProps) {
  if (!validation) {
    return (
      <div
        className={cn(
          'rounded-lg border border-dashed border-panel-line bg-panel-soft px-3 py-4 text-center text-xs text-ink-muted',
          className,
        )}
      >
        暂无连携验证流水线数据
      </div>
    );
  }

  const generatorVerdict = normalizeVerdict(validation.generatorVerdict);
  const reviewerVerdict = normalizeVerdict(validation.reviewerVerdict);
  const consensusVerdict = normalizeVerdict(validation.consensus);

  const confidence =
    typeof validation.confidence === 'number' && Number.isFinite(validation.confidence)
      ? validation.confidence <= 1
        ? Math.round(validation.confidence * 100)
        : Math.round(validation.confidence)
      : null;

  const steps = [
    {
      key: 'generate',
      title: '出招',
      actor: validation.generator || 'Claude 出招',
      model: validation.generatorModel || '模型未标注',
      verdict: generatorVerdict,
      detail: '生成 PoC / 交付物',
    },
    {
      key: 'review',
      title: '拆招',
      actor: validation.reviewer || 'Codex 拆招',
      model: validation.reviewerModel || '模型未标注',
      verdict: reviewerVerdict,
      detail: '复核 PoC / 独立反证',
    },
    {
      key: 'consensus',
      title: '连携',
      actor: '平台连携裁决',
      model: '双人连携汇总',
      verdict: consensusVerdict,
      detail:
        confidence !== null ? `置信度 ${confidence}%` : '置信度未记录',
    },
  ];

  return (
    <ol
      className={cn(
        'flex flex-col gap-2 md:flex-row md:items-stretch',
        compact ? 'gap-1.5' : 'gap-2',
        className,
      )}
    >
      {steps.map((step, index) => (
        <li
          key={step.key}
          className="flex flex-1 items-center gap-2 md:items-stretch"
        >
          <div
            className={cn(
              'flex-1 rounded-lg border',
              compact ? 'p-2.5' : 'p-3',
              VERDICT_STEP_CLASS[step.verdict] ?? FALLBACK_STEP_CLASS,
            )}
          >
            <div className="flex items-center justify-between gap-2">
              <span className="text-[11px] font-semibold uppercase tracking-wide text-ink-muted">
                {step.title}
              </span>
              <VerdictBadge verdict={step.verdict} />
            </div>
            <p className="mt-1 truncate text-sm font-medium text-ink">
              {step.actor}
            </p>
            <p className="truncate text-[11px] text-ink-muted">{step.model}</p>
            <p className="mt-0.5 truncate font-mono text-[10px] text-ink-dim">
              {step.detail}
            </p>
          </div>
          {index < steps.length - 1 && (
            <ArrowRight className="hidden h-4 w-4 shrink-0 self-center text-ink-dim md:block" />
          )}
        </li>
      ))}
    </ol>
  );
}
