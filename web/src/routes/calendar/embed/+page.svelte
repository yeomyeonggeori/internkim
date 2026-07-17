<script lang="ts">
	import { browser } from '$app/environment';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { ViewType } from '@dayflow/svelte';
	import { onMount } from 'svelte';
	import '@dayflow/core/dist/styles.css';
	import { calendarText } from '../text';
	import {
		normalizedVisibleDate
	} from './calendar-embed-view-helpers';
	import {
		createCalendarLocale
	} from './calendar-config';
	import { searchCalendarEvents } from './calendar-search';
	import {
		calendarSearchParams,
		initialCalendarDate as createInitialCalendarDate,
		initialCalendarEventID as createInitialCalendarEventID,
		initialCalendarView as createInitialCalendarView
	} from './calendar-initial-page-state';
	import {
		calendarStageEventsWithDraftPopover,
		shouldPreserveLocalCalendarEvent,
		visibleEventsWithPreservedLocalEvents
	} from './calendar-visible-events';
	import {
		broadcastCalendarVisibleDate,
		requestCalendarRefresh,
		requestCalendarSettingsOpen
	} from '../refresh-signal.svelte';
	import { createCalendarPageController } from './calendar-page-controller.svelte';
	import { installCalendarPageEffects } from './calendar-page-effects.svelte';
	import { installCalendarPageLifecycle } from './calendar-page-lifecycle-install';
	import { isCalendarMobileTwoDayWeekView } from './calendar-mobile-two-day-week';
	import CalendarPageContent from './calendar-page-content.svelte';
	import { syncCalendarThemeToDocument } from './calendar-page-theme';
	import { createCalendarEmbedPageState } from './calendar-page-state.svelte';
	import { fetchCalendarParticipants } from './calendar-participants';
	import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
	import './calendar-page.css';

	const text = createPageText(calendarText);
	const localeCode = $derived(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US');
	const calendarLocale = $derived(createCalendarLocale(currentLocale.value, text));
	const draftEventPlaceholderTitle = () => text.newEvent;
	const initialCalendarDate = () => createInitialCalendarDate(browser, calendarSearchParams(browser));
	const initialCalendarEventID = () => createInitialCalendarEventID(browser, calendarSearchParams(browser));
	const initialCalendarView = () => createInitialCalendarView(browser);
	const state = createCalendarEmbedPageState({
		toolbarDate: initialCalendarDate(),
		toolbarView: initialCalendarView(),
		pendingEventID: initialCalendarEventID()
	});
	const isMobile = new IsMobile();
	const isCompactEventEditor = $derived(isMobile.current);
	const isMobileTwoDayWeekView = $derived(isCalendarMobileTwoDayWeekView(state.toolbarView, isMobile.current));
	const calendarOptions = $derived([{ id: 'internkim', name: text.work }]);
	const controller = createCalendarPageController({
		isBrowser: () => browser,
		getCalendarLocale: () => calendarLocale,
		getLocaleCode: () => localeCode,
		getIsMobileTwoDayWeekView: () => isMobileTwoDayWeekView,
		initialCalendarDate,
		initialCalendarView,
		setVisibleDate,
		state,
		text
	});
	const {
		calendar,
		conflictActions,
		draftEvents,
		draftPopoverActions,
		eventActions,
		eventDetails,
		eventLoader,
		eventSelection,
		pageMessages,
		pageNavigation,
		rangePreview,
		renderSync,
		scrollOverlays,
		selectedMonthDate
	} = controller;

	const stageEvents = $derived(
		visibleEventsWithPreservedLocalEvents(
			state.visibleEvents,
			calendarStageEventsWithDraftPopover(calendar.events, draftEvents.createdEvents(), state.draftPopover),
			(event) => shouldPreserveLocalCalendarEvent(draftEvents, event)
		)
	);

	const selectedAuditEvent = $derived(
		state.selectedAuditEventID ? (calendar.events.find((event) => event.id === state.selectedAuditEventID) ?? null) : null
	);

	installCalendarPageEffects({
		isBrowser: () => browser,
		calendar,
		getCalendarLocale: () => calendarLocale,
		getCurrentLocale: () => currentLocale.value,
		getMonthRangeSelection: () => state.monthRangeSelection,
		getTimelineRangeSelection: () => state.timelineRangeSelection,
		getStageElement: () => state.calendarStageElement,
		getToolbarDate: () => state.toolbarDate,
		getToolbarView: () => state.toolbarView,
		getVisibleEvents: () => state.visibleEvents,
		getLocaleCode: () => localeCode,
		getSelectedMonthDateKey: () => state.selectedMonthDateKey,
		getIsMobileTwoDayWeekView: () => isMobileTwoDayWeekView,
		rangePreview,
		renderSync,
		selectedMonthDate,
		pageNavigation,
		text
	});

	onMount(() => {
		void loadParticipantCandidates();
		window.addEventListener('pagehide', eventActions.flushPendingDeleteOnPageHide);
		const uninstallCalendarPageLifecycle = installCalendarPageLifecycle({
			applyCalendarView: (view) => {
				calendar.changeView(view);
			},
			clearDraftPopover: () => {
				state.draftPopover = null;
			},
			clearMonthRangePreview: () => {
				state.monthRangePreviewSegments = [];
			},
			deleteEvent: eventActions.deleteEvent,
			draftPopoverActions,
			eventActions,
			eventLoader,
			eventSelection,
			getCurrentView: currentCalendarView,
			getDraftPopover: () => state.draftPopover,
			getLocaleCode: () => localeCode,
			getMonthRangeSelection: () => state.monthRangeSelection,
			getSelectedAuditEventID: () => state.selectedAuditEventID,
			getStageElement: () => state.calendarStageElement,
			getTimelineRangeSelection: () => state.timelineRangeSelection,
			getToolbarDate: () => state.toolbarDate,
			getIsMobileTwoDayWeekView: () => isMobileTwoDayWeekView,
			initialCalendarDate,
			initialCalendarView,
			openEventEditor: openCalendarEvent,
			pageMessages,
			pageNavigation,
			rangePreview,
			renderSync,
			scrollOverlays,
			selectedMonthDate,
			setMonthRangeSelection: (selection) => {
				state.monthRangeSelection = selection;
			},
			setSelectedAuditEventID: (eventID) => {
				state.selectedAuditEventID = eventID;
			},
			setToolbarView: (view) => {
				state.toolbarView = view;
			},
			syncCalendarThemeToDocument: () => syncCalendarThemeToDocument(calendar.app),
			text
		});
		return () => {
			window.removeEventListener('pagehide', eventActions.flushPendingDeleteOnPageHide);
			uninstallCalendarPageLifecycle();
			void eventActions.flushPendingDelete();
		};
	});

	const currentMonthTitle = $derived(
		state.toolbarDate.toLocaleDateString(localeCode, {
			year: 'numeric',
			month: 'long'
		})
	);
	const searchResults = $derived(searchCalendarEvents(state.searchText, state.visibleEvents));

	function setVisibleDate(date: Date) {
		const visibleDate = normalizedVisibleDate(date);
		state.toolbarDate = visibleDate;
		broadcastCalendarVisibleDate(visibleDate);
	}

	function currentCalendarView(): ViewType {
		if (calendar.currentView === ViewType.DAY) return ViewType.DAY;
		if (calendar.currentView === ViewType.WEEK) return ViewType.WEEK;
		if (calendar.currentView === ViewType.MONTH) return ViewType.MONTH;
		return state.toolbarView;
	}

	function createQuickEvent(event: MouseEvent): void {
		if (isMobileTwoDayWeekView) {
			eventActions.openQuickEventMobileEditor();
			return;
		}
		draftPopoverActions.createQuickDraftPopover(event);
	}

	function openCalendarEvent(eventID: string, anchor: DraftPopoverAnchor): void {
		eventSelection.selectCalendarEvent(eventID);
		if (isCompactEventEditor) {
			eventActions.openEventMobileEditor(eventID);
			return;
		}
		eventDetails.openEventDetails(eventID, anchor);
	}

	function openCalendarSettings(): void {
		requestCalendarSettingsOpen();
	}

	function refreshParentCalendar(): void {
		requestCalendarRefresh();
	}

	async function loadParticipantCandidates(): Promise<void> {
		try {
			state.participantCandidates = await fetchCalendarParticipants(text.error);
		} catch {
			state.participantCandidates = [];
		}
	}
</script>
<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<CalendarPageContent
	activeMobileEditorEventID={state.activeMobileEditorEventID}
	calendar={calendar}
	clearActiveMobileEditorEvent={(eventID) => {
		if (state.activeMobileEditorEventID === eventID) state.activeMobileEditorEventID = null;
	}}
	conflicts={state.calendarConflicts}
	dismissConflict={conflictActions.dismissCalendarConflict}
	refreshConflicts={conflictActions.dismissAllConflictsAndRefresh}
	{currentMonthTitle}
	bind:searchText={state.searchText}
	{searchResults}
	toolbarDate={state.toolbarDate}
	toolbarView={state.toolbarView}
	changeCalendarView={pageNavigation.changeCalendarView}
	goToPrevious={pageNavigation.goToPrevious}
	goToNext={pageNavigation.goToNext}
	navigateToDateKey={pageNavigation.navigateToDateKey}
	navigateToSearchResult={pageNavigation.navigateToSearchResult}
	{createQuickEvent}
	openSettings={openCalendarSettings}
	refreshCalendar={refreshParentCalendar}
	clearSelectedEvent={eventSelection.clearSelectedEvent}
	stageEvents={stageEvents}
	{localeCode}
	selectedEventID={state.selectedAuditEventID}
	openEvent={openCalendarEvent}
	selectEvent={eventSelection.selectCalendarEvent}
	selectDate={selectedMonthDate.selectMonthDate}
	saveMovedEvent={eventSelection.saveMovedMonthEvent}
	bind:stageElement={state.calendarStageElement}
	monthRangePreviewSegments={state.monthRangePreviewSegments}
	monthRangePreviewTitle={draftEventPlaceholderTitle()}
	timelineRangePreviewSegments={state.timelineRangePreviewSegments}
	timelineRangePreviewTitle={draftEventPlaceholderTitle()}
	monthScrollOverlayLabels={state.monthScrollOverlayLabels}
	isMobileTwoDayWeekView={isMobileTwoDayWeekView}
	popover={state.draftPopover}
	auditEvent={selectedAuditEvent}
	{calendarOptions}
	participantCandidates={state.participantCandidates}
	isSaving={state.isSaving}
	{text}
	updatePopover={draftPopoverActions.updateDraftPopover}
	repositionPopover={draftPopoverActions.repositionDraftPopover}
	savePopover={draftPopoverActions.saveDraftPopover}
	cancelPopover={draftPopoverActions.cancelDraftPopover}
	deletePopover={draftPopoverActions.deleteDraftPopover}
/>
