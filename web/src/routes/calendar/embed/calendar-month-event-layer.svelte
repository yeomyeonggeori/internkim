<script lang="ts">
	import type { Event as DayFlowEvent } from '@dayflow/core';
	import { ViewType } from '@dayflow/svelte';
	import { onMount } from 'svelte';
	import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
	import { calendarEventAnchorFromElement } from './calendar-event-anchor-capture';
	import {
		type MonthEventPlacement,
		type MonthMorePlacement
	} from './calendar-month-event-geometry';
	import {
		createMonthEventDragController,
		eventsWithMonthDragPreview,
		type MonthEventDragState
	} from './calendar-month-event-drag';
	import { createMonthEventMeasurementController } from './calendar-month-event-measurement';
	import { monthEventPlacementStyle } from './calendar-month-event-layer-format';
	import CalendarMonthMoreLayer from './calendar-month-more-layer.svelte';
	import { installMonthMoreDismiss } from './calendar-month-more-dismiss';

	type CalendarMonthEventLayerProps = {
		events: DayFlowEvent[];
		clearSelectedEvent: () => void;
		localeCode: string;
		monthMoreText: {
			ariaLabel: string;
			button: string;
		};
		openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
		saveMovedEvent: (event: DayFlowEvent) => void | Promise<void>;
		selectEvent: (eventID: string) => void;
		selectDate: (dateKey: string) => void;
		selectedEventID: string | null;
		stageElement: HTMLElement | null;
		toolbarView: ViewType;
	};

	let {
		events,
		clearSelectedEvent,
		localeCode,
		monthMoreText,
		openEvent,
		saveMovedEvent,
		selectEvent,
		selectDate,
		selectedEventID,
		stageElement,
		toolbarView
	}: CalendarMonthEventLayerProps = $props();

	let placements = $state<MonthEventPlacement[]>([]);
	let morePlacements = $state<MonthMorePlacement[]>([]);
	let activeMorePlacementID = $state<string | null>(null);
	let dragState = $state<MonthEventDragState | null>(null);
	let suppressedEventActivationUntil = 0;

	const layerEvents = $derived(eventsWithMonthDragPreview(events, dragState));
	const activeMorePlacement = $derived(morePlacements.find((placement) => placement.id === activeMorePlacementID) ?? null);
	const dragController = createMonthEventDragController({
		clearSelectedEvent: () => clearSelectedEvent(),
		getDragState: () => dragState,
		getStageElement: () => stageElement,
		saveMovedEvent: (event) => saveMovedEvent(event),
		setDragState: (state) => {
			dragState = state;
		},
		suppressNextClick: () => {
			suppressedEventActivationUntil = Date.now() + 450;
		}
	});
	const measurementController = createMonthEventMeasurementController({
		getActiveMorePlacementID: () => activeMorePlacementID,
		getEvents: () => layerEvents,
		getStageElement: () => stageElement,
		getToolbarView: () => toolbarView,
		clearActiveMorePlacement: () => {
			activeMorePlacementID = null;
		},
		setMorePlacements: (nextMorePlacements) => {
			morePlacements = nextMorePlacements;
		},
		setPlacements: (nextPlacements) => {
			placements = nextPlacements;
		}
	});

	$effect(() => {
		stageElement;
		toolbarView;
		layerEvents;
		measurementController.queueMeasurement();
	});

	$effect(() => {
		stageElement;
		return measurementController.observeStage();
	});

	$effect(() => {
		activeMorePlacementID;
		return installMonthMoreDismiss({
			isActive: () => Boolean(activeMorePlacementID),
			clearActiveMorePlacement: () => {
				activeMorePlacementID = null;
			}
		});
	});

	onMount(() => {
		measurementController.queueMeasurement();
		return () => {
			measurementController.stop();
			dragController.stop();
		};
	});

	function handlePointerDown(pointerEvent: PointerEvent, placement: MonthEventPlacement): void {
		if (pointerEvent.button !== 0) return;
		const event = events.find((calendarEvent) => calendarEvent.id === placement.eventID);
		if (!event) return;
		selectDate(placement.startDateKey);
		selectEvent(placement.eventID);
		dragController.startDrag(pointerEvent, event);
	}

	function handleClick(mouseEvent: MouseEvent, placement: MonthEventPlacement): void {
		mouseEvent.preventDefault();
		mouseEvent.stopPropagation();
		if (shouldSuppressEventActivation()) return;
		selectDate(placement.startDateKey);
		selectEvent(placement.eventID);
	}

	function handleDoubleClick(mouseEvent: MouseEvent, placement: MonthEventPlacement): void {
		mouseEvent.preventDefault();
		mouseEvent.stopPropagation();
		if (shouldSuppressEventActivation()) return;
		if (!(mouseEvent.currentTarget instanceof HTMLElement)) return;
		selectDate(placement.startDateKey);
		selectEvent(placement.eventID);
		openEvent(placement.eventID, calendarEventAnchorFromElement(mouseEvent.currentTarget));
	}

	function handleMoreButtonClick(mouseEvent: MouseEvent, placement: MonthMorePlacement): void {
		mouseEvent.preventDefault();
		mouseEvent.stopPropagation();
		activeMorePlacementID = activeMorePlacementID === placement.id ? null : placement.id;
	}

	function handleMoreEventClick(mouseEvent: MouseEvent, segment: MonthMorePlacement['hiddenSegments'][number]): void {
		mouseEvent.preventDefault();
		mouseEvent.stopPropagation();
		selectDate(segment.startDateKey);
		selectEvent(segment.eventID);
	}

	function handleMoreEventDoubleClick(mouseEvent: MouseEvent, segment: MonthMorePlacement['hiddenSegments'][number]): void {
		mouseEvent.preventDefault();
		mouseEvent.stopPropagation();
		if (!(mouseEvent.currentTarget instanceof HTMLElement)) return;
		selectDate(segment.startDateKey);
		selectEvent(segment.eventID);
		openEvent(segment.eventID, calendarEventAnchorFromElement(mouseEvent.currentTarget));
	}

	function shouldSuppressEventActivation(): boolean {
		if (Date.now() <= suppressedEventActivationUntil) return true;
		suppressedEventActivationUntil = 0;
		return false;
	}

</script>

{#if toolbarView === ViewType.MONTH}
	<div class="calendar-month-event-layer" aria-hidden="false">
		{#each placements as placement (placement.id)}
			<button
				type="button"
				class="df-event df-month-event calendar-month-direct-event"
				class:df-event-all-day={placement.isAllDay}
				class:df-event-timed={!placement.isAllDay}
				class:internkim-calendar-event-focused={placement.eventID === selectedEventID}
				data-calendar-month-event-id={placement.id}
				data-event-id={placement.eventID}
				style={monthEventPlacementStyle(placement)}
				aria-label={placement.titleText}
				onclick={(event) => handleClick(event, placement)}
				ondblclick={(event) => handleDoubleClick(event, placement)}
				onpointerdown={(event) => handlePointerDown(event, placement)}
			>
				<span class="calendar-event-content calendar-month-event-content">
					<span class="calendar-event-title calendar-month-event-title">{placement.titleText}</span>
				</span>
			</button>
		{/each}
		<CalendarMonthMoreLayer
			{activeMorePlacement}
			{localeCode}
			{monthMoreText}
			{morePlacements}
			{handleMoreButtonClick}
			{handleMoreEventClick}
			{handleMoreEventDoubleClick}
		/>
	</div>
{/if}
