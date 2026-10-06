import { expect, test, type Locator, type Page } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import { WorkspaceLoadingFixture, prepareWorkspaceLoading } from './workspace-loading-fixture';

test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

async function bounds(locator: Locator) {
	await expect(locator).toBeVisible();
	const rectangle = await locator.boundingBox();
	if (!rectangle) throw new Error('Visible layout target has no bounding box');
	return rectangle;
}

async function begin(page: Page, width: number, path: string, gate: string) {
	await page.setViewportSize({ width, height: 900 });
	const fixture = new WorkspaceLoadingFixture();
	fixture.hold(gate);
	await prepareWorkspaceLoading(page, fixture);
	await page.goto(`/example-co${path}`);
	await expect.poll(() => fixture.requested.has(gate)).toBe(true);
	await page.evaluate(() => document.fonts.ready);
	return fixture;
}

function expectAligned(actual: number, expected: number) {
	expect(Math.abs(actual - expected)).toBeLessThanOrEqual(1);
}

async function recordGeometry(scene: string, width: number, measurements: Record<string, unknown>) {
	const directory = process.env.LOADING_CAPTURE_DIR;
	if (!directory) return;
	const phase = process.env.LOADING_CAPTURE_PHASE ?? 'after';
	await mkdir(`${directory}/${phase}`, { recursive: true });
	await writeFile(`${directory}/${phase}/${scene}-${width}-geometry.json`, JSON.stringify(measurements, null, 2));
}

for (const width of [1280, 390]) {
	test(`file list reserves breadcrumb and row geometry at ${width}px`, async ({ page }) => {
		const fixture = await begin(page, width, '/files', 'person.files.roots');
		const skeleton = page.getByTestId('file-list-loading-skeleton');
		const pendingList = await bounds(skeleton);
		const pendingRow = await bounds(skeleton.locator(':scope > div').first());
		fixture.release('person.files.roots');
		const readyRow = await bounds(page.locator('section[aria-label] > button[aria-pressed]').first());
		expectAligned(pendingList.y, readyRow.y);
		expectAligned(pendingRow.height, readyRow.height);
		await recordGeometry('files', width, { pendingList, pendingRow, readyRow });
		await expect(skeleton).toHaveCount(0);
	});

	test(`mail cards reserve sender, subject and preview line boxes at ${width}px`, async ({ page }) => {
		const fixture = await begin(page, width, '/mail', 'mail-account');
		const pendingRow = await bounds(page.locator('div.pointer-events-none[aria-hidden="true"]').filter({ has: page.locator('[data-slot="skeleton"]') }).first());
		fixture.release('mail-account');
		const readyRow = await bounds(page.getByRole('button', { name: /메일 캐시 동작 확인/ }).first());
		expectAligned(pendingRow.y, readyRow.y);
		expectAligned(pendingRow.height, readyRow.height);
		await recordGeometry('mail', width, { pendingRow, readyRow });
	});

	test(`runs reserve daily summary and card layout at ${width}px`, async ({ page }) => {
		const fixture = await begin(page, width, '/runs', 'person.runs.list');
		const skeleton = page.getByTestId('run-list-loading-skeleton');
		await expect(page.getByRole('tab', { name: '작업', exact: true })).toBeVisible();
		const pendingCard = await bounds(skeleton);
		const pendingRow = await bounds(skeleton.locator(width < 768 ? '.divide-y > div' : 'tbody > tr').first());
		fixture.release('person.runs.list');
		const readyRow = await bounds(page.locator(width < 768 ? '[data-task-run-mobile-list] > div' : 'main tbody > tr').first());
		const readyCard = await bounds(page.locator('main [data-slot="card"]').filter({ has: page.locator('[data-task-run-mobile-list]') }));
		expectAligned(pendingCard.y, readyCard.y);
		expectAligned(pendingRow.y, readyRow.y);
		expectAligned(pendingRow.height, readyRow.height);
		await recordGeometry('runs', width, { pendingCard, pendingRow, readyCard, readyRow });
	});

	test(`run detail reserves result, facts and divider stages at ${width}px`, async ({ page }) => {
		const fixture = await begin(page, width, '/runs/dev-task-run-001', 'person.runs.detail');
		const skeleton = page.getByTestId('run-detail-loading-skeleton');
		const pendingFacts = await bounds(skeleton.locator('.grid.grid-cols-2'));
		const pendingTabList = skeleton.locator('[data-slot="underline-tabs-list"]');
		const pendingTabs = await bounds(pendingTabList);
		await expect(pendingTabList.locator('[data-slot="underline-tabs-trigger"]')).toHaveText(['진행', '전체 기록', '로그']);
		for (const tab of await pendingTabList.locator('[data-slot="underline-tabs-trigger"]').all()) await expect(tab).toBeDisabled();
		const pendingStep = await bounds(skeleton.locator('.divide-y > div').first());
		const pendingSecondStep = await bounds(skeleton.locator('.divide-y > div').nth(1));
		const pendingDivider = await skeleton.locator('.divide-y > div').first().evaluate(element => {
			const style = getComputedStyle(element);
			return { width: style.borderBottomWidth, color: style.borderBottomColor };
		});
		fixture.release('person.runs.detail');
		const readyFacts = await bounds(page.locator('main dl'));
		const readyTabs = await bounds(page.locator('main [data-slot="underline-tabs-list"]'));
		const readyStep = await bounds(page.locator('main [data-slot="accordion-item"]').first());
		const readySecondStep = await bounds(page.locator('main [data-slot="accordion-item"]').nth(1));
		const readyDivider = await page.locator('main [data-slot="accordion-item"]').first().evaluate(element => {
			const style = getComputedStyle(element);
			return { width: style.borderBottomWidth, color: style.borderBottomColor };
		});
		expectAligned(pendingFacts.y, readyFacts.y);
		expectAligned(pendingFacts.height, readyFacts.height);
		expectAligned(pendingTabs.y, readyTabs.y);
		expectAligned(pendingTabs.height, readyTabs.height);
		expectAligned(pendingStep.y, readyStep.y);
		expectAligned(pendingStep.height, readyStep.height);
		expectAligned(pendingSecondStep.y, readySecondStep.y);
		expect(pendingDivider).toEqual(readyDivider);
		expect(pendingDivider.width).toBe('1px');
		expect(pendingDivider.color).not.toBe('rgba(0, 0, 0, 0)');
		await recordGeometry('run-detail', width, { pendingFacts, pendingTabs, pendingStep, pendingSecondStep, pendingDivider, readyFacts, readyTabs, readyStep, readySecondStep, readyDivider });
	});

	test(`approval skeleton keeps the separated action footer at ${width}px`, async ({ page }) => {
		const fixture = await begin(page, width, '/runs/approvals', 'person.runs.list');
		const pendingCard = await bounds(page.getByTestId('approvals-loading-skeleton').locator('[data-slot="card"]').first());
		const pendingFooter = await bounds(page.getByTestId('approvals-loading-skeleton').locator('[data-slot="card-footer"]').first());
		fixture.release('person.runs.list');
		await expect(page.getByRole('button', { name: '이번만 승인', exact: true }).first()).toBeVisible();
		const readyCard = await bounds(page.locator('main section [data-slot="card"]').first());
		const readyFooter = await bounds(page.locator('main section [data-slot="card-footer"]').first());
		expectAligned(pendingCard.y, readyCard.y);
		expectAligned(pendingCard.height, readyCard.height);
		expectAligned(pendingFooter.y, readyFooter.y);
		expectAligned(pendingFooter.height, readyFooter.height);
		await recordGeometry('run-approvals', width, { pendingCard, pendingFooter, readyCard, readyFooter });
	});
}

for (const width of [1280, 390]) {
	test(`schedule skeleton reserves shared table and mobile action geometry at ${width}px`, async ({ page }) => {
		const fixture = await begin(page, width, '/memory/schedules', 'person.memory.schedules');
		const skeleton = page.getByTestId('schedule-loading-skeleton');
		const pendingRow = await bounds(skeleton.locator(width < 640 ? '.divide-y > div' : 'tbody > tr').first());
		const pendingHeader = width >= 640 ? await bounds(skeleton.locator('thead')) : undefined;
		fixture.release('person.memory.schedules');
		const readyRow = await bounds(page.locator(width < 640 ? 'main ul.divide-y > li' : 'main tbody > tr').first());
		const readyHeader = width >= 640 ? await bounds(page.locator('main thead')) : undefined;
		expectAligned(pendingRow.y, readyRow.y);
		expectAligned(pendingRow.height, readyRow.height);
		if (pendingHeader && readyHeader) {
			expectAligned(pendingHeader.y, readyHeader.y);
			expectAligned(pendingHeader.height, readyHeader.height);
		}
		await recordGeometry('schedules', width, { pendingRow, pendingHeader, readyRow, readyHeader });
	});
}
