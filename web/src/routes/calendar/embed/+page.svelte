<script lang="ts">
	import { browser } from '$app/environment';
	import { forwardAppShortcut } from '$lib/app-shortcut-message';
	import { isPlainShortcut } from '$lib/keyboard-shortcut';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { ViewType } from '@dayflow/svelte';
	import { onMount } from 'svelte';
	import { calendarText } from '../text';
	import {
		normalizedVisibleDate
	} from './calendar-embed-view-helpers';
	import {
		createCalendarLocale
	} from './calendar-config';
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
		broadcastCalendarVisibleDate(state.toolbarDate);
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


	async function loadParticipantCandidates(): Promise<void> {
		try {
			state.participantCandidates = await fetchCalendarParticipants(text.error);
		} catch {
			state.participantCandidates = [];
		}
	}
	function handleShortcut(event: KeyboardEvent) {
		if (isPlainShortcut(event, 'Slash')) {
			event.preventDefault();
			forwardAppShortcut('Slash');
			return;
		}
		if (!isPlainShortcut(event, 'KeyR')) return;
		event.preventDefault();
		forwardAppShortcut('KeyR');
	}
</script>

<svelte:window onkeydown={handleShortcut} />

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
	toolbarDate={state.toolbarDate}
	toolbarView={state.toolbarView}
	changeCalendarView={pageNavigation.changeCalendarView}
	goToPrevious={pageNavigation.goToPrevious}
	goToToday={pageNavigation.goToToday}
	goToNext={pageNavigation.goToNext}
	navigateToDateKey={pageNavigation.navigateToDateKey}
	selectedMonthDateKey={state.selectedMonthDateKey}
	visibleMonthChanged={(month) => pageNavigation.setVisibleDate(month)}
	addEventOnDay={(dateKey) => draftPopoverActions.openMonthSingleDayDraftPopover(dateKey)}
	addEventOnRange={(startDateKey, endDateKey) =>
		draftPopoverActions.openMonthRangeDraftPopover({
			pointerID: 0,
			startDateKey,
			endDateKey,
			startClientX: 0,
			startClientY: 0,
			hasMoved: true
		})}
	deleteEvent={(eventID) => void eventActions.deleteEvent(eventID)}
	{createQuickEvent}
	openSettings={openCalendarSettings}
	clearSelectedEvent={eventSelection.clearSelectedEvent}
	stageEvents={stageEvents}
	{localeCode}
	selectedEventID={state.selectedAuditEventID}
	openEvent={openCalendarEvent}
	selectDate={selectedMonthDate.selectMonthDate}
	saveMovedEvent={eventSelection.saveMovedMonthEvent}
	bind:stageElement={state.calendarStageElement}
	monthRangePreviewSegments={state.monthRangePreviewSegments}
	monthRangePreviewTitle={draftEventPlaceholderTitle()}
	timelineRangePreviewSegments={state.timelineRangePreviewSegments}
	timelineRangePreviewTitle={draftEventPlaceholderTitle()}
	isMobileTwoDayWeekView={isMobileTwoDayWeekView}
	popover={state.draftPopover}
	auditEvent={selectedAuditEvent}
	{calendarOptions}
	participantCandidates={state.participantCandidates}
	isSaving={state.isSaving}
	{text}
	updatePopover={draftPopoverActions.updateDraftPopover}
	savePopover={draftPopoverActions.saveDraftPopover}
	cancelPopover={draftPopoverActions.cancelDraftPopover}
	deletePopover={draftPopoverActions.deleteDraftPopover}
/>
