import { Link } from 'react-router-dom';
import { ChevronRight, Clock3, GitBranch } from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { Card } from '../../../components/ui/Card';
import { cn } from '../../../lib/cn';
import type { TaskSummary } from '../../../types/domain';
import type { ChainVerification } from '../types';
import { formatDateTime, TASK_STATUS_META } from '../utils';
import { ChainStateBadge } from './ChainStateBadge';

export interface AuditTaskRowProps {
  task: TaskSummary;
  chain: ChainVerification;
  lastNodeAt: string | null;
}

export function AuditTaskRow({ task, chain, lastNodeAt }: AuditTaskRowProps) {
  const statusMeta = TASK_STATUS_META[task.status];

  return (
    <Link to={`/audit/${task.id}`} className="group block focus:outline-none">
      <Card
        className={cn(
          'p-4 transition-all group-hover:-translate-y-0.5 group-hover:border-indigo-200 group-hover:shadow-md',
          'group-focus-visible:ring-2 group-focus-visible:ring-indigo-500/40',
        )}
      >
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 flex-1 space-y-2">
            <p className="text-xs text-slate-400">
              <span className="font-mono">{task.id}</span>
              <span className="mx-1.5">·</span>
              {task.category}
            </p>
            <h3 className="truncate text-sm font-semibold text-slate-900 transition-colors group-hover:text-indigo-600">
              {task.title}
            </h3>
            <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-slate-500">
              <Badge variant={statusMeta.variant}>{statusMeta.label}</Badge>
              <span className="inline-flex items-center gap-1">
                <Clock3 className="h-3.5 w-3.5 text-slate-400" aria-hidden="true" />
                最近节点：
                {lastNodeAt ? formatDateTime(lastNodeAt) : '暂无证据链'}
              </span>
              <span className="inline-flex items-center gap-1">
                <GitBranch className="h-3.5 w-3.5 text-slate-400" aria-hidden="true" />
                {chain.nodeCount > 0 ? `${chain.nodeCount} 个节点` : '0 个节点'}
              </span>
            </div>
          </div>

          <div className="flex shrink-0 items-center justify-between gap-3 sm:flex-col sm:items-end">
            <ChainStateBadge state={chain.state} />
            <span className="inline-flex items-center gap-1 text-xs font-medium text-indigo-600">
              查看证据链
              <ChevronRight className="h-3.5 w-3.5" aria-hidden="true" />
            </span>
          </div>
        </div>
      </Card>
    </Link>
  );
}
