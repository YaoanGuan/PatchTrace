import type {
  AiToolCategory,
  AiToolGrade,
  AiToolStatus,
  ComplianceSeverity,
  RiskLevel,
  ValidationVerdict,
} from '../../types/domain';

/** 接入工具分类 */
export const AI_TOOL_CATEGORY_LABEL: Record<AiToolCategory, string> = {
  CLOUD_COMPUTER: '云托管计算机',
  AI_CLI: 'AI CLI',
  AI_IDE: 'AI IDE',
  CHAT_UI: '聊天客户端',
};

export const AI_TOOL_CATEGORY_ORDER: AiToolCategory[] = [
  'CLOUD_COMPUTER',
  'AI_CLI',
  'AI_IDE',
  'CHAT_UI',
];

/** 接入工具状态 */
export const AI_TOOL_STATUS_LABEL: Record<AiToolStatus, string> = {
  CONNECTED: '已连接',
  PENDING: '待接入',
  SANDBOX: '沙箱环境',
};

export const AI_TOOL_STATUS_CLASS: Record<AiToolStatus, string> = {
  CONNECTED: 'bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-600/20',
  PENDING: 'bg-amber-50 text-amber-700 ring-1 ring-inset ring-amber-600/20',
  SANDBOX: 'bg-sky-50 text-sky-700 ring-1 ring-inset ring-sky-600/20',
};

/** 信任等级 */
export const AI_TOOL_GRADE_LABEL: Record<AiToolGrade, string> = {
  S: 'S 级',
  A: 'A 级',
  B: 'B 级',
  C: 'C 级',
};

export const AI_TOOL_GRADE_CLASS: Record<AiToolGrade, string> = {
  S: 'bg-indigo-600 text-white',
  A: 'bg-emerald-600 text-white',
  B: 'bg-amber-500 text-white',
  C: 'bg-slate-500 text-white',
};

export const AI_TOOL_GRADES: AiToolGrade[] = ['S', 'A', 'B', 'C'];

/** 交叉验证判定 */
export const VERDICT_LABEL: Record<ValidationVerdict, string> = {
  PASS: '通过',
  FAIL: '未通过',
  PENDING: '待定',
};

export const VERDICT_CLASS: Record<ValidationVerdict, string> = {
  PASS: 'bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-600/20',
  FAIL: 'bg-rose-50 text-rose-700 ring-1 ring-inset ring-rose-600/20',
  PENDING: 'bg-slate-100 text-slate-600 ring-1 ring-inset ring-slate-500/20',
};

export const VERDICT_STEP_CLASS: Record<ValidationVerdict, string> = {
  PASS: 'border-emerald-200 bg-emerald-50/50',
  FAIL: 'border-rose-200 bg-rose-50/50',
  PENDING: 'border-slate-200 bg-slate-50/60',
};

/** 合规严重级别 */
export const SEVERITY_LABEL: Record<ComplianceSeverity, string> = {
  LOW: '低',
  MEDIUM: '中',
  HIGH: '高',
  CRITICAL: '严重',
};

export const SEVERITY_CLASS: Record<ComplianceSeverity, string> = {
  LOW: 'text-slate-600 bg-slate-100',
  MEDIUM: 'text-amber-700 bg-amber-50',
  HIGH: 'text-orange-700 bg-orange-50',
  CRITICAL: 'text-rose-700 bg-rose-50',
};

/** 风险级别 */
export const RISK_LEVEL_LABEL: Record<RiskLevel, string> = {
  LOW: '低风险',
  MEDIUM: '中风险',
  HIGH: '高风险',
  CRITICAL: '严重风险',
};

export const RISK_LEVEL_TEXT_CLASS: Record<RiskLevel, string> = {
  LOW: 'text-emerald-600',
  MEDIUM: 'text-amber-600',
  HIGH: 'text-orange-600',
  CRITICAL: 'text-rose-600',
};

export const RISK_LEVEL_BADGE_CLASS: Record<RiskLevel, string> = {
  LOW: 'bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-600/20',
  MEDIUM: 'bg-amber-50 text-amber-700 ring-1 ring-inset ring-amber-600/20',
  HIGH: 'bg-orange-50 text-orange-700 ring-1 ring-inset ring-orange-600/20',
  CRITICAL: 'bg-rose-50 text-rose-700 ring-1 ring-inset ring-rose-600/20',
};

export const RISK_LEVEL_SEGMENT_CLASS: Record<RiskLevel, string> = {
  LOW: 'bg-emerald-400',
  MEDIUM: 'bg-amber-400',
  HIGH: 'bg-orange-500',
  CRITICAL: 'bg-rose-600',
};

export const RISK_LEVEL_ORDER: RiskLevel[] = [
  'LOW',
  'MEDIUM',
  'HIGH',
  'CRITICAL',
];

/** 根据分数推导风险级别（后端字段缺失时的兜底） */
export function deriveRiskLevel(score: number): RiskLevel {
  if (score >= 75) {
    return 'CRITICAL';
  }
  if (score >= 50) {
    return 'HIGH';
  }
  if (score >= 25) {
    return 'MEDIUM';
  }
  return 'LOW';
}

/** 将数值裁剪到 [0, 100]，非数值返回 0 */
export function clampScore(value: number | undefined | null): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return 0;
  }
  return Math.min(100, Math.max(0, Math.round(value)));
}
