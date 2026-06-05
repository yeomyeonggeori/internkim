import { describe, expect, test } from 'bun:test';

import {
	isLocalMailHostname,
	mailRequesterEmailHeader,
	mailRequesterEmailStorageKey,
	mailRequestHeaders,
	rememberLocalMailActorEmail,
	rememberMailActorEmail,
	resolveMailAccountSaveActorEmail,
	resolveMailActorEmail,
	storedMailActorEmail
} from '../../../src/routes/mail/mail-request-actor';

class TestMailActorStorage implements Pick<Storage, 'getItem' | 'setItem'> {
	private readonly values = new Map<string, string>();

	getItem(key: string) {
		return this.values.get(key) ?? null;
	}

	setItem(key: string, value: string) {
		this.values.set(key, value);
	}
}

describe('mail request actor', () => {
	test('detects local hostnames', () => {
		expect(isLocalMailHostname('127.0.0.1')).toBe(true);
		expect(isLocalMailHostname('::1')).toBe(true);
		expect(isLocalMailHostname('localhost')).toBe(true);
		expect(isLocalMailHostname('example.test')).toBe(false);
	});

	test('adds requester email header only for local pages', () => {
		expect(mailRequestHeaders(' Staff@Example.COM ', { 'Content-Type': 'application/json' }, '127.0.0.1')).toEqual({
			'Content-Type': 'application/json',
			[mailRequesterEmailHeader]: 'staff@example.com'
		});
		expect(mailRequestHeaders('staff@example.com', { 'Content-Type': 'application/json' }, 'app.example.test')).toEqual({
			'Content-Type': 'application/json'
		});
	});

	test('uses stored actor email before account and draft email', () => {
		const storage = new TestMailActorStorage();
		storage.setItem(mailRequesterEmailStorageKey, 'stored@example.com');

		expect(resolveMailActorEmail(' Account@Example.COM ', 'draft@example.com', storage)).toBe('stored@example.com');
		expect(resolveMailActorEmail('', '', storage)).toBe('stored@example.com');
		expect(resolveMailActorEmail(' Account@Example.COM ', 'draft@example.com', new TestMailActorStorage())).toBe('account@example.com');
		expect(resolveMailActorEmail('', ' Draft@Example.COM ', new TestMailActorStorage())).toBe('draft@example.com');
	});

	test('keeps stored actor email when saving an edited account address', () => {
		const storage = new TestMailActorStorage();
		storage.setItem(mailRequesterEmailStorageKey, 'owner@example.com');

		expect(resolveMailAccountSaveActorEmail('new-address@example.com', storage)).toBe('owner@example.com');
		expect(resolveMailAccountSaveActorEmail(' First@Example.COM ', new TestMailActorStorage())).toBe('first@example.com');
	});

	test('remembers normalized actor email', () => {
		const storage = new TestMailActorStorage();

		rememberMailActorEmail(' Staff@Example.COM ', storage);

		expect(storedMailActorEmail(storage)).toBe('staff@example.com');
	});

	test('remembers actor email only for local pages', () => {
		const storage = new TestMailActorStorage();

		rememberLocalMailActorEmail(' Staff@Example.COM ', 'app.example.test', storage);
		expect(storedMailActorEmail(storage)).toBe('');

		rememberLocalMailActorEmail(' Staff@Example.COM ', 'localhost', storage);
		expect(storedMailActorEmail(storage)).toBe('staff@example.com');
	});
});
