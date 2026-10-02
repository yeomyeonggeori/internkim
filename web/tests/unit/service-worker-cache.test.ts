import { expect, test } from 'bun:test';

test('service worker caches requested assets without downloading other screens during installation', async () => {
	const fixture = new URL('./service-worker-cache.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors] = await Promise.all([new Response(run.stdout).text(), new Response(run.stderr).text()]);
	expect(await run.exited, errors).toBe(0);
	expect(JSON.parse(output)).toEqual({
		afterInstall: { eagerAssets: 0, networkRequests: 0, skipWaiting: 1 },
		afterActivate: {
			names: ['another-app-cache', 'internkim-assets', 'internkim-notification-destination', 'internkim-previous'],
			assets: [],
			claimed: 1
		},
		previousOfflineText: 'previously visited asset',
		previousOfflineIcon: 'previous icon',
		currentIconText: 'current icon',
		uncachedFreshIcon: 'newest icon',
		onlineText: 'requested asset',
		offlineText: 'requested asset',
		fetched: ['/icon-192.png', '/icon-192.png', '/_app/immutable/chunks/unused-0.js'],
		privateIntercepted: false,
		notifications: [{ title: 'Task changed', options: {
			body: 'Review task', tag: 'task-sample', icon: '/icon-192.png', data: { openPath: '/task?task=sample' }
		} }],
		messages: [{ type: 'notification-opened', openPath: '/task?task=sample' }],
		focused: 1,
		closed: 1,
		pendingDestination: '/task?task=sample'
	});
});
