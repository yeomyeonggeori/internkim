import { expect, test } from 'bun:test';

async function outcome(scenario: string) {
	const fixture = new URL('./loading-order.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, '--conditions', 'browser', fixture, scenario], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors] = await Promise.all([new Response(run.stdout).text(), new Response(run.stderr).text()]);
	expect(await run.exited, errors).toBe(0);
	return JSON.parse(output);
}

test('mail deep links skip INBOX and the first body, and resolve later pages from real list rows', async () => {
	expect(await outcome('mail')).toEqual({
		cold: { pages: ['Archive:'], bodies: [2], selected: 2 },
		later: { pages: ['Archive:later'], bodies: [3], selected: 3 },
		missing: true, retried: true, lateIgnored: true, searchCanceled: true
	});
});

test('leave approvals start independent reads together and retain only ordered pending requests', async () => {
	expect(await outcome('approval')).toEqual({
		started: ['attendance_leave_policy_get', 'leave_list', 'company_settings_get', 'person_list'],
		input: { scope: 'all', status: 'requested' },
		pending: ['earlier', 'later'], historicalLabel: true, refused: true
	});
});

test('notification preferences render while reachability is pending and keep enablement guards', async () => {
	expect(await outcome('notifications')).toEqual({ preferencesStarted: true, visible: true, pendingDisabled: true, onEnabled: true, refusedDisabled: true, loadFailure: true });
});
