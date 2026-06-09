import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';
import { devFlowMockPlugin } from './dev-flow-mock-plugin';

const admindTarget = 'http://127.0.0.1:18080';

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), '');
	return {
		plugins: [
			devFlowMockPlugin({
				isEnabled: env.VITE_MOCK_FLOW === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com'
			}),
			tailwindcss(),
			sveltekit()
		],
		server: {
			proxy: {
				'/.well-known/caldav': admindTarget,
				'/admin/api': admindTarget,
				'/attendance/api': admindTarget,
				'/auth': admindTarget,
				'/calendar/api': admindTarget,
				'/calendar/dav': admindTarget,
				'/calendar/ics': admindTarget,
				'/calendar/oauth': admindTarget,
				'/flow/api': admindTarget,
				'/mail/api': admindTarget,
				'/memory/api': admindTarget
			}
		}
	};
});
