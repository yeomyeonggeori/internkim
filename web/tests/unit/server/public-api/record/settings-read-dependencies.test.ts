import { describe, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import { runToolOverTheRecord } from '$lib/server/public-api/record';
import { leavesTaskLabelsUndecided } from '$lib/server/public-api/record/task-labels';

function settingsRecord() {
	const paths: string[] = [];
	const request: typeof fetch = Object.assign(async (input: Parameters<typeof fetch>[0]) => {
		const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url);
		paths.push(url.pathname);
		if (url.pathname.endsWith('/company')) return Response.json({
			id: 'company', name: 'Example Co', country: 'KR', locale: 'ko', timezone: 'Asia/Seoul',
			currency_code: 'KRW', work_locations: [], rules: {}, profile: {},
			profile_image: 'company/picture.png', task_vocabulary: {}
		});
		if (url.pathname.includes('/object/sign/')) return Response.json({ signedURL: '/object/sign/asset/company/picture.png?token=sample' });
		throw new Error(`unexpected settings dependency ${url.pathname}`);
	}, { preconnect() {} });
	const caller = createClient('https://record.example.com', 'test-key', {
		auth: { persistSession: false, autoRefreshToken: false }, global: { fetch: request }
	});
	return { caller, paths };
}

describe('settings-only read dependencies', () => {
	test('timezone/settings reads need neither the people directory nor a signed image', async () => {
		const { caller, paths } = settingsRecord();
		const answer = await runToolOverTheRecord(caller, caller, 'member', 'company_settings_get',
			{ includeProfileImage: false }, new Date('2026-10-02'), leavesTaskLabelsUndecided);
		expect(answer.status).toBe(200);
		expect(answer.body).toMatchObject({ result: { timeZone: 'Asia/Seoul', currencyCode: 'KRW', profileImageURL: null } });
		expect(paths).toEqual(['/rest/v1/company', '/rest/v1/company']);
	});

	test('existing callers still receive their signed profile image', async () => {
		const { caller, paths } = settingsRecord();
		const answer = await runToolOverTheRecord(caller, caller, 'member', 'company_settings_get',
			{}, new Date('2026-10-02'), leavesTaskLabelsUndecided);
		expect(answer.status).toBe(200);
		expect(paths.some(path => path.includes('/object/sign/'))).toBe(true);
		expect(paths.some(path => path.endsWith('/member'))).toBe(false);
	});
});
