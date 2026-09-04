import { expect, type Locator, type Page } from '@playwright/test';
import { signInToTheCentralPlane } from './central-plane-sign-in';
import { centralPlaneAdminClient, exampleCompanyID } from './central-test-utils';

export async function signInToTheCRM(page: Page, email?: string): Promise<void> {
	await signInToTheCentralPlane(page, '/example-co/crm', email);
	await page.getByRole('button', { name: '빠른 추가' }).waitFor({ state: 'visible', timeout: 20000 });
}

export async function openQuickAdd(page: Page, kind: string): Promise<void> {
	await page.getByRole('button', { name: '빠른 추가' }).click();
	await page.getByRole('menuitem', { name: kind, exact: true }).click();
}

export function recordSheet(page: Page): Locator {
	return page.getByRole('dialog', { name: 'CRM 기록 추가' });
}

export async function organizationIDNamed(name: string): Promise<string> {
	const admin = centralPlaneAdminClient();
	const row = await admin
		.from('organization')
		.select('id')
		.eq('company_id', exampleCompanyID)
		.eq('name', name)
		.single<{ id: string }>();
	if (row.error) throw new Error(`Failed to find the organization named ${name}: ${row.error.message}`);
	return row.data.id;
}

export async function seedCRMActivity(organizationID: string, title: string): Promise<string> {
	const admin = centralPlaneAdminClient();
	const inserted = await admin
		.from('task')
		.insert({
			company_id: exampleCompanyID,
			organization_id: organizationID,
			title,
			status: 'planned',
			note: '컬럼 비율 검증용 활동 비고입니다.'
		})
		.select('id')
		.single<{ id: string }>();
	if (inserted.error) throw new Error(`Failed to seed a CRM activity: ${inserted.error.message}`);
	return inserted.data.id;
}

export async function removeCRMActivities(taskIDs: string[]): Promise<void> {
	if (taskIDs.length === 0) return;
	const admin = centralPlaneAdminClient();
	const deleted = await admin.from('task').delete().in('id', taskIDs);
	if (deleted.error) throw new Error(`Failed to clean up CRM activities: ${deleted.error.message}`);
}

export async function removeOrganizationsNamed(names: string[]): Promise<void> {
	if (names.length === 0) return;
	const admin = centralPlaneAdminClient();
	const deleted = await admin
		.from('organization')
		.delete()
		.eq('company_id', exampleCompanyID)
		.in('name', names);
	if (deleted.error) throw new Error(`Failed to clean up organizations: ${deleted.error.message}`);
}

export async function expectNoHorizontalOverflow(locator: Locator): Promise<void> {
	const overflow = await locator.evaluate((element) => element.scrollWidth - element.clientWidth);
	expect(overflow).toBeLessThanOrEqual(1);
}
