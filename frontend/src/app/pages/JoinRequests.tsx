import { useTranslation } from 'react-i18next';
import { useDeadline } from '@/features/deadline/useDeadline';
import { JoinRequestsPanel } from '@/features/team/JoinRequests';
import { PageHeading } from '../layouts';

export function JoinRequests() {
  const { t } = useTranslation();
  const deadline = useDeadline();
  return (
    <>
      <div className="space-y-1">
        <PageHeading title={t('nav.joinRequests')} />
        <p className="text-sm text-muted-foreground">{t('pages.joinRequests.description')}</p>
      </div>
      <JoinRequestsPanel closed={deadline.data !== undefined && deadline.data.state !== 'OPEN'} serverTime={deadline.data?.serverTime} />
    </>
  );
}
