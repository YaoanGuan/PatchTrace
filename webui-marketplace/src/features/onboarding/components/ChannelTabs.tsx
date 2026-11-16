import { Tabs } from '../../../components/ui/Tabs';
import type { RegistrationChannel } from '../../../types/domain';
import { CHANNEL_LABELS } from '../constants';

const CHANNELS: RegistrationChannel[] = [
  'UNIVERSITY',
  'ENTERPRISE',
  'FREELANCER',
];

export interface ChannelTabsProps {
  value: RegistrationChannel;
  onChange(channel: RegistrationChannel): void;
  className?: string;
}

export function ChannelTabs({ value, onChange, className }: ChannelTabsProps) {
  return (
    <Tabs
      className={className}
      value={value}
      items={CHANNELS.map((channel) => ({
        key: channel,
        label: CHANNEL_LABELS[channel],
      }))}
      onChange={(key) => onChange(key as RegistrationChannel)}
    />
  );
}
