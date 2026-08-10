//   bun run web/scripts/make-vapid-keys.ts --subject mailto:someone@example.com

import { encodeBase64URL } from '../src/lib/notifications/base64url';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const subject = argument('subject') ?? '';
if (!subject.startsWith('mailto:') && !subject.startsWith('https://')) {
	throw new Error('pass --subject as a mailto: address or an https:// url; push services reject anything else');
}

const pair = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, true, ['sign', 'verify']);
const publicPoint = await crypto.subtle.exportKey('raw', pair.publicKey);
const privateJWK = await crypto.subtle.exportKey('jwk', pair.privateKey);
if (!privateJWK.d) throw new Error('the generated key carries no private scalar');

console.log(`VAPID_PUBLIC_KEY=${encodeBase64URL(publicPoint)}`);
console.log(`VAPID_PRIVATE_KEY=${privateJWK.d}`);
console.log(`VAPID_SUBJECT=${subject}`);
