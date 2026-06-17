import {
	type DraftPopoverAnchor,
	type DraftPopoverMode,
	type DraftPopoverState
} from './calendar-draft-popover-state';
import {
	calendarEventAnchorFromElement,
	canonicalCalendarEventAnchorForID
} from './calendar-event-anchor-capture';
import { calendarEventElementsByID, isVisibleCalendarEventElement } from './calendar-event-elements';

export function shouldShowInitialPopover(mode: DraftPopoverMode, anchor: DraftPopoverAnchor | null): boolean {
	return mode === 'edit' || isEventBlockAnchor(anchor);
}

export function areSamePopoverPositions(
	firstPosition: DraftPopoverState['position'],
	secondPosition: DraftPopoverState['position']
): boolean {
	return (
		Math.abs(firstPosition.left - secondPosition.left) < 1 &&
		Math.abs(firstPosition.top - secondPosition.top) < 1 &&
		Math.abs(firstPosition.width - secondPosition.width) < 1 &&
		Math.abs(firstPosition.arrowTop - secondPosition.arrowTop) < 1 &&
		firstPosition.arrowSide === secondPosition.arrowSide &&
		firstPosition.isReady === secondPosition.isReady
	);
}

export function areSamePopoverAnchors(
	firstAnchor: DraftPopoverAnchor | null,
	secondAnchor: DraftPopoverAnchor | null
): boolean {
	if (!firstAnchor || !secondAnchor) return firstAnchor === secondAnchor;
	return (
		Math.abs(firstAnchor.clientX - secondAnchor.clientX) < 1 &&
		Math.abs(firstAnchor.clientY - secondAnchor.clientY) < 1 &&
		Math.abs((firstAnchor.leftClientX ?? 0) - (secondAnchor.leftClientX ?? 0)) < 1 &&
		Math.abs((firstAnchor.rightClientX ?? 0) - (secondAnchor.rightClientX ?? 0)) < 1 &&
		Math.abs((firstAnchor.titleEndClientX ?? 0) - (secondAnchor.titleEndClientX ?? 0)) < 1 &&
		Math.abs((firstAnchor.titleTopClientY ?? 0) - (secondAnchor.titleTopClientY ?? 0)) < 1 &&
		Math.abs((firstAnchor.titleBottomClientY ?? 0) - (secondAnchor.titleBottomClientY ?? 0)) < 1
	);
}

export function isAnchorInsideVisibleStage(
	stageElement: HTMLElement | null,
	anchor: DraftPopoverAnchor
): boolean {
	if (!stageElement) return true;
	const stageRectangle = stageElement.getBoundingClientRect();
	const left = Math.max(stageRectangle.left, 0);
	const top = Math.max(stageRectangle.top, 0);
	const right = Math.min(stageRectangle.right, window.innerWidth);
	const bottom = Math.min(stageRectangle.bottom, window.innerHeight);
	const anchorLeft = anchor.leftClientX ?? anchor.clientX;
	const anchorRight = anchor.rightClientX ?? anchor.titleEndClientX ?? anchor.clientX;
	const anchorTop = anchor.titleTopClientY ?? anchor.topClientY ?? anchor.clientY;
	const anchorBottom = anchor.titleBottomClientY ?? anchor.bottomClientY ?? anchor.clientY;
	return anchorRight >= left && anchorLeft <= right && anchorBottom >= top && anchorTop <= bottom;
}

export function clearDraftPopoverElementMotion(): void {
	const element = document.querySelector<HTMLElement>('.calendar-draft-popover');
	if (!element) return;
	element.style.transform = '';
	delete element.dataset.draftPopoverOffsetX;
	delete element.dataset.draftPopoverOffsetY;
}

export function calendarEventAnchorForEventID(
	stageElement: HTMLElement | null,
	eventID: string,
	anchor: DraftPopoverAnchor | null = null
): DraftPopoverAnchor | null {
	const canonicalAnchor = canonicalCalendarEventAnchorForID(stageElement, eventID);
	if (canonicalAnchor) return canonicalAnchor;
	const eventElement = calendarEventElementForAnchor(stageElement, eventID, anchor);
	return eventElement ? calendarEventAnchorFromElement(eventElement) : null;
}

export function genericEventAnchorForEventID(
	stageElement: HTMLElement | null,
	eventID: string
): DraftPopoverAnchor | null {
	const eventElement = calendarEventElementForAnchor(stageElement, eventID, null);
	return anchorFromElement(eventElement);
}

export function monthDateAnchor(stageElement: HTMLElement | null, dateKey: string): DraftPopoverAnchor | null {
	if (!stageElement) return null;
	const escapedDateKey = window.CSS?.escape(dateKey) ?? dateKey.replaceAll('"', '\\"');
	return anchorFromElement(stageElement.querySelector(`[data-date="${escapedDateKey}"]`));
}

export function anchorFromElement(element: EventTarget | Element | null): DraftPopoverAnchor | null {
	if (!(element instanceof Element)) return null;
	const rectangle = element.getBoundingClientRect();
	const titleHeight = Math.min(56, rectangle.height);
	return {
		clientX: rectangle.right - Math.min(18, rectangle.width / 2),
		clientY: rectangle.top + Math.min(40, rectangle.height / 2),
		leftClientX: rectangle.left,
		topClientY: rectangle.top,
		bottomClientY: rectangle.top + titleHeight
	};
}

function isEventBlockAnchor(anchor: DraftPopoverAnchor | null): boolean {
	return Boolean(anchor?.rightClientX !== undefined && anchor.titleTopClientY !== undefined && anchor.titleBottomClientY !== undefined);
}

function calendarEventElementForAnchor(
	stageElement: HTMLElement | null,
	eventID: string,
	anchor: DraftPopoverAnchor | null
): HTMLElement | null {
	const eventElements = calendarEventElementsByID(stageElement, eventID);
	const visibleEventElements = eventElements.filter(isVisibleCalendarEventElement);
	if (visibleEventElements.length === 0) return null;
	if (!anchor) return visibleEventElements[0] ?? null;
	return [...visibleEventElements].sort((firstElement, secondElement) => {
		return elementDistanceFromAnchor(firstElement, anchor) - elementDistanceFromAnchor(secondElement, anchor);
	})[0] ?? null;
}

function elementDistanceFromAnchor(element: HTMLElement, anchor: DraftPopoverAnchor): number {
	const rectangle = element.getBoundingClientRect();
	const centerX = Math.max(rectangle.left, Math.min(anchor.clientX, rectangle.right));
	const centerY = Math.max(rectangle.top, Math.min(anchor.clientY, rectangle.bottom));
	return Math.hypot(centerX - anchor.clientX, centerY - anchor.clientY);
}
