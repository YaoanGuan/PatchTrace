import type { BadgeVariant } from '../components/ui/Badge';
import type { TaskStatus } from '../types/domain';

/**
 * 任务状态唯一权威中文标签。
 * PUBLISHED 的产品口径统一为「招募中」，不得在业务模块内各自维护本地映射。
 */
export const TASK_STATUS_LABELS: Record<TaskStatus, string> = {
  DRAFT: '草稿',
  PUBLISHED: '招募中',
  ACCEPTED: '已接单',
  IN_PROGRESS: '执行中',
  SUBMITTED: '已提交',
  AI_VALIDATING: 'AI 复核中',
  VALIDATION_FAILED: '复核未通过',
  COMPLIANCE_SCAN: '合规扫描中',
  SETTLING: '结算中',
  COMPLETED: '已完成',
  DISPUTED: '争议中',
  REFUNDED: '已退款',
  CANCELLED: '已取消',
};

/** 任务状态唯一权威徽标色调，与标签同源维护。 */
export const TASK_STATUS_VARIANTS: Record<TaskStatus, BadgeVariant> = {
  DRAFT: 'outline',
  PUBLISHED: 'info',
  ACCEPTED: 'warning',
  IN_PROGRESS: 'warning',
  SUBMITTED: 'info',
  AI_VALIDATING: 'info',
  VALIDATION_FAILED: 'danger',
  COMPLIANCE_SCAN: 'warning',
  SETTLING: 'warning',
  COMPLETED: 'success',
  DISPUTED: 'danger',
  REFUNDED: 'default',
  CANCELLED: 'default',
};
