import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';
import { devAttendanceMockPlugin } from './dev-attendance-mock-plugin';
import { devFlowMockPlugin } from './dev-flow-mock-plugin';
import { devMemoryMockPlugin } from './dev-memory-mock-plugin';
import { devTasksMockPlugin } from './dev-tasks-mock-plugin';

const admindTarget = 'http://127.0.0.1:18080';

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), '');
	return {
		plugins: [
			devAttendanceMockPlugin({
				isEnabled: env.VITE_MOCK_ATTENDANCE === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com'
			}),
			devFlowMockPlugin({
				isEnabled: env.VITE_MOCK_FLOW === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com'
			}),
			devTasksMockPlugin({
				isEnabled: env.VITE_MOCK_TASKS === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com'
			}),
			devMemoryMockPlugin({
				isEnabled: env.VITE_MOCK_MEMORY === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com'
			}),
			tailwindcss(),
			sveltekit()
		],
		server: {
			proxy: {
				'/.well-known/caldav': admindTarget,
				'/api/v1': admindTarget,
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
