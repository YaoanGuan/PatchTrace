import { useState, type FormEvent } from 'react';
import { Loader2, Upload } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { cn } from '../../../lib/cn';
import { useChannelRegistration } from '../hooks/useChannelRegistration';
import type {
  ChannelFormErrors,
  RegistrationStatus,
  UniversityFormValues,
} from '../types';
import { FormField, inputClassName, inputErrorClassName } from './FormField';

const EMPTY_VALUES: UniversityFormValues = {
  school: '',
  studentId: '',
  schoolEmail: '',
  studentCardName: '',
};

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export interface UniversityVerificationFormProps {
  onRegistered?: (status: RegistrationStatus) => void;
}

export function UniversityVerificationForm({
  onRegistered,
}: UniversityVerificationFormProps) {
  const [values, setValues] = useState<UniversityFormValues>(EMPTY_VALUES);
  const [errors, setErrors] = useState<
    ChannelFormErrors<UniversityFormValues>
  >({});
  const { submitting, submit } = useChannelRegistration(
    'UNIVERSITY',
    onRegistered,
  );

  function update<K extends keyof UniversityFormValues>(
    key: K,
    value: UniversityFormValues[K],
  ) {
    setValues((prev) => ({ ...prev, [key]: value }));
    setErrors((prev) => ({ ...prev, [key]: undefined }));
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const nextErrors: ChannelFormErrors<UniversityFormValues> = {};
    if (!values.school.trim()) {
      nextErrors.school = '请填写学院名称。';
    }
    if (!values.studentId.trim()) {
      nextErrors.studentId = '请填写学籍号。';
    }
    if (!values.schoolEmail.trim()) {
      nextErrors.schoolEmail = '请填写学院邮箱。';
    } else if (!EMAIL_PATTERN.test(values.schoolEmail.trim())) {
      nextErrors.schoolEmail = '请输入有效的邮箱地址，建议使用 .edu.cn 学院邮箱。';
    }

    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) {
      return;
    }

    const ok = await submit({
      studentId: values.studentId.trim(),
      schoolEmail: values.schoolEmail.trim(),
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
          id="university-school"
          label="学院名称"
          required
          error={errors.school}
          className="sm:col-span-2"
        >
          <input
            id="university-school"
            value={values.school}
            onChange={(event) => update('school', event.target.value)}
            className={cn(
              inputClassName,
              errors.school && inputErrorClassName,
            )}
            placeholder="如：某某大学 网络空间安全学院"
            autoComplete="organization"
          />
        </FormField>

        <FormField
          id="university-student-id"
          label="学籍号"
          required
          error={errors.studentId}
          hint="仅用于学籍身份核验，不会公开展示。"
        >
          <input
            id="university-student-id"
            value={values.studentId}
            onChange={(event) => update('studentId', event.target.value)}
            className={cn(
              inputClassName,
              errors.studentId && inputErrorClassName,
            )}
            placeholder="请输入学籍号"
            autoComplete="off"
          />
        </FormField>

        <FormField
          id="university-email"
          label="学院邮箱"
          required
          error={errors.schoolEmail}
          hint="用于接收试炼结果与认证通知。"
        >
          <input
            id="university-email"
            type="email"
            value={values.schoolEmail}
            onChange={(event) => update('schoolEmail', event.target.value)}
            className={cn(
              inputClassName,
              errors.schoolEmail && inputErrorClassName,
            )}
            placeholder="name@university.edu.cn"
            autoComplete="email"
          />
        </FormField>
      </div>

      <FormField
        id="university-student-card"
        label="学员凭证上传"
        hint="演示占位：当前不会真正上传文件，仅记录文件名用于界面展示。"
      >
        <div className="rounded-lg border border-dashed border-arcane/40 bg-arcane/5 px-3 py-3">
          <input
            id="university-student-card"
            type="file"
            accept="image/*,application/pdf"
            className="sr-only"
            onChange={(event) =>
              update('studentCardName', event.target.files?.[0]?.name ?? '')
            }
          />
          <label
            htmlFor="university-student-card"
            className="flex cursor-pointer items-center gap-2 text-sm text-ink-muted transition-colors hover:text-arcane"
          >
            <Upload className="h-4 w-4 shrink-0 text-arcane" />
            <span className="truncate">
              {values.studentCardName ||
                '选择学生证照片或扫描件（演示占位）'}
            </span>
          </label>
        </div>
      </FormField>

      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit" disabled={submitting}>
          {submitting && <Loader2 className="h-4 w-4 animate-spin" />}
          {submitting ? '递交中…' : '递交学院试炼'}
        </Button>
        <span className="text-xs text-ink-dim">
          递交后进入试炼审核，认证状态以服务端返回为准。
        </span>
      </div>
    </form>
  );
}
