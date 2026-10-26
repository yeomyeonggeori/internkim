import { beforeAll, describe, expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

type Revisit = { askedWith: { externalID: string; avatarURL: string }[]; unchanged: string; changed: string; expired: string };

let revisit: Revisit;

beforeAll(async () => {
	const output = await runInItsOwnProcess(new URL('./person-picture-revisit.fixture.ts', import.meta.url), 'script');
	revisit = JSON.parse(output);
}, ownProcessTimeoutMilliseconds);

describe('a picture kept on an earlier visit', () => {
	test('is drawn without asking again while the messenger names the same avatar', () => {
		expect(revisit.askedWith.map((asked) => asked.externalID)).not.toContain('unchanged-account');
		expect(revisit.unchanged).toBe(
			'https://company.supabase.co/storage/v1/object/sign/asset/company-1/shared/person-picture/unchanged-account.png?token=kept'
		);
	});

	test('is signed again by its kept address, without asking the messenger, once its signature runs out', () => {
		expect(revisit.askedWith.map((asked) => asked.externalID)).not.toContain('expired-account');
		expect(revisit.expired).toBe(
			'https://company.supabase.co/storage/v1/object/sign/asset/company-1/shared/person-picture/expired-account.png?token=fresh'
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
