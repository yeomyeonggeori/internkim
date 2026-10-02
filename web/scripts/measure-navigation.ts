import { chromium, expect, type Page } from '@playwright/test';
import { performance } from 'node:perf_hooks';
import { writeFile } from 'node:fs/promises';
import { cpus, loadavg } from 'node:os';
import { createClient } from '@supabase/supabase-js';
import { taskWeekCodeForDateISO } from '../src/lib/task/task-week-code.ts';

const [baselineURL, candidateURL, output = 'tests/performance/navigation-results.json', roundsText = '12', profile = 'desktop', baselineBuild = 'unspecified', candidateBuild = 'unspecified'] = process.argv.slice(2);
if (!baselineURL || !candidateURL) throw new Error('Usage: bun scripts/measure-navigation.ts BASELINE_URL CANDIDATE_URL OUTPUT [ROUNDS] [desktop|mobile]');
const databaseURL = process.env.SUPABASE_URL ?? '';
if (!['localhost', '127.0.0.1', '[::1]'].includes(new URL(databaseURL).hostname)) throw new Error('Benchmarks only seed a loopback Supabase project');
for (const url of [baselineURL, candidateURL]) if (!['localhost', '127.0.0.1'].includes(new URL(url).hostname)) throw new Error('Both builds must be served locally');
const rounds = Number(roundsText);
const admin = createClient(databaseURL, process.env.SUPABASE_SECRET_KEY ?? '', { auth: { persistSession: false } });
const client = createClient(databaseURL, process.env.SUPABASE_PUBLISHABLE_KEY ?? '', { auth: { persistSession: false } });
const { data: auth, error: authError } = await client.auth.signInWithPassword({ email: 'member1@example.com', password: 'seed-password' });
if (authError || !auth.session) throw new Error('The seeded local member could not sign in');
const companyID = '000000cc-0000-0000-0000-000000000001';
const memberID = '000000ee-0000-0000-0000-000000000001';
const now = new Date();
const dateISO = now.toISOString().slice(0, 10);
const week = taskWeekCodeForDateISO(dateISO);
const previousDate = new Date(now.getTime() - 7 * 86400000);
const previousWeek = taskWeekCodeForDateISO(previousDate.toISOString().slice(0, 10));
const monday = new Date(previousDate);
monday.setUTCDate(monday.getUTCDate() - (monday.getUTCDay() + 6) % 7);
const sunday = new Date(monday.getTime() + 6 * 86400000);
const previousWeekLabel = `${monday.getUTCMonth() + 1}/${monday.getUTCDate()} - ${sunday.getUTCMonth() + 1}/${sunday.getUTCDate()}`;
const eventDate = new Date(now.getFullYear(), now.getMonth() + 1, 12, 9);
const { data: seeded, error: seedError } = await admin.from('task').insert([
	...Array.from({ length: 120 }, (_, index) => ({ company_id: companyID, title: `Navigation benchmark task ${index}`, status: 'planned', is_event: false, is_whole_day: false, starts_at: new Date(now.getFullYear(), now.getMonth(), now.getDate(), 9).toISOString(), ends_at: new Date(now.getFullYear(), now.getMonth(), now.getDate(), 10).toISOString(), requester_id: memberID })),
	{ company_id: companyID, title: 'Navigation benchmark calendar event', is_event: true, is_whole_day: false, status: 'planned', starts_at: new Date(now.getFullYear(), now.getMonth(), now.getDate(), 9).toISOString(), ends_at: new Date(now.getFullYear(), now.getMonth(), now.getDate(), 10).toISOString(), requester_id: memberID },
	{ company_id: companyID, title: 'Navigation benchmark next month', is_event: true, is_whole_day: false, status: 'planned', starts_at: eventDate.toISOString(), ends_at: new Date(eventDate.getTime() + 3600000).toISOString(), requester_id: memberID }
]).select('id');
if (seedError || !seeded) throw new Error(seedError?.message ?? 'No benchmark fixtures');
const taskIDs = seeded.slice(0, 120).map(row => row.id);
const { error: participantsError } = await admin.from('task_participant').insert(taskIDs.map(task_id => ({ task_id, member_id: memberID })));
if (participantsError) throw participantsError;
const browser = await chromium.launch();
const results: unknown[] = [];
const isMobile = profile === 'mobile';
const viewport = isMobile ? { width: 390, height: 844 } : { width: 1440, height: 1000 };
const browserVersion = browser.version();
const authStorageKey = `sb-${new URL(databaseURL).hostname.split('.')[0]}-auth-token`;
const baselinePages = new WeakSet<Page>();
const calendarOf = (page: Page) => baselinePages.has(page) ? page.frameLocator('iframe') : page;
const titleOf = (page: Page) => calendarOf(page).locator('.calendar-toolbar-title');

try {
	for (let round = -1; round < rounds; round++) {
		const variants = round % 2 === 0 ? [['baseline', baselineURL], ['candidate', candidateURL]] : [['candidate', candidateURL], ['baseline', baselineURL]];
		for (const [variant, baseURL] of variants) {
			const context = await browser.newContext({ baseURL, viewport, locale: 'ko-KR', timezoneId: 'Asia/Seoul', isMobile, hasTouch: isMobile });
			await context.addInitScript(({ key, session }) => localStorage.setItem(key, JSON.stringify(session)), { key: authStorageKey, session: auth.session });
			const page = await context.newPage();
			if (variant === 'baseline') baselinePages.add(page);
			const protocol = await context.newCDPSession(page);
			await protocol.send('Network.enable');
			await protocol.send('Emulation.setCPUThrottlingRate', { rate: isMobile ? 4 : 1 });
			await page.route('**/api/calendar/holidays?**', route => route.fulfill({ json: { holidays: [], degraded: false } }));
			let requests: { path: string; method: string }[] = [];
			let bytes = 0;
			page.on('request', request => requests.push({ path: new URL(request.url()).pathname, method: request.method() }));
			protocol.on('Network.loadingFinished', event => { bytes += event.encodedDataLength; });
			const measure = async (scenario: string, action: () => Promise<unknown>, feedback: () => Promise<unknown>, ready: () => Promise<unknown>) => {
				requests = []; bytes = 0;
				const start = performance.now();
				const readying = ready();
				const acting = action();
				const feedbackTime = Promise.resolve().then(feedback).then(() => performance.now() - start);
				await acting;
				await readying;
				const readyMilliseconds = performance.now() - start;
				const feedbackMilliseconds = await feedbackTime;
				await page.waitForTimeout(300);
				if (round >= 0) results.push({ variant, round, scenario, feedbackMilliseconds, readyMilliseconds, hostLoadAverage: loadavg(), requests: requests.length, toolRequests: requests.filter(request => request.path.includes('/tools/')).length, encodedBytes: bytes, requestPaths: requests.map(request => request.path) });
			};
			const renderedFrames = (locator: ReturnType<Page['locator']>) => locator.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
			const taskReady = async () => { await page.locator('[data-task-ready="true"]').waitFor(); };
			const taskCard = page.locator(`[data-task-board-card="${taskIDs[0]}"]`);
			const currentTaskReady = async () => { await taskReady(); await taskCard.waitFor(); await renderedFrames(taskCard); };
			const previousTaskReady = async () => { await page.waitForFunction(week => new URL(location.href).searchParams.get('week') === week, previousWeek); await taskReady(); await expect(page.locator('[data-slot="select-trigger"][aria-label="날짜로 주차 이동"]')).toHaveText(previousWeekLabel); };
			const nav = (route: string) => page.locator(`a:is([href="/example-co/${route}/"],[href="/example-co/${route}"]):visible`).first().click();
			await measure('task.first_authenticated_document', () => page.goto('/example-co/task'), () => page.locator('main').waitFor(), currentTaskReady);
			await measure('task.previous_week', () => page.getByRole('button', { name: '이전 주', exact: true }).click(), taskReady, previousTaskReady);
			await page.getByRole('button', { name: '다음 주', exact: true }).click();
			await currentTaskReady();
			const taskMarker = `Navigation fresh task ${profile} ${String(round + 1).padStart(2, '0')} ${variant === 'baseline' ? 'B' : 'C'}`;
			const taskUpdate = await admin.from('task').update({ title: taskMarker }).eq('id', taskIDs[0]);
			if (taskUpdate.error) throw taskUpdate.error;
			await expect(taskCard).not.toContainText(taskMarker);
			await measure('task.refresh', () => page.keyboard.press('r'), taskReady, async () => { await taskReady(); await expect(taskCard).toContainText(taskMarker); await renderedFrames(taskCard); });
			if (!isMobile) {
				await measure('organization.first_navigation', () => nav('organization'), () => page.locator('main').waitFor(), () => page.getByTestId('organization-board').waitFor());
				await measure('task.revisit', () => nav('task'), taskReady, currentTaskReady);
				await measure('navigation.back', () => page.goBack(), () => page.getByTestId('organization-board').waitFor(), () => page.getByTestId('organization-board').waitFor());
				await measure('navigation.forward', () => page.goForward(), taskReady, currentTaskReady);
			}
			await measure('calendar.first_navigation', () => nav('calendar'), async () => { await (await titleOf(page)).waitFor(); }, async () => { const event = calendarOf(page).locator(`[data-calendar-event-id="${seeded[120].id}"]:visible`).first(); await event.waitFor(); await renderedFrames(event); });
			const eventMarker = `Navigation fresh event ${profile} ${String(round + 1).padStart(2, '0')} ${variant === 'baseline' ? 'B' : 'C'}`;
			const eventUpdate = await admin.from('task').update({ title: eventMarker }).eq('id', seeded[120].id);
			if (eventUpdate.error) throw eventUpdate.error;
			await expect(calendarOf(page).locator(`[data-calendar-event-id="${seeded[120].id}"]:visible`).first()).not.toContainText(eventMarker);
			await measure('calendar.refresh', () => page.keyboard.press('r'), async () => { await titleOf(page).waitFor(); }, async () => { const event = calendarOf(page).locator(`[data-calendar-event-id="${seeded[120].id}"]:visible`).first(); await expect(event).toContainText(eventMarker); await renderedFrames(event); });
			if (isMobile && variant === 'candidate') await page.getByRole('button', { name: '더보기', exact: true }).click();
			await measure('calendar.locale', async () => { await page.getByRole('button', { name: '언어 변경' }).click(); await page.getByRole('menuitemradio', { name: 'English' }).click(); }, async () => { await (await titleOf(page)).waitFor(); }, async () => { await (await titleOf(page)).filter({ hasText: /October|November|January|February|March|April|May|June|July|August|September|December/ }).waitFor(); });
			await page.getByRole('button', { name: 'Change language' }).click(); await page.getByRole('menuitemradio', { name: '한국어' }).click();
			if (isMobile && variant === 'candidate') await page.getByRole('button', { name: '닫기', exact: true }).click();
			await measure('calendar.palette_same_route', async () => { await page.keyboard.press('/'); await page.getByPlaceholder('검색').fill('Navigation benchmark next month'); await page.getByRole('option').filter({ hasText: 'Navigation benchmark next month' }).first().click(); }, async () => { await (await titleOf(page)).waitFor(); }, async () => { await (await calendarOf(page)).locator(`[data-calendar-event-id="${seeded[121].id}"]:visible`).first().waitFor(); });
			await measure('attendance.first_navigation', () => nav('attendance'), () => page.locator('header[data-app-chrome]').waitFor(), () => isMobile ? page.getByTestId('mobile-attendance-tools-view').waitFor() : page.getByTestId('team-status-grid').waitFor());
			if (!isMobile) {
				await measure('attendance.approvals_first', () => page.getByTestId('leave-approval-navigation').click(), () => page.getByTestId('leave-approval-view').waitFor(), async () => { await page.getByTestId('leave-approval-view').waitFor(); await page.waitForFunction(() => { const button = document.querySelector('[data-testid=leave-approval-view] button'); return button && !(button as HTMLButtonElement).disabled; }); });
				await measure('attendance.management_first', () => page.getByTestId('leave-management-navigation').click(), () => page.getByTestId('leave-management-view').waitFor(), async () => { await page.getByTestId('leave-management-refresh').waitFor(); await page.waitForFunction(() => !(document.querySelector('[data-testid=leave-management-refresh]') as HTMLButtonElement)?.disabled); });
				await measure('attendance.handwritten_first', () => page.getByTestId('hand-written-navigation').click(), () => page.getByTestId('hand-written-view').waitFor(), async () => { await page.getByTestId('hand-written-refresh').waitFor(); await page.waitForFunction(() => !(document.querySelector('[data-testid=hand-written-refresh]') as HTMLButtonElement)?.disabled); });
				await measure('attendance.approvals_revisit', () => page.getByTestId('leave-approval-navigation').click(), () => page.getByTestId('leave-approval-view').waitFor(), async () => { await page.getByTestId('leave-approval-view').waitFor(); await page.waitForFunction(() => { const button = document.querySelector('[data-testid=leave-approval-view] button'); return button && !(button as HTMLButtonElement).disabled; }); });
				await nav('calendar'); await (await titleOf(page)).waitFor();
				await nav('attendance'); await page.getByTestId('team-status-grid').waitFor();
				await page.waitForTimeout(300);
				await page.getByTestId('leave-management-navigation').hover();
				await page.waitForTimeout(500);
				await measure('attendance.management_intent_500ms', () => page.getByTestId('leave-management-navigation').click(), () => page.getByTestId('leave-management-view').waitFor(), async () => { await page.getByTestId('leave-management-refresh').waitFor(); await page.waitForFunction(() => !(document.querySelector('[data-testid=leave-management-refresh]') as HTMLButtonElement)?.disabled); });
				await nav('calendar'); await titleOf(page).waitFor();
				await nav('attendance'); await page.getByTestId('team-status-grid').waitFor();
				await page.waitForTimeout(300);
				await measure('attendance.management_direct_first', () => page.getByTestId('leave-management-navigation').click(), () => page.getByTestId('leave-management-view').waitFor(), async () => { await page.getByTestId('leave-management-refresh').waitFor(); await page.waitForFunction(() => !(document.querySelector('[data-testid=leave-management-refresh]') as HTMLButtonElement)?.disabled); });

			}
			await context.close();
			console.log(`${profile} ${variant} ${round < 0 ? 'warmup' : round + 1}/${rounds}`);
		}
	}
} finally {
	await browser.close();
	await admin.from('task').delete().in('id', seeded.map(row => row.id));
	await writeFile(output, JSON.stringify({ protocolVersion: 4, candidateBuild, baselineBuild, runtimeVersion: process.version, baselineURL, candidateURL, backendOrigin: new URL(databaseURL).origin, measuredAt: new Date().toISOString(), baselineCommit: '5c5c1a0feddff45df40e054de57fd0163b7377bf', browserVersion, hostCPU: cpus()[0]?.model, hostCPUCount: cpus().length, viewport, profile, cpuRate: isMobile ? 4 : 1, rounds, fixtureTasks: 120, fixtureEvents: 2, fixtureWeek: week, timezone: 'Asia/Seoul', network: 'loopback, unthrottled; holidays provider stubbed equally', timing: 'Playwright action start to observed visible feedback/scenario-ready (task ready flag + fixture card; refresh new title marker + two animation frames); includes automation actionability and observation overhead', trafficWindow: 'through data-ready plus 300ms; CDP encodedDataLength, excludes unfinished and websocket traffic', results }, null, 2));
}
