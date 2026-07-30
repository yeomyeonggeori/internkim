import { describe, expect, test } from 'bun:test';
import { personAvatarSeed } from '../../src/lib/person-avatar-seed';

describe('personAvatarSeed', () => {
	test('uses the email so the same person looks identical on every surface', () => {
		expect(personAvatarSeed('Kim@Example.com', 'kim-intern', '김철수')).toBe(
			personAvatarSeed('kim@example.com', 'person-42', '김철수')
		);
	});

	test('falls back to the caller seed, then the name', () => {
		expect(personAvatarSeed('', 'conversation-7', '김철수')).toBe('conversation-7');
		expect(personAvatarSeed('', '', '김철수')).toBe('김철수');
		expect(personAvatarSeed('', '', '')).toBe('?');
	});
});
