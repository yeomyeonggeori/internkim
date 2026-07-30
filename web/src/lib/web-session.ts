export async function fetchWebSessionEmail(): Promise<string> {
	const response = await fetch('/auth/session', { credentials: 'include' });
	if (!response.ok) return '';
	const session = (await response.json()) as { email?: string };
	return (session.email ?? '').trim().toLowerCase();
}
