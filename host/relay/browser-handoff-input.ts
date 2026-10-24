export type DevtoolsCommand = {
	method: string;
	params: Record<string, unknown>;
};

export type Modifiers = {
	alt?: boolean;
	control?: boolean;
	meta?: boolean;
	shift?: boolean;
};

export type HandoffInput =
	| { type: 'mouse'; action: 'down' | 'up' | 'move'; x: number; y: number; button: MouseButton; clickCount: number; modifiers: Modifiers }
	| { type: 'wheel'; x: number; y: number; deltaX: number; deltaY: number }
	| { type: 'key'; action: 'down' | 'up'; key: string; code: string; keyCode: number; text: string; modifiers: Modifiers }
	| { type: 'text'; text: string }
	| { type: 'history'; direction: 'back' | 'forward' }
	| { type: 'reload' };

type MouseButton = 'left' | 'middle' | 'right' | 'none';

const largestInputsPerCall = 200;
const largestTextLength = 10_000;
const mouseButtons: MouseButton[] = ['left', 'middle', 'right', 'none'];

export function readHandoffInputs(offered: unknown): HandoffInput[] | null {
	if (!Array.isArray(offered) || offered.length === 0 || offered.length > largestInputsPerCall) return null;
	const inputs = offered.map(readHandoffInput);
	return inputs.every((input): input is HandoffInput => input !== null) ? inputs : null;
}

function readHandoffInput(offered: unknown): HandoffInput | null {
	const held = recordOf(offered);
	if (held.type === 'mouse') return readMouseInput(held);
	if (held.type === 'wheel') return readWheelInput(held);
	if (held.type === 'key') return readKeyInput(held);
	if (held.type === 'text') return readTextInput(held);
	if (held.type === 'history') return readHistoryInput(held);
	if (held.type === 'reload') return { type: 'reload' };
	return null;
}

function readMouseInput(held: Record<string, unknown>): HandoffInput | null {
	if (held.action !== 'down' && held.action !== 'up' && held.action !== 'move') return null;
	if (!isFiniteNumber(held.x) || !isFiniteNumber(held.y)) return null;
	const button = mouseButtons.find((candidate) => candidate === held.button) ?? 'none';
	const clickCount = isFiniteNumber(held.clickCount) ? Math.max(0, Math.round(held.clickCount)) : 0;
	return { type: 'mouse', action: held.action, x: held.x, y: held.y, button, clickCount, modifiers: readModifiers(held.modifiers) };
}

function readWheelInput(held: Record<string, unknown>): HandoffInput | null {
	if (!isFiniteNumber(held.x) || !isFiniteNumber(held.y)) return null;
	if (!isFiniteNumber(held.deltaX) || !isFiniteNumber(held.deltaY)) return null;
	return { type: 'wheel', x: held.x, y: held.y, deltaX: held.deltaX, deltaY: held.deltaY };
}

function readKeyInput(held: Record<string, unknown>): HandoffInput | null {
	if (held.action !== 'down' && held.action !== 'up') return null;
	if (typeof held.key !== 'string' || held.key === '') return null;
	return {
		type: 'key',
		action: held.action,
		key: held.key,
		code: typeof held.code === 'string' ? held.code : '',
		keyCode: isFiniteNumber(held.keyCode) ? Math.round(held.keyCode) : 0,
		text: typeof held.text === 'string' ? held.text.slice(0, 4) : '',
		modifiers: readModifiers(held.modifiers)
	};
}

function readTextInput(held: Record<string, unknown>): HandoffInput | null {
	if (typeof held.text !== 'string' || held.text === '' || held.text.length > largestTextLength) return null;
	return { type: 'text', text: held.text };
}

function readHistoryInput(held: Record<string, unknown>): HandoffInput | null {
	if (held.direction !== 'back' && held.direction !== 'forward') return null;
	return { type: 'history', direction: held.direction };
}

function readModifiers(offered: unknown): Modifiers {
	const held = recordOf(offered);
	return { alt: held.alt === true, control: held.control === true, meta: held.meta === true, shift: held.shift === true };
}

export function devtoolsCommandsFor(input: HandoffInput): DevtoolsCommand[] {
	switch (input.type) {
		case 'mouse':
			return [{ method: 'Input.dispatchMouseEvent', params: mouseEventOf(input) }];
		case 'wheel':
			return [
				{
					method: 'Input.dispatchMouseEvent',
					params: { type: 'mouseWheel', x: input.x, y: input.y, deltaX: input.deltaX, deltaY: input.deltaY }
				}
			];
		case 'key':
			return [{ method: 'Input.dispatchKeyEvent', params: keyEventOf(input) }];
		case 'text':
			return [{ method: 'Input.insertText', params: { text: input.text } }];
		case 'history':
			return [{ method: 'Runtime.evaluate', params: { expression: `history.${input.direction}()` } }];
		case 'reload':
			return [{ method: 'Page.reload', params: {} }];
	}
}

function mouseEventOf(input: Extract<HandoffInput, { type: 'mouse' }>): Record<string, unknown> {
	const typeByAction = { down: 'mousePressed', up: 'mouseReleased', move: 'mouseMoved' };
	return {
		type: typeByAction[input.action],
		x: input.x,
		y: input.y,
		button: input.button,
		clickCount: input.clickCount,
		modifiers: modifierBitsOf(input.modifiers)
	};
}

function keyEventOf(input: Extract<HandoffInput, { type: 'key' }>): Record<string, unknown> {
	const producesText = input.action === 'down' && input.text !== '' && !input.modifiers.control && !input.modifiers.meta;
	const type = input.action === 'up' ? 'keyUp' : producesText ? 'keyDown' : 'rawKeyDown';
	return {
		type,
		key: input.key,
		code: input.code,
		windowsVirtualKeyCode: input.keyCode,
		modifiers: modifierBitsOf(input.modifiers),
		...(producesText ? { text: input.text, unmodifiedText: input.text } : {})
	};
}

// Chrome DevTools Protocol Input domain: Alt=1, Ctrl=2, Meta/Command=4, Shift=8.
function modifierBitsOf(modifiers: Modifiers): number {
	return (modifiers.alt ? 1 : 0) | (modifiers.control ? 2 : 0) | (modifiers.meta ? 4 : 0) | (modifiers.shift ? 8 : 0);
}

function isFiniteNumber(offered: unknown): offered is number {
	return typeof offered === 'number' && Number.isFinite(offered);
}

function recordOf(offered: unknown): Record<string, unknown> {
	if (typeof offered !== 'object' || offered === null) return {};
	return Object.fromEntries(Object.entries(offered));
}
