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

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null;
}

function parseListedNetwork(value: unknown): ListedNetwork {
	if (!isRecord(value) || typeof value.ssid !== 'string' || typeof value.isSecured !== 'boolean' || typeof value.signalPercent !== 'number') {
		throw new Error(`The box listed a network the page cannot read: ${JSON.stringify(value)}.`);
	}
	return { ssid: value.ssid, isSecured: value.isSecured, signalPercent: value.signalPercent };
}

export function parseNetworkListing(value: unknown): NetworkListing {
	if (!isRecord(value) || !Array.isArray(value.networks) || typeof value.hasJoinFailed !== 'boolean') {
		throw new Error(`The box answered /networks with a listing the page cannot read: ${JSON.stringify(value)}.`);
	}
	return { networks: value.networks.map(parseListedNetwork), hasJoinFailed: value.hasJoinFailed };
}

export function joinForm(ssid: string, customSSID: string, password: string): URLSearchParams {
	return new URLSearchParams({ ssid, customSSID, password });
}

export function joinOutcomeFor(status: number): JoinOutcome {
	if (status >= 200 && status < 300) {
		return 'accepted';
	}
	if (status === 400) {
		return 'networkRequired';
	}
	if (status === 409) {
		return 'alreadySubmitted';
	}
	return 'unreachable';
}

export async function fetchNetworkListing(): Promise<NetworkListing> {
	const response = await fetch('/networks', { cache: 'no-store' });
	if (!response.ok) {
		throw new Error(`The box answered /networks with ${response.status}.`);
	}
	return parseNetworkListing(await response.json());
}

export async function submitJoin(ssid: string, customSSID: string, password: string): Promise<JoinOutcome> {
	try {
		const response = await fetch('/join', { method: 'POST', body: joinForm(ssid, customSSID, password) });
		return joinOutcomeFor(response.status);
	} catch (errorValue) {
		console.error('captive page could not reach the box to join', { errorValue });
		return 'unreachable';
	}
}
