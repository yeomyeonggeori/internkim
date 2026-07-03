import { describe, expect, test } from 'bun:test';
import { googleOAuthStartURLForReturnURL } from '../../../src/routes/calendar/calendar-google-account-state';

describe('calendar google account state', () => {
	test('forces account selection for the primary connect URL', () => {
		const url = googleOAuthStartURLForReturnURL('http://127.0.0.1:5174/calendar/');

		expect(url).toBe('/calendar/oauth/google/start?returnTo=http%3A%2F%2F127.0.0.1%3A5174%2Fcalendar%2F&switchAccount=true&popup=true');
	});
});
