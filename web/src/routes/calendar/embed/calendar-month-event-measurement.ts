import type { Event as DayFlowEvent } from '@dayflow/core';
import { ViewType } from '@dayflow/svelte';
import {
	measureMonthEventPlacements,
	type MonthEventPlacement,
	type MonthMorePlacement
} from './calendar-month-event-geometry';

type MonthEventMeasurementContext = {
	getActiveMorePlacementID: () => string | null;
	getEvents: () => DayFlowEvent[];
	getStageElement: () => HTMLElement | null;
	getToolbarView: () => ViewType;
	clearActiveMorePlacement: () => void;
	setMorePlacements: (placements: MonthMorePlacement[]) => void;
	setPlacements: (placements: MonthEventPlacement[]) => void;
};

export type MonthEventMeasurementController = {
	observeStage: () => () => void;
	queueMeasurement: () => void;
	stop: () => void;
};

export function createMonthEventMeasurementController(context: MonthEventMeasurementContext): MonthEventMeasurementController {
	let measurementFrame: number | null = null;

	function queueMeasurement(): void {
		if (typeof requestAnimationFrame === 'undefined') return;
		if (measurementFrame !== null) cancelAnimationFrame(measurementFrame);
		measurementFrame = requestAnimationFrame(() => {
			measurementFrame = null;
			measureEvents();
		});
	}

	function measureScrolledEvents(): void {
		measureEvents();
		queueMeasurement();
	}

	function observeStage(): () => void {
		const stageElement = context.getStageElement();
		if (!stageElement) return () => {};
		const observer = new MutationObserver(() => queueMeasurement());
		observer.observe(stageElement, { childList: true, subtree: true, attributes: true });
		const scroller = stageElement.querySelector<HTMLElement>('.df-month-view-virtual-scroller');
		scroller?.addEventListener('scroll', measureScrolledEvents, { passive: true });
		stageElement.addEventListener('scroll', measureScrolledEvents, true);
		document.addEventListener('scroll', measureScrolledEvents, true);
		window.addEventListener('resize', queueMeasurement);
		queueMeasurement();
		return () => {
			observer.disconnect();
			scroller?.removeEventListener('scroll', measureScrolledEvents);
			stageElement.removeEventListener('scroll', measureScrolledEvents, true);
			document.removeEventListener('scroll', measureScrolledEvents, true);
			window.removeEventListener('resize', queueMeasurement);
		};
	}

	function stop(): void {
		if (measurementFrame === null) return;
		cancelAnimationFrame(measurementFrame);
		measurementFrame = null;
	}

	function measureEvents(): void {
		if (context.getToolbarView() !== ViewType.MONTH) {
			context.setPlacements([]);
			context.setMorePlacements([]);
			return;
		}
		const placementResult = measureMonthEventPlacements(context.getStageElement(), context.getEvents());
		context.setPlacements(placementResult.placements);
		context.setMorePlacements(placementResult.morePlacements);
		if (shouldClearActiveMorePlacement(placementResult.morePlacements)) context.clearActiveMorePlacement();
	}

	function shouldClearActiveMorePlacement(morePlacements: MonthMorePlacement[]): boolean {
		const activeMorePlacementID = context.getActiveMorePlacementID();
		return Boolean(activeMorePlacementID && !morePlacements.some((placement) => placement.id === activeMorePlacementID));
	}

	return {
		observeStage,
		queueMeasurement,
		stop
	};
}
