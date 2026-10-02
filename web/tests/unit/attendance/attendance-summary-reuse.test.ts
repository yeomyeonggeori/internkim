import { expect, test } from 'bun:test';

test('attendance summary reuse and employee leave reads keep their API fixtures isolated', async () => {
	for (const filename of ['attendance-summary-reuse.fixture.ts', 'employee-leave-parallel-reads.fixture.ts']) {
		const fixture = new URL(filename, import.meta.url).pathname;
		const run = Bun.spawn([process.execPath, 'test', fixture], { stdout: 'pipe', stderr: 'pipe' });
		const [output, errors] = await Promise.all([
			new Response(run.stdout).text(), new Response(run.stderr).text()
		]);
		expect(await run.exited, `${output}\n${errors}`).toBe(0);
	}
});
