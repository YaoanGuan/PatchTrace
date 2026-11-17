import { useRef, useState, type FormEvent } from 'react';
import { Loader2, Star } from 'lucide-react';
import { useUiStore } from '../../../app/store/uiStore';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { cn } from '../../../lib/cn';
import { submitRating } from '../../../lib/api/client';
import type { Rating, RatingDirection } from '../../../types/domain';
import {
  RATING_DIRECTION_HINTS,
  RATING_DIRECTION_LABELS,
  RATING_TAGS,
} from '../constants';
import type { RatingFormErrors } from '../types';
import { FormField, inputClassName, inputErrorClassName } from './FormField';

export interface RatingFormProps {
  direction: RatingDirection;
  /** 固定关联任务；不传时由用户在表单内填写。 */
  taskId?: string;
  onSubmitted?: (rating: Rating) => void;
  className?: string;
}

export function RatingForm({
  direction,
  taskId,
  onSubmitted,
  className,
}: RatingFormProps) {
  const pushToast = useUiStore((state) => state.pushToast);
  const [taskIdValue, setTaskIdValue] = useState(taskId ?? '');
  const [score, setScore] = useState(0);
  const [tags, setTags] = useState<string[]>([]);
  const [comment, setComment] = useState('');
  const [errors, setErrors] = useState<RatingFormErrors>({});
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);

  const availableTags = RATING_TAGS[direction];
  const resolvedTaskId = (taskId ?? taskIdValue).trim();

  function toggleTag(tag: string) {
    setTags((prev) =>
      prev.includes(tag) ? prev.filter((item) => item !== tag) : [...prev, tag],
    );
    setErrors((prev) => ({ ...prev, tags: undefined }));
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const nextErrors: RatingFormErrors = {};
    if (!resolvedTaskId) {
      nextErrors.taskId = '请填写关联副本 ID。';
    }
    if (score < 1) {
      nextErrors.score = '请选择 1–5 星战绩评分。';
    }
    if (tags.length === 0) {
      nextErrors.tags = '请至少选择一个战报标签。';
    }

    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) {
      return;
    }
    if (submittingRef.current) {
      return;
    }

    submittingRef.current = true;
    setSubmitting(true);
    try {
      const rating = await submitRating({
        taskId: resolvedTaskId,
        direction,
        score,
        tags,
        comment: comment.trim(),
      });

      pushToast({
        id: `rating-${rating.id}-${Date.now()}`,
        title: '战绩评分已提交',
        description: `${RATING_DIRECTION_LABELS[direction]} · ${score} 星`,
        variant: 'success',
      });
      onSubmitted?.(rating);

      setScore(0);
      setTags([]);
      setComment('');
      setErrors({});
      if (!taskId) {
        setTaskIdValue('');
      }
    } catch (error) {
      pushToast({
        id: `rating-error-${direction}-${Date.now()}`,
        title: '战绩评分提交失败',
        description:
          error instanceof Error ? error.message : '网络异常，请稍后重试。',
        variant: 'error',
      });
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  }

  return (
    <Card className={cn('p-5', className)}>
      <form noValidate onSubmit={handleSubmit} className="space-y-4">
        <div className="space-y-1">
          <h3 className="text-sm font-semibold text-ink">
            {RATING_DIRECTION_LABELS[direction]}
          </h3>
          <p className="text-xs text-ink-muted">
            {RATING_DIRECTION_HINTS[direction]}
          </p>
        </div>

        {!taskId && (
          <FormField
            id={`rating-task-${direction}`}
            label="关联副本 ID"
            required
            error={errors.taskId}
            hint="战绩评分会绑定到已通关副本，用于防止刷分。"
          >
            <input
              id={`rating-task-${direction}`}
              value={taskIdValue}
              onChange={(event) => {
                setTaskIdValue(event.target.value);
                setErrors((prev) => ({ ...prev, taskId: undefined }));
              }}
              className={cn(
                inputClassName,
                errors.taskId && inputErrorClassName,
              )}
              placeholder="如 task_1001"
              autoComplete="off"
            />
          </FormField>
        )}

        <div className="space-y-1.5">
          <span className="text-sm font-medium text-ink">
            战绩星级 <span className="text-gold">*</span>
          </span>
          <div className="flex flex-wrap items-center gap-1">
            {[1, 2, 3, 4, 5].map((value) => (
              <button
                key={value}
                type="button"
                aria-label={`战绩评分 ${value} 星`}
                aria-pressed={score === value}
                onClick={() => {
                  setScore(value);
                  setErrors((prev) => ({ ...prev, score: undefined }));
                }}
                className="rounded p-1 transition-transform hover:scale-110 focus:outline-none focus-visible:ring-2 focus-visible:ring-neon/40"
              >
                <Star
                  className={cn(
                    'h-7 w-7',
                    value <= score
                      ? 'fill-gold text-gold'
                      : 'text-ink-dim/50',
                  )}
                />
              </button>
            ))}
            <span className="ml-2 font-mono text-sm text-ink-muted">
              {score > 0 ? `${score}.0 星` : '未评战绩'}
            </span>
          </div>
          {errors.score && (
            <p role="alert" className="text-xs text-boss">
              {errors.score}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <span className="text-sm font-medium text-ink">
            战报标签 <span className="text-gold">*</span>
          </span>
          <div className="flex flex-wrap gap-2">
            {availableTags.map((tag) => {
              const selected = tags.includes(tag);
              return (
                <button
                  key={tag}
                  type="button"
                  aria-pressed={selected}
                  onClick={() => toggleTag(tag)}
                  className={cn(
                    'rounded-full px-3 py-1 text-xs font-medium ring-1 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-neon/40',
                    selected
                      ? 'bg-gold/10 text-gold ring-gold/40'
                      : 'bg-panel text-ink-muted ring-panel-line hover:text-ink',
                  )}
                >
                  {tag}
                </button>
              );
            })}
          </div>
          {errors.tags && (
            <p role="alert" className="text-xs text-boss">
              {errors.tags}
            </p>
          )}
        </div>

        <FormField
          id={`rating-comment-${direction}`}
          label="战报点评"
          hint="可填写具体事实与改进建议，最多 300 字。"
        >
          <textarea
            id={`rating-comment-${direction}`}
            value={comment}
            onChange={(event) => setComment(event.target.value)}
            maxLength={300}
            rows={3}
            className={cn(inputClassName, 'resize-y')}
            placeholder="例如：PoC 可复现，证据链完整，沟通响应及时。"
          />
        </FormField>
        <p className="-mt-2 text-right font-mono text-[11px] text-ink-dim">
          {comment.length}/300
        </p>

        <Button type="submit" disabled={submitting} className="w-full">
          {submitting && <Loader2 className="h-4 w-4 animate-spin" />}
          {submitting ? '提交中…' : '提交战绩评分'}
        </Button>
      </form>
    </Card>
  );
}
