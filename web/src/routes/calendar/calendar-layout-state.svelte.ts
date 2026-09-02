import type { CalendarSyncResponse } from './calendar-layout-types';
import { calendarDateKey } from './calendar-layout-date';

export class CalendarLayoutState {
	syncInformation = $state<CalendarSyncResponse | null>(null);
	isSyncSheetOpen = $state(false);
	isRotatingSync = $state(false);
	syncError = $state('');
	selectedDateKey = $state('');
	today = new Date();

	applyVisibleDate(date: Date): void {
		this.selectedDateKey = calendarDateKey(date);
	}

	openSyncSheet(): void {
		this.syncError = '';
		this.isSyncSheetOpen = true;
	}
}
