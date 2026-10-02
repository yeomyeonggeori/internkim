export type CompanyOfSlug = (slug: string) => Promise<string | null>;

const rememberedMilliseconds = 60_000;

type Remembered = { companyID: string | null; until: number };

export class MessengerAddresses {
	private readonly remembered = new Map<string, Remembered>();

	constructor(
		private readonly ask: CompanyOfSlug,
		private readonly now: () => number = Date.now
	) {}

	async companyOf(slug: string): Promise<string | null> {
		const kept = this.remembered.get(slug);
		if (kept && kept.until > this.now()) return kept.companyID;
		const companyID = await this.ask(slug);
		this.remembered.set(slug, { companyID, until: this.now() + rememberedMilliseconds });
		return companyID;
	}
}
