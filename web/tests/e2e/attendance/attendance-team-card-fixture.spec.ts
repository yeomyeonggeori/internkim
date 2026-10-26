import { expect, test } from '@playwright/test';
import { AttendanceLoadingFixture } from './attendance-loading-fixture';

test.use({ locale: 'ko-KR' });

async function openDashboard(page: import('@playwright/test').Page): Promise<Record<string, unknown>[]> {
	const memberReads: Record<string, unknown>[] = [];
	page.on('request', (request) => {
		if (!request.url().endsWith('/attendance_team_dashboard_get/invoke')) return;
		const input = (request.postDataJSON() as { input: Record<string, unknown> }).input;
		if (input.pageKind === 'members') memberReads.push(input);
	});
	await new AttendanceLoadingFixture().install(page);
	await page.goto('/example-co/attendance');
	await expect(page.getByTestId('attendance-team-card')).toHaveCount(6);
	return memberReads;
}

test('a team card names who is in each state and opens that state', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 900 });
	const memberReads = await openDashboard(page);
	const card = page.getByTestId('attendance-team-card').first();
	await expect(card.getByText('내 팀')).toBeVisible();
	const done = card.getByTestId('attendance-team-status-done');
	await expect(done).toContainText('박예시2');
	await expect(card.getByTestId('attendance-team-status-working')).toContainText('이샘플1 외 4명');
	await card.screenshot({ path: test.info().outputPath('team-card.png') });

	await done.click();
	await expect(page.getByTestId('team-employee-page')).toBeVisible();
	expect(memberReads.at(-1)).toMatchObject({ selectedTeamKey: 'team-0', statusFilter: 'done' });
});

test('a company tile opens the whole company in that state', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 900 });
	const memberReads = await openDashboard(page);
	await page.getByTestId('attendance-company-done').click();
	await expect(page.getByRole('heading', { name: '회사 전체' })).toBeVisible();
	const read = memberReads.at(-1);
	expect(read).toMatchObject({ statusFilter: 'done' });
	expect(read).not.toHaveProperty('selectedTeamKey');
	const employees = page.getByTestId('team-employee-page');
	await expect(employees.locator('[data-testid^="team-employee-"]')).toHaveCount(8);
	await employees.screenshot({ path: test.info().outputPath('company-employees.png') });
});
