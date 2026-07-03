import { describe, expect, test } from 'bun:test';
import config from '../../vite.config';

const admindProxyPaths = [
	'/.well-known/caldav',
	'/api/v1',
	'/admin/api',
	'/attendance/api',
	'/auth',
	'/calendar/api',
	'/calendar/dav',
	'/calendar/ics',
	'/calendar/oauth',
	'/flow/api',
	'/mail/api',
	'/memory/api'
];

describe('vite proxy config', () => {
	test('forwards original host information to admind routes', async () => {
		const resolvedConfig = await (typeof config === 'function'
			? config({ command: 'serve', mode: 'proxy-test', isSsrBuild: false, isPreview: false })
			: config);
		const proxy = resolvedConfig.server?.proxy ?? {};

		for (const path of admindProxyPaths) {
			expect(proxy[path]).toMatchObject({
				target: 'http://127.0.0.1:18080',
				xfwd: true
			});
		}
	});
});
