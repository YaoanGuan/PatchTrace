import type { ReactNode } from 'react';
import { AlertTriangle, CheckCircle2, Link2, ShieldCheck } from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import { cn } from '../../../lib/cn';
import type { GetTraceResponse } from '../../../types/api';
import type { ChainVerification } from '../types';
import { ChainStateBadge } from './ChainStateBadge';
import { HashText } from './HashText';

export interface HashChainStatusProps {
  trace: GetTraceResponse | null;
  verification: ChainVerification;
}

export function HashChainStatus({ trace, verification }: HashChainStatusProps) {
  const nodes = trace?.nodes ?? [];
  const brokenIndexes = new Set(
    verification.issues.map((issue) => issue.index),
  );
  const isComplete = verification.state === 'VALID';

  return (
    <Card className="space-y-4 p-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <ShieldCheck className="h-4 w-4 text-indigo-500" aria-hidden="true" />
            <h2 className="text-sm font-semibold text-slate-900">哈希链校验</h2>
          </div>
          <p className="text-xs text-slate-500">
            逐节点校验 prevHash 是否等于上一节点 hash，并比对 headHash。
          </p>
        </div>
        <ChainStateBadge state={verification.state} />
      </div>

      <div className="grid gap-2 sm:grid-cols-3">
        <MetaItem label="节点数量">{verification.nodeCount} 个</MetaItem>
        <MetaItem label="校验方式">
          <span className="inline-flex items-center gap-1 whitespace-nowrap font-mono text-[11px] text-slate-500">
            <Link2 className="h-3 w-3 shrink-0" aria-hidden="true" />
            prevHash 链
          </span>
        </MetaItem>
        <MetaItem label="链头哈希">
          {verification.headHash ? (
            <HashText hash={verification.headHash} />
          ) : (
            <span className="text-xs text-slate-400">—</span>
          )}
        </MetaItem>
      </div>

      {verification.state === 'EMPTY' ? (
        <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-3 text-xs text-slate-500">
          {verification.reason ?? '暂无证据链节点，无法执行哈希链校验。'}
        </div>
      ) : isComplete ? (
        <div className="flex items-start gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-3 text-xs text-emerald-700">
          <CheckCircle2
            className="mt-0.5 h-4 w-4 shrink-0"
            aria-hidden="true"
          />
          <div>
            <p className="font-semibold">链完整</p>
            <p className="mt-0.5 text-emerald-600/90">
              全部 {verification.nodeCount} 个节点链接连续，headHash 与末节点 hash 一致。
            </p>
          </div>
        </div>
      ) : (
        <div className="space-y-2 rounded-lg border border-rose-200 bg-rose-50 px-3 py-3 text-xs text-rose-700">
          <div className="flex items-start gap-2">
            <AlertTriangle
              className="mt-0.5 h-4 w-4 shrink-0"
              aria-hidden="true"
            />
            <div>
              <p className="font-semibold">
                检测到 {verification.issues.length} 处链异常
              </p>
              <p className="mt-0.5 text-rose-600/90">
                首个异常节点：{verification.brokenNodeId ?? '—'}
              </p>
            </div>
          </div>
          <ul className="list-disc space-y-1 pl-8">
            {verification.issues.map((issue) => (
              <li key={`${issue.nodeId}-${issue.reason}`}>
                节点 #{issue.index + 1}（{issue.nodeId}）：{issue.reason}
              </li>
            ))}
          </ul>
        </div>
      )}

      {nodes.length > 0 && (
        <div className="space-y-1.5">
          <p className="text-xs font-medium text-slate-500">节点校验明细</p>
          <ul className="divide-y divide-slate-100 overflow-hidden rounded-lg border border-slate-100">
            {nodes.map((node, index) => {
              const broken = brokenIndexes.has(index);
              return (
                <li
                  key={node.id}
                  className={cn(
                    'flex items-center justify-between gap-2 px-3 py-2 text-xs',
                    broken ? 'bg-rose-50/60' : 'bg-white',
                  )}
                >
                  <span className="inline-flex min-w-0 items-center gap-2">
                    <span className="font-mono text-[11px] text-slate-400">
                      #{String(index + 1).padStart(2, '0')}
                    </span>
                    <span className="truncate font-mono text-[11px] text-slate-600">
                      {node.id}
                    </span>
                  </span>
                  <span className="inline-flex shrink-0 items-center gap-2">
                    <HashText hash={node.hash} />
                    {broken ? (
                      <span className="text-[11px] font-medium text-rose-600">
                        异常
                      </span>
                    ) : (
                      <CheckCircle2
                        className="h-3.5 w-3.5 text-emerald-500"
                        aria-label="链路通过"
                      />
                    )}
                  </span>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </Card>
  );
}

function MetaItem({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="rounded-lg border border-slate-100 bg-slate-50/70 px-3 py-2">
      <p className="text-[11px] font-medium text-slate-400">{label}</p>
      <div className="mt-1 text-xs text-slate-600">{children}</div>
    </div>
  );
}
