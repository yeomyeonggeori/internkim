import { onCompanyEvent } from '$lib/host-bridge';
import { announceTyping } from '$lib/messenger/messenger-api';
import { isSupabaseConfigured } from '$lib/supabase';
import {
	isTimeToAnnounceTyping,
	typersAfterSignal,
	typersIn,
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
	let announcedPlace: string | undefined;
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

	function heard(conversationID: string | undefined, externalID: string | undefined, threadRootID?: string): void {
		if (!externalID || !conversationID || conversationID !== channelID()) return;
		now = Date.now();
		typers = typersAfterSignal(typersChannelID === conversationID ? typers : [], externalID, now, threadRootID);
		typersChannelID = conversationID;
		pruneTimer ??= setInterval(prune, pruneIntervalMilliseconds);
	}

	$effect(() => {
		if (!isSupabaseConfigured()) return;
		const stopListening = onCompanyEvent((event) => {
			if (event.kind === typingEventKind) heard(event.conversationID, event.authorExternalID, event.threadRootID);
		});
		return () => {
			stopListening();
			clearInterval(pruneTimer);
			pruneTimer = undefined;
		};
	});

	return {
		typerExternalIDsIn(threadRootID?: string): string[] {
			return typersIn(stillTyping, threadRootID).map((typer) => typer.externalID);
		},
		announce(threadRootID?: string): void {
			const conversationID = channelID();
			if (!conversationID || !isSupabaseConfigured()) return;
			const at = Date.now();
			const place = `${conversationID}/${threadRootID ?? ''}`;
			if (announcedPlace === place && !isTimeToAnnounceTyping(lastAnnouncedAt, at)) return;
			lastAnnouncedAt = at;
			announcedPlace = place;
			announceTyping(conversationID, threadRootID).catch((error: unknown) => {
				console.warn('typing was not announced', { conversationID, threadRootID, error });
			});
		}
	};
}
