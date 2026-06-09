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
		calendarColors,
		createCalendarLocale,
		createCalendarViews,
		darkCalendarColors
	} from './calendar-config';
	import { createCalendarConflictActions } from './calendar-conflict-actions';
	import CalendarConflictBanner from './calendar-conflict-banner.svelte';
	import type { CalendarConflict } from './calendar-conflicts';
	import { CalendarDraftEventState } from './calendar-draft-events';
	import {
		createCalendarEventActions,
		type CalendarEventActions
	} from './calendar-event-actions';
	import {
		createCalendarEventLoader,
		type CalendarEventLoader
	} from './calendar-event-loader';
	import {
		focusCalendarEventElement,
		openCalendarEventDetailPanel
	} from './calendar-event-elements';
	import {
		installCalendarMonthRangeAction,
		monthRangePreviewSegmentsFromSelection as buildMonthRangePreviewSegments,
		type MonthRangePreviewSegment,
		type MonthRangeSelection
	} from './calendar-month-range-action';
	import {
		refreshSelectedMonthDateCell as refreshSelectedMonthDateCellElement
	} from './calendar-month-selection';
	import { CalendarProgrammaticUpdateState } from './calendar-programmatic-updates';
	import { syncRemoteCalendarAndRefreshConflicts } from './calendar-remote-sync';
	import { searchCalendarEvents, type CalendarSearchResult } from './calendar-search';
	import CalendarStage from './calendar-stage.svelte';
	import { loadSavedCalendarDate, saveCalendarDate } from './calendar-storage';
	import {
		installCalendarTimelineRangeAction,
		type TimelineRangeSelection
	} from './calendar-timeline-range-action';
	import CalendarToolbar from './calendar-toolbar.svelte';
	import {
		endOfMonthWindow,
		shiftedCalendarToolbarDate,
		startOfMonthWindow
	} from './calendar-visible-range';
	import { installCalendarWheelNavigation } from './calendar-wheel-navigation';
	import {
		isCalendarVisibilityMessage,
		loadSavedWorkCalendarVisibility
	} from './calendar-work-visibility';

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
	let isLoading = $state(false);
	let isSaving = $state(false);
	let errorMessage = $state('');
	let statusMessage = $state('');
	let eventCount = $state(0);
	let visibleEvents = $state<DayFlowEvent[]>([]);
	let calendarWorkVisible = $state(true);
	let calendarStageElement = $state<HTMLElement | null>(null);
	let monthRangeSelection = $state<MonthRangeSelection | null>(null);
	let monthRangePreviewSegments = $state<MonthRangePreviewSegment[]>([]);
	let timelineRangeSelection = $state<TimelineRangeSelection | null>(null);
	let selectedMonthDateKey = $state<string | null>(null);
	let searchText = $state('');
	let toolbarDate = $state(initialCalendarDate());
	let toolbarView = $state(ViewType.MONTH);
	let selectedAuditEventID = $state<string | null>(null);
	let pendingEventID = $state(initialCalendarEventID());
	const draftEvents = new CalendarDraftEventState(draftEventPlaceholderTitle, () => text.newEvent);
	const programmaticUpdates = new CalendarProgrammaticUpdateState();

	let calendarConflicts = $state<CalendarConflict[]>([]);
	const conflictActions = createCalendarConflictActions({
		isBrowser: () => browser,
		errorFallback: () => text.error,
		getCalendarConflicts: () => calendarConflicts,
		setCalendarConflicts: (conflicts) => {
			calendarConflicts = conflicts;
		},
		setErrorMessage: (message) => {
			errorMessage = message;
		},
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
		setEventCount: (nextEventCount) => {
			eventCount = nextEventCount;
		},
		setIsLoading: (nextIsLoading) => {
			isLoading = nextIsLoading;
		},
		setErrorMessage: (message) => {
			errorMessage = message;
		},
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
			updateCalendarEvent: async (eventID, changes, shouldRender) => {
				await calendar.updateEvent(eventID, changes, shouldRender);
			},
			setEventCount: (nextEventCount) => {
				eventCount = nextEventCount;
			},
			setVisibleEvents: (events) => {
				visibleEvents = events;
			},
			setIsSaving: (nextIsSaving) => {
				isSaving = nextIsSaving;
			},
			setStatusMessage: (message) => {
				statusMessage = message;
			},
			setErrorMessage: (message) => {
				errorMessage = message;
			},
			openEventDetails: (eventID) => openEventDetails(eventID),
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
		defaultView: ViewType.MONTH,
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
				toolbarDate = middle;
				saveCalendarDate(browser, middle);
			},
			onEventClick: (event) => openEventDetails(event.id),
			onEventCreate: (event) => eventActions.saveCreatedEvent(event),
			onEventUpdate: (event) => eventActions.saveUpdatedEvent(event),
			onEventDelete: (eventID) => eventActions.deleteEvent(eventID)
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
		calendarWorkVisible = loadSavedWorkCalendarVisibility(window.localStorage);
		calendar.changeView(ViewType.MONTH);
		toolbarView = ViewType.MONTH;
		syncCalendarThemeToDocument();
		if (!eventLoader.hasVisibleRange()) {
			const initialDate = initialCalendarDate();
			eventLoader.loadEvents(startOfMonthWindow(initialDate), endOfMonthWindow(initialDate));
		}
		void syncRemoteCalendarAndRefresh().catch((error: unknown) => {
			console.debug('calendar remote sync failed', { error });
		});
		const themeObserver = new MutationObserver(syncCalendarThemeToDocument);
		themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
		const draftTitleObserver = new MutationObserver(() => {
			eventActions.scheduleDraftTitleInputPlaceholderUpdates();
			eventActions.scheduleDraftEventVisibilitySync();
		});
		draftTitleObserver.observe(document.body, { childList: true, subtree: true });
		const visibilityChannel = new BroadcastChannel('internkim-calendar');
		visibilityChannel.addEventListener('message', handleCalendarVisibilityMessage);
		const stopMonthRangeCreate = calendarStageElement
			? installCalendarMonthRangeAction({
					stageElement: calendarStageElement,
					currentView: () => calendar.currentView,
					getSelection: () => monthRangeSelection,
					setSelection: (selection) => {
						monthRangeSelection = selection;
					},
					clearPreview: () => {
						monthRangePreviewSegments = [];
					},
					selectDate: selectMonthDate,
					createSingleDayEvent: eventActions.createMonthSingleDayEvent,
					createRangeEvent: eventActions.createMonthRangeEvent
				})
			: undefined;
		const stopTimelineRangeCreate = calendarStageElement
			? installCalendarTimelineRangeAction({
					stageElement: calendarStageElement,
					currentView: () => calendar.currentView,
					currentDate: () => toolbarDate,
					getSelection: () => timelineRangeSelection,
					setSelection: (selection) => {
						timelineRangeSelection = selection;
					},
					createSingleEvent: eventActions.createTimelineSingleEvent,
					createRangeEvent: eventActions.createTimelineRangeEvent
				})
			: undefined;
		const stopWheelNavigation = calendarStageElement
			? installCalendarWheelNavigation({
					stageElement: calendarStageElement,
					currentView: () => calendar.currentView,
					goToPrevious: () => calendar.app.goToPrevious(),
					goToNext: () => calendar.app.goToNext()
				})
			: undefined;
		return () => {
			visibilityChannel.removeEventListener('message', handleCalendarVisibilityMessage);
			visibilityChannel.close();
			themeObserver.disconnect();
			draftTitleObserver.disconnect();
			stopMonthRangeCreate?.();
			stopTimelineRangeCreate?.();
			stopWheelNavigation?.();
		};
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
	}

	function goToToday() {
		toolbarDate = new Date();
		calendar.goToToday();
	}

	function goToPrevious() {
		toolbarDate = shiftedCalendarToolbarDate(toolbarDate, toolbarView, -1);
		calendar.goToPrevious();
	}

	function goToNext() {
		toolbarDate = shiftedCalendarToolbarDate(toolbarDate, toolbarView, 1);
		calendar.goToNext();
	}

	function syncCalendarThemeToDocument() {
		const mode = document.documentElement.classList.contains('dark') ? 'dark' : 'light';
		calendar.app.updateConfig({ theme: { mode } });
	}

	function handleCalendarVisibilityMessage(event: MessageEvent<unknown>) {
		if (!isCalendarVisibilityMessage(event.data)) return;
		setWorkCalendarVisibility(event.data.work);
	}

	function setWorkCalendarVisibility(isVisible: boolean) {
		calendarWorkVisible = isVisible;
		renderCalendarEvents();
	}

	function renderCalendarEvents() {
		eventLoader.renderVisibleEvents(visibleEvents);
	}

	function navigateToSearchResult(result: CalendarSearchResult) {
		toolbarDate = result.startDate;
		calendar.app.setCurrentDate(result.startDate);
		calendar.app.selectDate(result.startDate);
		saveCalendarDate(browser, result.startDate);
	}

	$effect(() => {
		monthRangeSelection;
		monthRangePreviewSegments = buildMonthRangePreviewSegments(calendarStageElement, monthRangeSelection);
	});

	$effect(() => {
		selectedMonthDateKey;
		calendarStageElement;
		refreshSelectedMonthDateCellAfterRender();
	});

	function compareCalendarEventsForDisplay(leftEvent: DayFlowEvent, rightEvent: DayFlowEvent) {
		const leftPriority = leftEvent.allDay ? 0 : 1;
		const rightPriority = rightEvent.allDay ? 0 : 1;
		if (leftPriority !== rightPriority) return leftPriority - rightPriority;
		return leftEvent.title.localeCompare(rightEvent.title);
	}

	async function refreshCalendar() {
		await eventLoader.refreshCurrentRange();
	}

	async function syncRemoteCalendarAndRefresh() {
		await syncRemoteCalendarAndRefreshConflicts(text.error, refreshCalendar, loadCalendarConflicts);
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
		selectedAuditEventID = eventID;
		calendar.app.selectEvent(eventID);
		openCalendarEventDetailPanel(calendarStageElement, eventID);
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
		requestAnimationFrame(() => refreshSelectedMonthDateCell());
	}

	function refreshSelectedMonthDateCell() {
		refreshSelectedMonthDateCellElement(calendarStageElement, selectedMonthDateKey);
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
		createQuickEvent={eventActions.createQuickEvent}
	/>
	<CalendarStage
		{calendar}
		bind:stageElement={calendarStageElement}
		{monthRangePreviewSegments}
		monthRangePreviewTitle={draftEventPlaceholderTitle()}
		auditRows={selectedAuditRows}
		auditLabel={text.eventAudit}
	/>
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
