import type {
	CalendarAccountStatusResponse,
	CalendarSyncResponse
} from './calendar-layout-types';
import { calendarDateKey } from './calendar-layout-date';

export class CalendarLayoutState {
	syncInformation = $state<CalendarSyncResponse | null>(null);
	accountStatus = $state<CalendarAccountStatusResponse | null>(null);
	accountStatusError = $state(false);
	isLoadingAccountStatus = $state(false);
	isSyncSheetOpen = $state(false);
	isRotatingSync = $state(false);
	isUploadingGoogleOAuthClient = $state(false);
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
