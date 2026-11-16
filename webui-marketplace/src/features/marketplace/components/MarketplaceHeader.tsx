import { Bot, Briefcase } from 'lucide-react';
import { useSessionStore } from '../../../app/store/sessionStore';

const ROLE_LABEL = {
  EMPLOYER: '雇主视角',
  COMMANDER: 'AI 指挥官视角',
} as const;

const ROLE_HINT = {
  EMPLOYER: '可发布任务、冻结赏金、查看 AI 流水线与托管资金。',
  COMMANDER: '可接单、推进执行、提交交付并查看 AI 交叉复核结果。',
} as const;

export function MarketplaceHeader() {
  const activeRole = useSessionStore((state) => state.activeRole);
  const Icon = activeRole === 'EMPLOYER' ? Briefcase : Bot;

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div className="space-y-1">
        <p className="text-xs font-medium uppercase tracking-wide text-indigo-600">
          M2 · 任务撮合大厅与 AI 流水线
        </p>
        <h1 className="text-2xl font-semibold tracking-tight text-slate-900">
          任务撮合大厅
        </h1>
        <p className="max-w-2xl text-sm text-slate-500">
          AI 指挥官接单交付，平台审核 AI 独立交叉复核，资金全程托管结算。
          右上角可随时切换视角。
        </p>
      </div>
      <div className="flex max-w-xs shrink-0 items-start gap-2 rounded-xl border border-slate-200 bg-white px-3 py-2">
        <span className="rounded-lg bg-indigo-50 p-1.5 text-indigo-600">
          <Icon className="h-4 w-4" />
        </span>
        <div className="min-w-0">
          <p className="text-xs text-slate-400">当前视角</p>
          <p className="text-sm font-semibold text-slate-900">
            {ROLE_LABEL[activeRole]}
          </p>
          <p className="mt-0.5 text-xs leading-4 text-slate-500">
            {ROLE_HINT[activeRole]}
          </p>
        </div>
      </div>
    </div>
  );
}
