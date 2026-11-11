import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import type { avatarPresenceOf as AvatarPresenceOf } from '$lib/messenger/member-presence.svelte';

const labels = { presenceOnline: '접속 중', presenceOffline: '접속하지 않음' };
let originalState: unknown;
let avatarPresenceOf: typeof AvatarPresenceOf;

beforeAll(async () => {
	originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	({ avatarPresenceOf } = await import('$lib/messenger/member-presence.svelte'));
});

afterAll(() => {
	Reflect.set(globalThis, '$state', originalState);
});

describe('avatarPresenceOf', () => {
	test('shows the agent as online, since it is always on', () => {
		expect(avatarPresenceOf({ isAgent: true }, labels)).toEqual({ isOnline: true, presenceLabel: '접속 중' });
	});

	test('shows a member nobody hears from as offline', () => {
		expect(avatarPresenceOf({ memberID: 'member-1' }, labels)).toEqual({ isOnline: false, presenceLabel: '접속하지 않음' });
	});

	test('shows nothing for a person who is not a member', () => {
		expect(avatarPresenceOf({}, labels)).toEqual({});
		expect(avatarPresenceOf(undefined, labels)).toEqual({});
	});
});
