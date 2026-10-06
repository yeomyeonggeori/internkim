import { expect, test } from 'bun:test';

test('organization cache authority and late-read guards run in isolation', async () => {
	const fixture = new URL('organization-loading-scope.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, 'test', fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors] = await Promise.all([new Response(run.stdout).text(), new Response(run.stderr).text()]);
	expect(await run.exited, `${output}\n${errors}`).toBe(0);
});
