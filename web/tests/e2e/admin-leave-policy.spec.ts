import { expect, type Page, test } from '@playwright/test';
import type { AttendanceLeavePolicy, LeaveType } from '../../src/routes/admin/admin-types';

type MockAdminRole = 'admin' | 'operationsAdmin';

function leaveType(
	id: string,
	name: string,
	overrides: Partial<LeaveType> = {}
): LeaveType {
	return {
		id,
		systemKind: id,
		name,
		paid: id === 'annual',
		balanceMode: id === 'annual' ? 'annual' : 'none',
		grantCadence: id === 'annual' ? 'statutory' : 'none',
		grantAmountMilliDays: id === 'annual' ? 15000 : 0,
		expiryMode: id === 'annual' ? 'fiscalYearEnd' : 'none',
		carryoverEnabled: false,
		allowedUnits: ['fullDay'],
		isActive: true,
		isSystem: true,
		sortOrder: 0,
		...overrides
	};
}

function defaultPolicy(): AttendanceLeavePolicy {
	return {
		version: 1,
		fiscalYearStartMonth: 1,
		fiscalYearStartDay: 1,
		leaveTypes: [
			leaveType('annual', '연차', { allowedUnits: ['fullDay', 'halfDay', 'quarterDay'], sortOrder: 0 }),
			leaveType('sick', '병가', { allowedUnits: ['fullDay', 'halfDay', 'quarterDay'], sortOrder: 1 }),
			leaveType('bereavement', '경조휴가', { balanceMode: 'separate', grantCadence: 'manual', sortOrder: 2 }),
			leaveType('unpaid', '무급휴가', { allowedUnits: ['fullDay', 'halfDay', 'quarterDay'], sortOrder: 3 }),
			leaveType('other', '기타 휴가', { sortOrder: 4 })
		],
		updatedAt: ''
	};
}

function isAttendanceLeavePolicy(value: unknown): value is AttendanceLeavePolicy {
	if (!value || typeof value !== 'object') return false;
	return Reflect.get(value, 'version') === 1 && Array.isArray(Reflect.get(value, 'leaveTypes'));
}

async function mockAdminLeavePolicyPage(page: Page, role: MockAdminRole) {
	let policy = defaultPolicy();

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
		await route.fulfill({ json: { locale: 'ko' } });
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
	test('adds, reloads, and deactivates a custom leave type', async ({ page }) => {
		await mockAdminLeavePolicyPage(page, 'admin');
		await page.goto('/settings/?fleet_id=demo&section=attendanceSettings');

		const leavePolicySettings = page.getByTestId('attendance-leave-policy-settings');
		await expect(page.getByRole('tab', { name: '근태 설정', exact: true })).toBeVisible();
		await expect(
			leavePolicySettings.locator('[data-slot="card-title"]', { hasText: '근태 설정' })
		).toBeVisible();
		await expect(leavePolicySettings.getByRole('button', { name: /연차/ })).toBeVisible();
		await expect(leavePolicySettings.getByRole('button', { name: /병가/ })).toBeVisible();
		await expect(leavePolicySettings.getByText('회사 회계기간')).toHaveCount(0);
		await expect(leavePolicySettings.getByText('증빙 안내')).toHaveCount(0);
		await expect(
			leavePolicySettings.locator('[data-slot="card-title"]', {
				hasText: '휴가 종류 설정'
			})
		).toBeVisible();
		await expect(
			leavePolicySettings.getByText('기본 휴가', { exact: true })
		).toBeVisible();

		const nameInput = leavePolicySettings.getByLabel('이름');
		const cancelChanges = leavePolicySettings.getByRole('button', { name: '변경 취소' });
		await expect(cancelChanges).toBeDisabled();
		await nameInput.fill('임시 연차 이름');
		await expect(cancelChanges).toBeEnabled();
		await cancelChanges.click();
		await expect(nameInput).toHaveValue('연차');

		await leavePolicySettings.getByRole('button', { name: '휴가 종류 추가' }).click();
		await leavePolicySettings.getByLabel('이름').fill('저장하지 않을 휴가');
		await leavePolicySettings.getByRole('button', { name: '변경 취소' }).click();
		await expect(leavePolicySettings.getByLabel('이름')).toHaveValue('연차');

		await leavePolicySettings.getByRole('button', { name: '휴가 종류 추가' }).click();
		await leavePolicySettings.getByLabel('이름').fill('가족돌봄 휴가');
		await leavePolicySettings.getByRole('switch', { name: '반일' }).click();
		await leavePolicySettings.getByRole('button', { name: '저장' }).click();

		await expect(
			leavePolicySettings.getByRole('button', { name: /가족돌봄 휴가/ })
		).toBeVisible();
		await page.reload();
		const reloadedLeavePolicySettings = page.getByTestId('attendance-leave-policy-settings');
		await expect(
			reloadedLeavePolicySettings.getByRole('button', { name: /가족돌봄 휴가/ })
		).toBeVisible();

		await reloadedLeavePolicySettings
			.getByRole('button', { name: /가족돌봄 휴가/ })
			.click();
		await reloadedLeavePolicySettings.getByRole('switch', { name: '활성' }).click();
		await reloadedLeavePolicySettings.getByRole('button', { name: '저장' }).click();
		await expect(
			reloadedLeavePolicySettings
				.getByRole('button', { name: /가족돌봄 휴가/ })
				.getByText('비활성')
		).toBeVisible();
	});

	test('hides the attendance settings tab from operations admins', async ({ page }) => {
		await mockAdminLeavePolicyPage(page, 'operationsAdmin');
		await page.goto('/settings/?fleet_id=demo&section=attendanceSettings');

		await expect(page.getByRole('tab', { name: '근태 설정', exact: true })).toHaveCount(0);
		await expect(page.locator('[data-slot="card-title"]', { hasText: '근태 설정' })).toHaveCount(0);
		await expect(page.getByRole('tab', { name: '사용자' })).toBeVisible();
	});
});
