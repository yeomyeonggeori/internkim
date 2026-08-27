import { describe, expect, test } from 'bun:test';
import { personPicture } from '$lib/stores/person-picture.svelte';

// pictureOf answers from the directory the store loads for itself, and it is
// loaded by remember(), not by rememberExternals(). A caller that primes the
// pictures and then reads by member without going through remember() gets
// nothing — which is how the messenger's people list lost its avatars once.
// A caller holding a directory of its own reads by account instead.
describe('personPicture.pictureOf', () => {
	test('answers with nothing until the store has loaded a directory', () => {
		expect(personPicture.pictureOf({ memberID: 'member-1' })).toBe('');
		expect(personPicture.pictureOf({ email: 'sample@example.com' })).toBe('');
	});

	test('answers with nothing for an account it has not been told about', () => {
		expect(personPicture.pictureOfExternal('an-account-nobody-asked-after')).toBe('');
	});
});
