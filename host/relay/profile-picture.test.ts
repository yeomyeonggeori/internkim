import { afterEach, describe, expect, test } from 'bun:test';
import { answerByteLength, largestRawBytesThatFit, oversizeNotice } from './answer-size';
import { readProfilePicture } from './mattermost';

const ceiling = 200_000;
const settings = { baseURL: 'https://mattermost.test', email: 'bot@example.com', password: 'x' };
const session = { token: 'bot-token', userID: 'bot-id' };
const realFetch = globalThis.fetch;

function servePicture(byteCount: number): void {
	globalThis.fetch = Object.assign(
		async () => new Response(new Uint8Array(byteCount), { headers: { 'content-type': 'image/png' } }),
		{ preconnect: realFetch.preconnect }
	);
}

afterEach(() => {
	globalThis.fetch = realFetch;
});

describe('readProfilePicture against the answer ceiling', () => {
	test('a picture at the derived limit still fits in an answer', async () => {
		const largestBytes = largestRawBytesThatFit(ceiling);
		servePicture(largestBytes);

		const picture = await readProfilePicture(settings, session, 'person-1', largestBytes);
		expect(picture).not.toBeNull();

		const answer = { callID: 'a', status: 200, body: picture };
		expect(answerByteLength(answer)).toBeLessThanOrEqual(ceiling);
		expect(oversizeNotice(answer, ceiling)).toBeNull();
	});

	test('a picture past the limit is dropped instead of taking the call down', async () => {
		const largestBytes = largestRawBytesThatFit(ceiling);
		servePicture(largestBytes + 1);

		expect(await readProfilePicture(settings, session, 'person-1', largestBytes)).toBeNull();
	});

	test('the limit is the one passed in, never a constant of its own', async () => {
		servePicture(50_000);

		expect(await readProfilePicture(settings, session, 'person-1', 40_000)).toBeNull();
		expect(await readProfilePicture(settings, session, 'person-1', 60_000)).not.toBeNull();
	});
});
