<script lang="ts">
	import { browser } from '$app/environment';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { createEventsPlugin, DayFlowCalendar, useCalendarApp, ViewType } from '@dayflow/svelte';
	import { recalculateEventDays, type Event as DayFlowEvent } from '@dayflow/core';
	import { createDragPlugin } from '@dayflow/plugin-drag';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import SearchIcon from '@lucide/svelte/icons/search';
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
	import { fetchCalendarConflicts, dismissCalendarConflictOnServer, dismissCalendarConflictsOnServer, type CalendarConflict } from './calendar-conflicts';
	import { CalendarDraftEventState, type DraftEventParams } from './calendar-draft-events';
	import { calendarEventPayloadFromDayFlowEvent, dayFlowEventFromCalendarEvent } from './calendar-event-mapping';
	import {
		deletePersistedCalendarEvent,
		fetchCalendarEvents,
		writeCalendarEvent,
		type CalendarEvent
	} from './calendar-event-persistence';
	import {
		installCalendarMonthRangeAction,
		monthRangePreviewSegmentsFromSelection as buildMonthRangePreviewSegments,
		orderedDateKeys,
		type MonthRangePreviewSegment,
		type MonthRangeSelection
	} from './calendar-month-range-action';
	import {
		dateFromDateKey,
		refreshSelectedMonthDateCell as refreshSelectedMonthDateCellElement
	} from './calendar-month-selection';
	import { CalendarProgrammaticUpdateState } from './calendar-programmatic-updates';
	import { syncRemoteCalendar } from './calendar-remote-sync';
	import { searchCalendarEvents, type CalendarSearchResult } from './calendar-search';
	import { loadSavedCalendarDate, saveCalendarDate } from './calendar-storage';
	import { installCalendarWheelNavigation } from './calendar-wheel-navigation';

	type CalendarVisibilityMessage = {
		type: 'calendar-visibility';
		work: boolean;
	};

	const text = createPageText(calendarText);
	const localeCode = $derived(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US');
	const calendarLocale = $derived(createCalendarLocale(currentLocale.value, text));
	const draftEventPlaceholderTitle = () => text.newEvent;
	let visibleStartDate = $state<Date | null>(null);
	let visibleEndDate = $state<Date | null>(null);
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
	let selectedMonthDateKey = $state<string | null>(null);
	let searchText = $state('');
	let toolbarDate = $state(loadSavedCalendarDate(browser));
	let toolbarView = $state(ViewType.MONTH);
	let selectedAuditEventID = $state<string | null>(null);
	let lastMonthCellCreationTime = 0;
	let loadEventsRequestID = 0;
	const draftEvents = new CalendarDraftEventState(draftEventPlaceholderTitle, () => text.newEvent);
	const programmaticUpdates = new CalendarProgrammaticUpdateState();

	let calendarConflicts = $state<CalendarConflict[]>([]);

	const calendar = useCalendarApp({
		views: createCalendarViews(),
		defaultView: ViewType.MONTH,
		initialDate: loadSavedCalendarDate(browser),
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
				enableCreate: true,
				enableAllDayCreate: true,
				onEventDrop: (updatedEvent) => saveUpdatedEvent(updatedEvent),
				onEventResize: (updatedEvent) => saveUpdatedEvent(updatedEvent)
			})
		],
		callbacks: {
			onVisibleRangeChange: (startDate, endDate) => {
				loadEvents(startDate, endDate);
				const middle = new Date((startDate.getTime() + endDate.getTime()) / 2);
				toolbarDate = middle;
				saveCalendarDate(browser, middle);
			},
			onEventClick: (event) => openEventDetails(event.id),
			onEventCreate: (event) => saveCreatedEvent(event),
			onEventUpdate: (event) => saveUpdatedEvent(event),
			onEventDelete: (eventID) => deleteEvent(eventID)
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
		calendarWorkVisible = window.localStorage.getItem('internkim.calendar.workVisible') !== 'false';
		calendar.changeView(ViewType.MONTH);
		toolbarView = ViewType.MONTH;
		syncCalendarThemeToDocument();
		if (!visibleStartDate || !visibleEndDate) {
			const today = new Date();
			loadEvents(startOfMonthWindow(today), endOfMonthWindow(today));
		}
		void Promise.all([syncRemoteCalendarAndRefresh(), loadCalendarConflicts()]).catch((error: unknown) => {
			console.debug('calendar remote sync failed', { error });
		});
		const themeObserver = new MutationObserver(syncCalendarThemeToDocument);
		themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
		const draftTitleObserver = new MutationObserver(() => {
			scheduleDraftTitleInputPlaceholderUpdates();
			scheduleDraftEventVisibilitySync();
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
					createSingleDayEvent: createMonthSingleDayEvent,
					createRangeEvent: createMonthRangeEvent
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
		toolbarDate = shiftedToolbarDate(-1);
		calendar.goToPrevious();
	}

	function goToNext() {
		toolbarDate = shiftedToolbarDate(1);
		calendar.goToNext();
	}

	function shiftedToolbarDate(direction: -1 | 1): Date {
		const nextDate = new Date(toolbarDate);
		if (toolbarView === ViewType.DAY) {
			nextDate.setDate(nextDate.getDate() + direction);
			return nextDate;
		}
		if (toolbarView === ViewType.WEEK) {
			nextDate.setDate(nextDate.getDate() + direction * 7);
			return nextDate;
		}
		nextDate.setMonth(nextDate.getMonth() + direction);
		return nextDate;
	}

	function createQuickEvent() {
		const baseDate = calendar.currentDate ?? new Date();
		const startDate = new Date(baseDate.getFullYear(), baseDate.getMonth(), baseDate.getDate(), 9, 0, 0, 0);
		const endDate = new Date(baseDate.getFullYear(), baseDate.getMonth(), baseDate.getDate(), 10, 0, 0, 0);
		const event = createDraftEvent({
			id: `quick-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
			start: startDate,
			end: endDate,
			allDay: false,
			calendarId: 'internkim'
		});
		calendar.addEvent(event);
		eventCount = calendar.app.getAllEvents().length;
		visibleEvents = calendar.app.getAllEvents();
		openEventDetailsAfterRender(event.id);
	}

	function syncCalendarThemeToDocument() {
		const mode = document.documentElement.classList.contains('dark') ? 'dark' : 'light';
		calendar.app.updateConfig({ theme: { mode } });
	}

	function handleCalendarVisibilityMessage(event: MessageEvent<unknown>) {
		if (!isCalendarVisibilityMessage(event.data)) return;
		setWorkCalendarVisibility(event.data.work);
	}

	function isCalendarVisibilityMessage(value: unknown): value is CalendarVisibilityMessage {
		return Boolean(
			value &&
				typeof value === 'object' &&
				'type' in value &&
				value.type === 'calendar-visibility' &&
				'work' in value &&
				typeof value.work === 'boolean'
		);
	}

	function setWorkCalendarVisibility(isVisible: boolean) {
		calendarWorkVisible = isVisible;
		renderCalendarEvents();
	}

	function renderCalendarEvents() {
		if (!visibleStartDate) return;
		replaceCalendarEvents(calendarWorkVisible ? visibleEvents : [], visibleStartDate);
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

	function startOfMonthWindow(date: Date) {
		return new Date(date.getFullYear(), date.getMonth() - 1, 1);
	}

	function endOfMonthWindow(date: Date) {
		return new Date(date.getFullYear(), date.getMonth() + 2, 1);
	}

	async function loadEvents(startDate: Date, endDate: Date) {
		if (!browser) return;
		const requestID = (loadEventsRequestID += 1);
		visibleStartDate = startDate;
		visibleEndDate = endDate;
		isLoading = true;
		errorMessage = '';
		try {
			const calendarEvents = await fetchCalendarEvents(startDate, endDate, text.error);
			if (requestID !== loadEventsRequestID) return;
			const events = calendarEvents.map(dayFlowEventFromCalendarEvent);
			visibleEvents = events;
			eventCount = events.length;
			replaceCalendarEvents(calendarWorkVisible ? events : [], startDate);
		} catch (error) {
			if (requestID !== loadEventsRequestID) return;
			visibleEvents = [];
			errorMessage = error instanceof Error ? error.message : text.error;
		} finally {
			if (requestID === loadEventsRequestID) isLoading = false;
		}
	}

	function replaceCalendarEvents(events: DayFlowEvent[], visibleRangeStartDate: Date) {
		const eventIDs = calendar.app.getAllEvents().map((event) => event.id);
		calendar.app.applyEventsChanges({ delete: eventIDs, add: recalculateEventDays(events, visibleRangeStartDate) });
		calendar.app.triggerRender();
		refreshSelectedMonthDateCellAfterRender();
	}

	function compareCalendarEventsForDisplay(leftEvent: DayFlowEvent, rightEvent: DayFlowEvent) {
		const leftPriority = leftEvent.allDay ? 0 : 1;
		const rightPriority = rightEvent.allDay ? 0 : 1;
		if (leftPriority !== rightPriority) return leftPriority - rightPriority;
		return leftEvent.title.localeCompare(rightEvent.title);
	}

	async function refreshCalendar() {
		if (!visibleStartDate || !visibleEndDate) return;
		await loadEvents(visibleStartDate, visibleEndDate);
	}

	async function syncRemoteCalendarAndRefresh() {
		await syncRemoteCalendar(text.error);
		await refreshCalendar();
	}

	async function loadCalendarConflicts() {
		if (!browser) return;
		try {
			calendarConflicts = await fetchCalendarConflicts();
		} catch {
			return;
		}
	}

	async function dismissCalendarConflict(conflictID: number) {
		try {
			await dismissCalendarConflictOnServer(conflictID);
			calendarConflicts = calendarConflicts.filter((conflict) => conflict.id !== conflictID);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.error;
		}
	}

	async function dismissAllConflictsAndRefresh() {
		const pendingIDs = calendarConflicts.map((conflict) => conflict.id);
		try {
			await dismissCalendarConflictsOnServer(pendingIDs);
			calendarConflicts = [];
			await syncRemoteCalendarAndRefresh();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.error;
			await loadCalendarConflicts();
		}
	}

	function calendarConflictFieldLabel(field: string): string {
		return text.conflictField[field] ?? field;
	}

	async function saveCreatedEvent(event: DayFlowEvent) {
		draftEvents.addCreatedEvent(event);
		if (isPlaceholderEventTitle(event.title)) {
			void resetDraftEventTitle(event.id);
		}
		scheduleDraftTitleInputPlaceholderUpdates();
		scheduleDraftEventVisibilitySync();
	}

	async function createEventOnServer(event: DayFlowEvent) {
		if (isPlaceholderEventTitle(event.title)) {
			return;
		}
		beginEventPersistence();
		let savedEvent: CalendarEvent;
		try {
			savedEvent = await sendEventWriteRequest('/calendar/api/events', 'POST', event);
		} catch (error) {
			if (draftEvents.shouldReportCreateError(event.id)) {
				showEventPersistenceError(error, text.saveError);
			}
			finishEventPersistence();
			return;
		}
		if (draftEvents.wasDeletedDuringCreate(event.id)) {
			await deleteEventCreatedDuringPendingCreate(event.id);
			return;
		}
		await applyServerCalendarEventMetadata(event.id, savedEvent);
		markEventPersisted();
		finishEventPersistence();
	}

	function isPlaceholderEventTitle(title: string | undefined): boolean {
		return draftEvents.isPlaceholderTitle(title);
	}

	async function deleteEventCreatedDuringPendingCreate(eventID: string) {
		try {
			await deletePersistedEvent(eventID);
			refreshEventCountAfterRender();
		} catch (error) {
			showEventPersistenceError(error, text.deleteError);
			await refreshCalendar();
		} finally {
			finishEventPersistence();
		}
	}

	async function saveUpdatedEvent(event: DayFlowEvent) {
		if (programmaticUpdates.isActive(event.id)) return;
		if (draftEvents.isDraftEvent(event.id)) {
			if (!draftEvents.hasMeaningfulTitle(event)) {
				void resetDraftEventTitle(event.id);
				scheduleDraftTitleInputPlaceholderUpdates();
				return;
			}
			draftEvents.removeDraftEvent(event.id);
			await persistCreatedEvent(event);
			return;
		}
		await persistEvent(`/calendar/api/events/${encodeURIComponent(event.id)}`, 'PUT', event);
	}

	async function resetDraftEventTitle(eventID: string) {
		await programmaticUpdates.run(eventID, () => calendar.updateEvent(eventID, { title: '' }, false));
		visibleEvents = calendar.app.getAllEvents();
		scheduleDraftTitleInputPlaceholderUpdates();
		scheduleDraftEventVisibilitySync();
	}

	async function persistEvent(path: string, method: 'POST' | 'PUT', event: DayFlowEvent) {
		beginEventPersistence();
		try {
			const savedEvent = await sendEventWriteRequest(path, method, event);
			await applyServerCalendarEventMetadata(event.id, savedEvent);
			markEventPersisted();
		} catch (error) {
			showEventPersistenceError(error, text.saveError);
		} finally {
			finishEventPersistence();
		}
	}

	async function deleteEvent(eventID: string) {
		if (selectedAuditEventID === eventID) {
			selectedAuditEventID = null;
		}
		if (draftEvents.isDraftEvent(eventID)) {
			draftEvents.removeDraftEvent(eventID);
			refreshEventCountAfterRender();
			return;
		}
		if (draftEvents.hasPendingCreate(eventID)) {
			draftEvents.markDeletedDuringCreate(eventID);
			refreshEventCountAfterRender();
			return;
		}
		beginDeletePersistence();
		try {
			await deletePersistedEvent(eventID);
			refreshEventCountAfterRender();
		} catch (error) {
			showEventPersistenceError(error, text.deleteError);
			await refreshCalendar();
		} finally {
			finishEventPersistence();
		}
	}

	function beginEventPersistence() {
		isSaving = true;
		statusMessage = '';
		errorMessage = '';
	}

	function beginDeletePersistence() {
		isSaving = true;
		errorMessage = '';
	}

	function finishEventPersistence() {
		isSaving = false;
	}

	function markEventPersisted() {
		statusMessage = text.shared;
		eventCount = calendar.app.getAllEvents().length;
		visibleEvents = calendar.app.getAllEvents();
	}

	function refreshEventCountAfterRender() {
		if (!browser) return;
		requestAnimationFrame(() => {
			eventCount = calendar.app.getAllEvents().length;
			visibleEvents = calendar.app.getAllEvents();
		});
	}

	function showEventPersistenceError(error: unknown, fallback: string) {
		errorMessage = error instanceof Error ? error.message : fallback;
	}

	async function sendEventWriteRequest(path: string, method: 'POST' | 'PUT', event: DayFlowEvent): Promise<CalendarEvent> {
		const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
		const payload = calendarEventPayloadFromDayFlowEvent(event, calendarColors.lineColor, timeZone);
		return writeCalendarEvent(path, method, payload, text.saveError);
	}

	async function deletePersistedEvent(eventID: string) {
		await deletePersistedCalendarEvent(eventID, text.deleteError);
	}

	async function applyServerCalendarEventMetadata(eventID: string, event: CalendarEvent) {
		const currentEvent = calendar.app.getAllEvents().find((candidate) => candidate.id === eventID);
		await programmaticUpdates.run(eventID, () =>
			calendar.updateEvent(
				eventID,
				{
					meta: {
						...(currentEvent?.meta ?? {}),
						uid: event.uid,
						location: event.location,
						color: event.color,
						timeZone: event.timeZone,
						createdByEmail: event.createdByEmail,
						createdByName: event.createdByName,
						updatedByEmail: event.updatedByEmail ?? '',
						updatedByName: event.updatedByName ?? '',
						updatedByAt: event.updatedByAt ?? '',
						updatedAt: event.updatedAt
					}
				},
				false
			)
		);
		visibleEvents = calendar.app.getAllEvents();
	}

	function createMonthRangeEvent(selection: MonthRangeSelection) {
		const [startDateKey, endDateKey] = orderedDateKeys(selection.startDateKey, selection.endDateKey);
		if (startDateKey === endDateKey) {
			createMonthSingleDayEvent(startDateKey);
			return;
		}
		const event = createDraftEvent({
			id: `range-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
			start: dateFromDateKey(startDateKey),
			end: dateFromDateKey(endDateKey),
			allDay: true,
			calendarId: 'internkim'
		});
		calendar.addEvent(event);
		eventCount = calendar.app.getAllEvents().length;
		visibleEvents = calendar.app.getAllEvents();
		openEventDetailsAfterRender(event.id);
	}

	function createMonthSingleDayEvent(dateKey: string) {
		if (Date.now() - lastMonthCellCreationTime < 250) return;
		lastMonthCellCreationTime = Date.now();
		const startDate = dateFromDateKey(dateKey);
		startDate.setHours(9, 0, 0, 0);
		const endDate = new Date(startDate);
		endDate.setHours(10, 0, 0, 0);
		const event = createDraftEvent({
			id: `month-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
			start: startDate,
			end: endDate,
			allDay: false,
			calendarId: 'internkim'
		});
		calendar.addEvent(event);
		eventCount = calendar.app.getAllEvents().length;
		visibleEvents = calendar.app.getAllEvents();
		openEventDetailsAfterRender(event.id);
	}

	function openEventDetailsAfterRender(eventID: string) {
		if (!browser) return;
		requestAnimationFrame(() => {
			requestAnimationFrame(() => {
				openEventDetails(eventID);
				scheduleDraftTitleInputPlaceholderUpdates();
			});
		});
	}

	function openEventDetails(eventID: string) {
		selectedAuditEventID = eventID;
		calendar.app.selectEvent(eventID);
		const eventElement = eventElementByID(eventID);
		if (!eventElement) return;
		eventElement.dispatchEvent(detailOpenEventForElement(eventElement));
	}

	function detailOpenEventForElement(element: HTMLElement) {
		const rectangle = element.getBoundingClientRect();
		return new MouseEvent('dblclick', {
			bubbles: true,
			cancelable: true,
			view: window,
			clientX: rectangle.left + rectangle.width / 2,
			clientY: rectangle.top + rectangle.height / 2
		});
	}

	function eventElementByID(eventID: string) {
		const eventElements = eventElementsByID(eventID);
		return eventElements.find(isVisibleEventElement) ?? eventElements[0] ?? null;
	}

	function eventElementsByID(eventID: string): HTMLElement[] {
		const escapedEventID = window.CSS?.escape(eventID) ?? eventID.replaceAll('"', '\\"');
		return Array.from(
			calendarStageElement?.querySelectorAll<HTMLElement>(
				`[data-event-id="${escapedEventID}"], [data-event-id^="${escapedEventID}::"]`
			) ?? []
		);
	}

	async function persistCreatedEvent(event: DayFlowEvent) {
		await draftEvents.trackCreatedEvent(event, createEventOnServer);
	}

	function createDraftEvent(params: DraftEventParams) {
		return draftEvents.createDraftEvent(params);
	}

	function scheduleDraftTitleInputPlaceholderUpdates() {
		if (!browser) return;
		updateDraftTitleInputPlaceholders();
		requestAnimationFrame(updateDraftTitleInputPlaceholders);
		window.setTimeout(updateDraftTitleInputPlaceholders, 50);
	}

	function updateDraftTitleInputPlaceholders() {
		if (!browser) return;
		draftEvents.updateTitleInputPlaceholders(draftEventPlaceholderTitle(), (input) => {
			void commitDraftTitleInput(input);
		});
	}

	async function commitDraftTitleInput(input: HTMLInputElement) {
		const panel = input.closest<HTMLElement>('[data-event-id]');
		const eventID = panel?.dataset.eventId ?? '';
		if (!draftEvents.isDraftEvent(eventID)) return;
		const title = input.value.trim();
		if (isPlaceholderEventTitle(title)) {
			await resetDraftEventTitle(eventID);
			return;
		}
		await programmaticUpdates.run(eventID, () => calendar.updateEvent(eventID, { title }, false));
		const event = calendar.app.getAllEvents().find((candidate) => candidate.id === eventID);
		if (!event || !draftEvents.hasMeaningfulTitle(event)) return;
		draftEvents.removeDraftEvent(eventID);
		syncDraftEventVisibility();
		await persistCreatedEvent(event);
	}

	function scheduleDraftEventVisibilitySync() {
		if (!browser) return;
		syncDraftEventVisibility();
		requestAnimationFrame(syncDraftEventVisibility);
		window.setTimeout(syncDraftEventVisibility, 50);
	}

	function syncDraftEventVisibility() {
		if (!calendarStageElement) return;
		draftEvents.syncVisibility(calendar.app.getAllEvents(), calendarStageElement, eventElementsByID);
	}

	function isVisibleEventElement(element: HTMLElement) {
		const rectangle = element.getBoundingClientRect();
		return (
			rectangle.width > 0 &&
			rectangle.height > 0 &&
			rectangle.right > 0 &&
			rectangle.bottom > 0 &&
			rectangle.left < window.innerWidth &&
			rectangle.top < window.innerHeight
		);
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
	{#if calendarConflicts.length > 0}
		<aside class="calendar-conflict-banner" role="alert" aria-live="polite">
			<div class="conflict-banner-header">
				<p class="conflict-banner-title">{text.conflictBannerTitle}</p>
				<p class="conflict-banner-description">{text.conflictBannerDescription}</p>
			</div>
			<ul class="conflict-banner-list">
				{#each calendarConflicts as conflict (conflict.id)}
					<li class="conflict-banner-item">
						<span class="conflict-banner-field">{calendarConflictFieldLabel(conflict.field)}</span>
						<span class="conflict-banner-values">
							<span><em>{text.conflictMine}:</em> {conflict.localValue || '—'}</span>
							<span><em>{text.conflictRemote}:</em> {conflict.remoteValue || '—'}</span>
						</span>
						<button
							type="button"
							class="conflict-banner-action"
							onclick={() => dismissCalendarConflict(conflict.id)}
						>
							{text.conflictDismiss}
						</button>
					</li>
				{/each}
			</ul>
			<button
				type="button"
				class="conflict-banner-refresh"
				onclick={() => dismissAllConflictsAndRefresh()}
			>
				{text.conflictRefresh}
			</button>
		</aside>
	{/if}
	<header class="calendar-toolbar">
		<div class="calendar-toolbar-left">
			<button type="button" class="toolbar-button today-button" onclick={goToToday}>
				{text.today}
			</button>
			<button type="button" class="toolbar-icon-button" aria-label={text.previous} onclick={goToPrevious}>
				<ChevronLeftIcon class="size-4" />
			</button>
			<button type="button" class="toolbar-icon-button" aria-label={text.next} onclick={goToNext}>
				<ChevronRightIcon class="size-4" />
			</button>
			<h1 class="calendar-toolbar-title">{currentMonthTitle}</h1>
		</div>
		<div class="calendar-search-shell">
			<label class="calendar-search">
				<SearchIcon class="size-4" />
				<input bind:value={searchText} placeholder={text.search} aria-label={text.searchCalendar} autocomplete="off" />
			</label>
			{#if searchResults.length > 0}
				<div class="calendar-search-results" role="listbox" aria-label={text.searchResults}>
					{#each searchResults as result (result.id)}
						<button type="button" role="option" aria-selected="false" class="calendar-search-result" onclick={() => navigateToSearchResult(result)}>
							<span class="calendar-search-result-date">{result.dateLabel}</span>
							<span class="calendar-search-result-title">
								{#each result.highlightParts as part}
									{#if part.isMatch}
										<mark>{part.text}</mark>
									{:else}
										{part.text}
									{/if}
								{/each}
							</span>
						</button>
					{/each}
				</div>
			{:else if searchText.trim()}
				<div class="calendar-search-results" role="status">
					<p class="calendar-search-empty">{text.noResults}</p>
				</div>
			{/if}
		</div>
		<div class="calendar-view-switcher" aria-label={text.calendarView}>
			<button type="button" class:active-view={toolbarView === ViewType.DAY} onclick={() => changeCalendarView(ViewType.DAY)}>{text.day}</button>
			<button type="button" class:active-view={toolbarView === ViewType.WEEK} onclick={() => changeCalendarView(ViewType.WEEK)}>{text.week}</button>
			<button type="button" class:active-view={toolbarView === ViewType.MONTH} onclick={() => changeCalendarView(ViewType.MONTH)}>{text.month}</button>
		</div>
		<button type="button" class="new-event-button" onclick={createQuickEvent}>
			<PlusIcon class="size-4" />
			<span>{text.new}</span>
		</button>
	</header>
	<div bind:this={calendarStageElement} class="calendar-stage">
		<DayFlowCalendar {calendar} />

		{#each monthRangePreviewSegments as segment (segment.id)}
			<div
				class="month-range-preview"
				style={`left: ${segment.left}px; top: ${segment.top}px; width: ${segment.width}px; height: ${segment.height}px;`}
			>
				<span class="month-range-preview-dot"></span>
				<span class="month-range-preview-title">{draftEventPlaceholderTitle()}</span>
			</div>
		{/each}

		{#if selectedAuditRows.length > 0}
			<aside class="event-audit-card" aria-label={text.eventAudit}>
				{#each selectedAuditRows as row (row.label)}
					<p>
						<span>{row.label}</span>
						<strong>{row.person}</strong>
						{#if row.time}
							<time>{row.time}</time>
						{/if}
					</p>
				{/each}
			</aside>
		{/if}

	</div>
</main>

<style>
	.calendar-page {
		background: #ffffff;
		color: #18181b;
	}

	.calendar-stage {
		--df-color-background: #ffffff;
		--df-color-foreground: #2e2e2e;
		--df-color-hover: #f5f5f5;
		--df-color-border: #e5e5e5;
		--df-color-card: #ffffff;
		--df-color-card-foreground: #2e2e2e;
		--df-color-muted: #f3f4f6;
		--df-color-muted-foreground: #6b7280;
		--df-color-primary: oklch(0.55 0.19 255);
		--df-color-primary-foreground: #ffffff;
		--df-color-secondary: #64748b;
		--df-color-secondary-foreground: #ffffff;
		--df-color-destructive: #d42422;
		--df-color-destructive-foreground: #ffffff;
		--background: 0 0% 100%;
		--foreground: 222.2 84% 4.9%;
		--card: 0 0% 100%;
		--card-foreground: 222.2 84% 4.9%;
		--muted: 210 40% 96.1%;
		--muted-foreground: 215.4 16.3% 46.9%;
		--border: 214.3 31.8% 91.4%;
		--destructive: 0 84.2% 60.2%;
		position: relative;
		min-height: 0;
		flex: 1;
		padding: 0;
		color-scheme: light;
		background: #ffffff;
	}

	.calendar-stage :global(.df-calendar-container) {
		width: 100%;
		--df-calendar-height: calc(100svh - 52px);
		color-scheme: light;
		border-radius: 0;
		border: 0;
	}

	.calendar-stage :global(.draft-empty-title-event) {
		display: none !important;
	}

	.event-audit-card {
		position: absolute;
		right: 16px;
		bottom: 16px;
		z-index: 20;
		display: grid;
		gap: 6px;
		max-width: min(360px, calc(100vw - 32px));
		border: 1px solid #e5e7eb;
		border-radius: 8px;
		background: rgba(255, 255, 255, 0.96);
		padding: 10px 12px;
		box-shadow: 0 12px 30px rgba(15, 23, 42, 0.14);
		color: #111827;
		font-size: 12px;
		line-height: 1.35;
	}

	.event-audit-card p {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr) auto;
		align-items: center;
		gap: 8px;
		margin: 0;
	}

	.event-audit-card span {
		color: #6b7280;
		font-weight: 600;
	}

	.event-audit-card strong {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-weight: 700;
	}

	.event-audit-card time {
		color: #6b7280;
		white-space: nowrap;
	}

	.calendar-stage :global(.df-header),
	.calendar-stage :global(.df-view-header),
	.calendar-stage :global(.df-view-header-container),
	.calendar-stage :global(.df-calendar-container .df-view-header),
	.calendar-stage :global(.df-calendar-container .df-view-header-container) {
		display: none !important;
	}

	:global(.df-dialog-container),
	:global(.df-event-detail-panel),
	:global(.df-portal),
	:global(.df-range-picker) {
		--df-color-background: #ffffff;
		--df-color-foreground: #2e2e2e;
		--df-color-hover: #f5f5f5;
		--df-color-border: #e5e5e5;
		--df-color-card: #ffffff;
		--df-color-card-foreground: #2e2e2e;
		--df-color-muted: #f3f4f6;
		--df-color-muted-foreground: #6b7280;
		--df-color-primary: oklch(0.55 0.19 255);
		--df-color-primary-foreground: #ffffff;
		--df-color-secondary: #64748b;
		--df-color-secondary-foreground: #ffffff;
		--df-color-destructive: #d42422;
		--df-color-destructive-foreground: #ffffff;
		color-scheme: light;
	}

	.calendar-stage :global(.df-month-day-cell-surface[data-other-month='true']) {
		background: transparent;
	}

	.calendar-stage :global(.df-month-day-cell-surface[data-other-month='true'] .df-month-date-number) {
		opacity: 0.38;
	}

	.calendar-stage :global(.df-month-day-cell-surface[data-other-month='true'] .df-event) {
		opacity: 0.52 !important;
	}

	.calendar-stage :global(.df-week-grid > .df-day-label:nth-child(1)),
	.calendar-stage :global(.df-week-grid > .df-day-label:nth-child(7)) {
		color: #ef4444;
	}

	.calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(1) .df-month-date-number),
	.calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(7) .df-month-date-number) {
		color: #ef4444;
	}

	.calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(1)),
	.calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(7)) {
		background: #fafafa;
	}

	.calendar-stage :global(.df-month-title) {
		display: none !important;
	}

	.calendar-stage :global(.df-month-day-cell-surface[data-today='true'] .df-month-date-number),
	.calendar-stage :global(.df-week-day-header[data-today='true'] .df-week-date-number) {
		display: inline-flex;
		min-width: 22px;
		height: 22px;
		align-items: center;
		justify-content: center;
		border-radius: 9999px;
		background: oklch(0.55 0.19 255);
		color: #ffffff;
		font-weight: 700;
	}

	.calendar-toolbar {
		display: flex;
		height: 52px;
		flex-shrink: 0;
		align-items: center;
		gap: 8px;
		border-bottom: 1px solid #e5e7eb;
		background: #ffffff;
		padding: 0 16px;
	}

	.calendar-toolbar-left {
		display: flex;
		min-width: 0;
		align-items: center;
		gap: 12px;
	}

	.toolbar-button,
	.toolbar-icon-button,
	.new-event-button,
	.calendar-view-switcher button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		border: 1px solid #e5e7eb;
		background: #ffffff;
		color: #111827;
		font-weight: 600;
		white-space: nowrap;
		transition:
			background-color 120ms ease,
			border-color 120ms ease,
			color 120ms ease,
			box-shadow 120ms ease;
	}

	.toolbar-button:hover,
	.toolbar-icon-button:hover,
	.calendar-view-switcher button:hover {
		background: #f4f4f5;
	}

	.today-button {
		height: 36px;
		min-width: 94px;
		border-radius: 8px;
		padding: 0 12px;
		font-size: 14px;
	}

	.toolbar-icon-button {
		width: 32px;
		height: 32px;
		border-color: transparent;
		border-radius: 8px;
	}

	.calendar-toolbar-title {
		margin: 0 4px;
		white-space: nowrap;
		font-size: 20px;
		font-weight: 800;
		line-height: 1;
		letter-spacing: 0;
	}

	.calendar-search-shell {
		margin-left: auto;
		position: relative;
		width: min(220px, 20vw);
		flex-shrink: 1;
	}

	.calendar-search {
		display: flex;
		height: 36px;
		width: 100%;
		align-items: center;
		gap: 10px;
		border: 1px solid #e5e7eb;
		border-radius: 8px;
		background: #ffffff;
		padding: 0 12px;
		color: #71717a;
	}

	.calendar-search input {
		min-width: 0;
		flex: 1;
		border: 0;
		background: transparent;
		color: #111827;
		font-size: 14px;
		outline: none;
	}

	.calendar-search input::placeholder {
		color: #71717a;
	}

	.calendar-search-results {
		position: absolute;
		z-index: 40;
		top: calc(100% + 6px);
		right: 0;
		width: min(360px, 72vw);
		overflow: hidden;
		border: 1px solid #e5e7eb;
		border-radius: 10px;
		background: #ffffff;
		box-shadow: 0 14px 30px rgb(15 23 42 / 0.16);
	}

	.calendar-search-result {
		display: grid;
		width: 100%;
		grid-template-columns: 88px minmax(0, 1fr);
		align-items: center;
		gap: 10px;
		border: 0;
		background: transparent;
		padding: 10px 12px;
		text-align: left;
		color: #18181b;
		cursor: pointer;
	}

	.calendar-search-result:hover,
	.calendar-search-result:focus-visible {
		background: #f4f4f5;
		outline: none;
	}

	.calendar-search-result-date {
		color: #71717a;
		font-size: 12px;
		font-weight: 700;
		white-space: nowrap;
	}

	.calendar-search-result-title {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 13px;
		font-weight: 650;
	}

	.calendar-search-result-title mark {
		border-radius: 4px;
		background: color-mix(in oklab, oklch(0.55 0.19 255) 20%, transparent);
		color: inherit;
		padding: 0 1px;
	}

	.calendar-search-empty {
		margin: 0;
		padding: 10px 12px;
		color: #71717a;
		font-size: 13px;
	}

	.calendar-view-switcher {
		display: inline-flex;
		height: 36px;
		align-items: center;
		border-radius: 8px;
		background: #f4f4f5;
		padding: 2px;
	}

	.calendar-view-switcher button {
		height: 32px;
		min-width: 58px;
		border: 0;
		border-radius: 7px;
		background: transparent;
		padding: 0 8px;
		color: #71717a;
		font-size: 14px;
	}

	.calendar-view-switcher button.active-view {
		background: #ffffff;
		color: #111827;
		box-shadow: 0 1px 4px rgb(15 23 42 / 0.14);
	}

	.new-event-button {
		height: 36px;
		gap: 8px;
		border-color: transparent;
		border-radius: 8px;
		background: oklch(0.55 0.19 255);
		padding: 0 14px;
		color: #ffffff;
		font-size: 14px;
	}

	.new-event-button:hover {
		background: color-mix(in oklch, oklch(0.55 0.19 255) 88%, black);
	}

	:global(html.dark) .calendar-page {
		background: #09090b;
		color: #f4f4f5;
	}

	:global(html.dark) .calendar-stage {
		--df-color-background: #09090b;
		--df-color-foreground: #f4f4f5;
		--df-color-hover: #18181b;
		--df-color-border: #27272a;
		--df-color-card: #09090b;
		--df-color-card-foreground: #f4f4f5;
		--df-color-muted: #18181b;
		--df-color-muted-foreground: #a1a1aa;
		background: #09090b;
		color-scheme: dark;
	}

	:global(html.dark) .calendar-stage :global(.df-calendar-container) {
		color-scheme: dark;
		background: #09090b;
	}

	:global(html.dark) .event-audit-card {
		border-color: #27272a;
		background: rgba(9, 9, 11, 0.96);
		color: #f4f4f5;
		box-shadow: 0 12px 30px rgba(0, 0, 0, 0.36);
	}

	:global(html.dark) .event-audit-card span,
	:global(html.dark) .event-audit-card time {
		color: #a1a1aa;
	}

	:global(html.dark) :global(.df-dialog-container),
	:global(html.dark) :global(.df-event-detail-panel),
	:global(html.dark) :global(.df-portal),
	:global(html.dark) :global(.df-range-picker) {
		--df-color-background: #09090b;
		--df-color-foreground: #f4f4f5;
		--df-color-hover: #18181b;
		--df-color-border: #27272a;
		--df-color-card: #09090b;
		--df-color-card-foreground: #f4f4f5;
		--df-color-muted: #18181b;
		--df-color-muted-foreground: #a1a1aa;
		color-scheme: dark;
	}

	:global(html.dark) .calendar-toolbar {
		border-bottom-color: #27272a;
		background: #09090b;
	}

	:global(html.dark) .toolbar-button,
	:global(html.dark) .toolbar-icon-button,
	:global(html.dark) .calendar-view-switcher button {
		border-color: #27272a;
		background: #09090b;
		color: #f4f4f5;
	}

	:global(html.dark) .toolbar-button:hover,
	:global(html.dark) .toolbar-icon-button:hover,
	:global(html.dark) .calendar-view-switcher button:hover {
		background: #18181b;
	}

	:global(html.dark) .calendar-search {
		border-color: #27272a;
		background: #09090b;
		color: #a1a1aa;
	}

	:global(html.dark) .calendar-search input {
		color: #f4f4f5;
	}

	:global(html.dark) .calendar-search-results {
		border-color: #27272a;
		background: #09090b;
		box-shadow: 0 14px 30px rgb(0 0 0 / 0.38);
	}

	:global(html.dark) .calendar-search-result {
		color: #f4f4f5;
	}

	:global(html.dark) .calendar-search-result:hover,
	:global(html.dark) .calendar-search-result:focus-visible {
		background: #18181b;
	}

	:global(html.dark) .calendar-search-result-title mark {
		background: color-mix(in oklab, oklch(0.6 0.2 255) 36%, transparent);
	}

	:global(html.dark) .calendar-view-switcher {
		background: #18181b;
	}

	:global(html.dark) .calendar-view-switcher button {
		border-color: transparent;
		background: transparent;
		color: #a1a1aa;
	}

	:global(html.dark) .calendar-view-switcher button.active-view {
		background: #27272a;
		color: #f4f4f5;
		box-shadow: none;
	}

	:global(html.dark) .calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(1)),
	:global(html.dark) .calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(7)) {
		background: #111113;
	}

	:global(html.dark) .calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(1) .df-month-date-number),
	:global(html.dark) .calendar-stage :global(.df-month-week-grid > .df-month-day-cell:nth-child(7) .df-month-date-number) {
		color: #f87171;
	}

	:global(html.dark) .calendar-stage :global(.df-month-day-cell-surface[data-other-month='true']) {
		background: transparent;
	}

	.calendar-stage :global(.df-month-day-cell.month-selected-date) {
		background: color-mix(in oklab, var(--primary) 8%, transparent);
		box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--primary) 28%, transparent);
	}

	.calendar-stage :global(.df-month-week-grid > .df-month-day-cell.month-selected-date:nth-child(1)),
	.calendar-stage :global(.df-month-week-grid > .df-month-day-cell.month-selected-date:nth-child(7)) {
		background: color-mix(in oklab, var(--primary) 8%, transparent);
	}

	.month-range-preview {
		position: absolute;
		z-index: 15;
		display: flex;
		align-items: center;
		gap: 6px;
		overflow: hidden;
		border-radius: 4px;
		background: var(--primary);
		padding: 0 8px;
		color: var(--primary-foreground);
		font-size: 12px;
		font-weight: 600;
		line-height: 1;
		pointer-events: none;
		box-shadow:
			0 1px 2px rgb(15 23 42 / 0.18),
			inset 0 0 0 1px rgb(255 255 255 / 0.18);
	}

	.month-range-preview-dot {
		width: 6px;
		height: 6px;
		flex: 0 0 auto;
		border-radius: 9999px;
		background: color-mix(in oklab, var(--primary-foreground) 80%, transparent);
	}

	.month-range-preview-title {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	@media (max-width: 767px) {
		.calendar-stage {
			padding: 0;
		}

		.calendar-stage :global(.df-calendar-container) {
			border-radius: 0;
			--df-calendar-height: calc(100svh - 96px);
		}

		.calendar-toolbar {
			height: auto;
			flex-wrap: wrap;
			gap: 8px;
			padding: 10px;
		}

		.calendar-toolbar-title {
			margin-left: 2px;
			font-size: 16px;
		}

		.calendar-search-shell {
			order: 4;
			width: 100%;
			margin-left: 0;
		}

		.new-event-button {
			margin-left: auto;
		}
	}

	.calendar-conflict-banner {
		margin: 12px 16px 0;
		padding: 12px 16px;
		border-radius: var(--radius);
		background-color: var(--warning-subtle);
		border: 1px solid var(--warning);
		color: var(--warning-subtle-foreground);
		display: flex;
		flex-direction: column;
		gap: 8px;
		font-size: 13px;
	}

	.conflict-banner-title {
		margin: 0;
		font-weight: 600;
		font-size: 14px;
	}

	.conflict-banner-description {
		margin: 0;
		color: var(--warning-subtle-foreground);
		opacity: 0.85;
	}

	.conflict-banner-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.conflict-banner-item {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px;
		padding: 6px 8px;
		background-color: color-mix(in oklab, hsl(var(--card)) 70%, transparent);
		border-radius: 6px;
	}

	.conflict-banner-field {
		font-weight: 600;
		min-width: 80px;
	}

	.conflict-banner-values {
		display: flex;
		flex: 1;
		flex-wrap: wrap;
		gap: 12px;
		color: var(--warning-subtle-foreground);
	}

	.conflict-banner-values em {
		font-style: normal;
		color: var(--warning-subtle-foreground);
		opacity: 0.7;
		margin-right: 4px;
	}

	.conflict-banner-action,
	.conflict-banner-refresh {
		appearance: none;
		border: 1px solid var(--warning);
		background: transparent;
		color: var(--warning-subtle-foreground);
		padding: 4px 10px;
		border-radius: 6px;
		font-size: 12px;
		cursor: pointer;
	}

	.conflict-banner-action:hover,
	.conflict-banner-refresh:hover {
		background-color: color-mix(in oklab, var(--warning) 10%, transparent);
	}

	.conflict-banner-refresh {
		align-self: flex-end;
	}
</style>
