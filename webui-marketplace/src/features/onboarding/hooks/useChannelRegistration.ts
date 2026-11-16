import { useRef, useState } from 'react';
import { useUiStore } from '../../../app/store/uiStore';
import { registerChannel } from '../../../lib/api/client';
import type { RegisterChannelResponse, RegisterEvidence } from '../../../types/api';
import type { RegistrationChannel } from '../../../types/domain';
import { CHANNEL_LABELS } from '../constants';
import type { RegistrationStatus } from '../types';

export interface UseChannelRegistrationResult {
  submitting: boolean;
  submit(evidence: RegisterEvidence): Promise<boolean>;
}

/**
 * 三通道注册共用提交流程：调用 API、同步本地审核状态、统一 toast。
 * 认证状态只回传给页面本地，不写回 sessionStore，避免污染其他模块。
 */
export function useChannelRegistration(
  channel: RegistrationChannel,
  onRegistered?: (status: RegistrationStatus) => void,
): UseChannelRegistrationResult {
  const pushToast = useUiStore((state) => state.pushToast);
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);

  async function submit(evidence: RegisterEvidence): Promise<boolean> {
    // 阻止连续点击 / 回车触发的重复提交。
    if (submittingRef.current) {
      return false;
    }

    submittingRef.current = true;
    setSubmitting(true);
    try {
      const result: RegisterChannelResponse = await registerChannel({
        channel,
        evidence,
      });

      const status: RegistrationStatus = {
        channel: result.channel,
        status: result.status,
        submittedAt: new Date().toISOString(),
      };

      onRegistered?.(status);
      pushToast({
        id: `registration-${result.channel}-${Date.now()}`,
        title:
          result.status === 'VERIFIED'
            ? `${CHANNEL_LABELS[result.channel]}认证已通过`
            : `${CHANNEL_LABELS[result.channel]}资料已提交`,
        description:
          result.status === 'VERIFIED'
            ? '服务端已返回认证通过；平台保留后续复核与补充材料的权利。'
            : '资料已进入人工审核，预计 1–3 个工作日完成，请留意站内通知。',
        variant: result.status === 'VERIFIED' ? 'success' : 'info',
      });

      return true;
    } catch (error) {
      pushToast({
        id: `registration-error-${channel}-${Date.now()}`,
        title: '注册提交失败',
        description:
          error instanceof Error ? error.message : '网络异常，请稍后重试。',
        variant: 'error',
      });
      return false;
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  }

  return { submitting, submit };
}
