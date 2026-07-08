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
