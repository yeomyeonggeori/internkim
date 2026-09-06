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
		{ personID: 'person-left', name: '김예시', email: 'left@example.com', image: '/calendar/api/participants/person-left/image' },
		{ personID: 'person-right', name: '김예시', email: 'right@example.com' }
	]);

	expect(participants.length).toBe(2);
	expect(participants.map(calendarParticipantKey)).toEqual(['id:person-left', 'id:person-right']);
	expect(participants[0].image).toBe('/calendar/api/participants/person-left/image');
	expect(calendarParticipantOptionLabel(participants[0], participants)).toBe('김예시 · left@example.com');
});

test('keeps duplicate people distinguishable when their displayed name is localized', () => {
	const participants = [
		{ personID: 'left', name: '예시 박', email: 'left@example.com' },
		{ personID: 'right', name: '예시 박', email: 'right@example.com' }
	];
	expect(calendarParticipantOptionLabel(participants[0], participants, '박예시')).toBe('박예시 · left@example.com');
	expect(calendarParticipantOptionLabel(participants[1], participants, '박예시')).toBe('박예시 · right@example.com');
	expect(participants[0].name).toBe('예시 박');
});

test('builds participant payloads without response-only images', () => {
	const participants = calendarParticipantsFromUnknown([
		{ personID: 'person-left', name: '김예시', email: 'left@example.com', image: '/calendar/api/participants/person-left/image' }
	]);

	expect(calendarParticipantInputs(participants)).toEqual([{ personID: 'person-left', name: '김예시', email: 'left@example.com' }]);
});

test('filters participant candidates by name email and id', () => {
	const participant = { personID: 'person-gamyeong', name: '이샘플', email: 'gamyeong@example.com' };

	expect(calendarParticipantMatchesSearch(participant, '샘플')).toBe(true);
	expect(calendarParticipantMatchesSearch(participant, 'gamyeong')).toBe(true);
	expect(calendarParticipantMatchesSearch(participant, 'person-gamyeong')).toBe(true);
	expect(calendarParticipantMatchesSearch(participant, '김예시')).toBe(false);
});
