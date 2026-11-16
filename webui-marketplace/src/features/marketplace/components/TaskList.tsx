import { ListChecks } from 'lucide-react';
import { EmptyState } from '../../../components/ui/EmptyState';
import type { TaskSummary } from '../../../types/domain';
import { TaskCard } from './TaskCard';

export interface TaskListProps {
  tasks: TaskSummary[];
  category: string;
  loading: boolean;
}

function TaskListSkeleton() {
  return (
    <div className="grid gap-3 md:grid-cols-2">
      {Array.from({ length: 4 }).map((_, index) => (
        <div
          key={index}
          className="h-40 animate-pulse rounded-xl border border-slate-200 bg-white p-4"
        >
          <div className="mb-3 h-4 w-2/3 rounded bg-slate-100" />
          <div className="mb-2 h-3 w-1/2 rounded bg-slate-100" />
          <div className="mt-6 h-3 w-full rounded bg-slate-100" />
        </div>
      ))}
    </div>
  );
}

export function TaskList({ tasks, category, loading }: TaskListProps) {
  const filtered =
    category === 'ALL'
      ? tasks
      : tasks.filter((task) => task.category === category);

  if (loading && tasks.length === 0) {
    return <TaskListSkeleton />;
  }

  if (filtered.length === 0) {
    return (
      <EmptyState
        icon={<ListChecks className="h-8 w-8 text-slate-400" />}
        title="没有匹配的任务"
        description={
          tasks.length > 0
            ? '当前类别下暂无任务，试试切换类别或放宽筛选条件。'
            : '调整关键词、状态或赏金区间后再试一次。'
        }
      />
    );
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-xs text-slate-500">
        <span>{filtered.length} 个任务</span>
        {loading && <span className="text-indigo-600">刷新中…</span>}
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        {filtered.map((task) => (
          <TaskCard key={task.id} task={task} />
        ))}
      </div>
    </div>
  );
}
