export type ListedNetwork = {
	ssid: string;
	isSecured: boolean;
	signalPercent: number;
};

export type NetworkListing = {
	networks: ListedNetwork[];
	hasJoinFailed: boolean;
};

export type JoinOutcome = 'accepted' | 'networkRequired' | 'alreadySubmitted' | 'unreachable';

export async function fetchNetworkListing(): Promise<NetworkListing> {
	const response = await fetch('/networks', { cache: 'no-store' });
	if (!response.ok) {
		throw new Error(`The box answered /networks with ${response.status}.`);
	}
	const listing: NetworkListing = await response.json();
	return listing;
}

export async function submitJoin(ssid: string, customSSID: string, password: string): Promise<JoinOutcome> {
	const body = new URLSearchParams({ ssid, customSSID, password });
	const response = await fetch('/join', { method: 'POST', body }).catch(() => undefined);
	if (!response) {
		return 'unreachable';
	}
	if (response.ok) {
		return 'accepted';
	}
	if (response.status === 400) {
		return 'networkRequired';
	}
	if (response.status === 409) {
		return 'alreadySubmitted';
	}
	return 'unreachable';
}
