import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import {
  Bot,
  ClipboardCheck,
  FileText,
  Link2Off,
  PackageCheck,
  ShieldCheck,
  Terminal,
} from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { cn } from '../../../lib/cn';
import type {
  TraceActorKind,
  TraceNode as TraceNodeModel,
  TraceNodeType,
} from '../../../types/domain';
import { formatCost, formatDateTime } from '../utils';
import { HashText } from './HashText';

export interface TraceNodeProps {
  node: TraceNodeModel;
  index: number;
  isLast: boolean;
  broken: boolean;
}

interface NodeTypeMeta {
  label: string;
  description: string;
  icon: LucideIcon;
  bubble: string;
}

export const TRACE_NODE_META: Record<TraceNodeType, NodeTypeMeta> = {
  REQUIREMENT: {
    label: '需求发布',
    description: '任务目标与授权边界',
    icon: FileText,
    bubble: 'bg-indigo-50 text-indigo-600 ring-indigo-100',
  },
  ACCEPTANCE: {
    label: '验收标准',
    description: '固化验收口径',
    icon: ClipboardCheck,
    bubble: 'bg-sky-50 text-sky-600 ring-sky-100',
  },
  MODEL_CALL: {
    label: '模型调用',
    description: '模型、成本与耗时',
    icon: Bot,
    bubble: 'bg-violet-50 text-violet-600 ring-violet-100',
  },
  ACTION: {
    label: '执行动作',
    description: '沙箱内的具体操作',
    icon: Terminal,
    bubble: 'bg-amber-50 text-amber-600 ring-amber-100',
  },
  OUTPUT: {
    label: '产物输出',
    description: '可复现产物与日志',
    icon: PackageCheck,
    bubble: 'bg-emerald-50 text-emerald-600 ring-emerald-100',
  },
  VALIDATION: {
    label: '复核验证',
    description: '平台交叉复核结论',
    icon: ShieldCheck,
    bubble: 'bg-teal-50 text-teal-600 ring-teal-100',
  },
};

const ACTOR_KIND_LABEL: Record<TraceActorKind, string> = {
  USER: '用户',
  AGENT: 'Agent',
  SYSTEM: '系统',
};

export function TraceNode({
  node,
  index,
  isLast,
  broken,
}: TraceNodeProps) {
  const meta = TRACE_NODE_META[node.type];
  const Icon = meta.icon;

  return (
    <li className="relative flex gap-4 pb-5 last:pb-0">
      <div className="flex flex-col items-center">
        <span
          className={cn(
            'flex h-9 w-9 shrink-0 items-center justify-center rounded-full ring-4 ring-white',
            meta.bubble,
          )}
          aria-hidden="true"
        >
          <Icon className="h-4 w-4" />
        </span>
        {!isLast && <span className="mt-1 w-px flex-1 bg-slate-200" />}
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs font-semibold text-slate-400">
            #{String(index + 1).padStart(2, '0')}
          </span>
          <span className="text-sm font-semibold text-slate-900">
            {meta.label}
          </span>
          <span className="text-xs text-slate-400">{meta.description}</span>
          {broken ? (
            <Badge variant="danger">
              <Link2Off className="h-3 w-3" aria-hidden="true" />
              断链
            </Badge>
          ) : (
            <Badge variant="success">链校验通过</Badge>
          )}
          <span className="ml-auto text-xs text-slate-400">
            {formatDateTime(node.at)}
          </span>
        </div>

        <div
          className={cn(
            'mt-2 rounded-lg border bg-white p-3',
            broken
              ? 'border-rose-200 ring-1 ring-rose-100'
              : 'border-slate-200',
          )}
        >
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
            <span className="font-medium text-slate-700">
              {node.actor.label}
            </span>
            <Badge variant="outline">{ACTOR_KIND_LABEL[node.actor.kind]}</Badge>
            <code
              className="max-w-full truncate font-mono text-[11px] text-slate-400"
              title={node.actor.id}
            >
              {node.actor.id}
            </code>
          </div>

          <p className="mt-2 text-sm leading-6 text-slate-700">{node.summary}</p>

          <dl className="mt-3 grid gap-2 sm:grid-cols-2">
            <EvidenceItem label="证据哈希">
              <HashText hash={node.evidence.hash} />
            </EvidenceItem>
            <EvidenceItem label="载荷引用">
              {node.evidence.payloadRef ? (
                <code
                  className="break-all font-mono text-[11px] text-slate-500"
                  title={node.evidence.payloadRef}
                >
                  {node.evidence.payloadRef}
                </code>
              ) : (
                <span className="text-xs text-slate-400">—</span>
              )}
            </EvidenceItem>
            <EvidenceItem label="模型">
              <span className="font-mono text-[11px] text-slate-500">
                {node.evidence.model ?? '—'}
              </span>
            </EvidenceItem>
            <EvidenceItem label="成本">
              <span className="font-mono text-[11px] text-slate-500">
                {formatCost(node.evidence.cost)}
              </span>
            </EvidenceItem>
          </dl>

          <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-slate-100 pt-2 text-[11px] text-slate-400">
            <span>prevHash</span>
            <HashText hash={node.prevHash ?? ''} />
            <span>→</span>
            <span>当前</span>
            <HashText hash={node.hash} />
            {node.prevHash === null && (
              <span className="text-slate-400">（链首节点）</span>
            )}
          </div>
        </div>
      </div>
    </li>
  );
}

function EvidenceItem({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="rounded-md bg-slate-50 px-2.5 py-2">
      <dt className="text-[11px] font-medium text-slate-400">{label}</dt>
      <dd className="mt-1 min-w-0">{children}</dd>
    </div>
  );
}
