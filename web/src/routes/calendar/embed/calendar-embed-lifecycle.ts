import type { ViewType } from '../calendar-view-type';
import { installCalendarDraftPopoverDismiss } from './calendar-draft-popover-dismiss';
import type { CalendarDraftPopoverDismissOptions } from './calendar-draft-popover-dismiss';
import { installCalendarKeyboardDelete } from './calendar-keyboard-delete';
import type { CalendarKeyboardDeleteContext } from './calendar-keyboard-delete';
import { installCalendarKeyboardSave } from './calendar-keyboard-save';
import type { CalendarKeyboardSaveContext } from './calendar-keyboard-save';
import { installCalendarKeyboardUndo } from './calendar-keyboard-undo';
import type { CalendarKeyboardUndoContext } from './calendar-keyboard-undo';
import { installCalendarMonthKeyboardNavigation } from './calendar-month-selection';
import type { CalendarMonthKeyboardNavigationOptions } from './calendar-month-selection';
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
	hasVisibleRange: () => boolean;
	initialCalendarDate: () => Date;
	loadEvents: (startDate: Date, endDate: Date) => void;
	syncRemoteCalendarAndRefresh: () => Promise<void>;
	draftPopoverDismiss: Omit<CalendarDraftPopoverDismissOptions, 'stageElement'>;
	monthKeyboardNavigation: CalendarMonthKeyboardNavigationOptions;
	keyboardDelete: CalendarKeyboardDeleteContext;
	keyboardUndo: CalendarKeyboardUndoContext;
	keyboardSave: CalendarKeyboardSaveContext;
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
	loadInitialVisibleRange(options);
	startInitialRemoteCalendarSync(options);
	const stopStageActions = installStageActions(options);
	const stopMonthKeyboardNavigation = installCalendarMonthKeyboardNavigation(options.monthKeyboardNavigation);
	const stopKeyboardDelete = installCalendarKeyboardDelete(options.keyboardDelete);
	const stopKeyboardUndo = installCalendarKeyboardUndo(options.keyboardUndo);
	const stopKeyboardSave = installCalendarKeyboardSave(options.keyboardSave);
	return () => {
		calendarChannel.removeEventListener('message', options.handleCalendarChannelMessage);
		window.removeEventListener('message', options.handleCalendarWindowMessage, { capture: true });
		window.removeEventListener('storage', options.handleCalendarStorageMessage, { capture: true });
		calendarChannel.close();
		stopStageActions();
		stopMonthKeyboardNavigation();
		stopKeyboardDelete();
		stopKeyboardUndo();
		stopKeyboardSave();
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

function installStageActions(options: CalendarEmbedLifecycleOptions): () => void {
	if (!options.stageElement) return () => {};
	const stageElement = options.stageElement;
	return installCalendarDraftPopoverDismiss({ stageElement, ...options.draftPopoverDismiss });
}
