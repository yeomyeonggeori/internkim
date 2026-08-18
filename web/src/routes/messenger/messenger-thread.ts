import type { MessengerChannel, MessengerPost } from '$lib/messenger/messenger-api';

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

