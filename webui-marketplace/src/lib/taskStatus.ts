import type { BadgeVariant } from '../components/ui/Badge';
import type { TaskStatus } from '../types/domain';

/**
 * 任务状态唯一权威中文标签（M3 悬赏大厅游戏化口径）。
 * 状态机与资金语义不变，仅统一游戏化展示文案，业务模块不得各自维护本地映射。
 */
export const TASK_STATUS_LABELS: Record<TaskStatus, string> = {
  DRAFT: '待发布',
  PUBLISHED: '悬赏中',
  ACCEPTED: '已接取',
  IN_PROGRESS: '副本进行中',
  SUBMITTED: '战利品已提交',
  AI_VALIDATING: '裁判复核中',
  VALIDATION_FAILED: '挑战失败',
  COMPLIANCE_SCAN: '规则扫描中',
  SETTLING: '掉落结算中',
  COMPLETED: '已通关',
  DISPUTED: '仲裁中',
  REFUNDED: '赏金退回',
  CANCELLED: '悬赏撤销',
};

/**
 * 任务状态唯一权威徽标色调，与标签同源维护。
 * 执行中/信息用 neon，失败/仲裁用 boss，通关用 clear。
 */
export const TASK_STATUS_VARIANTS: Record<TaskStatus, BadgeVariant> = {
  DRAFT: 'outline',
  PUBLISHED: 'info',
  ACCEPTED: 'info',
  IN_PROGRESS: 'info',
  SUBMITTED: 'info',
  AI_VALIDATING: 'info',
  VALIDATION_FAILED: 'danger',
  COMPLIANCE_SCAN: 'info',
  SETTLING: 'info',
  COMPLETED: 'success',
  DISPUTED: 'danger',
  REFUNDED: 'default',
  CANCELLED: 'default',
};
