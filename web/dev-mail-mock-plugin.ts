import type { Plugin } from 'vite';
import { readRequestBody, writeJSON } from './dev-admin-mock';
import { createDevMailMockResponse, shouldHandleDevMailMockRequest } from './dev-mail-mock-response';
import { createDevMailMockState } from './dev-mail-mock-state';

type DevMailMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

export { createDevMailMockResponse } from './dev-mail-mock-response';
export { createDevMailMockState, type DevMailMockState } from './dev-mail-mock-state';

export function devMailMockPlugin(options: DevMailMockPluginOptions): Plugin {
	const state = createDevMailMockState(options.userEmail);

	return {
		name: 'internkim-dev-mail-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				const method = request.method ?? 'GET';
				if (!shouldHandleDevMailMockRequest(method, requestURL.pathname)) {
					next();
					return;
				}
				readRequestBody(request, (body) => {
					const mockResponse = createDevMailMockResponse(state, {
						method,
						pathname: requestURL.pathname,
						searchParams: requestURL.searchParams,
						body
					});
					if (!mockResponse) {
						next();
						return;
					}
					writeJSON(response, mockResponse.status, mockResponse.body);
				});
			});
		}
	};
}
