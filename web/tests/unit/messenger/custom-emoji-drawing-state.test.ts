import { expect, test } from 'bun:test';

test('a custom emoji is still drawing until its picture arrives or cannot, so nothing shows its name in between', async () => {
	const fixture = new URL('./custom-emoji-drawing-state.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors, exitCode] = await Promise.all([
		new Response(run.stdout).text(), new Response(run.stderr).text(), run.exited
	]);
	expect(exitCode, errors).toBe(0);
	expect(JSON.parse(output)).toEqual({
		beforeTheListArrives: true, listedButNotYetDrawn: true, notInTheList: false, drawn: false, couldNotBeDrawn: false
	});
});
