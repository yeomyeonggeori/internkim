import { afterEach, describe, expect, test } from 'bun:test';
import { answerByteLength, defaultAnswerByteCeiling, oversizeNotice } from './answer-size';
import { listCustomEmoji, readCustomEmojiImage } from './mattermost';

const settings = { baseURL: 'https://mattermost.test', email: 'bot@example.com', password: 'x' };
const token = 'member-token';
const realFetch = globalThis.fetch;

function serveSet(count: number, imageBytes: number): { pathsAsked: string[] } {
	const pathsAsked: string[] = [];
	const listed = Array.from({ length: count }, (_unused, index) => ({
		id: `emoji-${index}`,
		name: `party_parrot_${index}`
	}));
	globalThis.fetch = Object.assign(
		async (input: string | URL | Request) => {
			const path = new URL(String(input)).pathname;
			pathsAsked.push(path);
			if (path.endsWith('/image')) {
				return new Response(new Uint8Array(imageBytes), { headers: { 'content-type': 'image/png' } });
			}
			return new Response(JSON.stringify(listed), {
				headers: { 'content-type': 'application/json' }
			});
		},
		{ preconnect: realFetch.preconnect }
	);
	return { pathsAsked };
}

afterEach(() => {
	globalThis.fetch = realFetch;
});

describe('the emoji set is an index, not every drawing at once', () => {
	test('listing draws nothing, so a large set still fits in one answer', async () => {
		const { pathsAsked } = serveSet(200, 90_000);

		const listed = await listCustomEmoji(settings, token);

		expect(listed).toHaveLength(200);
		expect(pathsAsked.filter((path) => path.endsWith('/image'))).toHaveLength(0);
		const answer = { callID: 'a', status: 200, body: listed.map(({ name }) => ({ name })) };
		expect(oversizeNotice(answer, defaultAnswerByteCeiling)).toBeNull();
		expect(answerByteLength(answer)).toBeLessThan(defaultAnswerByteCeiling);
	});

	test('a drawing is fetched one at a time, by the id the index carried', async () => {
		const { pathsAsked } = serveSet(3, 1_000);

		const drawn = await readCustomEmojiImage(settings, token, 'emoji-1', 100_000);

		expect(drawn?.dataURL.startsWith('data:image/png;base64,')).toBe(true);
		expect(pathsAsked).toEqual(['/api/v4/emoji/emoji-1/image']);
	});

	test('a drawing past the limit is dropped rather than sent', async () => {
		serveSet(1, 120_000);

		expect(await readCustomEmojiImage(settings, token, 'emoji-0', 100_000)).toBeNull();
	});
});
