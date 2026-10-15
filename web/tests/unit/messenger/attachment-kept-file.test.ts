import { describe, expect, test } from 'bun:test';

async function outcome() {
	const fixture = `${import.meta.dir}/attachment-kept-file.fixture.ts`;
	const run = Bun.spawn([process.execPath, '--conditions', 'browser', fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors] = await Promise.all([new Response(run.stdout).text(), new Response(run.stderr).text()]);
	expect(await run.exited, errors).toBe(0);
	return JSON.parse(output);
}

describe('a messenger picture kept on the device', () => {
	test('opens from the device without copying or signing, and a new picture is kept after it is signed', async () => {
		expect(await outcome()).toEqual({
			keptOpenable: 'blob:https://workspace.example.test/12',
			keptWork: { signings: 0, copies: 0 },
			newOpenable: 'signed-2',
			documentOpenable: 'signed-1',
			fetched: ['signed-2'],
			newKept: true,
			documentKept: false
		});
	});
});
