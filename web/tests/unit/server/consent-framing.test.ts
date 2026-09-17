import { describe, expect, test } from 'bun:test';
import { asksForConsent } from '../../../src/lib/server/consent-framing';

describe('the page that asks a person to allow an app', () => {
	test('is the consent route, wherever the address puts a company in front of it', () => {
		expect(asksForConsent('/oauth/consent')).toBe(true);
		expect(asksForConsent('/samplecompany/oauth/consent')).toBe(true);
	});

	test('is no other page, which may still be embedded', () => {
		expect(asksForConsent('/calendar/embed')).toBe(false);
		expect(asksForConsent('/oauthentic')).toBe(false);
	});
});
