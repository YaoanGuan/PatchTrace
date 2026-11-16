import { Bot, Briefcase, CheckCircle2 } from 'lucide-react';
import type { UserRole } from '../../types/domain';
import { useSessionStore } from '../../app/store/sessionStore';
import { Button } from '../ui/Button';
import { Modal } from '../ui/Modal';
import { cn } from '../../lib/cn';

export interface RoleSwitcherProps {
  variant?: 'compact' | 'panel';
  className?: string;
}

const ROLE_LABEL: Record<UserRole, string> = {
  EMPLOYER: '雇主',
  COMMANDER: 'AI 指挥官',
};

const ROLE_HINT: Record<UserRole, string> = {
  EMPLOYER: '发布任务、冻结赏金、验收交付',
  COMMANDER: '领取任务、调度 Agent、提交证据',
};

const ROLE_ORDER: UserRole[] = ['EMPLOYER', 'COMMANDER'];

interface RoleConfirmModalProps {
  open: boolean;
  role: UserRole | null;
  onCancel: () => void;
  onConfirm: () => void;
}

function RoleConfirmModal({
  open,
  role,
  onCancel,
  onConfirm,
}: RoleConfirmModalProps) {
  return (
    <Modal
      open={open}
      onClose={onCancel}
      title="切换视角"
      description="activeRole 只影响当前可执行动作，不改变账号身份。"
      footer={
        <div className="flex items-center justify-end gap-2">
          <Button variant="secondary" onClick={onCancel}>
            取消
          </Button>
          <Button onClick={onConfirm}>
            确认切换为{role ? ROLE_LABEL[role] : '新视角'}
          </Button>
        </div>
      }
    >
      <p className="text-sm leading-6 text-slate-600">
        确认后，页面操作将以「{role ? ROLE_LABEL[role] : '新视角'}」视角呈现。
        同一个人可以同时是发单方与接单方，切换视角不会改变账号身份。
      </p>
      {role && (
        <p className="mt-2 rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">
          {ROLE_HINT[role]}
        </p>
      )}
    </Modal>
  );
}

export function RoleSwitcher({
  variant = 'compact',
  className,
}: RoleSwitcherProps) {
  const activeRole = useSessionStore((state) => state.activeRole);
  const pendingRole = useSessionStore((state) => state.pendingRole);
  const setActiveRole = useSessionStore((state) => state.setActiveRole);
  const toggleRole = useSessionStore((state) => state.toggleRole);

  const cancel = () => setActiveRole(activeRole);
  const confirm = () => {
    if (pendingRole) {
      setActiveRole(pendingRole);
    }
  };

  if (variant === 'panel') {
    return (
      <div className={cn('grid gap-2 sm:grid-cols-2', className)}>
        {ROLE_ORDER.map((role) => {
          const active = role === activeRole;
          const Icon = role === 'EMPLOYER' ? Briefcase : Bot;
          return (
            <button
              key={role}
              type="button"
              onClick={() => {
                if (!active) {
                  toggleRole();
                }
              }}
              className={cn(
                'flex items-start gap-3 rounded-xl border p-4 text-left transition-colors',
                active
                  ? 'border-indigo-500 bg-indigo-50/60'
                  : 'border-slate-200 bg-white hover:border-slate-300 hover:bg-slate-50',
              )}
            >
              <span
                className={cn(
                  'mt-0.5 rounded-lg p-2',
                  active
                    ? 'bg-indigo-600 text-white'
                    : 'bg-slate-100 text-slate-500',
                )}
              >
                <Icon className="h-4 w-4" />
              </span>
              <span className="flex-1 space-y-1">
                <span className="flex items-center gap-1.5 text-sm font-semibold text-slate-900">
                  {ROLE_LABEL[role]}
                  {active && (
                    <CheckCircle2 className="h-3.5 w-3.5 text-indigo-600" />
                  )}
                </span>
                <span className="block text-xs leading-5 text-slate-500">
                  {ROLE_HINT[role]}
                </span>
              </span>
            </button>
          );
        })}
        <RoleConfirmModal
          open={pendingRole !== null}
          role={pendingRole}
          onCancel={cancel}
          onConfirm={confirm}
        />
      </div>
    );
  }

  const ActiveIcon = activeRole === 'EMPLOYER' ? Briefcase : Bot;

  return (
    <>
      <button
        type="button"
        onClick={toggleRole}
        title="切换视角"
        className={cn(
          'inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition-colors hover:border-slate-300 hover:bg-slate-50',
          className,
        )}
      >
        <ActiveIcon className="h-4 w-4 text-indigo-600" />
        <span>{ROLE_LABEL[activeRole]}</span>
      </button>
      <RoleConfirmModal
        open={pendingRole !== null}
        role={pendingRole}
        onCancel={cancel}
        onConfirm={confirm}
      />
    </>
  );
}
