import { expect, type Locator, type Page } from '@playwright/test';

type GoogleCalendarRouteEntry = {
	calendarID: string;
	summary: string;
	accessRole: string;
	timeZone?: string;
	primary: boolean;
	backgroundColor: string;
	canWrite?: boolean;
	canSelect?: boolean;
	selectionDisabledReason?: string;
};

type GoogleCalendarRouteOptions = {
	setSelectedCalendarID: (calendarID: string) => void;
	accountEmail?: string;
	calendars?: GoogleCalendarRouteEntry[];
};

const defaultWritableGoogleCalendars: GoogleCalendarRouteEntry[] = [
	{
		calendarID: 'family@example.com',
		summary: '가족',
		accessRole: 'owner',
		primary: false,
		backgroundColor: '#0ea5e9'
	},
	{
		calendarID: 'primary',
		summary: 'calendar-admin@example.com',
		accessRole: 'owner',
		primary: true,
		backgroundColor: '#22c55e'
	}
];

export async function routeCalendarShellAPI(page: Page): Promise<void> {
	await page.route('**/admin/api/session', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
	});
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
	});
	await page.route('**/calendar/api/sync', async (route) => {
		await route.fulfill({
			json: {
				caldavURL: '',
				caldavUsername: '',
				caldavPassword: '',
				icsURL: ''
			}
		});
	});
	await page.route('**/calendar/api/account-status', async (route) => {
		await route.fulfill({
			json: {
				connected: false,
				needsReauth: false,
				googleOAuthConfigured: true,
				canManageGoogleOAuth: false
			}
		});
	});
	await page.route('**/calendar/api/events?**', async (route) => {
		await route.fulfill({
			json: {
				events: [
					{
						id: 'event-2026-06-15',
						title: 'Shell event',
						startISO: '2026-06-15T09:00:00.000Z',
						endISO: '2026-06-15T10:00:00.000Z',
						isAllDay: false
					}
				]
			}
		});
	});
	await page.route('**/calendar/api/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], source: 'holiday_api' } });
	});
	await page.route('**/calendar/api/participants', async (route) => {
		await route.fulfill({ json: { participants: [] } });
	});
	await page.route('**/calendar/api/google-calendars**', async (route) => {
		if (route.request().method() !== 'GET') {
			await route.fallback();
			return;
		}
		await route.fulfill({ json: { accountEmail: '', calendars: [] } });
	});
	await page.route('**/calendar/api/remote-sync', async (route) => {
		await route.fulfill({ json: { synced: false } });
	});
	await page.route('**/calendar/api/conflicts', async (route) => {
		await route.fulfill({ json: { conflicts: [] } });
	});
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
}

export async function routeConnectedGoogleCalendarAccount(page: Page): Promise<void> {
	await page.unroute('**/calendar/api/account-status');
	await page.route('**/calendar/api/account-status', async (route) => {
		await route.fulfill({
			json: {
				connected: true,
				accountEmail: 'calendar-admin@example.com',
				selectedCalendarID: '',
				selectedCalendarName: '',
				selectedCalendarAccessRole: '',
				needsReauth: false,
				needsCalendarSelection: true,
				initialSyncCompleted: false,
				calendarSyncReady: false,
				googleOAuthConfigured: true,
				canManageGoogleOAuth: true
			}
		});
	});
}

export async function routeWritableGoogleCalendars(page: Page, options: GoogleCalendarRouteOptions): Promise<void> {
	const accountEmail = options.accountEmail ?? 'calendar-admin@example.com';
	const calendars = (options.calendars ?? defaultWritableGoogleCalendars).map(normalizeGoogleCalendarRouteEntry);
	await page.unroute('**/calendar/api/google-calendars**');
	await page.route('**/calendar/api/google-calendars**', async (route) => {
		if (route.request().method() === 'GET') {
			await route.fulfill({
				json: {
					accountEmail,
					calendars
				}
			});
			return;
		}
		const body = JSON.parse(route.request().postData() ?? '{}') as { calendarID?: string };
		const selectedCalendarID = body.calendarID ?? '';
		options.setSelectedCalendarID(selectedCalendarID);
		const selectedCalendar = calendars.find((calendar) => calendar.calendarID === selectedCalendarID) ?? calendars[0];
		await route.fulfill({
			json: {
				accountEmail,
				selectedCalendar,
				calendarURL: `https://apidata.googleusercontent.com/caldav/v2/${selectedCalendarID}/events/`
			}
		});
	});
}

export async function openCalendarSettings(page: Page): Promise<void> {
	const calendarFrame = page.frameLocator('iframe');
	await expect(calendarFrame.locator('.calendar-toolbar-title')).toBeVisible();
	const settingsButton = calendarFrame.getByRole('button', { name: '설정' });
	const settingsHeading = page.getByRole('heading', { name: '설정' });
	await expect
		.poll(async () => {
			if ((await settingsHeading.count()) > 0) return true;
			await settingsButton.click();
			await page.waitForTimeout(100);
			return (await settingsHeading.count()) > 0;
		})
		.toBe(true);
	await expect(page.getByRole('heading', { name: '설정' })).toBeVisible();
}

export async function expectGoogleOAuthUploadLayoutToFit(page: Page): Promise<void> {
	await expectElementNotToOverflow(page.locator('[data-slot="sheet-content"]'));
	await expectElementNotToOverflow(page.getByRole('group', { name: 'Google OAuth client.json' }));
}

async function expectElementNotToOverflow(locator: Locator): Promise<void> {
	await expect(locator).toBeVisible();
	await expect
		.poll(async () =>
			locator.evaluate((element) => {
				const htmlElement = element as HTMLElement;
				return htmlElement.scrollWidth <= htmlElement.clientWidth;
			})
		)
		.toBe(true);
}

function normalizeGoogleCalendarRouteEntry(calendar: GoogleCalendarRouteEntry): GoogleCalendarRouteEntry {
	const canWrite = calendar.canWrite ?? (calendar.accessRole === 'writer' || calendar.accessRole === 'owner');
	return {
		...calendar,
		timeZone: calendar.timeZone ?? 'Asia/Seoul',
		canWrite,
		canSelect: calendar.canSelect ?? canWrite
	};
}
