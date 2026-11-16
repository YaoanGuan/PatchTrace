import { useState } from 'react';
import { useSessionStore } from '../../../app/store/sessionStore';
import { useTaskStore } from '../../../app/store/taskStore';
import { useUiStore } from '../../../app/store/uiStore';
import type { TaskFilters as TaskFiltersValue } from '../../../types/domain';
import { CATEGORY_OPTIONS, errorMessage } from '../utils';
import { AcceptTaskPanel } from './AcceptTaskPanel';
import { TaskFilters } from './TaskFilters';
import { TaskList } from './TaskList';
import { TaskPublishForm } from './TaskPublishForm';

export function TaskMarketplace() {
  const activeRole = useSessionStore((state) => state.activeRole);
  const tasks = useTaskStore((state) => state.tasks);
  const filters = useTaskStore((state) => state.filters);
  const loading = useTaskStore((state) => state.loading);
  const fetchTasks = useTaskStore((state) => state.fetchTasks);
  const pushToast = useUiStore((state) => state.pushToast);
  const [category, setCategory] = useState('ALL');

  const applyFilters = (next: Partial<TaskFiltersValue>) => {
    void fetchTasks(next).catch((error) => {
      pushToast({
        id: `filters-error-${Date.now()}`,
        title: '筛选任务失败',
        description: errorMessage(error),
        variant: 'error',
      });
    });
  };

  const handleCategoryChange = (nextCategory: string) => {
    setCategory(nextCategory);
    // API 不支持 category 查询参数，刷新最新列表后在 TaskList 内按类别过滤。
    applyFilters({});
  };

  const handleReset = () => {
    setCategory('ALL');
    applyFilters({
      status: 'ALL',
      keyword: '',
      rewardMin: undefined,
      rewardMax: undefined,
      page: 1,
    });
  };

  return (
    <div className="space-y-4">
      <TaskFilters
        filters={filters}
        category={category}
        categories={CATEGORY_OPTIONS}
        onChange={applyFilters}
        onCategoryChange={handleCategoryChange}
        onReset={handleReset}
      />

      <div className="grid gap-4 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <TaskList tasks={tasks} category={category} loading={loading} />
        </div>
        <div className="space-y-4">
          {activeRole === 'EMPLOYER' ? (
            <TaskPublishForm />
          ) : (
            <AcceptTaskPanel tasks={tasks} />
          )}
        </div>
      </div>
    </div>
  );
}
