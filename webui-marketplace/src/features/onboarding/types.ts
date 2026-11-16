import type { RegistrationChannel } from '../../types/domain';

/** 注册提交后服务端返回的审核状态。 */
export type RegistrationReviewStatus = 'PENDING' | 'VERIFIED';

/** 页面级注册回执：通道 + 审核状态 + 本地提交时间。 */
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
