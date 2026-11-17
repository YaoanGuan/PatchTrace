import { AlertTriangle, RotateCw } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { EmptyState } from '../../../components/ui/EmptyState';
import type { AuditTaskEntry } from '../types';
import { AuditTaskRow } from './AuditTaskRow';

export interface AuditTaskListProps {
  entries: AuditTaskEntry[];
  loading: boolean;
  error: string | null;
  onRetry(): void;
}

export function AuditTaskList({
  entries,
  loading,
  error,
  onRetry,
}: AuditTaskListProps) {
  if (loading) {
    return (
      <div className="space-y-3" aria-busy="true" aria-label="战报加载中">
        {[0, 1, 2, 3].map((item) => (
          <div
            key={item}
            className="h-28 animate-pulse rounded-xl border border-slate-200 bg-slate-100/80"
          />
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <Card className="flex flex-col items-center gap-3 border-boss/30 bg-boss/10 px-6 py-10 text-center">
        <AlertTriangle className="h-8 w-8 text-boss" aria-hidden="true" />
        <div className="space-y-1">
          <h3 className="text-sm font-medium text-boss">战报数据加载失败</h3>
          <p className="text-xs text-boss/80">{error}</p>
        </div>
        <Button variant="outline" size="sm" onClick={onRetry}>
          <RotateCw className="h-3.5 w-3.5" aria-hidden="true" />
          重新加载
        </Button>
      </Card>
    );
  }

  if (entries.length === 0) {
    return (
      <EmptyState
        title="没有匹配的战报副本"
        description="尝试调整关键词或切换封印链状态筛选条件。"
      />
    );
  }

  return (
    <div className="space-y-3">
      {entries.map((entry) => (
        <AuditTaskRow
          key={entry.task.id}
          task={entry.task}
          chain={entry.chain}
          lastNodeAt={entry.lastNodeAt}
        />
      ))}
    </div>
  );
}
