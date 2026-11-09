import type { Action } from 'svelte/action';

export type TaskRoadmapDragOptions = {
	enabled: boolean;
	dayWidth: () => number;
	onPreview: (dayOffset: number) => void;
	onCommit: (dayOffset: number) => void;
	onCancel: () => void;
};

const dragThresholdPixels = 4;

export const taskRoadmapDrag: Action<HTMLElement, TaskRoadmapDragOptions> = (element, initialOptions) => {
	let options = initialOptions;
	let pointerID: number | undefined;
	let originX = 0;
	let dragged = false;
	let dayOffset = 0;

	function pointerDown(event: PointerEvent): void {
		if (!options.enabled || event.button !== 0) return;
		pointerID = event.pointerId;
		originX = event.clientX;
		dragged = false;
		dayOffset = 0;
		element.setPointerCapture(event.pointerId);
	}

	function pointerMove(event: PointerEvent): void {
		if (event.pointerId !== pointerID) return;
		const deltaX = event.clientX - originX;
		if (!dragged && Math.abs(deltaX) < dragThresholdPixels) return;
		dragged = true;
		const next = Math.round(deltaX / Math.max(1, options.dayWidth()));
		if (next === dayOffset) return;
		dayOffset = next;
		options.onPreview(dayOffset);
	}

	function pointerUp(event: PointerEvent): void {
		if (event.pointerId !== pointerID) return;
		pointerID = undefined;
		if (!dragged) return;
		element.addEventListener('click', swallowClick, { capture: true, once: true });
		if (dayOffset === 0) options.onCancel();
		else options.onCommit(dayOffset);
	}

	function pointerCancel(event: PointerEvent): void {
		if (event.pointerId !== pointerID) return;
		pointerID = undefined;
		if (dragged) options.onCancel();
	}

	function swallowClick(event: MouseEvent): void {
		event.preventDefault();
		event.stopPropagation();
	}

	element.addEventListener('pointerdown', pointerDown);
	element.addEventListener('pointermove', pointerMove);
	element.addEventListener('pointerup', pointerUp);
	element.addEventListener('pointercancel', pointerCancel);

	return {
		update(nextOptions) {
			options = nextOptions;
		},
		destroy() {
			element.removeEventListener('pointerdown', pointerDown);
			element.removeEventListener('pointermove', pointerMove);
			element.removeEventListener('pointerup', pointerUp);
			element.removeEventListener('pointercancel', pointerCancel);
		}
	};
};
