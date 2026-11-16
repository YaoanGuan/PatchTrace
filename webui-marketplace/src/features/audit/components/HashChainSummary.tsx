import type { ReactNode } from 'react';
import { Activity, ShieldAlert, ShieldCheck, ShieldOff } from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { Card } from '../../../components/ui/Card';
import { Progress } from '../../../components/ui/Progress';
import type { AuditTaskEntry } from '../types';

export interface HashChainSummaryProps {
  entries: AuditTaskEntry[];
}

export function HashChainSummary({ entries }: HashChainSummaryProps) {
  const total = entries.length;
  const valid = entries.filter((entry) => entry.chain.state === 'VALID').length;
  const invalid = entries.filter((entry) => entry.chain.state === 'INVALID').length;
  const unverified = total - valid - invalid;
  const nodeTotal = entries.reduce((sum, entry) => sum + entry.chain.nodeCount, 0);
  const checked = valid + invalid;

  const overall =
    invalid > 0
      ? {
          label: '存在异常链路',
          variant: 'danger' as const,
          description: `${invalid} 条任务的哈希链校验未通过，请进入详情定位异常节点。`,
        }
      : valid > 0
        ? {
            label: '已校验链路完整',
            variant: 'success' as const,
            description:
              unverified > 0
                ? `已获取证据链的 ${valid} 条任务校验全部通过，另有 ${unverified} 条任务暂无证据链。`
                : '当前列表全部任务的 prevHash 与 headHash 校验均通过。',
          }
        : {
            label: '暂无已校验链路',
            variant: 'outline' as const,
            description: '当前列表中的任务均未提供证据链节点。',
          };

  return (
    <Card className="space-y-4 p-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-indigo-500" aria-hidden="true" />
            <h2 className="text-sm font-semibold text-slate-900">哈希链概览</h2>
          </div>
          <p className="text-xs text-slate-500">
            对当前列表逐条调用证据链接口，执行 prevHash → 上一节点 hash 的现算校验。
          </p>
        </div>
        <Badge variant={overall.variant}>{overall.label}</Badge>
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <SummaryStat
          label="已校验"
          value={valid}
          suffix="条"
          tone="text-emerald-600"
          icon={<ShieldCheck className="h-4 w-4" aria-hidden="true" />}
        />
        <SummaryStat
          label="异常"
          value={invalid}
          suffix="条"
          tone="text-rose-600"
          icon={<ShieldAlert className="h-4 w-4" aria-hidden="true" />}
        />
        <SummaryStat
          label="未校验"
          value={unverified}
          suffix="条"
          tone="text-slate-500"
          icon={<ShieldOff className="h-4 w-4" aria-hidden="true" />}
        />
        <SummaryStat
          label="节点总数"
          value={nodeTotal}
          suffix="个"
          tone="text-indigo-600"
          icon={<Activity className="h-4 w-4" aria-hidden="true" />}
        />
      </div>

      <Progress
        value={checked}
        max={Math.max(total, 1)}
        label={`校验覆盖率（${checked}/${total} 条任务）`}
        barClassName={invalid > 0 ? 'bg-rose-500' : 'bg-emerald-500'}
      />

      <p className="text-xs text-slate-400">{overall.description}</p>
    </Card>
  );
}

interface SummaryStatProps {
  label: string;
  value: number;
  suffix: string;
  tone: string;
  icon: ReactNode;
}

function SummaryStat({ label, value, suffix, tone, icon }: SummaryStatProps) {
  return (
    <div className="rounded-lg border border-slate-100 bg-slate-50/70 px-3 py-3">
      <div className={`flex items-center gap-1.5 text-xs ${tone}`}>
        {icon}
        {label}
      </div>
      <p className="mt-1 text-xl font-semibold tracking-tight text-slate-900">
        {value}
        <span className="ml-1 text-xs font-normal text-slate-400">{suffix}</span>
      </p>
    </div>
  );
}
