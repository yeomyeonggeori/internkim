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
	test('keeps shared team roots over their direct report groups', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 1000 });
		await mockVisualOrgchartDirectory(page);

		await page.goto('/orgchart/');

		const kimRootBox = await visibleElementBox(page.getByTestId('orgchart-person-node-user-kim-first'), 'kim root');
		const leeRootBox = await visibleElementBox(page.getByTestId('orgchart-person-node-user-lee-second'), 'lee root');
		const pptxRootBox = await visibleElementBox(page.getByTestId('orgchart-person-node-user-pptx'), 'pptx root');
		const sharedTeamBox = await visibleElementBox(page.getByTestId('orgchart-team-column-shared'), 'shared team');
		const sharedTeamTitleBox = await visibleElementBox(page.getByTestId('orgchart-team-column-shared').getByRole('heading', { name: '공유팀' }), 'shared team title');
		const parkStaffBox = await visibleElementBox(page.getByTestId('orgchart-tree-node-user-park-staff'), 'park staff');
		const newStaffBox = await visibleElementBox(page.getByTestId('orgchart-tree-node-user-new-staff'), 'new staff');
		const extraStaffBox = await visibleElementBox(page.getByTestId('orgchart-tree-node-user-extra-staff'), 'extra staff');

		expect(Math.abs(horizontalCenter(kimRootBox) - horizontalCenter(parkStaffBox))).toBeLessThanOrEqual(4);
		expect(Math.abs(horizontalCenter(leeRootBox) - horizontalGroupCenter([newStaffBox, extraStaffBox]))).toBeLessThanOrEqual(4);
		expect(Math.abs(horizontalCenter(sharedTeamTitleBox) - horizontalCenter(sharedTeamBox))).toBeLessThanOrEqual(4);
		expect(Math.abs(kimRootBox.y - leeRootBox.y)).toBeLessThanOrEqual(2);
		expect(Math.abs(leeRootBox.y - pptxRootBox.y)).toBeLessThanOrEqual(2);
		expect(pptxRootBox.x).toBeGreaterThan(sharedTeamBox.x + sharedTeamBox.width);
		expect(sharedTeamBox.y).toBeGreaterThan(kimRootBox.y + kimRootBox.height);
	});
});

test.describe('employee orgchart visual layout screenshot', () => {
	test.skip(process.platform !== 'darwin', 'The checked-in orgchart visual baseline is captured for Darwin Chromium.');

	test('matches the shared-team root block desktop baseline', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 1000 });
		await mockVisualOrgchartDirectory(page);

		await page.goto('/orgchart/');

		const board = page.getByTestId('orgchart-board');
		await expect(board).toBeVisible();
		await expect(page.getByTestId('orgchart-person-node-user-kim-first')).toBeVisible();
		await expect(page.getByTestId('orgchart-person-node-user-lee-second')).toBeVisible();
		await expect(page.getByTestId('orgchart-person-node-user-pptx')).toBeVisible();
		await expect(page.getByTestId('orgchart-team-column-shared')).toBeVisible();
		await expect(page.getByTestId('orgchart-tree-node-user-park-staff')).toBeVisible();
		await expect(page.getByTestId('orgchart-tree-node-user-new-staff')).toBeVisible();
		await expect(board).toHaveScreenshot('orgchart-shared-team-root-block-desktop.png', {
			animations: 'disabled',
			caret: 'hide',
			maxDiffPixelRatio: 0.01
		});
	});
});

async function visibleElementBox(locator: Locator, label: string): Promise<ElementBox> {
	await expect(locator).toBeVisible();
	const elementBox = await locator.boundingBox();
	if (!elementBox) throw new Error(`Bounding box unavailable for ${label}`);
	return elementBox;
}

function horizontalCenter(elementBox: ElementBox): number {
	return elementBox.x + elementBox.width / 2;
}

function horizontalGroupCenter(elementBoxes: ElementBox[]): number {
	const left = Math.min(...elementBoxes.map((elementBox) => elementBox.x));
	const right = Math.max(...elementBoxes.map((elementBox) => elementBox.x + elementBox.width));
	return (left + right) / 2;
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
