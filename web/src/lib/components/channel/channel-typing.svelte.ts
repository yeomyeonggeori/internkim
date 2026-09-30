import { onCompanyEvent } from '$lib/host-bridge';
import { announceTyping } from '$lib/messenger/messenger-api';
import { isSupabaseConfigured } from '$lib/supabase';
import {
	isTimeToAnnounceTyping,
	typersAfterSignal,
	typersStillTyping,
	typingEventKind,
	type SentMessage,
	type Typer
} from '$lib/messenger/typing-signal';

const pruneIntervalMilliseconds = 1_000;

export type ChannelTyping = ReturnType<typeof createChannelTyping>;

export function createChannelTyping(channelID: () => string | undefined, messages: () => SentMessage[]) {
	let typers = $state<Typer[]>([]);
	let typersChannelID = $state<string | undefined>(undefined);
	let now = $state(Date.now());
	let lastAnnouncedAt: number | null = null;
	let announcedChannelID: string | undefined;
	let pruneTimer: ReturnType<typeof setInterval> | undefined;

	const stillTyping = $derived(
		typersChannelID === channelID() ? typersStillTyping(typers, messages(), now) : []
	);

	function prune(): void {
		now = Date.now();
		typers = typersStillTyping(typers, messages(), now);
		if (typers.length > 0) return;
		clearInterval(pruneTimer);
		pruneTimer = undefined;
	}

	function heard(conversationID: string | undefined, externalID: string | undefined): void {
		if (!externalID || !conversationID || conversationID !== channelID()) return;
		now = Date.now();
		typers = typersAfterSignal(typersChannelID === conversationID ? typers : [], externalID, now);
		typersChannelID = conversationID;
		pruneTimer ??= setInterval(prune, pruneIntervalMilliseconds);
	}

	$effect(() => {
		if (!isSupabaseConfigured()) return;
		const stopListening = onCompanyEvent((event) => {
			if (event.kind === typingEventKind) heard(event.conversationID, event.authorExternalID);
		});
		return () => {
			stopListening();
			clearInterval(pruneTimer);
			pruneTimer = undefined;
		};
	});

	return {
		get typerExternalIDs(): string[] {
			return stillTyping.map((typer) => typer.externalID);
		},
		announce(): void {
			const conversationID = channelID();
			if (!conversationID || !isSupabaseConfigured()) return;
			const at = Date.now();
			if (announcedChannelID === conversationID && !isTimeToAnnounceTyping(lastAnnouncedAt, at)) return;
			lastAnnouncedAt = at;
			announcedChannelID = conversationID;
			announceTyping(conversationID).catch((error: unknown) => {
				console.warn('typing was not announced', { conversationID, error });
			});
		}
	};
}
