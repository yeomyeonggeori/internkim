import { signBuzzEvent } from "./buzz-relay-client";
import { unwrapSecretWithPasskey, unwrapSecretWithPassword, type WrappedSecret } from "./buzz-key-vault";
import { deriveBuzzPasskeyOutput } from "./buzz-passkey";

const KEY_LOGIN_KIND = 27235;

type BuzzVaultDocument = { copies: WrappedSecret[] };

export type BuzzLoginFactor = { kind: "password"; password: string } | { kind: "passkey" };

async function fetchLoginVault(email: string): Promise<BuzzVaultDocument> {
	const response = await fetch(`/auth/vault?email=${encodeURIComponent(email)}`, { credentials: "include" });
	if (response.status === 404) {
		throw new Error("등록된 계정이 없습니다. 먼저 가입해 주세요.");
	}
	if (!response.ok) {
		throw new Error("계정 정보를 불러오지 못했습니다.");
	}
	return response.json();
}

function copyOfKind(document: BuzzVaultDocument, kind: WrappedSecret["kind"]): WrappedSecret | undefined {
	return document.copies.find((copy) => copy.kind === kind);
}

async function unlockFromVault(document: BuzzVaultDocument, factor: BuzzLoginFactor): Promise<string> {
	if (factor.kind === "password") {
		const copy = copyOfKind(document, "password");
		if (!copy) {
			throw new Error("비밀번호 로그인이 설정되지 않은 계정입니다.");
		}
		return unwrapSecretWithPassword(copy, factor.password);
	}
	const copy = copyOfKind(document, "passkey");
	if (!copy) {
		throw new Error("이 계정에는 패스키가 없습니다. 비밀번호로 로그인하세요.");
	}
	return unwrapSecretWithPasskey(copy, await deriveBuzzPasskeyOutput());
}

// Returning login: unlock the signing key from the server-held sealed vault,
// prove possession by signing the server challenge, and receive a session. No
// Cloudflare round-trip after the first enrollment.
export async function buzzKeyLogin(email: string, factor: BuzzLoginFactor): Promise<string> {
	const document = await fetchLoginVault(email);
	const secretHex = await unlockFromVault(document, factor);

	const challengeResponse = await fetch("/auth/challenge", { credentials: "include" });
	if (!challengeResponse.ok) {
		throw new Error("로그인 챌린지를 받지 못했습니다.");
	}
	const { challenge } = (await challengeResponse.json()) as { challenge: string };

	const event = signBuzzEvent(secretHex, {
		kind: KEY_LOGIN_KIND,
		content: "",
		tags: [["challenge", challenge]]
	});

	const loginResponse = await fetch("/auth/key-login", {
		method: "POST",
		credentials: "include",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ email, event })
	});
	if (!loginResponse.ok) {
		throw new Error((await loginResponse.text()).trim() || "로그인에 실패했습니다.");
	}
	return secretHex;
}
