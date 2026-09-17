export type Modifiers = {
	alt: boolean;
	control: boolean;
	meta: boolean;
	shift: boolean;
};

export type MouseButton = 'left' | 'middle' | 'right' | 'none';

export type HandoffInput =
	| { type: 'mouse'; action: 'down' | 'up' | 'move'; x: number; y: number; button: MouseButton; clickCount: number; modifiers: Modifiers }
	| { type: 'wheel'; x: number; y: number; deltaX: number; deltaY: number }
	| { type: 'key'; action: 'down' | 'up'; key: string; code: string; keyCode: number; text: string; modifiers: Modifiers }
	| { type: 'text'; text: string }
	| { type: 'history'; direction: 'back' | 'forward' }
	| { type: 'reload' };

export type Viewport = { width: number; height: number };

export type ScreenFrame = Viewport & { image: string };

export type Point = { x: number; y: number };

export type ScreenBox = { left: number; top: number; width: number; height: number };

export type ModifierKeys = { altKey: boolean; ctrlKey: boolean; metaKey: boolean; shiftKey: boolean };

export type KeyPress = ModifierKeys & {
	key: string;
	code: string;
	keyCode: number;
	isComposing: boolean;
};

export type PressedPoint = Point & { at: number; clickCount: number };

const keysAnInputMethodOwns = new Set(['Process', 'Unidentified', 'Dead']);
const inputMethodKeyCode = 229;
const doubleClickMilliseconds = 500;
const doubleClickDistance = 6;

export function pointOnViewport(clientX: number, clientY: number, box: ScreenBox, viewport: Viewport): Point {
	const x = ((clientX - box.left) / box.width) * viewport.width;
	const y = ((clientY - box.top) / box.height) * viewport.height;
	return { x: clamp(Math.round(x), 0, viewport.width - 1), y: clamp(Math.round(y), 0, viewport.height - 1) };
}

export function mouseButtonOf(button: number): MouseButton {
	if (button === 0) return 'left';
	if (button === 1) return 'middle';
	if (button === 2) return 'right';
	return 'none';
}

export function modifiersOf(keys: ModifierKeys): Modifiers {
	return { alt: keys.altKey, control: keys.ctrlKey, meta: keys.metaKey, shift: keys.shiftKey };
}

export function nextPress(previous: PressedPoint | null, point: Point, at: number): PressedPoint {
	const isRepeated =
		previous !== null &&
		at - previous.at <= doubleClickMilliseconds &&
		Math.abs(point.x - previous.x) <= doubleClickDistance &&
		Math.abs(point.y - previous.y) <= doubleClickDistance;
	return { ...point, at, clickCount: isRepeated ? previous.clickCount + 1 : 1 };
}

export function keyInputOf(press: KeyPress, action: 'down' | 'up'): HandoffInput | null {
	if (isOwnedByTheInputMethod(press)) return null;
	if (isPasteShortcut(press)) return null;
	return {
		type: 'key',
		action,
		key: press.key,
		code: press.code,
		keyCode: press.keyCode,
		text: textTypedBy(press),
		modifiers: modifiersOf(press)
	};
}

function isOwnedByTheInputMethod(press: KeyPress): boolean {
	return press.isComposing || press.keyCode === inputMethodKeyCode || keysAnInputMethodOwns.has(press.key);
}

function isPasteShortcut(press: KeyPress): boolean {
	return (press.ctrlKey || press.metaKey) && press.key.toLowerCase() === 'v';
}

function textTypedBy(press: KeyPress): string {
	if (press.key === 'Enter') return '\r';
	return [...press.key].length === 1 ? press.key : '';
}

export function withQueuedInput(queued: HandoffInput[], input: HandoffInput): HandoffInput[] {
	const last = queued.at(-1);
	if (last?.type === 'mouse' && last.action === 'move' && input.type === 'mouse' && input.action === 'move') {
		return [...queued.slice(0, -1), input];
	}
	if (last?.type === 'wheel' && input.type === 'wheel') {
		return [...queued.slice(0, -1), { ...input, deltaX: last.deltaX + input.deltaX, deltaY: last.deltaY + input.deltaY }];
	}
	return [...queued, input];
}

function clamp(value: number, smallest: number, largest: number): number {
	return Math.min(Math.max(value, smallest), largest);
}

export type TouchTrack = { startX: number; startY: number; lastX: number; lastY: number; hasMoved: boolean };

export type TouchMove = { track: TouchTrack; scroll: { deltaX: number; deltaY: number } | null };

const tapDistance = 10;

export function beginTouch(clientX: number, clientY: number): TouchTrack {
	return { startX: clientX, startY: clientY, lastX: clientX, lastY: clientY, hasMoved: false };
}

export function moveTouch(track: TouchTrack, clientX: number, clientY: number, viewportPerScreenPixel: number): TouchMove {
	const isStillATap = !track.hasMoved && Math.hypot(clientX - track.startX, clientY - track.startY) < tapDistance;
	if (isStillATap) return { track, scroll: null };
	return {
		track: { ...track, lastX: clientX, lastY: clientY, hasMoved: true },
		scroll: {
			deltaX: Math.round((track.lastX - clientX) * viewportPerScreenPixel),
			deltaY: Math.round((track.lastY - clientY) * viewportPerScreenPixel)
		}
	};
}

const pixelsPerWheelLine = 16;

export function wheelPixelsOf(delta: number, isInLines: boolean): number {
	return Math.round(isInLines ? delta * pixelsPerWheelLine : delta);
}

export function containedBox(area: ScreenBox, frame: Viewport): ScreenBox {
	const scale = Math.min(area.width / frame.width, area.height / frame.height);
	const width = frame.width * scale;
	const height = frame.height * scale;
	return { left: area.left + (area.width - width) / 2, top: area.top + (area.height - height) / 2, width, height };
}

export function viewportOfArea(width: number, height: number): Viewport {
	return { width: Math.floor(width), height: Math.floor(height) };
}
