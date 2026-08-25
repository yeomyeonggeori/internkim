import { expect, type Locator, type Page } from '@playwright/test';
import type { OrgProfileUpdate, UsersResponse } from './admin-organization-fixtures';

type OrganizationMockHandlers = {
	deviceManaged?: boolean;
	getUsersResponse: () => UsersResponse;
	saveProfiles?: (profiles: OrgProfileUpdate[]) => Promise<UsersResponse>;
	saveGroups?: (groups: { id: string; name: string }[]) => Promise<UsersResponse>;
};

export async function mockAdminOrganization(page: Page, handlers: OrganizationMockHandlers): Promise<void> {
	await page.route('**/admin/api/session', async (route) => {
		await route.fulfill({ json: { email: 'admin@example.com', role: 'admin', deviceManaged: handlers.deviceManaged ?? true } });
	});
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'admin@example.com' } });
	});
	await page.route('**/admin/api/users?includePolicy=true', async (route) => {
		await route.fulfill({ json: handlers.getUsersResponse() });
	});
	await page.route('**/organization/api/people', async (route) => {
		await route.fulfill({ json: handlers.getUsersResponse() });
	});
	await page.route('**/admin/api/users/org-profiles?includePolicy=true', async (route) => {
		const body = (await route.request().postDataJSON()) as { profiles: OrgProfileUpdate[] };
		try {
			const response = handlers.saveProfiles ? await handlers.saveProfiles(body.profiles) : handlers.getUsersResponse();
			await route.fulfill({ json: response });
		} catch (error) {
			await route.fulfill({ status: 500, body: error instanceof Error ? error.message : 'Save failed' });
		}
	});
	await page.route('**/admin/api/org-groups?includePolicy=true', async (route) => {
		const body = (await route.request().postDataJSON()) as { groups: { id: string; name: string }[] };
		try {
			const response = handlers.saveGroups ? await handlers.saveGroups(body.groups) : handlers.getUsersResponse();
			await route.fulfill({ json: response });
		} catch (error) {
			await route.fulfill({ status: 500, body: error instanceof Error ? error.message : 'Save failed' });
		}
	});
}

export async function openOrganizationEditor(page: Page): Promise<void> {
	await page.goto('/organization/?edit=1');
	await expect(page.getByRole('button', { name: '조직 작업' })).toBeVisible();
	await expect(page.getByTestId('organization-profile-user-grace')).toHaveCount(0);
}

export async function enableOrganizationEditMode(page: Page): Promise<void> {
	await expect(page.getByRole('button', { name: '조직 작업' })).toBeVisible();
}

async function closeAnyOpenDetailSheet(page: Page): Promise<void> {
	const sheet = page.getByTestId('organization-detail-sheet');
	if (!(await sheet.isVisible())) return;
	await sheet.getByRole('button', { name: '상세 닫기' }).click();
	await expect(sheet).toHaveCount(0);
}

export async function openCardEditor(page: Page, memberID: string): Promise<Locator> {
	await closeAnyOpenDetailSheet(page);
	await page.getByTestId(`organization-person-node-${memberID}`).click();
	const panel = page.getByTestId('organization-person-detail-panel');
	await panel.getByRole('button', { name: '수정하기' }).click();
	const editor = page.getByTestId(`organization-profile-${memberID}`);
	await expect(editor.getByLabel('직책', { exact: true })).toBeVisible();
	await expect(cardEditorButton(page, '취소')).toBeVisible();
	await expect(cardEditorButton(page, '저장')).toBeVisible();
	return editor;
}

export function cardEditorButton(page: Page, name: '저장' | '취소'): Locator {
	return page.getByTestId('organization-person-detail-panel').getByRole('button', { name, exact: true });
}

export async function openOrganizationForm(page: Page): Promise<void> {
	await page.getByRole('button', { name: '조직 작업' }).click();
	await page.getByRole('menuitem', { name: '조직 추가', exact: true }).click();
	await expect(page.getByTestId('organization-add-organization-popover')).toBeVisible();
	await expect(page.getByLabel('새 조직')).toBeVisible();
	await expect(page.getByRole('button', { name: '추가', exact: true })).toBeVisible();
}

export async function selectCardOption(page: Page, card: Locator, label: string, name: string): Promise<void> {
	await card.getByLabel(label).click();
	await page.getByRole('option', { name }).click();
}

export function cloneUsersResponse(response: UsersResponse): UsersResponse {
	return JSON.parse(JSON.stringify(response)) as UsersResponse;
}

export async function expectCardBefore(page: Page, firstTestID: string, secondTestID: string): Promise<void> {
	const cardOrder = await page.locator('[data-testid^="organization-person-node-"]').evaluateAll((elements) => elements.map((element) => element.getAttribute('data-testid')));
	expect(cardOrder.indexOf(profileTestIDToPersonNodeTestID(firstTestID))).toBeLessThan(cardOrder.indexOf(profileTestIDToPersonNodeTestID(secondTestID)));
}

function profileTestIDToPersonNodeTestID(testID: string): string {
	return testID.replace('organization-profile-', 'organization-person-node-');
}

export function applySavedProfiles(response: UsersResponse, profiles: OrgProfileUpdate[]): UsersResponse {
	const profilesByUserID = new Map(profiles.map((profile) => [profile.memberID, profile]));
	return {
		...response,
		records: response.records.map((record) => ({ ...record, ...profilesByUserID.get(record.memberID) }))
	};
}
