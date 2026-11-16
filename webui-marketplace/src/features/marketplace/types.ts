import type { EscrowStatus } from '../../types/domain';

/** 任务类别选项（仅前端筛选使用，API 查询参数不含 category）。 */
export interface CategoryOption {
  value: string;
  label: string;
}

/** 技能选项（发布表单多选）。 */
export interface SkillOption {
  value: string;
  label: string;
}

/** 托管任务行：由钱包流水按 taskId 聚合而来。 */
export interface EscrowRowModel {
  taskId: string;
  amount: number;
  state: EscrowStatus;
  at?: string;
}
