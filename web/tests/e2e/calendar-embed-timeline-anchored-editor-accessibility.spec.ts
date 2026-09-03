import { expect, test, type Page } from '@playwright/test';
import { cleanupCalendarEvents, seedCalendarEvents, signInToCalendar } from './calendar-central-test-utils';
import {
	accessibleEventActivator,
	prepareAccessibleParticipantSuggestions
} from './calendar-embed-timeline-editor-accessibility-helpers';

test.describe('desktop anchored calendar event editor accessibility', () => {
	test.use({ locale: 'ko-KR' });

	let eventID = '';

	test.beforeEach(async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await routeCalendarHolidays(page);
		[eventID] = await seedCalendarEvents([
			{ title: 'Accessible Editor Event', startISO: '2026-06-08T09:00:00+09:00', endISO: '2026-06-08T10:00:00+09:00' }
		]);
		await openDesktopDayView(page);
	});

	test.afterEach(async () => {
		await cleanupCalendarEvents([eventID]);
	});

	test('opens a localized named edit dialog with Enter', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page, eventID);
		await eventActivator.focus();

		await page.keyboard.press('Enter');

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover).toHaveAttribute('aria-label', '일정 편집');
	});

	test('opens a localized named edit dialog with Space', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page, eventID);
		await eventActivator.focus();

		await page.keyboard.press('Space');

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover).toHaveAttribute('aria-label', '일정 편집');
	});

	test('closes when Escape is pressed from another editor field', async ({ page }) => {
		const eventActivator = accessibleEventActivator(page, eventID);
		await eventActivator.focus();
		await page.keyboard.press('Enter');
		const popover = page.locator('.calendar-draft-popover');
		await popover.getByLabel('장소').focus();

		await page.keyboard.press('Escape');

		await expect(popover).toHaveCount(0);
	});

	test('closes desktop participant suggestions before closing the editor with Escape', async ({ page }) => {
		await prepareAccessibleParticipantSuggestions(page);
		const eventActivator = accessibleEventActivator(page, eventID);
		await eventActivator.focus();
		await page.keyboard.press('Enter');
		const popover = page.locator('.calendar-draft-popover');
		const participantCombobox = popover.getByRole('combobox', { name: '참여자' });
		await participantCombobox.click();
		await expect(participantCombobox).toHaveAttribute('aria-expanded', 'true');

		await page.keyboard.press('Escape');

		await expect(popover).toBeVisible();
		await expect(participantCombobox).toHaveAttribute('aria-expanded', 'false');
		await page.keyboard.press('Escape');
		await expect(popover).toHaveCount(0);
	});

	test('uses the existing English edit event name', async ({ page }) => {
		await switchToEnglishLocale(page);
		const eventActivator = accessibleEventActivator(page, eventID);
		await eventActivator.focus();

		await page.keyboard.press('Enter');

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover).toHaveAttribute('aria-label', 'Edit Event');
	});

	test('uses the existing English new event name', async ({ page }) => {
		await switchToEnglishLocale(page);

		await page.locator('[data-calendar-date="2026-06-08"]').click();

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover).toHaveAttribute('aria-label', 'New Event');
		await expect(popover.getByLabel('Title')).toBeFocused();
		await page.keyboard.press('Escape');
	});

	test('saves an edit made through the keyboard-opened editor', async ({ page }) => {
		const updatedEvents = await routeEventUpdateInvoke(page);
		const eventActivator = accessibleEventActivator(page, eventID);
		await eventActivator.focus();
		await page.keyboard.press('Enter');
		const popover = page.locator('.calendar-draft-popover');
		await popover.getByLabel('제목').fill('Accessible Editor Event Updated');

		await popover.getByLabel('제목').press('Enter');

		await expect(popover).toHaveCount(0);
		await expect.poll(() => updatedEvents.length).toBe(1);
		expect(updatedEvents[0]?.title).toBe('Accessible Editor Event Updated');
	});

	test('deletes the event from the keyboard-opened editor', async ({ page }) => {
		const deletedEventHints = await routeEventDeleteInvoke(page);
		const eventActivator = accessibleEventActivator(page, eventID);
		await eventActivator.focus();
		await page.keyboard.press('Enter');
		const popover = page.locator('.calendar-draft-popover');

		await popover.getByRole('button', { name: '삭제' }).click();

		await expect(popover).toHaveCount(0);
		await expect(page.locator(`[data-calendar-event-id="${eventID}"]`)).toHaveCount(0);
		await expect(page.locator('.calendar-delete-undo-toast')).toBeVisible();
		await expect.poll(() => deletedEventHints, { timeout: 8000 }).toContain(eventID);
	});

	test('keeps pointer click in the current page', async ({ context, page }) => {
		const eventActivator = accessibleEventActivator(page, eventID);
		await eventActivator.click();
		const popover = page.locator('.calendar-draft-popover');

		await expect(popover).toBeVisible();
		expect(context.pages()).toHaveLength(1);
		await popover.getByLabel('장소').focus();
		await page.keyboard.press('Escape');
		await expect(popover).toHaveCount(0);
	});
});

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

async function openDesktopDayView(page: Page): Promise<void> {
	await page.setViewportSize({ width: 1280, height: 900 });
	await signInToCalendar(page);
	await page.goto('/calendar/embed?date=2026-06-08');
	await page.evaluate(() => window.localStorage.setItem('internkim.calendar.view', 'day'));
	await page.reload();
	await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-day/);
}

async function switchToEnglishLocale(page: Page): Promise<void> {
	await page.evaluate(() => window.localStorage.setItem('internkim.locale', 'en'));
	await page.reload();
	await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-day/);
}

async function routeEventUpdateInvoke(page: Page): Promise<{ eventID: string; title: string }[]> {
	const updated: { eventID: string; title: string }[] = [];
	await page.route('**/api/v1/tools/event_update/invoke', async (route) => {
		const payload = route.request().postDataJSON() as { input: { eventHint: string; title: string } };
		updated.push({ eventID: payload.input.eventHint, title: payload.input.title });
		await route.fulfill({
			json: {
				result: {
					eventID: payload.input.eventHint,
					title: payload.input.title,
					note: '',
					location: '',
					startsAt: '2026-06-08T09:00:00.000Z',
					endsAt: '2026-06-08T10:00:00.000Z',
					isWholeDay: false,
					participants: [],
					updatedAt: '2026-06-08T00:00:00.000Z'
				}
			}
		});
	});
	return updated;
}

async function routeEventDeleteInvoke(page: Page): Promise<string[]> {
	const deleted: string[] = [];
	await page.route('**/api/v1/tools/event_delete/invoke', async (route) => {
		const payload = route.request().postDataJSON() as { input: { eventHint: string } };
		deleted.push(payload.input.eventHint);
		await route.fulfill({ json: { result: {} } });
	});
	return deleted;
}
