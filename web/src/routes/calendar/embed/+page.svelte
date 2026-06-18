<script lang="ts">
	import { browser } from '$app/environment';
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
		broadcastCalendarVisibleDate
	} from '../refresh-signal.svelte';
	import { createCalendarPageController } from './calendar-page-controller.svelte';
	import { installCalendarPageEffects } from './calendar-page-effects.svelte';
	import { installCalendarPageLifecycle } from './calendar-page-lifecycle-install';
	import CalendarPageContent from './calendar-page-content.svelte';
	import { syncCalendarThemeToDocument } from './calendar-page-theme';
	import { createCalendarEmbedPageState } from './calendar-page-state.svelte';
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
	const calendarOptions = $derived([{ id: 'internkim', name: text.work }]);
	const controller = createCalendarPageController({
		isBrowser: () => browser,
		getCalendarLocale: () => calendarLocale,
		getLocaleCode: () => localeCode,
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
		state.calendarWorkVisible
			? visibleEventsWithPreservedLocalEvents(
					state.visibleEvents,
					calendarStageEventsWithDraftPopover(calendar.events, draftEvents.createdEvents(), state.draftPopover),
					(event) => shouldPreserveLocalCalendarEvent(draftEvents, event)
				)
			: []
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
		rangePreview,
		renderSync,
		selectedMonthDate,
		pageNavigation,
		text
	});

	onMount(() => {
		return installCalendarPageLifecycle({
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
			eventDetails,
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
			initialCalendarDate,
			initialCalendarView,
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
			setWorkCalendarVisible: (isVisible) => {
				state.calendarWorkVisible = isVisible;
			},
			syncCalendarThemeToDocument: () => syncCalendarThemeToDocument(calendar.app),
			text
		});
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
</script>
<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<CalendarPageContent
	calendar={calendar}
	conflicts={state.calendarConflicts}
	dismissConflict={conflictActions.dismissCalendarConflict}
	refreshConflicts={conflictActions.dismissAllConflictsAndRefresh}
	{currentMonthTitle}
	bind:searchText={state.searchText}
	{searchResults}
	toolbarView={state.toolbarView}
	changeCalendarView={pageNavigation.changeCalendarView}
	goToToday={pageNavigation.goToToday}
	goToPrevious={pageNavigation.goToPrevious}
	goToNext={pageNavigation.goToNext}
	navigateToSearchResult={pageNavigation.navigateToSearchResult}
	createQuickEvent={draftPopoverActions.createQuickDraftPopover}
	clearSelectedEvent={eventSelection.clearSelectedEvent}
	stageEvents={stageEvents}
	{localeCode}
	selectedEventID={state.selectedAuditEventID}
	openEvent={eventDetails.openEventDetails}
	selectEvent={eventSelection.selectCalendarEvent}
	selectDate={selectedMonthDate.selectMonthDate}
	saveMovedEvent={eventSelection.saveMovedMonthEvent}
	bind:stageElement={state.calendarStageElement}
	monthRangePreviewSegments={state.monthRangePreviewSegments}
	monthRangePreviewTitle={draftEventPlaceholderTitle()}
	timelineRangePreviewSegments={state.timelineRangePreviewSegments}
	timelineRangePreviewTitle={draftEventPlaceholderTitle()}
	monthScrollOverlayLabels={state.monthScrollOverlayLabels}
	popover={state.draftPopover}
	auditEvent={selectedAuditEvent}
	{calendarOptions}
	isSaving={state.isSaving}
	{text}
	updatePopover={draftPopoverActions.updateDraftPopover}
	repositionPopover={draftPopoverActions.repositionDraftPopover}
	savePopover={draftPopoverActions.saveDraftPopover}
	cancelPopover={draftPopoverActions.cancelDraftPopover}
	deletePopover={draftPopoverActions.deleteDraftPopover}
/>
