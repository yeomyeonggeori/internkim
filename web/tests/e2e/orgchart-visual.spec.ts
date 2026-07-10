import { expect, test } from '@playwright/test';
import type { Locator, Page } from '@playwright/test';
import type { UsersResponse } from './admin-orgchart-fixtures';

const visualUsersResponse: UsersResponse = {
	availableGroups: [
		{ id: 'leadership', name: '경영팀' },
		{ id: 'shared', name: '공유팀' },
		{ id: 'skill', name: '스킬' }
	],
	records: [
		{
			userID: 'user-kim-first',
			handle: 'kim-first',
			name: '김첫째',
			email: 'kim-first@example.com',
			hireDate: '2026-01-01',
			role: 'member',
			jobTitle: 'CEO',
			primaryGroupID: 'leadership',
			groupIDs: ['leadership']
		},
		{
			userID: 'user-lee-second',
			handle: 'lee-second',
			name: '이둘째',
			email: 'lee-second@example.com',
			hireDate: '2026-01-01',
			role: 'member',
			jobTitle: 'CTO',
			primaryGroupID: 'leadership',
			groupIDs: ['leadership']
		},
		{
			userID: 'user-pptx',
			handle: 'pptx',
			name: 'PPTX Tester',
			email: 'pptx@example.com',
			hireDate: '2026-01-01',
			role: 'member',
			jobTitle: '깍두기',
			primaryGroupID: 'skill',
			groupIDs: ['skill']
		},
		{
			userID: 'user-park-staff',
			handle: 'park-staff',
			name: '박둘직원',
			email: 'park-staff@example.com',
			hireDate: '2026-02-01',
			role: 'member',
			jobTitle: '엔지니어',
			primaryGroupID: 'shared',
			groupIDs: ['shared'],
			supervisorID: 'user-kim-first'
		},
		{
			userID: 'user-new-staff',
			handle: 'new-staff',
			name: '새직원',
			email: 'new-staff@example.com',
			hireDate: '2026-02-01',
			role: 'member',
			jobTitle: '엔지니어',
			primaryGroupID: 'shared',
			groupIDs: ['shared'],
			supervisorID: 'user-lee-second'
		},
		{
			userID: 'user-extra-staff',
			handle: 'extra-staff',
			name: '이추가',
			email: 'extra-staff@example.com',
			hireDate: '2026-02-02',
			role: 'member',
			jobTitle: '엔지니어',
			primaryGroupID: 'shared',
			groupIDs: ['shared'],
			supervisorID: 'user-lee-second'
		}
	]
};

type ElementBox = {
	x: number;
	y: number;
	width: number;
	height: number;
};

test.describe('employee orgchart visual layout geometry', () => {
	test('renders organization members as full-width list rows', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 1000 });
		await mockVisualOrgchartDirectory(page);

		await page.goto('/orgchart/');

		const sharedTeam = page.getByTestId('orgchart-team-column-shared');
		const sharedLeaderBox = await visibleElementBox(sharedTeam.getByTestId('orgchart-person-node-user-park-staff'), 'shared leader');
		const newStaffBox = await visibleElementBox(sharedTeam.getByTestId('orgchart-person-node-user-new-staff'), 'new staff');
		const memberList = sharedTeam.getByTestId('orgchart-organization-members-shared');

		await expect(sharedTeam.getByText('팀장')).toHaveCount(1);
		await expect(memberList).toBeVisible();
		await expect(page.getByTestId('orgchart-team-column-shared')).toContainText('공유팀');
		expect(newStaffBox.y).toBeGreaterThan(sharedLeaderBox.y + sharedLeaderBox.height);
		expect(Math.abs(newStaffBox.x - sharedLeaderBox.x)).toBeLessThanOrEqual(2);
		expect(Math.abs(newStaffBox.width - sharedLeaderBox.width)).toBeLessThanOrEqual(2);
		await expectRowDivider(memberList);
	});
});

async function visibleElementBox(locator: Locator, label: string): Promise<ElementBox> {
	await expect(locator).toBeVisible();
	const elementBox = await locator.boundingBox();
	if (!elementBox) throw new Error(`Bounding box unavailable for ${label}`);
	return elementBox;
}

async function expectRowDivider(locator: Locator): Promise<void> {
	const firstRowBorderBottomWidth = await locator.locator(':scope > div').first().evaluate((element) => Number.parseFloat(getComputedStyle(element).borderBottomWidth));
	expect(firstRowBorderBottomWidth).toBeGreaterThan(0);
}

async function mockVisualOrgchartDirectory(page: Page): Promise<void> {
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'kim-first@example.com' } });
	});
	await page.route('**/admin/api/session', async (route) => {
		await route.fulfill({ status: 403, body: 'admin access required' });
	});
	await page.route('**/orgchart/api/people', async (route) => {
		await route.fulfill({ json: visualUsersResponse });
	});
}
