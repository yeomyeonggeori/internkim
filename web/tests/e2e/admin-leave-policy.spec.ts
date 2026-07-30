import { expect, type Page, test } from '@playwright/test';
import { createDefaultAttendanceLeavePolicy } from '../../dev-attendance-leave-policy-mock';
import type { AttendanceLeavePolicy } from '../../src/routes/admin/admin-types';
import { mockBuzzDisabled } from './buzz-test-routes';

type MockAdminRole = 'admin' | 'operationsAdmin';

function isAttendanceLeavePolicy(value: unknown): value is AttendanceLeavePolicy {
	if (!value || typeof value !== 'object') return false;
	return Reflect.get(value, 'version') === 2 && Array.isArray(Reflect.get(value, 'leaveTypes'));
}

async function mockAdminLeavePolicyPage(
	page: Page,
	role: MockAdminRole,
	locale: 'ko' | 'en' = 'ko'
) {
	let policy = createDefaultAttendanceLeavePolicy();

	await mockBuzzDisabled(page);
	await page.route('**/admin/api/session', async (route) => {
		await route.fulfill({
			json: {
				email: role === 'admin' ? 'admin@example.com' : 'operator@example.com',
				claimedAdminEmail: 'admin@example.com',
				isAdmin: role === 'admin',
				role,
				isClaimed: true,
				bootstrapStatus: 'claimed',
				deviceManaged: true
			}
		});
	});
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale } });
	});
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'admin@example.com' } });
	});
	await page.route('**/admin/api/users?includePolicy=true', async (route) => {
		await route.fulfill({ json: { records: [], availableCircles: [], availableGroups: [] } });
	});
	await page.route('**/admin/api/attendance-leave-policy', async (route) => {
		if (route.request().method() === 'PUT') {
			const requestedPolicy: unknown = route.request().postDataJSON();
			if (!isAttendanceLeavePolicy(requestedPolicy)) throw new Error('invalid attendance leave policy request');
			policy = {
				...requestedPolicy,
				updatedAt: '2026-07-28T00:00:00Z',
				leaveTypes: requestedPolicy.leaveTypes.map((currentLeaveType) => ({
					...currentLeaveType,
					id: currentLeaveType.id || 'custom-family-care'
				}))
			};
		}
		await route.fulfill({ json: policy });
	});
}

test.describe('admin leave policy settings', () => {
	test('edits annual policy and manages a custom leave type', async ({ page }) => {
		await mockAdminLeavePolicyPage(page, 'admin');
		await page.goto('/settings/?fleet_id=demo&section=attendanceSettings');

		const leavePolicySettings = page.getByTestId('attendance-leave-policy-settings');
		await expect(page.getByRole('tab', { name: '근태 설정', exact: true })).toBeVisible();
		await expect(
			leavePolicySettings.locator('[data-slot="card-title"]', { hasText: '근태 설정' })
		).toBeVisible();
		await expect(leavePolicySettings.getByRole('button', { name: /연차/ })).toBeVisible();
		await expect(leavePolicySettings.getByRole('button', { name: /병가/ })).toBeVisible();
		await expect(leavePolicySettings.getByRole('button', { name: /포상휴가/ })).toBeVisible();
		await expect(leavePolicySettings.getByRole('button', { name: /육아휴직/ })).toBeVisible();
		const leavePolicyListScroll = leavePolicySettings.getByTestId('leave-policy-list-scroll');
		const scrollMetrics = await leavePolicyListScroll.evaluate((element) => ({
			clientHeight: element.clientHeight,
			scrollHeight: element.scrollHeight,
			overflowY: getComputedStyle(element).overflowY
		}));
		expect(scrollMetrics.scrollHeight).toBeGreaterThan(scrollMetrics.clientHeight);
		expect(scrollMetrics.overflowY).toBe('auto');
		await expect(leavePolicySettings.getByTestId('leave-policy-list-scroll-fade')).toBeVisible();
		await leavePolicyListScroll.evaluate((element) => {
			element.scrollTop = element.scrollHeight;
			element.dispatchEvent(new Event('scroll'));
		});
		await expect(leavePolicySettings.getByTestId('leave-policy-list-scroll-fade')).toHaveCount(0);
		await expect(
			leavePolicySettings.getByRole('button', { name: '휴가 종류 추가' })
		).toBeVisible();
		await expect(leavePolicySettings.getByText('회사 회계기간')).toHaveCount(0);
		await expect(leavePolicySettings.getByText('증빙 안내')).toHaveCount(0);
		await expect(
			leavePolicySettings.locator('[data-slot="card-title"]', {
				hasText: '연차'
			})
		).toBeVisible();
		await expect(
			leavePolicySettings.getByText('기본 휴가', { exact: true })
		).toBeVisible();
		await expect(
			leavePolicySettings.getByRole('switch', { name: '활성', exact: true })
		).toHaveCount(0);

		await expect(leavePolicySettings.getByLabel('부여 주기')).toBeEnabled();
		await expect(
			leavePolicySettings.getByRole('switch', { name: '휴가 현황 합계에 포함' })
		).toBeChecked();
		await expect(leavePolicySettings.getByText('허용 단위', { exact: true })).toHaveCount(0);
		const editorScroll = leavePolicySettings.getByTestId('leave-policy-editor-scroll');
		const editorFooter = leavePolicySettings.getByTestId('leave-policy-editor-footer');
		const annualFooterBox = await editorFooter.boundingBox();
		expect(annualFooterBox).not.toBeNull();
		await expect(leavePolicySettings.getByTestId('leave-policy-editor-scroll-fade')).toBeVisible();
		await editorScroll.evaluate((element) => {
			element.scrollTop = element.scrollHeight;
			element.dispatchEvent(new Event('scroll'));
		});
		await expect(leavePolicySettings.getByTestId('leave-policy-editor-scroll-fade')).toHaveCount(0);
		await leavePolicySettings.getByRole('button', { name: /^병가/ }).click();
		await expect(
			leavePolicySettings.locator('[data-slot="card-title"]', { hasText: '병가' })
		).toBeVisible();
		const sickFooterBox = await editorFooter.boundingBox();
		expect(sickFooterBox).not.toBeNull();
		expect(Math.abs((sickFooterBox?.y ?? 0) - (annualFooterBox?.y ?? 0))).toBeLessThanOrEqual(1);
		await expect(leavePolicySettings.getByTestId('leave-policy-editor-scroll-fade')).toHaveCount(0);
		await leavePolicySettings.getByRole('button', { name: /^연차/ }).click();
		const grantDaysInput = leavePolicySettings.getByLabel('연간 부여 일수');
		await expect(grantDaysInput).toBeEnabled();
		await grantDaysInput.fill('14');
		await leavePolicySettings.getByRole('button', { name: '저장' }).click();
		await expect(leavePolicySettings.getByRole('status')).toContainText(
			'근태 설정을 저장했습니다.'
		);
		await expect(grantDaysInput).toHaveValue('14');
		await leavePolicySettings.getByRole('button', { name: '소멸 방식' }).click();
		await page.getByRole('option', { name: '없음', exact: true }).click();
		await leavePolicySettings.getByRole('button', { name: '저장' }).click();
		await expect(
			page.getByRole('alertdialog').getByText('소멸·이월 정책을 변경할까요?')
		).toBeVisible();
		await page.getByRole('button', { name: '변경 저장' }).click();
		await expect(leavePolicySettings.getByRole('status')).toContainText(
			'근태 설정을 저장했습니다.'
		);

		const nameInput = leavePolicySettings.getByLabel('이름');
		const cancelChanges = leavePolicySettings.getByRole('button', { name: '변경 취소' });
		await expect(cancelChanges).toBeDisabled();
		await nameInput.fill('임시 연차 이름');
		await expect(cancelChanges).toBeEnabled();
		await cancelChanges.click();
		await expect(nameInput).toHaveValue('연차');

		await leavePolicySettings.getByRole('button', { name: '휴가 종류 추가' }).click();
		const pendingLeaveType = leavePolicySettings.getByTestId('pending-leave-type');
		await expect(pendingLeaveType).toContainText('새 휴가');
		await expect(pendingLeaveType).toContainText('저장 전');
		await leavePolicySettings.getByLabel('이름').fill('저장하지 않을 휴가');
		await expect(pendingLeaveType).toContainText('저장하지 않을 휴가');
		await leavePolicySettings.getByRole('button', { name: '변경 취소' }).click();
		await expect(pendingLeaveType).toHaveCount(0);
		await expect(leavePolicySettings.getByLabel('이름')).toHaveValue('연차');

		await leavePolicySettings.getByRole('button', { name: '휴가 종류 추가' }).click();
		await leavePolicySettings.getByLabel('이름').fill('회사 특별 휴가');
		await expect(leavePolicySettings.getByTestId('pending-leave-type')).toContainText(
			'회사 특별 휴가'
		);
		await leavePolicySettings.getByRole('button', { name: '차감 방식' }).click();
		await page.getByRole('option', { name: '연차 차감' }).click();
		await expect(
			leavePolicySettings.getByText(
				'이 휴가는 별도 잔액을 만들지 않고 연차 잔액에서 차감합니다.'
			)
		).toBeVisible();
		await expect(leavePolicySettings.getByLabel('부여 주기')).toHaveCount(0);
		await leavePolicySettings.getByRole('switch', { name: '반차', exact: true }).click();
		await leavePolicySettings.getByRole('button', { name: '저장' }).click();

		await expect(leavePolicySettings.getByTestId('pending-leave-type')).toHaveCount(0);
		await expect(
			leavePolicySettings.getByRole('button', { name: /회사 특별 휴가/ })
		).toBeVisible();
		await page.reload();
		const reloadedLeavePolicySettings = page.getByTestId('attendance-leave-policy-settings');
		await expect(
			reloadedLeavePolicySettings.getByRole('button', { name: /회사 특별 휴가/ })
		).toBeVisible();

		await reloadedLeavePolicySettings
			.getByRole('button', { name: /회사 특별 휴가/ })
			.click();
		await expect(reloadedLeavePolicySettings.getByLabel('이름')).toHaveValue('회사 특별 휴가');
		await expect(
			reloadedLeavePolicySettings.getByRole('button', { name: '사용 중지' })
		).toHaveCount(0);
		await expect(
			reloadedLeavePolicySettings.getByRole('button', { name: '다시 사용' })
		).toHaveCount(0);
		await reloadedLeavePolicySettings.getByRole('button', { name: '휴가 종류 제거' }).click();
		await expect(page.getByRole('alertdialog')).toContainText(
			'‘회사 특별 휴가’ 휴가 종류는 신규 신청과 설정 목록에서 제거됩니다.'
		);
		await page.getByRole('alertdialog').getByRole('button', { name: '제거' }).click();
		await expect(
			reloadedLeavePolicySettings.getByRole('button', { name: /회사 특별 휴가/ })
		).toHaveCount(0);
	});

	test('hides the attendance settings tab from operations admins', async ({ page }) => {
		await mockAdminLeavePolicyPage(page, 'operationsAdmin');
		await page.goto('/settings/?fleet_id=demo&section=attendanceSettings');

		await expect(page.getByRole('tab', { name: '근태 설정', exact: true })).toHaveCount(0);
		await expect(page.locator('[data-slot="card-title"]', { hasText: '근태 설정' })).toHaveCount(0);
		await expect(page.getByRole('tab', { name: '사용자' })).toBeVisible();
	});

	test('localizes default leave names and preserves administrator names in English', async ({
		page
	}) => {
		await mockAdminLeavePolicyPage(page, 'admin', 'en');
		await page.goto('/settings/?fleet_id=demo&section=attendanceSettings');

		const settings = page.getByTestId('attendance-leave-policy-settings');
		await expect(settings.getByTestId('leave-policy-list-scroll')).toContainText('Annual leave');
		await expect(settings.getByLabel('Name')).toHaveValue('Annual leave');
		await settings.getByLabel('Name').fill('Company annual leave');
		await settings.getByRole('button', { name: 'Save', exact: true }).click();
		await expect(settings.getByLabel('Name')).toHaveValue('Company annual leave');
		await page.reload();
		await expect(
			page.getByTestId('attendance-leave-policy-settings').getByLabel('Name')
		).toHaveValue('Company annual leave');
	});
});
