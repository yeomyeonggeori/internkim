import { describe, expect, test } from 'bun:test';
import { acceptedMemberPicture } from '../../src/lib/profile/member-picture';

describe('the messenger picture kept as a member picture', () => {
	test('keeps the types a member picture can be, a phone-saved heic among them', () => {
		expect(acceptedMemberPicture(new Blob(['x'], { type: 'image/jpeg' })).type).toBe('image/jpeg');
		expect(acceptedMemberPicture(new Blob(['x'], { type: 'image/heic' })).type).toBe('image/heic');
	});

	test('refuses a picture it cannot keep, rather than passing for someone with no picture', () => {
		expect(() => acceptedMemberPicture(new Blob(['<svg/>'], { type: 'image/svg+xml' }))).toThrow('image/svg+xml');
	});
});
