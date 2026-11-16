import { useState } from 'react';
import { Send } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import { useTaskStore } from '../../../app/store/taskStore';
import { useUiStore } from '../../../app/store/uiStore';
import { cn } from '../../../lib/cn';
import { CATEGORY_OPTIONS, SKILL_OPTIONS, errorMessage, formatMoney } from '../utils';
import { ComplianceAck } from './ComplianceAck';
import { RewardInput } from './RewardInput';

const fieldClass =
  'w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 outline-none transition-colors placeholder:text-slate-400 focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100';

function toLocalInput(date: Date): string {
  const offset = date.getTimezoneOffset();
  return new Date(date.getTime() - offset * 60_000).toISOString().slice(0, 16);
}

function defaultDeadline(): string {
  return toLocalInput(new Date(Date.now() + 7 * 24 * 60 * 60 * 1000));
}

type FieldErrors = Partial<
  Record<
    'title' | 'description' | 'reward' | 'deadline' | 'criteria' | 'skills' | 'ack',
    string
  >
>;

export function TaskPublishForm() {
  const publishTask = useTaskStore((state) => state.publishTask);
  const fetchTasks = useTaskStore((state) => state.fetchTasks);
  const pushToast = useUiStore((state) => state.pushToast);

  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [category, setCategory] = useState('SECURITY');
  const [reward, setReward] = useState<number | undefined>(1200);
  const [deadline, setDeadline] = useState(defaultDeadline);
  const [criteriaText, setCriteriaText] = useState(
    '可复现脚本\n执行日志\n结论报告',
  );
  const [skills, setSkills] = useState<string[]>(['AGENT_ORCHESTRATION']);
  const [ack, setAck] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState<FieldErrors>({});

  const toggleSkill = (skill: string) => {
    setSkills((current) =>
      current.includes(skill)
        ? current.filter((item) => item !== skill)
        : [...current, skill],
    );
  };

  const validate = (): FieldErrors => {
    const next: FieldErrors = {};
    if (!title.trim()) {
      next.title = '请填写任务标题';
    }
    if (!description.trim()) {
      next.description = '请填写任务描述';
    }
    if (!reward || !Number.isFinite(reward) || reward <= 0) {
      next.reward = '赏金必须大于 0';
    }
    if (!deadline || Number.isNaN(new Date(deadline).getTime())) {
      next.deadline = '请选择有效的截止时间';
    }
    if (
      criteriaText
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean).length === 0
    ) {
      next.criteria = '至少填写一条验收标准';
    }
    if (skills.length === 0) {
      next.skills = '至少选择一个所需技能';
    }
    if (!ack) {
      next.ack = '发布前必须确认合规承诺';
    }
    return next;
  };

  const handleSubmit = async () => {
    const nextErrors = validate();
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) {
      pushToast({
        id: `publish-invalid-${Date.now()}`,
        title: '发布信息不完整',
        description: Object.values(nextErrors)[0],
        variant: 'warning',
      });
      return;
    }

    setSubmitting(true);
    try {
      const created = await publishTask({
        title: title.trim(),
        description: description.trim(),
        category,
        reward: { amount: reward as number, currency: 'CNY' },
        deadline: new Date(deadline).toISOString(),
        acceptanceCriteria: criteriaText
          .split('\n')
          .map((line) => line.trim())
          .filter(Boolean),
        requiredSkills: skills,
        complianceAck: ack,
      });

      pushToast({
        id: `publish-${created.id}`,
        title: '任务已发布',
        description: `赏金 ${formatMoney(created.reward.amount)} 已进入托管，等待 AI 指挥官接单。`,
        variant: 'success',
      });

      setTitle('');
      setDescription('');
      setReward(1200);
      setDeadline(defaultDeadline());
      setCriteriaText('可复现脚本\n执行日志\n结论报告');
      setSkills(['AGENT_ORCHESTRATION']);
      setAck(false);
      setErrors({});
      await fetchTasks({ page: 1 });
    } catch (error) {
      pushToast({
        id: `publish-error-${Date.now()}`,
        title: '任务发布失败',
        description: errorMessage(error),
        variant: 'error',
      });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="space-y-3 rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
      <div>
        <h2 className="text-sm font-semibold text-slate-900">发布新任务</h2>
        <p className="mt-0.5 text-xs text-slate-500">
          雇主视角：明确目标、验收标准与所需技能，发布后赏金进入平台托管。
        </p>
      </div>

      <div className="space-y-1">
        <label className="text-xs font-medium text-slate-600" htmlFor="task-title">
          任务标题
        </label>
        <input
          id="task-title"
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="例如：为一款 IoT 固件编写协议模糊测试 Agent"
          className={cn(fieldClass, errors.title && 'border-rose-300')}
        />
        {errors.title && <p className="text-xs text-rose-600">{errors.title}</p>}
      </div>

      <div className="space-y-1">
        <label
          className="text-xs font-medium text-slate-600"
          htmlFor="task-description"
        >
          任务描述
        </label>
        <textarea
          id="task-description"
          rows={3}
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          placeholder="目标、范围、授权边界与可交付物"
          className={cn(fieldClass, errors.description && 'border-rose-300')}
        />
        {errors.description && (
          <p className="text-xs text-rose-600">{errors.description}</p>
        )}
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-1">
          <label className="text-xs font-medium text-slate-600" htmlFor="task-category">
            任务类别
          </label>
          <select
            id="task-category"
            value={category}
            onChange={(event) => setCategory(event.target.value)}
            className={fieldClass}
          >
            {CATEGORY_OPTIONS.filter((option) => option.value !== 'ALL').map(
              (option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ),
            )}
          </select>
        </div>

        <div className="space-y-1">
          <label className="text-xs font-medium text-slate-600" htmlFor="task-deadline">
            截止时间
          </label>
          <input
            id="task-deadline"
            type="datetime-local"
            value={deadline}
            onChange={(event) => setDeadline(event.target.value)}
            className={cn(fieldClass, errors.deadline && 'border-rose-300')}
          />
          {errors.deadline && (
            <p className="text-xs text-rose-600">{errors.deadline}</p>
          )}
        </div>
      </div>

      <RewardInput
        value={reward}
        onChange={setReward}
        error={errors.reward}
        disabled={submitting}
      />

      <div className="space-y-1">
        <label className="text-xs font-medium text-slate-600" htmlFor="task-criteria">
          验收标准（每行一条）
        </label>
        <textarea
          id="task-criteria"
          rows={3}
          value={criteriaText}
          onChange={(event) => setCriteriaText(event.target.value)}
          className={cn(fieldClass, errors.criteria && 'border-rose-300')}
        />
        {errors.criteria && (
          <p className="text-xs text-rose-600">{errors.criteria}</p>
        )}
      </div>

      <div className="space-y-1">
        <span className="text-xs font-medium text-slate-600">所需技能</span>
        <div className="flex flex-wrap gap-1.5">
          {SKILL_OPTIONS.map((skill) => {
            const active = skills.includes(skill.value);
            return (
              <button
                key={skill.value}
                type="button"
                onClick={() => toggleSkill(skill.value)}
                className={cn(
                  'rounded-full border px-2.5 py-1 text-xs font-medium transition-colors',
                  active
                    ? 'border-indigo-500 bg-indigo-50 text-indigo-700'
                    : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300',
                )}
              >
                {skill.label}
              </button>
            );
          })}
        </div>
        {errors.skills && (
          <p className="text-xs text-rose-600">{errors.skills}</p>
        )}
      </div>

      <ComplianceAck checked={ack} onChange={setAck} disabled={submitting} />
      {errors.ack && <p className="text-xs text-rose-600">{errors.ack}</p>}

      <Button
        className="w-full"
        onClick={() => void handleSubmit()}
        disabled={submitting}
      >
        <Send className="h-4 w-4" />
        {submitting ? '发布中…' : '发布任务并托管赏金'}
      </Button>
    </div>
  );
}
