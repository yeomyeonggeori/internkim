import { describe, expect, test } from 'bun:test';
import {
	beginTouch,
	containedBox,
	isOnAField,
	keyInputOf,
	keyInputsOfEdit,
	moveTouch,
	nextPress,
	pointOnViewport,
	viewportOfArea,
	wheelPixelsOf,
	withQueuedInput,
	type HandoffInput,
	type KeyPress
} from '../../../src/lib/browser-handoff/handoff-input';

const viewport = { width: 1280, height: 800 };

function keyPress(overrides: Partial<KeyPress>): KeyPress {
	return { key: 'a', code: 'KeyA', keyCode: 65, isComposing: false, altKey: false, ctrlKey: false, metaKey: false, shiftKey: false, ...overrides };
}

describe('pointOnViewport', () => {
	test('scales a point on the shown screen to the browser viewport', () => {
		const box = { left: 100, top: 50, width: 640, height: 400 };

		expect(pointOnViewport(420, 250, box, viewport)).toEqual({ x: 640, y: 400 });
	});

	test('keeps a point dragged past the edge inside the viewport', () => {
		const box = { left: 0, top: 0, width: 640, height: 400 };

		expect(pointOnViewport(-20, 900, box, viewport)).toEqual({ x: 0, y: 799 });
	});
});

describe('nextPress', () => {
	test('a quick second press in the same place is a double click', () => {
		const first = nextPress(null, { x: 10, y: 10 }, 1_000);
		const second = nextPress(first, { x: 12, y: 11 }, 1_200);

		expect(first.clickCount).toBe(1);
		expect(second.clickCount).toBe(2);
	});

	test('a slow or distant press starts counting again', () => {
		const first = nextPress(null, { x: 10, y: 10 }, 1_000);

		expect(nextPress(first, { x: 10, y: 10 }, 2_000).clickCount).toBe(1);
		expect(nextPress(first, { x: 200, y: 10 }, 1_100).clickCount).toBe(1);
	});
});

describe('keyInputOf', () => {
	test('a printable key carries the character it types', () => {
		expect(keyInputOf(keyPress({ shiftKey: true, key: 'A' }), 'down')).toEqual({
			type: 'key',
			action: 'down',
			key: 'A',
			code: 'KeyA',
			keyCode: 65,
			text: 'A',
			modifiers: { alt: false, control: false, meta: false, shift: true }
		});
	});

	test('enter types a carriage return and backspace types nothing', () => {
		expect(keyInputOf(keyPress({ key: 'Enter', code: 'Enter', keyCode: 13 }), 'down')).toMatchObject({ text: '\r' });
		expect(keyInputOf(keyPress({ key: 'Backspace', code: 'Backspace', keyCode: 8 }), 'down')).toMatchObject({ text: '' });
	});

	test('keys the input method is composing are left to the text it commits', () => {
		expect(keyInputOf(keyPress({ key: 'ㅎ', keyCode: 229 }), 'down')).toBeNull();
		expect(keyInputOf(keyPress({ key: 'Process' }), 'down')).toBeNull();
		expect(keyInputOf(keyPress({ isComposing: true }), 'up')).toBeNull();
		expect(keyInputOf(keyPress({ key: 'Unidentified', keyCode: 229 }), 'down')).toBeNull();
	});

	test('pasting is left to the text field so the local clipboard arrives', () => {
		expect(keyInputOf(keyPress({ key: 'v', metaKey: true }), 'down')).toBeNull();
		expect(keyInputOf(keyPress({ key: 'V', ctrlKey: true, shiftKey: true }), 'down')).toBeNull();
		expect(keyInputOf(keyPress({ key: 'a', metaKey: true }), 'down')).not.toBeNull();
	});
});

describe('withQueuedInput', () => {
	const noModifiers = { alt: false, control: false, meta: false, shift: false };
	const move = (x: number): HandoffInput => ({ type: 'mouse', action: 'move', x, y: 0, button: 'none', clickCount: 0, modifiers: noModifiers });

	test('only the latest of consecutive moves waits to be sent', () => {
		expect(withQueuedInput([move(1)], move(2))).toEqual([move(2)]);
	});

	test('consecutive wheel turns add up', () => {
		const queued = withQueuedInput([{ type: 'wheel', x: 1, y: 1, deltaX: 0, deltaY: 100 }], { type: 'wheel', x: 2, y: 2, deltaX: 5, deltaY: 40 });

		expect(queued).toEqual([{ type: 'wheel', x: 2, y: 2, deltaX: 5, deltaY: 140 }]);
	});

	test('a move after a press is kept so the drag starts where it was pressed', () => {
		const pressed: HandoffInput = { type: 'mouse', action: 'down', x: 1, y: 0, button: 'left', clickCount: 1, modifiers: noModifiers };

		expect(withQueuedInput([pressed], move(2))).toEqual([pressed, move(2)]);
	});

	test('every move while a button is held is kept so the drag follows its path', () => {
		const dragTo = (x: number): HandoffInput => ({ type: 'mouse', action: 'move', x, y: 0, button: 'left', clickCount: 0, modifiers: noModifiers });

		expect(withQueuedInput([dragTo(1)], dragTo(2))).toEqual([dragTo(1), dragTo(2)]);
	});
});

describe('keyInputsOfEdit', () => {
	test('deleting and breaking a line on a phone keyboard press the matching keys', () => {
		expect(keyInputsOfEdit('deleteContentBackward')).toMatchObject([
			{ type: 'key', action: 'down', key: 'Backspace' },
			{ type: 'key', action: 'up', key: 'Backspace' }
		]);
		expect(keyInputsOfEdit('insertParagraph')).toMatchObject([{ key: 'Enter', text: '\r' }, { key: 'Enter', text: '\r' }]);
	});

	test('typed text is left to the text the field sends', () => {
		expect(keyInputsOfEdit('insertText')).toEqual([]);
	});
});

describe('isOnAField', () => {
	const fields = [{ x: 20, y: 40, width: 200, height: 30 }];

	test('a point inside a field is on it and a point past its far edge is not', () => {
		expect(isOnAField({ x: 20, y: 40 }, fields)).toBe(true);
		expect(isOnAField({ x: 219, y: 69 }, fields)).toBe(true);
		expect(isOnAField({ x: 220, y: 50 }, fields)).toBe(false);
		expect(isOnAField({ x: 100, y: 70 }, fields)).toBe(false);
	});
});

describe('touch', () => {
	test('a finger that barely moves is still a tap', () => {
		const track = beginTouch(100, 100, 0);

		expect(moveTouch(track, 104, 103, 900, 2)).toEqual({ track, scroll: null, startsDrag: false });
	});

	test('a finger swiped right away scrolls the page the other way, in viewport pixels', () => {
		const moved = moveTouch(beginTouch(100, 100, 0), 100, 60, 100, 2);
		const movedAgain = moveTouch(moved.track, 100, 50, 1000, 2);

		expect(moved.scroll).toEqual({ deltaX: 0, deltaY: 80 });
		expect(movedAgain.scroll).toEqual({ deltaX: 0, deltaY: 20 });
		expect(movedAgain.track.gesture).toBe('scroll');
	});

	test('a finger held still before moving drags instead of scrolling', () => {
		const moved = moveTouch(beginTouch(100, 100, 0), 140, 100, 450, 2);
		const movedAgain = moveTouch(moved.track, 180, 100, 500, 2);

		expect(moved).toMatchObject({ scroll: null, startsDrag: true, track: { gesture: 'drag' } });
		expect(movedAgain).toMatchObject({ scroll: null, startsDrag: false, track: { gesture: 'drag', lastX: 180 } });
	});
});

describe('wheelPixelsOf', () => {
	test('turns wheel lines into pixels', () => {
		expect(wheelPixelsOf(3, true)).toBe(48);
		expect(wheelPixelsOf(12.4, false)).toBe(12);
	});
});


describe('containedBox', () => {
	test('a frame the size of its area fills it exactly', () => {
		expect(containedBox({ left: 10, top: 20, width: 390, height: 700 }, { width: 390, height: 700 })).toEqual({ left: 10, top: 20, width: 390, height: 700 });
	});

	test('a frame from before a resize is centred inside the new area', () => {
		expect(containedBox({ left: 0, top: 0, width: 400, height: 700 }, { width: 1280, height: 800 })).toEqual({ left: 0, top: 225, width: 400, height: 250 });
	});
});

describe('viewportOfArea', () => {
	test('never asks for a pixel more than the area has', () => {
		expect(viewportOfArea(390.9, 700.2)).toEqual({ width: 390, height: 700 });
	});
});
