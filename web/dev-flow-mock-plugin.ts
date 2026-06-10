import type { Plugin } from 'vite';
import type { IncomingMessage, ServerResponse } from 'node:http';
import { createDevFlowState, createDevFlowWeeklySummary } from './src/routes/flow/dev-flow-fixture';

type DevFlowMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

export function devFlowMockPlugin(options: DevFlowMockPluginOptions): Plugin {
	let locale = 'ko';

	return {
		name: 'internkim-dev-flow-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				if (request.method === 'GET' && requestURL.pathname === '/flow/api/summary') {
					const summary = createDevFlowWeeklySummary(requestURL.searchParams.get('week'));
					writeJSON(response, summary);
					return;
				}
				if (request.method === 'GET' && requestURL.pathname === '/flow/api/state') {
					writeJSON(response, createDevFlowState(options.userEmail));
					return;
				}
				if (request.method === 'GET' && requestURL.pathname === '/auth/session') {
					writeJSON(response, { authenticated: true, email: options.userEmail, isAdmin: true });
					return;
				}
				if (request.method === 'GET' && requestURL.pathname === '/admin/api/session') {
					writeJSON(response, {
						email: options.userEmail,
						claimedAdminEmail: options.userEmail,
						isAdmin: true,
						isClaimed: true,
						bootstrapStatus: 'claimed'
					});
					return;
				}
				if (request.method === 'GET' && requestURL.pathname === '/admin/api/locale') {
					writeJSON(response, { locale });
					return;
				}
				if (request.method === 'PUT' && requestURL.pathname === '/admin/api/locale') {
					readLocaleRequest(request, (nextLocale) => {
						locale = nextLocale;
						writeJSON(response, { locale });
					});
					return;
				}
				next();
			});
		}
	};
}

function readLocaleRequest(request: IncomingMessage, callback: (locale: 'ko' | 'en') => void) {
	let body = '';
	request.on('data', (chunk: Buffer) => {
		body += chunk.toString('utf8');
	});
	request.on('end', () => {
		try {
			const payload = JSON.parse(body) as { locale?: string };
			callback(payload.locale === 'en' ? 'en' : 'ko');
		} catch {
			callback('ko');
		}
	});
	request.on('error', () => callback('ko'));
}

function writeJSON(response: ServerResponse, body: unknown) {
	response.statusCode = 200;
	response.setHeader('Content-Type', 'application/json');
	response.end(JSON.stringify(body));
}
