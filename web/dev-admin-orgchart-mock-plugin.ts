import type { Plugin } from 'vite';
import {
	readRequestBody,
	writeJSON
} from './dev-admin-mock';
import {
	createDevAdminOrgchartMockResponse,
	createDevAdminOrgchartMockState,
	shouldHandleDevAdminOrgchartMockRequest
} from './dev-admin-orgchart-state';

type DevAdminOrgchartMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

export function devAdminOrgchartMockPlugin(options: DevAdminOrgchartMockPluginOptions): Plugin {
	const state = createDevAdminOrgchartMockState(options.userEmail);

	return {
		name: 'internkim-dev-admin-orgchart-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				const method = request.method ?? 'GET';
				if (!shouldHandleDevAdminOrgchartMockRequest(method, requestURL.pathname)) {
					next();
					return;
				}
				readRequestBody(request, (body) => {
					const mockResponse = createDevAdminOrgchartMockResponse(state, {
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
