import { expect, test, type Locator, type Page, type Route } from '@playwright/test';

type Audit = {
	createdAt: string;
	createdByPersonID: string;
	updatedAt: string;
	updatedByPersonID: string;
};

type Account = Record<string, unknown> & { id: string; name: string; audit: Audit };
type Contact = Record<string, unknown> & { id: string; name: string; audit: Audit };
type Opportunity = Record<string, unknown> & { id: string; name: string; stage: string; stagePosition: number; audit: Audit };
type Activity = Record<string, unknown> & { id: string; title: string; audit: Audit };

const audit: Audit = {
	createdAt: '2026-08-03T00:00:00Z',
	createdByPersonID: 'person-crm',
	updatedAt: '2026-08-03T00:00:00Z',
	updatedByPersonID: 'person-crm'
};

const responsiveColumnRatios: Record<number, Record<string, number[]>> = {
	390: {
		'관계처': [0.65, 0.35],
		'연락처': [0.45, 0.55],
		'진행상황': [0.65, 0.35],
		'활동': [0.35, 0.65],
		'리포트': [0.45, 0.2, 0.35]
	},
	640: {
		'관계처': [0.45, 0.2, 0.35],
		'연락처': [0.3, 0.35, 0.35],
		'진행상황': [0.4, 0.25, 0.2, 0.15],
		'활동': [0.25, 0.25, 0.15, 0.35],
		'리포트': [0.35, 0.15, 0.15, 0.35]
	},
	768: {
		'관계처': [0.4, 0.15, 0.15, 0.3],
		'연락처': [0.25, 0.25, 0.15, 0.35],
		'진행상황': [0.35, 0.2, 0.15, 0.15, 0.15],
		'활동': [0.2, 0.2, 0.15, 0.15, 0.3],
		'리포트': [0.25, 0.15, 0.15, 0.25, 0.2]
	},
	1024: {
		'관계처': [0.3, 0.12, 0.12, 0.22, 0.16, 0.08],
		'연락처': [0.22, 0.22, 0.14, 0.28, 0.14],
		'진행상황': [0.27, 0.17, 0.12, 0.12, 0.14, 0.18],
		'활동': [0.16, 0.16, 0.12, 0.12, 0.24, 0.2],
		'리포트': [0.2, 0.15, 0.15, 0.3, 0.2]
	},
	1280: {
		'관계처': [0.26, 0.14, 0.1, 0.13, 0.17, 0.12, 0.08],
		'연락처': [0.16, 0.18, 0.14, 0.2, 0.14, 0.18],
		'진행상황': [0.19, 0.14, 0.09, 0.09, 0.11, 0.12, 0.18, 0.08],
		'활동': [0.13, 0.13, 0.08, 0.09, 0.12, 0.17, 0.2, 0.08],
		'리포트': [0.2, 0.15, 0.15, 0.3, 0.2]
	}
};

test.describe('CRM service UI', () => {
	let accounts: Account[];
	let contacts: Contact[];
	let opportunities: Opportunity[];
	let activities: Activity[];
	let permissionDenied: boolean;
	let failNextCRMList: boolean;

	test.beforeEach(async ({ page }) => {
		accounts = [account('account-1', '기존 관계처')];
		contacts = [];
		opportunities = [];
		activities = [];
		permissionDenied = false;
		failNextCRMList = false;
		await installSessionRoutes(page);
		await page.route('**/organization/api/people', (route) => route.fulfill({ json: {
			records: [{ userID: 'person-crm', handle: 'crm', name: 'CRM 담당자', email: 'crm@example.com', groupID: 'team-sales' }],
			availableGroups: [{ id: 'team-sales', name: '영업팀' }]
		} }));
		await page.route('**/crm/api/**', async (route) => {
			if (permissionDenied) {
				await route.fulfill({ status: 403, json: { error: { code: 'permission_denied', message: 'CRM access required' } } });
				return;
			}
			if (failNextCRMList && route.request().method() === 'GET') {
				failNextCRMList = false;
				await route.fulfill({ status: 500, json: { error: { code: 'request_failed', message: 'refresh failed' } } });
				return;
			}
			await handleCRMRoute(route, accounts, contacts, opportunities, activities);
		});
	});

	test('left-aligns relationship columns and gives names more room than counts', async ({ page }) => {
		await openCRM(page);
		const headers = page.getByRole('columnheader');
		await expect(headers).toHaveCount(7);
		for (let index = 0; index < 7; index += 1) {
			await expect(headers.nth(index)).toHaveCSS('text-align', 'left');
		}
		const nameWidth = await headers.nth(0).evaluate((element) => element.getBoundingClientRect().width);
		const countWidth = await headers.nth(6).evaluate((element) => element.getBoundingClientRect().width);
		expect(nameWidth).toBeGreaterThan(countWidth);
	});

	test('keeps the primary CRM tab list static without horizontal scrolling', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await openCRM(page);
		const tabList = page.locator('[data-slot="underline-tabs-list"]').first();
		await expect(tabList).toHaveCSS('overflow-x', 'clip');
		const overflow = await tabList.evaluate((element) => element.scrollWidth - element.clientWidth);
		expect(overflow).toBeLessThanOrEqual(1);
	});

	test('keeps CRM tables left-aligned with intentional column ratios and no horizontal overflow', async ({ page }) => {
		accounts = [{ ...account('account-1', '긴 이름의 기존 관계처'), description: '긴 설명이 있어도 화면 안에서 자연스럽게 줄바꿈되는 관계처입니다.' }];
		contacts = [{ ...contact('contact-1', '박예시 담당자', 'account-1'), title: '사업 개발 및 구매 담당', note: '이메일과 전화번호, 비고를 좁은 화면에서도 모두 확인합니다.' }];
		opportunities = [{ ...opportunity('opportunity-1', '장기 도입 검토 및 파트너십 진행 건', 'lead', 1024), description: '긴 진행 건 설명' }];
		activities = [{
			id: 'activity-1',
			title: '도입 검토 후속 미팅과 요구사항 확인',
			accountID: 'account-1',
			opportunityID: 'opportunity-1',
			business: 'general',
			kind: 'meeting',
			occurredAt: '2026-08-03T09:30:00Z',
			content: '후속 일정과 담당자를 함께 확인하는 긴 활동 비고입니다.',
			audit
		}];

		for (const width of [1280, 1024, 768, 640, 390]) {
			await page.setViewportSize({ width, height: 900 });
			await openCRM(page);
			await expectNoHorizontalOverflow(page.locator('html'));
			await expectNoHorizontalOverflow(page.locator('[data-crm-metrics]'));

			for (const tabName of ['관계처', '연락처', '진행상황', '활동', '리포트']) {
				await page.getByRole('tab', { name: tabName, exact: true }).click();
				const panel = page.getByRole('tabpanel', { name: tabName });
				await expect(panel).toBeVisible();
				await expectNoHorizontalOverflow(panel);
				await expect(panel.locator('tbody tr').first()).toHaveCSS('display', 'table-row');
				await expectTableColumns(panel, responsiveColumnRatios[width][tabName], `${width}px ${tabName}`);
				for (const container of await panel.locator('[data-slot="table-container"]').all()) {
					await expectNoHorizontalOverflow(container);
				}
			}
		}
	});

	test('creates and edits a contact through the service API', async ({ page }) => {
		await openCRM(page);
		await page.getByRole('button', { name: '빠른 추가' }).click();
		await page.getByRole('menuitem', { name: '담당자', exact: true }).click();
		const createSheet = page.getByRole('dialog', { name: 'CRM 기록 추가' });
		await createSheet.getByLabel('연락처 이름').fill('신규 담당자');
		await createSheet.getByLabel('이메일').fill('new-contact@example.com');
		await createSheet.getByRole('button', { name: '추가', exact: true }).click();
		const contactRow = page.getByRole('row', { name: /연락처 수정 · 신규 담당자/ });
		await expect(contactRow).toContainText('new-contact@example.com');
		await expect(contactRow).toContainText('없음');
		await expect(contactRow).not.toContainText('금액 없음');
		contacts[0] = { ...contacts[0], department: '파트너십' };
		await page.reload();
		await expect(page.locator('[data-crm-ready="true"]')).toBeVisible();
		await page.getByRole('tab', { name: '연락처' }).click();

		await page.getByRole('row', { name: /연락처 수정 · 신규 담당자/ }).press('Enter');
		const editSheet = page.getByRole('dialog', { name: '연락처 수정' });
		await editSheet.getByLabel('연락처 이름').fill('수정된 담당자');
		await editSheet.getByRole('button', { name: '저장', exact: true }).click();
		await expect(page.getByRole('row', { name: /연락처 수정 · 수정된 담당자/ })).toBeVisible();
		expect(contacts[0]?.department).toBe('파트너십');
	});

	test('creates a B2C contact without a relationship', async ({ page }) => {
		await openCRM(page);
		await page.getByRole('button', { name: '빠른 추가' }).click();
		await page.getByRole('menuitem', { name: '담당자', exact: true }).click();
		const createSheet = page.getByRole('dialog', { name: 'CRM 기록 추가' });
		await createSheet.getByLabel('관계처').click();
		await page.getByRole('option', { name: '없음', exact: true }).click();
		await createSheet.getByLabel('연락처 이름').fill('개인 고객');
		await createSheet.getByLabel('이메일').fill('individual@example.com');
		await createSheet.getByRole('button', { name: '추가', exact: true }).click();

		expect(contacts[0]?.accountID).toBe('');
		await expect(page.getByRole('row', { name: /연락처 수정 · 개인 고객/ })).toContainText('없음');
	});

	test('requires an email or phone when creating a contact', async ({ page }) => {
		await openCRM(page);
		await page.getByRole('button', { name: '빠른 추가' }).click();
		await page.getByRole('menuitem', { name: '담당자', exact: true }).click();
		const createSheet = page.getByRole('dialog', { name: 'CRM 기록 추가' });
		await createSheet.getByLabel('연락처 이름').fill('연락 수단 없음');
		await createSheet.getByRole('button', { name: '추가', exact: true }).click();

		await expect(createSheet).toContainText('이메일 또는 전화번호를 입력해 주세요.');
		expect(contacts).toHaveLength(0);
	});

	test('requires an email or phone when editing a contact', async ({ page }) => {
		contacts = [contact('contact-1', '기존 담당자', 'account-1')];
		await openCRM(page);
		await page.getByRole('tab', { name: '연락처' }).click();
		await page.getByRole('row', { name: /연락처 수정 · 기존 담당자/ }).press('Enter');
		const editSheet = page.getByRole('dialog', { name: '연락처 수정' });
		await editSheet.getByLabel('이메일').fill('');
		await editSheet.getByLabel('전화번호').fill('');
		await editSheet.getByRole('button', { name: '저장', exact: true }).click();

		await expect(editSheet).toContainText('이메일 또는 전화번호를 입력해 주세요.');
		expect(contacts[0]?.email).toBe('contact-1@example.com');
	});

	test('creates, edits, archives, and reloads a relationship without app fixtures', async ({ page }) => {
		await openCRM(page);
		await page.getByRole('button', { name: '빠른 추가' }).click();
		await page.getByRole('menuitem', { name: '관계처', exact: true }).click();
		const createSheet = page.getByRole('dialog', { name: 'CRM 기록 추가' });
		await createSheet.getByLabel('이름 또는 제목').fill('새 관계처');
		await createSheet.getByRole('button', { name: '추가', exact: true }).click();
		await expect(page.getByLabel('관계처').getByText('새 관계처', { exact: true })).toBeVisible();
		expect(accounts.find((candidate) => candidate.name === '새 관계처')?.types).toEqual([]);

		await page.reload();
		await expect(page.locator('[data-crm-ready="true"]')).toBeVisible();
		await expect(page.getByLabel('관계처').getByText('새 관계처', { exact: true })).toBeVisible();
		await page.getByLabel('관계처').getByText('새 관계처', { exact: true }).click();
		await page.getByRole('dialog').getByRole('button', { name: '수정', exact: true }).click();
		const editSheet = page.getByRole('dialog', { name: '관계처 수정' });
		await expect(editSheet.getByLabel('담당자 이메일')).toBeDisabled();
		await expect(editSheet.getByLabel('마지막 접촉')).toBeDisabled();
		await expect(editSheet.getByLabel('다음 연락일')).toBeDisabled();
		await editSheet.getByLabel('관계처', { exact: true }).fill('수정된 관계처');
		await editSheet.getByRole('checkbox', { name: '파트너', exact: true }).click();
		await editSheet.getByRole('checkbox', { name: '투자 대상', exact: true }).click();
		await editSheet.getByRole('button', { name: '저장', exact: true }).click();
		await expect(page.getByRole('row', { name: /수정된 관계처/ })).toBeVisible();
		expect(accounts.find((candidate) => candidate.name === '수정된 관계처')?.types).toEqual(['partner', 'portfolio']);

		await page.getByRole('row', { name: /수정된 관계처/ }).click();
		await page.getByRole('dialog').getByRole('button', { name: '수정', exact: true }).click();
		const clearTypesSheet = page.getByRole('dialog', { name: '관계처 수정' });
		await clearTypesSheet.getByRole('checkbox', { name: '파트너', exact: true }).click();
		await clearTypesSheet.getByRole('checkbox', { name: '투자 대상', exact: true }).click();
		await clearTypesSheet.getByRole('button', { name: '저장', exact: true }).click();
		expect(accounts.find((candidate) => candidate.name === '수정된 관계처')?.types).toEqual([]);

		await page.getByRole('row', { name: /수정된 관계처/ }).click();
		await page.getByRole('dialog').getByRole('button', { name: '수정', exact: true }).click();
		await page.getByRole('dialog', { name: '관계처 수정' }).getByRole('button', { name: '보관', exact: true }).click();
		await page.getByRole('alertdialog', { name: '보관' }).getByRole('button', { name: '보관', exact: true }).click();
		await expect(page.getByLabel('관계처').getByText('수정된 관계처', { exact: true })).toHaveCount(0);
	});

	test('creates an opportunity, moves its stage, and records an activity', async ({ page }) => {
		await openCRM(page);
		await page.getByRole('button', { name: '빠른 추가' }).click();
		await page.getByRole('menuitem', { name: '진행 건', exact: true }).click();
		const progressSheet = page.getByRole('dialog', { name: 'CRM 기록 추가' });
		await progressSheet.getByLabel('이름 또는 제목').fill('서비스 연결 진행 건');
		await progressSheet.getByRole('button', { name: '추가', exact: true }).click();
		await expect(page.getByText('서비스 연결 진행 건', { exact: true })).toBeVisible();
		opportunities[0] = { ...opportunities[0], contacts: [{ contactID: 'contact-linked', isPrimary: true }] };
		await page.reload();
		await expect(page.locator('[data-crm-ready="true"]')).toBeVisible();
		await page.getByRole('tab', { name: '진행상황' }).click();

		await page.getByText('서비스 연결 진행 건', { exact: true }).click();
		const editSheet = page.getByRole('dialog', { name: '진행 건 수정' });
		await editSheet.getByText('리드', { exact: true }).click();
		await page.getByRole('option', { name: '검토', exact: true }).click();
		await editSheet.getByRole('button', { name: '저장', exact: true }).click();
		await expect(page.getByRole('row', { name: /진행 건 수정 · 서비스 연결 진행 건/ })).toContainText('검토');
		await expect(page.getByRole('row', { name: /진행 건 수정 · 서비스 연결 진행 건/ })).not.toContainText('qualified');
		expect(opportunities[0]?.contacts).toEqual([{ contactID: 'contact-linked', isPrimary: true }]);

		await page.getByRole('button', { name: '빠른 추가' }).click();
		await page.getByRole('menuitem', { name: '활동 기록', exact: true }).click();
		const activitySheet = page.getByRole('dialog', { name: 'CRM 기록 추가' });
		await activitySheet.getByLabel('활동 제목').fill('API 연결 미팅');
		await activitySheet.getByRole('button', { name: '추가', exact: true }).click();
		await expect(page.getByText('API 연결 미팅', { exact: true })).toBeVisible();

		await page.getByRole('tab', { name: '진행상황' }).click();
		await page.getByRole('row', { name: /진행 건 수정 · 서비스 연결 진행 건/ }).click();
		await page.getByRole('dialog', { name: '진행 건 수정' }).getByRole('button', { name: '보관', exact: true }).click();
		await page.getByRole('alertdialog', { name: '보관' }).getByRole('button', { name: '보관', exact: true }).click();
		await expect(page.getByRole('row', { name: /진행 건 수정 · 서비스 연결 진행 건/ })).toHaveCount(0);
	});

	test('creates a contact-only opportunity', async ({ page }) => {
		contacts = [contact('contact-b2c', '개인 고객', '')];
		await openCRM(page);
		await page.getByRole('button', { name: '빠른 추가' }).click();
		await page.getByRole('menuitem', { name: '진행 건', exact: true }).click();
		const createSheet = page.getByRole('dialog', { name: 'CRM 기록 추가' });
		await createSheet.getByLabel('관계처').click();
		await page.getByRole('option', { name: '없음', exact: true }).click();
		await createSheet.getByLabel('이름 또는 제목').fill('개인 고객 상담');
		await createSheet.getByRole('checkbox', { name: '개인 고객', exact: true }).click();
		await createSheet.getByRole('button', { name: '추가', exact: true }).click();

		expect(opportunities[0]?.accountID).toBe('');
		expect(opportunities[0]?.contacts).toEqual([{ contactID: 'contact-b2c', isPrimary: false }]);
	});

	test('requires a relationship or contact when creating an opportunity', async ({ page }) => {
		await openCRM(page);
		await page.getByRole('button', { name: '빠른 추가' }).click();
		await page.getByRole('menuitem', { name: '진행 건', exact: true }).click();
		const createSheet = page.getByRole('dialog', { name: 'CRM 기록 추가' });
		await createSheet.getByLabel('관계처').click();
		await page.getByRole('option', { name: '없음', exact: true }).click();
		await createSheet.getByLabel('이름 또는 제목').fill('연결 대상 없는 진행 건');
		await createSheet.getByRole('button', { name: '추가', exact: true }).click();

		await expect(createSheet).toContainText('관계처 또는 연결 담당자를 선택해 주세요.');
		expect(opportunities).toHaveLength(0);
	});

	test('localizes the missing opportunity relationship in English', async ({ page }) => {
		await page.unroute('**/admin/api/locale**');
		await page.route('**/admin/api/locale**', (route) => route.fulfill({ json: { locale: 'en' } }));
		await openCRM(page);
		await page.getByRole('button', { name: 'Quick add' }).click();
		await page.getByRole('menuitem', { name: 'Opportunity', exact: true }).click();
		const createSheet = page.getByRole('dialog', { name: 'Add CRM record' });
		await createSheet.getByLabel('Relationship').click();
		await page.getByRole('option', { name: 'None', exact: true }).click();
		await createSheet.getByLabel('Name or title').fill('Unlinked opportunity');
		await createSheet.getByRole('button', { name: 'Add', exact: true }).click();

		await expect(createSheet).toContainText('Select a relationship or linked contact.');
		expect(opportunities).toHaveLength(0);
	});

	test('keeps the last contact on a contact-only opportunity', async ({ page }) => {
		contacts = [contact('contact-b2c', '개인 고객', '')];
		opportunities = [{
			...opportunity('opportunity-b2c', '개인 고객 상담', 'lead', 1024),
			accountID: '',
			contacts: [{ contactID: 'contact-b2c', isPrimary: false }]
		}];
		await openCRM(page);
		await page.getByRole('tab', { name: '진행상황' }).click();
		await page.getByRole('row', { name: /개인 고객 상담/ }).click();
		const editSheet = page.getByRole('dialog', { name: '진행 건 수정' });
		await editSheet.getByRole('checkbox', { name: '개인 고객', exact: true }).click();
		await editSheet.getByRole('button', { name: '저장', exact: true }).click();

		await expect(editSheet).toContainText('관계처 또는 연결 담당자를 선택해 주세요.');
		expect(opportunities[0]?.contacts).toEqual([{ contactID: 'contact-b2c', isPrimary: false }]);
	});

	test('moves and sorts pipeline cards through service APIs and keeps the order after reload', async ({ page }) => {
		opportunities = [
			opportunity('opportunity-a', '첫 번째 진행 건', 'lead', 1024),
			opportunity('opportunity-b', '두 번째 진행 건', 'lead', 2048),
			opportunity('opportunity-c', '세 번째 진행 건', 'qualified', 1024)
		];
		await openCRM(page);
		await page.getByRole('tab', { name: '진행상황' }).click();
		await page.getByRole('tab', { name: '보드' }).click();

		await page.locator('[data-crm-opportunity-card="opportunity-a"]').dragTo(
			page.locator('[data-crm-pipeline-drop-zone="qualified"]')
		);
		await expect(page.locator('[data-crm-pipeline-column="qualified"] [data-crm-opportunity-card="opportunity-a"]')).toBeVisible();
		expect(opportunities.find((item) => item.id === 'opportunity-a')?.stage).toBe('qualified');

		await page.locator('[data-crm-opportunity-card="opportunity-c"]').dragTo(
			page.locator('[data-crm-opportunity-card="opportunity-a"]'),
			{ targetPosition: { x: 10, y: 1 } }
		);
		await expect.poll(() => qualifiedOpportunityIDs(opportunities)).toEqual(['opportunity-c', 'opportunity-a']);

		await page.reload();
		await expect(page.locator('[data-crm-ready="true"]')).toBeVisible();
		await page.getByRole('tab', { name: '진행상황' }).click();
		await page.getByRole('tab', { name: '보드' }).click();
		await expect(page.locator('[data-crm-pipeline-column="qualified"] [data-crm-opportunity-card]').first()).toHaveAttribute(
			'data-crm-opportunity-card',
			/opportunity-c/
		);
	});

	test('collects terminal-stage details when a board move closes progress', async ({ page }) => {
		opportunities = [opportunity('opportunity-terminal', '종결할 진행 건', 'qualified', 1024)];
		await openCRM(page);
		await page.getByRole('tab', { name: '진행상황' }).click();
		await page.getByRole('tab', { name: '보드' }).click();

		const dataTransfer = await page.evaluateHandle(() => new DataTransfer());
		const source = page.locator('[data-crm-opportunity-card="opportunity-terminal"]');
		const target = page.locator('[data-crm-pipeline-column="lost"]');
		await source.dispatchEvent('dragstart', { dataTransfer });
		await target.dispatchEvent('dragover', { dataTransfer });
		await target.dispatchEvent('drop', { dataTransfer });
		await source.dispatchEvent('dragend', { dataTransfer });
		const sheet = page.getByRole('dialog', { name: '진행 건 수정' });
		await expect(sheet).toBeVisible();
		await expect(sheet.getByLabel('단계')).toContainText('실패');
		await sheet.getByLabel('손실 사유').click();
		await page.getByRole('option', { name: '예산 부족', exact: true }).click();
		await sheet.getByRole('button', { name: '저장', exact: true }).click();

		expect(opportunities[0]).toMatchObject({
			stage: 'lost',
			lostReason: 'budget',
			baseAmountMinor: 1000,
			baseCurrencyCode: 'KRW'
		});
	});

	test('rejects closing foreign-currency progress without a converted base amount', async ({ page }) => {
		opportunities = [{ ...opportunity('opportunity-foreign', '외화 진행 건', 'qualified', 1024), currencyCode: 'USD' }];
		await openCRM(page);
		await page.getByRole('tab', { name: '진행상황' }).click();
		await page.getByRole('tab', { name: '보드' }).click();

		const dataTransfer = await page.evaluateHandle(() => new DataTransfer());
		const source = page.locator('[data-crm-opportunity-card="opportunity-foreign"]');
		const target = page.locator('[data-crm-pipeline-column="won"]');
		await source.dispatchEvent('dragstart', { dataTransfer });
		await target.dispatchEvent('dragover', { dataTransfer });
		await target.dispatchEvent('drop', { dataTransfer });
		await source.dispatchEvent('dragend', { dataTransfer });
		const sheet = page.getByRole('dialog', { name: '진행 건 수정' });
		await sheet.getByRole('button', { name: '저장', exact: true }).click();

		await expect(sheet.getByText('외화 진행 건은 기준 통화 환산액이 있어야 종결할 수 있습니다.')).toBeVisible();
		expect(opportunities[0]?.stage).toBe('qualified');
	});

	test('creates valued progress directly in a lost stage with its required details', async ({ page }) => {
		opportunities = [opportunity('opportunity-existing-lost', '기존 손실 진행 건', 'lost', 1024)];
		const mutations: Array<{ path: string; payload: Record<string, unknown> }> = [];
		page.on('request', (request) => {
			if (request.method() !== 'GET' && request.url().includes('/crm/api/opportunities')) {
				mutations.push({ path: new URL(request.url()).pathname, payload: recordPayload(request.postDataJSON()) });
			}
		});
		await openCRM(page);
		await page.getByRole('button', { name: '빠른 추가' }).click();
		await page.getByRole('menuitem', { name: '진행 건', exact: true }).click();
		const sheet = page.getByRole('dialog', { name: 'CRM 기록 추가' });
		await sheet.getByLabel('이름 또는 제목').fill('손실 단계 생성 진행 건');
		await sheet.getByLabel('단계').click();
		await page.getByRole('option', { name: '실패', exact: true }).click();
		await sheet.getByLabel('손실 사유').click();
		await page.getByRole('option', { name: '예산 부족', exact: true }).click();
		await sheet.getByLabel('금액').fill('1000');
		await sheet.getByRole('button', { name: '추가', exact: true }).click();

		expect(opportunities[0]).toMatchObject({
			name: '손실 단계 생성 진행 건',
			stage: 'lost',
			lostReason: 'budget',
			baseAmountMinor: 1000,
			baseCurrencyCode: 'KRW',
			stagePosition: 0
		});
		expect(mutations.map((mutation) => mutation.path)).toEqual(['/crm/api/opportunities']);
		expect(numberPayload(recordPayload(mutations[0]?.payload.transition), 'stagePosition')).toBe(0);
	});

	test('keeps realized progress within terminal stages', async ({ page }) => {
		opportunities = [{
			...opportunity('opportunity-realized', '종결된 진행 건', 'won', 1024),
			baseAmountMinor: 1000,
			baseCurrencyCode: 'KRW'
		}];
		await openCRM(page);
		await page.getByRole('tab', { name: '진행상황' }).click();
		await page.getByRole('row', { name: /종결된 진행 건/ }).click();
		const sheet = page.getByRole('dialog', { name: '진행 건 수정' });
		await sheet.getByLabel('단계').click();
		await expect(page.getByRole('option', { name: '성사', exact: true })).toBeVisible();
		await expect(page.getByRole('option', { name: '실패', exact: true })).toBeVisible();
		await expect(page.getByRole('option', { name: '리드', exact: true })).toHaveCount(0);
		await expect(page.getByRole('option', { name: '검토', exact: true })).toHaveCount(0);
		await page.keyboard.press('Escape');
		await page.keyboard.press('Escape');
		await page.getByRole('tab', { name: '보드' }).click();
		await expect(page.locator('[data-crm-opportunity-card="opportunity-realized"]')).toHaveAttribute('draggable', 'false');
	});

	test('replaces linked contacts when an opportunity relationship changes', async ({ page }) => {
		accounts.push(account('account-2', '새 관계처'));
		contacts = [
			contact('contact-old', '기존 담당자', 'account-1'),
			contact('contact-new', '새 담당자', 'account-2')
		];
		opportunities = [{
			...opportunity('opportunity-reassign', '관계처 변경 진행 건', 'lead', 1024),
			contacts: [{ contactID: 'contact-old', isPrimary: true }]
		}];
		await openCRM(page);
		await page.getByRole('tab', { name: '진행상황' }).click();
		await page.getByRole('row', { name: /관계처 변경 진행 건/ }).click();
		const sheet = page.getByRole('dialog', { name: '진행 건 수정' });

		await sheet.getByLabel('관계처').click();
		await page.getByRole('option', { name: '새 관계처', exact: true }).click();
		await sheet.getByText('새 담당자', { exact: true }).click();
		await sheet.getByLabel('주 담당자').click();
		await page.getByRole('option', { name: '새 담당자', exact: true }).click();
		await sheet.getByRole('button', { name: '저장', exact: true }).click();

		expect(opportunities[0]).toMatchObject({
			accountID: 'account-2',
			contacts: [{ contactID: 'contact-new', isPrimary: true }]
		});
	});

	test('keeps activity references consistent when the relationship changes', async ({ page }) => {
		accounts.push(account('account-2', '다른 관계처'));
		opportunities = [opportunity('opportunity-linked', '연결 진행 건', 'lead', 1024)];
		await openCRM(page);
		await page.getByRole('button', { name: '빠른 추가' }).click();
		await page.getByRole('menuitem', { name: '활동 기록', exact: true }).click();
		const sheet = page.getByRole('dialog', { name: 'CRM 기록 추가' });

		await sheet.getByLabel('관련 진행상황').click();
		await page.getByRole('option', { name: '연결 진행 건', exact: true }).click();
		await expect(sheet.getByLabel('사업')).toBeDisabled();

		await sheet.getByLabel('관계처', { exact: true }).click();
		await page.getByRole('option', { name: '다른 관계처', exact: true }).click();
		await expect(sheet.getByLabel('관련 진행상황')).toContainText('진행상황과 연결하지 않음');
		await expect(sheet.getByLabel('사업')).toBeEnabled();

		await sheet.getByLabel('활동 제목').fill('관계처 변경 활동');
		await sheet.getByRole('button', { name: '추가', exact: true }).click();
		expect(activities[0]).toMatchObject({
			accountID: 'account-2',
			opportunityID: '',
			business: 'general'
		});
	});

	test('shows permission errors and completes a mobile create flow', async ({ page }) => {
		permissionDenied = true;
		await page.goto('/crm/');
		await expect(page.getByRole('alert')).toContainText('CRM을 보거나 수정할 권한이 없습니다.');

		permissionDenied = false;
		await page.setViewportSize({ width: 390, height: 844 });
		await page.getByRole('button', { name: '다시 시도' }).click();
		await expect(page.locator('[data-crm-ready="true"]')).toBeVisible();
		await page.getByRole('button', { name: '관계처', exact: true }).last().click();
		const sheet = page.getByRole('dialog', { name: 'CRM 기록 추가' });
		await sheet.getByLabel('이름 또는 제목').fill('모바일 관계처');
		await sheet.getByRole('button', { name: '추가', exact: true }).click();
		await expect(page.getByLabel('관계처').getByText('모바일 관계처', { exact: true })).toBeVisible();
	});

	test('localizes permission errors in English', async ({ page }) => {
		await page.unroute('**/admin/api/locale**');
		await page.route('**/admin/api/locale**', (route) => route.fulfill({ json: { locale: 'en' } }));
		permissionDenied = true;

		await page.goto('/crm/');
		await expect(page.getByRole('alert')).toContainText('You do not have permission to view or edit CRM records.');
	});

	test('reports a refresh failure without retrying a successful create', async ({ page }) => {
		await openCRM(page);
		await page.getByRole('button', { name: '빠른 추가' }).click();
		await page.getByRole('menuitem', { name: '관계처', exact: true }).click();
		const sheet = page.getByRole('dialog', { name: 'CRM 기록 추가' });
		await sheet.getByLabel('이름 또는 제목').fill('새로고침 실패 관계처');
		failNextCRMList = true;
		await sheet.getByRole('button', { name: '추가', exact: true }).click();

		await expect(sheet).not.toBeVisible();
		await expect(page.getByRole('status')).toContainText('CRM 서비스에 저장했습니다.');
		await expect(page.getByRole('alert')).toContainText('저장했지만 목록을 새로 불러오지 못했습니다.');
		expect(accounts.filter((item) => item.name === '새로고침 실패 관계처')).toHaveLength(1);
	});
});

async function openCRM(page: Page): Promise<void> {
	await page.goto('/crm/');
	await expect(page.locator('[data-crm-ready="true"]')).toBeVisible();
}

async function expectNoHorizontalOverflow(locator: Locator): Promise<void> {
	const overflow = await locator.evaluate((element) => element.scrollWidth - element.clientWidth);
	expect(overflow).toBeLessThanOrEqual(1);
}

async function expectTableColumns(panel: Locator, expectedRatios: number[], context: string): Promise<void> {
	const measurements = await panel.getByRole('columnheader').evaluateAll((headers) =>
		headers
			.filter((header) => (header as HTMLElement).offsetParent !== null)
			.map((header) => ({
				width: header.getBoundingClientRect().width,
				textAlign: getComputedStyle(header).textAlign
			}))
	);
	expect(measurements).toHaveLength(expectedRatios.length);
	const totalWidth = measurements.reduce((total, measurement) => total + measurement.width, 0);
	for (let index = 0; index < expectedRatios.length; index += 1) {
		expect(measurements[index].textAlign).toBe('left');
		expect(
			Math.abs(measurements[index].width / totalWidth - expectedRatios[index]),
			`${context} column ${index + 1}`
		).toBeLessThanOrEqual(0.025);
	}
	const cellAlignments = await panel.locator('tbody tr').first().getByRole('cell').evaluateAll((cells) =>
		cells
			.filter((cell) => (cell as HTMLElement).offsetParent !== null)
			.map((cell) => getComputedStyle(cell).textAlign)
	);
	expect(cellAlignments).toHaveLength(expectedRatios.length);
	expect(cellAlignments.every((alignment) => alignment === 'left')).toBe(true);
}

async function installSessionRoutes(page: Page): Promise<void> {
	await page.route('**/auth/session**', (route) => route.fulfill({ json: { authenticated: true, email: 'crm@example.com', canViewTasks: true } }));
	await page.route('**/admin/api/session**', (route) => route.fulfill({ json: { email: 'crm@example.com', role: 'admin' } }));
	await page.route('**/admin/api/locale**', (route) => route.fulfill({ json: { locale: 'ko' } }));
	await page.route('**/calendar/api/**', (route) => route.fulfill({ json: { events: [] } }));
	await page.route('**/flow/api/**', (route) => route.fulfill({ json: {} }));
	await page.route('**/agent/api/**', (route) => {
		const path = new URL(route.request().url()).pathname;
		return route.fulfill({ json: path === '/agent/api/buzz-vault' ? { found: true } : {} });
	});
}

async function handleCRMRoute(route: Route, accounts: Account[], contacts: Contact[], opportunities: Opportunity[], activities: Activity[]): Promise<void> {
	const request = route.request();
	const url = new URL(request.url());
	const path = url.pathname;
	const method = request.method();
	if (path === '/crm/api/accounts' && method === 'GET') return fulfill(route, { accounts });
	if (path === '/crm/api/accounts' && method === 'POST') {
		const payload = recordPayload(request.postDataJSON());
		const created: Account = { ...payload, id: `account-${accounts.length + 1}`, name: stringPayload(payload, 'name'), audit };
		accounts.unshift(created);
		return fulfill(route, { account: created }, 201);
	}
	const accountMatch = path.match(/^\/crm\/api\/accounts\/([^/]+)(\/archive)?$/);
	if (accountMatch && method === 'PUT') {
		const index = accounts.findIndex((account) => account.id === accountMatch[1]);
		const payload = recordPayload(request.postDataJSON());
		const updated: Account = { ...accounts[index], ...payload, id: accountMatch[1], name: stringPayload(payload, 'name'), audit };
		accounts[index] = updated;
		return fulfill(route, { account: updated });
	}
	if (accountMatch?.[2] === '/archive' && method === 'POST') {
		const index = accounts.findIndex((account) => account.id === accountMatch[1]);
		accounts.splice(index, 1);
		return fulfill(route, { id: accountMatch[1], archivedAt: new Date().toISOString() });
	}
	if (path === '/crm/api/contacts' && method === 'GET') return fulfill(route, { contacts });
	if (path === '/crm/api/contacts' && method === 'POST') {
		const payload = recordPayload(request.postDataJSON());
		const created: Contact = { ...payload, id: `contact-${contacts.length + 1}`, name: stringPayload(payload, 'name'), audit };
		contacts.unshift(created);
		return fulfill(route, { contact: created }, 201);
	}
	const contactMatch = path.match(/^\/crm\/api\/contacts\/([^/]+)$/);
	if (contactMatch && method === 'PUT') {
		const index = contacts.findIndex((contact) => contact.id === contactMatch[1]);
		const payload = recordPayload(request.postDataJSON());
		const updated: Contact = { ...contacts[index], ...payload, id: contactMatch[1], name: stringPayload(payload, 'name'), audit };
		contacts[index] = updated;
		return fulfill(route, { contact: updated });
	}
	if (path === '/crm/api/opportunities' && method === 'GET') return fulfill(route, { opportunities });
	if (path === '/crm/api/opportunities' && method === 'POST') {
		const payload = recordPayload(request.postDataJSON());
		const { transition: transitionValue, ...fields } = payload;
		const transition = isRecord(transitionValue) ? transitionValue : undefined;
		const transitionStage = transition ? stringPayload(transition, 'stage') : 'lead';
		const requestedStagePosition = transition ? numberPayload(transition, 'stagePosition') : 0;
		const created: Opportunity = {
			...fields,
			id: `opportunity-${opportunities.length + 1}`,
			name: stringPayload(payload, 'name'),
			stage: transitionStage,
			stagePosition: createStagePosition(opportunities, transitionStage, requestedStagePosition),
			stageChangedAt: new Date().toISOString(),
			currencyCode: typeof payload.currencyCode === 'string' ? payload.currencyCode : '',
			...(transition ?? {}),
			audit
		};
		opportunities.unshift(created);
		return fulfill(route, { opportunity: created }, 201);
	}
	const opportunityMatch = path.match(/^\/crm\/api\/opportunities\/([^/]+)(?:\/(transition|position|archive))?$/);
	if (opportunityMatch && !opportunityMatch[2] && method === 'PUT') {
		const index = opportunities.findIndex((opportunity) => opportunity.id === opportunityMatch[1]);
		const current = opportunities[index];
		const payload = recordPayload(request.postDataJSON());
		const { transition: transitionValue, ...fields } = payload;
		const transition = isRecord(transitionValue) ? transitionValue : undefined;
		const updated: Opportunity = {
			...current,
			...fields,
			...(transition ?? {}),
			id: current.id,
			name: stringPayload(payload, 'name'),
			stage: transition ? stringPayload(transition, 'stage') : current.stage,
			stagePosition: transition ? numberPayload(transition, 'stagePosition') : current.stagePosition,
			stageChangedAt: transition ? new Date().toISOString() : current.stageChangedAt,
			audit
		};
		opportunities[index] = updated;
		return fulfill(route, { opportunity: updated });
	}
	if (opportunityMatch?.[2] === 'transition' && method === 'POST') {
		const index = opportunities.findIndex((opportunity) => opportunity.id === opportunityMatch[1]);
		const payload = recordPayload(request.postDataJSON());
		opportunities[index] = { ...opportunities[index], ...payload, stage: stringPayload(payload, 'stage'), stagePosition: numberPayload(payload, 'stagePosition'), stageChangedAt: new Date().toISOString() };
		return fulfill(route, { opportunity: opportunities[index] });
	}
	if (opportunityMatch?.[2] === 'position' && method === 'POST') {
		const index = opportunities.findIndex((opportunity) => opportunity.id === opportunityMatch[1]);
		const payload = recordPayload(request.postDataJSON());
		const [moved] = opportunities.splice(index, 1);
		const beforeOpportunityID = stringPayload(payload, 'beforeOpportunityID');
		const beforeIndex = beforeOpportunityID
			? opportunities.findIndex((opportunity) => opportunity.id === beforeOpportunityID)
			: -1;
		const insertIndex = beforeIndex >= 0 ? beforeIndex : opportunities.length;
		moved.stagePosition = numberPayload(payload, 'position');
		opportunities.splice(insertIndex, 0, moved);
		return fulfill(route, { opportunity: moved });
	}
	if (opportunityMatch?.[2] === 'archive' && method === 'POST') {
		const index = opportunities.findIndex((opportunity) => opportunity.id === opportunityMatch[1]);
		opportunities.splice(index, 1);
		return fulfill(route, { id: opportunityMatch[1], archivedAt: new Date().toISOString() });
	}
	if (path === '/crm/api/activities' && method === 'GET') return fulfill(route, { activities });
	if (path === '/crm/api/activities' && method === 'POST') {
		const payload = recordPayload(request.postDataJSON());
		const created: Activity = { ...payload, id: `activity-${activities.length + 1}`, title: stringPayload(payload, 'title'), audit };
		activities.unshift(created);
		return fulfill(route, { activity: created }, 201);
	}
	if (path === '/crm/api/pipelines' && method === 'GET') return fulfill(route, { pipelines: [{ pipeline: 'sales', label: '판매', direction: 'outbound', isActive: true }] });
	if (path === '/crm/api/pipelines/sales/stages' && method === 'GET') return fulfill(route, { stages: [{ pipeline: 'sales', stage: 'lead', position: 1, outcome: 'open' }, { pipeline: 'sales', stage: 'qualified', position: 2, outcome: 'open' }, { pipeline: 'sales', stage: 'won', position: 3, outcome: 'won' }, { pipeline: 'sales', stage: 'lost', position: 4, outcome: 'lost' }] });
	if (path === '/crm/api/lost-reasons' && method === 'GET') return fulfill(route, { lostReasons: [{ reason: 'budget', label: '예산 부족', isActive: true }] });
	await route.fulfill({ status: 404, json: { error: { code: 'not_found', message: path } } });
}

function account(id: string, name: string): Account {
	return { id, name, status: 'active', types: ['customer'], tags: [], importance: 'high', ownerPersonID: 'person-crm', ownerCircleID: 'team-sales', description: '', audit };
}

function opportunity(id: string, name: string, stage: string, stagePosition: number): Opportunity {
	return {
		id,
		name,
		accountID: 'account-1',
		business: 'general',
		pipeline: 'sales',
		stage,
		stagePosition,
		stageChangedAt: '2026-08-03T00:00:00Z',
		ownerPersonID: 'person-crm',
		amountMinor: 1000,
		currencyCode: 'KRW',
		importance: 'medium',
		contacts: [],
		audit
	};
}

function contact(id: string, name: string, accountID: string): Contact {
	return {
		id,
		name,
		accountID,
		email: `${id}@example.com`,
		phone: '',
		title: '',
		department: '',
		isPrimary: false,
		ownerPersonID: 'person-crm',
		ownerCircleID: 'team-sales',
		description: '',
		audit
	};
}

function qualifiedOpportunityIDs(opportunities: Opportunity[]): string[] {
	return opportunities.filter((opportunity) => opportunity.stage === 'qualified').map((opportunity) => opportunity.id);
}

function createStagePosition(opportunities: Opportunity[], stage: string, requestedPosition: number): number {
	if (requestedPosition !== 0) return requestedPosition;
	const positions = opportunities.filter((opportunity) => opportunity.stage === stage).map((opportunity) => opportunity.stagePosition);
	return positions.length === 0 ? 1024 : Math.min(...positions) - 1024;
}

async function fulfill(route: Route, json: object, status = 200): Promise<void> {
	await route.fulfill({ status, json });
}

function recordPayload(value: unknown): Record<string, unknown> {
	if (!isRecord(value)) throw new Error('expected an object request payload');
	return value;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringPayload(payload: Record<string, unknown>, key: string): string {
	const value = payload[key];
	if (typeof value !== 'string') throw new Error(`expected ${key} to be a string`);
	return value;
}

function numberPayload(payload: Record<string, unknown>, key: string): number {
	const value = payload[key];
	if (typeof value !== 'number') throw new Error(`expected ${key} to be a number`);
	return value;
}
