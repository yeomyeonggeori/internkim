import { expect, test, type Page } from '@playwright/test';
import { openCalendarEmbed } from './calendar-embed-interaction-helpers';
import {
	accessibleEventActivator,
	expectForcedColorFocusRing,
	prepareAccessibleParticipantSuggestions
} from './calendar-embed-timeline-editor-accessibility-helpers';
import {
	routeCalendarDeleteIntents,
	routeCalendarEvents,
	routeCalendarEventUpdates,
	routeDefaultCalendarAPI,
	type CalendarTestLocale
} from './calendar-embed-test-utils';

test.describe('desktop anchored calendar event editor accessibility', () => {
	test.beforeEach(async ({ page }) => {
		await prepareDesktopAccessibleEditor(page);
	});

	test('opens a localized named edit dialog with Enter', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();

		await page.keyboard.press('Enter');

		const dialog = page.getByRole('dialog', { name: '일정 편집' });
		await expect(dialog).toBeVisible();
		await expect(dialog).not.toHaveAttribute('aria-modal');
	});

	test('opens a localized named edit dialog with Space', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();

		await page.keyboard.press('Space');

		await expect(page.getByRole('dialog', { name: '일정 편집' })).toBeVisible();
	});

	test('moves focus into the edit title field', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();

		await page.keyboard.press('Enter');

		await expect(page.locator('.calendar-draft-popover').getByLabel('제목')).toBeFocused();
	});

	test('closes when Escape is pressed from another editor field', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();
		await page.keyboard.press('Enter');
		const popover = page.locator('.calendar-draft-popover');
		await popover.getByLabel('장소').focus();

		await page.keyboard.press('Escape');

		await expect(popover).toHaveCount(0);
		await expect(eventActivator).toBeFocused();
	});

	test('closes desktop participant suggestions before closing the editor with Escape', async ({ page }) => {
		await prepareAccessibleParticipantSuggestions(page);
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();
		await page.keyboard.press('Enter');
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

	test('restores focus to the originating activator after close', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();
		await page.keyboard.press('Enter');
		const titleInput = page.locator('.calendar-draft-popover').getByLabel('제목');
		await titleInput.focus();

		await page.keyboard.press('Escape');

		await expect(eventActivator).toBeFocused();
	});

	test('keeps focus on an outside control after pointer dismiss', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();
		await page.keyboard.press('Enter');
		const searchInput = page.getByRole('textbox', { name: '일정 검색' });

		await searchInput.click();

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(searchInput).toBeFocused();
		await expect(eventActivator).toHaveAttribute('aria-pressed', 'false');
		await page.waitForTimeout(1_200);
		await expect(searchInput).toBeFocused();
	});

	test('keeps the event focus indicator visible in forced colors', async ({ page }) => {
		await page.emulateMedia({ forcedColors: 'active' });
		const eventActivator = accessibleEventActivator(page);

		await eventActivator.focus();

		const focusStyle = await eventActivator.evaluate((element) => {
			const style = window.getComputedStyle(element);
			return {
				outlineColor: style.outlineColor,
				outlineStyle: style.outlineStyle,
				outlineWidth: style.outlineWidth
			};
		});
		expect(focusStyle.outlineStyle).toBe('solid');
		expect(Number.parseFloat(focusStyle.outlineWidth)).toBeGreaterThanOrEqual(2);
		expect(focusStyle.outlineColor).not.toBe('rgba(0, 0, 0, 0)');
	});

	test('keeps the desktop editor field focus ring visible in forced colors', async ({ page }) => {
		await page.emulateMedia({ forcedColors: 'active' });
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();
		await page.keyboard.press('Enter');

		await expectForcedColorFocusRing(page.locator('.calendar-draft-popover').getByLabel('제목'));
	});

	test('uses the existing English edit event name', async ({ page }) => {
		await prepareDesktopAccessibleEditor(page, 'en');
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();

		await page.keyboard.press('Enter');

		await expect(page.getByRole('dialog', { name: 'Edit Event' })).toBeVisible();
	});

	test('uses the existing English new event name', async ({ page }) => {
		await prepareDesktopAccessibleEditor(page, 'en');

		await page.locator('.desktop-new-event-button').click();

		const dialog = page.getByRole('dialog', { name: 'New Event' });
		await expect(dialog).toBeVisible();
		await expect(dialog.getByLabel('Title')).toBeFocused();
	});

	test('restores focus to the replacement activator after saving', async ({ page }) => {
		const updatedEvents = await routeCalendarEventUpdates(page);
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();
		await page.keyboard.press('Enter');
		const dialog = page.getByRole('dialog', { name: '일정 편집' });
		await dialog.getByLabel('제목').fill('Accessible Editor Event Updated');

		await dialog.getByRole('button', { name: '완료' }).click();

		await expect.poll(() => updatedEvents.length).toBe(1);
		await expect(accessibleEventActivator(page)).toBeFocused();
	});

	test('moves focus to the calendar stage after deleting the event', async ({ page }) => {
		const deleteIntentRequests = await routeCalendarDeleteIntents(page);
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.focus();
		await page.keyboard.press('Enter');

		await page.getByRole('dialog', { name: '일정 편집' }).getByRole('button', { name: '삭제' }).click();

		await expect.poll(() => deleteIntentRequests.registeredEventIDs).toEqual(['accessible-editor-event']);
		await expect(page.locator('.calendar-stage')).toBeFocused();
	});

	test('keeps pointer double click in the current page', async ({ context, page }) => {
		const eventActivator = accessibleEventActivator(page);
		await eventActivator.dblclick();
		const dialog = page.getByRole('dialog', { name: '일정 편집' });

		await expect(dialog).toBeVisible();
		expect(context.pages()).toHaveLength(1);
		await dialog.getByLabel('장소').focus();
		await page.keyboard.press('Escape');
		await expect(eventActivator).toBeFocused();
	});
});

async function prepareDesktopAccessibleEditor(page: Page, locale: CalendarTestLocale = 'ko'): Promise<void> {
	await page.setViewportSize({ width: 768, height: 900 });
	await routeDefaultCalendarAPI(page, locale);
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
}
