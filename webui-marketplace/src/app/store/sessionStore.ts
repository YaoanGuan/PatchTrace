import { create } from 'zustand';
import type { User, UserRole } from '../../types/domain';
import { getMe } from '../../lib/api/client';

export interface SessionState {
  currentUser: User | null;
  /** 当前视角：雇主 / AI 指挥官。只影响可执行动作，不改变账号身份。 */
  activeRole: UserRole;
  /** 切换视角时的待确认角色 */
  pendingRole: UserRole | null;
  setActiveRole(role: UserRole): void;
  toggleRole(): void;
  loadProfile(): Promise<void>;
}

export const useSessionStore = create<SessionState>((set, get) => ({
  currentUser: null,
  activeRole: 'EMPLOYER',
  pendingRole: null,

  setActiveRole: (role) => set({ activeRole: role, pendingRole: null }),

  toggleRole: () =>
    set({
      pendingRole: get().activeRole === 'EMPLOYER' ? 'COMMANDER' : 'EMPLOYER',
    }),

  loadProfile: async () => {
    const user = await getMe();
    set((state) => ({
      currentUser: user,
      // 首次加载采用账号默认视角，之后保留用户手动选择。
      activeRole: state.currentUser ? state.activeRole : user.activeRole,
    }));
  },
}));
