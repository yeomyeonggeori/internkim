<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import AppFloatingActionButton from '$lib/components/app-floating-action-button.svelte';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { forwardAppShortcut } from '$lib/app-shortcut-message';
	import { isPlainShortcut } from '$lib/keyboard-shortcut';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { ViewType } from '../calendar-view-type';
	import { onMount, untrack } from 'svelte';
	import { calendarText } from '../text';
	import {
		normalizedVisibleDate
	} from './calendar-embed-view-helpers';
	import {
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
		calendarRefresh,
		calendarNavigation,
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
		calendarViewerParticipants,
		fetchCalendarParticipants
	} from './calendar-participants';
	import { signedInEmail } from '$lib/signed-in-email';
	import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
	import './calendar-page.css';
	import { calendarSettings } from '../calendar-settings-context';
	import { calendarDateKey } from '../calendar-layout-date';

	let { embedded = false }: { embedded?: boolean } = $props();
	const openSettings = calendarSettings();

	const text = createPageText(calendarText);
	const localeCode = $derived(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US');
	const draftEventPlaceholderTitle = () => text.newEvent;
	const initialCalendarDate = () => createInitialCalendarDate(page.url.searchParams);
	const initialCalendarEventID = () => createInitialCalendarEventID(page.url.searchParams);
	const initialCalendarView = () => createInitialCalendarView(browser);
	const state = createCalendarEmbedPageState({
		toolbarDate: initialCalendarDate(),
		toolbarView: initialCalendarView(),
		pendingEventID: initialCalendarEventID()
	});
	const isMobile = new IsMobile();
	const isMobileTwoDayWeekView = $derived(isCalendarMobileTwoDayWeekView(state.toolbarView, isMobile.current));
	const calendarOptions = $derived([{ id: 'internkim', name: text.work }]);
	const controller = createCalendarPageController({
		isBrowser: () => browser,
		getIsMobileTwoDayWeekView: () => isMobileTwoDayWeekView,
		getLocaleCode: () => localeCode,
		getLocale: () => currentLocale.value,
		initialCalendarDate,
		initialCalendarView,
		setVisibleDate,
		state,
		text
	});
	const {
		eventStore,
		draftEvents,
		draftPopoverActions,
		eventActions,
		eventDetails,
		eventLoader,
		eventSelection,
		pageMessages,
		pageNavigation,
		rangePreview,
		selectedMonthDate
	} = controller;

	let loadedEventWindowKey = '';
	let lastRefresh = calendarRefresh.ticks;
	let lastQuery = page.url.search;
	let isDisposed = false;

	$effect(() => {
		const dateKey = calendarNavigation.dateKey;
		if (embedded || !dateKey) return;
		untrack(() => pageNavigation.navigateToDateKey(dateKey));
	});

	$effect(() => {
		const query = page.url.search;
		if (query === lastQuery) return;
		lastQuery = query;
		untrack(() => {
			state.pendingEventID = initialCalendarEventID();
			const date = initialCalendarDate();
			const monthChanged = date.getFullYear() !== state.toolbarDate.getFullYear() || date.getMonth() !== state.toolbarDate.getMonth();
			pageNavigation.navigateToDateKey(calendarDateKey(date));
			if (!monthChanged) eventSelection.openPendingCalendarEvent(state.visibleEvents);
		});
	});

	$effect(() => {
		const refresh = calendarRefresh.ticks;
		if (refresh === lastRefresh) return;
		lastRefresh = refresh;
		untrack(() => void eventLoader.refreshCurrentRange());
	});

	$effect(() => {
		const visibleDate = state.toolbarDate;
		const windowKey = `${visibleDate.getFullYear()}-${visibleDate.getMonth()}-${currentLocale.value}`;
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
		text
	});

	onMount(() => {
		const releaseRefresh = embedded ? () => {} : pageActions.setRefresh(eventLoader.refreshCurrentRange);
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
			isDisposed = true;
			releaseRefresh();
			eventLoader.invalidatePendingLoad();
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
		if (embedded) requestCalendarSettingsOpen();
		else openSettings?.();
	}


	async function loadParticipantCandidates(): Promise<void> {
		try {
			const [candidates, viewerEmail] = await Promise.all([fetchCalendarParticipants(), signedInEmail()]);
			if (isDisposed) return;
			state.participantCandidates = calendarParticipantsWithViewerFirst(candidates, viewerEmail);
			state.viewerParticipants = calendarViewerParticipants(candidates, viewerEmail);
		} catch {
			state.participantCandidates = [];
		}
	}
	function handleShortcut(event: KeyboardEvent) {
		if (!embedded) return;
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
	loadErrorMessage={state.loadErrorMessage}
	isLoading={state.isLoading}
	isInitialLoading={state.isInitialLoading}
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

{#if !embedded}
	<AppFloatingActionButton label={text.newEvent} onclick={() => draftPopoverActions.createQuickDraftPopover(null)}>
		<PlusIcon />
	</AppFloatingActionButton>
{/if}
