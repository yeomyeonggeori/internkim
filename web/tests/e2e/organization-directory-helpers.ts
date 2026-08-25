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
			memberID: 'user-sample-lee',
			handle: 'sample-lee',
			name: '이샘플',
			email: 'sample-lee@example.com',
			hireDate: '2026-01-03',
			role: 'member',
			jobTitle: '  FoUn-Der  ',
			groupID: 'leadership'
		},
		{
			memberID: 'user-example-park',
			handle: 'example-park',
			name: '박예시',
			email: 'example-park@example.com',
			hireDate: '2026-02-10',
			role: 'member',
			jobTitle: '제품팀 리드',
			groupID: 'product',
			supervisorID: 'user-sample-lee'
		},
		{
			memberID: 'user-specimen-choi',
			handle: 'specimen-choi',
			name: '최견본',
			email: 'specimen-choi@example.com',
			image: 'data:image/gif;base64,R0lGODlhAQABAAAAACw=',
			hireDate: '2026-03-11',
			role: 'member',
			jobTitle: '프론트엔드 개발자',
			groupID: 'product',
			supervisorID: 'user-example-park'
		},
		{
			memberID: 'user-sample-kim',
			handle: 'sample-kim',
			name: '김샘플',
			email: 'sample-kim@example.com',
			hireDate: '2026-03-13',
			role: 'member',
			jobTitle: '백엔드 개발자',
			groupID: 'product',
			supervisorID: 'user-example-park'
		},
		{
			memberID: 'user-example-jung',
			handle: 'example-jung',
			name: '정예시',
			email: 'example-jung@example.com',
			hireDate: '2026-03-20',
			role: 'member',
			jobTitle: 'QA 엔지니어',
			groupID: 'product',
			supervisorID: 'user-specimen-choi'
		},
		{
			memberID: 'user-specimen-han',
			handle: 'specimen-han',
			name: '한견본',
			email: 'specimen-han@example.com',
			hireDate: '2026-02-12',
			role: 'member',
			jobTitle: '디자인 리드',
			groupID: 'design',
			supervisorID: 'user-sample-lee'
		},
		{
			memberID: 'user-sample-oh',
			handle: 'sample-oh',
			name: '오샘플',
			email: 'sample-oh@example.com',
			hireDate: '2026-04-01',
			role: 'member',
			jobTitle: '사업 개발',
			supervisorID: 'user-sample-lee'
		}
	]
};

export async function mockOrganizationDirectory(page: Page, options: MockOrganizationDirectoryOptions = {}): Promise<void> {
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: options.locale ?? 'ko' } });
	});
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'specimen-choi@example.com' } });
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

export function detailSheet(page: Page): Locator {
	return page.getByTestId('organization-detail-sheet');
}

export function detailPanel(page: Page): Locator {
	return page.getByTestId('organization-person-detail-panel');
}

export function closeDetailSheet(page: Page): Promise<void> {
	return detailSheet(page).getByRole('button', { name: '상세 닫기' }).click();
}

export async function chooseInOrganizationMenu(page: Page, itemName: string): Promise<void> {
	await page.getByRole('button', { name: '조직 작업' }).click();
	await page.getByRole('menuitem', { name: itemName, exact: true }).click();
}

export async function filterByPerson(page: Page, searchTerm: string, personName: string): Promise<void> {
	await page.getByRole('combobox', { name: '직원 선택' }).click();
	await page.getByPlaceholder('이름 또는 직책 검색').fill(searchTerm);
	await page.getByRole('option', { name: new RegExp(personName) }).first().click();
}

export async function filterByOrganization(page: Page, organizationName: string): Promise<void> {
	await page.getByRole('combobox', { name: '조직 선택' }).click();
	await page.getByRole('option', { name: organizationName, exact: true }).click();
}

export async function expectPersonDetailPanelContent(panel: Locator): Promise<void> {
	await expect(panel).toBeVisible();
	await expect(panel).toContainText('최견본');
	await expect(panel).toContainText('프론트엔드 개발자');
	await expect(panel).toContainText('specimen-choi@example.com');
	await expect(panel).toContainText('제품팀');
	await expect(panel).toContainText('직속 상관');
	await expect(panel).toContainText('박예시 · 제품팀 리드');
	await expect(panel).toContainText('2026-03-11');
	await expect(panel.locator('[aria-hidden="true"] svg').first()).toBeVisible();
}

export async function expectDetailSheetBesideTheList(page: Page): Promise<void> {
	const listBox = await page.getByTestId('organization-list-scroll').boundingBox();
	const sheetBox = await detailSheet(page).boundingBox();
	const panelBox = await detailPanel(page).boundingBox();
	if (!listBox || !sheetBox || !panelBox) throw new Error('Organization detail layout box unavailable');
	expect(sheetBox.x).toBeGreaterThan(listBox.x);
	expect(panelBox.x).toBeGreaterThanOrEqual(sheetBox.x - 3);
	expect(panelBox.y).toBeGreaterThanOrEqual(sheetBox.y - 3);
	expect(panelBox.y + panelBox.height).toBeLessThanOrEqual(sheetBox.y + sheetBox.height + 3);
}

export async function expectDetailSheetStableWhileListScrolls(page: Page): Promise<void> {
	const panel = detailPanel(page);
	const beforeBox = await panel.boundingBox();
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

	const afterBox = await panel.boundingBox();
	if (!afterBox) throw new Error('Organization detail panel box unavailable after scroll');
	expect(Math.abs(afterBox.y - beforeBox.y)).toBeLessThanOrEqual(1);
}

type DetailSheetLayout = {
	top: number;
	bottom: number;
	left: number;
	right: number;
	height: number;
	width: number;
	position: string;
	overflowY: string;
	viewportHeight: number;
	viewportWidth: number;
};

function measureDetailSheet(page: Page): Promise<DetailSheetLayout> {
	return detailSheet(page).evaluate((element) => {
		const rect = element.getBoundingClientRect();
		const style = getComputedStyle(element);
		return {
			top: rect.top,
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
}

async function waitForDetailSheetSlideInToSettle(page: Page): Promise<void> {
	await expect
		.poll(async () => {
			const settling = await measureDetailSheet(page);
			return settling.right - settling.viewportWidth;
		})
		.toBeLessThanOrEqual(1);
}

export async function expectDetailSheetFitsTheViewport(page: Page): Promise<void> {
	await waitForDetailSheetSlideInToSettle(page);

	const layout = await measureDetailSheet(page);
	expect(layout.position).toBe('fixed');
	expect(layout.overflowY).toBe('hidden');
	expect(Math.abs(layout.top)).toBeLessThanOrEqual(2);
	expect(Math.abs(layout.bottom)).toBeLessThanOrEqual(2);
	expect(layout.left).toBeGreaterThanOrEqual(0);
	expect(layout.width).toBeLessThanOrEqual(layout.viewportWidth);
	expect(layout.height).toBeLessThanOrEqual(layout.viewportHeight + 1);
}

export async function expectMobileHeaderControlsShareOneRow(page: Page): Promise<void> {
	const organizationsBox = await page.getByRole('button', { name: '목차' }).boundingBox();
	const organizationFilterBox = await page.getByRole('combobox', { name: '조직 선택' }).boundingBox();
	const personFilterBox = await page.getByRole('combobox', { name: '직원 선택' }).boundingBox();
	if (!organizationsBox || !organizationFilterBox || !personFilterBox) throw new Error('Mobile organization header layout box unavailable');
	expect(Math.abs(organizationFilterBox.y - organizationsBox.y)).toBeLessThanOrEqual(12);
	expect(Math.abs(personFilterBox.y - organizationsBox.y)).toBeLessThanOrEqual(12);
	expect(organizationFilterBox.x).toBeGreaterThan(organizationsBox.x);
	expect(personFilterBox.x).toBeGreaterThan(organizationFilterBox.x);
}

export async function selectOrganizationInTree(page: Page, groupID: string): Promise<void> {
	await page.getByTestId(`organization-row-${groupID}`).getByRole('button').first().click();
}

export async function expectDetailPanelScrollsToItsFooter(panel: Locator): Promise<void> {
	const scrollState = await panel.locator('div.overflow-y-auto').first().evaluate((element) => {
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

	const saveButton = panel.getByRole('button', { name: '저장', exact: true });
	await expect(saveButton).toBeVisible();
	const panelBox = await panel.boundingBox();
	const saveButtonBox = await saveButton.boundingBox();
	if (!panelBox || !saveButtonBox) throw new Error('Detail panel footer button box unavailable');
	expect(saveButtonBox.y + saveButtonBox.height).toBeLessThanOrEqual(panelBox.y + panelBox.height + 1);
}
