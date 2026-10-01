import type { SealedSecret } from '../../../src/lib/company/box';
import { boxSealingSuite, bytesOfBase64URL, type SealPurpose } from '../../../src/lib/company/seal-to-box';

export async function openedBy(boxSecretKey: Uint8Array, sealed: SealedSecret, purpose: SealPurpose): Promise<string> {
	const recipientKey = await boxSealingSuite.kem.importKey('raw', new Uint8Array(boxSecretKey).buffer, false);
	const opened = await boxSealingSuite.open(
		{
			recipientKey,
			enc: bytesOfBase64URL(sealed.enc),
			info: new TextEncoder().encode(purpose.information)
		},
		bytesOfBase64URL(sealed.ciphertext),
		new TextEncoder().encode(purpose.additionalData)
	);
	return new TextDecoder().decode(opened);
}
