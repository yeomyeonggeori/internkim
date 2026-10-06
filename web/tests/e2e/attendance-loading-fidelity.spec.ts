import { expect, test, type Page, type Route } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import { AttendanceLoadingFixture } from './attendance-loading-fixture';

test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });
const folder = process.env.LOADING_EVIDENCE_DIR;

class AttendanceFidelityFixture extends AttendanceLoadingFixture {
	async answer(route: Route): Promise<void> {
		const tool = new URL(route.request().url()).pathname.split('/').at(-2);
		const input = route.request().postDataJSON().input;
		if (tool !== 'leave_list' || input.scope === 'all') return super.answer(route);
		await this.leave.wait();
		const leave = Array.from({ length: 3 }, (_, index) => ({ leaveID: `sample-own-leave-${index}`, personID: this.identity.memberID, person: '이샘플', kindID: 'annual', kind: '연차', days: 1, status: 'requested', isPaid: true, isDeducted: true, startDate: `2026-10-${15 + index}`, endDate: `2026-10-${15 + index}`, startsAt: `2026-10-${15 + index}T00:00:00+09:00`, endsAt: `2026-10-${16 + index}T00:00:00+09:00`, note: '개인 일정' }));
		await route.fulfill({ json: { result: { count: leave.length, leave, registeredKinds: ['annual'] } } });
	}
}

async function capture(page: Page, scene: string, width: number, state: string, geometry: unknown) {
	await page.evaluate(async () => {
		await document.fonts.ready;
		(document.activeElement as HTMLElement | null)?.blur();
		await Promise.all(document.getAnimations().filter(animation => animation.playState === 'running' && Number.isFinite(animation.effect?.getComputedTiming().endTime)).map(animation => animation.finished.catch(() => {})));
		window.scrollTo(0, 0);
		await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
	});
	expect(await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches)).toBe(true);
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
	if (!folder) return;
	await mkdir(folder, { recursive: true });
	const path = `${folder}/attendance-${scene}-${width}-${state}`;
	await page.screenshot({ path: `${path}.png`, animations: 'allow', caret: 'initial' });
	await writeFile(`${path}.json`, JSON.stringify({ viewport: page.viewportSize(), fixture: 'three short personal requests; three approval requests; three corrections; eight employees', geometry }, null, 2));
}

for (const width of [1280, 390, 320]) {
	for (const scene of [
		{ name: 'approvals', label: '휴가 승인', panel: 'leave-approval-view', row: '[data-testid^="leave-approval-request-"]', admin: true },
		{ name: 'leave-history', label: '내 휴가', panel: 'leave-history-view', row: '[data-testid="leave-history-row"]', admin: false },
		{ name: 'changes', label: '수정 내역', panel: 'hand-written-view', row: '[data-testid="hand-written-row"]', admin: true },
		{ name: 'leave-employees', label: '휴가 관리', panel: 'leave-management-view', row: '[data-testid="leave-management-employee-row"]', admin: true }
	]) {
		test(`attendance ${scene.name} loading matches the loaded row at ${width}px`, async ({ page }) => {
			await page.setViewportSize({ width, height: 900 });
			const fixture = new AttendanceFidelityFixture();
			const gate = scene.name === 'changes' ? fixture.changes : fixture.leave;
			gate.block();
			await fixture.install(page);
			await page.goto('/example-co/attendance');
			await expect(page.getByTestId('attendance-team-card')).toHaveCount(6);
			if (width < 768 && scene.admin) {
				await page.getByRole('button', { name: '관리', exact: true }).click();
				await page.getByRole('menuitem', { name: scene.label, exact: true }).click();
			} else await page.getByRole('button', { name: scene.label, exact: true }).filter({ visible: true }).click();
			const panel = page.getByTestId(scene.panel);
			const loading = panel.getByTestId('attendance-list-loading');
			await expect(loading).toBeVisible();
			await expect.poll(() => gate.reads).toBeGreaterThan(0);
			const pendingRow = await loading.locator('[data-loading-row]:visible').first().boundingBox();
			expect(pendingRow).not.toBeNull();
			await capture(page, scene.name, width, 'loading', pendingRow);
			gate.release();
			await expect(loading).toHaveCount(0);
			const loadedRow = await panel.locator(`${scene.row}:visible`).first().boundingBox();
			expect(loadedRow).not.toBeNull();
			await capture(page, scene.name, width, 'loaded', loadedRow);
			for (const key of ['x', 'y', 'width', 'height'] as const) {
				expect(Math.abs(pendingRow![key] - loadedRow![key]), `${scene.name} ${width}px first-row ${key}`).toBeLessThanOrEqual(2);
			}
		});
	}
}
