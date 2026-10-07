import { expect, test, type Locator, type Page } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import { WorkspaceLoadingFixture, prepareWorkspaceLoading } from './workspace-loading-fixture';
import { taskWeekOfDate } from '../../src/lib/task/task-week-code';

const output = process.env.SKELETON_FIDELITY_DIRECTORY;
test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

class TaskFidelityFixture extends WorkspaceLoadingFixture {
	tool(name: string, input: Record<string, unknown>): unknown {
		if (name !== 'task_board_get' && name !== 'task_list') return super.tool(name, input);
		return {
			scope: 'all', count: 1,
			tasks: [{ taskID: '30000000-0000-4000-8000-000000000001', content: '확인된 샘플 업무', status: 'planned', size: 'M', ownerID: '10000000-0000-4000-8000-000000000001', ownerName: '이샘플', participantIDs: ['10000000-0000-4000-8000-000000000001'], participantNames: ['이샘플'], createdAt: '2026-10-06T03:00:00Z', weekCode: taskWeekOfDate(new Date('2026-10-06T03:00:00Z')).code }],
			registeredLabels: { businesses: [], types: [], sizes: ['S', 'M', 'L'], statuses: ['requested', 'planned', 'in_progress', 'completed', 'paused', 'rejected', 'stopped'] }
		};
	}
}

async function settle(page: Page): Promise<void> {
	await page.evaluate(async () => {
		await document.fonts.ready;
		if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
		for (const element of document.querySelectorAll('main, [data-app-shell-scroll]')) element.scrollTo({ top: 0, behavior: 'instant' });
		window.scrollTo({ top: 0, behavior: 'instant' });
		await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
	});
}

async function box(locator: Locator) {
	await expect(locator).toBeVisible();
	const bounds = await locator.boundingBox();
	if (!bounds) throw new Error('Visible element has no bounding box');
	return bounds;
}

async function headings(locator: Locator): Promise<string[]> {
	return locator.locator('th:visible').evaluateAll(elements => elements.map(element => {
		const clone = element.cloneNode(true);
		if (!(clone instanceof HTMLElement)) return '';
		for (const hidden of clone.querySelectorAll('.sr-only')) hidden.remove();
		return clone.textContent?.trim() ?? '';
	}));
}

async function capture(page: Page, name: string, anchor: Locator): Promise<void> {
	if (!output) return;
	await mkdir(output, { recursive: true });
	const readScroll = () => page.evaluate(() => ({ windowX: window.scrollX, windowY: window.scrollY, regions: Array.from(document.querySelectorAll('main, [data-app-shell-scroll], [role="tabpanel"]')).map(element => ({ tag: element.tagName, role: element.getAttribute('role'), top: element.scrollTop, left: element.scrollLeft })) }));
	const before = { scroll: await readScroll(), anchor: await box(anchor) };
	await page.screenshot({ path: `${output}/${name}.png`, animations: 'disabled' });
	const after = { scroll: await readScroll(), anchor: await box(anchor) };
	expect(after).toEqual(before);
	await writeFile(`${output}/${name}.json`, JSON.stringify({ route: new URL(page.url()).pathname, viewport: page.viewportSize(), reducedMotionVerified: await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches), populatedReady: name.endsWith('-ready'), before, after }, null, 2));
}

for (const width of [1280, 390, 320]) {
	for (const scene of ['crm', 'organization']) {
		test(`${scene} loading preserves loaded structure at ${width}px`, async ({ page }) => {
			await page.setViewportSize({ width, height: 900 });
			const fixture = new WorkspaceLoadingFixture();
			const gate = scene === 'crm' ? 'crm_organization_list' : 'person_list';
			fixture.hold(gate);
			await prepareWorkspaceLoading(page, fixture);
			await page.goto(`/example-co/${scene}`);
			await expect.poll(() => fixture.requested.has(gate)).toBe(true);
			const loading = page.getByTestId(scene === 'crm' ? 'crm-loading-skeleton' : 'organization-skeleton');
			await expect(loading).toBeVisible();
			if (scene === 'organization') await expect(loading.getByRole('button', { name: '구성원 초대' })).toBeVisible();
			await settle(page);
			const pendingContent = scene === 'crm' ? loading.locator('table') : loading.locator('[data-slot="item"]').first();
			const pending = await box(pendingContent);
			const pendingHeaders = scene === 'crm' ? await headings(loading) : [];
			await capture(page, `${scene}-${width}-loading`, pendingContent);
			fixture.release(gate);
			await expect(loading).toHaveCount(0);
			await settle(page);
			const readyContent = scene === 'crm' ? page.locator('main table').filter({ visible: true }) : page.locator('[data-testid^="organization-person-card-"]').first();
			const ready = await box(readyContent);
			await capture(page, `${scene}-${width}-ready`, readyContent);
			expect(Math.abs(pending.x - ready.x)).toBeLessThanOrEqual(1);
			expect(Math.abs(pending.y - ready.y)).toBeLessThanOrEqual(1);
			expect(Math.abs(pending.width - ready.width)).toBeLessThanOrEqual(1);
			if (scene === 'organization') expect(Math.abs(pending.height - ready.height)).toBeLessThanOrEqual(1);
			else expect(pendingHeaders).toEqual(await headings(readyContent));
			expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
		});
	}
	for (const variant of ['list', 'members']) {
		test(`task ${variant} loading preserves loaded structure at ${width}px`, async ({ page }) => {
			await page.setViewportSize({ width, height: 900 });
			const fixture = new TaskFidelityFixture();
			fixture.hold('task_list');
			await prepareWorkspaceLoading(page, fixture);
			await page.goto('/example-co/task');
			await expect(page.locator('[data-task-ready="true"]')).toBeVisible();
			await page.getByRole('tab', { name: variant === 'list' ? '표' : '구성원', exact: true }).click();
			await expect.poll(() => fixture.requested.has('task_list')).toBe(true);
			const loading = page.locator(`[data-task-content-skeleton="${variant}"]`);
			await expect(loading).toBeVisible();
			await settle(page);
			const pendingContent = width >= 640 ? loading.locator('table') : loading.locator(variant === 'list' ? '[data-task-list-row-skeleton]' : '[data-task-member-skeleton]').first();
			const pending = await box(pendingContent);
			const pendingHeaders = width >= 640 ? await headings(loading) : [];
			await capture(page, `task-${variant}-${width}-loading`, pendingContent);
			fixture.release('task_list');
			await expect(loading).toHaveCount(0);
			const readyPane = page.locator('main');
			await settle(page);
			const readyContent = width >= 640 ? readyPane.locator('table').filter({ visible: true }) : readyPane.locator(variant === 'list' ? '[role="tabpanel"][data-state="active"] ul > li' : '[data-slot="card-content"] ul > li').first();
			const ready = await box(readyContent);
			await capture(page, `task-${variant}-${width}-ready`, readyContent);
			expect(Math.abs(pending.x - ready.x)).toBeLessThanOrEqual(1);
			expect(Math.abs(pending.y - ready.y)).toBeLessThanOrEqual(1);
			if (width < 640) {
				expect(Math.abs(pending.width - ready.width)).toBeLessThanOrEqual(1);
				expect(Math.abs(pending.height - ready.height)).toBeLessThanOrEqual(1);
			} else expect(pendingHeaders).toEqual(await headings(readyContent));
			expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
		});
	}
}
