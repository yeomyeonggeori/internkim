import { expect, type Page } from '@playwright/test';

export async function expectDayTimelineRowsRightBorderHidden(page: Page): Promise<void> {
	await page.evaluate(() => {
		window.scrollTo(0, document.documentElement.scrollHeight);
	});

	await expect
		.poll(async () =>
			page.evaluate(() => {
				const rows = document.querySelector('.df-day-content-grid-rows');
				const boundary = document.querySelector('.df-day-content-grid-boundary-bottom');
				if (!(rows instanceof HTMLElement) || !(boundary instanceof HTMLElement)) {
					return {
						status: 'missing',
						hasRowsRightBorder: true
					};
				}
				const rowsStyle = window.getComputedStyle(rows);
				const rowsRightLineStyle = window.getComputedStyle(rows, '::before');
				const boundaryRectangle = boundary.getBoundingClientRect();
				const rowsRectangle = rows.getBoundingClientRect();
				const rowsRightLineBottom = rowsRectangle.bottom - Number.parseFloat(rowsRightLineStyle.bottom);
				return {
					status: 'measured',
					hasRowsRightBorder: rowsStyle.borderRightStyle !== 'none' && Number.parseFloat(rowsStyle.borderRightWidth) > 0,
					hasRowsRightLine:
						rowsRightLineStyle.content !== 'none' &&
						rowsRightLineStyle.backgroundColor === 'rgb(225, 229, 235)' &&
						rowsRightLineStyle.right === '0px' &&
						Number.parseFloat(rowsRightLineStyle.width) >= 1,
					rowsRightLineStopsAtBoundary: Math.abs(rowsRightLineBottom - boundaryRectangle.top) <= 1,
					rowsBottom: Math.round(rowsRectangle.bottom),
					boundaryBottom: Math.round(boundaryRectangle.bottom)
				};
			})
		)
		.toMatchObject({
			status: 'measured',
			hasRowsRightBorder: false,
			hasRowsRightLine: true,
			rowsRightLineStopsAtBoundary: true
		});
}

export async function expectDayRightPanelDividerContinuous(page: Page): Promise<void> {
	await expect
		.poll(async () =>
			page.evaluate(() => {
				const dayContent = document.querySelector('.df-day-content');
				const rightPanel = document.querySelector('.df-right-panel');
				if (!(dayContent instanceof HTMLElement) || !(rightPanel instanceof HTMLElement)) {
					return {
						status: 'missing'
					};
				}
				const dayContentDividerStyle = window.getComputedStyle(dayContent, '::before');
				const rightPanelDividerStyle = window.getComputedStyle(rightPanel, '::before');
				const rightPanelRectangle = rightPanel.getBoundingClientRect();
				return {
					status: 'measured',
					dayContentDividerContent: dayContentDividerStyle.content,
					rightPanelDividerBackground: rightPanelDividerStyle.backgroundColor,
					rightPanelDividerBottom: rightPanelDividerStyle.bottom,
					rightPanelDividerContent: rightPanelDividerStyle.content,
					rightPanelDividerCoversPanel:
						Math.abs(Number.parseFloat(rightPanelDividerStyle.height) - rightPanelRectangle.height) <= 1,
					rightPanelDividerHeight: Math.round(Number.parseFloat(rightPanelDividerStyle.height)),
					rightPanelDividerLeft: rightPanelDividerStyle.left,
					rightPanelDividerTop: rightPanelDividerStyle.top,
					rightPanelDividerWidth: Math.round(Number.parseFloat(rightPanelDividerStyle.width)),
					rightPanelDividerZIndex: Number.parseInt(rightPanelDividerStyle.zIndex, 10),
					rightPanelHeight: Math.round(rightPanelRectangle.height)
				};
			})
		)
		.toMatchObject({
			status: 'measured',
			dayContentDividerContent: 'none',
			rightPanelDividerBackground: 'rgb(225, 229, 235)',
			rightPanelDividerBottom: '0px',
			rightPanelDividerContent: '""',
			rightPanelDividerCoversPanel: true,
			rightPanelDividerLeft: '0px',
			rightPanelDividerTop: '0px',
			rightPanelDividerWidth: 1,
			rightPanelDividerZIndex: 6
		});
}
