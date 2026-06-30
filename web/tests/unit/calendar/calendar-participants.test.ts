import { expect, test } from 'bun:test';

import {
	calendarParticipantInputs,
	calendarParticipantKey,
	calendarParticipantMatchesSearch,
	calendarParticipantOptionLabel,
	calendarParticipantsFromUnknown
} from '../../../src/routes/calendar/embed/calendar-participants';

test('keeps duplicate participant names when identities differ', () => {
	const participants = calendarParticipantsFromUnknown([
		{ personID: 'person-left', name: '김여명', email: 'left@example.com', image: '/calendar/api/participants/person-left/image' },
		{ personID: 'person-right', name: '김여명', email: 'right@example.com' }
	]);

	expect(participants.length).toBe(2);
	expect(participants.map(calendarParticipantKey)).toEqual(['id:person-left', 'id:person-right']);
	expect(participants[0].image).toBe('/calendar/api/participants/person-left/image');
	expect(calendarParticipantOptionLabel(participants[0], participants)).toBe('김여명 · left@example.com');
});

test('builds participant payloads without response-only images', () => {
	const participants = calendarParticipantsFromUnknown([
		{ personID: 'person-left', name: '김여명', email: 'left@example.com', image: '/calendar/api/participants/person-left/image' }
	]);

	expect(calendarParticipantInputs(participants)).toEqual([{ personID: 'person-left', name: '김여명', email: 'left@example.com' }]);
});

test('filters participant candidates by name email and id', () => {
	const participant = { personID: 'person-dongha', name: '이동하', email: 'dongha@example.com' };

	expect(calendarParticipantMatchesSearch(participant, '동하')).toBe(true);
	expect(calendarParticipantMatchesSearch(participant, 'dongha')).toBe(true);
	expect(calendarParticipantMatchesSearch(participant, 'person-dongha')).toBe(true);
	expect(calendarParticipantMatchesSearch(participant, '김여명')).toBe(false);
});
