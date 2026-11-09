import type { Action } from 'svelte/action';

export type TaskRoadmapDragOffsets = {
	startOffset: number;
	endOffset: number;
};

export type TaskRoadmapDragOptions = {
	enabled: boolean;
	dayWidth: () => number;
	onPreview: (offsets: TaskRoadmapDragOffsets) => void;
	onCommit: (offsets: TaskRoadmapDragOffsets) => void;
	onCancel: () => void;
};

type DragMode = 'move' | 'start' | 'end';

const dragThresholdPixels = 4;

export const taskRoadmapDrag: Action<HTMLElement, TaskRoadmapDragOptions> = (element, initialOptions) => {
	let options = initialOptions;
	let pointerID: number | undefined;
	let mode: DragMode = 'move';
	let originX = 0;
	let dragged = false;
	let offsets: TaskRoadmapDragOffsets = { startOffset: 0, endOffset: 0 };

	function offsetsFor(dayDelta: number): TaskRoadmapDragOffsets {
		if (mode === 'start') return { startOffset: dayDelta, endOffset: 0 };
		if (mode === 'end') return { startOffset: 0, endOffset: dayDelta };
		return { startOffset: dayDelta, endOffset: dayDelta };
	}

	function pointerDown(event: PointerEvent): void {
		if (!options.enabled || event.button !== 0) return;
		const handle = event.target instanceof Element ? event.target.closest('[data-roadmap-handle]')?.getAttribute('data-roadmap-handle') : null;
		mode = handle === 'start' || handle === 'end' ? handle : 'move';
		pointerID = event.pointerId;
		originX = event.clientX;
		dragged = false;
		offsets = { startOffset: 0, endOffset: 0 };
		element.setPointerCapture(event.pointerId);
	}

	function pointerMove(event: PointerEvent): void {
		if (event.pointerId !== pointerID) return;
		const deltaX = event.clientX - originX;
		if (!dragged && Math.abs(deltaX) < dragThresholdPixels) return;
		dragged = true;
		const next = offsetsFor(Math.round(deltaX / Math.max(1, options.dayWidth())));
		if (next.startOffset === offsets.startOffset && next.endOffset === offsets.endOffset) return;
		offsets = next;
		options.onPreview(offsets);
	}

	function pointerUp(event: PointerEvent): void {
		if (event.pointerId !== pointerID) return;
		pointerID = undefined;
		if (!dragged) return;
		element.addEventListener('click', swallowClick, { capture: true, once: true });
		if (offsets.startOffset === 0 && offsets.endOffset === 0) options.onCancel();
		else options.onCommit(offsets);
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
