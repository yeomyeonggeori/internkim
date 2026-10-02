import { beforeAll, describe, expect, test } from 'bun:test';

let initialPictures: { member: string; email: string; external: string };

beforeAll(async () => {
	// The exported store is a singleton. Read its initial state in a fresh
	// process so directory-loading tests cannot prime it for this assertion.
	const fixture = new URL('./person-picture-store.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errorOutput, exitCode] = await Promise.all([
		new Response(run.stdout).text(),
		new Response(run.stderr).text(),
		run.exited
	]);
	expect(exitCode, errorOutput).toBe(0);
	initialPictures = JSON.parse(output);
});

// pictureOf answers from the directory the store loads for itself, and it is
// loaded by remember(), not by rememberExternals(). A caller that primes the
// pictures and then reads by member without going through remember() gets
// nothing — which is how the messenger's people list lost its avatars once.
// A caller holding a directory of its own reads by account instead.
describe('personPicture.pictureOf', () => {
	test('answers with nothing until the store has loaded a directory', () => {
		expect(initialPictures.member).toBe('');
		expect(initialPictures.email).toBe('');
	});

	test('answers with nothing for an account it has not been told about', () => {
		expect(initialPictures.external).toBe('');
	});
});
