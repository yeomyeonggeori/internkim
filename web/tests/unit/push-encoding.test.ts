import { describe, expect, test } from 'bun:test';
import { decodeBase64URL, encodeBase64URL } from '../../src/lib/notifications/base64url';

const vapidPublicKey =
	'BG0w6CuCogoJKa593BzjeAk_VAOmSYtz4Crk7OBQPEYa3_peOcMJEln_GG6LyW-0nl82LPHDClzU8_0nB4Z5dcs';

describe('decodeBase64URL', () => {
	test('a VAPID key decodes to the 65 bytes an uncompressed P-256 point takes', () => {
		const decoded = decodeBase64URL(vapidPublicKey);
		expect(decoded.length).toBe(65);
		expect(decoded[0]).toBe(4);
	});

	test('the URL alphabet is read, not the standard one', () => {
		expect([...decodeBase64URL('-_8')]).toEqual([251, 255]);
	});

	test('padding the browser omitted is restored', () => {
		expect([...decodeBase64URL('AQ')]).toEqual([1]);
		expect([...decodeBase64URL('AQI')]).toEqual([1, 2]);
		expect([...decodeBase64URL('AQID')]).toEqual([1, 2, 3]);
	});
});

describe('encodeBase64URL', () => {
	test('a key the browser handed us survives the round trip', () => {
		expect(encodeBase64URL(decodeBase64URL(vapidPublicKey).buffer)).toBe(vapidPublicKey);
	});

	test('nothing encodes to nothing', () => {
		expect(encodeBase64URL(null)).toBe('');
	});

	test('what it writes carries no character a URL would have to escape', () => {
		const everyByte = new Uint8Array(256).map((_, index) => index);
		expect(encodeBase64URL(everyByte.buffer)).not.toMatch(/[+/=]/);
	});
});
