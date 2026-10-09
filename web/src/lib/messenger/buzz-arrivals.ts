import { Relay } from 'nostr-tools/relay';
import type { Subscription } from 'nostr-tools/abstract-relay';
import type { Event } from 'nostr-tools/pure';
import { signBuzzEvent } from '$lib/buzz-relay-client';
import { centralBuzzRelayURL } from '$lib/buzz-relay-central-address';
import { announceCompanyEvent } from '$lib/host-bridge';

const streamMessageKind = 9;
const authRequiredPrefix = 'auth-required';
const resubscribeMilliseconds = 1_000;
const firstReconnectMilliseconds = 1_000;
const longestReconnectMilliseconds = 30_000;

type Following = {
	secretHex: string;
	relay: Promise<Relay | null>;
	channelIDs: string[];
	subscription: Subscription | null;
	isStopped: boolean;
	reconnectMilliseconds: number;
	reconnectTimer: ReturnType<typeof setTimeout> | undefined;
};

let following: Following | null = null;

export function followBuzzArrivals(secretHex: string | null, channelIDs: string[]): void {
	if (!secretHex || channelIDs.length === 0) {
		stopBuzzArrivals();
		return;
	}
	if (following?.secretHex !== secretHex) {
		stopBuzzArrivals();
		following = {
			secretHex,
			relay: openRelay(secretHex),
			channelIDs: [],
			subscription: null,
			isStopped: false,
			reconnectMilliseconds: firstReconnectMilliseconds,
			reconnectTimer: undefined
		};
	}
	if (isSameChannelSet(following.channelIDs, channelIDs)) return;
	following.channelIDs = channelIDs;
	void subscribe(following);
}

export function stopBuzzArrivals(): void {
	if (!following) return;
	const stopped = following;
	following = null;
	stopped.isStopped = true;
	clearTimeout(stopped.reconnectTimer);
	stopped.subscription?.close();
	void stopped.relay.then((relay) => relay?.close()).catch(() => undefined);
}

async function openRelay(secretHex: string): Promise<Relay | null> {
	const relayURL = await centralBuzzRelayURL();
	if (!relayURL) return null;
	const relay = new Relay(relayURL, { enablePing: true, enableReconnect: true });
	relay.onauth = (template) => Promise.resolve(signBuzzEvent(secretHex, template));
	await relay.connect();
	return relay;
}

async function subscribe(target: Following): Promise<void> {
	let relay: Relay | null;
	try {
		relay = await target.relay;
	} catch (failure) {
		console.warn('the messenger could not listen to the Buzz relay', failure);
		reconnectLater(target);
		return;
	}
	if (!relay || target.isStopped) return;
	target.reconnectMilliseconds = firstReconnectMilliseconds;
	const channelIDs = target.channelIDs;
	target.subscription?.close();
	target.subscription = relay.subscribe([{ kinds: [streamMessageKind], '#h': channelIDs, since: nowSeconds() }], {
		onevent: announceArrival,
		onclose: (reason) => {
			if (!reason.startsWith(authRequiredPrefix) || target.isStopped || target.channelIDs !== channelIDs) return;
			setTimeout(() => void subscribe(target), resubscribeMilliseconds);
		}
	});
}

function reconnectLater(target: Following): void {
	if (target.isStopped) return;
	clearTimeout(target.reconnectTimer);
	target.reconnectTimer = setTimeout(() => {
		target.relay = openRelay(target.secretHex);
		void subscribe(target);
	}, target.reconnectMilliseconds);
	target.reconnectMilliseconds = Math.min(target.reconnectMilliseconds * 2, longestReconnectMilliseconds);
}

function announceArrival(event: Event): void {
	const conversationID = event.tags.find((tag) => tag[0] === 'h')?.[1];
	if (!conversationID) return;
	announceCompanyEvent({ kind: 'message.arrived', conversationID, messageID: event.id, authorExternalID: event.pubkey });
}

function isSameChannelSet(current: string[], next: string[]): boolean {
	if (current.length !== next.length) return false;
	const known = new Set(current);
	return next.every((channelID) => known.has(channelID));
}

function nowSeconds(): number {
	return Math.floor(Date.now() / 1000);
}
