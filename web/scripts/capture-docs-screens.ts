//   tools/with-local-plane bun run web/scripts/capture-docs-screens.ts

import { chromium, type Browser, type Locator, type Page } from 'playwright';
import { createClient } from '@supabase/supabase-js';
import { join } from 'node:path';

type Locale = 'ko' | 'en';

type QuickstartSample = {
	adminEmail: string;
	companyName: string;
	companySlug: string;
	adminName: string;
	coworkerName: string;
	addressUsable: string;
	inviteMember: string;
};

const repositoryRoot = join(import.meta.dir, '../..');
const imageDirectory = join(repositoryRoot, 'docs/web/public/images');
const port = 5197;
const appURL = `http://localhost:${port}`;
const samplePassword = 'docs-sample-1';
const seedMemberEmail = 'member1@example.com';
const seedMemberPassword = 'seed-password';
const seedCompanyID = '000000cc-0000-0000-0000-000000000001';
const firstCompileMilliseconds = 120_000;
const locales: Locale[] = ['ko', 'en'];

const quickstartSamples: Record<Locale, QuickstartSample> = {
	ko: {
		adminEmail: 'quickstart-admin-ko@example.com',
		companyName: '샘플회사',
		companySlug: 'sample-company',
		adminName: '이샘플',
		coworkerName: '박예시',
		addressUsable: '쓸 수 있습니다.',
		inviteMember: '구성원 초대'
	},
	en: {
		adminEmail: 'quickstart-admin-en@example.com',
		companyName: 'Sample Company',
		companySlug: 'sample-co',
		adminName: 'Lee Sample',
		coworkerName: 'Park Example',
		addressUsable: 'Available.',
		inviteMember: 'Invite a member'
	}
};

const englishBusinesses: Record<string, string> = { 사업하나: 'Product', 사업둘: 'Services' };
const englishTaskTypes: Record<string, string> = {
	기능: 'Feature', 개선: 'Improvement', 회의: 'Meeting', 통화: 'Call', 메일: 'Mail', 메모: 'Note'
};
const englishMembers: Record<string, { name: string; jobTitle: string }> = {
	'member1@example.com': { name: 'Lee Sample', jobTitle: 'CTO' },
	'member2@example.com': { name: 'Kim Example', jobTitle: 'CEO' },
	'member3@example.com': { name: 'Park Example', jobTitle: 'Researcher' },
	'member4@example.com': { name: 'Choi Sample', jobTitle: 'Researcher' }
};
const englishTeams: Record<string, string> = { 개발팀: 'Engineering', 태스크포스: 'Task force' };
const englishTaskTitles: Record<string, string> = {
	'예시 API 문서 작성': 'Write the API docs',
	'예시 화면 리디자인': 'Redesign the sign-in screen',
	'끝이 정해지지 않은 예시 일': 'Ongoing support',
	'주간 회의': 'Weekly meeting',
	'협력 조건 검토 미팅': 'Partnership terms review',
	'후속 조건 확인 통화': 'Follow-up call on terms',
	'유지보수 조건 협의 통화': 'Maintenance terms call',
	'납품 검수 확인 미팅': 'Delivery acceptance meeting',
	'IR 자료 공유': 'Share the investor deck'
};

type LocalPlane = { API_URL: string; PUBLISHABLE_KEY: string; SECRET_KEY: string };

function localPlane(): LocalPlane {
	const status = Bun.spawnSync(['supabase', 'status', '--env'], { cwd: repositoryRoot });
	if (status.exitCode !== 0) throw new Error(`supabase status failed: ${status.stderr.toString()}`);
	return JSON.parse(status.stdout.toString()) as LocalPlane;
}

async function localSigningKey(): Promise<string> {
	const keys = (await Bun.file(join(repositoryRoot, 'supabase/.temp/local-signing-keys.json')).json()) as unknown[];
	return JSON.stringify(keys[0]);
}

const plane = localPlane();
const signingKey = await localSigningKey();
const service = createClient(plane.API_URL, plane.SECRET_KEY, { auth: { persistSession: false } });

function resetTheLocalPlane(): void {
	const reset = Bun.spawnSync(['supabase', 'db', 'reset'], { cwd: repositoryRoot });
	if (reset.exitCode !== 0) throw new Error(`supabase db reset failed: ${reset.stderr.toString()}`);
}

async function createTheAdminAccount(email: string): Promise<void> {
	const { error } = await service.auth.admin.createUser({ email, password: samplePassword, email_confirm: true });
	if (error) throw new Error(`creating ${email}: ${error.message}`);
}

async function updateOrFail(table: string, values: Record<string, unknown>, column: string, value: string): Promise<void> {
	const { error } = await service.from(table).update(values).eq(column, value);
	if (error) throw new Error(`updating ${table} where ${column} = ${value}: ${error.message}`);
}

async function translateTheSeedCompany(): Promise<void> {
	await updateOrFail('company', {
		name: 'Example Co',
		locale: 'en',
		work_locations: [
			{ name: 'Office', color: '#9929bd' }, { name: 'Remote', color: '#669c35' }, { name: 'Outside', color: '#0ea5e9' }
		],
		task_vocabulary: {
			businesses: [{ name: 'Product', color: '#216fe4' }, { name: 'Services', color: '#475569' }],
			types: Object.values(englishTaskTypes).map((name) => ({ name }))
		}
	}, 'id', seedCompanyID);
	for (const [email, member] of Object.entries(englishMembers)) {
		await updateOrFail('member', { name: member.name, job_title: member.jobTitle }, 'email', email);
	}
	for (const [korean, english] of Object.entries(englishTeams)) {
		await updateOrFail('team', { name: english }, 'name', korean);
	}
	await translateTheSeedTasks();
}

async function translateTheSeedTasks(): Promise<void> {
	const { data, error } = await service.from('task').select('id, title, business, type').eq('company_id', seedCompanyID);
	if (error) throw new Error(`reading the seed tasks: ${error.message}`);
	for (const task of data) {
		await updateOrFail('task', {
			title: englishTaskTitles[task.title] ?? task.title,
			business: task.business ? englishBusinesses[task.business] ?? task.business : null,
			type: task.type ? englishTaskTypes[task.type] ?? task.type : null,
			note: null
		}, 'id', task.id);
	}
}

async function startTheApp(): Promise<ReturnType<typeof Bun.spawn>> {
	const log = Bun.file(join(repositoryRoot, '.artifacts/capture-docs-server.log'));
	const server = Bun.spawn(['bunx', 'vite', 'dev', '--port', String(port), '--strictPort'], {
		cwd: join(repositoryRoot, 'web'),
		env: {
			...process.env,
			SUPABASE_URL: plane.API_URL,
			SUPABASE_PUBLISHABLE_KEY: plane.PUBLISHABLE_KEY,
			SUPABASE_SECRET_KEY: plane.SECRET_KEY,
			SUPABASE_JWT_SIGNING_KEY: signingKey
		},
		stdout: log,
		stderr: log
	});
	for (let attempt = 0; attempt < 120; attempt += 1) {
		const isUp = await fetch(appURL).then(() => true, () => false);
		if (isUp) return server;
		await Bun.sleep(1000);
	}
	server.kill();
	throw new Error(`the app did not answer on ${appURL}`);
}

async function openPage(browser: Browser, locale: Locale): Promise<Page> {
	const context = await browser.newContext({
		viewport: { width: 1280, height: 800 },
		deviceScaleFactor: 2,
		colorScheme: 'light',
		locale: locale === 'ko' ? 'ko-KR' : 'en-US',
		timezoneId: 'Asia/Seoul'
	});
	const page = await context.newPage();
	page.setDefaultTimeout(firstCompileMilliseconds);
	page.on('pageerror', (failure) => console.error(`page error: ${failure.message}`));
	page.on('console', (message) => message.type() === 'error' && console.error(`console: ${message.text()}`));
	return page;
}

async function open(page: Page, path: string): Promise<void> {
	await page.goto(`${appURL}${path}`);
	const field = page.locator('input').first();
	const isReady = await field.waitFor({ timeout: 30_000 }).then(() => true, () => false);
	if (isReady) return;
	await page.reload();
	await field.waitFor();
}

async function signIn(page: Page, path: string, email: string, password: string): Promise<void> {
	await open(page, path);
	await page.locator('input[type=email]').fill(email);
	await page.locator('input[type=password]').fill(password);
	await page.locator('input[type=password]').press('Enter');
}

async function capture(target: Locator | Page, section: string, name: string): Promise<void> {
	const page = 'page' in target ? target.page() : target;
	await page.waitForTimeout(800);
	await target.screenshot({ path: join(imageDirectory, section, `${name}.png`) });
}

async function captureTheWorkspace(browser: Browser, locale: Locale): Promise<void> {
	const page = await openPage(browser, locale);
	await signIn(page, '/task', seedMemberEmail, seedMemberPassword);
	const firstTask = locale === 'ko' ? '예시 화면 리디자인' : englishTaskTitles['예시 화면 리디자인'];
	await page.getByText(firstTask).first().waitFor();
	await capture(page, 'overview', `workspace-${locale}`);
	await page.context().close();
}

async function captureTheQuickstart(browser: Browser, locale: Locale): Promise<void> {
	const sample = quickstartSamples[locale];
	await createTheAdminAccount(sample.adminEmail);
	const page = await openPage(browser, locale);
	await open(page, '/auth/claim?new-company=1');
	await page.locator('input[type=email]').fill(sample.adminEmail);
	await capture(page.locator('[data-slot=card]').first(), 'quickstart', `signup-${locale}`);

	await signIn(page, '/attendance/', sample.adminEmail, samplePassword);
	await page.waitForURL('**/start**');
	await page.locator('input[autocomplete=organization]').fill(sample.companyName);
	await page.locator('input[autocomplete=off]').fill(sample.companySlug);
	await page.locator('input[autocomplete=name]').fill(sample.adminName);
	await page.getByText(sample.addressUsable).waitFor();
	await capture(page.locator('[data-slot=card]').first(), 'quickstart', `company-${locale}`);
	await page.locator('button[type=submit]').click();
	await page.waitForURL('**/settings/setup**');

	await capture(page.locator('[data-slot=card]').first(), 'quickstart', `setup-${locale}`);
	await page.getByRole('button', { name: sample.inviteMember }).click();
	const dialog = page.getByRole('dialog');
	await dialog.locator('input').nth(0).fill(sample.coworkerName);
	await dialog.locator('input[type=email]').fill('coworker@example.com');
	await capture(dialog, 'quickstart', `invite-${locale}`);
	await page.context().close();
}

const server = await startTheApp();
const browser = await chromium.launch();
try {
	await captureTheWorkspace(browser, 'ko');
	await translateTheSeedCompany();
	await captureTheWorkspace(browser, 'en');
	for (const locale of locales) await captureTheQuickstart(browser, locale);
} finally {
	await browser.close();
	server.kill();
	resetTheLocalPlane();
}
