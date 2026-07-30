<script lang="ts">
	import { browser } from '$app/environment';
	import { forwardAppShortcut } from '$lib/app-shortcut-message';
	import { isPlainShortcut } from '$lib/keyboard-shortcut';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { ViewType } from '../calendar-view-type';
	import { onMount } from 'svelte';
	import { calendarText } from '../text';
	import {
		normalizedVisibleDate
	} from './calendar-embed-view-helpers';
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
	import { endOfMonthWindow, startOfMonthWindow } from './calendar-visible-range';
	import { createCalendarEmbedPageState } from './calendar-page-state.svelte';
	import {
		calendarParticipantKey,
		calendarParticipantsFromUnknown,
		calendarParticipantsWithViewerFirst,
		fetchCalendarParticipants
	} from './calendar-participants';
	import { fetchWebSessionEmail } from '$lib/web-session';
	import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
	import './calendar-page.css';

	const text = createPageText(calendarText);
	const localeCode = $derived(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US');
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
		getIsMobileTwoDayWeekView: () => isMobileTwoDayWeekView,
		getLocaleCode: () => localeCode,
		initialCalendarDate,
		initialCalendarView,
		setVisibleDate,
		state,
		text
	});
	const {
		eventStore,
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

	let loadedEventWindowKey = '';

	$effect(() => {
		const visibleDate = state.toolbarDate;
		const windowKey = `${visibleDate.getFullYear()}-${visibleDate.getMonth()}`;
		if (windowKey === loadedEventWindowKey) return;
		loadedEventWindowKey = windowKey;
		void eventLoader.loadEvents(startOfMonthWindow(visibleDate), endOfMonthWindow(visibleDate));
	});

	const stageEvents = $derived(
		visibleEventsWithPreservedLocalEvents(
			state.visibleEvents,
			calendarStageEventsWithDraftPopover(eventStore.events, draftEvents.createdEvents(), state.draftPopover),
			(event) => shouldPreserveLocalCalendarEvent(draftEvents, event)
		)
	);

	const filteredStageEvents = $derived(
		state.participantFilterKey
			? stageEvents.filter((event) =>
					calendarParticipantsFromUnknown(event.meta?.participants).some(
						(participant) => calendarParticipantKey(participant) === state.participantFilterKey
					)
				)
			: stageEvents
	);


	installCalendarPageEffects({
		isBrowser: () => browser,
		getStageElement: () => state.calendarStageElement,
		getToolbarDate: () => state.toolbarDate,
		getToolbarView: () => state.toolbarView,
		getVisibleEvents: () => state.visibleEvents,
		getMonthRangeSelection: () => state.monthRangeSelection,
		getTimelineRangeSelection: () => state.timelineRangeSelection,
		rangePreview,
		renderSync,
		text
	});

	onMount(() => {
		void loadParticipantCandidates();
		broadcastCalendarVisibleDate(state.toolbarDate);
		const uninstallCalendarPageLifecycle = installCalendarPageLifecycle({
			applyCalendarView: (view) => {
				state.toolbarView = view;
			},
			clearDraftPopover: () => {
				state.draftPopover = null;
			},
			deleteEvent: eventActions.deleteEvent,
			undoLastDelete: eventActions.undoLastDelete,
			draftPopoverActions,
			eventActions,
			eventLoader,
			eventSelection,
			getCurrentView: currentCalendarView,
			getDraftPopover: () => state.draftPopover,
			getSelectedAuditEventID: () => state.selectedAuditEventID,
			getStageElement: () => state.calendarStageElement,
			getToolbarDate: () => state.toolbarDate,
			initialCalendarDate,
			initialCalendarView,
			pageMessages,
			pageNavigation,
			renderSync,
			selectedMonthDate,
			setSelectedAuditEventID: (eventID) => {
				state.selectedAuditEventID = eventID;
			},
			setToolbarView: (view) => {
				state.toolbarView = view;
			},
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
		return state.toolbarView;
	}

	function openCalendarEvent(eventID: string, anchor: DraftPopoverAnchor): void {
		eventSelection.selectCalendarEvent(eventID);
		eventDetails.openEventDetails(eventID, anchor);
	}

	function openCalendarSettings(): void {
		requestCalendarSettingsOpen();
	}


	async function loadParticipantCandidates(): Promise<void> {
		try {
			const candidates = await fetchCalendarParticipants(text.error);
			state.participantCandidates = calendarParticipantsWithViewerFirst(candidates, await fetchWebSessionEmail());
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
	addEventOnTimeRange={(start, end) =>
		draftPopoverActions.openTimelineRangeDraftPopover(start, end, { clientX: 0, clientY: 0 })}
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
	openSettings={openCalendarSettings}
	clearSelectedEvent={eventSelection.clearSelectedEvent}
	stageEvents={filteredStageEvents}
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
	participantFilterKey={state.participantFilterKey}
	selectParticipantFilter={(participantKey) => {
		state.participantFilterKey = participantKey;
	}}
	{calendarOptions}
	participantCandidates={state.participantCandidates}
	{text}
	updatePopover={draftPopoverActions.updateDraftPopover}
	savePopover={draftPopoverActions.saveDraftPopover}
	cancelPopover={draftPopoverActions.cancelDraftPopover}
	deletePopover={draftPopoverActions.deleteDraftPopover}
/>
