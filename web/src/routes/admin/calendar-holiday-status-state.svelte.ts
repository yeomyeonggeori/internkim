import { apiErrorMessage, fetchCalendarHolidayStatus, refreshCalendarHolidayStatus } from './admin-api';
import type { CalendarHolidayStatus } from './admin-types';

export class CalendarHolidayStatusState {
	status = $state<CalendarHolidayStatus | null>(null);
	message = $state('');
	isLoading = $state(false);
	isRefreshing = $state(false);

	async load(adminBaseURL: string, fallbackMessage: string): Promise<void> {
		if (!adminBaseURL || this.isLoading) return;
		this.isLoading = true;
		this.message = '';
		try {
			this.status = await fetchCalendarHolidayStatus(adminBaseURL, fallbackMessage);
		} catch (error) {
			this.message = apiErrorMessage(error, fallbackMessage);
		} finally {
			this.isLoading = false;
		}
	}

	async refresh(adminBaseURL: string, fallbackMessage: string): Promise<void> {
		if (!adminBaseURL || this.isRefreshing) return;
		this.isRefreshing = true;
		this.message = '';
		try {
			this.status = await refreshCalendarHolidayStatus(adminBaseURL, fallbackMessage);
		} catch (error) {
			this.message = apiErrorMessage(error, fallbackMessage);
		} finally {
			this.isRefreshing = false;
		}
	}
}
