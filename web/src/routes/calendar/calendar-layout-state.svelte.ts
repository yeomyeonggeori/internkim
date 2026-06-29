import type {
	CalendarAccountStatusResponse,
	CalendarSource,
	CalendarSyncResponse
} from './calendar-layout-types';
import { calendarDateKey } from './calendar-layout-date';
import type { CalendarViewValue } from './calendar-navigation-message';

export class CalendarLayoutState {
	syncInformation = $state<CalendarSyncResponse | null>(null);
	accountStatus = $state<CalendarAccountStatusResponse | null>(null);
	accountStatusError = $state(false);
	isLoadingAccountStatus = $state(false);
	isSyncSheetOpen = $state(false);
	isRotatingSync = $state(false);
	isUploadingGoogleOAuthClient = $state(false);
	syncError = $state('');
	miniMonthEventDates = $state<Set<string>>(new Set());
	miniMonthEventCount = $state(0);
	selectedMiniDateKey = $state('');
	miniMonthCalendarView = $state<CalendarViewValue>('month');
	today = new Date();
	miniMonth = $state(new Date(this.today.getFullYear(), this.today.getMonth(), 1));

	setMiniMonth(month: Date): void {
		this.miniMonth = new Date(month.getFullYear(), month.getMonth(), 1);
	}

	applyVisibleDate(date: Date): void {
		this.miniMonth = new Date(date.getFullYear(), date.getMonth(), 1);
		this.selectedMiniDateKey = calendarDateKey(date);
	}

	selectMiniMonthDate(date: Date): void {
		this.setMiniMonth(date);
		this.selectedMiniDateKey = calendarDateKey(date);
	}

	openSyncSheet(): void {
		this.syncError = '';
		this.isSyncSheetOpen = true;
	}

	resetMiniMonthEvents(): void {
		this.miniMonthEventDates = new Set();
		this.miniMonthEventCount = 0;
	}

	setMiniMonthEvents(eventDates: Set<string>, eventCount: number): void {
		this.miniMonthEventDates = eventDates;
		this.miniMonthEventCount = eventCount;
	}

	calendarSources(workLabel: string, isWorkVisible: boolean): CalendarSource[] {
		return [
			{
				id: 'work',
				label: workLabel,
				count: String(this.miniMonthEventCount),
				isChecked: isWorkVisible,
				colorClass: 'border-[#4a8dde] bg-[#4a8dde] text-white'
			}
		];
	}
}
