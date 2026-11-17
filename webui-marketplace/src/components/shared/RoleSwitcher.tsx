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
  EMPLOYER: '雇主公会',
  COMMANDER: '指挥官战队',
};

const ROLE_HINT: Record<UserRole, string> = {
  EMPLOYER: '发布悬赏、冻结赏金、验收战利品',
  COMMANDER: '接取副本、调度 Agent、提交战利品',
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
      title="切换身份"
      description="身份只影响当前可执行的副本动作，不改变角色档案。"
      footer={
        <div className="flex items-center justify-end gap-2">
          <Button variant="secondary" onClick={onCancel}>
            取消
          </Button>
          <Button onClick={onConfirm}>
            确认切换为{role ? ROLE_LABEL[role] : '新身份'}
          </Button>
        </div>
      }
    >
      <p className="text-sm leading-6 text-ink-muted">
        确认后，页面操作将以「{role ? ROLE_LABEL[role] : '新身份'}」视角呈现。
        同一个人可以同时是发悬赏的公会与接副本的战队，切换身份不会改变账号。
      </p>
      {role && (
        <p className="mt-2 rounded border border-panel-line bg-panel-soft px-3 py-2 text-xs text-ink-muted">
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
                'flex items-start gap-3 rounded border p-4 text-left transition-all',
                active
                  ? 'border-gold/40 bg-gold/10'
                  : 'border-panel-line bg-panel/60 hover:border-neon/40 hover:bg-panel-soft',
              )}
            >
              <span
                className={cn(
                  'mt-0.5 rounded p-2',
                  active
                    ? 'bg-gold text-slate-950'
                    : 'bg-panel-soft text-ink-dim',
                )}
              >
                <Icon className="h-4 w-4" />
              </span>
              <span className="flex-1 space-y-1">
                <span className="flex items-center gap-1.5 text-sm font-semibold text-ink">
                  {ROLE_LABEL[role]}
                  {active && (
                    <CheckCircle2 className="h-3.5 w-3.5 text-gold" />
                  )}
                </span>
                <span className="block text-xs leading-5 text-ink-muted">
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
        title="切换身份"
        className={cn(
          'inline-flex items-center gap-2 rounded border border-panel-line bg-panel-soft px-3 py-1.5 text-xs font-medium text-ink-muted transition-colors hover:border-gold/40 hover:text-gold',
          className,
        )}
      >
        <ActiveIcon className="h-4 w-4 text-gold" />
        <span className="hidden sm:inline">{ROLE_LABEL[activeRole]}</span>
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
