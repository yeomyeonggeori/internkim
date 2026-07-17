export type DraftPopoverAnchor = {
	clientX: number;
	clientY: number;
	originElement?: HTMLElement;
	leftClientX?: number;
	rightClientX?: number;
	topClientY?: number;
	bottomClientY?: number;
	titleEndClientX?: number;
	titleTopClientY?: number;
	titleBottomClientY?: number;
	preferredSide?: 'left' | 'right';
};

export type DraftPopoverPosition = {
	left: number;
	top: number;
	width: number;
	arrowTop: number;
	arrowSide: 'left' | 'right';
	isReady: boolean;
};

export type DraftPopoverSize = {
	width?: number;
	height?: number;
};

type PopoverBounds = {
	left: number;
	top: number;
	right: number;
	bottom: number;
	width: number;
	height: number;
};

const estimatedDraftPopoverHeight = 552;
const preferredDraftPopoverWidth = 400;
const defaultPopoverGap = 16;
const eventPopoverGap = 24;

export function draftPopoverPositionFromAnchor(
	anchor: DraftPopoverAnchor | null,
	stageElement: HTMLElement | null,
	size: DraftPopoverSize = {}
): DraftPopoverPosition {
	const stageRectangle = stageElement?.getBoundingClientRect();
	if (!stageRectangle) return { left: 16, top: 16, width: 320, arrowTop: 42, arrowSide: 'left', isReady: true };
	const visibleRectangle = visibleStageRectangle(stageRectangle);
	const stageMargin = 12;
	const preferredWidth = Math.max(1, Math.min(preferredDraftPopoverWidth, visibleRectangle.width - stageMargin * 2));
	const measuredHeight = size.height ?? estimatedDraftPopoverHeight;
	const height = Math.max(1, Math.min(measuredHeight, visibleRectangle.height - stageMargin * 2));
	const isMeasuredHeight = size.height !== undefined;
	const fallbackAnchor: DraftPopoverAnchor = {
		clientX: visibleRectangle.left + visibleRectangle.width / 2,
		clientY: visibleRectangle.top + Math.min(220, visibleRectangle.height / 3)
	};
	const targetAnchor = anchor ?? fallbackAnchor;
	if (!isEventBlockAnchor(targetAnchor)) {
		return genericDraftPopoverPosition(targetAnchor, visibleRectangle, preferredWidth, height, stageMargin, defaultPopoverGap);
	}
	return eventDraftPopoverPosition(targetAnchor, visibleRectangle, preferredWidth, height, isMeasuredHeight, stageMargin, eventPopoverGap);
}

function genericDraftPopoverPosition(
	targetAnchor: DraftPopoverAnchor,
	visibleRectangle: PopoverBounds,
	width: number,
	height: number,
	stageMargin: number,
	popoverGap: number
): DraftPopoverPosition {
	const targetLeft = targetAnchor.leftClientX ?? targetAnchor.clientX;
	const targetRight = targetAnchor.rightClientX ?? targetAnchor.clientX;
	const anchorTop = targetAnchor.topClientY ?? targetAnchor.clientY - 8;
	const anchorBottom = targetAnchor.bottomClientY ?? targetAnchor.clientY + 8;
	const anchorCenterY = (anchorTop + anchorBottom) / 2;
	const rightAvailableWidth = Math.max(0, visibleRectangle.right - stageMargin - (targetRight + popoverGap));
	const leftAvailableWidth = Math.max(0, targetLeft - popoverGap - (visibleRectangle.left + stageMargin));
	const shouldPlaceRight =
		targetAnchor.preferredSide === 'right' ||
		(targetAnchor.preferredSide !== 'left' && (rightAvailableWidth >= width || rightAvailableWidth >= leftAvailableWidth));
	const arrowSide = shouldPlaceRight ? 'left' : 'right';
	const unclampedLeft = shouldPlaceRight ? targetRight + popoverGap : targetLeft - width - popoverGap;
	const minLeft = visibleRectangle.left + stageMargin;
	const maxLeft = visibleRectangle.right - width - stageMargin;
	const left = Math.max(minLeft, Math.min(unclampedLeft, Math.max(minLeft, maxLeft)));
	const top = popoverTopForAnchor(anchorCenterY, visibleRectangle, height, stageMargin);
	const arrowTop = Math.max(18, Math.min(anchorCenterY - top - 8, height - 28));
	return { left, top, width, arrowTop, arrowSide, isReady: true };
}

function eventDraftPopoverPosition(
	targetAnchor: DraftPopoverAnchor,
	visibleRectangle: PopoverBounds,
	preferredWidth: number,
	height: number,
	isMeasuredHeight: boolean,
	stageMargin: number,
	popoverGap: number
): DraftPopoverPosition {
	const titleTop = targetAnchor.titleTopClientY ?? targetAnchor.topClientY ?? targetAnchor.clientY;
	const titleBottom = targetAnchor.titleBottomClientY ?? targetAnchor.bottomClientY ?? targetAnchor.clientY;
	const titleCenterY = (titleTop + titleBottom) / 2;
	const targetLeft = targetAnchor.leftClientX ?? targetAnchor.clientX;
	const targetRight = targetAnchor.titleEndClientX ?? targetAnchor.rightClientX ?? targetAnchor.clientX;
	const rightAvailableWidth = Math.max(0, visibleRectangle.right - stageMargin - (targetRight + popoverGap));
	const leftAvailableWidth = Math.max(0, targetLeft - popoverGap - (visibleRectangle.left + stageMargin));
	const rightFitsPreferredWidth = rightAvailableWidth >= preferredWidth;
	const leftFitsPreferredWidth = leftAvailableWidth >= preferredWidth;
	const shouldPlaceRight =
		targetAnchor.preferredSide === 'right' ||
		(targetAnchor.preferredSide !== 'left' &&
			(rightFitsPreferredWidth || (!leftFitsPreferredWidth && rightAvailableWidth >= leftAvailableWidth)));
	const sideAvailableWidth = shouldPlaceRight ? rightAvailableWidth : leftAvailableWidth;
	const width = Math.max(1, Math.min(preferredWidth, sideAvailableWidth || preferredWidth));
	const arrowSide = shouldPlaceRight ? 'left' : 'right';
	const unclampedLeft = shouldPlaceRight ? targetRight + popoverGap : targetLeft - width - popoverGap;
	const minLeft = visibleRectangle.left + stageMargin;
	const maxLeft = visibleRectangle.right - width - stageMargin;
	const left = Math.max(minLeft, Math.min(unclampedLeft, Math.max(minLeft, maxLeft)));
	const top = isMeasuredHeight || isYInsideBounds(titleCenterY, visibleRectangle)
		? popoverTopForAnchor(titleCenterY, visibleRectangle, height, stageMargin)
		: titleCenterY - 48;
	const arrowTop = Math.max(18, Math.min(titleCenterY - top - 8, height - 28));
	return { left, top, width, arrowTop, arrowSide, isReady: true };
}

function popoverTopForAnchor(anchorY: number, visibleRectangle: PopoverBounds, height: number, stageMargin: number): number {
	const sideAlignedTop = anchorY - 48;
	const minTop = visibleRectangle.top + stageMargin;
	const maxTop = visibleRectangle.bottom - height - stageMargin;
	const minimumBoundedMaxTop = Math.max(minTop, maxTop);
	return Math.max(minTop, Math.min(sideAlignedTop, minimumBoundedMaxTop));
}

function isEventBlockAnchor(anchor: DraftPopoverAnchor): boolean {
	return anchor.rightClientX !== undefined && anchor.titleTopClientY !== undefined && anchor.titleBottomClientY !== undefined;
}

function isYInsideBounds(value: number, bounds: PopoverBounds): boolean {
	return bounds.top <= value && value <= bounds.bottom;
}

function visibleStageRectangle(stageRectangle: DOMRect): PopoverBounds {
	const viewportRectangle = viewportBounds();
	if (!viewportRectangle) return stageRectangle;
	const left = Math.max(stageRectangle.left, viewportRectangle.left);
	const top = Math.max(stageRectangle.top, viewportRectangle.top);
	const right = Math.min(stageRectangle.right, viewportRectangle.right);
	const bottom = Math.min(stageRectangle.bottom, viewportRectangle.bottom);
	if (right <= left || bottom <= top) return stageRectangle;
	return {
		left,
		top,
		right,
		bottom,
		width: right - left,
		height: bottom - top
	};
}

function viewportBounds(): PopoverBounds | null {
	if (typeof window === 'undefined') return null;
	const width = window.innerWidth;
	const height = window.innerHeight;
	if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return null;
	return {
		left: 0,
		top: 0,
		right: width,
		bottom: height,
		width,
		height
	};
}
