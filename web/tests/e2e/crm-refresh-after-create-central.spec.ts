import { expect, test } from '@playwright/test';
import { openCreateForm, recordSheet, removeOrganizationsNamed, signInToTheCRM } from './crm-central-test-utils';
import { centralPlaneAdminClient, exampleCompanyID } from './central-test-utils';

test.use({ locale: 'ko-KR' });

const organizationName = 'E2E 새로고침 실패 관계처';

test.afterEach(async () => {
	await removeOrganizationsNamed([organizationName]);
});

test('reports a refresh failure without losing a successful create', async ({ page }) => {
	await signInToTheCRM(page);

	let failNextOrganizationList = false;
	await page.route('**/tools/crm_organization_list/invoke', async (route) => {
		if (!failNextOrganizationList) {
			await route.continue();
			return;
		}
		failNextOrganizationList = false;
		await route.fulfill({ status: 500, json: { error: 'refresh failed', errorCode: 'request_failed' } });
	});

	await openCreateForm(page, '관계처', '관계처');
	const sheet = recordSheet(page);
	await sheet.getByLabel('이름 또는 제목').fill(organizationName);
	failNextOrganizationList = true;
	await sheet.getByRole('button', { name: '추가', exact: true }).click();

	await expect(sheet).not.toBeVisible();
	await expect(page.getByRole('status')).toContainText('CRM 서비스에 저장했습니다.');
	await expect(page.getByRole('alert')).toContainText('저장했지만 목록을 새로 불러오지 못했습니다.');

	const admin = centralPlaneAdminClient();
	const created = await admin
		.from('organization')
		.select('id')
		.eq('company_id', exampleCompanyID)
		.eq('name', organizationName);
	expect(created.error).toBeNull();
	expect(created.data).toHaveLength(1);
});
