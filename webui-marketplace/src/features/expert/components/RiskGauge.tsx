import { AlertTriangle, Gauge, ShieldCheck } from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import { EmptyState } from '../../../components/ui/EmptyState';
import { cn } from '../../../lib/cn';
import type { RiskAssessment, RiskLevel } from '../../../types/domain';
import {
  RISK_LEVEL_BADGE_CLASS,
  RISK_LEVEL_LABEL,
  RISK_LEVEL_ORDER,
  RISK_LEVEL_SEGMENT_CLASS,
  RISK_LEVEL_TEXT_CLASS,
  clampScore,
  deriveRiskLevel,
} from '../labels';

export interface RiskGaugeProps {
  risk?: RiskAssessment | null;
  compact?: boolean;
  className?: string;
}

function resolveLevel(level: RiskLevel | undefined, score: number): RiskLevel {
  return level && RISK_LEVEL_ORDER.includes(level)
    ? level
    : deriveRiskLevel(score);
}

/**
 * 风险定级仪表：0–100 分、四级配色、阈值线、阻断状态与触发原因。
 * 使用分段条绘制，避免 canvas / 图表依赖。
 */
export function RiskGauge({ risk, compact = false, className }: RiskGaugeProps) {
  if (!risk) {
    return (
      <EmptyState
        className={className}
        title="暂无风险定级数据"
        description="任务通过交叉验证与合规扫描后，会在这里生成风险评分与阻断结论。"
      />
    );
  }

  const score = clampScore(risk.score);
  const threshold = clampScore(risk.threshold);
  const level = resolveLevel(risk.level, score);
  const blocked = Boolean(risk.blocked);
  const reasons = risk.reasons ?? [];

  return (
    <Card className={cn('p-4', className)}>
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h2 className="flex items-center gap-1.5 text-sm font-semibold text-slate-900">
            <Gauge className="h-4 w-4 text-indigo-600" />
            风险定级与阻断
          </h2>
          <p className="mt-0.5 text-xs text-slate-500">
            综合交叉验证与合规命中，超过阈值即触发阻断。
          </p>
        </div>
        <span
          className={cn(
            'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold',
            blocked
              ? 'bg-rose-50 text-rose-700 ring-1 ring-inset ring-rose-600/20'
              : 'bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-600/20',
          )}
        >
          {blocked ? (
            <AlertTriangle className="h-3 w-3" />
          ) : (
            <ShieldCheck className="h-3 w-3" />
          )}
          {blocked ? '已阻断' : '未阻断'}
        </span>
      </div>

      <div className="mt-3 flex items-end justify-between gap-3">
        <div className="flex items-baseline gap-1">
          <span
            className={cn(
              'font-bold tabular-nums',
              compact ? 'text-3xl' : 'text-4xl',
              RISK_LEVEL_TEXT_CLASS[level],
            )}
          >
            {score}
          </span>
          <span className="text-xs text-slate-400">/ 100 分</span>
        </div>
        <span
          className={cn(
            'rounded-full px-2 py-0.5 text-[11px] font-semibold',
            RISK_LEVEL_BADGE_CLASS[level],
          )}
        >
          {RISK_LEVEL_LABEL[level]}
        </span>
      </div>

      <div className="mt-3">
        <div className="mb-1 flex justify-between text-[10px] text-slate-400">
          <span>0</span>
          <span>25</span>
          <span>50</span>
          <span>75</span>
          <span>100</span>
        </div>
        <div className="relative h-5">
          <div className="flex h-3 w-full overflow-hidden rounded-full">
            {RISK_LEVEL_ORDER.map((segment) => (
              <div
                key={segment}
                className={cn('h-full w-1/4', RISK_LEVEL_SEGMENT_CLASS[segment])}
                title={RISK_LEVEL_LABEL[segment]}
              />
            ))}
          </div>
          {/* 阈值线 */}
          <div
            className="absolute top-0 h-3 w-0.5 -translate-x-1/2 bg-slate-900/70"
            style={{ left: `${threshold}%` }}
            title={`风险阈值 ${threshold} 分`}
          />
          {/* 当前分数指针 */}
          <div
            className="absolute top-0 h-5 w-1.5 -translate-x-1/2 rounded-sm border border-white bg-slate-900 shadow"
            style={{ left: `${score}%` }}
            title={`当前风险 ${score} 分`}
          />
        </div>
        <div className="mt-1 flex flex-wrap items-center justify-between gap-2 text-[10px] text-slate-500">
          <div className="flex flex-wrap items-center gap-2">
            {RISK_LEVEL_ORDER.map((segment) => (
              <span key={segment} className="inline-flex items-center gap-1">
                <span
                  className={cn(
                    'h-1.5 w-1.5 rounded-full',
                    RISK_LEVEL_SEGMENT_CLASS[segment],
                  )}
                />
                {RISK_LEVEL_LABEL[segment]}
              </span>
            ))}
          </div>
          <span className="inline-flex items-center gap-1">
            <span className="h-3 w-0.5 bg-slate-900/70" />
            阈值 {threshold} 分
          </span>
        </div>
      </div>

      <div className="mt-3 border-t border-slate-100 pt-3">
        <p className="mb-1.5 text-[11px] font-semibold text-slate-500">
          触发原因
        </p>
        {reasons.length === 0 ? (
          <p className="text-xs text-slate-400">当前未触发风险规则。</p>
        ) : (
          <ul className="space-y-1">
            {reasons.map((reason, index) => (
              <li
                key={`${reason}-${index}`}
                className="flex items-start gap-1.5 text-xs leading-5 text-slate-600"
              >
                <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0 text-amber-500" />
                <span>{reason || '未命名风险原因'}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Card>
  );
}
