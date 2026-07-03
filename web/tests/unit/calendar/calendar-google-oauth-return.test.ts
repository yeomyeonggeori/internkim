import { describe, expect, test } from 'bun:test';
import {
	googleOAuthReturnStatusFromMessageEvent,
	googleOAuthReturnStatusFromStorageEvent,
	googleOAuthReturnStatusFromURL,
	googleOAuthReturnStorageKey,
	notifyGoogleOAuthReturn
} from '../../../src/routes/calendar/calendar-google-oauth-return';

describe('calendar Google OAuth return', () => {
	test('reads OAuth return status from URL', () => {
		expect(googleOAuthReturnStatusFromURL(new URL('http://127.0.0.1:5174/calendar/?googleOAuth=connected'))).toBe('connected');
		expect(googleOAuthReturnStatusFromURL(new URL('http://127.0.0.1:5174/calendar/?googleOAuth=failed'))).toBe('failed');
		expect(googleOAuthReturnStatusFromURL(new URL('http://127.0.0.1:5174/calendar/?googleOAuth=other'))).toBe(null);
	});

	test('reads OAuth return status from storage event', () => {
		const event = {
			key: googleOAuthReturnStorageKey,
			newValue: JSON.stringify({ status: 'connected', issuedAt: 123 })
		} as StorageEvent;

		expect(googleOAuthReturnStatusFromStorageEvent(event)).toBe('connected');
	});

	test('reads OAuth return status from same-origin popup message', () => {
		const event = {
			origin: 'http://127.0.0.1:5174',
			data: {
				type: 'internkim:calendar:google-oauth-return',
				status: 'connected'
			}
		} as MessageEvent;

		expect(googleOAuthReturnStatusFromMessageEvent(event, 'http://127.0.0.1:5174')).toBe('connected');
	});

	test('ignores OAuth return messages from another origin', () => {
		const event = {
			origin: 'https://evil.example',
			data: {
				type: 'internkim:calendar:google-oauth-return',
				status: 'connected'
			}
		} as MessageEvent;

		expect(googleOAuthReturnStatusFromMessageEvent(event, 'http://127.0.0.1:5174')).toBe(null);
	});

	test('does not throw when OAuth return storage notification fails', () => {
		const originalWindowDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'window');
		Object.defineProperty(globalThis, 'window', {
			configurable: true,
			value: {
				localStorage: {
					setItem() {
						throw new Error('storage disabled');
					}
				}
			}
		});

		try {
			expect(() => notifyGoogleOAuthReturn('connected')).not.toThrow();
		} finally {
			if (originalWindowDescriptor) {
				Object.defineProperty(globalThis, 'window', originalWindowDescriptor);
			} else {
				Reflect.deleteProperty(globalThis, 'window');
			}
		}
	});
});
