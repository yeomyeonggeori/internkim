import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv, type ProxyOptions } from 'vite';
import { devFilesMockPlugin } from './dev-files-mock-plugin';
import { devMailMockPlugin } from './dev-mail-mock-plugin';
import { devMemoryMockPlugin } from './dev-memory-mock-plugin';
import { devTasksMockPlugin } from './dev-tasks-mock-plugin';
import type { DevAdminMockUserRole } from './dev-admin-mock';
import { isMemberRole } from './src/lib/member-vocabulary';

function devUserRoleFromEnv(value: string | undefined): DevAdminMockUserRole {
	return isMemberRole(value) ? value : 'admin';
}

function admindProxy(target: string, devUserEmail?: string): ProxyOptions {
	const options: ProxyOptions = {
		target
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

function admindProxies(target: string, devUserEmail?: string): Record<string, ProxyOptions> {
	return {
		'/admin/api': admindProxy(target),
		'/agent/api': admindProxy(target, devUserEmail),
		'/buzz/api': admindProxy(target, devUserEmail),
		'/attendance/api': admindProxy(target),
		'^/auth/(?!claim)': admindProxy(target, devUserEmail),
		'/calendar/api': admindProxy(target),
		'/crm/api': admindProxy(target, devUserEmail),
		'/mail/api': admindProxy(target),
		'/memory/api': admindProxy(target),
		'/organization/api': admindProxy(target)
	};
}

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), '');
	const admindTarget = env.VITE_ADMIND_TARGET ?? 'http://127.0.0.1:18080';
	const devUserRole = devUserRoleFromEnv(env.VITE_DEV_USER_ROLE);
	const isAdminMockEnabled = env.VITE_MOCK_ADMIN === '1' || env.VITE_MOCK_CRM === '1';
	const devUserEmail = env.VITE_DEV_USER_EMAIL;
	return {
		plugins: [
			devFilesMockPlugin({
				isEnabled: env.VITE_MOCK_FILES === '1',
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
		optimizeDeps: {
			entries: ['src/**/*.svelte']
		},
		server: {
			warmup: {
				clientFiles: ['./src/routes/**/+layout.svelte', './src/routes/**/+page.svelte'],
				ssrFiles: ['./src/routes/**/+layout.svelte', './src/routes/**/+page.svelte']
			},
			allowedHosts: env.VITE_ALLOWED_HOSTS ? env.VITE_ALLOWED_HOSTS.split(',') : undefined,
			proxy: admindTarget ? admindProxies(admindTarget, devUserEmail) : {}
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
