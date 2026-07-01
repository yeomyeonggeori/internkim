import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';
import { devAttendanceMockPlugin } from './dev-attendance-mock-plugin';
import { devAdminOrgchartMockPlugin } from './dev-admin-orgchart-mock-plugin';
import { devFilesMockPlugin } from './dev-files-mock-plugin';
import { devFlowMockPlugin } from './dev-flow-mock-plugin';
import { devMailMockPlugin } from './dev-mail-mock-plugin';
import { devMemoryMockPlugin } from './dev-memory-mock-plugin';
import { devTasksMockPlugin } from './dev-tasks-mock-plugin';
import type { DevAdminMockUserRole } from './dev-admin-mock';

const admindTarget = 'http://127.0.0.1:18080';
const devUserRoles = new Set(['admin', 'operationsAdmin', 'member']);

function devUserRoleFromEnv(value: string | undefined): DevAdminMockUserRole {
	return devUserRoles.has(value ?? '') ? (value as DevAdminMockUserRole) : 'admin';
}

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), '');
	const devUserRole = devUserRoleFromEnv(env.VITE_DEV_USER_ROLE);
	return {
		plugins: [
			devAdminOrgchartMockPlugin({
				isEnabled: env.VITE_MOCK_ADMIN === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com',
				userRole: devUserRole
			}),
			devAttendanceMockPlugin({
				isEnabled: env.VITE_MOCK_ATTENDANCE === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'admin@example.com'
			}),
			devFilesMockPlugin({
				isEnabled: env.VITE_MOCK_FILES === '1',
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
		},
		build: {
			rollupOptions: {
				output: {
					manualChunks(moduleID) {
						if (moduleID.includes('/node_modules/temporal-polyfill/')) {
							return 'calendar-temporal';
						}
						if (moduleID.includes('/node_modules/@dayflow/core/') || moduleID.includes('/node_modules/@dayflow/plugin-drag/')) {
							return 'calendar-dayflow-core';
						}
						if (moduleID.includes('/node_modules/@dayflow/blossom-color-picker/')) {
							return 'calendar-dayflow-color';
						}
						if (moduleID.includes('/node_modules/@dayflow/svelte/')) {
							return 'calendar-dayflow-svelte';
						}
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
