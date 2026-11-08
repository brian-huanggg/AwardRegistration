import { setupServer } from 'msw/node';

/** Shared MSW server for unit/component tests; handlers are added per test with server.use(). */
export const server = setupServer();
