import { describe, expect, test } from 'bun:test';
import { noteEditorPositionFromRect } from '../../src/routes/admin/user-note-position';

describe('noteEditorPositionFromRect', () => {
	test('places the editor above the button when there is room', () => {
		const position = noteEditorPositionFromRect({ left: 320, top: 420, bottom: 452 }, 1040, 720);

		expect(position).toEqual({ left: 320, top: 192 });
	});

	test('keeps the editor inside the right and top viewport edges', () => {
		const position = noteEditorPositionFromRect({ left: 980, top: 80, bottom: 112 }, 1040, 720);

		expect(position).toEqual({ left: 708, top: 120 });
	});
});
