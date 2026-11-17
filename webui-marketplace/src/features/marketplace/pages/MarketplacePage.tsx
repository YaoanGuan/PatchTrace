import { useEffect } from 'react';
import { useTaskStore } from '../../../app/store/taskStore';
import { useUiStore } from '../../../app/store/uiStore';
import {
  AIValidationPipeline,
  EscrowWallet,
  MarketplaceHeader,
  TaskMarketplace,
} from '../components';
import { errorMessage } from '../utils';

export default function MarketplacePage() {
  const tasks = useTaskStore((state) => state.tasks);
  const selectedTask = useTaskStore((state) => state.selectedTask);
  const loading = useTaskStore((state) => state.loading);
  const fetchTasks = useTaskStore((state) => state.fetchTasks);
  const fetchTask = useTaskStore((state) => state.fetchTask);
  const pushToast = useUiStore((state) => state.pushToast);

  useEffect(() => {
    fetchTasks().catch((error) => {
      pushToast({
        id: `tasks-load-error-${Date.now()}`,
        title: '悬赏列表加载失败',
        description: errorMessage(error),
        variant: 'error',
      });
    });
  }, [fetchTasks, pushToast]);

  // 流水线橱窗默认聚焦当前任务列表中的第一个任务；筛选变化时自动跟随。
  const focusId = tasks.some((task) => task.id === selectedTask?.id)
    ? selectedTask?.id
    : tasks[0]?.id;

  useEffect(() => {
    if (focusId && focusId !== selectedTask?.id) {
      void fetchTask(focusId);
    }
  }, [fetchTask, focusId, selectedTask?.id]);

  const pipelineTask =
    selectedTask && focusId === selectedTask.id ? selectedTask : null;

  return (
    <div className="space-y-6">
      <MarketplaceHeader />
      <TaskMarketplace />

      <div className="grid gap-4 lg:grid-cols-2">
        <AIValidationPipeline
          task={pipelineTask}
          loading={loading && !pipelineTask}
        />
        <EscrowWallet />
      </div>
    </div>
  );
}
