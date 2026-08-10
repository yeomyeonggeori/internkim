const passwordAlphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789';
const passwordLength = 20;

export type ClaimableAccount = { identities?: unknown[] | null } | undefined;

// A deployment whose Supabase project cannot send mail cannot send a code, so
// nobody can set a sign-in up at all. Where that is so, the first person to name
// an address is handed a password on screen and every later attempt on that
// address is refused. Off unless AUTH_CLAIM_WITHOUT_EMAIL is 1.
export function mailCanCarryTheCode(environment: Record<string, string | undefined>): boolean {
	return (environment.AUTH_CLAIM_WITHOUT_EMAIL ?? '').trim() !== '1';
}

export function isAlreadyClaimed(account: ClaimableAccount): boolean {
	return (account?.identities ?? []).length > 0;
}

export function anIssuedPassword(): string {
	const drawn = crypto.getRandomValues(new Uint32Array(passwordLength));
	return [...drawn].map((value) => passwordAlphabet[value % passwordAlphabet.length]).join('');
}
