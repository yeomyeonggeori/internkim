import { expect, type Locator, type Page } from '@playwright/test';
import { signInToTheCentralPlane } from '../support/central-plane-sign-in';
import { centralPlaneAdminClient, exampleCompanyID, exampleCompanySlug } from '../support/central-test-utils';

export async function signInToTheCRM(page: Page, email?: string): Promise<void> {
	await signInToTheCentralPlane(page, `/${exampleCompanySlug}/crm`, email);
	await page.locator('[data-crm-ready="true"]').waitFor({ state: 'visible', timeout: 20000 });
}

export async function openCreateForm(page: Page, tab: string, createButton: string): Promise<void> {
	await page.getByRole('tab', { name: tab, exact: true }).click();
	// Scoped to the toolbar: sortable column headers are buttons carrying the same labels.
	const toolbar = page.getByRole('tabpanel', { name: tab }).locator('div').first();
	await toolbar.getByRole('button', { name: createButton, exact: true }).click();
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

type LongTextRow = { table: 'organization' | 'contact' | 'opportunity'; id: string; original: Record<string, string | null> };

export async function lengthenCRMText(organizationID: string, suffix: string): Promise<() => Promise<void>> {
	const admin = centralPlaneAdminClient();
	const organization = await admin.from('organization').select('id, name').eq('id', organizationID).single<{ id: string; name: string }>();
	const contacts = await admin.from('contact').select('id, name, email, title').eq('organization_id', organizationID).returns<{ id: string; name: string; email: string | null; title: string | null }[]>();
	const opportunities = await admin.from('opportunity').select('id, name').eq('organization_id', organizationID).returns<{ id: string; name: string }[]>();
	if (organization.error) throw new Error(`Failed to read the organization to lengthen: ${organization.error.message}`);
	if (contacts.error) throw new Error(`Failed to read the contacts to lengthen: ${contacts.error.message}`);
	if (opportunities.error) throw new Error(`Failed to read the opportunities to lengthen: ${opportunities.error.message}`);

	const rows: LongTextRow[] = [
		{ table: 'organization', id: organization.data.id, original: { name: organization.data.name } },
		...contacts.data.map((contact): LongTextRow => ({ table: 'contact', id: contact.id, original: { name: contact.name, email: contact.email, title: contact.title } })),
		...opportunities.data.map((opportunity): LongTextRow => ({ table: 'opportunity', id: opportunity.id, original: { name: opportunity.name } }))
	];
	await writeCRMRows(rows.map((row) => ({ ...row, original: lengthened(row.original, suffix) })));
	return () => writeCRMRows(rows);
}

function lengthened(values: Record<string, string | null>, suffix: string): Record<string, string | null> {
	return Object.fromEntries(
		Object.entries(values).map(([field, value]) => [field, field === 'email' ? `a.very.long.address.for.the.column.fit.${value ?? 'contact@example.com'}` : `${value ?? ''}${suffix}`])
	);
}

async function writeCRMRows(rows: LongTextRow[]): Promise<void> {
	const admin = centralPlaneAdminClient();
	for (const row of rows) {
		const updated = await admin.from(row.table).update(row.original).eq('id', row.id);
		if (updated.error) throw new Error(`Failed to write ${row.table} ${row.id}: ${updated.error.message}`);
	}
}
