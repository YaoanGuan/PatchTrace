import { useEffect } from 'react';
import { Outlet } from 'react-router-dom';
import { X } from 'lucide-react';
import { TopBar } from './TopBar';
import { SideNav } from './SideNav';
import { useSessionStore } from '../../app/store/sessionStore';
import { useWalletStore } from '../../app/store/walletStore';
import { useUiStore } from '../../app/store/uiStore';
import { Button } from '../ui/Button';
import { Modal } from '../ui/Modal';
import { cn } from '../../lib/cn';
import type { ToastVariant } from '../../types/domain';

const toastTone: Record<ToastVariant, string> = {
  info: 'border-indigo-200 bg-indigo-50 text-indigo-800',
  success: 'border-emerald-200 bg-emerald-50 text-emerald-800',
  warning: 'border-amber-200 bg-amber-50 text-amber-800',
  error: 'border-rose-200 bg-rose-50 text-rose-800',
};

/**
 * 全局免责与责任边界弹窗。
 * 由 uiStore.legalModal 驱动；审计模块可以在此基础上扩展条款勾选与签名。
 */
function LegalLiabilityModal() {
  const legalModal = useUiStore((state) => state.legalModal);
  const closeLegalModal = useUiStore((state) => state.closeLegalModal);

  return (
    <Modal
      open={legalModal.open}
      onClose={closeLegalModal}
      title="免责与责任边界"
      description={
        legalModal.taskId ? `任务 ${legalModal.taskId}` : undefined
      }
      footer={
        <div className="flex justify-end">
          <Button onClick={closeLegalModal}>我已了解</Button>
        </div>
      }
    >
      <div className="space-y-3 text-sm leading-6 text-slate-600">
        <p>
          平台仅提供任务封装、资金托管、执行证据链与争议仲裁流程，不直接参与任务执行。
        </p>
        <ul className="list-disc space-y-1 pl-5">
          <li>雇主：对任务目标、授权范围与验收标准负责。</li>
          <li>AI 指挥官：对执行过程、工具调用与产出证据的真实性负责。</li>
          <li>平台：对托管资金、哈希链审计与争议仲裁流程负责。</li>
        </ul>
        <p className="text-xs text-slate-400">
          完整条款勾选与签名哈希由审计模块的契约签署面板生成。
        </p>
      </div>
    </Modal>
  );
}

function ToastViewport() {
  const toasts = useUiStore((state) => state.toasts);
  const dismissToast = useUiStore((state) => state.dismissToast);

  return (
    <div className="pointer-events-none fixed bottom-4 right-4 z-50 flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className={cn(
            'pointer-events-auto flex items-start gap-3 rounded-lg border px-4 py-3 shadow-lg',
            toastTone[toast.variant ?? 'info'],
          )}
        >
          <div className="min-w-0 flex-1 space-y-0.5">
            <p className="text-sm font-medium">{toast.title}</p>
            {toast.description && (
              <p className="text-xs opacity-80">{toast.description}</p>
            )}
          </div>
          <button
            type="button"
            onClick={() => dismissToast(toast.id)}
            aria-label="关闭提示"
            className="rounded p-0.5 opacity-60 transition-opacity hover:opacity-100"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      ))}
    </div>
  );
}

export function AppShell() {
  const loadProfile = useSessionStore((state) => state.loadProfile);
  const refreshWallet = useWalletStore((state) => state.refresh);

  useEffect(() => {
    void loadProfile();
    void refreshWallet();
  }, [loadProfile, refreshWallet]);

  return (
    <div className="flex min-h-screen flex-col bg-slate-50 text-slate-900">
      <TopBar />
      <div className="flex flex-1 flex-col sm:flex-row">
        <SideNav />
        <main className="flex-1 overflow-y-auto px-4 py-6 sm:px-6 lg:px-8">
          <div className="mx-auto w-full max-w-6xl">
            <Outlet />
          </div>
        </main>
      </div>
      <LegalLiabilityModal />
      <ToastViewport />
    </div>
  );
}
