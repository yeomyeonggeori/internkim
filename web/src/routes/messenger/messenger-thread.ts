import type { MessengerChannel, MessengerPost } from '$lib/messenger/messenger-api';
import { personLabel, type MessengerDirectory } from '$lib/messenger/messenger-directory';

export type ThreadedPost = {
	post: MessengerPost;
	replies: MessengerPost[];
};

export function threadsOf(posts: MessengerPost[]): ThreadedPost[] {
	const threads = new Map<string, ThreadedPost>();
	const loose: MessengerPost[] = [];

	for (const post of posts) {
		if (post.parentID) continue;
		threads.set(post.id, { post, replies: [] });
	}
	for (const post of posts) {
		if (!post.parentID) continue;
		const thread = threads.get(post.parentID);
		if (thread) thread.replies.push(post);
		else loose.push(post);
	}

	const ordered = posts.filter((post) => !post.parentID).map((post) => threads.get(post.id)!);
	return [...ordered, ...loose.map((post) => ({ post, replies: [] }))];
}

export function channelLabel(channel: MessengerChannel, directory: MessengerDirectory | null): string {
	if (!channel.isDirect) return channel.name;
	if (!directory) return channel.name;
	const names = channel.participants.map((person) => personLabel(person, directory)).filter(Boolean);
	return names.length > 0 ? names.join(', ') : channel.name;
}
