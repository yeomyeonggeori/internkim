import { expect, test, type Page } from '@playwright/test';
import { signInToTheCentralPlane } from './central-plane-sign-in';

test.describe.configure({ mode: 'serial', timeout: 60_000 });
test.use({ locale: 'ko-KR' });

const customLeaveTypeName = 'E2E 중앙 휴가';
const companyHolidayName = 'E2E 창립기념일';

async function openSettings(page: Page): Promise<void> {
	await signInToTheCentralPlane(page, '/example-co/settings');
	await switchToAdministratorTab(page);
}

async function switchToAdministratorTab(page: Page): Promise<void> {
	await page.getByRole('tab', { name: '관리자' }).click();
	await page
		.getByTestId('attendance-work-settings')
		.waitFor({ state: 'visible', timeout: 20000 });
}

function leaveSettings(page: Page) {
	return page.getByTestId('attendance-leave-policy-settings');
}

test('an administrator changes the work mode and it survives a reload', async ({ page }) => {
	await openSettings(page);
	const workSettings = page.getByTestId('attendance-work-settings');
	await workSettings.getByRole('button', { name: '자율 근무제' }).click();
	await workSettings.getByRole('button', { name: '변경사항 저장' }).click();
	await expect(page.getByText('근무 설정을 저장했습니다.')).toBeVisible({ timeout: 20000 });

	await page.reload();
	await switchToAdministratorTab(page);
	await expect(workSettings.getByRole('button', { name: '자율 근무제' })).toHaveAttribute(
		'aria-pressed',
		'true'
	);
});

test('a leave type an administrator adds is offered on the leave request form', async ({ page }) => {
	await openSettings(page);
	const settings = leaveSettings(page);
	await settings.getByRole('button', { name: '휴가 종류 추가' }).click();
	await settings.getByLabel('이름').fill(customLeaveTypeName);
	await settings.getByRole('button', { name: '저장', exact: true }).click();
	await expect(page.getByText('휴가 설정을 저장했습니다.')).toBeVisible({ timeout: 20000 });

	await page.goto('/example-co/attendance');
	await page.getByRole('button', { name: '휴가 등록' }).click();
	await page.getByTestId('leave-request-type-trigger').click();
	await expect(page.getByRole('option', { name: customLeaveTypeName })).toBeVisible({
		timeout: 20000
	});
});

test('a leave type nobody has taken leave under disappears when it is removed', async ({ page }) => {
	await openSettings(page);
	const settings = leaveSettings(page);
	await settings.getByRole('button', { name: customLeaveTypeName }).click();
	await settings.getByRole('button', { name: '삭제', exact: true }).click();
	await page.getByRole('alertdialog').getByRole('button', { name: '삭제', exact: true }).click();
	await expect(page.getByText('휴가 종류를 삭제했습니다.')).toBeVisible({ timeout: 20000 });

	await page.reload();
	await switchToAdministratorTab(page);
	await expect(settings.getByRole('button', { name: customLeaveTypeName })).toHaveCount(0);
});

test('a company holiday an administrator adds survives a reload', async ({ page }) => {
	await openSettings(page);
	const holidays = page.getByTestId('company-holiday-settings');
	await holidays.getByRole('button', { name: '휴일 추가' }).click();
	await holidays.getByLabel('휴일명').fill(companyHolidayName);
	await holidays.getByLabel('날짜').fill('2026-07-17');
	await holidays.getByRole('button', { name: '저장', exact: true }).click();
	await expect(holidays.getByText('회사 휴일을 저장했습니다.')).toBeVisible({ timeout: 20000 });

	await page.reload();
	await switchToAdministratorTab(page);
	await expect(holidays.getByText(companyHolidayName)).toBeVisible();
});
