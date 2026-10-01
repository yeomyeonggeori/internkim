import { REALTIME_SUBSCRIBE_STATES, type RealtimeChannel, type RealtimePresenceState } from '@supabase/supabase-js';
import { isSupabaseConfigured, supabase } from '$lib/supabase';

type BroadcastListener = { event: string; heard: () => void };
type PresenceListener = (state: RealtimePresenceState) => void;

const broadcastListeners = new Set<BroadcastListener>();
const presenceListeners = new Set<PresenceListener>();
let presencePayload: Record<string, string> | null = null;
let channel: RealtimeChannel | null = null;
let isOpening = false;

export function onCompanyBroadcast(event: string, heard: () => void): () => void {
	const listener = { event, heard };
	broadcastListeners.add(listener);
	open();
	return () => {
		broadcastListeners.delete(listener);
		closeWhenUnused();
	};
}

export function shareCompanyPresence(payload: Record<string, string>, heard: PresenceListener): () => void {
	presencePayload = payload;
	presenceListeners.add(heard);
	open();
	if (channel?.state === 'joined') void channel.track(payload);
	return () => {
		presenceListeners.delete(heard);
		presencePayload = null;
		if (channel?.state === 'joined') void channel.untrack();
		closeWhenUnused();
	};
}

function isUsed(): boolean {
	return broadcastListeners.size > 0 || presenceListeners.size > 0;
}

function open(): void {
	if (channel || isOpening || !isSupabaseConfigured()) return;
	isOpening = true;
	void supabase()
		.rpc('my_company_topic')
		.then(
			({ data }) => {
				isOpening = false;
				if (!isUsed() || typeof data !== 'string' || !data) return;
				channel = subscribed(data);
			},
			(failure: unknown) => {
				isOpening = false;
				console.warn('the company channel was not opened', failure);
			}
		);
}

function subscribed(topic: string): RealtimeChannel {
	const opened = supabase().channel(topic, { config: { private: true, presence: { enabled: true } } });
	return opened
		.on('broadcast', { event: '*' }, ({ event }) => {
			for (const listener of broadcastListeners) if (listener.event === event) listener.heard();
		})
		.on('presence', { event: 'sync' }, () => {
			const state = opened.presenceState();
			for (const heard of presenceListeners) heard(state);
		})
		.subscribe((status) => {
			if (status === REALTIME_SUBSCRIBE_STATES.SUBSCRIBED && presencePayload) void opened.track(presencePayload);
		});
}

function closeWhenUnused(): void {
	if (isUsed() || !channel) return;
	void supabase().removeChannel(channel);
	channel = null;
}
