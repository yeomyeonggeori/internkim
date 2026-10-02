import { chromium, expect, type BrowserContext, type Page, type Request } from '@playwright/test';
import { createClient } from '@supabase/supabase-js';
import { performance } from 'node:perf_hooks';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir, cpus, loadavg } from 'node:os';
import { join } from 'node:path';

const [baselineURL, candidateURL, output, roundsText = '12', profile = 'desktop'] = process.argv.slice(2);
const databaseURL = process.env.SUPABASE_URL ?? '';
for (const value of [baselineURL, candidateURL, databaseURL]) {
	if (!value || !['127.0.0.1', 'localhost'].includes(new URL(value).hostname)) throw new Error('Only loopback builds and fixtures are allowed');
}
if (!output) throw new Error('An output JSON path is required');
const admin = createClient(databaseURL, process.env.SUPABASE_SECRET_KEY ?? '', { auth: { persistSession: false } });
const client = createClient(databaseURL, process.env.SUPABASE_PUBLISHABLE_KEY ?? '', { auth: { persistSession: false } });
const { data: auth, error } = await client.auth.signInWithPassword({ email: 'member1@example.com', password: 'seed-password' });
if (error || !auth.session) throw new Error('Local fixture sign-in failed');
const authKey = `sb-${new URL(databaseURL).hostname.split('.')[0]}-auth-token`;
const snapshotKey = 'internkim:task-snapshot:v1';
const now = new Date();
const startsAt = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 9).toISOString();
const endsAt = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 10).toISOString();
const memberID = '000000ee-0000-0000-0000-000000000001';
const { data: tasks, error: seedError } = await admin.from('task').insert(Array.from({ length: 120 }, (_, index) => ({ company_id: '000000cc-0000-0000-0000-000000000001', title: `Restart benchmark task ${index}`, is_event: false, is_whole_day: false, status: 'planned', starts_at: startsAt, ends_at: endsAt, requester_id: memberID }))).select('id');
if (seedError || !tasks) throw new Error('Local fixture creation failed');
const ids = tasks.map(task => task.id);
const parent = await mkdtemp(join(tmpdir(), 'internkim-task-restart-'));
const results: unknown[] = [];
const rounds = Number(roundsText);
const isMobile = profile === 'mobile';
const viewport = isMobile ? { width: 390, height: 844 } : { width: 1440, height: 1000 };
let version = '';
let activeContext: BrowserContext | null = null;

async function open(profilePath: string, baseURL: string) {
	const context = await chromium.launchPersistentContext(profilePath, { baseURL, viewport, locale: 'ko-KR', timezoneId: 'Asia/Seoul', isMobile, hasTouch: isMobile });
	activeContext = context;
	version = context.browser()?.version() ?? version;
	await context.addInitScript(({ authKey, session }) => {
		if (!localStorage.getItem(authKey)) localStorage.setItem(authKey, JSON.stringify(session));
	}, { authKey, session: auth.session });
	const page = context.pages()[0] ?? await context.newPage();
	page.setDefaultNavigationTimeout(30000);
	const cdp = await context.newCDPSession(page);
	await cdp.send('Emulation.setCPUThrottlingRate', { rate: isMobile ? 4 : 1 });
	return { context, page };
}

async function frames(page: Page) {
	await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
}

async function measuredVisit(page: Page, variant: string, round: number, phase: string, marker: string) {
	const api: { path: string; milliseconds: number; startedMilliseconds: number }[] = [];
	const starts = new Map<Request, number>();
	page.on('request', request => { if (request.url().includes('/tools/')) starts.set(request, performance.now()); });
	page.on('requestfinished', request => {
		const start = starts.get(request);
		if (start !== undefined) api.push({ path: new URL(request.url()).pathname, milliseconds: performance.now() - start, startedMilliseconds: start - visitStart });
	});
	const start = performance.now();
	const visitStart = start;
	const card = page.locator(`[data-task-board-card="${ids[0]}"]`);
	const structure = page.locator('main').waitFor().then(async () => { await frames(page); return performance.now() - start; }).catch(error => ({ error }));
	let firstCardText = '';
	const firstCard = card.waitFor().then(async () => { firstCardText = await card.innerText(); await frames(page); return performance.now() - start; }).catch(error => ({ error }));
	try {
	await page.goto('/example-co/task');
	const structureMilliseconds = await structure;
	const cardMilliseconds = await firstCard;
	if (typeof structureMilliseconds !== 'number') throw structureMilliseconds.error;
	if (typeof cardMilliseconds !== 'number') throw cardMilliseconds.error;
	await page.locator('[data-task-ready="true"]').waitFor();
	await expect(card).toContainText(marker);
	await frames(page);
	const freshMilliseconds = performance.now() - start;
	await page.waitForTimeout(300);
	if (round >= 0) results.push({ variant, round, phase, structureMilliseconds, cardMilliseconds, freshMilliseconds, firstCardText, api, hostLoadAverage: loadavg() });
	} catch (error) {
		console.error(JSON.stringify({ variant, round, phase, pathname: new URL(page.url()).pathname, api, visible: await page.locator('main').innerText().catch(() => 'No main') }));
		throw error;
	}
}

try {
	const participants = await admin.from('task_participant').insert(ids.map(task_id => ({ task_id, member_id: memberID })));
	if (participants.error) throw participants.error;
	for (let round = -1; round < rounds; round++) {
		const variants = round % 2 === 0 ? [['baseline', baselineURL], ['candidate', candidateURL]] : [['candidate', candidateURL], ['baseline', baselineURL]];
		for (const [variant, baseURL] of variants) {
			const path = join(parent, `${variant}-${round}`);
			const marker = `Restart fresh ${profile} ${String(round + 1).padStart(2, '0')} ${variant === 'baseline' ? 'B' : 'C'}`;
			const prepared = await admin.from('task').update({ title: marker }).eq('id', ids[0]);
			if (prepared.error) throw prepared.error;
			let { context, page } = await open(path, baseURL);
			await measuredVisit(page, variant, round, 'fresh_storage', marker);
			if (variant === 'candidate') await expect.poll(() => page.evaluate(key => localStorage.getItem(key), snapshotKey)).not.toBeNull();
			await context.close(); activeContext = null;
			const nextMarker = marker.replace('fresh', 'newest');
			const updated = await admin.from('task').update({ title: nextMarker }).eq('id', ids[0]);
			if (updated.error) throw updated.error;
			({ context, page } = await open(path, baseURL));
			await measuredVisit(page, variant, round, 'browser_restart', nextMarker);
			await context.close(); activeContext = null;
			await rm(path, { recursive: true, force: true });
			console.log(`${profile} ${variant} ${round < 0 ? 'warmup' : round + 1}/${rounds}`);
		}
	}
} finally {
	await activeContext?.close();
	await admin.from('task').delete().in('id', ids);
	await rm(parent, { recursive: true, force: true });
	await writeFile(output, JSON.stringify({ protocolVersion: 4, candidateBuild: process.env.BENCHMARK_CANDIDATE_BUILD ?? 'unspecified', baselineBuild: process.env.BENCHMARK_BASELINE_BUILD ?? 'unspecified', runtimeVersion: process.version, baselineURL, candidateURL, backendOrigin: new URL(databaseURL).origin, measuredAt: new Date().toISOString(), browserVersion: version, baselineCommit: '5c5c1a0feddff45df40e054de57fd0163b7377bf', hostCPU: cpus()[0]?.model, viewport, profile, cpuRate: isMobile ? 4 : 1, fixtureTasks: 120, rounds, timing: 'Playwright navigation start through main structure/card/new marker plus 2rAF; automation overhead included. Each phase launches a new Chromium process; restart reuses only that variant/round profile. Backend title changes while browser is closed. API durations are request start to finished body, not SQL execution duration.', results }, null, 2));
}
