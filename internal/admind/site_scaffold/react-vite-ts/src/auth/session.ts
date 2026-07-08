import { useEffect, useState } from "react";

export type SessionUser = {
	id: string;
	email: string;
	name?: string;
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
		email: String(record.email ?? ""),
		name: typeof record.name === "string" && record.name ? record.name : undefined,
	};
}

export async function signIn(userCollection: string, email: string, password: string): Promise<AuthResult> {
	const response = await fetch(`/api/collections/${userCollection}/auth-with-password`, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ identity: email, password }),
	});
	const body = await response.json().catch(() => ({}));
	if (!response.ok) {
		return { ok: false, message: String(body?.message ?? "로그인에 실패했습니다. 이메일과 비밀번호를 확인해 주세요.") };
	}
	storeSession({ token: String(body.token), user: extractUser(body.record ?? {}) });
	return { ok: true };
}

export async function signUp(userCollection: string, email: string, password: string, name: string): Promise<AuthResult> {
	const createResponse = await fetch(`/api/collections/${userCollection}/records`, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ email, password, passwordConfirm: password, name }),
	});
	if (!createResponse.ok) {
		const body = await createResponse.json().catch(() => ({}));
		const fieldErrors = body?.data ? Object.values(body.data).map((entry) => String((entry as { message?: string })?.message ?? "")).filter(Boolean) : [];
		return { ok: false, message: fieldErrors[0] ?? String(body?.message ?? "가입에 실패했습니다.") };
	}
	return signIn(userCollection, email, password);
}
