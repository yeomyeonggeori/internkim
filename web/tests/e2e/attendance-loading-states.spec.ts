import { expect, test, type Page } from '@playwright/test';
import { AttendanceLoadingFixture } from './attendance-loading-fixture';

const phase = process.env.LOADING_EVIDENCE_PHASE || 'after';
const screenshots = process.env.LOADING_EVIDENCE_DIR;
async function capture(page: Page, scene: string, width: number): Promise<void> {
	await page.evaluate(() => {
		let element: HTMLElement | null = document.querySelector('[data-testid="attendance-team-dashboard"]');
		while (element) { element.scrollTop = 0; element = element.parentElement; }
		window.scrollTo(0, 0);
	});
	if (screenshots) await page.screenshot({ animations: 'disabled', path: `${screenshots}/attendance-${scene}-${width}-${phase}.png` });
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
}
test.use({ locale: 'ko-KR' });

for (const width of [1280, 390, 320]) {
	test(`attendance loading scenes retain geometry at ${width}px`, async ({ page }) => {
		test.setTimeout(90_000);
		await page.setViewportSize({ width, height: 900 });
		const fixture = new AttendanceLoadingFixture();
		fixture.current.block(); fixture.teams.block(); fixture.members.block(); fixture.progress.block(); fixture.month.block();
		await fixture.install(page);
		await page.goto('/example-co/attendance');
		await expect(page.getByTestId('attendance-skeleton').first()).toBeVisible();
		const initialCard = phase === 'before' ? null : await page.locator('[data-attendance-skeleton="dashboard"] [data-slot="card"]').first().boundingBox();
		const initialMetric = phase === 'before' ? null : await page.locator('[data-attendance-skeleton="dashboard"] [data-slot="card"]').nth(1).boundingBox();
		await capture(page, 'initial', width);
		fixture.current.release();
		await expect(page.getByTestId('attendance-own-strip')).toBeVisible();
		await expect.poll(() => fixture.teams.reads).toBeGreaterThan(0);
		if (phase !== 'before') await expect(page.locator('[data-attendance-skeleton="teams"]')).toBeVisible();
		await capture(page, 'teams-pending', width);
		fixture.teams.release();
		await expect(page.getByTestId('attendance-team-card')).toHaveCount(6);
		if (phase !== 'before') {
			const ownCard = await page.getByTestId('attendance-own-strip').boundingBox();
			const metric = await page.getByTestId('attendance-company-summary').locator('[data-slot="card"]').first().boundingBox();
			if (!initialCard || !initialMetric || !ownCard || !metric) throw new Error('Attendance card geometry was not measurable');
			expect(Math.abs(initialCard.y - ownCard.y)).toBeLessThanOrEqual(1);
			expect(Math.abs(initialCard.height - ownCard.height)).toBeLessThanOrEqual(2);
			expect(Math.abs(initialMetric.height - metric.height)).toBeLessThanOrEqual(2);
		}
		await capture(page, 'teams-loaded', width);
		await page.getByTestId('attendance-team-card').first().getByRole('button', { name: /구성원|직원/ }).click();
		await expect.poll(() => fixture.members.reads).toBeGreaterThan(0);
		if (phase !== 'before') await expect(page.locator('[data-attendance-skeleton="members"]')).toBeVisible();
		await capture(page, 'members-pending', width);
		fixture.members.release();
		await expect(page.getByTestId('team-employee-page')).toBeVisible();
		await expect.poll(() => fixture.progress.reads).toBeGreaterThan(0);
		if (phase !== 'before') {
			await expect(page.getByTestId('employee-today-progress-loading')).toHaveCount(8);
			await expect(page.getByTestId('team-employee-page').getByTestId('employee-today-progress')).toHaveCount(0);
		}
		await capture(page, 'progress-pending', width);
		fixture.progress.release();
		await expect(page.getByTestId('team-employee-page').getByTestId('employee-today-progress')).toHaveCount(8);
		await capture(page, 'members-loaded', width);
		await page.getByTestId('team-employee-page').getByRole('button').first().click();
		await expect(page.getByRole('dialog')).toBeVisible();
		await expect.poll(() => fixture.month.reads).toBeGreaterThan(0);
		await capture(page, 'month-pending', width);
		fixture.month.release();
		await expect(page.getByTestId('team-status-grid')).toBeVisible();
		await capture(page, 'month-loaded', width);
	});
}

test('attendance error is terminal and retry can show content', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 900 });
	const fixture = new AttendanceLoadingFixture();
	fixture.current.fail = true;
	await fixture.install(page);
	await page.goto('/example-co/attendance');
	await expect(page.getByRole('alert').filter({ hasText: '샘플 응답' })).toBeVisible();
	if (phase !== 'before') await expect(page.getByTestId('attendance-skeleton')).toHaveCount(0);
	await capture(page, 'initial-error', 1280);
	fixture.current.fail = false;
	await page.getByRole('button', { name: '새로고침', exact: true }).last().click();
	await expect(page.getByTestId('attendance-team-card')).toHaveCount(6);
});

for (const width of [1280, 390, 320]) {
	for (const scene of [
		{ name: 'leave-history', label: '내 휴가', testId: 'leave-history-view', admin: false },
		{ name: 'approvals', label: '휴가 승인', testId: 'leave-approval-view', admin: true },
		{ name: 'leave-management', label: '휴가 관리', testId: 'leave-management-view', admin: true },
		{ name: 'changes', label: '수정 내역', testId: 'hand-written-view', admin: true }
	]) {
		test(`attendance ${scene.name} pending and ready at ${width}px`, async ({ page }) => {
			await page.setViewportSize({ width, height: 900 });
			const fixture = new AttendanceLoadingFixture();
			const gate = scene.name === 'changes' ? fixture.changes : fixture.leave;
			gate.block();
			await fixture.install(page);
			await page.goto('/example-co/attendance');
			await expect(page.getByTestId('attendance-team-card')).toHaveCount(6);
			if (width < 768 && scene.admin) {
				await page.getByRole('button', { name: '관리', exact: true }).click();
				await page.getByRole('menuitem', { name: scene.label, exact: true }).click();
			} else await page.getByRole('button', { name: scene.label, exact: true }).filter({ visible: true }).click();
			await expect.poll(() => gate.reads).toBeGreaterThan(0);
			if (phase !== 'before') await expect(page.locator('[data-slot="skeleton"]').filter({ visible: true }).first()).toBeVisible();
			await capture(page, `${scene.name}-pending`, width);
			gate.release();
			await expect(page.locator('[data-slot="skeleton"]:visible')).toHaveCount(0);
			await capture(page, `${scene.name}-loaded`, width);
		});
	}
}
