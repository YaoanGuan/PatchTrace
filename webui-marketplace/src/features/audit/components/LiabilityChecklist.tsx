import type { LucideIcon } from 'lucide-react';
import { Building2, Shield, UserCog } from 'lucide-react';
import { cn } from '../../../lib/cn';
import type { LiabilityClause, LiabilityParty } from '../types';

export interface LiabilityChecklistProps {
  clauses: LiabilityClause[];
  accepted: boolean[];
  disabled?: boolean;
  onToggle(index: number): void;
}

const PARTY_META: Record<
  LiabilityParty,
  {
    label: string;
    description: string;
    icon: LucideIcon;
    accent: string;
    iconTone: string;
  }
> = {
  EMPLOYER: {
    label: '雇主',
    description: '任务发起方 · 对目标与授权边界负责',
    icon: Building2,
    accent: 'border-indigo-100 bg-indigo-50/40',
    iconTone: 'bg-indigo-100 text-indigo-600',
  },
  COMMANDER: {
    label: 'AI 指挥官',
    description: '任务执行方 · 对 Agent 编排与战报真实性负责',
    icon: UserCog,
    accent: 'border-violet-100 bg-violet-50/40',
    iconTone: 'bg-violet-100 text-violet-600',
  },
  PLATFORM: {
    label: '平台',
    description: '撮合与仲裁方 · 对托管、封印审计与争议流程负责',
    icon: Shield,
    accent: 'border-teal-100 bg-teal-50/40',
    iconTone: 'bg-teal-100 text-teal-600',
  },
};

const PARTY_ORDER: LiabilityParty[] = ['EMPLOYER', 'COMMANDER', 'PLATFORM'];

export function LiabilityChecklist({
  clauses,
  accepted,
  disabled = false,
  onToggle,
}: LiabilityChecklistProps) {
  const acceptedCount = accepted.filter(Boolean).length;

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-ink">三方责任边界</h3>
          <p className="text-xs text-ink-muted">
            请逐条阅读并勾选，全部勾选后方可生成签名。
          </p>
        </div>
        <span
          className={cn(
            'rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset',
            acceptedCount === clauses.length
              ? 'bg-emerald-50 text-emerald-700 ring-emerald-600/20'
              : 'bg-amber-50 text-amber-700 ring-amber-600/20',
          )}
        >
          已勾选 {acceptedCount}/{clauses.length}
        </span>
      </div>

      {PARTY_ORDER.map((party) => {
        const meta = PARTY_META[party];
        const Icon = meta.icon;
        const partyClauses = clauses
          .map((clause, index) => ({ clause, index }))
          .filter((item) => item.clause.party === party);

        return (
          <section
            key={party}
            className={cn('rounded-lg border px-3 py-3', meta.accent)}
          >
            <div className="flex items-center gap-2">
              <span
                className={cn(
                  'flex h-7 w-7 items-center justify-center rounded-full',
                  meta.iconTone,
                )}
                aria-hidden="true"
              >
                <Icon className="h-3.5 w-3.5" />
              </span>
              <div>
                <p className="text-xs font-semibold text-slate-800">
                  {meta.label}
                </p>
                <p className="text-[11px] text-slate-500">{meta.description}</p>
              </div>
            </div>

            <ul className="mt-3 space-y-2">
              {partyClauses.map(({ clause, index }) => (
                <li key={clause.id}>
                  <label
                    className={cn(
                      'flex cursor-pointer items-start gap-2.5 rounded-md border bg-panel px-3 py-2.5 transition-colors',
                      accepted[index]
                        ? 'border-emerald-200 ring-1 ring-emerald-100'
                        : 'border-slate-200 hover:border-slate-300',
                      disabled && 'cursor-not-allowed opacity-70',
                    )}
                  >
                    <input
                      type="checkbox"
                      checked={accepted[index] ?? false}
                      disabled={disabled}
                      onChange={() => onToggle(index)}
                      className="mt-0.5 h-4 w-4 shrink-0 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
                    />
                    <span className="min-w-0">
                      <span className="block text-xs font-medium text-slate-700">
                        {clause.title}
                      </span>
                      <span className="mt-0.5 block text-[11px] leading-5 text-slate-500">
                        {clause.detail}
                      </span>
                    </span>
                  </label>
                </li>
              ))}
            </ul>
          </section>
        );
      })}
    </div>
  );
}
