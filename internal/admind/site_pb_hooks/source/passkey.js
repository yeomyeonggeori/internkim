import { p256 } from "@noble/curves/p256";
import { sha256 } from "@noble/hashes/sha256";

const BASE64URL_ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";

function base64urlDecode(text) {
	const cleaned = String(text || "").replace(/=+$/, "");
	const bytes = [];
	let buffer = 0;
	let bits = 0;
	for (const character of cleaned) {
		const value = BASE64URL_ALPHABET.indexOf(character);
		if (value < 0) continue;
		buffer = (buffer << 6) | value;
		bits += 6;
		if (bits >= 8) {
			bits -= 8;
			bytes.push((buffer >> bits) & 0xff);
		}
	}
	return new Uint8Array(bytes);
}

function base64urlEncode(bytes) {
	let output = "";
	let buffer = 0;
	let bits = 0;
	for (const byte of bytes) {
		buffer = (buffer << 8) | byte;
		bits += 8;
		while (bits >= 6) {
			bits -= 6;
			output += BASE64URL_ALPHABET[(buffer >> bits) & 0x3f];
		}
	}
	if (bits > 0) output += BASE64URL_ALPHABET[(buffer << (6 - bits)) & 0x3f];
	return output;
}

function utf8Decode(bytes) {
	let output = "";
	let index = 0;
	while (index < bytes.length) {
		const byte = bytes[index];
		if (byte < 0x80) {
			output += String.fromCharCode(byte);
			index += 1;
		} else if (byte < 0xe0) {
			output += String.fromCharCode(((byte & 0x1f) << 6) | (bytes[index + 1] & 0x3f));
			index += 2;
		} else if (byte < 0xf0) {
			output += String.fromCharCode(((byte & 0x0f) << 12) | ((bytes[index + 1] & 0x3f) << 6) | (bytes[index + 2] & 0x3f));
			index += 3;
		} else {
			const codePoint = ((byte & 0x07) << 18) | ((bytes[index + 1] & 0x3f) << 12) | ((bytes[index + 2] & 0x3f) << 6) | (bytes[index + 3] & 0x3f);
			output += String.fromCodePoint(codePoint);
			index += 4;
		}
	}
	return output;
}

function decodeCBOR(bytes) {
	let offset = 0;

	function readLength(additional) {
		if (additional < 24) return additional;
		if (additional === 24) return bytes[offset++];
		if (additional === 25) {
			const value = (bytes[offset] << 8) | bytes[offset + 1];
			offset += 2;
			return value;
		}
		if (additional === 26) {
			const value = (bytes[offset] << 24) | (bytes[offset + 1] << 16) | (bytes[offset + 2] << 8) | bytes[offset + 3];
			offset += 4;
			return value >>> 0;
		}
		throw new Error("cbor length too large");
	}

	function readItem() {
		const initial = bytes[offset++];
		const major = initial >> 5;
		const additional = initial & 0x1f;
		if (major === 0) return readLength(additional);
		if (major === 1) return -1 - readLength(additional);
		if (major === 2) {
			const length = readLength(additional);
			const value = bytes.slice(offset, offset + length);
			offset += length;
			return value;
		}
		if (major === 3) {
			const length = readLength(additional);
			const value = utf8Decode(bytes.slice(offset, offset + length));
			offset += length;
			return value;
		}
		if (major === 4) {
			const length = readLength(additional);
			const items = [];
			for (let index = 0; index < length; index += 1) items.push(readItem());
			return items;
		}
		if (major === 5) {
			const length = readLength(additional);
			const map = new Map();
			for (let index = 0; index < length; index += 1) {
				const key = readItem();
				map.set(key, readItem());
			}
			return map;
		}
		if (major === 7) {
			if (additional === 20) return false;
			if (additional === 21) return true;
			if (additional === 22) return null;
			throw new Error("cbor float unsupported");
		}
		throw new Error("cbor tag unsupported");
	}

	return readItem();
}

function concatBytes(first, second) {
	const output = new Uint8Array(first.length + second.length);
	output.set(first, 0);
	output.set(second, first.length);
	return output;
}

function parseAuthenticatorData(authenticatorData) {
	const flags = authenticatorData[32];
	const counter = ((authenticatorData[33] << 24) | (authenticatorData[34] << 16) | (authenticatorData[35] << 8) | authenticatorData[36]) >>> 0;
	const parsed = { rpIDHash: authenticatorData.slice(0, 32), flags, counter };
	if (flags & 0x40) {
		const credentialIDLength = (authenticatorData[53] << 8) | authenticatorData[54];
		parsed.credentialID = authenticatorData.slice(55, 55 + credentialIDLength);
		parsed.credentialPublicKeyCBOR = authenticatorData.slice(55 + credentialIDLength);
	}
	return parsed;
}

function coseKeyToUncompressed(coseBytes) {
	const coseKey = decodeCBOR(coseBytes);
	if (coseKey.get(1) !== 2 || coseKey.get(-1) !== 1) throw new Error("only ES256 P-256 passkeys are supported");
	const x = coseKey.get(-2);
	const y = coseKey.get(-3);
	const uncompressed = new Uint8Array(65);
	uncompressed[0] = 0x04;
	uncompressed.set(x, 1);
	uncompressed.set(y, 33);
	return uncompressed;
}

function requestRelyingPartyID(event) {
	const host = event.request.host || "";
	return host.split(":")[0];
}

function verifyClientData(clientDataJSONBytes, expectedType, expectedChallenge) {
	const clientData = JSON.parse(utf8Decode(clientDataJSONBytes));
	if (clientData.type !== expectedType) throw new Error("unexpected clientData type " + clientData.type);
	if (clientData.challenge !== expectedChallenge) throw new Error("challenge mismatch");
	return clientData;
}

function challengeStoreKey(purpose, username) {
	return "sitePasskey:" + purpose + ":" + username.toLowerCase();
}

function issueChallenge(purpose, username) {
	const challenge = base64urlEncode(randomBytes(32));
	$app.store().set(challengeStoreKey(purpose, username), { challenge, issuedAt: Date.now() });
	return challenge;
}

function consumeChallenge(purpose, username) {
	const key = challengeStoreKey(purpose, username);
	const entry = $app.store().get(key);
	$app.store().remove(key);
	if (!entry || Date.now() - entry.issuedAt > 5 * 60 * 1000) throw new Error("challenge expired; request new options");
	return entry.challenge;
}

function randomBytes(length) {
	const alphabet = "0123456789abcdef";
	const hex = $security.randomStringWithAlphabet(length * 2, alphabet);
	const bytes = new Uint8Array(length);
	for (let index = 0; index < length; index += 1) {
		bytes[index] = parseInt(hex.substr(index * 2, 2), 16);
	}
	return bytes;
}

function normalizedUsername(raw) {
	const username = String(raw || "").trim().toLowerCase();
	if (!/^[a-z0-9_.-]{3,60}$/.test(username)) throw new Error("아이디는 3-60자의 영문 소문자, 숫자, _ . - 만 쓸 수 있습니다.");
	return username;
}

function findUserByUsername(username) {
	try {
		return $app.findFirstRecordByData("users", "username", username);
	} catch (error) {
		return null;
	}
}

function findPasskeysForUser(userID) {
	return $app.findRecordsByFilter("sitePasskeys", "user = {:user}", "-created", 20, 0, { user: userID });
}

function passkeyRegisterOptions(event) {
	const body = event.requestInfo().body;
	const username = normalizedUsername(body.username);
	if (findUserByUsername(username)) throw new BadRequestError("이미 사용 중인 아이디입니다.");
	const challenge = issueChallenge("register", username);
	return event.json(200, {
		publicKey: {
			challenge,
			rp: { id: requestRelyingPartyID(event), name: requestRelyingPartyID(event) },
			user: { id: base64urlEncode(randomBytes(16)), name: username, displayName: username },
			pubKeyCredParams: [{ type: "public-key", alg: -7 }],
			authenticatorSelection: { residentKey: "preferred", userVerification: "preferred" },
			timeout: 60000,
			attestation: "none",
		},
	});
}

function passkeyRegister(event) {
	const body = event.requestInfo().body;
	const username = normalizedUsername(body.username);
	const expectedChallenge = consumeChallenge("register", username);
	if (findUserByUsername(username)) throw new BadRequestError("이미 사용 중인 아이디입니다.");

	verifyClientData(base64urlDecode(body.credential.response.clientDataJSON), "webauthn.create", expectedChallenge);
	const attestation = decodeCBOR(base64urlDecode(body.credential.response.attestationObject));
	const authenticatorData = parseAuthenticatorData(attestation.get("authData"));
	if (!authenticatorData.credentialID) throw new BadRequestError("등록 응답에 자격 증명이 없습니다.");
	const publicKey = coseKeyToUncompressed(authenticatorData.credentialPublicKeyCBOR);

	const usersCollection = $app.findCollectionByNameOrId("users");
	const user = new Record(usersCollection);
	user.set("username", username);
	user.setPassword(base64urlEncode(randomBytes(24)));
	$app.save(user);

	const passkeysCollection = $app.findCollectionByNameOrId("sitePasskeys");
	const passkey = new Record(passkeysCollection);
	passkey.set("user", user.id);
	passkey.set("credentialID", base64urlEncode(authenticatorData.credentialID));
	passkey.set("publicKey", base64urlEncode(publicKey));
	passkey.set("counter", authenticatorData.counter);
	$app.save(passkey);

	return $apis.recordAuthResponse(event, user, "passkey");
}

function passkeyLoginOptions(event) {
	const body = event.requestInfo().body;
	const username = normalizedUsername(body.username);
	const user = findUserByUsername(username);
	if (!user) throw new BadRequestError("등록되지 않은 아이디입니다.");
	const passkeys = findPasskeysForUser(user.id);
	if (passkeys.length === 0) throw new BadRequestError("이 계정에 등록된 패스키가 없습니다. 비밀번호로 로그인해 주세요.");
	const challenge = issueChallenge("login", username);
	return event.json(200, {
		publicKey: {
			challenge,
			rpId: requestRelyingPartyID(event),
			allowCredentials: passkeys.map((record) => ({ type: "public-key", id: record.get("credentialID") })),
			userVerification: "preferred",
			timeout: 60000,
		},
	});
}

function passkeyLogin(event) {
	const body = event.requestInfo().body;
	const username = normalizedUsername(body.username);
	const expectedChallenge = consumeChallenge("login", username);
	const user = findUserByUsername(username);
	if (!user) throw new BadRequestError("등록되지 않은 아이디입니다.");

	const credentialID = String(body.credential.id || "");
	const passkeys = findPasskeysForUser(user.id);
	const passkey = passkeys.find((record) => record.get("credentialID") === credentialID);
	if (!passkey) throw new BadRequestError("이 계정의 패스키가 아닙니다.");

	const clientDataJSONBytes = base64urlDecode(body.credential.response.clientDataJSON);
	verifyClientData(clientDataJSONBytes, "webauthn.get", expectedChallenge);
	const authenticatorData = base64urlDecode(body.credential.response.authenticatorData);
	const signedPayload = concatBytes(authenticatorData, sha256(clientDataJSONBytes));
	const publicKey = base64urlDecode(passkey.get("publicKey"));
	const signature = base64urlDecode(body.credential.response.signature);
	const isValid = p256.verify(signature, sha256(signedPayload), publicKey, { format: "der", lowS: false });
	if (!isValid) throw new BadRequestError("패스키 서명 검증에 실패했습니다.");

	const counter = parseAuthenticatorData(authenticatorData).counter;
	passkey.set("counter", counter);
	$app.save(passkey);

	return $apis.recordAuthResponse(event, user, "passkey");
}

function guarded(handler, event) {
	try {
		return handler(event);
	} catch (error) {
		if (error instanceof BadRequestError) throw error;
		throw new BadRequestError("passkey: " + String(error && error.message ? error.message : error));
	}
}

module.exports = {
	registerOptions: (event) => guarded(passkeyRegisterOptions, event),
	register: (event) => guarded(passkeyRegister, event),
	loginOptions: (event) => guarded(passkeyLoginOptions, event),
	login: (event) => guarded(passkeyLogin, event),
};
