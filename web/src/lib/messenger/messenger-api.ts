import { callCompanyApp } from '$lib/host-bridge';

export type MessengerPerson = {
	memberID?: string;
	name: string;
};

export type MessengerChannel = {
	id: string;
	platform: string;
	name: string;
	isDirect: boolean;
	position: number;
	participants: MessengerPerson[];
};

export type MessengerReaction = {
	emoji: string;
	people: MessengerPerson[];
};

export type MessengerPost = {
	id: string;
	channelID: string;
	parentID?: string;
	author: MessengerPerson;
	body: string;
	postedAt: string;
	editedAt?: string;
	reactions: MessengerReaction[];
};

async function ask<Value>(method: 'GET' | 'POST' | 'PUT' | 'DELETE', path: string, body?: unknown): Promise<Value> {
	const answer = await callCompanyApp({ method, path, body });
	if (answer.status >= 400) throw new Error(messageOf(answer.body, `the app answered ${answer.status}`));
	return answer.body as Value;
}

function messageOf(body: unknown, fallback: string): string {
	if (typeof body === 'object' && body !== null && typeof (body as { error?: unknown }).error === 'string') {
		return (body as { error: string }).error;
	}
	return fallback;
}

export function fetchChannels(platform?: string): Promise<MessengerChannel[]> {
	const query = platform ? `?platform=${encodeURIComponent(platform)}` : '';
	return ask<MessengerChannel[]>('GET', `/channel${query}`);
}

export function openDirectChannel(people: string[], platform: string): Promise<MessengerChannel> {
	return ask<MessengerChannel>('POST', '/channel/direct', { platform, memberIDs: people });
}

export function fetchPosts(channelID: string, before?: string): Promise<MessengerPost[]> {
	const query = before ? `?before=${encodeURIComponent(before)}` : '';
	return ask<MessengerPost[]>('GET', `/channel/${encodeURIComponent(channelID)}/post${query}`);
}

export function writePost(channelID: string, body: string, parentID?: string): Promise<MessengerPost> {
	return ask<MessengerPost>('POST', `/channel/${encodeURIComponent(channelID)}/post`, { body, parentID });
}

export function editPost(postID: string, body: string): Promise<MessengerPost> {
	return ask<MessengerPost>('PUT', `/post/${encodeURIComponent(postID)}`, { body });
}

export function erasePost(postID: string): Promise<void> {
	return ask<void>('DELETE', `/post/${encodeURIComponent(postID)}`);
}

export function addReaction(postID: string, emoji: string): Promise<void> {
	return ask<void>('POST', `/post/${encodeURIComponent(postID)}/reaction`, { emoji });
}

export function removeReaction(postID: string, emoji: string): Promise<void> {
	return ask<void>('DELETE', `/post/${encodeURIComponent(postID)}/reaction?emoji=${encodeURIComponent(emoji)}`);
}
