import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv, type ProxyOptions } from 'vite';
import { devAttendanceMockPlugin } from './dev-attendance-mock-plugin';
import { devAdminOrganizationMockPlugin } from './dev-admin-organization-mock-plugin';
import { devAdminUsersMockPlugin } from './dev-admin-users-mock-plugin';
import { devFilesMockPlugin } from './dev-files-mock-plugin';
import { devTaskMockPlugin } from './dev-task-mock-plugin';
import { devMailMockPlugin } from './dev-mail-mock-plugin';
import { devMemoryMockPlugin } from './dev-memory-mock-plugin';
import { devPersonProfileImageMockPlugin } from './dev-person-profile-image-mock-plugin';
import { devTasksMockPlugin } from './dev-tasks-mock-plugin';
import type { DevAdminMockUserRole } from './dev-admin-mock';

const devUserRoles = new Set(['admin', 'operationsAdmin', 'member']);

function devUserRoleFromEnv(value: string | undefined): DevAdminMockUserRole {
	return devUserRoles.has(value ?? '') ? (value as DevAdminMockUserRole) : 'admin';
}

function admindProxy(target: string, devUserEmail?: string): ProxyOptions {
	const options: ProxyOptions = {
		target,
		xfwd: true
	};
	if (devUserEmail) {
		options.configure = (proxy) => {
			proxy.on('proxyReq', (proxyRequest) => {
				proxyRequest.setHeader('X-Forwarded-Email', devUserEmail);
			});
		};
	}
	return options;
}

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), '');
	const admindTarget = env.VITE_ADMIND_TARGET || 'http://127.0.0.1:18080';
	const devUserRole = devUserRoleFromEnv(env.VITE_DEV_USER_ROLE);
	const isAttendanceMockEnabled = env.VITE_MOCK_ATTENDANCE === '1';
	const isTaskMockEnabled = env.VITE_MOCK_TASK === '1';
	const isAdminMockEnabled = env.VITE_MOCK_ADMIN === '1' || env.VITE_MOCK_CRM === '1';
	const devUserEmail = env.VITE_DEV_USER_EMAIL;
	return {
		plugins: [
			devAdminUsersMockPlugin({
				isEnabled: isAdminMockEnabled,
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'kim@example.com'
			}),
			devAdminOrganizationMockPlugin({
				isEnabled: env.VITE_MOCK_ADMIN === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'kim@example.com',
				userRole: devUserRole
			}),
			devAttendanceMockPlugin({
				isEnabled: isAttendanceMockEnabled,
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'kim@example.com'
			}),
			devFilesMockPlugin({
				isEnabled: env.VITE_MOCK_FILES === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'kim@example.com'
			}),
			devPersonProfileImageMockPlugin({ isEnabled: isTaskMockEnabled }),
			devTaskMockPlugin({
				isEnabled: isAttendanceMockEnabled || isTaskMockEnabled,
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'kim@example.com'
			}),
			devTasksMockPlugin({
				isEnabled: env.VITE_MOCK_TASKS === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'kim@example.com'
			}),
			devMailMockPlugin({
				isEnabled: env.VITE_MOCK_MAIL === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'kim@example.com'
			}),
			devMemoryMockPlugin({
				isEnabled: env.VITE_MOCK_MEMORY === '1',
				userEmail: env.VITE_DEV_USER_EMAIL ?? 'kim@example.com'
			}),
			tailwindcss(),
			sveltekit()
		],
		server: {
			allowedHosts: env.VITE_ALLOWED_HOSTS ? env.VITE_ALLOWED_HOSTS.split(',') : undefined,
			proxy: {
				'/api/v1': admindProxy(admindTarget),
				'/admin/api': admindProxy(admindTarget),
				'/agent/api': admindProxy(admindTarget, devUserEmail),
				'/buzz/api': admindProxy(admindTarget, devUserEmail),
				'/attendance/api': admindProxy(admindTarget),
				'/auth': admindProxy(admindTarget, devUserEmail),
				'/calendar/api': admindProxy(admindTarget),
				'/crm/api': admindProxy(admindTarget, devUserEmail),
				'/task/api': admindProxy(admindTarget),
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
