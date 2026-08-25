import { describe, expect, test } from 'bun:test';
import { calendarParticipantsWithViewerFirst } from '../../../src/routes/calendar/embed/calendar-participants';

const participants = [
	{ personID: 'leesample', name: '이영희', email: 'member1@example.com' },
	{ personID: 'kim', name: '김철수', email: 'kim@example.com' },
	{ personID: 'park', name: '박지민', email: 'park@example.com' }
];

describe('calendar participants viewer ordering', () => {
	test('moves the viewer to the front of the list', () => {
		const ordered = calendarParticipantsWithViewerFirst(participants, 'KIM@example.com ');
		expect(ordered.map((participant) => participant.personID)).toEqual(['kim', 'leesample', 'park']);
	});

	test('keeps the original order when the viewer is not a candidate', () => {
		const ordered = calendarParticipantsWithViewerFirst(participants, 'someone@example.com');
		expect(ordered.map((participant) => participant.personID)).toEqual(['leesample', 'kim', 'park']);
	});

	test('keeps the original order without a viewer email', () => {
		const ordered = calendarParticipantsWithViewerFirst(participants, '   ');
		expect(ordered.map((participant) => participant.personID)).toEqual(['leesample', 'kim', 'park']);
	});
});
