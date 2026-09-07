export type HeldCredential = { kind: string; secret: string } | null;

type Kept = { credential: HeldCredential; until: number };

const heldForMilliseconds = 10 * 60_000;
const absenceHeldForMilliseconds = 30_000;

export class CredentialCache {
	private readonly kept = new Map<string, Kept>();
	private readonly reading = new Map<string, Promise<HeldCredential>>();

	constructor(
		private readonly read: (memberID: string) => Promise<HeldCredential>,
		private readonly now: () => number = Date.now
	) {}

	credentialOf(memberID: string): Promise<HeldCredential> {
		const held = this.kept.get(memberID);
		if (held && held.until > this.now()) return Promise.resolve(held.credential);
		const inFlight = this.reading.get(memberID);
		if (inFlight) return inFlight;

		const reading = this.read(memberID)
			.then((credential) => {
				this.kept.set(memberID, { credential, until: this.now() + this.heldFor(credential) });
				return credential;
			})
			.finally(() => this.reading.delete(memberID));
		this.reading.set(memberID, reading);
		return reading;
	}

	forget(memberID: string): void {
		this.kept.delete(memberID);
	}

	forgetEveryone(): void {
		this.kept.clear();
	}

	private heldFor(credential: HeldCredential): number {
		return credential ? heldForMilliseconds : absenceHeldForMilliseconds;
	}
}
