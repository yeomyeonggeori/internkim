import { expect, test } from 'bun:test';
import { aCompanyPlane } from './a-company-plane';
import { whatStarts } from './the-entrypoint';

test('a plane told two different messengers refuses to start', async () => {
	const refusal = aCompanyPlane({ messengerPlatform: 'buzz', disagreeAbout: 'capabilityd' });
	await expect(refusal).rejects.toThrow(/disagrees with itself/);
}, 120_000);

test('the company box tells capabilityd and admind the same messenger', () => {
	expect(
		whatStarts('internkim-admind', '-chatd-platform'),
		`host/entrypoint.sh names a different messenger to each daemon, so a message leaves on ` +
			`whichever one capabilityd was told and answers "sent" either way`
	).toBe(whatStarts('internkim-capabilityd', '--chatd-platform'));
});

test('the company box tells admind and blueclaw the same policy file', () => {
	expect(
		whatStarts('internkim-admind', '-blueclaw-policy'),
		`host/entrypoint.sh has admind reconcile the company roster onto one file and blueclaw read ` +
			`another, so nobody the company hires reaches the agent`
	).toBe(whatStarts('blueclaw', '-policy'));
});
