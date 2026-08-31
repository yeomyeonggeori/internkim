import { expect, test } from '@playwright/test';
import type { Locator, Page } from '@playwright/test';
import type { UsersResponse } from './admin-organization-fixtures';

const visualUsersResponse: UsersResponse = {
	availableGroups: [
		{ id: 'leadership', name: '경영팀' },
		{ id: 'shared', name: '공유팀', parentID: 'leadership' },
		{ id: 'skill', name: '스킬', parentID: 'leadership' }
	],
	records: [
		{
			memberID: 'user-kim-first',
			handle: 'kim-first',
			name: '김첫째',
			email: 'kim-first@example.com',
			hireDate: '2026-01-01',
			role: 'member',
			jobTitle: 'CEO',
			groupID: 'leadership'
		},
		{
			memberID: 'user-leesample-second',
			handle: 'leesample-second',
			name: '이둘째',
			email: 'leesample-second@example.com',
			hireDate: '2026-01-01',
			role: 'member',
			jobTitle: 'CTO',
			groupID: 'leadership'
		},
		{
			memberID: 'user-pptx',
			handle: 'pptx',
			name: 'PPTX Tester',
			email: 'pptx@example.com',
			hireDate: '2026-01-01',
			role: 'member',
			jobTitle: '깍두기',
			groupID: 'skill'
		},
		{
			memberID: 'user-park-member',
			handle: 'park-member',
			name: '박둘직원',
			email: 'park-member@example.com',
			hireDate: '2026-02-01',
			role: 'member',
			jobTitle: '엔지니어',
			groupID: 'shared',
			supervisorID: 'user-kim-first'
		},
		{
			memberID: 'user-new-member',
			handle: 'new-member',
			name: '새직원',
			email: 'new-member@example.com',
			hireDate: '2026-02-01',
			role: 'member',
			jobTitle: '엔지니어',
			groupID: 'shared',
			supervisorID: 'user-leesample-second'
		},
		{
			memberID: 'user-extra-member',
			handle: 'extra-member',
			name: '이추가',
			email: 'extra-member@example.com',
			hireDate: '2026-02-02',
			role: 'member',
			jobTitle: '엔지니어',
			groupID: 'shared',
			supervisorID: 'user-leesample-second'
		}
	]
};

type ElementBox = {
	x: number;
	y: number;
	width: number;
	height: number;
};

test.describe('employee organization visual layout geometry', () => {
	test('renders a fixed organization sidebar and hierarchy people layer', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 900 });
		await mockVisualOrganizationDirectory(page);

		await page.goto('/organization/');

		const sidebar = page.getByTestId('organization-sidebar');
		await expect(sidebar).toBeVisible();
		await expect(sidebar.getByTestId('organization-root')).toContainText('전체');
		await expect(sidebar.getByTestId('organization-row-leadership')).toBeVisible();
		await expect(sidebar.getByTestId('organization-row-shared')).toBeVisible();
		await expect(sidebar.getByTestId('organization-avatar-stack').first()).toBeVisible();
		await expect(page.getByTestId('organization-people-layer')).toBeVisible();
		await expect(page.getByRole('button', { name: /명 더 보기/ })).toHaveCount(0);

		const sidebarBox = await visibleElementBox(sidebar, 'organization sidebar');
		expect(sidebarBox.width).toBeGreaterThanOrEqual(260);
		expect(sidebarBox.width).toBeLessThanOrEqual(280);
	});

	test('renders organization members as an even card grid', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 1000 });
		await mockVisualOrganizationDirectory(page);

		await page.goto('/organization/');

		const sharedTeam = page.getByTestId('organization-section-shared');
		const sharedLeaderBox = await visibleElementBox(sharedTeam.getByTestId('organization-person-card-user-park-member'), 'shared leader');
		const newMemberBox = await visibleElementBox(sharedTeam.getByTestId('organization-person-card-user-new-member'), 'new member');
		const extraMemberBox = await visibleElementBox(sharedTeam.getByTestId('organization-person-card-user-extra-member'), 'extra member');
		const memberList = sharedTeam.getByTestId('organization-members-shared');

		await expect(memberList).toBeVisible();
		await expect(sharedTeam).toContainText('공유팀');
		expect(newMemberBox.x).toBeGreaterThan(sharedLeaderBox.x + sharedLeaderBox.width - 1);
		expect(Math.abs(newMemberBox.y - sharedLeaderBox.y)).toBeLessThanOrEqual(2);
		expect(Math.abs(newMemberBox.width - sharedLeaderBox.width)).toBeLessThanOrEqual(2);
		expect(Math.abs(extraMemberBox.width - sharedLeaderBox.width)).toBeLessThanOrEqual(2);
		expect(Math.abs(extraMemberBox.height - sharedLeaderBox.height)).toBeLessThanOrEqual(2);
	});
});

async function visibleElementBox(locator: Locator, label: string): Promise<ElementBox> {
	await expect(locator).toBeVisible();
	const elementBox = await locator.boundingBox();
	if (!elementBox) throw new Error(`Bounding box unavailable for ${label}`);
	return elementBox;
}

async function mockVisualOrganizationDirectory(page: Page): Promise<void> {
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'kim-first@example.com' } });
	});
	await page.route('**/admin/api/session', async (route) => {
		await route.fulfill({ status: 403, body: 'admin access required' });
	});
	await page.route('**/organization/api/people', async (route) => {
		await route.fulfill({ json: visualUsersResponse });
	});
}
