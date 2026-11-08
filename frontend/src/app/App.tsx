import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createBrowserRouter, type DataRouter } from 'react-router';
import { routes } from './router';

const queryClient = new QueryClient();
const browserRouter = createBrowserRouter(routes);

export function App({ router = browserRouter, client = queryClient }: { router?: DataRouter; client?: QueryClient }) {
  return (
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
