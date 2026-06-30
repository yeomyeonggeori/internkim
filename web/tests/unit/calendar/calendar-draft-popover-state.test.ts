import { describe, expect, test } from 'bun:test';
import { createEvent } from '@dayflow/core';
import {
	draftPopoverChanges,
	draftPopoverPositionFromAnchor,
	draftPopoverStartDateTimeChanges,
	draftPopoverStateFromEvent,
	hasDraftPopoverEventChanges,
	type DraftPopoverAnchor
} from '../../../src/routes/calendar/embed/calendar-draft-popover-state';

test('calendar draft popover carries participants through changes', () => {
	const event = createEvent({
		id: 'participant-draft',
		title: 'Participant draft',
		description: 'Bring agenda',
		start: new Date(2026, 5, 18, 12),
		end: new Date(2026, 5, 18, 13),
		allDay: false,
		calendarId: 'internkim',
		meta: {
			location: 'Studio',
			participants: [{ personID: 'person-dongha', name: '이샘플', email: 'dongha@example.com' }]
		}
	});
	const popover = draftPopoverStateFromEvent(event, 'edit', null, null);

	expect(popover.participants).toEqual([{ personID: 'person-dongha', name: '이샘플', email: 'dongha@example.com' }]);
	expect(draftPopoverChanges(popover).meta.participants).toEqual([
		{ personID: 'person-dongha', name: '이샘플', email: 'dongha@example.com' }
	]);
	expect(hasDraftPopoverEventChanges({ ...popover, participants: [] }, event)).toBe(true);
});

test('keeps the existing duration when a timed draft popover start moves after the current end', () => {
	const event = createEvent({
		id: 'duration-draft',
		title: 'Duration draft',
		start: new Date(2026, 5, 18, 10),
		end: new Date(2026, 5, 18, 12),
		allDay: false,
		calendarId: 'internkim'
	});
	const popover = draftPopoverStateFromEvent(event, 'edit', null, null);

	const changes = draftPopoverStartDateTimeChanges(popover, '2026-06-18', '14:00');

	expect(changes).toMatchObject({
		dateKey: '2026-06-18',
		startTime: '14:00',
		endDateKey: '2026-06-18',
		endTime: '16:00'
	});
});

describe('calendar draft popover position', () => {
	test('keeps the popover from covering the clicked event title on narrow stages', () => {
		const stageElement = elementWithRectangle({ left: 0, top: 0, right: 420, bottom: 620, width: 420, height: 620 });
		const titleAnchor: DraftPopoverAnchor = {
			clientX: 260,
			clientY: 190,
			leftClientX: 180,
			rightClientX: 260,
			topClientY: 180,
			bottomClientY: 200,
			titleEndClientX: 260,
			titleTopClientY: 180,
			titleBottomClientY: 200
		};

		const position = draftPopoverPositionFromAnchor(titleAnchor, stageElement);

		expect(
			rectanglesOverlap(
				{ left: position.left, right: position.left + position.width, top: position.top, bottom: position.top + 520 },
				{ left: 180, right: 260, top: 180, bottom: 200 }
			)
		).toBe(false);
	});

	test('places right-edge events on the left side of the clicked block', () => {
		const stageElement = elementWithRectangle({ left: 0, top: 0, right: 900, bottom: 700, width: 900, height: 700 });
		const titleAnchor: DraftPopoverAnchor = {
			clientX: 874,
			clientY: 220,
			leftClientX: 760,
			rightClientX: 874,
			topClientY: 210,
			bottomClientY: 230,
			titleEndClientX: 874,
			titleTopClientY: 210,
			titleBottomClientY: 230
		};

		const position = draftPopoverPositionFromAnchor(titleAnchor, stageElement);

		expect(position.arrowSide).toBe('right');
		expect(position.left + position.width <= 760 - 24).toBe(true);
	});

	test('places long event popovers beside the visible title end instead of the full block end', () => {
		const stageElement = elementWithRectangle({ left: 0, top: 0, right: 900, bottom: 700, width: 900, height: 700 });
		const titleAnchor: DraftPopoverAnchor = {
			clientX: 780,
			clientY: 220,
			leftClientX: 160,
			rightClientX: 780,
			topClientY: 210,
			bottomClientY: 230,
			titleEndClientX: 250,
			titleTopClientY: 210,
			titleBottomClientY: 230
		};

		const position = draftPopoverPositionFromAnchor(titleAnchor, stageElement);

		expect(position.arrowSide).toBe('left');
		expect(position.left >= 250 + 24).toBe(true);
		expect(position.left < 780).toBe(true);
	});

	test('honors left side placement for right panel event anchors', () => {
		const stageElement = elementWithRectangle({ left: 0, top: 0, right: 1200, bottom: 700, width: 1200, height: 700 });
		const titleAnchor: DraftPopoverAnchor = {
			clientX: 880,
			clientY: 220,
			leftClientX: 600,
			rightClientX: 880,
			topClientY: 210,
			bottomClientY: 230,
			titleEndClientX: 640,
			titleTopClientY: 210,
			titleBottomClientY: 230,
			preferredSide: 'left'
		};

		const position = draftPopoverPositionFromAnchor(titleAnchor, stageElement);

		expect(position.arrowSide).toBe('right');
		expect(position.left + position.width <= 600 - 24).toBe(true);
	});

	test('keeps event popovers attached to their title when the calendar stage scrolls beyond the page', () => {
		const originalWindow = globalThis.window;
		Object.defineProperty(globalThis, 'window', {
			value: { innerWidth: 900, innerHeight: 600 },
			configurable: true
		});
		const stageElement = elementWithRectangle({ left: 0, top: 0, right: 900, bottom: 1200, width: 900, height: 1200 });
		const titleAnchor: DraftPopoverAnchor = {
			clientX: 260,
			clientY: 930,
			leftClientX: 180,
			rightClientX: 260,
			topClientY: 920,
			bottomClientY: 940,
			titleEndClientX: 260,
			titleTopClientY: 920,
			titleBottomClientY: 940
		};

		try {
			const position = draftPopoverPositionFromAnchor(titleAnchor, stageElement);
			const titleCenterY = 930;

			expect(position.top).toBe(titleCenterY - 48);
			expect(position.top + position.arrowTop + 8).toBe(titleCenterY);
		} finally {
			Object.defineProperty(globalThis, 'window', {
				value: originalWindow,
				configurable: true
			});
		}
	});

	test('keeps measured popovers attached to their title instead of adding internal scroll', () => {
		const originalWindow = globalThis.window;
		Object.defineProperty(globalThis, 'window', {
			value: { innerWidth: 900, innerHeight: 760 },
			configurable: true
		});
		const stageElement = elementWithRectangle({ left: 0, top: 0, right: 900, bottom: 1200, width: 900, height: 1200 });
		const titleAnchor: DraftPopoverAnchor = {
			clientX: 420,
			clientY: 470,
			leftClientX: 360,
			rightClientX: 460,
			topClientY: 460,
			bottomClientY: 480,
			titleEndClientX: 460,
			titleTopClientY: 460,
			titleBottomClientY: 480
		};

		try {
			const position = draftPopoverPositionFromAnchor(titleAnchor, stageElement, { height: 640 });
			const titleCenterY = 470;

			expect(position.top + 640 <= 748).toBe(true);
			expect(position.top + position.arrowTop + 8).toBe(titleCenterY);
		} finally {
			Object.defineProperty(globalThis, 'window', {
				value: originalWindow,
				configurable: true
			});
		}
	});

	test('keeps measured popovers inside the visible stage bottom', () => {
		const originalWindow = globalThis.window;
		Object.defineProperty(globalThis, 'window', {
			value: { innerWidth: 900, innerHeight: 600 },
			configurable: true
		});
		const stageElement = elementWithRectangle({ left: 0, top: 0, right: 900, bottom: 900, width: 900, height: 900 });
		const titleAnchor: DraftPopoverAnchor = {
			clientX: 420,
			clientY: 560,
			leftClientX: 360,
			rightClientX: 460,
			topClientY: 550,
			bottomClientY: 570,
			titleEndClientX: 460,
			titleTopClientY: 550,
			titleBottomClientY: 570
		};

		try {
			const position = draftPopoverPositionFromAnchor(titleAnchor, stageElement, { height: 400 });

			expect(position.top + 400 <= 588).toBe(true);
			expect(position.arrowTop >= 18).toBe(true);
		} finally {
			Object.defineProperty(globalThis, 'window', {
				value: originalWindow,
				configurable: true
			});
		}
	});

	test('keeps generic draft anchors at the preferred popover width', () => {
		const stageElement = elementWithRectangle({ left: 0, top: 0, right: 900, bottom: 700, width: 900, height: 700 });
		const draftAnchor: DraftPopoverAnchor = {
			clientX: 360,
			clientY: 220,
			leftClientX: 280,
			topClientY: 210,
			bottomClientY: 230
		};

		const position = draftPopoverPositionFromAnchor(draftAnchor, stageElement);

		expect(position.width).toBe(400);
	});
});

type TestRectangle = {
	left: number;
	top: number;
	right: number;
	bottom: number;
	width: number;
	height: number;
};

function elementWithRectangle(rectangle: TestRectangle): HTMLElement {
	return {
		getBoundingClientRect: () => rectangle
	} as HTMLElement;
}

function rectanglesOverlap(
	firstRectangle: Pick<TestRectangle, 'left' | 'right' | 'top' | 'bottom'>,
	secondRectangle: Pick<TestRectangle, 'left' | 'right' | 'top' | 'bottom'>
): boolean {
	return (
		firstRectangle.left < secondRectangle.right &&
		secondRectangle.left < firstRectangle.right &&
		firstRectangle.top < secondRectangle.bottom &&
		secondRectangle.top < firstRectangle.bottom
	);
}
