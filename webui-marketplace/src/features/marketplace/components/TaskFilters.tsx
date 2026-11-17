import { RotateCcw, Search, SlidersHorizontal } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { cn } from '../../../lib/cn';
import type { TaskFilters as TaskFiltersValue, TaskStatus } from '../../../types/domain';
import { STATUS_LABEL } from '../utils';
import type { CategoryOption } from '../types';

export interface TaskFiltersProps {
  filters: TaskFiltersValue;
  category: string;
  categories: CategoryOption[];
  onChange: (next: Partial<TaskFiltersValue>) => void;
  onCategoryChange: (category: string) => void;
  onReset: () => void;
  className?: string;
}

const STATUS_OPTIONS: Array<TaskStatus | 'ALL'> = [
  'ALL',
  'PUBLISHED',
  'ACCEPTED',
  'IN_PROGRESS',
  'SUBMITTED',
  'AI_VALIDATING',
  'VALIDATION_FAILED',
  'COMPLIANCE_SCAN',
  'SETTLING',
  'COMPLETED',
  'DISPUTED',
  'REFUNDED',
  'CANCELLED',
  'DRAFT',
];

const inputClass =
  'h-9 w-full rounded-lg border border-slate-300 bg-panel px-3 text-sm text-slate-900 outline-none transition-colors placeholder:text-slate-400 focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100';

export function TaskFilters({
  filters,
  category,
  categories,
  onChange,
  onCategoryChange,
  onReset,
  className,
}: TaskFiltersProps) {
  return (
    <Card className={cn('space-y-3 p-4', className)}>
      <div className="flex items-center justify-between">
        <h2 className="flex items-center gap-1.5 text-sm font-semibold text-ink">
          <SlidersHorizontal className="h-4 w-4 text-neon" />
          悬赏筛选
        </h2>
        <Button variant="ghost" size="sm" onClick={onReset}>
          <RotateCcw className="h-3.5 w-3.5" />
          重置
        </Button>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <label className="space-y-1 sm:col-span-2 lg:col-span-1">
          <span className="text-xs font-medium text-slate-600">关键词</span>
          <span className="relative block">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
            <input
              type="search"
              value={filters.keyword}
              onChange={(event) =>
                onChange({ keyword: event.target.value, page: 1 })
              }
              placeholder="标题、发布者或类别"
              className={cn(inputClass, 'pl-9')}
            />
          </span>
        </label>

        <label className="space-y-1">
          <span className="text-xs font-medium text-slate-600">副本类别</span>
          <select
            value={category}
            onChange={(event) => onCategoryChange(event.target.value)}
            className={inputClass}
          >
            {categories.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>

        <label className="space-y-1">
          <span className="text-xs font-medium text-slate-600">关卡状态</span>
          <select
            value={filters.status}
            onChange={(event) =>
              onChange({
                status: event.target.value as TaskFiltersValue['status'],
                page: 1,
              })
            }
            className={inputClass}
          >
            {STATUS_OPTIONS.map((status) => (
              <option key={status} value={status}>
                {status === 'ALL' ? '全部状态' : STATUS_LABEL[status]}
              </option>
            ))}
          </select>
        </label>

        <div className="space-y-1">
          <span className="text-xs font-medium text-slate-600">金币赏金区间</span>
          <div className="flex items-center gap-2">
            <input
              type="number"
              min={0}
              step={100}
              value={filters.rewardMin ?? ''}
              onChange={(event) =>
                onChange({
                  rewardMin:
                    event.target.value === ''
                      ? undefined
                      : Number(event.target.value),
                  page: 1,
                })
              }
              placeholder="最低金币"
              className={inputClass}
            />
            <span className="text-xs text-slate-400">至</span>
            <input
              type="number"
              min={0}
              step={100}
              value={filters.rewardMax ?? ''}
              onChange={(event) =>
                onChange({
                  rewardMax:
                    event.target.value === ''
                      ? undefined
                      : Number(event.target.value),
                  page: 1,
                })
              }
              placeholder="最高金币"
              className={inputClass}
            />
          </div>
        </div>
      </div>
    </Card>
  );
}
