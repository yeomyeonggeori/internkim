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
