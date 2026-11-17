import type { RegistrationChannel } from '../../types/domain';

/** 创建角色递交后服务端返回的试炼状态。 */
export type RegistrationReviewStatus = 'PENDING' | 'VERIFIED';

/** 页面级创建角色回执：通道 + 试炼状态 + 本地递交时间。 */
export interface RegistrationStatus {
  channel: RegistrationChannel;
  status: RegistrationReviewStatus;
  submittedAt: string;
}

export interface UniversityFormValues {
  school: string;
  studentId: string;
  schoolEmail: string;
  studentCardName: string;
}

export interface EnterpriseFormValues {
  companyName: string;
  workEmail: string;
  credential: string;
}

export interface FreelancerFormValues {
  realName: string;
  portfolioUrl: string;
  historyNote: string;
}

export interface RatingFormValues {
  taskId: string;
  score: number;
  tags: string[];
  comment: string;
}

export type ChannelFormErrors<T> = Partial<Record<keyof T, string>>;

export type RatingFormErrors = Partial<Record<keyof RatingFormValues, string>>;
