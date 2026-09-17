<script lang="ts">
	import {
		beginTouch,
		containedBox,
		keyInputOf,
		modifiersOf,
		mouseButtonOf,
		moveTouch,
		nextPress,
		pointOnViewport,
		viewportOfArea,
		wheelPixelsOf,
		type HandoffInput,
		type MouseButton,
		type Point,
		type PressedPoint,
		type ScreenFrame,
		type TouchTrack,
		type Viewport
	} from '$lib/browser-handoff/handoff-input';
	import type { Attachment } from 'svelte/attachments';

	type Props = {
		frame: ScreenFrame | null;
		label: string;
		waitingText: string;
		onInput: (input: HandoffInput) => void;
		onResize: (area: Viewport) => void;
	};

	let { frame, label, waitingText, onInput, onResize }: Props = $props();

	let keyboard = $state<HTMLTextAreaElement>();
	let area = $state<Viewport>({ width: 0, height: 0 });
	let lastPress: PressedPoint | null = null;
	let pressedButton: MouseButton = 'none';
	let touch: TouchTrack | null = null;

	const imageBox = $derived(frame ? containedBox({ left: 0, top: 0, ...area }, frame) : null);

	export function focusKeyboard(): void {
		keyboard?.focus();
	}

	function pointOf(event: MouseEvent, screen: HTMLElement): Point | null {
		if (!frame) return null;
		const box = containedBox(screen.getBoundingClientRect(), frame);
		return pointOnViewport(event.clientX, event.clientY, box, frame);
	}

	function press(point: Point, button: MouseButton, at: number, modifiers: MouseEvent): void {
		lastPress = nextPress(lastPress, point, at);
		onInput({ type: 'mouse', action: 'down', ...point, button, clickCount: lastPress.clickCount, modifiers: modifiersOf(modifiers) });
	}

	function release(point: Point, button: MouseButton, modifiers: MouseEvent): void {
		onInput({ type: 'mouse', action: 'up', ...point, button, clickCount: lastPress?.clickCount ?? 1, modifiers: modifiersOf(modifiers) });
	}

	function onPointerDown(event: PointerEvent & { currentTarget: HTMLDivElement }): void {
		event.preventDefault();
		if (event.pointerType === 'touch') {
			touch = beginTouch(event.clientX, event.clientY);
			return;
		}
		focusKeyboard();
		const point = pointOf(event, event.currentTarget);
		if (!point) return;
		event.currentTarget.setPointerCapture(event.pointerId);
		pressedButton = mouseButtonOf(event.button);
		press(point, pressedButton, event.timeStamp, event);
	}

	function onPointerMove(event: PointerEvent & { currentTarget: HTMLDivElement }): void {
		const point = pointOf(event, event.currentTarget);
		if (!point || !frame) return;
		if (event.pointerType !== 'touch') {
			onInput({ type: 'mouse', action: 'move', ...point, button: pressedButton, clickCount: 0, modifiers: modifiersOf(event) });
			return;
		}
		if (!touch || !imageBox) return;
		const moved = moveTouch(touch, event.clientX, event.clientY, frame.width / imageBox.width);
		touch = moved.track;
		if (moved.scroll) onInput({ type: 'wheel', ...point, ...moved.scroll });
	}

	function onPointerUp(event: PointerEvent & { currentTarget: HTMLDivElement }): void {
		const point = pointOf(event, event.currentTarget);
		if (!point) return;
		if (event.pointerType !== 'touch') {
			release(point, mouseButtonOf(event.button), event);
			pressedButton = 'none';
			return;
		}
		const wasATap = touch !== null && !touch.hasMoved;
		touch = null;
		if (!wasATap) return;
		press(point, 'left', event.timeStamp, event);
		release(point, 'left', event);
	}

	function onKey(event: KeyboardEvent, action: 'down' | 'up'): void {
		const input = keyInputOf(event, action);
		if (!input) return;
		event.preventDefault();
		onInput(input);
	}

	function sendTypedText(): void {
		if (!keyboard?.value) return;
		const text = keyboard.value;
		keyboard.value = '';
		onInput({ type: 'text', text });
	}

	function onTyped(event: Event): void {
		if (event instanceof InputEvent && event.isComposing) return;
		sendTypedText();
	}

	const followSize: Attachment<HTMLDivElement> = (screen) => {
		const observer = new ResizeObserver(([entry]) => {
			area = viewportOfArea(entry.contentRect.width, entry.contentRect.height);
			onResize(area);
		});
		observer.observe(screen);
		return () => observer.disconnect();
	};

	const takeWheel: Attachment<HTMLDivElement> = (screen) => {
		const onWheel = (event: WheelEvent) => {
			event.preventDefault();
			const point = pointOf(event, screen);
			if (!point) return;
			const isInLines = event.deltaMode === WheelEvent.DOM_DELTA_LINE;
			onInput({ type: 'wheel', ...point, deltaX: wheelPixelsOf(event.deltaX, isInLines), deltaY: wheelPixelsOf(event.deltaY, isInLines) });
		};
		screen.addEventListener('wheel', onWheel, { passive: false });
		return () => screen.removeEventListener('wheel', onWheel);
	};
</script>

<div
	role="application"
	aria-label={label}
	class="relative size-full touch-none overflow-hidden bg-muted select-none"
	{@attach followSize}
	{@attach takeWheel}
	onpointerdown={onPointerDown}
	onpointermove={onPointerMove}
	onpointerup={onPointerUp}
	oncontextmenu={(event) => event.preventDefault()}
>
	{#if frame && imageBox}
		<img
			src={frame.image}
			alt=""
			draggable="false"
			class="pointer-events-none absolute max-w-none"
			style:left="{imageBox.left}px"
			style:top="{imageBox.top}px"
			style:width="{imageBox.width}px"
			style:height="{imageBox.height}px"
		/>
	{:else}
		<div class="flex size-full items-center justify-center text-sm text-muted-foreground">{waitingText}</div>
	{/if}
	<textarea
		bind:this={keyboard}
		aria-label={label}
		autocapitalize="off"
		autocomplete="off"
		spellcheck="false"
		class="pointer-events-none absolute top-0 left-0 size-px resize-none text-base opacity-0"
		onkeydown={(event) => onKey(event, 'down')}
		onkeyup={(event) => onKey(event, 'up')}
		oninput={onTyped}
		oncompositionend={sendTypedText}
	></textarea>
</div>
