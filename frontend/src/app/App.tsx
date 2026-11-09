import { useCallback, useEffect } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createBrowserRouter, type DataRouter } from 'react-router';
import { setUnauthenticatedHandler } from '@/shared/api';
import { routes } from './router';
import { IdleWarning } from '@/features/auth/IdleWarning';
import { handleSessionEnded, useMe } from './session';

const queryClient = new QueryClient();
const browserRouter = createBrowserRouter(routes);

/** Mounted inside the providers so it can read `me`; ends the session like a 401 when the idle time is up. */
function Idle({ router, client }: { router: DataRouter; client: QueryClient }) {
  const { data: me } = useMe();
  const onExpire = useCallback(() => handleSessionEnded(client, router), [client, router]);
  return <IdleWarning signedIn={!!me} onExpire={onExpire} />;
}

export function App({ router = browserRouter, client = queryClient }: { router?: DataRouter; client?: QueryClient }) {
  useEffect(() => setUnauthenticatedHandler(() => handleSessionEnded(client, router)), [client, router]);
  return (
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Idle router={router} client={client} />
    </QueryClientProvider>
  );
}
