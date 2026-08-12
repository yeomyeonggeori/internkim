type PostWithEmoji = { body: string; reactions: { emoji: string }[] };

export function shortcodePattern(): RegExp {
	return /:([^:\s]+):/g;
}

export function shortcodeNamesIn(text: string): string[] {
	return [...text.matchAll(shortcodePattern())].map(([, name]) => name);
}

export function customEmojiNamesIn(posts: PostWithEmoji[]): string[] {
	return posts.flatMap((post) => [
		...shortcodeNamesIn(post.body),
		...post.reactions.map((reaction) => reaction.emoji)
	]);
}
