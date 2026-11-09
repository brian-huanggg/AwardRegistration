import { useTranslation } from 'react-i18next';
import { useDeadline } from '@/features/deadline/useDeadline';
import { JoinRequestsPanel } from '@/features/team/JoinRequests';
import { PageHeading } from '../layouts';

export function JoinRequests() {
  const { t } = useTranslation();
  const deadline = useDeadline();
  return (
    <>
      <PageHeading title={t('nav.joinRequests')} />
      <JoinRequestsPanel closed={deadline.data !== undefined && deadline.data.state !== 'OPEN'} />
    </>
  );
}
