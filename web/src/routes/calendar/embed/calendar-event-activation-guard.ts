import { hasCalendarEventGestureMoved } from './calendar-event-gesture';

export type CalendarEventActivationGuard = {
	shouldSuppressActivation: (event: MouseEvent) => boolean;
	destroy: () => void;
};

type MovedEventActivation = {
	eventID: string;
	expiresAt: number;
};

type PointerEventGesture = {
	eventID: string;
	pointerID: number;
	startClientX: number;
	startClientY: number;
	hasMoved: boolean;
};

const activationSuppressionDurationMs = 800;

export function createCalendarEventActivationGuard(stageElement: HTMLElement): CalendarEventActivationGuard {
	let gesture: PointerEventGesture | null = null;
	let movedActivation: MovedEventActivation | null = null;

	const handlePointerDown = (event: PointerEvent): void => {
		const eventID = eventIDFromTarget(stageElement, event.target);
		if (!eventID) {
			gesture = null;
			return;
		}
		gesture = {
			eventID,
			pointerID: event.pointerId,
			startClientX: event.clientX,
			startClientY: event.clientY,
			hasMoved: false
		};
	};

	const handlePointerMove = (event: PointerEvent): void => {
		if (!gesture || gesture.pointerID !== event.pointerId) return;
		if (gesture.hasMoved) return;
		gesture = {
			...gesture,
			hasMoved: hasCalendarEventGestureMoved(
				{ clientX: gesture.startClientX, clientY: gesture.startClientY },
				event
			)
		};
	};

	const handlePointerEnd = (event: PointerEvent): void => {
		if (!gesture || gesture.pointerID !== event.pointerId) return;
		const hasMoved =
			gesture.hasMoved ||
			hasCalendarEventGestureMoved(
				{ clientX: gesture.startClientX, clientY: gesture.startClientY },
				event
			);
		if (hasMoved) {
			movedActivation = {
				eventID: gesture.eventID,
				expiresAt: Date.now() + activationSuppressionDurationMs
			};
		}
		gesture = null;
	};

	stageElement.addEventListener('pointerdown', handlePointerDown, true);
	document.addEventListener('pointermove', handlePointerMove);
	window.addEventListener('pointerup', handlePointerEnd, true);
	window.addEventListener('pointercancel', handlePointerEnd, true);

	return {
		shouldSuppressActivation(event) {
			const eventID = eventIDFromTarget(stageElement, event.target);
			if (!eventID || !movedActivation) return false;
			if (Date.now() > movedActivation.expiresAt) {
				movedActivation = null;
				return false;
			}
			const shouldSuppress = movedActivation.eventID === eventID;
			if (shouldSuppress) movedActivation = null;
			return shouldSuppress;
		},
		destroy() {
			stageElement.removeEventListener('pointerdown', handlePointerDown, true);
			document.removeEventListener('pointermove', handlePointerMove);
			window.removeEventListener('pointerup', handlePointerEnd, true);
			window.removeEventListener('pointercancel', handlePointerEnd, true);
		}
	};
}

function eventIDFromTarget(stageElement: HTMLElement, target: EventTarget | null): string | null {
	if (!(target instanceof Element)) return null;
	const eventElement = target.closest<HTMLElement>('[data-event-id].df-event, .calendar-multi-day-all-day-proxy[data-event-id]');
	if (!eventElement || !stageElement.contains(eventElement)) return null;
	if (eventElement.classList.contains('calendar-month-direct-event')) return null;
	return normalizedEventID(eventElement.dataset.eventId);
}

function normalizedEventID(eventID: string | undefined): string | null {
	if (!eventID) return null;
	return eventID.split('::')[0] ?? null;
}
