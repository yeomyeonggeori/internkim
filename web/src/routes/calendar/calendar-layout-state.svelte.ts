import type { CalendarSyncResponse } from './calendar-layout-types';

export class CalendarLayoutState {
	syncInformation = $state<CalendarSyncResponse | null>(null);
	isSyncSheetOpen = $state(false);
	isRotatingSync = $state(false);
	syncError = $state('');
	openSyncSheet(): void {
		this.syncError = '';
		this.isSyncSheetOpen = true;
	}
}
