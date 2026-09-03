import { describe, expect, test } from 'bun:test';
import config from '../../vite.config';
import type { ProxyOptions } from 'vite';

const admindProxyPaths = [
	'/admin/api',
	'/attendance/api',
	'/auth',
	'/calendar/api',
	'/mail/api',
	'/memory/api'
];

async function devServerProxy(): Promise<Record<string, string | ProxyOptions>> {
	const resolvedConfig = await (typeof config === 'function'
		? config({ command: 'serve', mode: 'proxy-test', isSsrBuild: false, isPreview: false })
		: config);
	return resolvedConfig.server?.proxy ?? {};
}

describe('vite proxy config', () => {
	test('forwards original host information to admind routes', async () => {
		const proxy = await devServerProxy();

		for (const path of admindProxyPaths) {
			expect(proxy[path]).toMatchObject({
				target: 'http://127.0.0.1:18080',
				xfwd: true
			});
		}
	});

	test('leaves the public API to the app that answers it', async () => {
		const proxy = await devServerProxy();

		expect(Object.keys(proxy).filter((path) => path.startsWith('/api'))).toEqual([]);
	});
});
