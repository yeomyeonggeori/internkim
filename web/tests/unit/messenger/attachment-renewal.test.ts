import { describe, expect, test } from 'bun:test';

async function outcome() {
	const fixture = `${import.meta.dir}/attachment-renewal.fixture.ts`;
	const run = Bun.spawn([process.execPath, '--conditions', 'browser', fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors] = await Promise.all([new Response(run.stdout).text(), new Response(run.stderr).text()]);
	expect(await run.exited, errors).toBe(0);
	return JSON.parse(output);
}

describe('a messenger picture whose signed address runs out', () => {
	test('is signed again once the address is about to expire, and not before', async () => {
		expect(await outcome()).toEqual({
			first: 'signed-1',
			whileFresh: 'signed-1',
			afterBrokenImage: 'signed-2',
			afterReopen: 'signed-3',
			signings: 3
		});
	});

});
