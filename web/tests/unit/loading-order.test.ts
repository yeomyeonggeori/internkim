import { expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../support/in-its-own-process';

async function outcome(scenario: string) {
	return JSON.parse(await runInItsOwnProcess(new URL('./loading-order.fixture.ts', import.meta.url), 'browser', scenario));
}

test('mail deep links skip INBOX and the first body, and resolve later pages from real list rows', async () => {
	expect(await outcome('mail')).toEqual({
		cold: { pages: ['Archive:'], bodies: [2], selected: 2 },
		later: { pages: ['Archive:later'], bodies: [3], selected: 3 },
		missing: true, retried: true, lateIgnored: true, searchCanceled: true
	});
}, ownProcessTimeoutMilliseconds);

test('leave approvals start independent reads together and retain only ordered pending requests', async () => {
	expect(await outcome('approval')).toEqual({
		started: ['attendance_leave_policy_get', 'leave_list', 'company_settings_get', 'person_list'],
		input: { scope: 'all', status: 'requested', limit: 100 },
		pending: ['earlier', 'later'], historicalLabel: true, refused: true
	});
}, ownProcessTimeoutMilliseconds);

test('notification preferences render while reachability is pending and keep enablement guards', async () => {
	expect(await outcome('notifications')).toEqual({ preferencesStarted: true, visible: true, pendingDisabled: true, onEnabled: true, refusedDisabled: true, loadFailure: true });
}, ownProcessTimeoutMilliseconds);
