const passwordStepLifetimeMilliseconds = 10 * 60 * 1000;
const mailboxProvingMethods = new Set(['otp', 'magiclink', 'email/signup', 'invite', 'recovery']);

type AuthenticationMethodReference = string | { method: string; timestamp: number };

export function hasThePasswordStepExpired(openedAt: number, now: number): boolean {
	return now - openedAt > passwordStepLifetimeMilliseconds;
}

export function whenTheMailboxWasProved(references: readonly AuthenticationMethodReference[] | undefined): number {
	const provedAt = (references ?? []).flatMap((reference) =>
		typeof reference !== 'string' && mailboxProvingMethods.has(reference.method) ? [reference.timestamp * 1000] : []
	);
	return Math.max(0, ...provedAt);
}
