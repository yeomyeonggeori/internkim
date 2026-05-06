import { describe, expect, test } from 'vitest';
import { isCompanionVerified, normalizeManualPairingInput, parsePairingLink, statusLabel } from '../src/lib/pairing';

describe('pairing links', () => {
	test('parses internkim pair links', () => {
		const payload = parsePairingLink('internkim://pair?device_url=https%3A%2F%2Fabc.intern.kim&code=ABCD-1234');
		expect(payload).toEqual({ deviceURL: 'https://abc.intern.kim', code: 'ABCD-1234' });
	});

	test('rejects non-http device links', () => {
		expect(() => parsePairingLink('internkim://pair?device_url=file%3A%2F%2Ftmp&code=ABCD-1234')).toThrow();
	});
});

describe('manual pairing', () => {
	test('normalizes slash and code casing', () => {
		const payload = normalizeManualPairingInput('https://abc.intern.kim/', 'abcd-1234');
		expect(payload).toEqual({ deviceURL: 'https://abc.intern.kim', code: 'ABCD-1234' });
	});
});

describe('status label', () => {
	test('shows unpaired state', () => {
		expect(statusLabel({ paired: false })).toBe('Not connected');
	});

	test('shows connected device', () => {
		expect(statusLabel({ paired: true, authStatus: 'verified', deviceURL: 'https://abc.intern.kim' })).toBe('Connected to https://abc.intern.kim');
	});

	test('does not show connected from paired state without verified auth', () => {
		expect(statusLabel({ paired: true, deviceURL: 'https://abc.intern.kim' })).toBe('Connection unknown');
		expect(isCompanionVerified({ paired: true, deviceURL: 'https://abc.intern.kim' })).toBe(false);
	});

	test('shows reconnect state when signing key is missing', () => {
		expect(statusLabel({ paired: true, authStatus: 'missing-signing-key', deviceURL: 'https://abc.intern.kim' })).toBe('Reconnect required');
	});
});
