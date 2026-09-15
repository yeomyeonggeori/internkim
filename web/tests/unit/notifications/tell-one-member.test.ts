import { describe, expect, test } from 'bun:test';
import { tellingAsked } from '../../../../supabase/functions/_shared/tell-one-member.ts';

const uniquely = 'once';

describe('the plane asks the project to tell one member', () => {
	test('a member, a known category and a title are enough', () => {
		expect(
			tellingAsked({ memberID: 'member-one', category: 'attendance', title: '퇴근을 안 찍었어요' }, uniquely)
		).toEqual({
			memberID: 'member-one',
			category: 'attendance',
			title: '퇴근을 안 찍었어요',
			body: '',
			openPath: '/attendance/',
			tag: 'attendance-once'
		});
	});

	test('an open path and a tag are carried when given', () => {
		const telling = tellingAsked(
			{
				memberID: 'member-one',
				category: 'task',
				title: '제목',
				body: '본문',
				openPath: '/attendance/',
				tag: 'shift-one'
			},
			uniquely
		);

		expect(telling?.openPath).toBe('/attendance/');
		expect(telling?.tag).toBe('shift-one');
	});

	test('a category the plane does not have is refused rather than guessed', () => {
		expect(tellingAsked({ memberID: 'member-one', category: 'gossip', title: '제목' }, uniquely)).toBeNull();
	});

	test('no member and no title are each a refusal', () => {
		expect(tellingAsked({ memberID: '', category: 'attendance', title: '제목' }, uniquely)).toBeNull();
		expect(tellingAsked({ memberID: 'member-one', category: 'attendance', title: '  ' }, uniquely)).toBeNull();
	});

	test('anything that is not a string is read as nothing given', () => {
		expect(tellingAsked({ memberID: 'member-one', category: 'attendance', title: '제목', body: 42 }, uniquely)?.body).toBe('');
	});

	test('a line longer than the push services carry is cut', () => {
		const long = 'ㄱ'.repeat(500);
		expect(tellingAsked({ memberID: 'member-one', category: 'attendance', title: long }, uniquely)?.title).toHaveLength(200);
	});

	test('the member who sent it is carried when named, and left out when not', () => {
		const named = tellingAsked(
			{ memberID: 'member-one', category: 'attendance', title: '제목', senderMemberID: 'member-two' },
			uniquely
		);
		const unnamed = tellingAsked({ memberID: 'member-one', category: 'attendance', title: '제목', senderMemberID: 7 }, uniquely);

		expect(named?.senderMemberID).toBe('member-two');
		expect(unnamed).not.toHaveProperty('senderMemberID');
	});

	test('two tellings of one category do not share a tag, so neither replaces the other', () => {
		const first = tellingAsked({ memberID: 'member-one', category: 'attendance', title: '제목' }, 'first');
		const second = tellingAsked({ memberID: 'member-one', category: 'attendance', title: '제목' }, 'second');

		expect(first?.tag).not.toBe(second?.tag);
	});
});
