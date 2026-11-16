import { useState, type FormEvent } from 'react';
import { Loader2 } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { cn } from '../../../lib/cn';
import { useChannelRegistration } from '../hooks/useChannelRegistration';
import type {
  ChannelFormErrors,
  EnterpriseFormValues,
  RegistrationStatus,
} from '../types';
import { FormField, inputClassName, inputErrorClassName } from './FormField';

const EMPTY_VALUES: EnterpriseFormValues = {
  companyName: '',
  workEmail: '',
  credential: '',
};

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export interface EnterpriseWhitelistFormProps {
  onRegistered?: (status: RegistrationStatus) => void;
}

export function EnterpriseWhitelistForm({
  onRegistered,
}: EnterpriseWhitelistFormProps) {
  const [values, setValues] = useState<EnterpriseFormValues>(EMPTY_VALUES);
  const [errors, setErrors] = useState<
    ChannelFormErrors<EnterpriseFormValues>
  >({});
  const { submitting, submit } = useChannelRegistration(
    'ENTERPRISE',
    onRegistered,
  );

  function update<K extends keyof EnterpriseFormValues>(
    key: K,
    value: EnterpriseFormValues[K],
  ) {
    setValues((prev) => ({ ...prev, [key]: value }));
    setErrors((prev) => ({ ...prev, [key]: undefined }));
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const nextErrors: ChannelFormErrors<EnterpriseFormValues> = {};
    if (!values.companyName.trim()) {
      nextErrors.companyName = '请填写企业名称。';
    }
    if (!values.workEmail.trim()) {
      nextErrors.workEmail = '请填写企业邮箱。';
    } else if (!EMAIL_PATTERN.test(values.workEmail.trim())) {
      nextErrors.workEmail = '请输入有效的企业邮箱地址。';
    }
    if (!values.credential.trim()) {
      nextErrors.credential = '请填写工牌号或统一社会信用代码。';
    }

    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) {
      return;
    }

    const ok = await submit({
      workEmail: values.workEmail.trim(),
    });
    if (ok) {
      setValues(EMPTY_VALUES);
      setErrors({});
    }
  }

  return (
    <form noValidate onSubmit={handleSubmit} className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <FormField
          id="enterprise-company"
          label="企业名称"
          required
          error={errors.companyName}
          className="sm:col-span-2"
        >
          <input
            id="enterprise-company"
            value={values.companyName}
            onChange={(event) => update('companyName', event.target.value)}
            className={cn(
              inputClassName,
              errors.companyName && inputErrorClassName,
            )}
            placeholder="如：某某安全实验室有限公司"
            autoComplete="organization"
          />
        </FormField>

        <FormField
          id="enterprise-email"
          label="企业邮箱"
          required
          error={errors.workEmail}
          hint="需为企业域名邮箱，平台将核验域名与白名单记录。"
        >
          <input
            id="enterprise-email"
            type="email"
            value={values.workEmail}
            onChange={(event) => update('workEmail', event.target.value)}
            className={cn(
              inputClassName,
              errors.workEmail && inputErrorClassName,
            )}
            placeholder="name@company.com"
            autoComplete="email"
          />
        </FormField>

        <FormField
          id="enterprise-credential"
          label="工牌号 / 统一社会信用代码"
          required
          error={errors.credential}
          hint="用于白名单人工复核，不会公开展示。"
        >
          <input
            id="enterprise-credential"
            value={values.credential}
            onChange={(event) => update('credential', event.target.value)}
            className={cn(
              inputClassName,
              errors.credential && inputErrorClassName,
            )}
            placeholder="工牌号或 18 位统一社会信用代码"
            autoComplete="off"
          />
        </FormField>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit" disabled={submitting}>
          {submitting && <Loader2 className="h-4 w-4 animate-spin" />}
          {submitting ? '提交中…' : '提交企业白名单申请'}
        </Button>
        <span className="text-xs text-slate-400">
          企业名称与工牌号用于审核展示，接口当前提交企业邮箱作为核验证据。
        </span>
      </div>
    </form>
  );
}
