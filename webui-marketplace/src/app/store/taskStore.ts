import { create } from 'zustand';
import {
  ALLOWED_TRANSITIONS,
  type TaskDetail,
  type TaskFilters,
  type TaskStatus,
  type TaskSummary,
} from '../../types/domain';
import type { PublishTaskInput } from '../../types/api';
import {
  acceptTask as acceptTaskApi,
  getTask,
  getTasks,
  publishTask as publishTaskApi,
} from '../../lib/api/client';

export interface TaskState {
  tasks: TaskSummary[];
  filters: TaskFilters;
  selectedTask: TaskDetail | null;
  loading: boolean;
  fetchTasks(filters?: Partial<TaskFilters>): Promise<void>;
  fetchTask(id: string): Promise<void>;
  publishTask(input: PublishTaskInput): Promise<TaskDetail>;
  acceptTask(id: string): Promise<void>;
  advanceStage(id: string, next: TaskStatus): void;
}

export const defaultTaskFilters: TaskFilters = {
  status: 'ALL',
  keyword: '',
  page: 1,
};

const toSummary = (task: TaskDetail): TaskSummary => ({
  id: task.id,
  title: task.title,
  category: task.category,
  status: task.status,
  reward: task.reward,
  deadline: task.deadline,
  publisher: task.publisher,
  requiredSkills: task.requiredSkills,
  acceptCount: 0,
  validationScore: 0,
});

export const useTaskStore = create<TaskState>((set, get) => ({
  tasks: [],
  filters: defaultTaskFilters,
  selectedTask: null,
  loading: false,

  fetchTasks: async (next) => {
    const filters: TaskFilters = { ...get().filters, ...next };
    set({ filters, loading: true });
    try {
      const response = await getTasks({
        status: filters.status === 'ALL' ? undefined : filters.status,
        keyword: filters.keyword || undefined,
        rewardMin: filters.rewardMin,
        rewardMax: filters.rewardMax,
        page: filters.page,
      });
      set({ tasks: response.items });
    } finally {
      set({ loading: false });
    }
  },

  fetchTask: async (id) => {
    set({ loading: true });
    try {
      const task = await getTask(id);
      set({ selectedTask: task });
    } catch {
      set({ selectedTask: null });
    } finally {
      set({ loading: false });
    }
  },

  publishTask: async (input) => {
    const task = await publishTaskApi(input);
    set((state) => ({
      tasks: [toSummary(task), ...state.tasks],
      selectedTask: task,
    }));
    return task;
  },

  acceptTask: async (id) => {
    const response = await acceptTaskApi(id);
    get().advanceStage(id, response.status);
    // 接单后刷新详情，拿到最新 assignee 与流水线状态。
    await get().fetchTask(id);
  },

  advanceStage: (id, next) => {
    const state = get();
    const currentStatus: TaskStatus | null =
      state.selectedTask?.id === id
        ? state.selectedTask.status
        : (state.tasks.find((task) => task.id === id)?.status ?? null);

    if (!currentStatus) {
      throw new Error(`Task ${id} not found`);
    }

    if (!ALLOWED_TRANSITIONS[currentStatus].includes(next)) {
      throw new Error(`Illegal task transition: ${currentStatus} -> ${next}`);
    }

    set({
      tasks: state.tasks.map((task) =>
        task.id === id ? { ...task, status: next } : task,
      ),
      selectedTask:
        state.selectedTask && state.selectedTask.id === id
          ? { ...state.selectedTask, status: next }
          : state.selectedTask,
    });
  },
}));
