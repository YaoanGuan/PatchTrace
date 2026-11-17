import type { Channel, RatingDirection } from '../../types/domain';
import type { RegistrationReviewStatus } from './types';

export const CHANNEL_LABELS: Record<Channel, string> = {
  UNIVERSITY: '学院试炼（高校学生）',
  ENTERPRISE: '公会担保（企业白名单）',
  FREELANCER: '游侠自荐（自由职业者）',
};

export const CHANNEL_SHORT_LABELS: Record<Channel, string> = {
  UNIVERSITY: '学院试炼',
  ENTERPRISE: '公会担保',
  FREELANCER: '游侠自荐',
};

export const CHANNEL_DESCRIPTIONS: Record<Channel, string> = {
  UNIVERSITY:
    '面向在校学生的学院试炼。使用学籍号与学院邮箱完成身份核验，降低陌生人协作的初始信任成本。',
  ENTERPRISE:
    '面向安全企业与合作实验室的公会担保。通过企业域名邮箱与工牌 / 统一社会信用代码进入白名单。',
  FREELANCER:
    '面向独立研究者与自由职业者的游侠自荐。以实名、作品集与历史战报说明进入人工审核。',
};

export const REVIEW_STATUS_LABELS: Record<RegistrationReviewStatus, string> = {
  PENDING: '试炼审核中',
  VERIFIED: '试炼通过',
};

export const RATING_DIRECTION_LABELS: Record<RatingDirection, string> = {
  EMPLOYER_TO_COMMANDER: '雇主 → AI 指挥官',
  COMMANDER_TO_EMPLOYER: 'AI 指挥官 → 雇主',
};

export const RATING_DIRECTION_HINTS: Record<RatingDirection, string> = {
  EMPLOYER_TO_COMMANDER: '点评交付质量、证据完整度与沟通协作。',
  COMMANDER_TO_EMPLOYER: '点评需求清晰度、验收合理性与结算及时性。',
};

export const RATING_TAGS: Record<RatingDirection, string[]> = {
  EMPLOYER_TO_COMMANDER: [
    '交付准时',
    '证据完整',
    '沟通顺畅',
    '文档清晰',
    '成本可控',
    '超出预期',
  ],
  COMMANDER_TO_EMPLOYER: [
    '需求清晰',
    '验收合理',
    '结算及时',
    '授权完整',
    '沟通高效',
    '尊重专业',
  ],
};
