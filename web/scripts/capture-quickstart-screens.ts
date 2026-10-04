//   tools/with-local-plane bun run web/scripts/capture-quickstart-screens.ts

import { chromium, type Locator, type Page } from 'playwright';
import { createClient } from '@supabase/supabase-js';
import { join } from 'node:path';

const repositoryRoot = join(import.meta.dir, '../..');
const imageDirectory = join(repositoryRoot, 'docs/web/public/images/quickstart');
const port = 5197;
const appURL = `http://localhost:${port}`;
const adminEmail = 'quickstart-admin@example.com';
const adminPassword = 'quickstart-sample-1';
const companySlug = 'sample-company';
const firstCompileMilliseconds = 120_000;

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

async function removeTheSample(): Promise<void> {
	await service.from('company').delete().eq('slug', companySlug);
	const { data } = await service.auth.admin.listUsers({ perPage: 1000 });
	const account = data.users.find((user) => user.email === adminEmail);
	if (account) await service.auth.admin.deleteUser(account.id);
}

function resetTheLocalPlane(): void {
	const reset = Bun.spawnSync(['supabase', 'db', 'reset'], { cwd: repositoryRoot });
	if (reset.exitCode !== 0) throw new Error(`supabase db reset failed: ${reset.stderr.toString()}`);
}

async function createTheAdminAccount(): Promise<void> {
	const { error } = await service.auth.admin.createUser({ email: adminEmail, password: adminPassword, email_confirm: true });
	if (error) throw new Error(`creating ${adminEmail}: ${error.message}`);
}

async function startTheApp(): Promise<ReturnType<typeof Bun.spawn>> {
	const server = Bun.spawn(['bunx', 'vite', 'dev', '--port', String(port), '--strictPort'], {
		cwd: join(repositoryRoot, 'web'),
		env: {
			...process.env,
			SUPABASE_URL: plane.API_URL,
			SUPABASE_PUBLISHABLE_KEY: plane.PUBLISHABLE_KEY,
			SUPABASE_SECRET_KEY: plane.SECRET_KEY,
			SUPABASE_JWT_SIGNING_KEY: signingKey
		},
		stdout: Bun.file(join(repositoryRoot, '.artifacts/capture-quickstart-server.log')),
		stderr: Bun.file(join(repositoryRoot, '.artifacts/capture-quickstart-server.log'))
	});
	for (let attempt = 0; attempt < 120; attempt += 1) {
		const isUp = await fetch(appURL).then(() => true, () => false);
		if (isUp) return server;
		await Bun.sleep(1000);
	}
	server.kill();
	throw new Error(`the app did not answer on ${appURL}`);
}

async function open(page: Page, path: string): Promise<void> {
	await page.goto(`${appURL}${path}`);
	const field = page.locator('input').first();
	const isReady = await field.waitFor({ timeout: 30_000 }).then(() => true, () => false);
	if (isReady) return;
	await page.reload();
	await field.waitFor();
}

async function capture(target: Locator, name: string): Promise<void> {
	await target.page().waitForTimeout(800);
	await target.screenshot({ path: join(imageDirectory, `${name}.png`) });
}

async function captureTheSignUp(page: Page): Promise<void> {
	await open(page, '/auth/claim?new-company=1');
	await page.locator('input[type=email]').fill(adminEmail);
	await capture(page.locator('[data-slot=card]').first(), 'signup-ko');
}

async function captureTheCompanyForm(page: Page): Promise<void> {
	await open(page, '/attendance/');
	await page.locator('input[type=email]').fill(adminEmail);
	await page.locator('input[type=password]').fill(adminPassword);
	await page.locator('input[type=password]').press('Enter');
	await page.waitForURL('**/start**');
	await page.locator('input[autocomplete=organization]').fill('샘플회사');
	await page.locator('input[autocomplete=off]').fill(companySlug);
	await page.locator('input[autocomplete=name]').fill('이샘플');
	await page.getByText('쓸 수 있습니다.').waitFor();
	await capture(page.locator('[data-slot=card]').first(), 'company-ko');
	await page.locator('button[type=submit]').click();
	await page.waitForURL('**/settings/setup**');
}

async function captureTheSetup(page: Page): Promise<void> {
	await capture(page.locator('[data-slot=card]').first(), 'setup-ko');
	await page.getByRole('button', { name: '구성원 초대' }).click();
	const dialog = page.getByRole('dialog');
	await dialog.locator('input').nth(0).fill('박예시');
	await dialog.locator('input[type=email]').fill('coworker@example.com');
	await capture(dialog, 'invite-ko');
}

await removeTheSample();
await createTheAdminAccount();
const server = await startTheApp();
const browser = await chromium.launch();
try {
	const context = await browser.newContext({
		viewport: { width: 1280, height: 900 },
		deviceScaleFactor: 2,
		colorScheme: 'light',
		locale: 'ko-KR',
		timezoneId: 'Asia/Seoul'
	});
	const page = await context.newPage();
	page.setDefaultTimeout(firstCompileMilliseconds);
	page.on('pageerror', (failure) => console.error(`page error: ${failure.message}`));
	page.on('console', (message) => message.type() === 'error' && console.error(`console: ${message.text()}`));
	try {
		await captureTheSignUp(page);
		await captureTheCompanyForm(page);
		await captureTheSetup(page);
	} catch (failure) {
		await page.screenshot({ path: join(repositoryRoot, '.artifacts/capture-quickstart-failure.png') });
		throw failure;
	}
} finally {
	await browser.close();
	server.kill();
	resetTheLocalPlane();
}
