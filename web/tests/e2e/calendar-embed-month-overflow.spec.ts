import { expect, test, type Locator, type Page } from '@playwright/test';
import {
	computedPseudoStyle,
	routeCalendarEvents,
	routeCalendarLocale,
	routeDefaultCalendarAPI
} from './calendar-embed-test-utils';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar month overflow', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('collapses overflowing same-day month events behind a more button without crossing the date cell boundary', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'overflow-long-event',
				title: 'Overflow Long',
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-20T00:00:00+09:00',
				isAllDay: true,
				updatedAt: '2026-06-01T00:00:00Z'
			},
			...Array.from({ length: 8 }, (_, index) => ({
				id: `overflow-short-${index}`,
				title: `Overflow Short ${index}`,
				startISO: '2026-06-17T09:00:00+09:00',
				endISO: '2026-06-17T10:00:00+09:00',
				isAllDay: false,
				updatedAt: `2026-06-${String(10 + index).padStart(2, '0')}T00:00:00Z`
			}))
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-17');

		const moreButton = page.locator('.calendar-month-more-button[data-date-key="2026-06-17"]');
		await expect(moreButton).toBeVisible();
		await expect(moreButton).toContainText(/\+\d+ 더보기/);
		await expect(moreButton).toHaveAttribute('aria-label', /숨겨진 일정 \d+개 보기/);
		await expect(page.locator('.df-month-more-events')).toBeHidden();
		await expect(page.getByText(/\+\d+ more/)).toBeHidden();

		const overflowMeasurements = await page.evaluate(() => {
			const cell = document.querySelector<HTMLElement>('.df-month-day-cell[data-date="2026-06-17"]');
			if (!cell) throw new Error('Missing 2026-06-17 date cell');
			const cellRectangle = cell.getBoundingClientRect();
			const eventRectangles = Array.from(
				document.querySelectorAll<HTMLElement>('.calendar-month-direct-event[data-event-id^="overflow-"]')
			).map((element) => element.getBoundingClientRect());
			return {
				cellBottom: Math.round(cellRectangle.bottom),
				visibleEventCount: eventRectangles.length,
				maxEventBottom: Math.max(...eventRectangles.map((rectangle) => Math.round(rectangle.bottom)))
			};
		});
		expect(overflowMeasurements.visibleEventCount).toBeLessThan(9);
		expect(overflowMeasurements.maxEventBottom).toBeLessThanOrEqual(overflowMeasurements.cellBottom - 18);

		await moreButton.click();
		const morePopover = page.locator('.calendar-month-more-popover');
		await expect(morePopover).toBeVisible();
		await expect(morePopover.locator('.calendar-month-more-popover-event')).toHaveCount(9 - overflowMeasurements.visibleEventCount);
	});

	test('shows more buttons on every covered date when hidden multi-day events overflow', async ({ page }) => {
		await routeCalendarEvents(
			page,
			Array.from({ length: 10 }, (_, index) => ({
				id: `overflow-multi-day-${index}`,
				title: `Overflow Multi Day ${index}`,
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-21T00:00:00+09:00',
				isAllDay: true,
				updatedAt: `2026-06-${String(10 + index).padStart(2, '0')}T00:00:00Z`
			}))
		);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-17');

		const coveredDateMoreButton = page.locator('.calendar-month-more-button[data-date-key="2026-06-18"]');
		await expect(coveredDateMoreButton).toBeVisible();
		await expect(coveredDateMoreButton).toContainText(/\+\d+ 더보기/);

		await coveredDateMoreButton.click();
		const morePopover = page.locator('.calendar-month-more-popover');
		await expect(morePopover).toBeVisible();
		await expect(morePopover.locator('.calendar-month-more-popover-event').first()).toContainText('Overflow Multi Day');
	});

	test('exposes a named overflow dialog and moves focus to its first hidden event', async ({ page }) => {
		const { moreButton } = await openOverflowCalendar(page, 'dialog-semantics');

		await expect(moreButton).toHaveAttribute('aria-haspopup', 'dialog');
		await expect(moreButton).toHaveAttribute('aria-expanded', 'false');
		const controlledDialogID = await moreButton.getAttribute('aria-controls');
		expect(controlledDialogID).toBeTruthy();

		await moreButton.click();

		await expect(moreButton).toHaveAttribute('aria-expanded', 'true');
		const dialog = page.locator(`#${controlledDialogID}`);
		const dialogTitle = dialog.locator('.calendar-month-more-popover-title');
		await expect(dialog).toHaveAttribute('role', 'dialog');
		await expect(dialog).toHaveAttribute('aria-labelledby', await dialogTitle.getAttribute('id') ?? '');
		await expect(dialogTitle).toHaveText('6. 17.');
		await expect(dialog.locator('.calendar-month-more-popover-event').first()).toBeFocused();
	});

	test('returns focus to the overflow trigger when Escape closes the overflow dialog', async ({ page }) => {
		const { moreButton, morePopover } = await openOverflowCalendar(page, 'escape-focus');
		await moreButton.click();
		await expect(morePopover.locator('.calendar-month-more-popover-event').first()).toBeFocused();

		await page.keyboard.press('Escape');

		await expect(morePopover).toHaveCount(0);
		await expect(moreButton).toHaveAttribute('aria-expanded', 'false');
		await expect(moreButton).toBeFocused();
	});

	test('lets the desktop editor handle Escape before the overflow dialog', async ({ page }) => {
		const { moreButton, morePopover } = await openOverflowCalendar(page, 'desktop-editor-escape');
		await moreButton.click();
		const hiddenEvent = morePopover.locator('.calendar-month-more-popover-event').first();
		await hiddenEvent.click();
		const editor = page.locator('.calendar-draft-popover');
		await expect(editor.getByLabel('제목')).toBeFocused();

		await page.keyboard.press('Escape');

		await expect(editor).toHaveCount(0);
		await expect(morePopover).toBeVisible();
		await expect(moreButton).toHaveAttribute('aria-expanded', 'true');
		await expect(hiddenEvent).toBeFocused();
	});

	test('keeps the overflow dialog open while interacting with the desktop editor', async ({ page }) => {
		const { moreButton, morePopover } = await openOverflowCalendar(page, 'desktop-editor-pointer');
		await moreButton.click();
		await morePopover.locator('.calendar-month-more-popover-event').first().click();
		const editor = page.locator('.calendar-draft-popover');
		await editor.getByLabel('장소').click();

		await expect(editor).toBeVisible();
		await expect(morePopover).toBeVisible();
		await expect(moreButton).toHaveAttribute('aria-expanded', 'true');
	});

	test('lets the mobile editor handle Escape before the overflow dialog', async ({ page }) => {
		await page.setViewportSize({ width: 600, height: 900 });
		const { moreButton, morePopover } = await openOverflowCalendar(page, 'mobile-editor-escape');
		await moreButton.click();
		const hiddenEvent = morePopover.locator('.calendar-month-more-popover-event').first();
		await hiddenEvent.click();
		const editor = page.locator('.calendar-mobile-event-editor [role="dialog"]');
		await expect(editor).toBeVisible();

		await page.keyboard.press('Escape');

		await expect(editor).toHaveCount(0);
		await expect(morePopover).toBeVisible();
		await expect(moreButton).toHaveAttribute('aria-expanded', 'true');
		await expect(hiddenEvent).toBeFocused();
	});

	test('formats hidden event dates with the active locale', async ({ page }) => {
		await routeCalendarLocale(page, 'en');
		const { moreButton, morePopover } = await openOverflowCalendar(page, 'english-date');

		await moreButton.click();

		await expect(morePopover.locator('.calendar-month-more-popover-title')).toHaveText('6/17');
		await expect(morePopover.locator('.calendar-month-more-popover-event-date').first()).toHaveText(/^6\/17 \d{2}:00$/);
	});

	test('shows a visible overflow trigger focus ring in standard and forced color modes', async ({ page }) => {
		const { moreButton } = await openOverflowCalendar(page, 'trigger-focus-ring');
		await moreButton.focus();

		const standardFocusStyle = await moreButton.evaluate((element) => {
			const style = window.getComputedStyle(element);
			return { boxShadow: style.boxShadow, outlineStyle: style.outlineStyle };
		});
		expect(standardFocusStyle.boxShadow).not.toBe('none');

		await page.emulateMedia({ forcedColors: 'active' });
		const forcedColorFocusStyle = await moreButton.evaluate((element) => {
			const style = window.getComputedStyle(element);
			return { boxShadow: style.boxShadow, outlineColor: style.outlineColor, outlineStyle: style.outlineStyle };
		});
		expect(forcedColorFocusStyle.outlineStyle).toBe('solid');
		expect(forcedColorFocusStyle.outlineColor).not.toBe('rgba(0, 0, 0, 0)');
	});

	test('opens a hidden month event editor when activated from the keyboard', async ({ page }) => {
		await routeCalendarEvents(
			page,
			Array.from({ length: 8 }, (_, index) => ({
				id: `keyboard-overflow-${index}`,
				title: `Keyboard Overflow ${index}`,
				startISO: `2026-06-17T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-17T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false
			}))
		);
		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		await page.locator('.calendar-month-more-button[data-date-key="2026-06-17"]').click();

		const hiddenEvent = page.locator('.calendar-month-more-popover-event').first();
		await expect(hiddenEvent).toHaveAttribute('aria-pressed', 'false');
		await hiddenEvent.focus();
		await page.keyboard.press('Enter');

		await expect(page.locator('.calendar-stage')).toHaveAttribute('data-calendar-selected-date-key', '2026-06-17');
		await expect(hiddenEvent).toHaveAttribute('aria-pressed', 'true');
		await expect(hiddenEvent).toHaveClass(/internkim-calendar-event-focused/);
		await expect.poll(() => hiddenEvent.evaluate((element) => window.getComputedStyle(element).backgroundColor)).toBe(
			'rgb(37, 99, 235)'
		);
		const selectedColors = await hiddenEvent.evaluate((element) => {
			const dateElement = element.querySelector<HTMLElement>('.calendar-month-more-popover-event-date');
			if (!dateElement) throw new Error('Missing overflow event date');
			return {
				date: window.getComputedStyle(dateElement).color,
				indicator: window.getComputedStyle(element, '::before').backgroundColor
			};
		});
		expect(selectedColors).toEqual({ date: 'rgb(255, 255, 255)', indicator: 'rgb(255, 255, 255)' });
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.getByLabel('제목')).toHaveValue(/Keyboard Overflow/);
	});

	test('opens the correct hidden event and date from a synthesized accessible click', async ({ page }) => {
		await routeCalendarEvents(
			page,
			Array.from({ length: 8 }, (_, index) => ({
				id: `accessible-overflow-${index}`,
				title: `Accessible Overflow ${index}`,
				startISO: `2026-06-17T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-17T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false
			}))
		);
		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		await page.locator('.calendar-month-more-button[data-date-key="2026-06-17"]').click();

		const hiddenEvent = page.locator('.calendar-month-more-popover-event').first();
		const hiddenEventTitle = await hiddenEvent.locator('.calendar-month-more-popover-event-title').textContent();
		await hiddenEvent.dispatchEvent('click');

		await expect(page.locator('.calendar-stage')).toHaveAttribute('data-calendar-selected-date-key', '2026-06-17');
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.getByLabel('제목')).toHaveValue(hiddenEventTitle ?? '');
	});

	test('uses the active covered date for a synthesized hidden multi-day event click', async ({ page }) => {
		const hiddenEvent = await openCoveredDateOverflowEvent(page, 'synthesized-covered-date');
		const hiddenEventTitle = await hiddenEvent.locator('.calendar-month-more-popover-event-title').textContent();

		await hiddenEvent.dispatchEvent('click');

		await expect(page.locator('.calendar-stage')).toHaveAttribute('data-calendar-selected-date-key', '2026-06-18');
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.getByLabel('제목')).toHaveValue(hiddenEventTitle ?? '');
	});

	test('uses the active covered date for keyboard activation of a hidden multi-day event', async ({ page }) => {
		const hiddenEvent = await openCoveredDateOverflowEvent(page, 'keyboard-covered-date');

		await hiddenEvent.focus();
		await page.keyboard.press('Enter');

		await expect(page.locator('.calendar-stage')).toHaveAttribute('data-calendar-selected-date-key', '2026-06-18');
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
	});

	test('paints the selected date cell behind a more button as a full cell layer', async ({ page }) => {
		await routeCalendarEvents(
			page,
			Array.from({ length: 8 }, (_, index) => ({
				id: `selected-overflow-${index}`,
				title: `Selected Overflow ${index}`,
				startISO: `2026-06-17T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-17T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false,
				updatedAt: `2026-06-${String(10 + index).padStart(2, '0')}T00:00:00Z`
			}))
		);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-17');
		await expect(page.locator('.calendar-month-more-button[data-date-key="2026-06-17"]')).toBeVisible();

		const selectedCellLayerStyle = await computedPseudoStyle(
			page,
			'.df-month-day-cell[data-date="2026-06-17"].month-selected-date',
			'::before',
			['content', 'position', 'inset', 'background-color']
		);

		expect(selectedCellLayerStyle.content).not.toBe('none');
		expect(selectedCellLayerStyle.position).toBe('absolute');
		expect(selectedCellLayerStyle.inset).toBe('0px');
		expect(selectedCellLayerStyle['background-color']).not.toBe('rgba(0, 0, 0, 0)');
	});
});

async function openCoveredDateOverflowEvent(page: Page, eventIDPrefix: string): Promise<Locator> {
	const targetEventID = `${eventIDPrefix}-target`;
	await routeCalendarEvents(
		page,
		[
			...Array.from({ length: 10 }, (_, index) => ({
				id: `${eventIDPrefix}-filler-${index}`,
				title: `${eventIDPrefix} filler ${index}`,
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-21T00:00:00+09:00',
				isAllDay: true,
				updatedAt: `2026-06-${String(10 + index).padStart(2, '0')}T00:00:00Z`
			})),
			{
				id: targetEventID,
				title: `${eventIDPrefix} target`,
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-21T00:00:00+09:00',
				isAllDay: true,
				updatedAt: '2026-06-01T00:00:00Z'
			}
		]
	);
	await openCalendarEmbed(page, '월');
	await navigateEmbeddedCalendar(page, '2026-06-17');
	const coveredDateMoreButton = page.locator('.calendar-month-more-button[data-date-key="2026-06-18"]');
	await expect(coveredDateMoreButton).toBeVisible();
	await coveredDateMoreButton.click();
	const hiddenEvent = page.locator(`.calendar-month-more-popover-event[data-event-id="${targetEventID}"]`);
	await expect(hiddenEvent).toBeVisible();
	return hiddenEvent;
}

async function openOverflowCalendar(
	page: Page,
	eventIDPrefix: string
): Promise<{ moreButton: Locator; morePopover: Locator }> {
	await routeCalendarEvents(
		page,
		Array.from({ length: 8 }, (_, index) => ({
			id: `${eventIDPrefix}-${index}`,
			title: `${eventIDPrefix} ${index}`,
			startISO: `2026-06-17T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
			endISO: `2026-06-17T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
			isAllDay: false
		}))
	);
	await openCalendarEmbed(page, '월');
	await navigateEmbeddedCalendar(page, '2026-06-17');
	const moreButton = page.locator('.calendar-month-more-button[data-date-key="2026-06-17"]');
	await expect(moreButton).toBeVisible();
	return {
		moreButton,
		morePopover: page.locator('.calendar-month-more-popover')
	};
}
