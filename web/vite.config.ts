import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, type Plugin } from 'vite';
import { createDevFlowSummary } from './src/routes/flow/dev-flow-fixture';

const admindTarget = 'http://127.0.0.1:18080';
const isMockFlowEnabled = process.env.VITE_MOCK_FLOW === '1';
const mockFlowUserEmail = process.env.VITE_DEV_USER_EMAIL ?? 'admin@example.com';

function mockFlowPlugin(): Plugin {
	return {
		name: 'internkim-dev-flow-mock',
		configureServer(server) {
			if (!isMockFlowEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				if (request.method !== 'GET' || requestURL.pathname !== '/flow/api/summary') {
					next();
					return;
				}

				const summary = createDevFlowSummary(requestURL.searchParams.get('week'), mockFlowUserEmail);
				response.statusCode = 200;
				response.setHeader('Content-Type', 'application/json');
				response.end(JSON.stringify(summary));
			});
		}
	};
}

export default defineConfig({
	plugins: [mockFlowPlugin(), tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/.well-known/caldav': admindTarget,
			'/admin/api': admindTarget,
			'/attendance/api': admindTarget,
			'/calendar/api': admindTarget,
			'/calendar/dav': admindTarget,
			'/calendar/ics': admindTarget,
			'/calendar/oauth': admindTarget,
			'/flow/api': admindTarget,
			'/mail/api': admindTarget
		}
	}
});
