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
