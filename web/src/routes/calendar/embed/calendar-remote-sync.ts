import { responseErrorMessage } from './calendar-event-persistence';

let remoteSyncInFlight: Promise<void> | null = null;

export async function syncRemoteCalendar(errorFallback: string): Promise<void> {
	if (remoteSyncInFlight) return remoteSyncInFlight;
	remoteSyncInFlight = fetch('/calendar/api/remote-sync', {
		method: 'POST',
		credentials: 'include'
	}).then(async (response) => {
		if (!response.ok) throw new Error(await responseErrorMessage(response, errorFallback));
	});
	try {
		await remoteSyncInFlight;
	} finally {
		remoteSyncInFlight = null;
	}
}

export async function syncRemoteCalendarAndRefreshConflicts(
	errorFallback: string,
	refreshCalendar: () => Promise<void>,
	loadCalendarConflicts: () => Promise<void>
): Promise<void> {
	try {
		await syncRemoteCalendar(errorFallback);
		await refreshCalendar();
	} finally {
		await loadCalendarConflicts();
	}
}
