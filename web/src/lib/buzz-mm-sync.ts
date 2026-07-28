import { publishBuzzMessage } from './buzz-relay-client';

export type BuzzMMPendingItem = {
	externalId: string;
	externalChannelId: string;
	buzzChannelId: string;
	text: string;
	updatedAt: number;
};

const CURSOR_STORAGE_KEY = 'internkim.buzz.mm-cursor';

// Mirrors the pending Mattermost posts in updated-at order, advancing the cursor
// only through consecutive successes. On the first failure it stops, so that
// message and every later one are retried on the next refresh rather than
// skipped past the cursor.
export async function mirrorPending(
	items: BuzzMMPendingItem[],
	cursor: number,
	mirror: (item: BuzzMMPendingItem) => Promise<void>
): Promise<{ cursor: number; mirrored: number }> {
	const ordered = [...items].sort((first, second) => first.updatedAt - second.updatedAt);
	let advanced = cursor;
	let mirrored = 0;
	for (const item of ordered) {
		try {
			await mirror(item);
			advanced = item.updatedAt;
			mirrored += 1;
		} catch {
			break;
		}
	}
	return { cursor: advanced, mirrored };
}

async function relayURL(): Promise<string | null> {
	try {
		const document: { relayURL?: string } = await fetch('/agent/api/buzz-relay-config', {
			credentials: 'include'
		}).then((response) => response.json());
		return document.relayURL?.trim() || null;
	} catch {
		return null;
	}
}

// On messenger load, pull the person's Mattermost posts not yet mirrored to
// Buzz, sign each with their key in the browser, publish to Buzz, and record the
// mapping — so MM-authored messages reach Buzz client-signed, not server-signed.
export async function syncMattermostToBuzz(secretHex: string): Promise<number> {
	const url = await relayURL();
	if (!url) return 0;
	const cursor = Number(localStorage.getItem(CURSOR_STORAGE_KEY) ?? '0');
	const pending: { items: BuzzMMPendingItem[] } = await fetch(`/agent/api/buzz-mm-pending?since=${cursor}`, {
		credentials: 'include'
	}).then((response) => response.json());

	const result = await mirrorPending(pending.items ?? [], cursor, async (item) => {
		const eventId = await publishBuzzMessage(url, secretHex, {
			channelId: item.buzzChannelId,
			content: item.text
		});
		const response = await fetch('/agent/api/buzz-mm-mirrored', {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				externalId: item.externalId,
				externalChannelId: item.externalChannelId,
				buzzEventId: eventId
			})
		});
		if (!response.ok) throw new Error(`record mirrored returned ${response.status}`);
	});

	localStorage.setItem(CURSOR_STORAGE_KEY, String(result.cursor));
	return result.mirrored;
}
