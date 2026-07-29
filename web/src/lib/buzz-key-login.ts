import { signBuzzEvent } from "./buzz-relay-client";
import { unwrapSecretWithPasskey, unwrapSecretWithPassword, type WrappedSecret } from "./buzz-key-vault";
import { loginWithBuzzPasskey } from "./buzz-passkey";

const KEY_LOGIN_KIND = 27235;

type BuzzVaultDocument = { copies: WrappedSecret[] };

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

// Prove possession of the unlocked key by signing the server challenge, and
// receive a session in return.
async function establishSessionWithKey(email: string, secretHex: string): Promise<string> {
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

// Returning login with email + password: unlock the key from the server-held
// sealed vault, then establish a session. No Cloudflare after first enrollment.
export async function buzzPasswordLogin(email: string, password: string): Promise<string> {
	const document = await fetchLoginVault(email);
	const copy = copyOfKind(document, "password");
	if (!copy) {
		throw new Error("비밀번호 로그인이 설정되지 않은 계정입니다.");
	}
	const secretHex = await unwrapSecretWithPassword(copy, password);
	return establishSessionWithKey(email, secretHex);
}

// Returning login with a passkey: the discoverable passkey identifies the
// person and unlocks the key in one gesture — no typed email.
export async function mattermostPasswordLogin(email: string, password: string): Promise<string> {
	const response = await fetch("/auth/password-login", {
		method: "POST",
		credentials: "include",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ email, password })
	});
	if (!response.ok) {
		throw new Error((await response.text()).trim() || "이메일 또는 비밀번호가 올바르지 않습니다.");
	}
	const result = (await response.json()) as { secretHex?: string };
	return result.secretHex ?? "";
}

export async function buzzPasskeyLogin(): Promise<string> {
	const { email, output } = await loginWithBuzzPasskey();
	const document = await fetchLoginVault(email);
	const copy = copyOfKind(document, "passkey");
	if (!copy) {
		throw new Error("이 계정에는 패스키가 없습니다. 이메일로 로그인하세요.");
	}
	const secretHex = await unwrapSecretWithPasskey(copy, output);
	return establishSessionWithKey(email, secretHex);
}
