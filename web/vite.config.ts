import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv, type ProxyOptions } from 'vite';
import { devAttendanceMockPlugin } from './dev-attendance-mock-plugin';
import { devCalendarMockPlugin } from './dev-calendar-mock-plugin';
import { devAdminOrganizationMockPlugin } from './dev-admin-organization-mock-plugin';
import { devAdminUsersMockPlugin } from './dev-admin-users-mock-plugin';
import { devFilesMockPlugin } from './dev-files-mock-plugin';
import { devFlowMockPlugin } from './dev-flow-mock-plugin';
import { devMailMockPlugin } from './dev-mail-mock-plugin';
import { devMemoryMockPlugin } from './dev-memory-mock-plugin';
import { devPersonProfileImageMockPlugin } from './dev-person-profile-image-mock-plugin';
import { devTasksMockPlugin } from './dev-tasks-mock-plugin';
import type { DevAdminMockUserRole } from './dev-admin-mock';

const devUserRoles = new Set(['admin', 'operationsAdmin', 'member']);

function devUserRoleFromEnv(value: string | undefined): DevAdminMockUserRole {
	return devUserRoles.has(value ?? '') ? (value as DevAdminMockUserRole) : 'admin';
}

function admindProxy(target: string): ProxyOptions {
	return {
		target,
		xfwd: true
	};
}

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), '');
	const admindTarget = env.VITE_ADMIND_TARGET || 'http://127.0.0.1:18080';
	const devUserRole = devUserRoleFromEnv(env.VITE_DEV_USER_ROLE);
	const isAttendanceMockEnabled = env.VITE_MOCK_ATTENDANCE === '1';
	const isFlowMockEnabled = env.VITE_MOCK_FLOW === '1';
	return {
		plugins: [
			devAdminUsersMockPlugin({
				isEnabled: env.VITE_MOCK_ADMIN === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com'
			}),
			devAdminOrganizationMockPlugin({
				isEnabled: env.VITE_MOCK_ADMIN === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com',
				userRole: devUserRole
			}),
			devAttendanceMockPlugin({
				isEnabled: isAttendanceMockEnabled,
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com'
			}),
			devCalendarMockPlugin({
				isEnabled: isAttendanceMockEnabled || env.VITE_MOCK_CALENDAR === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com'
			}),
			devFilesMockPlugin({
				isEnabled: env.VITE_MOCK_FILES === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com'
			}),
			devPersonProfileImageMockPlugin({ isEnabled: isFlowMockEnabled }),
			devFlowMockPlugin({
				isEnabled: isAttendanceMockEnabled || isFlowMockEnabled,
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com'
			}),
			devTasksMockPlugin({
				isEnabled: env.VITE_MOCK_TASKS === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com'
			}),
			devMailMockPlugin({
				isEnabled: env.VITE_MOCK_MAIL === '1',
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
				'/.well-known/caldav': admindProxy(admindTarget),
				'/api/v1': admindProxy(admindTarget),
				'/admin/api': admindProxy(admindTarget),
				'/attendance/api': admindProxy(admindTarget),
				'/auth': admindProxy(admindTarget),
				'/calendar/api': admindProxy(admindTarget),
				'/calendar/dav': admindProxy(admindTarget),
				'/calendar/ics': admindProxy(admindTarget),
				'/calendar/oauth': admindProxy(admindTarget),
				'/flow/api': admindProxy(admindTarget),
				'/mail/api': admindProxy(admindTarget),
				'/memory/api': admindProxy(admindTarget),
				'/organization/api': admindProxy(admindTarget)
			}
		},
		build: {
			rollupOptions: {
				output: {
					manualChunks(moduleID) {
						if (moduleID.includes('/node_modules/preact/')) {
							return 'calendar-preact';
						}
						if (moduleID.includes('/node_modules/svelte/')) {
							return 'svelte-runtime';
						}
					}
				}
			}
		}
	};
});
