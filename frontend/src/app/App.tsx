import { useEffect } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createBrowserRouter, type DataRouter } from 'react-router';
import { setUnauthenticatedHandler } from '@/shared/api';
import { routes } from './router';
import { handleSessionEnded } from './session';

const queryClient = new QueryClient();
const browserRouter = createBrowserRouter(routes);

export function App({ router = browserRouter, client = queryClient }: { router?: DataRouter; client?: QueryClient }) {
  useEffect(() => setUnauthenticatedHandler(() => handleSessionEnded(client, router)), [client, router]);
  return (
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
