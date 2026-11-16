import { EmptyState } from '../../../components/ui/EmptyState';
import type { TraceNode as TraceNodeModel, TraceNodeType } from '../../../types/domain';
import type { ChainVerification } from '../types';
import { TRACE_NODE_META, TraceNode } from './TraceNode';

export interface TraceabilityTimelineProps {
  nodes: TraceNodeModel[];
  verification: ChainVerification;
}

const NODE_TYPE_ORDER: TraceNodeType[] = [
  'REQUIREMENT',
  'ACCEPTANCE',
  'MODEL_CALL',
  'ACTION',
  'OUTPUT',
  'VALIDATION',
];

export function TraceabilityTimeline({
  nodes,
  verification,
}: TraceabilityTimelineProps) {
  const brokenIndexes = new Set(verification.issues.map((issue) => issue.index));

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-6">
        {NODE_TYPE_ORDER.map((type) => {
          const meta = TRACE_NODE_META[type];
          const Icon = meta.icon;
          const count = nodes.filter((node) => node.type === type).length;
          return (
            <div
              key={type}
              className="flex items-center gap-2 rounded-lg border border-slate-100 bg-slate-50/70 px-2.5 py-2"
            >
              <span
                className={`flex h-7 w-7 items-center justify-center rounded-full ${meta.bubble}`}
                aria-hidden="true"
              >
                <Icon className="h-3.5 w-3.5" />
              </span>
              <div className="min-w-0">
                <p className="truncate text-[11px] font-medium text-slate-600">
                  {meta.label}
                </p>
                <p className="text-[11px] text-slate-400">{count} 个节点</p>
              </div>
            </div>
          );
        })}
      </div>

      {nodes.length === 0 ? (
        <EmptyState
          title="暂无证据链节点"
          description="该任务尚未产生可审计的执行记录，无法执行哈希链校验或签署免责契约。"
        />
      ) : (
        <ol className="space-y-0">
          {nodes.map((node, index) => (
            <TraceNode
              key={node.id}
              node={node}
              index={index}
              isLast={index === nodes.length - 1}
              broken={brokenIndexes.has(index)}
            />
          ))}
        </ol>
      )}
    </div>
  );
}
