import { afterAll, beforeAll, describe, expect, test } from 'bun:test';

let originalState: unknown;
let originalFetch: typeof fetch;
let personPicture: {
	remember(people: { memberID?: string; email?: string }[]): Promise<void>;
	pictureOf(person: { memberID?: string; email?: string }): string;
};

let served: { pictures: { email: string; pictureURL?: string }[] } | null = null;
let servedCalls = 0;

beforeAll(async () => {
	originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	originalFetch = globalThis.fetch;
	globalThis.fetch = ((input: RequestInfo | URL) => {
		if (String(input).includes('/agent/api/person-pictures')) {
			servedCalls += 1;
			if (!served) return Promise.reject(new Error('the host is unreachable'));
			return Promise.resolve(new Response(JSON.stringify(served)));
		}
		return originalFetch(input);
	}) as typeof fetch;
	({ personPicture } = await import('$lib/stores/person-picture.svelte'));
});

afterAll(() => {
	globalThis.fetch = originalFetch;
	if (originalState === undefined) Reflect.deleteProperty(globalThis, '$state');
	else Reflect.set(globalThis, '$state', originalState);
});

describe('personPicture on a device host', () => {
	test('an unreachable host is asked again, then every face reads from one directory', async () => {
		served = null;
		await personPicture.remember([{ email: 'sample@example.com' }]);
		expect(personPicture.pictureOf({ email: 'sample@example.com' })).toBe('');

		served = {
			pictures: [
				{ email: 'sample@example.com', pictureURL: 'https://media.example.com/sample.png' },
				{ email: 'bare@example.com' }
			]
		};
		await personPicture.remember([{ email: 'sample@example.com' }]);
		expect(personPicture.pictureOf({ email: 'Sample@Example.com ' })).toBe('https://media.example.com/sample.png');
		expect(personPicture.pictureOf({ email: 'bare@example.com' })).toBe('');
	});

	test('the directory is fetched once for every surface that asks', async () => {
		const before = servedCalls;
		await personPicture.remember([{ email: 'bare@example.com' }]);
		await personPicture.remember([{ email: 'sample@example.com' }]);
		expect(servedCalls).toBe(before);
	});
});
