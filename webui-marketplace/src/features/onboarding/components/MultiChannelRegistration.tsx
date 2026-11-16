import { useState } from 'react';
import { Card } from '../../../components/ui/Card';
import { cn } from '../../../lib/cn';
import type { RegistrationChannel } from '../../../types/domain';
import { CHANNEL_DESCRIPTIONS } from '../constants';
import type { RegistrationStatus } from '../types';
import { ChannelTabs } from './ChannelTabs';
import { EnterpriseWhitelistForm } from './EnterpriseWhitelistForm';
import { FreelancerForm } from './FreelancerForm';
import { UniversityVerificationForm } from './UniversityVerificationForm';

export interface MultiChannelRegistrationProps {
  onRegistered?: (status: RegistrationStatus) => void;
  className?: string;
}

export function MultiChannelRegistration({
  onRegistered,
  className,
}: MultiChannelRegistrationProps) {
  const [channel, setChannel] = useState<RegistrationChannel>('UNIVERSITY');

  return (
    <Card className={cn('space-y-5 p-5 sm:p-6', className)}>
      <div className="space-y-1">
        <h2 className="text-lg font-semibold text-slate-900">
          多通道准入注册
        </h2>
        <p className="max-w-3xl text-sm text-slate-500">
          选择与你的身份匹配的通道提交认证材料。资料提交后进入平台审核，
          认证状态以服务端返回结果为准，不会跳过审核直接生效。
        </p>
      </div>

      <div className="space-y-3">
        <ChannelTabs value={channel} onChange={setChannel} />
        <p className="text-xs text-slate-500">{CHANNEL_DESCRIPTIONS[channel]}</p>
      </div>

      <div className="border-t border-slate-100 pt-5">
        {channel === 'UNIVERSITY' && (
          <UniversityVerificationForm onRegistered={onRegistered} />
        )}
        {channel === 'ENTERPRISE' && (
          <EnterpriseWhitelistForm onRegistered={onRegistered} />
        )}
        {channel === 'FREELANCER' && (
          <FreelancerForm onRegistered={onRegistered} />
        )}
      </div>
    </Card>
  );
}
