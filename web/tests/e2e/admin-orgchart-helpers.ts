import { expect, type Locator, type Page } from '@playwright/test';
import type { OrgProfileUpdate, UsersResponse } from './admin-orgchart-fixtures';

type OrgchartMockHandlers = {
	deviceManaged?: boolean;
	getUsersResponse: () => UsersResponse;
	saveProfiles?: (profiles: OrgProfileUpdate[]) => Promise<UsersResponse>;
	saveGroups?: (groups: { id: string; name: string }[]) => Promise<UsersResponse>;
};

export async function mockAdminOrgchart(page: Page, handlers: OrgchartMockHandlers): Promise<void> {
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

export async function openOrgchartEditor(page: Page): Promise<void> {
	await page.goto('/admin/?fleet_id=demo&section=orgchart');
	await expect(page.getByRole('button', { name: '조직도' })).toBeVisible();
	await page.getByRole('button', { name: '조직도' }).click();
	await page.getByLabel('편집').click();
	await expect(page.getByTestId('orgchart-profile-user-grace')).toBeVisible();
}

export async function enableOrgchartEditMode(page: Page): Promise<void> {
	await expect(page.getByRole('button', { name: '조직도' })).toBeVisible();
	await page.getByRole('button', { name: '조직도' }).click();
	await page.getByLabel('편집').click();
}

export async function openCardEditor(card: Locator): Promise<void> {
	await card.getByRole('button', { name: '편집' }).click();
	await expect(card.getByLabel('직책', { exact: true })).toBeVisible();
	await expect(card.getByRole('button', { name: '취소' })).toBeVisible();
	await expect(card.getByRole('button', { name: '저장' })).toBeVisible();
}

export async function selectCardOption(page: Page, card: Locator, label: string, name: string): Promise<void> {
	await card.getByLabel(label).click();
	await page.getByRole('option', { name }).click();
}

export function cloneUsersResponse(response: UsersResponse): UsersResponse {
	return JSON.parse(JSON.stringify(response)) as UsersResponse;
}

export async function expectCardBefore(page: Page, firstTestID: string, secondTestID: string): Promise<void> {
	const cardOrder = await page.locator('[data-testid^="orgchart-profile-"]').evaluateAll((elements) => elements.map((element) => element.getAttribute('data-testid')));
	expect(cardOrder.indexOf(firstTestID)).toBeLessThan(cardOrder.indexOf(secondTestID));
}

export function applySavedProfiles(response: UsersResponse, profiles: OrgProfileUpdate[]): UsersResponse {
	const profilesByUserID = new Map(profiles.map((profile) => [profile.userID, profile]));
	return {
		...response,
		records: response.records.map((record) => ({ ...record, ...profilesByUserID.get(record.userID) }))
	};
}
