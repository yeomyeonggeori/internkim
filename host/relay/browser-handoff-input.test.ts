import { describe, expect, test } from 'bun:test';
import { devtoolsCommandsFor, readHandoffInputs } from './browser-handoff-input';

describe('readHandoffInputs', () => {
	test('reads each kind of input a person can give the browser', () => {
		const inputs = readHandoffInputs([
			{ type: 'mouse', action: 'down', x: 10, y: 20, button: 'left', clickCount: 1, modifiers: { shift: true } },
			{ type: 'wheel', x: 10, y: 20, deltaX: 0, deltaY: 120 },
			{ type: 'key', action: 'down', key: 'a', code: 'KeyA', keyCode: 65, text: 'a' },
			{ type: 'text', text: '안녕하세요' },
			{ type: 'history', direction: 'back' },
			{ type: 'reload' },
			{ type: 'open', url: 'https://example.com/' }
		]);

		expect(inputs).toHaveLength(7);
		expect(inputs?.[0]).toEqual({
			type: 'mouse',
			action: 'down',
			x: 10,
			y: 20,
			button: 'left',
			clickCount: 1,
			modifiers: { alt: false, control: false, meta: false, shift: true }
		});
	});

	test('refuses the whole list when one input is not understood', () => {
		expect(readHandoffInputs([{ type: 'reload' }, { type: 'mouse', action: 'down', x: 'left' }])).toBeNull();
		expect(readHandoffInputs([{ type: 'launch-missiles' }])).toBeNull();
	});

	test('refuses an empty list and anything that is not a list', () => {
		expect(readHandoffInputs([])).toBeNull();
		expect(readHandoffInputs({ type: 'reload' })).toBeNull();
	});

	test('opens only web addresses', () => {
		expect(readHandoffInputs([{ type: 'open', url: 'file:///etc/passwd' }])).toBeNull();
		expect(readHandoffInputs([{ type: 'open', url: 'javascript:alert(1)' }])).toBeNull();
		expect(readHandoffInputs([{ type: 'open', url: 'http://example.com' }])).not.toBeNull();
	});
});

describe('devtoolsCommandsFor', () => {
	test('a pressed mouse button becomes a pressed mouse event with modifier bits', () => {
		expect(
			devtoolsCommandsFor({
				type: 'mouse',
				action: 'down',
				x: 5,
				y: 6,
				button: 'left',
				clickCount: 2,
				modifiers: { control: true, shift: true }
			})
		).toEqual([
			{
				method: 'Input.dispatchMouseEvent',
				params: { type: 'mousePressed', x: 5, y: 6, button: 'left', clickCount: 2, modifiers: 10 }
			}
		]);
	});

	test('a key that types a character carries its text', () => {
		expect(
			devtoolsCommandsFor({ type: 'key', action: 'down', key: 'a', code: 'KeyA', keyCode: 65, text: 'a', modifiers: {} })
		).toEqual([
			{
				method: 'Input.dispatchKeyEvent',
				params: { type: 'keyDown', key: 'a', code: 'KeyA', windowsVirtualKeyCode: 65, modifiers: 0, text: 'a', unmodifiedText: 'a' }
			}
		]);
	});

	test('a shortcut and a key without text are raw key presses', () => {
		const [shortcut] = devtoolsCommandsFor({
			type: 'key',
			action: 'down',
			key: 'a',
			code: 'KeyA',
			keyCode: 65,
			text: 'a',
			modifiers: { meta: true }
		});
		const [backspace] = devtoolsCommandsFor({
			type: 'key',
			action: 'down',
			key: 'Backspace',
			code: 'Backspace',
			keyCode: 8,
			text: '',
			modifiers: {}
		});

		expect(shortcut.params).toEqual({ type: 'rawKeyDown', key: 'a', code: 'KeyA', windowsVirtualKeyCode: 65, modifiers: 4 });
		expect(backspace.params).toEqual({ type: 'rawKeyDown', key: 'Backspace', code: 'Backspace', windowsVirtualKeyCode: 8, modifiers: 0 });
	});

	test('composed text is inserted as a whole', () => {
		expect(devtoolsCommandsFor({ type: 'text', text: '한글' })).toEqual([
			{ method: 'Input.insertText', params: { text: '한글' } }
		]);
	});
});
