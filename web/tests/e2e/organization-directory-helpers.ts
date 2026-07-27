import { expect, type Locator, type Page } from '@playwright/test';
import type { UsersResponse } from './admin-organization-fixtures';

type MockOrganizationDirectoryOptions = {
	canManage?: boolean;
	directoryResponse?: UsersResponse;
	locale?: 'ko' | 'en';
};

export const organizationDirectoryUsersResponse: UsersResponse = {
	availableGroups: [
		{ id: 'leadership', name: '경영' },
		{ id: 'product', name: '제품팀' },
		{ id: 'design', name: '디자인팀', parentID: 'product' },
		{ id: 'field', name: '현장지원팀' }
	],
	records: [
		{
			userID: 'user-ceo',
			handle: 'ceo',
			name: '김도형',
			email: 'ceo@example.com',
			hireDate: '2026-01-03',
			role: 'member',
			jobTitle: '  FoUn-Der  ',
			primaryGroupID: 'leadership',
			groupIDs: ['leadership', 'product']
		},
		{
			userID: 'user-junho',
			handle: 'junho',
			name: '이정훈',
			email: 'junho@example.com',
			hireDate: '2026-02-10',
			role: 'member',
			jobTitle: '제품팀 리드',
			primaryGroupID: 'product',
			groupIDs: ['product'],
			supervisorID: 'user-ceo'
		},
		{
			userID: 'user-dabin',
			handle: 'dabin',
			name: '김다빈',
			email: 'dabin@example.com',
			image: 'data:image/gif;base64,R0lGODlhAQABAAAAACw=',
			hireDate: '2026-03-11',
			role: 'member',
			jobTitle: '프론트엔드 개발자',
			primaryGroupID: 'product',
			groupIDs: ['product'],
			supervisorID: 'user-junho'
		},
		{
			userID: 'user-minjae',
			handle: 'minjae',
			name: '강민재',
			email: 'minjae@example.com',
			hireDate: '2026-03-13',
			role: 'member',
			jobTitle: '백엔드 개발자',
			primaryGroupID: 'product',
			groupIDs: ['product'],
			supervisorID: 'user-junho'
		},
		{
			userID: 'user-taehyun',
			handle: 'taehyun',
			name: '신태현',
			email: 'taehyun@example.com',
			hireDate: '2026-03-20',
			role: 'member',
			jobTitle: 'QA 엔지니어',
			primaryGroupID: 'product',
			groupIDs: ['product'],
			supervisorID: 'user-dabin'
		},
		{
			userID: 'user-jieun',
			handle: 'jieun',
			name: '박지은',
			email: 'jieun@example.com',
			hireDate: '2026-02-12',
			role: 'member',
			jobTitle: '디자인 리드',
			primaryGroupID: 'design',
			groupIDs: ['design'],
			supervisorID: 'user-ceo'
		},
		{
			userID: 'user-nam',
			handle: 'nam',
			name: '남지훈',
			email: 'nam@example.com',
			hireDate: '2026-04-01',
			role: 'member',
			jobTitle: '사업 개발',
			groupIDs: [],
			supervisorID: 'user-ceo'
		}
	]
};

export async function mockOrganizationDirectory(page: Page, options: MockOrganizationDirectoryOptions = {}): Promise<void> {
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: options.locale ?? 'ko' } });
	});
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'dabin@example.com' } });
	});
	await page.route('**/admin/api/session', async (route) => {
		if (options.canManage) {
			await route.fulfill({ json: { email: 'admin@example.com', role: 'admin', deviceManaged: true } });
			return;
		}
		await route.fulfill({ status: 403, body: 'admin access required' });
	});
	await page.route('**/organization/api/people', async (route) => {
		await route.fulfill({ json: options.directoryResponse ?? organizationDirectoryUsersResponse });
	});
}

export async function expectPersonDetailPanelContent(detailPanel: Locator): Promise<void> {
	await expect(detailPanel).toBeVisible();
	await expect(detailPanel).toContainText('직원 상세');
	await expect(detailPanel).toContainText('김다빈');
	await expect(detailPanel).toContainText('프론트엔드 개발자');
	await expect(detailPanel).toContainText('dabin@example.com');
	await expect(detailPanel).toContainText('제품팀');
	await expect(detailPanel).toContainText('직속 상관');
	await expect(detailPanel).toContainText('이정훈 · 제품팀 리드');
	await expect(detailPanel).toContainText('2026-03-11');
	await expect(detailPanel.locator('[aria-hidden="true"] svg').first()).toBeVisible();
}

export async function expectDetailPanelInRightColumn(page: Page): Promise<void> {
	const listBox = await page.getByTestId('organization-list-scroll').boundingBox();
	const detailColumnBox = await page.getByTestId('organization-detail-column').boundingBox();
	const detailPanelBox = await page.getByTestId('organization-person-detail-panel').boundingBox();
	if (!listBox || !detailColumnBox || !detailPanelBox) throw new Error('Organization detail layout box unavailable');
	expect(detailPanelBox.y).toBeGreaterThanOrEqual(detailColumnBox.y);
	expect(detailPanelBox.y + detailPanelBox.height).toBeLessThanOrEqual(detailColumnBox.y + detailColumnBox.height + 1);
	expect(detailPanelBox.x).toBeGreaterThan(listBox.x + listBox.width);
}

export async function expectDetailPanelStableWhileListScrolls(page: Page): Promise<void> {
	const detailPanel = page.getByTestId('organization-person-detail-panel');
	const beforeBox = await detailPanel.boundingBox();
	if (!beforeBox) throw new Error('Organization detail panel box unavailable before scroll');

	const scrollState = await page.getByTestId('organization-list-scroll').evaluate((element) => {
		element.scrollTop = element.scrollHeight;
		return {
			clientHeight: element.clientHeight,
			scrollHeight: element.scrollHeight,
			scrollTop: element.scrollTop
		};
	});
	expect(scrollState.scrollHeight).toBeGreaterThan(scrollState.clientHeight);
	expect(scrollState.scrollTop).toBeGreaterThan(0);

	const afterBox = await detailPanel.boundingBox();
	if (!afterBox) throw new Error('Organization detail panel box unavailable after scroll');
	expect(Math.abs(afterBox.y - beforeBox.y)).toBeLessThanOrEqual(1);
}

export async function expectMobileDetailSheetLayout(page: Page): Promise<void> {
	const layout = await page.getByTestId('organization-mobile-detail-sheet').evaluate((element) => {
		const rect = element.getBoundingClientRect();
		const style = getComputedStyle(element);
		return {
			bottom: window.innerHeight - rect.bottom,
			left: rect.left,
			right: rect.right,
			height: rect.height,
			width: rect.width,
			position: style.position,
			overflowY: style.overflowY,
			viewportHeight: window.innerHeight,
			viewportWidth: window.innerWidth
		};
	});
	const detailPanelLayout = await page.getByTestId('organization-mobile-detail-sheet').getByTestId('organization-person-detail-panel').evaluate((element) => {
		const style = getComputedStyle(element);
		return {
			overflowY: style.overflowY
		};
	});

	expect(layout.position).toBe('fixed');
	expect(layout.overflowY).toBe('hidden');
	expect(detailPanelLayout.overflowY).toBe('auto');
	expect(Math.abs(layout.bottom)).toBeLessThanOrEqual(2);
	expect(layout.left).toBeGreaterThanOrEqual(0);
	expect(layout.right).toBeLessThanOrEqual(layout.viewportWidth);
	expect(layout.width).toBeLessThanOrEqual(layout.viewportWidth);
	expect(layout.height).toBeLessThanOrEqual(layout.viewportHeight * 0.85 + 2);
}

export async function expectMobileHeaderControlsInTitleRow(page: Page): Promise<void> {
	const titleBox = await page.getByRole('heading', { name: '조직도' }).boundingBox();
	const organizationsBox = await page.getByRole('button', { name: '조직 목록' }).boundingBox();
	const addOrganizationBox = await page.getByRole('button', { name: '조직 추가' }).boundingBox();
	const searchBox = await page.getByLabel('검색').boundingBox();
	if (!titleBox || !organizationsBox || !addOrganizationBox || !searchBox) throw new Error('Mobile organization header layout box unavailable');
	expect(Math.abs(organizationsBox.y - titleBox.y)).toBeLessThanOrEqual(12);
	expect(Math.abs(addOrganizationBox.y - titleBox.y)).toBeLessThanOrEqual(12);
	expect(organizationsBox.x).toBeGreaterThan(titleBox.x + titleBox.width);
	expect(addOrganizationBox.x).toBeGreaterThan(organizationsBox.x + organizationsBox.width);
	expect(searchBox.y).toBeGreaterThan(titleBox.y + titleBox.height);
}

export async function openFilterPopover(page: Page): Promise<void> {
	const filterButton = page.getByRole('button', { name: '필터' });
	await filterButton.click();
	const filterPopover = page.getByTestId('organization-filter-popover');
	await expect(filterPopover).toBeVisible();
	await expect(filterPopover.getByRole('button', { name: '조직', exact: true })).toBeVisible();
	const buttonBox = await filterButton.boundingBox();
	const popoverBox = await filterPopover.boundingBox();
	if (!buttonBox || !popoverBox) throw new Error('필터 popover 위치를 확인할 수 없습니다.');
	expect(popoverBox.y).toBeGreaterThanOrEqual(buttonBox.y + buttonBox.height - 1);
}

export async function expectMobileDetailPanelScrollsToBottom(detailPanel: Locator): Promise<void> {
	const scrollState = await detailPanel.evaluate((element) => {
		element.scrollTop = element.scrollHeight;
		const style = getComputedStyle(element);
		return {
			clientHeight: element.clientHeight,
			scrollHeight: element.scrollHeight,
			scrollTop: element.scrollTop,
			overflowY: style.overflowY
		};
	});
	expect(scrollState.overflowY).toBe('auto');
	expect(scrollState.scrollHeight).toBeGreaterThan(scrollState.clientHeight);
	expect(scrollState.scrollTop).toBeGreaterThan(0);

	const saveButton = detailPanel.getByRole('button', { name: '저장' });
	await expect(saveButton).toBeVisible();
	const panelBox = await detailPanel.boundingBox();
	const saveButtonBox = await saveButton.boundingBox();
	if (!panelBox || !saveButtonBox) throw new Error('Mobile edit sheet button box unavailable');
	expect(saveButtonBox.y + saveButtonBox.height).toBeLessThanOrEqual(panelBox.y + panelBox.height + 1);
}
