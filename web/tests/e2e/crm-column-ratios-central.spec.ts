import { expect, test, type Locator } from '@playwright/test';
import {
	expectNoHorizontalOverflow,
	organizationIDNamed,
	removeCRMActivities,
	seedCRMActivity,
	signInToTheCRM
} from './crm-central-test-utils';

test.use({ locale: 'ko-KR' });

const activityTitle = 'E2E 컬럼 비율 검증 활동';

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

let seededActivityID = '';

test.beforeAll(async () => {
	const organizationID = await organizationIDNamed('예시 협력 기관');
	seededActivityID = await seedCRMActivity(organizationID, activityTitle);
});

test.afterAll(async () => {
	await removeCRMActivities([seededActivityID]);
});

test('keeps CRM table alignment intentional with stable column ratios and no horizontal overflow', async ({ page }) => {
	test.setTimeout(150_000);

	for (const width of [1280, 1024, 768, 640, 390]) {
		await page.setViewportSize({ width, height: 900 });
		await signInToTheCRM(page);
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

async function expectTableColumns(panel: Locator, expectedRatios: number[], context: string): Promise<void> {
	const measurements = await panel.getByRole('columnheader').evaluateAll((headers) =>
		headers
			.filter((header) => (header as HTMLElement).offsetParent !== null)
			.map((header) => ({
				contentLeft: header.getBoundingClientRect().left + Number.parseFloat(getComputedStyle(header).paddingLeft),
				center: header.getBoundingClientRect().left + header.getBoundingClientRect().width / 2,
				label: header.textContent?.trim() ?? '',
				width: header.getBoundingClientRect().width,
				textAlign: getComputedStyle(header).textAlign
			}))
	);
	expect(measurements).toHaveLength(expectedRatios.length);
	const totalWidth = measurements.reduce((total, measurement) => total + measurement.width, 0);
	for (let index = 0; index < expectedRatios.length; index += 1) {
		const expectedAlignment = ['단계', '활동 종류', '활동 상태'].includes(measurements[index].label) ? 'center' : 'left';
		expect(measurements[index].textAlign).toBe(expectedAlignment);
		expect(
			Math.abs(measurements[index].width / totalWidth - expectedRatios[index]),
			`${context} column ${index + 1}`
		).toBeLessThanOrEqual(0.025);
	}
	const cellMeasurements = await panel.locator('tbody tr').first().getByRole('cell').evaluateAll((cells) =>
		cells
			.filter((cell) => (cell as HTMLElement).offsetParent !== null)
			.map((cell) => ({
				contentLeft: cell.getBoundingClientRect().left + Number.parseFloat(getComputedStyle(cell).paddingLeft),
				center: cell.getBoundingClientRect().left + cell.getBoundingClientRect().width / 2,
				textAlign: getComputedStyle(cell).textAlign
			}))
	);
	expect(cellMeasurements).toHaveLength(expectedRatios.length);
	for (let index = 0; index < cellMeasurements.length; index += 1) {
		const expectedAlignment = ['단계', '활동 종류', '활동 상태'].includes(measurements[index].label) ? 'center' : 'left';
		expect(cellMeasurements[index].textAlign).toBe(expectedAlignment);
		if (expectedAlignment === 'center') {
			expect(Math.abs(cellMeasurements[index].center - measurements[index].center), `${context} column ${index + 1} center`).toBeLessThanOrEqual(1);
		} else if (!['유형', '연결 일정'].includes(measurements[index].label)) {
			expect(Math.abs(cellMeasurements[index].contentLeft - measurements[index].contentLeft), `${context} column ${index + 1} content start`).toBeLessThanOrEqual(1);
		}
	}
	const leadingPills = await panel.locator('tbody tr').first().locator('[data-crm-leading-pill]:visible').evaluateAll((pills) =>
		pills.map((pill) => {
			const cell = pill.closest('td');
			if (!(cell instanceof HTMLTableCellElement)) throw new Error('CRM leading pill must be inside a table cell');
			const header = cell.closest('table')?.tHead?.rows[0]?.cells[cell.cellIndex];
			if (!(header instanceof HTMLTableCellElement)) throw new Error('CRM leading pill column header is missing');
			return {
				headerTextLeft: header.getBoundingClientRect().left + Number.parseFloat(getComputedStyle(header).paddingLeft),
				pillLeft: pill.getBoundingClientRect().left
			};
		})
	);
	for (const [index, pill] of leadingPills.entries()) {
		expect(Math.abs(pill.pillLeft - pill.headerTextLeft), `${context} leading pill ${index + 1} start`).toBeLessThanOrEqual(1);
	}
	const leadingPillTexts = await panel.locator('tbody tr').first().locator('[data-crm-leading-pill-text]:visible').evaluateAll((pills) =>
		pills.map((pill) => {
			const cell = pill.closest('td');
			if (!(cell instanceof HTMLTableCellElement)) throw new Error('CRM text-aligned pill must be inside a table cell');
			const header = cell.closest('table')?.tHead?.rows[0]?.cells[cell.cellIndex];
			if (!(header instanceof HTMLTableCellElement)) throw new Error('CRM text-aligned pill column header is missing');
			const textNode = [...pill.childNodes].find((node) => node.nodeType === Node.TEXT_NODE && node.textContent?.trim());
			if (!textNode) throw new Error('CRM text-aligned pill text is missing');
			const range = document.createRange();
			range.selectNodeContents(textNode);
			return {
				headerTextLeft: header.getBoundingClientRect().left + Number.parseFloat(getComputedStyle(header).paddingLeft),
				pillTextLeft: range.getBoundingClientRect().left
			};
		})
	);
	for (const [index, pill] of leadingPillTexts.entries()) {
		expect(Math.abs(pill.pillTextLeft - pill.headerTextLeft), `${context} leading pill text ${index + 1} start`).toBeLessThanOrEqual(1);
	}
	const centeredPills = await panel.locator('tbody tr').first().locator('[data-crm-centered-pill]:visible').evaluateAll((pills) =>
		pills.map((pill) => {
			const cell = pill.closest('td');
			if (!(cell instanceof HTMLTableCellElement)) throw new Error('CRM centered pill must be inside a table cell');
			const header = cell.closest('table')?.tHead?.rows[0]?.cells[cell.cellIndex];
			if (!(header instanceof HTMLTableCellElement)) throw new Error('CRM centered pill column header is missing');
			return {
				headerCenter: header.getBoundingClientRect().left + header.getBoundingClientRect().width / 2,
				pillCenter: pill.getBoundingClientRect().left + pill.getBoundingClientRect().width / 2
			};
		})
	);
	for (const [index, pill] of centeredPills.entries()) {
		expect(Math.abs(pill.pillCenter - pill.headerCenter), `${context} centered pill ${index + 1}`).toBeLessThanOrEqual(1);
	}
}
