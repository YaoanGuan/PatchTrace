import { useState, type FormEvent } from 'react';
import { Loader2 } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { cn } from '../../../lib/cn';
import { useChannelRegistration } from '../hooks/useChannelRegistration';
import type {
  ChannelFormErrors,
  FreelancerFormValues,
  RegistrationStatus,
} from '../types';
import { FormField, inputClassName, inputErrorClassName } from './FormField';

const EMPTY_VALUES: FreelancerFormValues = {
  realName: '',
  portfolioUrl: '',
  historyNote: '',
};

const HTTP_URL_PATTERN = /^https?:\/\/[^\s]+\.[^\s]+/i;

export interface FreelancerFormProps {
  onRegistered?: (status: RegistrationStatus) => void;
}

export function FreelancerForm({ onRegistered }: FreelancerFormProps) {
  const [values, setValues] = useState<FreelancerFormValues>(EMPTY_VALUES);
  const [errors, setErrors] = useState<
    ChannelFormErrors<FreelancerFormValues>
  >({});
  const { submitting, submit } = useChannelRegistration(
    'FREELANCER',
    onRegistered,
  );

  function update<K extends keyof FreelancerFormValues>(
    key: K,
    value: FreelancerFormValues[K],
  ) {
    setValues((prev) => ({ ...prev, [key]: value }));
    setErrors((prev) => ({ ...prev, [key]: undefined }));
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const nextErrors: ChannelFormErrors<FreelancerFormValues> = {};
    if (!values.realName.trim()) {
      nextErrors.realName = '请填写真实姓名。';
    }
    if (!values.portfolioUrl.trim()) {
      nextErrors.portfolioUrl = '请填写作品集链接。';
    } else if (!HTTP_URL_PATTERN.test(values.portfolioUrl.trim())) {
      nextErrors.portfolioUrl = '请输入以 http:// 或 https:// 开头的有效链接。';
    }

    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) {
      return;
    }

    const ok = await submit({
      realName: values.realName.trim(),
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
          id="freelancer-real-name"
          label="实名"
          required
          error={errors.realName}
          hint="仅用于人工审核，不会公开展示。"
        >
          <input
            id="freelancer-real-name"
            value={values.realName}
            onChange={(event) => update('realName', event.target.value)}
            className={cn(
              inputClassName,
              errors.realName && inputErrorClassName,
            )}
            placeholder="请输入真实姓名"
            autoComplete="name"
          />
        </FormField>

        <FormField
          id="freelancer-portfolio"
          label="作品集链接"
          required
          error={errors.portfolioUrl}
          hint="GitHub、博客、漏洞报告或项目主页均可。"
        >
          <input
            id="freelancer-portfolio"
            type="url"
            value={values.portfolioUrl}
            onChange={(event) => update('portfolioUrl', event.target.value)}
            className={cn(
              inputClassName,
              errors.portfolioUrl && inputErrorClassName,
            )}
            placeholder="https://example.com/portfolio"
            autoComplete="url"
          />
        </FormField>

        <FormField
          id="freelancer-history"
          label="历史评价说明"
          error={errors.historyNote}
          hint="可补充过往合作方、代表性交付或争议说明，帮助审核人员判断初始信誉。"
          className="sm:col-span-2"
        >
          <textarea
            id="freelancer-history"
            value={values.historyNote}
            onChange={(event) => update('historyNote', event.target.value)}
            maxLength={500}
            rows={3}
            className={cn(
              inputClassName,
              'resize-y',
              errors.historyNote && inputErrorClassName,
            )}
            placeholder="例如：曾为 3 家中小企业完成代码审计，交付证据链完整，无争议记录。"
          />
        </FormField>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit" disabled={submitting}>
          {submitting && <Loader2 className="h-4 w-4 animate-spin" />}
          {submitting ? '提交中…' : '提交自由职业者申请'}
        </Button>
        <span className="text-xs text-slate-400">
          作品集与历史评价说明用于人工审核展示，接口当前提交实名作为核验证据。
        </span>
      </div>
    </form>
  );
}
