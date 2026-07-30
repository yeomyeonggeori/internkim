<script lang="ts">
	import { setContext } from 'svelte';
	import * as ContextMenu from '$lib/components/ui/context-menu';
	import CalendarPlusIcon from '@lucide/svelte/icons/calendar-plus';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import TrashIcon from '@lucide/svelte/icons/trash';
	import { ViewType } from '../calendar-view-type';
	import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
	import type { CalendarLocaleText } from '../text';
	import type { CalendarParticipant } from './calendar-participants';
	import CalendarMonthView from '../grid/calendar-month-view.svelte';
	import CalendarTimeView from '../grid/calendar-time-view.svelte';
	import { calendarGridEventsFromDayFlowEvents } from '../grid/calendar-grid-events';
	import { calendarGridDateKey } from '../grid/calendar-grid-dates';
	import type { CalendarGridEvent } from '../grid/calendar-grid-layout';
	import CalendarMonthRangePreview from './calendar-month-range-preview.svelte';
	import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
	import type { MonthRangePreviewSegment } from './calendar-month-range-action';
	import { dateKeyFromWeekHeaderTarget } from './calendar-month-selection';
	import { calendarMobileTwoDayWeekDateKeyForColumn } from './calendar-mobile-two-day-week';
	import {
		mobileEventEditorActivationContextKey,
		mobileEventEditorLocaleContextKey,
		mobileEventEditorParticipantsContextKey,
		mobileEventEditorPersistenceContextKey,
		type MobileEventEditorActivationContext,
		type MobileEventEditorLocaleContext,
		type MobileEventEditorParticipantsContext,
		type MobileEventEditorPersistenceContext
	} from './calendar-mobile-event-editor-types';
	import CalendarTimelineRangePreview from './calendar-timeline-range-preview.svelte';
	import type { TimelineRangePreviewSegment } from './calendar-timeline-preview';

	type MonthScrollOverlayLabel = {
		id: string;
		text: string;
		top: number;
		direction: 1 | -1;
		isFading: boolean;
	};

	type CalendarStageProps = {
		activeMobileEditorEventID: string | null;
		clearActiveMobileEditorEvent: (eventID: string) => void;
		clearSelectedEvent: () => void;
		events: DayFlowEvent[];
		participantCandidates: CalendarParticipant[];
		isMobileTwoDayWeekView: boolean;
		localeCode: string;
		text: CalendarLocaleText;
		monthMoreText: {
			ariaLabel: string;
			button: string;
		};
		monthRangePreviewSegments: MonthRangePreviewSegment[];
		monthRangePreviewTitle: string;
		editingEvent: Pick<CalendarGridEvent, 'id' | 'title' | 'start' | 'end' | 'isAllDay'> | null;
		navigateToDateKey: (dateKey: string) => void;
		selectedMonthDateKey: string | null;
		visibleMonthChanged: (month: Date) => void;
		addEventOnDay: (dateKey: string) => void;
		addEventOnRange: (startDateKey: string, endDateKey: string) => void;
		addEventOnTimeRange: (start: Date, end: Date) => void;
		deleteEvent: (eventID: string) => void;
		openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
		saveMovedEvent: (event: DayFlowEvent) => void | Promise<void>;
		selectDate: (dateKey: string) => void;
		selectedEventID: string | null;
		timelineRangePreviewSegments: TimelineRangePreviewSegment[];
		timelineRangePreviewTitle: string;
		toolbarView: ViewType;
		toolbarDate: Date;
		stageElement?: HTMLElement | null;
	};

	let {
		activeMobileEditorEventID,
		clearActiveMobileEditorEvent,
		clearSelectedEvent,
		events,
		participantCandidates,
		isMobileTwoDayWeekView,
		localeCode,
		text,
		monthMoreText,
		monthRangePreviewSegments,
		monthRangePreviewTitle,
		editingEvent,
		navigateToDateKey,
		selectedMonthDateKey,
		visibleMonthChanged,
		addEventOnDay,
		addEventOnRange,
		addEventOnTimeRange,
		deleteEvent,
		openEvent,
		saveMovedEvent,
		selectDate,
		selectedEventID,
		timelineRangePreviewSegments,
		timelineRangePreviewTitle,
		toolbarView,
		toolbarDate,
		stageElement = $bindable<HTMLElement | null>(null)
	}: CalendarStageProps = $props();

	setContext<MobileEventEditorActivationContext>(mobileEventEditorActivationContextKey, {
		getActiveEventID: () => activeMobileEditorEventID,
		getStageElement: () => stageElement,
		clearActiveEvent: (eventID) => clearActiveMobileEditorEvent(eventID)
	});

	setContext<MobileEventEditorPersistenceContext>(mobileEventEditorPersistenceContextKey, {
		saveEvent: (event) => saveMovedEvent(event)
	});
	setContext<MobileEventEditorLocaleContext>(mobileEventEditorLocaleContextKey, {
		getText: () => text
	});
	setContext<MobileEventEditorParticipantsContext>(mobileEventEditorParticipantsContextKey, {
		getCandidates: () => participantCandidates
	});

	function handleMobileTwoDayWeekDateClick(event: MouseEvent): void {
		if (!isMobileTwoDayWeekView || !stageElement) return;
		if (!(event.target instanceof HTMLElement)) return;
		const dateButton = event.target.closest<HTMLElement>('.df-compact-header-date-button');
		if (!dateButton || !stageElement.contains(dateButton)) return;
		const dateButtons = Array.from(stageElement.querySelectorAll<HTMLElement>('.df-compact-header-date-button'));
		const columnIndex = dateButtons.indexOf(dateButton);
		if (columnIndex < 0) return;
		navigateToDateKey(calendarMobileTwoDayWeekDateKeyForColumn(toolbarDate, columnIndex));
	}

	function handleWeekHeaderDateClick(event: MouseEvent): void {
		if (toolbarView !== ViewType.WEEK || isMobileTwoDayWeekView || !stageElement) return;
		if (!(event.target instanceof Element)) return;
		const dateKey = dateKeyFromWeekHeaderTarget(stageElement, event.target, toolbarDate);
		if (!dateKey) return;
		navigateToDateKey(dateKey);
	}

	const replayedContextMenuEventKey = 'calendarStageReplayedContextMenu';

	const gridEvents = $derived(
		calendarGridEventsFromDayFlowEvents(events).map((event) =>
			event.id === editingEvent?.id ? { ...event, ...editingEvent } : event
		)
	);
	function openGridEvent(event: CalendarGridEvent, originElement: HTMLElement): void {
		const rectangle = originElement.getBoundingClientRect();
		openEvent(event.id, {
			clientX: rectangle.right,
			clientY: rectangle.top,
			originElement,
			leftClientX: rectangle.left,
			topClientY: rectangle.top,
			bottomClientY: rectangle.bottom
		});
	}

	let contextTarget = $state<{ dateKey: string; eventID: string; anchor: DraftPopoverAnchor } | null>(null);

	const contextEventTitle = $derived(events.find((event) => event.id === contextTarget?.eventID)?.title ?? '');
	const contextDayLabel = $derived(
		contextTarget?.dateKey
			? new Date(`${contextTarget.dateKey}T12:00:00`).toLocaleDateString(localeCode, { month: 'long', day: 'numeric' })
			: ''
	);

	function captureContextTarget(event: MouseEvent): void {
		if (!(event.target instanceof Element) || isReplayedContextMenuEvent(event)) return;
		const eventElement = event.target.closest<HTMLElement>('[data-calendar-event-id], [data-event-id], .df-event, .df-month-segment-event');
		const dayElement = event.target.closest<HTMLElement>('[data-calendar-date], [data-date]');
		contextTarget = {
			eventID: eventElement?.dataset.calendarEventId ?? eventElement?.dataset.eventId ?? '',
			dateKey: dayElement?.dataset.calendarDate ?? dayElement?.dataset.date ?? '',
			anchor: { clientX: event.clientX, clientY: event.clientY }
		};
		if (!contextTarget.eventID && !contextTarget.dateKey) return;
		event.preventDefault();
		event.stopPropagation();
		replayContextMenuEventOnStage(event);
	}

	function replayContextMenuEventOnStage(event: MouseEvent): void {
		const currentStageElement = stageElement;
		if (!currentStageElement) return;
		const replayedEvent = new MouseEvent('contextmenu', {
			bubbles: true,
			cancelable: true,
			clientX: event.clientX,
			clientY: event.clientY,
			button: event.button
		});
		Object.defineProperty(replayedEvent, replayedContextMenuEventKey, { value: true });
		currentStageElement.dispatchEvent(replayedEvent);
	}

	function isReplayedContextMenuEvent(event: MouseEvent): boolean {
		return replayedContextMenuEventKey in event;
	}

	function handleStageDateClick(event: MouseEvent): void {
		handleMobileTwoDayWeekDateClick(event);
		handleWeekHeaderDateClick(event);
	}

	$effect(() => {
		const currentStageElement = stageElement;
		if (!currentStageElement) return;
		currentStageElement.addEventListener('click', handleStageDateClick, true);
		currentStageElement.addEventListener('contextmenu', captureContextTarget, true);
		return () => {
			currentStageElement.removeEventListener('click', handleStageDateClick, true);
			currentStageElement.removeEventListener('contextmenu', captureContextTarget, true);
		};
	});

	import './calendar-stage.css';
</script>

<ContextMenu.Root>
	<ContextMenu.Trigger>
		{#snippet child({ props })}
			<div
				{...props}
				bind:this={stageElement}
				class="calendar-stage"
					class:calendar-stage-day={toolbarView === ViewType.DAY}
					class:calendar-stage-week={toolbarView === ViewType.WEEK}
					class:calendar-stage-month={toolbarView === ViewType.MONTH}
					class:calendar-stage-mobile-two-day-week={isMobileTwoDayWeekView}
					tabindex="-1"
					role="region"
					aria-label={text.pageTitle}
				>
					{#if toolbarView === ViewType.MONTH}
						<div class="absolute inset-0 flex min-h-0 flex-col">
						<CalendarMonthView
							visibleDate={toolbarDate}
							selectedDateKey={selectedMonthDateKey ?? ''}
							selectedEventID={selectedEventID ?? ''}
							events={gridEvents}
							{localeCode}
							moreEventsText={monthMoreText.button}
							draftPreviewTitle={text.newEvent}
							selectDay={(day) => selectDate(calendarGridDateKey(day))}
							openDay={(day) => navigateToDateKey(calendarGridDateKey(day))}
							addEventOnDay={(day) => addEventOnDay(calendarGridDateKey(day))}
							addEventOnRange={(startDateKey, endDateKey) => addEventOnRange(startDateKey, endDateKey)}
							openEvent={openGridEvent}
							visibleMonthChanged={(month) => visibleMonthChanged(month)}
						/>
						</div>
					{:else}
						<div class="absolute inset-0 flex min-h-0 flex-col">
							<CalendarTimeView
								visibleDate={toolbarDate}
								dayCount={toolbarView === ViewType.DAY ? 1 : 7}
								selectedEventID={selectedEventID ?? ''}
								events={gridEvents}
								{localeCode}
								draftPreviewTitle={text.newEvent}
								selectDay={(day) => navigateToDateKey(calendarGridDateKey(day))}
								openEvent={openGridEvent}
								addEventOnTimeRange={(start, end) => addEventOnTimeRange(start, end)}
								addEventOnDayRange={(startDateKey, endDateKey) => addEventOnRange(startDateKey, endDateKey)}
							/>
						</div>
					{/if}
					<CalendarMonthRangePreview segments={monthRangePreviewSegments} title={monthRangePreviewTitle} />
				<CalendarTimelineRangePreview segments={timelineRangePreviewSegments} title={timelineRangePreviewTitle} />
			</div>
		{/snippet}
	</ContextMenu.Trigger>
	<ContextMenu.Content class="w-52">
		{#if contextTarget?.eventID}
			<ContextMenu.Label class="truncate">{contextEventTitle}</ContextMenu.Label>
			<ContextMenu.Item onclick={() => contextTarget && openEvent(contextTarget.eventID, contextTarget.anchor)}>
				<PencilIcon />
				{text.editEvent}
			</ContextMenu.Item>
			<ContextMenu.Separator />
			<ContextMenu.Item variant="destructive" onclick={() => contextTarget && deleteEvent(contextTarget.eventID)}>
				<TrashIcon />
				{text.deleteEvent}
			</ContextMenu.Item>
		{:else if contextTarget?.dateKey}
			<ContextMenu.Label>{contextDayLabel}</ContextMenu.Label>
			<ContextMenu.Item onclick={() => contextTarget && addEventOnDay(contextTarget.dateKey)}>
				<CalendarPlusIcon />
				{text.addEventOnDay}
			</ContextMenu.Item>
		{/if}
	</ContextMenu.Content>
</ContextMenu.Root>
