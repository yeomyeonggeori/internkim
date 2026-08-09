import { describe, expect, test } from 'bun:test';
import { refusalOf } from '../../src/lib/supabase-passkey';

describe('refusalOf', () => {
	test('dismissing the prompt is not a failure', () => {
		expect(refusalOf({ code: 'ERROR_CEREMONY_ABORTED', message: 'aborted' })).toBe('cancelled');
	});

	test('a device that already holds one says so', () => {
		expect(refusalOf({ code: 'ERROR_AUTHENTICATOR_PREVIOUSLY_REGISTERED' })).toBe('already-registered');
	});

	test('every other ceremony error is a failure', () => {
		expect(refusalOf({ code: 'ERROR_INVALID_RP_ID' })).toBe('failed');
		expect(refusalOf({ code: 'ERROR_AUTHENTICATOR_GENERAL_ERROR' })).toBe('failed');
	});

	test('an error carrying no code is a failure', () => {
		expect(refusalOf(new Error('the network went away'))).toBe('failed');
		expect(refusalOf({ code: 7 })).toBe('failed');
		expect(refusalOf(null)).toBe('failed');
		expect(refusalOf('ERROR_CEREMONY_ABORTED')).toBe('failed');
	});
});
