import { useRef, useState, type ChangeEvent } from 'react';
import { FileUp, ShieldCheck } from 'lucide-react';
import { Modal } from '../../../components/ui/Modal';
import { Button } from '../../../components/ui/Button';
import { useUiStore } from '../../../app/store/uiStore';
import { cn } from '../../../lib/cn';
import {
  CREDENTIAL_TYPE_OPTIONS,
  type CredentialType,
} from '../types';

export interface CredentialUploadDialogProps {
  open: boolean;
  onClose: () => void;
}

/**
 * 认证凭证上传弹窗。
 * 演示实现：只维护本地凭证类型与文件名状态，提交时推送 toast，不真正上传。
 */
export function CredentialUploadDialog({
  open,
  onClose,
}: CredentialUploadDialogProps) {
  const pushToast = useUiStore((state) => state.pushToast);
  const [credentialType, setCredentialType] =
    useState<CredentialType>('STUDENT_ID');
  const [fileName, setFileName] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const selected =
    CREDENTIAL_TYPE_OPTIONS.find(
      (option) => option.value === credentialType,
    ) ?? CREDENTIAL_TYPE_OPTIONS[0];

  const reset = () => {
    setCredentialType('STUDENT_ID');
    setFileName(null);
    if (inputRef.current) {
      inputRef.current.value = '';
    }
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0] ?? null;
    setFileName(file?.name ?? null);
  };

  const handleSubmit = () => {
    if (!fileName) {
      pushToast({
        id: `credential-warning-${Date.now()}`,
        title: '请先选择凭证文件',
        description: `请上传${selected.label}的扫描件或截图。`,
        variant: 'warning',
      });
      return;
    }

    pushToast({
      id: `credential-success-${Date.now()}`,
      title: '凭证已提交审核',
      description: `${selected.label} · ${fileName}（本地演示，不会真实上传）`,
      variant: 'success',
    });
    handleClose();
  };

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="上传认证凭证"
      description="学籍 / 工牌 / 企业资质，仅用于演示本地状态。"
      footer={
        <div className="flex items-center justify-end gap-2">
          <Button variant="secondary" onClick={handleClose}>
            取消
          </Button>
          <Button onClick={handleSubmit}>提交审核</Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div>
          <p className="mb-2 text-xs font-medium text-slate-600">凭证类型</p>
          <div className="grid gap-2 sm:grid-cols-2">
            {CREDENTIAL_TYPE_OPTIONS.map((option) => {
              const active = option.value === credentialType;
              return (
                <button
                  key={option.value}
                  type="button"
                  onClick={() => setCredentialType(option.value)}
                  aria-pressed={active}
                  className={cn(
                    'rounded-lg border p-3 text-left transition-colors',
                    active
                      ? 'border-indigo-500 bg-indigo-50/60'
                      : 'border-slate-200 bg-white hover:border-slate-300 hover:bg-slate-50',
                  )}
                >
                  <span className="flex items-center gap-1.5 text-sm font-medium text-slate-900">
                    {option.label}
                    {active && (
                      <ShieldCheck
                        className="h-3.5 w-3.5 text-indigo-600"
                        aria-hidden="true"
                      />
                    )}
                  </span>
                  <span className="mt-0.5 block text-xs leading-5 text-slate-500">
                    {option.hint}
                  </span>
                </button>
              );
            })}
          </div>
        </div>

        <div>
          <p className="mb-2 text-xs font-medium text-slate-600">凭证文件</p>
          <label className="flex cursor-pointer flex-col items-center justify-center gap-1.5 rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-center transition-colors hover:border-indigo-400 hover:bg-indigo-50/40">
            <FileUp className="h-5 w-5 text-slate-400" aria-hidden="true" />
            <span className="text-sm text-slate-700">
              {fileName ?? '点击选择文件'}
            </span>
            <span className="text-[11px] text-slate-400">
              支持 PDF / PNG / JPG，单个文件不超过 10MB
            </span>
            <input
              ref={inputRef}
              type="file"
              accept=".pdf,.png,.jpg,.jpeg"
              onChange={handleFileChange}
              className="sr-only"
            />
          </label>
        </div>

        <p className="rounded-lg bg-amber-50 px-3 py-2 text-[11px] leading-5 text-amber-700">
          演示环境：文件仅停留在浏览器本地状态，不会真正上传到服务器。
        </p>
      </div>
    </Modal>
  );
}
