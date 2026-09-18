import { describe, expect, test } from 'bun:test';
import { passkeyFailureText, refusalOf } from '../../src/lib/supabase-passkey';

describe('refusalOf', () => {
	test('dismissing the prompt is not a failure', () => {
		expect(refusalOf({ code: 'ERROR_CEREMONY_ABORTED', message: 'aborted' })).toBe('cancelled');
	});

	test('a platform that refuses the ceremony itself is a dismissal', () => {
		expect(
			refusalOf({
				code: 'ERROR_PASSTHROUGH_SEE_CAUSE_PROPERTY',
				name: 'NotAllowedError',
				message: 'The operation either timed out or was not allowed.'
			})
		).toBe('cancelled');
	});

	test('a dismissal named only by the cause still counts', () => {
		expect(
			refusalOf({
				code: 'ERROR_PASSTHROUGH_SEE_CAUSE_PROPERTY',
				name: 'Unknown Error',
				cause: { name: 'NotAllowedError' }
			})
		).toBe('cancelled');
	});

	test('the same code around an unexpected credential is a failure', () => {
		expect(
			refusalOf({
				code: 'ERROR_PASSTHROUGH_SEE_CAUSE_PROPERTY',
				name: 'WebAuthnUnknownError',
				message: 'Empty credential response',
				cause: {}
			})
		).toBe('failed');
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

	test('says what the browser said rather than the code standing in for it', () => {
		expect(
			passkeyFailureText('등록 실패', {
				code: 'ERROR_PASSTHROUGH_SEE_CAUSE_PROPERTY',
				message: 'Empty credential response'
			})
		).toBe('등록 실패 (Empty credential response)');
	});

	test('reads the reason the passthrough code points at', () => {
		expect(
			passkeyFailureText('등록 실패', {
				code: 'ERROR_PASSTHROUGH_SEE_CAUSE_PROPERTY',
				message: 'a Non-Webauthn related error has occurred',
				cause: new TypeError('Failed to fetch attestation options')
			})
		).toBe('등록 실패 (Failed to fetch attestation options)');
	});

	test('keeps the curated message of a code that carries one', () => {
		expect(
			passkeyFailureText('등록 실패', {
				code: 'ERROR_AUTHENTICATOR_MISSING_USER_VERIFICATION_SUPPORT',
				message: 'User verification was required but no available authenticator supported it',
				cause: new Error('The operation failed')
			})
		).toBe('등록 실패 (User verification was required but no available authenticator supported it)');
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
