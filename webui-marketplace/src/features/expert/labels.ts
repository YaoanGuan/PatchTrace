import type {
  AiToolCategory,
  AiToolGrade,
  AiToolStatus,
  ComplianceSeverity,
  RiskLevel,
  ValidationVerdict,
} from '../../types/domain';

/** 战友召唤图鉴：召唤通道分类（保留原英文枚举与后端数据）。 */
export const AI_TOOL_CATEGORY_LABEL: Record<AiToolCategory, string> = {
  CLOUD_COMPUTER: '云座驾',
  AI_CLI: '命令行召唤',
  AI_IDE: '锻造台',
  CHAT_UI: '传声筒',
};

/** 分类对应的原始接入说明，用于副标题与 tooltip，避免语义丢失。 */
export const AI_TOOL_CATEGORY_ORIGIN_LABEL: Record<AiToolCategory, string> = {
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

/** 契约状态：已缔结契约 / 待召唤 / 试炼场。 */
export const AI_TOOL_STATUS_LABEL: Record<AiToolStatus, string> = {
  CONNECTED: '已缔结契约',
  PENDING: '待召唤',
  SANDBOX: '试炼场',
};

export const AI_TOOL_STATUS_CLASS: Record<AiToolStatus, string> = {
  CONNECTED: 'bg-clear/10 text-clear ring-1 ring-inset ring-clear/30',
  PENDING: 'bg-amber-500/10 text-amber-300 ring-1 ring-inset ring-amber-500/30',
  SANDBOX: 'bg-neon/10 text-neon ring-1 ring-inset ring-neon/30',
};

/** 稀有度：传说 / 史诗 / 稀有 / 普通（保留 S/A/B/C 字母）。 */
export const AI_TOOL_GRADE_LABEL: Record<AiToolGrade, string> = {
  S: '传说 S',
  A: '史诗 A',
  B: '稀有 B',
  C: '普通 C',
};

export const AI_TOOL_GRADE_CLASS: Record<AiToolGrade, string> = {
  S: 'bg-gold/15 text-gold ring-1 ring-inset ring-gold/40',
  A: 'bg-arcane/15 text-arcane ring-1 ring-inset ring-arcane/40',
  B: 'bg-neon/15 text-neon ring-1 ring-inset ring-neon/40',
  C: 'bg-panel-soft text-ink-muted ring-1 ring-inset ring-panel-line',
};

export const AI_TOOL_GRADES: AiToolGrade[] = ['S', 'A', 'B', 'C'];

/** 双人连携判定。 */
export const VERDICT_LABEL: Record<ValidationVerdict, string> = {
  PASS: '通关',
  FAIL: '挑战失败',
  PENDING: '待定',
};

export const VERDICT_CLASS: Record<ValidationVerdict, string> = {
  PASS: 'bg-clear/10 text-clear ring-1 ring-inset ring-clear/30',
  FAIL: 'bg-boss/10 text-boss ring-1 ring-inset ring-boss/30',
  PENDING: 'bg-panel-soft text-ink-muted ring-1 ring-inset ring-panel-line',
};

export const VERDICT_STEP_CLASS: Record<ValidationVerdict, string> = {
  PASS: 'border-clear/30 bg-clear/10',
  FAIL: 'border-boss/30 bg-boss/10',
  PENDING: 'border-panel-line bg-panel-soft',
};

/** 副本规则扫描的严重级别。 */
export const SEVERITY_LABEL: Record<ComplianceSeverity, string> = {
  LOW: '低危',
  MEDIUM: '警戒',
  HIGH: '高危',
  CRITICAL: '灭团级',
};

export const SEVERITY_CLASS: Record<ComplianceSeverity, string> = {
  LOW: 'text-clear bg-clear/10',
  MEDIUM: 'text-amber-300 bg-amber-500/10',
  HIGH: 'text-orange-300 bg-orange-500/10',
  CRITICAL: 'text-boss bg-boss/15',
};

/** 大Boss 威胁值等级。 */
export const RISK_LEVEL_LABEL: Record<RiskLevel, string> = {
  LOW: '低',
  MEDIUM: '中',
  HIGH: '高',
  CRITICAL: '严重',
};

export const RISK_LEVEL_TEXT_CLASS: Record<RiskLevel, string> = {
  LOW: 'text-clear',
  MEDIUM: 'text-amber-300',
  HIGH: 'text-orange-300',
  CRITICAL: 'text-boss',
};

export const RISK_LEVEL_BADGE_CLASS: Record<RiskLevel, string> = {
  LOW: 'bg-clear/10 text-clear ring-1 ring-inset ring-clear/30',
  MEDIUM:
    'bg-amber-500/10 text-amber-300 ring-1 ring-inset ring-amber-500/30',
  HIGH:
    'bg-orange-500/10 text-orange-300 ring-1 ring-inset ring-orange-500/30',
  CRITICAL: 'bg-boss/15 text-boss ring-1 ring-inset ring-boss/40',
};

export const RISK_LEVEL_SEGMENT_CLASS: Record<RiskLevel, string> = {
  LOW: 'bg-clear',
  MEDIUM: 'bg-amber-400',
  HIGH: 'bg-orange-500',
  CRITICAL: 'bg-boss',
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
