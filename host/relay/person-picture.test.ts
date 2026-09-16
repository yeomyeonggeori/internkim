import { describe, expect, test } from 'bun:test';
import {
	digestInURL,
	MessengerAnswered,
	PersonPictures,
	pictureOf,
	type PersonPictureSources
} from './person-picture';

const pictureAnswer = { status: 200, body: { image: { dataURL: 'data:image/png;base64,AQID' } } };
const digest = 'a'.repeat(64);
const namedPicture = `https://relay.example.com/media/${digest}.png`;
const actor = { kind: 'buzz-secret', secret: 's1' };

function sourcesWith(overrides: Partial<PersonPictureSources> = {}) {
	const asked: Record<string, unknown>[] = [];
	const kept: { bytes: number[]; contentType: string }[] = [];
	const lookedFor: string[] = [];
	const reported: string[] = [];
	let clock = 0;
	const sources: PersonPictureSources = {
		keptAlready: async (wanted) => {
			lookedFor.push(wanted);
			return null;
		},
		askChatd: async (_capability, body) => {
			asked.push(body);
			return pictureAnswer;
		},
		keep: async (bytes, contentType) => {
			kept.push({ bytes: [...bytes], contentType });
			return 'company/shared/person-picture/digest.png';
		},
		memberIDOf: async (externalID) => (externalID === 'npub-member' ? 'member-1' : null),
		credentialOf: async () => actor,
		report: (line) => reported.push(line),
		now: () => clock,
		...overrides
	};
	return { sources, asked, kept, lookedFor, reported, advance: (milliseconds: number) => (clock += milliseconds) };
}

describe('the kept copy of a picture', () => {
	test('one the company already holds is named without the messenger being asked', async () => {
		const held = sourcesWith({ keptAlready: async () => 'company/shared/person-picture/held.png' });
		const path = await new PersonPictures(held.sources).keptPathOf({ externalID: 'npub-member', avatarURL: namedPicture, actor });
		expect(path).toBe('company/shared/person-picture/held.png');
		expect(held.asked).toHaveLength(0);
	});

	test('one the company has never seen is read as the asker and kept', async () => {
		const held = sourcesWith();
		const path = await new PersonPictures(held.sources).keptPathOf({ externalID: 'npub-member', avatarURL: namedPicture, actor });
		expect(path).toBe('company/shared/person-picture/digest.png');
		expect(held.lookedFor).toEqual([digest]);
		expect(held.asked).toEqual([{ actor, externalID: 'npub-member' }]);
		expect(held.kept).toEqual([{ bytes: [1, 2, 3], contentType: 'image/png' }]);
	});

	test('a person who set no picture has none, and nothing is kept', async () => {
		const held = sourcesWith({ askChatd: async () => ({ status: 200, body: { image: null } }) });
		expect(await new PersonPictures(held.sources).keptPathOf({ externalID: 'npub-member', avatarURL: '', actor })).toBe('');
		expect(held.kept).toHaveLength(0);
	});

	test('a picture the messenger names and then hands none of is a failure, not an absence', async () => {
		const held = sourcesWith({ askChatd: async () => ({ status: 200, body: { image: null } }) });
		const read = new PersonPictures(held.sources).keptPathOf({ externalID: 'npub-member', avatarURL: namedPicture, actor });
		await expect(read).rejects.toBeInstanceOf(MessengerAnswered);
		await expect(read).rejects.toMatchObject({ status: 502 });
	});

	test('a messenger that refused is answered with its own status', async () => {
		const held = sourcesWith({ askChatd: async () => ({ status: 401, body: null }) });
		const read = new PersonPictures(held.sources).keptPathOf({ externalID: 'npub-member', avatarURL: '', actor });
		await expect(read).rejects.toMatchObject({ status: 401, message: 'person.picture answered 401' });
	});
});

describe('the sender picture on a notification', () => {
	test('is read as the sender and kept where the phone can fetch it', async () => {
		const held = sourcesWith();
		expect(await new PersonPictures(held.sources).pathForNotification('npub-member')).toBe(
			'company/shared/person-picture/digest.png'
		);
		expect(held.asked).toEqual([{ actor, externalID: 'npub-member' }]);
	});

	test('is read once for a burst of messages, and again after an hour', async () => {
		const held = sourcesWith();
		const pictures = new PersonPictures(held.sources);
		await pictures.pathForNotification('npub-member');
		await pictures.pathForNotification('npub-member');
		expect(held.asked).toHaveLength(1);
		held.advance(60 * 60 * 1000);
		await pictures.pathForNotification('npub-member');
		expect(held.asked).toHaveLength(2);
	});

	test('is nothing for someone who is not a member, or a member with no messenger credential', async () => {
		const stranger = sourcesWith();
		expect(await new PersonPictures(stranger.sources).pathForNotification('npub-contact')).toBe('');
		const uncredentialed = sourcesWith({ credentialOf: async () => null });
		expect(await new PersonPictures(uncredentialed.sources).pathForNotification('npub-member')).toBe('');
		expect(stranger.asked).toHaveLength(0);
		expect(uncredentialed.asked).toHaveLength(0);
	});

	test('a failed read is reported and tried again on the next message', async () => {
		let answers = 0;
		const held = sourcesWith({
			askChatd: async () => (++answers === 1 ? { status: 502, body: null } : pictureAnswer)
		});
		const pictures = new PersonPictures(held.sources);
		expect(await pictures.pathForNotification('npub-member')).toBe('');
		expect(held.reported[0]).toContain('person.picture answered 502');
		expect(await pictures.pathForNotification('npub-member')).toBe('company/shared/person-picture/digest.png');
	});

	test('gives up on a picture that does not come in time, so the notification still goes out', async () => {
		const held = sourcesWith({ askChatd: () => new Promise(() => {}), timeLimitMilliseconds: 10 });
		expect(await new PersonPictures(held.sources).pathForNotification('npub-member')).toBe('');
		expect(held.reported[0]).toContain('no answer within 10ms');
	});
});

describe('digestInURL', () => {
	test('reads the hash a blossom media url is named by, with or without an extension', () => {
		expect(digestInURL(namedPicture)).toBe(digest);
		expect(digestInURL(`https://relay.example.com/media/${digest}`)).toBe(digest);
	});

	test('names nothing for a url that is not addressed by content', () => {
		expect(digestInURL('https://example.com/avatars/me.png')).toBe('');
		expect(digestInURL('not a url')).toBe('');
		expect(digestInURL('')).toBe('');
	});
});

describe('pictureOf', () => {
	test('reads the type and bytes out of a data url', () => {
		expect(pictureOf(pictureAnswer.body)).toEqual({ contentType: 'image/png', bytes: new Uint8Array([1, 2, 3]) });
	});

	test('refuses an answer that carries no data url', () => {
		expect(pictureOf({ image: { dataURL: 'https://example.com/a.png' } })).toBeNull();
		expect(pictureOf(null)).toBeNull();
	});
});
