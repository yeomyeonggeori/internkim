import type { Action } from 'svelte/action';

type LightboxDragDismissOptions = {
	onDismiss: () => void;
	isDisabled: () => boolean;
};

type DragStart = { x: number; y: number; time: number };

const axisLockDistance = 10;
const dismissDistance = 120;
const dismissSpeedPixelsPerMillisecond = 0.5;
const fadeDistance = 400;

export class LightboxDragDismiss {
	offset = $state(0);
	isDragging = $state(false);
	backdropOpacity = $derived(1 - Math.min(Math.abs(this.offset) / fadeDistance, 1));
	#start: DragStart | null = null;

	constructor(private readonly options: LightboxDragDismissOptions) {}

	reset(): void {
		this.offset = 0;
		this.isDragging = false;
		this.#start = null;
	}

	start(event: TouchEvent): void {
		const touch = event.touches[0];
		if (event.touches.length !== 1 || !touch || this.options.isDisabled()) {
			this.#start = null;
			return;
		}
		this.#start = { x: touch.clientX, y: touch.clientY, time: event.timeStamp };
	}

	move(event: TouchEvent): void {
		const touch = event.touches[0];
		if (!this.#start || !touch) return;
		if (event.touches.length !== 1) {
			this.reset();
			return;
		}
		const deltaX = touch.clientX - this.#start.x;
		const deltaY = touch.clientY - this.#start.y;
		if (!this.isDragging) {
			if (Math.max(Math.abs(deltaX), Math.abs(deltaY)) < axisLockDistance) return;
			if (Math.abs(deltaX) >= Math.abs(deltaY)) {
				this.#start = null;
				return;
			}
			this.isDragging = true;
		}
		event.preventDefault();
		this.offset = deltaY;
	}

	end(event: TouchEvent): void {
		if (!this.#start || !this.isDragging) {
			this.#start = null;
			return;
		}
		const elapsed = Math.max(event.timeStamp - this.#start.time, 1);
		const isFlung = Math.abs(this.offset) / elapsed >= dismissSpeedPixelsPerMillisecond;
		const isFarEnough = Math.abs(this.offset) >= dismissDistance;
		this.#start = null;
		this.isDragging = false;
		if (isFarEnough || isFlung) this.options.onDismiss();
		else this.offset = 0;
	}
}

export const dragsToDismiss: Action<HTMLElement, LightboxDragDismiss> = (node, drag) => {
	const start = (event: TouchEvent) => drag.start(event);
	const move = (event: TouchEvent) => drag.move(event);
	const end = (event: TouchEvent) => drag.end(event);
	const cancel = () => drag.reset();
	node.addEventListener('touchstart', start, { passive: true });
	node.addEventListener('touchmove', move, { passive: false });
	node.addEventListener('touchend', end);
	node.addEventListener('touchcancel', cancel);
	return {
		destroy() {
			node.removeEventListener('touchstart', start);
			node.removeEventListener('touchmove', move);
			node.removeEventListener('touchend', end);
			node.removeEventListener('touchcancel', cancel);
		}
	};
};
