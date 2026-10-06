import { expect, test, type Page } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import { WorkspaceLoadingFixture, prepareWorkspaceLoading } from './workspace-loading-fixture';

const phase = process.env.LOADING_CAPTURE_PHASE ?? 'after';
const output = process.env.LOADING_CAPTURE_DIR;
test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

async function capture(page: Page, scene: string, width: number, state: string): Promise<void> {
	if (!output) return;
	await mkdir(`${output}/${phase}`, { recursive: true });
	const motion = await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches);
	expect(motion).toBe(true);
	const scroll = await page.evaluate(() => ({ windowY: window.scrollY, regions: Array.from(document.querySelectorAll('main,[data-app-shell-scroll],[role="tabpanel"]')).map(element => ({ tag: element.tagName, role: element.getAttribute('role'), top: element.scrollTop, left: element.scrollLeft })) }));
	await page.screenshot({ path: `${output}/${phase}/${scene}-${width}-${state}.png`, animations: 'disabled' });
	await writeFile(`${output}/${phase}/${scene}-${width}-${state}.json`, JSON.stringify({ route: new URL(page.url()).pathname, viewport: page.viewportSize(), reducedMotionVerified: motion, animations: 'disabled', scroll }, null, 2));
}

const scenes = [
	{ name: 'crm', path: '/crm', gate: 'crm_organization_list', skeleton: 'crm-loading-skeleton', ready: '[data-crm-ready="true"]' },
	{ name: 'organization', path: '/organization', gate: 'person_list', skeleton: 'organization-skeleton', ready: '[data-testid="organization-board"]' },
	{ name: 'files', path: '/files', gate: 'person.files.roots', skeleton: 'file-list-loading-skeleton', ready: 'button:has-text("주간-회고-1.md")' },
	{ name: 'mail', path: '/mail', gate: 'mail-account', skeleton: '', ready: 'button:has-text("메일 캐시 동작 확인")' },
	{ name: 'mail-body', path: '/mail', gate: 'mail_message_read', skeleton: 'mail-body-loading-skeleton', ready: 'p.whitespace-pre-wrap' },
	{ name: 'memory', path: '/memory', gate: 'person.memory.facts', skeleton: 'memory-loading-skeleton', ready: '#memory-layers-title' },
	{ name: 'schedules', path: '/memory/schedules', gate: 'person.memory.schedules', skeleton: 'schedule-loading-skeleton', ready: 'button:has-text("수정")' },
	{ name: 'runs', path: '/runs', gate: 'person.runs.list', skeleton: 'run-list-loading-skeleton', ready: '[data-task-run-mobile-list], main tbody' },
	{ name: 'run-detail', path: '/runs/dev-task-run-001', gate: 'person.runs.detail', skeleton: 'run-detail-loading-skeleton', ready: 'main h1' }
];

for (const width of [1280, 390, 320]) {
	for (const scene of scenes) {
		test(`${scene.name} pending and ready layout at ${width}px`, async ({ page }) => {
			await page.setViewportSize({ width, height: 900 });
			const fixture = new WorkspaceLoadingFixture();
			fixture.omitMailPreview = scene.name === 'mail-body';
			fixture.hold(scene.gate);
			await prepareWorkspaceLoading(page, fixture);
			await page.goto(`/example-co${scene.path}`);
			expect(await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches)).toBe(true);
			if (scene.name === 'mail-body') await page.getByRole('button', { name: /메일 캐시 동작 확인/ }).first().click();
			await expect.poll(() => fixture.requested.has(scene.gate)).toBe(true);
			if (phase === 'after') {
				if (scene.skeleton) await expect(page.getByTestId(scene.skeleton)).toBeVisible();
				else await expect(page.locator('[data-slot="skeleton"]').first()).toBeVisible();
				expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
			}
			await capture(page, scene.name, width, 'loading');
			fixture.release(scene.gate);
			await expect(page.locator(scene.ready).filter({ visible: true }).first()).toBeVisible();
			await expect(page.locator('[data-slot="skeleton"]')).toHaveCount(0);
			await capture(page, scene.name, width, 'ready');
		});
	}
}

test('same-scope memory refresh keeps the list and detail through a failed read', async ({ page }) => {
	test.skip(phase === 'before', 'Baseline documents the pre-change loading presentation');
	const fixture = new WorkspaceLoadingFixture();
	await prepareWorkspaceLoading(page, fixture);
	await page.goto('/example-co/memory');
	await expect(page.locator('#memory-layers-title')).toBeVisible();
	const fact = page.locator('main button[aria-pressed]').filter({ has: page.locator('p') }).first();
	await fact.click();
	const selected = await fact.textContent();
	fixture.hold('person.memory.facts');
	await page.getByRole('main').getByRole('button', { name: '새로고침', exact: true }).click();
	await expect.poll(() => fixture.requested.has('person.memory.facts')).toBe(true);
	await expect(fact).toHaveText(selected ?? '');
	await expect(page.getByTestId('memory-loading-skeleton')).toHaveCount(0);
	fixture.refused.add('person.memory.facts');
	fixture.release('person.memory.facts');
	await expect(page.getByRole('alert')).toBeVisible();
	await expect(fact).toHaveText(selected ?? '');
});

test('workspace refresh retains entries while its read is pending and failed', async ({ page }) => {
	test.skip(phase === 'before', 'Baseline clears the directory during refresh');
	const fixture = new WorkspaceLoadingFixture();
	await prepareWorkspaceLoading(page, fixture);
	await page.goto('/example-co/files');
	const first = page.getByRole('button', { name: /주간-회고-1.md/ });
	await expect(first).toBeVisible();
	fixture.hold('person.files.list');
	await page.getByRole('tabpanel', { name: '워크스페이스' }).getByRole('button', { name: '새로고침', exact: true }).click();
	await expect.poll(() => fixture.requested.has('person.files.list')).toBe(true);
	await expect(first).toBeVisible();
	await expect(page.getByTestId('file-list-loading-skeleton')).toHaveCount(0);
	fixture.refused.add('person.files.list');
	fixture.release('person.files.list');
	await expect(page.getByRole('alert')).toBeVisible();
	await expect(first).toBeVisible();
});
