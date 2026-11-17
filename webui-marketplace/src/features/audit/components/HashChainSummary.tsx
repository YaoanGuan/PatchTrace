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
          label: '存在封印破损',
          variant: 'danger' as const,
          description: `${invalid} 条副本的封印链校验未通过，请进入详情定位破损节点。`,
        }
      : valid > 0
        ? {
            label: '封印完好',
            variant: 'success' as const,
            description:
              unverified > 0
                ? `已获取战报的 ${valid} 条副本封印全部完好，另有 ${unverified} 条副本暂无战报。`
                : '当前列表全部副本的 prevHash 与 headHash 封印校验均通过。',
          }
        : {
            label: '暂无封印记录',
            variant: 'outline' as const,
            description: '当前列表中的副本均未提供战报节点。',
          };

  return (
    <Card className="space-y-4 p-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-indigo-500" aria-hidden="true" />
            <h2 className="text-sm font-semibold text-ink">封印链概览</h2>
          </div>
          <p className="text-xs text-ink-muted">
            对当前列表逐条调用战报接口，执行 prevHash → 上一节点 hash 的封印现算校验。
          </p>
        </div>
        <Badge variant={overall.variant}>{overall.label}</Badge>
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <SummaryStat
          label="封印完好"
          value={valid}
          suffix="条"
          tone="text-clear"
          icon={<ShieldCheck className="h-4 w-4" aria-hidden="true" />}
        />
        <SummaryStat
          label="封印破损"
          value={invalid}
          suffix="条"
          tone="text-boss"
          icon={<ShieldAlert className="h-4 w-4" aria-hidden="true" />}
        />
        <SummaryStat
          label="暂无战报"
          value={unverified}
          suffix="条"
          tone="text-ink-dim"
          icon={<ShieldOff className="h-4 w-4" aria-hidden="true" />}
        />
        <SummaryStat
          label="战报节点总数"
          value={nodeTotal}
          suffix="个"
          tone="text-neon"
          icon={<Activity className="h-4 w-4" aria-hidden="true" />}
        />
      </div>

      <Progress
        value={checked}
        max={Math.max(total, 1)}
        label={`封印校验覆盖率（${checked}/${total} 条副本）`}
        barClassName={invalid > 0 ? 'bg-boss' : 'bg-clear'}
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
      <p className="mt-1 font-mono text-xl font-semibold tracking-tight text-ink">
        {value}
        <span className="ml-1 text-xs font-normal text-slate-400">{suffix}</span>
      </p>
    </div>
  );
}
