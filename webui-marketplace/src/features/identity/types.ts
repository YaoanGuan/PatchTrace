import type { Channel, ExpertiseKind, UserRole } from '../../types/domain';

/** 技能图鉴 Tab：直接复用领域层的 ExpertiseKind，但只暴露三个可展示分类。 */
export type ExpertiseTabKey = Extract<
  ExpertiseKind,
  'AGENT' | 'DATA_ASSET' | 'PROMPT_TEMPLATE'
>;

export interface ExpertiseTabItem {
  key: ExpertiseTabKey;
  label: string;
}

export const EXPERTISE_TABS: ExpertiseTabItem[] = [
  { key: 'AGENT', label: '战友技能' },
  { key: 'DATA_ASSET', label: '战利品档案' },
  { key: 'PROMPT_TEMPLATE', label: '咒语模板' },
];

/** 凭证上传弹窗中的本地凭证类型（不接真实后端）。 */
export type CredentialType =
  | 'STUDENT_ID'
  | 'SCHOOL_EMAIL'
  | 'WORK_BADGE'
  | 'ENTERPRISE_LICENSE';

export interface CredentialTypeOption {
  value: CredentialType;
  label: string;
  hint: string;
}

export const CREDENTIAL_TYPE_OPTIONS: CredentialTypeOption[] = [
  {
    value: 'STUDENT_ID',
    label: '学籍 / 学生证',
    hint: '学院试炼 · 学籍号或学生证照片',
  },
  {
    value: 'SCHOOL_EMAIL',
    label: '学校邮箱',
    hint: '学院试炼 · 使用 .edu 域名邮箱认证',
  },
  {
    value: 'WORK_BADGE',
    label: '企业工牌',
    hint: '公会担保 · 工牌或在职证明',
  },
  {
    value: 'ENTERPRISE_LICENSE',
    label: '企业资质',
    hint: '公会担保 · 统一社会信用代码等材料',
  },
];

export const CHANNEL_LABELS: Record<Channel, string> = {
  UNIVERSITY: '学院试炼',
  ENTERPRISE: '公会担保',
  FREELANCER: '游侠自荐',
};

export const ROLE_LABELS: Record<UserRole, string> = {
  EMPLOYER: '雇主',
  COMMANDER: 'AI 指挥官',
};

export interface LevelInfo {
  code: string;
  title: string;
  /** 当前等级内的进度，0–100 */
  progress: number;
  /** 距离下一等级还需通关的副本数；已满级为 0 */
  remaining: number;
  nextCode: string | null;
}

interface LevelThreshold {
  min: number;
  code: string;
  title: string;
}

const LEVEL_THRESHOLDS: LevelThreshold[] = [
  { min: 0, code: 'L1', title: '见习指挥官' },
  { min: 10, code: 'L2', title: '初级指挥官' },
  { min: 20, code: 'L3', title: '认证指挥官' },
  { min: 35, code: 'L4', title: '资深指挥官' },
  { min: 50, code: 'L5', title: '首席指挥官' },
];

/**
 * 由真实通关副本数推导指挥官等级。
 * 缺失或非法输入按 0 处理，保证页面不崩溃。
 */
export function deriveLevel(
  completedTasksRaw: number | null | undefined,
): LevelInfo {
  const completedTasks =
    typeof completedTasksRaw === 'number' && Number.isFinite(completedTasksRaw)
      ? Math.max(0, Math.floor(completedTasksRaw))
      : 0;

  let index = 0;
  for (let cursor = 0; cursor < LEVEL_THRESHOLDS.length; cursor += 1) {
    if (completedTasks >= LEVEL_THRESHOLDS[cursor].min) {
      index = cursor;
    }
  }

  const current = LEVEL_THRESHOLDS[index];
  const next = LEVEL_THRESHOLDS[index + 1];

  if (!next) {
    return {
      code: current.code,
      title: current.title,
      progress: 100,
      remaining: 0,
      nextCode: null,
    };
  }

  const span = next.min - current.min;
  const progress =
    span > 0
      ? Math.round(((completedTasks - current.min) / span) * 100)
      : 100;

  return {
    code: current.code,
    title: current.title,
    progress: Math.max(0, Math.min(100, progress)),
    remaining: Math.max(0, next.min - completedTasks),
    nextCode: next.code,
  };
}

/** 把 ISO 时间裁剪为 YYYY-MM-DD；非法输入回退为原始字符串的前 10 位。 */
export function formatShortDate(iso?: string): string {
  if (!iso) {
    return '未知时间';
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return iso.slice(0, 10) || '未知时间';
  }
  return date.toISOString().slice(0, 10);
}
