import { useEffect, useState } from "react";

export type SessionUser = {
	id: string;
	username: string;
};

type SessionState = {
	token: string;
	user: SessionUser;
};

const STORAGE_KEY = "site-session";
const listeners = new Set<() => void>();

function readStoredSession(): SessionState | null {
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return null;
		const parsed = JSON.parse(raw);
		if (typeof parsed?.token === "string" && typeof parsed?.user?.id === "string") return parsed;
	} catch {
		return null;
	}
	return null;
}

function notify() {
	for (const listener of listeners) listener();
}

export function currentSession(): SessionState | null {
	return readStoredSession();
}

export function storeSession(state: SessionState) {
	localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
	notify();
}

export function clearSession() {
	localStorage.removeItem(STORAGE_KEY);
	notify();
}

export function useSession(): SessionState | null {
	const [session, setSession] = useState<SessionState | null>(currentSession());
	useEffect(() => {
		const listener = () => setSession(currentSession());
		listeners.add(listener);
		return () => {
			listeners.delete(listener);
		};
	}, []);
	return session;
}

type AuthResult = { ok: true } | { ok: false; message: string };

function extractUser(record: Record<string, unknown>): SessionUser {
	return {
		id: String(record.id ?? ""),
		username: String(record.username ?? ""),
	};
}

export async function signIn(userCollection: string, username: string, password: string): Promise<AuthResult> {
	const response = await fetch(`/api/collections/${userCollection}/auth-with-password`, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ identity: username, password }),
	});
	const body = await response.json().catch(() => ({}));
	if (!response.ok) {
		return { ok: false, message: String(body?.message ?? "로그인에 실패했습니다. 아이디와 비밀번호를 확인해 주세요.") };
	}
	storeSession({ token: String(body.token), user: extractUser(body.record ?? {}) });
	return { ok: true };
}

export async function refreshSession(userCollection: string): Promise<void> {
	const session = currentSession();
	if (!session) return;
	const response = await fetch(`/api/collections/${userCollection}/auth-refresh`, {
		method: "POST",
		headers: { Authorization: session.token },
	});
	if (response.status === 401 || response.status === 403 || response.status === 404) {
		clearSession();
		return;
	}
	if (response.ok) {
		const body = await response.json().catch(() => null);
		if (body?.token) storeSession({ token: String(body.token), user: extractUser(body.record ?? {}) });
	}
}

export async function signUp(userCollection: string, username: string, password: string): Promise<AuthResult> {
	const createResponse = await fetch(`/api/collections/${userCollection}/records`, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ username, password, passwordConfirm: password }),
	});
	if (!createResponse.ok) {
		const body = await createResponse.json().catch(() => ({}));
		const fieldErrors = body?.data ? Object.values(body.data).map((entry) => String((entry as { message?: string })?.message ?? "")).filter(Boolean) : [];
		return { ok: false, message: fieldErrors[0] ?? String(body?.message ?? "가입에 실패했습니다.") };
	}
	return signIn(userCollection, username, password);
}

function base64urlToBuffer(text: string): ArrayBuffer {
	const padded = text.replace(/-/g, "+").replace(/_/g, "/");
	const raw = atob(padded + "=".repeat((4 - (padded.length % 4)) % 4));
	return Uint8Array.from(raw, (character) => character.charCodeAt(0)).buffer;
}

function bufferToBase64url(buffer: ArrayBuffer): string {
	return btoa(String.fromCharCode(...new Uint8Array(buffer)))
		.replace(/\+/g, "-")
		.replace(/\//g, "_")
		.replace(/=+$/, "");
}

export function supportsPasskey(): boolean {
	return typeof window !== "undefined" && !!window.PublicKeyCredential;
}

async function postJSON(path: string, body: unknown): Promise<{ ok: boolean; payload: Record<string, unknown> }> {
	const response = await fetch(path, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(body),
	});
	const payload = await response.json().catch(() => ({}));
	return { ok: response.ok, payload };
}

function storeAuthPayload(payload: Record<string, unknown>) {
	storeSession({ token: String(payload.token), user: extractUser((payload.record as Record<string, unknown>) ?? {}) });
}

export async function passkeySignUp(userCollection: string, username: string): Promise<AuthResult> {
	void userCollection;
	const optionsResponse = await postJSON("/api/site-auth/passkey/register-options", { username });
	if (!optionsResponse.ok) return { ok: false, message: String(optionsResponse.payload.message ?? "패스키 등록 준비에 실패했습니다.") };
	const publicKey = optionsResponse.payload.publicKey as Record<string, unknown>;
	publicKey.challenge = base64urlToBuffer(String(publicKey.challenge));
	(publicKey.user as Record<string, unknown>).id = base64urlToBuffer(String((publicKey.user as Record<string, unknown>).id));
	const credential = (await navigator.credentials.create({ publicKey: publicKey as unknown as PublicKeyCredentialCreationOptions })) as PublicKeyCredential;
	const attestation = credential.response as AuthenticatorAttestationResponse;
	const verifyResponse = await postJSON("/api/site-auth/passkey/register", {
		username,
		credential: {
			id: credential.id,
			response: {
				clientDataJSON: bufferToBase64url(attestation.clientDataJSON),
				attestationObject: bufferToBase64url(attestation.attestationObject),
			},
		},
	});
	if (!verifyResponse.ok) return { ok: false, message: String(verifyResponse.payload.message ?? "패스키 등록에 실패했습니다.") };
	storeAuthPayload(verifyResponse.payload);
	return { ok: true };
}

export async function passkeyLogin(userCollection: string, username: string): Promise<AuthResult> {
	void userCollection;
	const optionsResponse = await postJSON("/api/site-auth/passkey/login-options", { username });
	if (!optionsResponse.ok) return { ok: false, message: String(optionsResponse.payload.message ?? "패스키 로그인 준비에 실패했습니다.") };
	const publicKey = optionsResponse.payload.publicKey as Record<string, unknown>;
	publicKey.challenge = base64urlToBuffer(String(publicKey.challenge));
	publicKey.allowCredentials = ((publicKey.allowCredentials as Array<Record<string, unknown>>) ?? []).map((entry) => ({
		...entry,
		id: base64urlToBuffer(String(entry.id)),
	}));
	const credential = (await navigator.credentials.get({ publicKey: publicKey as unknown as PublicKeyCredentialRequestOptions })) as PublicKeyCredential;
	const assertion = credential.response as AuthenticatorAssertionResponse;
	const verifyResponse = await postJSON("/api/site-auth/passkey/login", {
		username,
		credential: {
			id: credential.id,
			response: {
				clientDataJSON: bufferToBase64url(assertion.clientDataJSON),
				authenticatorData: bufferToBase64url(assertion.authenticatorData),
				signature: bufferToBase64url(assertion.signature),
			},
		},
	});
	if (!verifyResponse.ok) return { ok: false, message: String(verifyResponse.payload.message ?? "패스키 로그인에 실패했습니다.") };
	storeAuthPayload(verifyResponse.payload);
	return { ok: true };
}
