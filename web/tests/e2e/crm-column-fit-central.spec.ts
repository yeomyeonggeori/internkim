import { expect, test, type Locator, type Page } from '@playwright/test';
import {
	expectNoHorizontalOverflow,
	organizationIDNamed,
	removeCRMActivities,
	seedCRMActivity,
	signInToTheCRM
} from './crm-central-test-utils';

test.use({ locale: 'ko-KR' });

const activityTitle = 'E2E 컬럼 맞춤 검증 활동';

// The four list tables size themselves to their content; the report's owner table keeps its own layout.
const contentSizedTabs = ['관계처', '연락처', '거래', '활동'];

const rightAlignedColumns: Record<string, string[]> = {
	관계처: ['마지막 연락', '다음 활동', '열린 거래 금액'],
	연락처: [],
	거래: ['금액', '마감 예정일', '경과 기간'],
	활동: ['시작일', '종료일'],
	보고: []
};

const reportTables: Record<string, string[]> = {
	quiet: ['마지막 연락', '경과'],
	owner: ['관계처', '열린 거래', '열린 거래 금액', '성사 금액', '활동 없는 거래']
};

let seededActivityID = '';

test.beforeAll(async () => {
	const organizationID = await organizationIDNamed('예시 협력 기관');
	seededActivityID = await seedCRMActivity(organizationID, activityTitle);
});

test.afterAll(async () => {
	await removeCRMActivities([seededActivityID]);
});

test('fits every CRM table to its content with the name column absorbing the slack', async ({ page }) => {
	test.setTimeout(150_000);

	for (const width of [390, 768, 1024, 1440]) {
		await page.setViewportSize({ width, height: 900 });
		await signInToTheCRM(page);
		await expectNoHorizontalOverflow(page.locator('html'));
		await expectNoHorizontalOverflow(page.locator('[data-crm-metrics]'));

		for (const tabName of ['관계처', '연락처', '거래', '활동', '보고']) {
			await page.getByRole('tab', { name: tabName, exact: true }).click();
			const panel = page.getByRole('tabpanel', { name: tabName });
			await expect(panel).toBeVisible();
			await expectNoHorizontalOverflow(panel);
			await expect(panel.locator('tbody tr').first()).toHaveCSS('display', 'table-row');
			for (const container of await panel.locator('[data-slot="table-container"]').all()) {
				await expectNoHorizontalOverflow(container);
			}
			if (tabName === '보고') {
				for (const [card, rightAligned] of Object.entries(reportTables)) {
					await expectColumnFit(
						panel.locator(`[data-crm-report-card="${card}"]`),
						rightAligned,
						false,
						`${width}px 보고 ${card}`
					);
				}
				continue;
			}

			await expectColumnFit(
				panel,
				rightAlignedColumns[tabName] ?? [],
				contentSizedTabs.includes(tabName),
				`${width}px ${tabName}`
			);
		}
	}
});

async function expectColumnFit(
	panel: Locator,
	rightAligned: string[],
	isContentSized: boolean,
	context: string
): Promise<void> {
	const columns = await panel.getByRole('columnheader').evaluateAll((headers) =>
		headers
			.filter((header) => (header as HTMLElement).offsetParent !== null)
			.map((header) => ({
				label: header.textContent?.trim() ?? '',
				width: header.getBoundingClientRect().width,
				textAlign: getComputedStyle(header).textAlign
			}))
	);
	expect(columns.length, `${context} has columns`).toBeGreaterThan(1);

	if (isContentSized) {
		// The name column is first and takes every pixel the others do not need.
		const widest = columns.reduce((largest, column) => (column.width > largest.width ? column : largest));
		expect(widest.label, `${context} widest column`).toBe(columns[0].label);
	}

	for (const column of columns) {
		const expected = rightAligned.includes(column.label) ? 'right' : 'left';
		expect(column.textAlign, `${context} column ${column.label} alignment`).toBe(expected);
	}

	if (!isContentSized) return;

	// Every column except the name one sizes to its content, so its text never wraps and never clips.
	const secondaryCells = await panel
		.locator('tbody tr')
		.first()
		.getByRole('cell')
		.evaluateAll((cells) =>
			cells
				.filter((cell) => (cell as HTMLElement).offsetParent !== null)
				.slice(1)
				.map((cell) => ({
					label: cell.textContent?.trim().slice(0, 20) ?? '',
					whiteSpace: getComputedStyle(cell).whiteSpace,
					clipped: cell.scrollWidth > cell.clientWidth + 1
				}))
		);

	for (const cell of secondaryCells) {
		expect(cell.whiteSpace, `${context} cell "${cell.label}" does not wrap`).toBe('nowrap');
		expect(cell.clipped, `${context} cell "${cell.label}" is not clipped`).toBe(false);
	}
}
