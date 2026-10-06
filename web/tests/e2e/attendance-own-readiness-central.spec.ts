import { expect, test, type Route } from '@playwright/test';
import { signInToTheTaskBoard } from './task-central-test-utils';

test.use({ locale: 'ko-KR' });

test('own clock actions become usable while every team history input remains pending', async ({ page }) => {
	await page.addInitScript(() => {
		const events: { event: string; time: number }[] = [];
		Reflect.set(window, 'clockReadinessEvents', events);
		for (const event of ['pageshow', 'focus', 'visibilitychange']) window.addEventListener(event, () => events.push({ event, time: Date.now() }));
	});
	await signInToTheTaskBoard(page);
	const client = await page.context().newCDPSession(page);
	await client.send('Network.enable');
	await client.send('Debugger.enable');
	await client.send('Debugger.setAsyncCallStackDepth', { maxDepth: 32 });
	const initiators: unknown[] = [];
	client.on('Network.requestWillBeSent', (event) => {
		if (event.request.url.endsWith('/attendance_current_get/invoke')) initiators.push(event.initiator);
	});
	const actions: { action: string; time: number }[] = [];
	const pending: string[] = [];
	const pendingAttendanceInputs: Record<string, unknown>[] = [];
	const heldRoutes: Route[] = [];
	let release = () => {};
	const gate = new Promise<void>((resolve) => { release = resolve; });
	await page.route(/\/api\/v1\/tools\/(attendance_team_dashboard_get|attendance_list|leave_list|person_list|attendance_work_policy_get)\/invoke$/, async (route) => {
		heldRoutes.push(route);
		pending.push(new URL(route.request().url()).pathname);
		if (route.request().url().endsWith('/attendance_list/invoke')) {
			pendingAttendanceInputs.push((route.request().postDataJSON() as { input: Record<string, unknown> }).input);
		}
		await gate;
		await route.abort().catch(() => undefined);
	});
	const ownReads: number[] = [];
	const answers: { status: number; time: number }[] = [];
	page.on('request', (request) => {
		if (request.url().endsWith('/attendance_current_get/invoke')) ownReads.push(Date.now());
	});
	page.on('response', (response) => {
		if (response.url().endsWith('/attendance_current_get/invoke')) answers.push({ status: response.status(), time: Date.now() });
	});
	try {
		actions.push({ action: 'navigate', time: Date.now() });
		await page.goto('/example-co/attendance');
		await expect.poll(() => pending.length).toBeGreaterThan(0);
		await expect(page.getByTestId('attendance-own-strip').getByRole('button', { name: /출근|퇴근/ }).first()).toBeVisible();
		actions.push({ action: 'open palette', time: Date.now() });
		await page.keyboard.press('/');
		await expect(page.getByRole('dialog').getByRole('option', { name: /^(출근 ·|퇴근)/ }).first()).toBeVisible();
		actions.push({ action: 'clock option visible', time: Date.now() });
		console.log(JSON.stringify({ ownReads, answers, actions, initiators, events: await page.evaluate(() => Reflect.get(window, 'clockReadinessEvents')) }));
		expect(ownReads).toHaveLength(1);
		expect(pending).toContain('/api/v1/tools/attendance_team_dashboard_get/invoke');
		expect(pendingAttendanceInputs.every((input) => input.scope !== 'all')).toBe(true);
	} finally {
		await Promise.all(heldRoutes.map((route) => route.abort().catch(() => undefined)));
		release();
		await page.unrouteAll({ behavior: 'ignoreErrors' });
	}
});
