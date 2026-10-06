import { beforeAll, describe, expect, test } from 'bun:test';

type Revisit = { askedWith: { externalID: string; avatarURL: string }[]; unchanged: string; changed: string };

let revisit: Revisit;

beforeAll(async () => {
	const fixture = new URL('./person-picture-revisit.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errorOutput, exitCode] = await Promise.all([
		new Response(run.stdout).text(),
		new Response(run.stderr).text(),
		run.exited
	]);
	expect(exitCode, errorOutput).toBe(0);
	revisit = JSON.parse(output);
});

describe('a picture kept on an earlier visit', () => {
	test('is drawn without asking again while the messenger names the same avatar', () => {
		expect(revisit.askedWith.map((asked) => asked.externalID)).not.toContain('unchanged-account');
		expect(revisit.unchanged).toBe(
			'https://company.supabase.co/storage/v1/object/sign/asset/company-1/shared/person-picture/unchanged-account.png?token=kept'
		);
	});

	test('is asked after again once the messenger names a different avatar', () => {
		expect(revisit.askedWith).toContainEqual({
			externalID: 'changed-account',
			avatarURL: 'https://relay.example.com/media/new.png'
		});
		expect(revisit.changed).toBe(
			'https://company.supabase.co/storage/v1/object/sign/asset/company-1/shared/person-picture/changed-account-latest.png?token=fresh'
		);
	});
});
