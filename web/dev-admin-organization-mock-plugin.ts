import type { Plugin } from 'vite';
import {
	readRequestBody,
	writeJSON
} from './dev-admin-mock';
import {
	createDevAdminOrganizationMockResponse,
	createDevAdminOrganizationMockState,
	shouldHandleDevAdminOrganizationMockRequest
} from './dev-admin-organization-state';
import type { DevAdminMockUserRole } from './dev-admin-mock';

type DevAdminOrganizationMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
	userRole?: DevAdminMockUserRole;
};

export function devAdminOrganizationMockPlugin(options: DevAdminOrganizationMockPluginOptions): Plugin {
	const state = createDevAdminOrganizationMockState(options.userEmail, options.userRole);

	return {
		name: 'internkim-dev-admin-organization-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				const method = request.method ?? 'GET';
				if (!shouldHandleDevAdminOrganizationMockRequest(method, requestURL.pathname)) {
					next();
					return;
				}
				readRequestBody(request, (body) => {
					const mockResponse = createDevAdminOrganizationMockResponse(state, {
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
