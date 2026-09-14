import { describe, expect, test } from 'bun:test';
import { pictureOf, SenderPictures, type SenderPictureSources } from './sender-picture';

const pictureAnswer = { status: 200, body: { image: { dataURL: 'data:image/png;base64,AQID' } } };

function sourcesWith(overrides: Partial<SenderPictureSources> = {}) {
	const asked: Record<string, unknown>[] = [];
	const kept: { bytes: number[]; contentType: string }[] = [];
	const reported: string[] = [];
	let clock = 0;
	const sources: SenderPictureSources = {
		memberIDOf: async (externalID) => (externalID === 'npub-member' ? 'member-1' : null),
		credentialOf: async () => ({ kind: 'buzz-secret', secret: 's1' }),
		askChatd: async (_capability, body) => {
			asked.push(body);
			return pictureAnswer;
		},
		keep: async (bytes, contentType) => {
			kept.push({ bytes: [...bytes], contentType });
			return 'company/shared/sender-picture/digest.png';
		},
		report: (line) => reported.push(line),
		now: () => clock,
		...overrides
	};
	return { sources, asked, kept, reported, advance: (milliseconds: number) => (clock += milliseconds) };
}

describe('SenderPictures', () => {
	test('reads the picture as the sender and keeps a copy the phone can fetch', async () => {
		const held = sourcesWith();
		const path = await new SenderPictures(held.sources).pathOf('npub-member');
		expect(path).toBe('company/shared/sender-picture/digest.png');
		expect(held.asked).toEqual([{ actor: { kind: 'buzz-secret', secret: 's1' }, externalID: 'npub-member' }]);
		expect(held.kept).toEqual([{ bytes: [1, 2, 3], contentType: 'image/png' }]);
	});

	test('reads a picture once for a burst of messages, and again after an hour', async () => {
		const held = sourcesWith();
		const pictures = new SenderPictures(held.sources);
		await pictures.pathOf('npub-member');
		await pictures.pathOf('npub-member');
		expect(held.asked).toHaveLength(1);
		held.advance(60 * 60 * 1000);
		await pictures.pathOf('npub-member');
		expect(held.asked).toHaveLength(2);
	});

	test('names no picture for someone who is not a member', async () => {
		const held = sourcesWith();
		expect(await new SenderPictures(held.sources).pathOf('npub-contact')).toBe('');
		expect(held.asked).toHaveLength(0);
	});

	test('names no picture for a member with no messenger credential', async () => {
		const held = sourcesWith({ credentialOf: async () => null });
		expect(await new SenderPictures(held.sources).pathOf('npub-member')).toBe('');
		expect(held.asked).toHaveLength(0);
	});

	test('names no picture for a sender who set none', async () => {
		const held = sourcesWith({ askChatd: async () => ({ status: 200, body: { image: null } }) });
		expect(await new SenderPictures(held.sources).pathOf('npub-member')).toBe('');
		expect(held.kept).toHaveLength(0);
	});

	test('a failed read is reported and tried again on the next message', async () => {
		let answers = 0;
		const held = sourcesWith({
			askChatd: async () => (++answers === 1 ? { status: 502, body: null } : pictureAnswer)
		});
		const pictures = new SenderPictures(held.sources);
		expect(await pictures.pathOf('npub-member')).toBe('');
		expect(held.reported[0]).toContain('person.picture answered 502');
		expect(await pictures.pathOf('npub-member')).toBe('company/shared/sender-picture/digest.png');
	});
});

describe('SenderPictures under a slow messenger', () => {
	test('gives up on a picture that does not come in time, so the notification still goes out', async () => {
		const held = sourcesWith({
			askChatd: () => new Promise(() => {}),
			timeLimitMilliseconds: 10
		});
		expect(await new SenderPictures(held.sources).pathOf('npub-member')).toBe('');
		expect(held.reported[0]).toContain('no answer within 10ms');
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
