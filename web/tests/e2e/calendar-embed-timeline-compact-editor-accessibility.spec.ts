import { expect, test } from '@playwright/test';
import { openCalendarEmbed } from './calendar-embed-interaction-helpers';
import {
	accessibleEventActivator,
	accessibleEventBlock,
	colorContrastRatio,
	expectForcedColorFocusRing,
	prepareAccessibleParticipantSuggestions
} from './calendar-embed-timeline-editor-accessibility-helpers';
import {
	routeCalendarDeleteIntents,
	routeCalendarEvents,
	routeCalendarEventUpdates,
	routeDefaultCalendarAPI
} from './calendar-embed-test-utils';

test.describe('embedded calendar event editor accessibility', () => {
	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await routeDefaultCalendarAPI(page);
		await routeCalendarEvents(page, [
			{
				id: 'accessible-editor-event',
				title: 'Accessible Editor Event',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:00:00+09:00',
				isAllDay: false
			}
		]);
		await openCalendarEmbed(page, '일');
	});

	test('exposes the calendar stage as a named focus fallback region', async ({ page }) => {
		await expect(page.getByRole('region', { name: '일정 · 김인턴' })).toHaveClass(/calendar-stage/);
	});

	test('exposes the compact editor as a named modal and restores event focus after Escape', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();
		await eventActivator.dblclick();

		const dialog = page.getByRole('dialog', { name: '일정 편집' });
		await expect(dialog).toBeVisible();
		await expect(dialog.locator('input[data-mobile-editor-field="title"]')).toBeFocused();

		await dialog.locator('input[data-mobile-editor-field="location"]').focus();
		await page.keyboard.press('Escape');

		await expect(dialog).toHaveCount(0);
		await expect(eventActivator).toBeFocused();
	});

	test('keeps Tab focus inside the compact editor', async ({ page }) => {
		await accessibleEventActivator(page).dblclick();
		const dialog = page.getByRole('dialog', { name: '일정 편집' });
		const deleteButton = dialog.locator('[data-mobile-editor-action="delete"]');
		await deleteButton.focus();

		await page.keyboard.press('Tab');

		await expect(dialog.locator('[data-mobile-editor-action="close"]')).toBeFocused();
	});

	test('closes compact participant suggestions before closing the editor with Escape', async ({ page }) => {
		await prepareAccessibleParticipantSuggestions(page);
		await accessibleEventActivator(page).dblclick();
		const dialog = page.getByRole('dialog', { name: '일정 편집' });
		const participantCombobox = dialog.getByRole('combobox', { name: '참여자' });
		await participantCombobox.focus();
		await expect(participantCombobox).toHaveAttribute('aria-expanded', 'true');

		await page.keyboard.press('Escape');

		await expect(dialog).toBeVisible();
		await expect(participantCombobox).toHaveAttribute('aria-expanded', 'false');
		await page.keyboard.press('Escape');
		await expect(dialog).toHaveCount(0);
	});

	test('opens the compact editor when a focused event is activated with Enter', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();

		await page.keyboard.press('Enter');

		await expect(page.getByRole('dialog', { name: '일정 편집' })).toBeVisible();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('opens the compact editor when a focused event is activated with Space', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();

		await page.keyboard.press('Space');

		await expect(page.getByRole('dialog', { name: '일정 편집' })).toBeVisible();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('opens the compact editor when Shift is held during Enter activation', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();

		await page.keyboard.press('Shift+Enter');

		await expect(page.getByRole('dialog', { name: '일정 편집' })).toBeVisible();
	});

	test('keeps the event selected while interacting with the compact editor', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page);
		const eventBlock = accessibleEventBlock(page);
		await eventActivator.dblclick();
		const dialog = page.getByRole('dialog', { name: '일정 편집' });

		await expect(eventBlock).toHaveClass(/internkim-calendar-event-focused/);
		await dialog.locator('input[data-mobile-editor-field="location"]').click();

		await expect(eventBlock).toHaveClass(/internkim-calendar-event-focused/);
	});

	test('includes the visible event time in the accessible name', async ({ page }) => {
		await expect(accessibleEventActivator(page)).toHaveAccessibleName(/Accessible Editor Event.*09:00/);
	});

	test('reports whether the event is selected on its activator', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page);
		await expect(eventActivator).toHaveAttribute('aria-pressed', 'false');

		await accessibleEventBlock(page).click();

		await expect(eventActivator).toHaveAttribute('aria-pressed', 'true');
	});

	test('keeps selected event text at WCAG AA contrast', async ({ page }) => {
		const eventBlock = accessibleEventBlock(page);
		await eventBlock.click();

		const colors = await eventBlock.evaluate((element) => {
			const style = window.getComputedStyle(element);
			return { background: style.backgroundColor, foreground: style.color };
		});

		expect(colorContrastRatio(colors.foreground, colors.background)).toBeGreaterThanOrEqual(4.5);
	});

	test('preserves the selected event state in forced colors', async ({ page }) => {
		await page.emulateMedia({ forcedColors: 'active' });
		const eventBlock = accessibleEventBlock(page);
		await eventBlock.click();

		const selectionStyle = await eventBlock.evaluate((element) => {
			const reference = document.createElement('div');
			reference.style.backgroundColor = 'Highlight';
			reference.style.color = 'HighlightText';
			reference.style.forcedColorAdjust = 'none';
			document.body.appendChild(reference);
			const style = window.getComputedStyle(element);
			const referenceStyle = window.getComputedStyle(reference);
			const result = {
				background: style.backgroundColor,
				foreground: style.color,
				forcedColorAdjust: style.forcedColorAdjust,
				referenceBackground: referenceStyle.backgroundColor,
				referenceForeground: referenceStyle.color
			};
			reference.remove();
			return result;
		});

		expect(selectionStyle.forcedColorAdjust).toBe('none');
		expect(selectionStyle.background).toBe(selectionStyle.referenceBackground);
		expect(selectionStyle.foreground).toBe(selectionStyle.referenceForeground);
	});

	test('keeps the compact editor field focus ring visible in forced colors', async ({ page }) => {
		await page.emulateMedia({ forcedColors: 'active' });
		await accessibleEventActivator(page).dblclick();

		await expectForcedColorFocusRing(page.locator('input[data-mobile-editor-field="title"]'));
	});

	test('opens the compact editor from a synthesized accessible click', async ({ page }) => {
		await accessibleEventActivator(page).dispatchEvent('click');

		await expect(accessibleEventBlock(page)).toHaveClass(/internkim-calendar-event-focused/);
		await expect(page.getByRole('dialog', { name: '일정 편집' })).toBeVisible();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('restores focus to the replaced event after saving', async ({ page }) => {
		const updatedEvents = await routeCalendarEventUpdates(page);
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();
		await eventActivator.dblclick();
		const dialog = page.getByRole('dialog', { name: '일정 편집' });
		await dialog.locator('input[data-mobile-editor-field="title"]').fill('Accessible Editor Event Updated');
		await dialog.getByRole('button', { name: '완료' }).click();

		await expect.poll(() => updatedEvents.length).toBe(1);
		await expect(accessibleEventActivator(page)).toBeFocused();
	});

	test('moves focus to the calendar after deleting the focused event', async ({ page }) => {
		const deleteIntentRequests = await routeCalendarDeleteIntents(page);
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();
		await eventActivator.dblclick();
		await page.getByRole('dialog', { name: '일정 편집' }).getByRole('button', { name: '삭제' }).click();

		await expect.poll(() => deleteIntentRequests.registeredEventIDs).toEqual(['accessible-editor-event']);
		await expect(page.locator('.calendar-stage')).toBeFocused();
	});
});
