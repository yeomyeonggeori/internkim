import { expect, test } from 'bun:test';

test('CRM loading and mutation lifecycle fixtures run in isolation', async () => {
	const fixture = new URL('crm-loading.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, 'test', fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors] = await Promise.all([new Response(run.stdout).text(), new Response(run.stderr).text()]);
	expect(await run.exited, `${output}\n${errors}`).toBe(0);
});
