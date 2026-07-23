import type { Plugin } from 'vite';

type DevPersonProfileImageMockPluginOptions = {
	isEnabled: boolean;
};

type DevPersonProfileImageMockResponse = {
	status: number;
};

const participantProfileImagePath = /^\/calendar\/api\/participants\/[^/]+\/image$/;

export function createDevPersonProfileImageMockResponse(
	method: string,
	pathname: string
): DevPersonProfileImageMockResponse | undefined {
	if (method !== 'GET') return undefined;
	if (!participantProfileImagePath.test(pathname)) return undefined;
	return { status: 404 };
}

export function devPersonProfileImageMockPlugin(
	options: DevPersonProfileImageMockPluginOptions
): Plugin {
	return {
		name: 'internkim-dev-person-profile-image-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				const mockResponse = createDevPersonProfileImageMockResponse(
					request.method ?? 'GET',
					requestURL.pathname
				);
				if (!mockResponse) {
					next();
					return;
				}
				response.statusCode = mockResponse.status;
				response.end();
			});
		}
	};
}
