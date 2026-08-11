import { describe, expect, test } from 'bun:test';
import { passkeyFailureText, refusalOf } from '../../src/lib/supabase-passkey';

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

describe('passkeyFailureText', () => {
	test('names the code the authenticator refused with', () => {
		expect(passkeyFailureText('등록 실패', { code: 'ERROR_INVALID_RP_ID' })).toBe(
			'등록 실패 (ERROR_INVALID_RP_ID)'
		);
	});

	test('falls back to the message when there is no code', () => {
		expect(passkeyFailureText('등록 실패', new Error('the relying party is not this origin'))).toBe(
			'등록 실패 (the relying party is not this origin)'
		);
	});

	test('says only what it knows when the failure carries nothing', () => {
		expect(passkeyFailureText('등록 실패', {})).toBe('등록 실패');
		expect(passkeyFailureText('등록 실패', null)).toBe('등록 실패');
	});
});
