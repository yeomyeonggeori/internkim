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
	test('reaches admind as a local caller, never as a forwarding proxy', async () => {
		const proxy = await devServerProxy();

		for (const path of admindProxyPaths) {
			const options = proxy[path];
			expect(options).toMatchObject({ target: 'http://127.0.0.1:18080' });
			expect(typeof options === 'string' ? undefined : options.xfwd).toBeUndefined();
		}
	});

	test('leaves the public API to the app that answers it', async () => {
		const proxy = await devServerProxy();

		expect(Object.keys(proxy).filter((path) => path.startsWith('/api'))).toEqual([]);
	});
});
