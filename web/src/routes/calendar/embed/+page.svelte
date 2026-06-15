<script lang="ts">
	import { browser } from '$app/environment';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { createEventsPlugin, useCalendarApp, ViewType } from '@dayflow/svelte';
	import type { Event as DayFlowEvent } from '@dayflow/core';
	import { createDragPlugin } from '@dayflow/plugin-drag';
	import { onMount } from 'svelte';
	import '@dayflow/core/dist/styles.css';
	import { calendarText } from '../text';
	import { calendarAuditRows } from './calendar-audit';
	import {
		calendarViewMessageValue,
		calendarViewType,
		compareCalendarEventsForDisplay,
		formatMonthScrollOverlayText,
		isDateInVisibleRange,
		miniCalendarWeekdayLabels,
		normalizedVisibleDate
	} from './calendar-embed-view-helpers';
	import {
		calendarColors,
		createCalendarLocale,
		createCalendarViews,
		darkCalendarColors
	} from './calendar-config';
	import { createCalendarConflictActions } from './calendar-conflict-actions';
	import CalendarConflictBanner from './calendar-conflict-banner.svelte';
	import type { CalendarConflict } from './calendar-conflicts';
	import CalendarDraftPopover from './calendar-draft-popover.svelte';
	import {
		scheduleCalendarAllDayLayoutSync,
		scheduleDayFlowMiniCalendarEnhancement
	} from './calendar-embed-dom-sync';
	import {
		createCalendarDraftPopoverActions,
		type CalendarDraftPopoverActions
	} from './calendar-draft-popover-actions';
	import {
		dateKey,
		type DraftPopoverState
	} from './calendar-draft-popover-state';
	import { CalendarDraftEventState } from './calendar-draft-events';
	import {
		createCalendarEventActions,
		type CalendarEventActions
	} from './calendar-event-actions';
	import {
		createCalendarEventLoader,
		type CalendarEventLoader
	} from './calendar-event-loader';
	import { focusCalendarEventElement } from './calendar-event-elements';
	import {
		monthRangePreviewSegmentsFromSelection as buildMonthRangePreviewSegments,
		type MonthRangePreviewSegment,
		type MonthRangeSelection
	} from './calendar-month-range-action';
	import {
		dateFromDateKey,
		refreshSelectedMonthDateCell as refreshSelectedMonthDateCellElement
	} from './calendar-month-selection';
	import { CalendarProgrammaticUpdateState } from './calendar-programmatic-updates';
	import { syncRemoteCalendarAndRefreshConflicts } from './calendar-remote-sync';
	import { searchCalendarEvents, type CalendarSearchResult } from './calendar-search';
	import CalendarStage from './calendar-stage.svelte';
	import { installCalendarEmbedLifecycle } from './calendar-embed-lifecycle';
	import {
		loadSavedCalendarDate,
		loadSavedCalendarView
	} from './calendar-storage';
	import type { TimelineRangeSelection } from './calendar-timeline-range-action';
	import {
		clearTimelineOverlapLayout,
		syncTimelinePreviewOverlapLayout,
		timelineRangePreviewSegments as buildTimelineRangePreviewSegments,
		type TimelineRangePreviewSegment
	} from './calendar-timeline-preview';
	import CalendarToolbar from './calendar-toolbar.svelte';
	import { shiftedCalendarToolbarDate } from './calendar-visible-range';
	import type { CalendarMonthScrollLabel } from './calendar-wheel-navigation';
	import { isCalendarVisibilityMessage } from './calendar-work-visibility';
	import { isCalendarNavigationMessage } from '../calendar-navigation-message';
	import {
		broadcastCalendarEventsChanged,
		broadcastCalendarView,
		broadcastCalendarVisibleDate
	} from '../refresh-signal.svelte';
	import { calendarDateStorageKey } from '../calendar-storage-keys';

	type VisibleMonthScrollLabel = CalendarMonthScrollLabel & {
		text: string;
		isFading: boolean;
	};

	const text = createPageText(calendarText);
	const localeCode = $derived(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US');
	const calendarLocale = $derived(createCalendarLocale(currentLocale.value, text));
	const draftEventPlaceholderTitle = () => text.newEvent;
	const initialCalendarDate = () => {
		if (!browser) return loadSavedCalendarDate(browser);
		const dateValue = new URLSearchParams(window.location.search).get('date') ?? '';
		if (!dateValue) return loadSavedCalendarDate(browser);
		const parsedDate = new Date(`${dateValue}T00:00:00`);
		if (Number.isNaN(parsedDate.getTime())) return loadSavedCalendarDate(browser);
		return parsedDate;
	};
	const initialCalendarEventID = () => {
		if (!browser) return '';
		return new URLSearchParams(window.location.search).get('event') ?? '';
	};
	const initialCalendarView = () => calendarViewType(loadSavedCalendarView(browser));
	let isSaving = $state(false);
	let visibleEvents = $state<DayFlowEvent[]>([]);
	let calendarWorkVisible = $state(true);
	let calendarStageElement = $state<HTMLElement | null>(null);
	let monthRangeSelection = $state<MonthRangeSelection | null>(null);
	let monthRangePreviewSegments = $state<MonthRangePreviewSegment[]>([]);
	let timelineRangeSelection = $state<TimelineRangeSelection | null>(null);
	let timelineRangePreviewSegments = $state<TimelineRangePreviewSegment[]>([]);
	let monthScrollOverlayLabels = $state<VisibleMonthScrollLabel[]>([]);
	let selectedMonthDateKey = $state<string | null>(null);
	let searchText = $state('');
	let toolbarDate = $state(initialCalendarDate());
	let toolbarView = $state(initialCalendarView());
	let selectedAuditEventID = $state<string | null>(null);
	let pendingEventID = $state(initialCalendarEventID());
	let draftPopover = $state<DraftPopoverState | null>(null);
	let monthScrollOverlayTimer: number | null = null;
	const draftEvents = new CalendarDraftEventState(draftEventPlaceholderTitle, () => text.newEvent);
	const programmaticUpdates = new CalendarProgrammaticUpdateState();
	const calendarOptions = $derived([{ id: 'internkim', name: text.work }]);

	let calendarConflicts = $state<CalendarConflict[]>([]);
	const conflictActions = createCalendarConflictActions({
		isBrowser: () => browser,
		errorFallback: () => text.error,
		getCalendarConflicts: () => calendarConflicts,
		setCalendarConflicts: (conflicts) => {
			calendarConflicts = conflicts;
		},
		setErrorMessage: () => {},
		syncRemoteCalendarAndRefresh: async () => {
			await syncRemoteCalendarAndRefresh();
		}
	});

	const eventLoader: CalendarEventLoader = createCalendarEventLoader({
		isBrowser: () => browser,
		errorFallback: () => text.error,
		isWorkCalendarVisible: () => calendarWorkVisible,
		getCalendarEvents: () => calendar.app.getAllEvents(),
		applyCalendarEventsChanges: (changes) => {
			calendar.app.applyEventsChanges(changes);
		},
		triggerCalendarRender: () => {
			calendar.app.triggerRender();
		},
		setVisibleEvents: (events) => {
			visibleEvents = events;
		},
		setEventCount: () => {},
		setIsLoading: () => {},
		setErrorMessage: () => {},
		refreshSelectedMonthDateCell: () => {
			refreshSelectedMonthDateCellAfterRender();
		},
		afterRenderEvents: (events) => {
			openPendingCalendarEvent(events);
		}
	});

	const eventActions: CalendarEventActions = createCalendarEventActions(
		{
			isBrowser: () => browser,
			getCurrentDate: () => calendar.currentDate,
			getStageElement: () => calendarStageElement,
			getSelectedAuditEventID: () => selectedAuditEventID,
			setSelectedAuditEventID: (eventID) => {
				selectedAuditEventID = eventID;
			},
			getCalendarEvents: () => calendar.app.getAllEvents(),
			addCalendarEvent: (event) => calendar.addEvent(event),
			removeCalendarEvent: (eventID) => {
				calendar.app.applyEventsChanges({ delete: [eventID], add: [] });
			},
			updateCalendarEvent: async (eventID, changes, shouldRender) => {
				await calendar.updateEvent(eventID, changes, shouldRender);
			},
			setEventCount: () => {},
			setVisibleEvents: (events) => {
				visibleEvents = events;
			},
			setIsSaving: (nextIsSaving) => {
				isSaving = nextIsSaving;
			},
			setStatusMessage: () => {},
			setErrorMessage: () => {},
			openEventDetails: (eventID) => openEventDetails(eventID),
			notifyEventsChanged: broadcastCalendarEventsChanged,
			refreshCalendar: async () => {
				await refreshCalendar();
			},
			text: {
				get deleteError() {
					return text.deleteError;
				},
				get draftTitlePlaceholder() {
					return text.newEvent;
				},
				get saveError() {
					return text.saveError;
				},
				get shared() {
					return text.shared;
				}
			}
		},
		draftEvents,
		programmaticUpdates
	);

	const calendar = useCalendarApp({
		views: createCalendarViews(),
		defaultView: initialCalendarView(),
		initialDate: initialCalendarDate(),
		locale: createCalendarLocale(currentLocale.value, text),
		timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
		switcherMode: 'buttons',
		useCalendarHeader: false,
		useEventDetailDialog: false,
		useEventDetailPanel: true,
		calendars: [
			{
				id: 'internkim',
				name: text.work,
				colors: calendarColors,
				darkColors: darkCalendarColors,
				isVisible: true
			}
		],
		defaultCalendar: 'internkim',
		theme: { mode: 'light' },
		allDaySortComparator: compareCalendarEventsForDisplay,
		plugins: [
			createEventsPlugin(),
			createDragPlugin({
				enableDrag: true,
				enableResize: true,
				enableCreate: false,
				enableAllDayCreate: true,
				onEventDrop: (updatedEvent) => eventActions.saveUpdatedEvent(updatedEvent),
				onEventResize: (updatedEvent) => eventActions.saveUpdatedEvent(updatedEvent)
			})
		],
		callbacks: {
			onVisibleRangeChange: (startDate, endDate) => {
				eventLoader.loadEvents(startDate, endDate);
				const middle = new Date((startDate.getTime() + endDate.getTime()) / 2);
				const visibleDate = isDateInVisibleRange(toolbarDate, startDate, endDate) ? toolbarDate : middle;
				setVisibleDate(visibleDate);
			},
			onEventClick: (event) => draftPopoverActions.openEventDraftPopover(event, 'edit'),
			onEventCreate: (event) => eventActions.saveCreatedEvent(event),
			onEventUpdate: (event) => eventActions.saveUpdatedEvent(event),
			onEventDelete: (eventID) => eventActions.deleteEvent(eventID)
		}
	});

	const draftPopoverActions: CalendarDraftPopoverActions = createCalendarDraftPopoverActions({
		eventActions,
		getDraftPopover: () => draftPopover,
		setDraftPopover: (popover) => {
			draftPopover = popover;
		},
		getCalendarEvents: () => calendar.events,
		getStageElement: () => calendarStageElement,
		selectEvent: (eventID) => {
			selectedAuditEventID = eventID;
			calendar.app.selectEvent(eventID);
		},
		replaceLocalEvent: (event) => {
			calendar.app.applyEventsChanges({ delete: [event.id], add: [event] });
		}
	});

	const selectedAuditEvent = $derived(
		selectedAuditEventID ? (calendar.events.find((event) => event.id === selectedAuditEventID) ?? null) : null
	);
	const selectedAuditRows = $derived(calendarAuditRows(selectedAuditEvent));

	$effect(() => {
		calendar.app.updateConfig({ locale: calendarLocale });
		calendar.app.triggerRender();
	});

	onMount(() => {
		return installCalendarEmbedLifecycle({
			stageElement: calendarStageElement,
			setWorkCalendarVisible: (isVisible) => {
				calendarWorkVisible = isVisible;
			},
			handleCalendarChannelMessage,
			handleCalendarWindowMessage,
			handleCalendarStorageMessage,
			initialCalendarView,
			applyCalendarView: (view) => {
				calendar.changeView(view);
			},
			setToolbarView: (view) => {
				toolbarView = view;
			},
			syncCalendarThemeToDocument,
			hasVisibleRange: () => eventLoader.hasVisibleRange(),
			initialCalendarDate,
			loadEvents: (startDate, endDate) => eventLoader.loadEvents(startDate, endDate),
			syncRemoteCalendarAndRefresh,
			scheduleDraftTitleInputPlaceholderUpdates: eventActions.scheduleDraftTitleInputPlaceholderUpdates,
			scheduleDraftEventVisibilitySync: eventActions.scheduleDraftEventVisibilitySync,
			refreshSelectedMonthDateCellAfterRender,
			clearMonthScrollOverlays,
			monthRangeAction: {
				currentView: () => calendar.currentView,
				getSelection: () => monthRangeSelection,
				setSelection: (selection) => {
					monthRangeSelection = selection;
				},
				clearPreview: () => {
					monthRangePreviewSegments = [];
				},
				selectDate: selectMonthDate,
				createSingleDayEvent: draftPopoverActions.openMonthSingleDayDraftPopover,
				createRangeEvent: draftPopoverActions.openMonthRangeDraftPopover
			},
			timelineRangeAction: {
				currentView: () => calendar.currentView,
				currentDate: () => toolbarDate,
				getSelection: () => timelineRangeSelection,
				setSelection: setTimelineRangeSelection,
				createSingleEvent: draftPopoverActions.openTimelineSingleDraftPopover,
				createRangeEvent: draftPopoverActions.openTimelineRangeDraftPopover
			},
			wheelNavigation: {
				currentView: () => calendar.currentView,
				showMonthLabels: showMonthScrollOverlays,
				hideMonthLabels: clearMonthScrollOverlays,
				selectVisibleDate: setVisibleDate
			},
			draftPopoverDismiss: {
				getDraftPopover: () => draftPopover,
				saveDraftPopover: draftPopoverActions.saveDraftPopover,
				cancelDraftPopover: draftPopoverActions.cancelDraftPopover
			},
			keyboardDelete: {
				getSelectedEventID: () => selectedAuditEventID,
				getDraftPopover: () => draftPopover,
				deleteSelectedEvent: (eventID) => {
					selectedAuditEventID = null;
					draftPopover = null;
					void eventActions.deleteEvent(eventID);
				}
			},
			miniCalendarMonthPicker: {
				getStageElement: () => calendarStageElement,
				getCurrentDate: () => toolbarDate,
				localeCode: () => (currentLocale.value === 'ko' ? 'ko-KR' : 'en-US'),
				labels: () => ({
					pickMonthAndYear: text.pickMonthAndYear,
					previousYear: text.previousYear,
					nextYear: text.nextYear,
					previousTwelveYears: text.previousTwelveYears,
					nextTwelveYears: text.nextTwelveYears
				}),
				selectDate: (date) => {
					navigateToDateKey(dateKey(date));
				}
			},
			navigateToDateKey
		});
	});

	const currentMonthTitle = $derived(
		toolbarDate.toLocaleDateString(localeCode, {
			year: 'numeric',
			month: 'long'
		})
	);
	const searchResults = $derived(searchCalendarEvents(searchText, visibleEvents));

	function changeCalendarView(viewType: ViewType) {
		toolbarView = viewType;
		calendar.changeView(viewType);
		const view = calendarViewMessageValue(viewType);
		broadcastCalendarView(view);
	}

	function goToToday() {
		setVisibleDate(new Date());
		calendar.goToToday();
	}

	function goToPrevious() {
		setVisibleDate(shiftedCalendarToolbarDate(toolbarDate, toolbarView, -1));
		calendar.goToPrevious();
	}

	function goToNext() {
		setVisibleDate(shiftedCalendarToolbarDate(toolbarDate, toolbarView, 1));
		calendar.goToNext();
	}

	function showMonthScrollOverlays(labels: CalendarMonthScrollLabel[]) {
		monthScrollOverlayLabels = labels.map((label) => ({
			...label,
			text: formatMonthScrollOverlayText(label.date, localeCode),
			isFading: false
		}));
		if (monthScrollOverlayTimer) window.clearTimeout(monthScrollOverlayTimer);
		monthScrollOverlayTimer = null;
	}

	function clearMonthScrollOverlays() {
		if (monthScrollOverlayTimer) window.clearTimeout(monthScrollOverlayTimer);
		if (monthScrollOverlayLabels.length === 0) {
			monthScrollOverlayTimer = null;
			return;
		}
		monthScrollOverlayLabels = monthScrollOverlayLabels.map((label) => ({ ...label, isFading: true }));
		monthScrollOverlayTimer = window.setTimeout(() => {
			monthScrollOverlayLabels = [];
			monthScrollOverlayTimer = null;
		}, 180);
	}

	function syncCalendarThemeToDocument() {
		const mode = document.documentElement.classList.contains('dark') ? 'dark' : 'light';
		calendar.app.updateConfig({ theme: { mode } });
	}

	function handleCalendarChannelMessage(event: MessageEvent<unknown>) {
		if (isCalendarVisibilityMessage(event.data)) {
			setWorkCalendarVisibility(event.data.work);
			return;
		}
		if (isCalendarNavigationMessage(event.data)) {
			navigateToDateKey(event.data.dateKey);
		}
	}

	function handleCalendarWindowMessage(event: MessageEvent<unknown>) {
		if (event.origin !== window.location.origin) return;
		if (!isCalendarNavigationMessage(event.data)) return;
		navigateToDateKey(event.data.dateKey);
	}

	function handleCalendarStorageMessage(event: StorageEvent) {
		if (event.key !== calendarDateStorageKey || !event.newValue) return;
		const date = new Date(event.newValue);
		if (Number.isNaN(date.getTime())) return;
		navigateToDateKey(dateKey(date));
	}

	function setWorkCalendarVisibility(isVisible: boolean) {
		calendarWorkVisible = isVisible;
		renderCalendarEvents();
	}

	function renderCalendarEvents() {
		eventLoader.renderVisibleEvents(visibleEvents);
	}

	function navigateToDateKey(dateKey: string) {
		const date = dateFromDateKey(dateKey);
		const navigationDate = new Date(date.getFullYear(), date.getMonth(), date.getDate(), 12, 0, 0, 0);
		selectedMonthDateKey = dateKey;
		setVisibleDate(navigationDate);
		calendar.app.setCurrentDate(navigationDate);
		calendar.app.setVisibleMonth(navigationDate);
		calendar.app.selectDate(navigationDate);
		refreshSelectedMonthDateCellAfterRender();
	}

	function navigateToSearchResult(result: CalendarSearchResult) {
		setVisibleDate(result.startDate);
		calendar.app.setCurrentDate(result.startDate);
		calendar.app.setVisibleMonth(result.startDate);
		calendar.app.selectDate(result.startDate);
	}

	$effect(() => {
		monthRangeSelection;
		monthRangePreviewSegments = buildMonthRangePreviewSegments(calendarStageElement, monthRangeSelection);
	});

	$effect(() => {
		calendarStageElement;
		timelineRangeSelection;
		toolbarDate;
		toolbarView;
		visibleEvents;
		refreshTimelineRangePreview();
	});

	$effect(() => {
		calendarStageElement;
		toolbarDate;
		toolbarView;
		visibleEvents;
		if (!browser) return;
		scheduleDayFlowMiniCalendarEnhancement({
			stageElement: calendarStageElement,
			currentDate: toolbarDate,
			events: visibleEvents,
			localeCode,
			weekdayLabels: miniCalendarWeekdayLabels(currentLocale.value),
			previousLabel: text.previous,
			nextLabel: text.next,
			selectDateKey: navigateToDateKey
		});
	});

	$effect(() => {
		calendarStageElement;
		toolbarView;
		visibleEvents;
		if (!browser) return;
		scheduleCalendarAllDayLayoutSync(calendarStageElement, toolbarView);
	});

	$effect(() => {
		selectedMonthDateKey;
		calendarStageElement;
		refreshSelectedMonthDateCellAfterRender();
	});

	async function refreshCalendar() {
		await eventLoader.refreshCurrentRange();
	}

	async function syncRemoteCalendarAndRefresh() {
		await syncRemoteCalendarAndRefreshConflicts(text.error, refreshCalendar, loadCalendarConflicts);
		broadcastCalendarEventsChanged();
	}

	async function loadCalendarConflicts() {
		await conflictActions.loadCalendarConflicts();
	}

	async function dismissCalendarConflict(conflictID: number) {
		await conflictActions.dismissCalendarConflict(conflictID);
	}

	async function dismissAllConflictsAndRefresh() {
		await conflictActions.dismissAllConflictsAndRefresh();
	}

	function openEventDetails(eventID: string) {
		const event = calendar.events.find((calendarEvent) => calendarEvent.id === eventID);
		if (!event) return;
		draftPopoverActions.openEventDraftPopover(event, 'edit');
	}

	function openPendingCalendarEvent(events: DayFlowEvent[]) {
		if (!pendingEventID) return;
		if (!events.some((event) => event.id === pendingEventID)) return;
		const eventID = pendingEventID;
		pendingEventID = '';
		selectedAuditEventID = eventID;
		calendar.app.selectEvent(eventID);
		requestAnimationFrame(() => focusCalendarEventElement(calendarStageElement, eventID));
	}

	function selectMonthDate(dateKey: string) {
		selectedMonthDateKey = dateKey;
		refreshSelectedMonthDateCellAfterRender();
	}

	function refreshSelectedMonthDateCellAfterRender() {
		if (!browser) return;
		requestAnimationFrame(() => {
			refreshSelectedMonthDateCell();
			requestAnimationFrame(() => refreshSelectedMonthDateCell());
		});
		window.setTimeout(() => refreshSelectedMonthDateCell(), 0);
	}

	function refreshSelectedMonthDateCell() {
		refreshSelectedMonthDateCellElement(calendarStageElement, selectedMonthDateKey);
	}

	function setTimelineRangeSelection(selection: TimelineRangeSelection | null) {
		timelineRangeSelection = selection;
		refreshTimelineRangePreview();
	}

	function refreshTimelineRangePreview() {
		updateTimelineRangePreviewSegments();
		scheduleTimelinePreviewOverlapLayoutSync();
	}

	function updateTimelineRangePreviewSegments() {
		timelineRangePreviewSegments = buildTimelineRangePreviewSegments({
			stageElement: calendarStageElement,
			selection: timelineRangeSelection,
			currentView: toolbarView,
			currentDate: toolbarDate,
			events: visibleEvents
		});
	}

	function scheduleTimelinePreviewOverlapLayoutSync() {
		if (!browser) return;
		syncTimelinePreviewOverlapLayout(calendarStageElement, timelineRangeSelection, visibleEvents);
		requestAnimationFrame(() => {
			updateTimelineRangePreviewSegments();
			syncTimelinePreviewOverlapLayout(calendarStageElement, timelineRangeSelection, visibleEvents);
		});
		if (!timelineRangeSelection) requestAnimationFrame(() => clearTimelineOverlapLayout(calendarStageElement));
	}

	function setVisibleDate(date: Date) {
		const visibleDate = normalizedVisibleDate(date);
		toolbarDate = visibleDate;
		broadcastCalendarVisibleDate(visibleDate);
	}
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main class="calendar-page flex min-h-screen flex-col">
	<CalendarConflictBanner
		conflicts={calendarConflicts}
		dismissConflict={dismissCalendarConflict}
		refreshConflicts={dismissAllConflictsAndRefresh}
	/>
	<CalendarToolbar
		{currentMonthTitle}
		bind:searchText
		{searchResults}
		{toolbarView}
		{changeCalendarView}
		{goToToday}
		{goToPrevious}
		{goToNext}
		{navigateToSearchResult}
		createQuickEvent={draftPopoverActions.createQuickDraftPopover}
	/>
	<CalendarStage
		{calendar}
		{toolbarView}
		bind:stageElement={calendarStageElement}
		{monthRangePreviewSegments}
		monthRangePreviewTitle={draftEventPlaceholderTitle()}
		{timelineRangePreviewSegments}
		timelineRangePreviewTitle={draftEventPlaceholderTitle()}
		{monthScrollOverlayLabels}
		auditRows={selectedAuditRows}
		auditLabel={text.eventAudit}
	/>
	{#if draftPopover}
		<CalendarDraftPopover
			popover={draftPopover}
			{calendarOptions}
			isSaving={isSaving}
			text={{
				title: text.conflictField.title,
				allDay: text.allDay,
				location: text.conflictField.location,
				description: text.conflictField.description,
				calendar: text.draftPopover.calendar,
				cancel: text.draftPopover.cancel,
				complete: text.draftPopover.complete,
				delete: text.draftPopover.delete,
				startDate: text.draftPopover.startDate,
				endDate: text.draftPopover.endDate,
				startTime: text.draftPopover.startTime,
				endTime: text.draftPopover.endTime
			}}
			updatePopover={draftPopoverActions.updateDraftPopover}
			savePopover={draftPopoverActions.saveDraftPopover}
			cancelPopover={draftPopoverActions.cancelDraftPopover}
			deletePopover={draftPopoverActions.deleteDraftPopover}
		/>
	{/if}
</main>

<style>
	.calendar-page {
		background: #ffffff;
		color: #18181b;
	}

	:global(html.dark) .calendar-page {
		background: #09090b;
		color: #f4f4f5;
	}
</style>
