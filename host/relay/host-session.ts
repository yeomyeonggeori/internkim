export type HostSession = { companyID: string; accessToken: string; expiresAt: number };

type HostSessionSource = {
	credential: () => Promise<string>;
	exchange: (credential: string) => Promise<HostSession>;
	nowSeconds?: () => number;
};

const renewalMarginSeconds = 300;

export type HostSessionKeeper = {
	current: () => HostSession;
	keepFresh: () => Promise<boolean>;
};

export async function keepHostSession(source: HostSessionSource): Promise<HostSessionKeeper> {
	const nowSeconds = source.nowSeconds ?? (() => Math.floor(Date.now() / 1000));
	let mintedFrom = await source.credential();
	let session = await source.exchange(mintedFrom);

	async function keepFresh(): Promise<boolean> {
		const credential = await source.credential();
		const isExpiring = session.expiresAt - nowSeconds() <= renewalMarginSeconds;
		if (credential === mintedFrom && !isExpiring) return false;
		session = await source.exchange(credential);
		mintedFrom = credential;
		return true;
	}

	return { current: () => session, keepFresh };
}

export type RoleAnswer = { data: unknown; error: { message: string } | null; status: number };

export class NoLongerServes extends Error {}

export async function confirmStillServing(askRole: () => PromiseLike<RoleAnswer>): Promise<void> {
	const answer = await askRole();
	if (answer.status === 401 || answer.status === 403) {
		throw new NoLongerServes(`the central plane refused this computer's session (${answer.status})`);
	}
	if (answer.error) throw new Error(`asking whether this computer still serves its company: ${answer.error.message}`);
	if (answer.data !== 'service') {
		throw new NoLongerServes(`the central plane answers that this computer is ${answer.data ?? 'no longer its company\'s'}`);
	}
}
