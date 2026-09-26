import { SignJWT, errors, importJWK, jwtVerify, type JWTPayload } from 'jose';

export type RecordIdentity = {
	userID: string;
	email: string | null;
	appMetadata?: Record<string, string>;
};

export type RecordToken = { accessToken: string; expiresAt: number };

export const recordTokenLifetimeSeconds = 60 * 60;

type SigningKey = { kty: string; kid?: string; [member: string]: unknown };

const algorithmByKeyType: Record<string, string> = {
	EC: 'ES256',
	RSA: 'RS256',
	OKP: 'EdDSA',
	oct: 'HS256',
};

function signingKeyOf(written: string): SigningKey {
	const parsed: unknown = JSON.parse(written);
	if (!parsed || typeof parsed !== 'object' || !('kty' in parsed) || typeof parsed.kty !== 'string') {
		throw new Error('the record signing key is not a JWK');
	}
	const kid = 'kid' in parsed && typeof parsed.kid === 'string' ? parsed.kid : undefined;
	return { ...parsed, kty: parsed.kty, kid };
}

function algorithmOf(key: SigningKey): string {
	const algorithm = algorithmByKeyType[key.kty];
	if (!algorithm) throw new Error(`no algorithm signs with a ${key.kty} key`);
	return algorithm;
}

export async function recordTokenFor(
	signingKey: string,
	projectURL: string,
	identity: RecordIdentity,
	now: Date = new Date(),
): Promise<RecordToken> {
	const key = signingKeyOf(signingKey);
	const algorithm = algorithmOf(key);
	const issuedAt = Math.floor(now.getTime() / 1000);
	const expiresAt = issuedAt + recordTokenLifetimeSeconds;
	const accessToken = await new SignJWT({
		role: 'authenticated',
		email: identity.email ?? undefined,
		app_metadata: identity.appMetadata ?? {},
		is_anonymous: false,
	})
		.setProtectedHeader({ alg: algorithm, typ: 'JWT', kid: key.kid })
		.setIssuer(`${projectURL}/auth/v1`)
		.setSubject(identity.userID)
		.setAudience('authenticated')
		.setIssuedAt(issuedAt)
		.setExpirationTime(expiresAt)
		.sign(await importJWK(keyMaterialOf(key), algorithm));
	return { accessToken, expiresAt };
}

const keyMaterialMembers = new Set(['kid', 'crv', 'x', 'y', 'd', 'k', 'n', 'e', 'p', 'q', 'dp', 'dq', 'qi']);

function keyMaterialOf(key: SigningKey): SigningKey {
	const material = Object.entries(key).filter(([member]) => keyMaterialMembers.has(member));
	return { kty: key.kty, ...Object.fromEntries(material) };
}

const privateKeyMembers = new Set(['d', 'p', 'q', 'dp', 'dq', 'qi']);

function verifyingKeyOf(key: SigningKey): SigningKey {
	if (key.kty === 'oct') return keyMaterialOf(key);
	const material = Object.entries(keyMaterialOf(key)).filter(([member]) => !privateKeyMembers.has(member));
	return { ...Object.fromEntries(material), kty: key.kty };
}

export async function verifiedRecordToken(
	signingKey: string,
	projectURL: string,
	accessToken: string,
): Promise<JWTPayload | null> {
	const key = signingKeyOf(signingKey);
	const algorithm = algorithmOf(key);
	const verifyingKey = await importJWK(verifyingKeyOf(key), algorithm);
	try {
		const { payload } = await jwtVerify(accessToken, verifyingKey, {
			issuer: `${projectURL}/auth/v1`,
			audience: 'authenticated',
			algorithms: [algorithm],
		});
		return payload;
	} catch (failure) {
		if (failure instanceof errors.JOSEError) return null;
		throw failure;
	}
}
