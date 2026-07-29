import type { ViewType } from '@dayflow/svelte';
import { installCalendarAllDayCellAction } from './calendar-all-day-cell-action';
import type { CalendarAllDayCellActionOptions } from './calendar-all-day-cell-action';
import { clearCalendarAllDayLayout } from './calendar-all-day-layout';
import { installCalendarDraftPopoverDismiss } from './calendar-draft-popover-dismiss';
import type { CalendarDraftPopoverDismissOptions } from './calendar-draft-popover-dismiss';
import { installCalendarEventKeyboardActivation } from './calendar-event-keyboard-activation';
import type { CalendarEventKeyboardActivationOptions } from './calendar-event-keyboard-activation';
import { installCalendarEventAnchorCapture } from './calendar-event-anchor-capture';
import { installCalendarEventSelection } from './calendar-event-selection';
import type { CalendarEventSelectionOptions } from './calendar-event-selection';
import { installCalendarKeyboardDelete } from './calendar-keyboard-delete';
import type { CalendarKeyboardDeleteContext } from './calendar-keyboard-delete';
import { installCalendarKeyboardUndo } from './calendar-keyboard-undo';
import type { CalendarKeyboardUndoContext } from './calendar-keyboard-undo';
import { installCalendarMonthKeyboardNavigation } from './calendar-month-selection';
import type { CalendarMonthKeyboardNavigationOptions } from './calendar-month-selection';
import { installCalendarMonthRangeAction } from './calendar-month-range-action';
import type { MonthRangeActionOptions } from './calendar-month-range-action';
import { installCalendarTimelineRangeAction } from './calendar-timeline-range-action';
import type { TimelineRangeActionOptions } from './calendar-timeline-range-action';
import { installCalendarTimelineScrollState } from './calendar-timeline-scroll-state';
import { installCalendarWheelNavigation } from './calendar-wheel-navigation';
import type { CalendarWheelNavigationOptions } from './calendar-wheel-navigation';
import { installDayFlowMiniCalendarMonthPicker } from './calendar-dayflow-mini-calendar-picker';
import type { DayFlowMiniCalendarPickerContext } from './calendar-dayflow-mini-calendar-picker';
import { installDayFlowMiniCalendarDateSelection } from './calendar-dayflow-mini-calendar-enhancement';
import { startOfMonthWindow, endOfMonthWindow } from './calendar-visible-range';
import { calendarChannelName } from '../refresh-signal.svelte';

export type CalendarEmbedLifecycleOptions = {
	stageElement: HTMLElement | null;
	handleCalendarChannelMessage: (event: MessageEvent<unknown>) => void;
	handleCalendarWindowMessage: (event: MessageEvent<unknown>) => void;
	handleCalendarStorageMessage: (event: StorageEvent) => void;
	initialCalendarView: () => ViewType;
	applyCalendarView: (view: ViewType) => void;
	setToolbarView: (view: ViewType) => void;
	syncCalendarThemeToDocument: () => void;
	hasVisibleRange: () => boolean;
	initialCalendarDate: () => Date;
	loadEvents: (startDate: Date, endDate: Date) => void;
	syncRemoteCalendarAndRefresh: () => Promise<void>;
	scheduleDraftTitleInputPlaceholderUpdates: () => void;
	scheduleDraftEventVisibilitySync: () => void;
	refreshSelectedMonthDateCellAfterRender: () => void;
	monthRangeAction: Omit<MonthRangeActionOptions, 'stageElement'>;
	allDayCellAction: Omit<CalendarAllDayCellActionOptions, 'stageElement'>;
	timelineRangeAction: Omit<TimelineRangeActionOptions, 'stageElement'>;
	wheelNavigation: Omit<CalendarWheelNavigationOptions, 'stageElement'>;
	draftPopoverDismiss: Omit<CalendarDraftPopoverDismissOptions, 'stageElement'>;
	eventKeyboardActivation: Omit<CalendarEventKeyboardActivationOptions, 'stageElement'>;
	eventSelection: Omit<CalendarEventSelectionOptions, 'stageElement'>;
	monthKeyboardNavigation: CalendarMonthKeyboardNavigationOptions;
	keyboardDelete: CalendarKeyboardDeleteContext;
	keyboardUndo: CalendarKeyboardUndoContext;
	miniCalendarMonthPicker: DayFlowMiniCalendarPickerContext;
	navigateToDateKey: (dateKey: string) => void;
};

export function installCalendarEmbedLifecycle(options: CalendarEmbedLifecycleOptions): () => void {
	const calendarChannel = new BroadcastChannel(calendarChannelName);
	calendarChannel.addEventListener('message', options.handleCalendarChannelMessage);
	window.addEventListener('message', options.handleCalendarWindowMessage, { capture: true });
	window.addEventListener('storage', options.handleCalendarStorageMessage, { capture: true });
	const savedView = options.initialCalendarView();
	options.applyCalendarView(savedView);
	options.setToolbarView(savedView);
	options.syncCalendarThemeToDocument();
	loadInitialVisibleRange(options);
	startInitialRemoteCalendarSync(options);
	const themeObserver = observeThemeChanges(options);
	const draftTitleObserver = observeDraftTitleChanges(options);
	const selectedDateObserver = observeSelectedDateChanges(options);
	const stopStageActions = installStageActions(options);
	const stopMonthKeyboardNavigation = installCalendarMonthKeyboardNavigation(options.monthKeyboardNavigation);
	const stopKeyboardDelete = installCalendarKeyboardDelete(options.keyboardDelete);
	const stopKeyboardUndo = installCalendarKeyboardUndo(options.keyboardUndo);
	const stopMiniCalendarDateSelection = installDayFlowMiniCalendarDateSelection(options.navigateToDateKey);
	const stopMiniCalendarMonthPicker = installDayFlowMiniCalendarMonthPicker(options.miniCalendarMonthPicker);
	return () => {
		calendarChannel.removeEventListener('message', options.handleCalendarChannelMessage);
		window.removeEventListener('message', options.handleCalendarWindowMessage, { capture: true });
		window.removeEventListener('storage', options.handleCalendarStorageMessage, { capture: true });
		calendarChannel.close();
		themeObserver.disconnect();
		draftTitleObserver.disconnect();
		selectedDateObserver.disconnect();
		stopStageActions();
		stopMonthKeyboardNavigation();
		stopKeyboardDelete();
		stopKeyboardUndo();
		stopMiniCalendarDateSelection();
		stopMiniCalendarMonthPicker();
		clearCalendarAllDayLayout(options.stageElement);
	};
}

function loadInitialVisibleRange(options: CalendarEmbedLifecycleOptions): void {
	if (options.hasVisibleRange()) return;
	const initialDate = options.initialCalendarDate();
	options.loadEvents(startOfMonthWindow(initialDate), endOfMonthWindow(initialDate));
}

function startInitialRemoteCalendarSync(options: CalendarEmbedLifecycleOptions): void {
	void options.syncRemoteCalendarAndRefresh().catch(() => undefined);
}

function observeThemeChanges(options: CalendarEmbedLifecycleOptions): MutationObserver {
	const observer = new MutationObserver(options.syncCalendarThemeToDocument);
	observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
	return observer;
}

function observeDraftTitleChanges(options: CalendarEmbedLifecycleOptions): MutationObserver {
	const observer = new MutationObserver(() => {
		options.scheduleDraftTitleInputPlaceholderUpdates();
		options.scheduleDraftEventVisibilitySync();
	});
	observer.observe(document.body, { childList: true, subtree: true });
	return observer;
}

function observeSelectedDateChanges(options: CalendarEmbedLifecycleOptions): MutationObserver {
	const observer = new MutationObserver(options.refreshSelectedMonthDateCellAfterRender);
	if (options.stageElement) observer.observe(options.stageElement, { childList: true, subtree: true });
	return observer;
}

function installStageActions(options: CalendarEmbedLifecycleOptions): () => void {
	if (!options.stageElement) return () => {};
	const stageElement = options.stageElement;
	const stopMonthRangeCreate = installCalendarMonthRangeAction({ stageElement, ...options.monthRangeAction });
	const stopAllDayCellCreate = installCalendarAllDayCellAction({ stageElement, ...options.allDayCellAction });
	const stopTimelineRangeCreate = installCalendarTimelineRangeAction({ stageElement, ...options.timelineRangeAction });
	const stopWheelNavigation = installCalendarWheelNavigation({ stageElement, ...options.wheelNavigation });
	const stopTimelineScrollState = installCalendarTimelineScrollState(stageElement);
	const stopDraftPopoverDismiss = installCalendarDraftPopoverDismiss({ stageElement, ...options.draftPopoverDismiss });
	const stopEventKeyboardActivation = installCalendarEventKeyboardActivation({
		stageElement,
		...options.eventKeyboardActivation
	});
	const stopEventSelection = installCalendarEventSelection({ stageElement, ...options.eventSelection });
	const stopEventAnchorCapture = installCalendarEventAnchorCapture(stageElement);
	return () => {
		stopMonthRangeCreate();
		stopAllDayCellCreate();
		stopTimelineRangeCreate();
		stopWheelNavigation();
		stopTimelineScrollState();
		stopDraftPopoverDismiss();
		stopEventKeyboardActivation();
		stopEventSelection();
		stopEventAnchorCapture();
	};
}
