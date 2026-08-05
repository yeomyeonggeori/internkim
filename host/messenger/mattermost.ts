export type MattermostSettings = {
	baseURL: string;
	email: string;
	password: string;
};

export type MattermostSession = {
	token: string;
	userID: string;
};

export async function signIn(settings: MattermostSettings): Promise<MattermostSession> {
	const response = await fetch(`${settings.baseURL}/api/v4/users/login`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ login_id: settings.email, password: settings.password })
	});
	if (!response.ok) throw new Error(`mattermost login returned ${response.status}`);
	const token = response.headers.get('token');
	if (!token) throw new Error('mattermost returned no session token');
	const account = (await response.json()) as { id: string };
	return { token, userID: account.id };
}

async function ask<Value>(
	settings: MattermostSettings,
	session: MattermostSession,
	method: string,
	path: string,
	body?: unknown
): Promise<Value> {
	const response = await fetch(`${settings.baseURL}/api/v4${path}`, {
		method,
		headers: {
			Authorization: `Bearer ${session.token}`,
			...(body === undefined ? {} : { 'Content-Type': 'application/json' })
		},
		body: body === undefined ? undefined : JSON.stringify(body)
	});
	if (!response.ok) throw new Error(`mattermost ${method} ${path} returned ${response.status}`);
	if (response.status === 204) return undefined as Value;
	return (await response.json()) as Value;
}

type MattermostChannel = { id: string; display_name: string; name: string; type: string };
type MattermostPost = {
	id: string;
	channel_id: string;
	root_id: string;
	user_id: string;
	message: string;
	create_at: number;
	edit_at: number;
};
type MattermostUser = { id: string; username: string; first_name: string; last_name: string; email: string };
type MattermostReaction = { user_id: string; post_id: string; emoji_name: string };

export async function readChannels(settings: MattermostSettings, session: MattermostSession) {
	const teams = await ask<{ id: string }[]>(settings, session, 'GET', `/users/me/teams`);
	const channels: MattermostChannel[] = [];
	for (const team of teams) {
		channels.push(
			...(await ask<MattermostChannel[]>(settings, session, 'GET', `/users/me/teams/${team.id}/channels`))
		);
	}

	const named = channels.map((channel) => ({
		channel,
		isDirect: channel.type === 'D' || channel.type === 'G',
		name: channel.display_name || channel.name
	}));
	named.sort((left, right) => {
		if (left.isDirect !== right.isDirect) return left.isDirect ? 1 : -1;
		return left.name < right.name ? -1 : left.name > right.name ? 1 : 0;
	});

	const read = [];
	for (const [position, entry] of named.entries()) {
		read.push({
			id: entry.channel.id,
			platform: 'mattermost',
			name: entry.name,
			isDirect: entry.isDirect,
			position,
			participants: entry.isDirect ? await readChannelPeople(settings, session, entry.channel.id) : []
		});
	}
	return read;
}

async function readChannelPeople(settings: MattermostSettings, session: MattermostSession, channelID: string) {
	const members = await ask<{ user_id: string }[]>(settings, session, 'GET', `/channels/${channelID}/members`);
	return members
		.filter((member) => member.user_id !== session.userID)
		.map((member) => ({ externalID: member.user_id }));
}

export async function openDirectChannel(
	settings: MattermostSettings,
	session: MattermostSession,
	externalIDs: string[]
) {
	const everyone = [...new Set([session.userID, ...externalIDs])];
	const channel =
		everyone.length === 2
			? await ask<MattermostChannel>(settings, session, 'POST', '/channels/direct', everyone)
			: await ask<MattermostChannel>(settings, session, 'POST', '/channels/group', everyone);
	return {
		id: channel.id,
		platform: 'mattermost',
		name: channel.display_name || channel.name,
		isDirect: true,
		position: 0,
		participants: externalIDs.map((externalID) => ({ externalID }))
	};
}

export async function readPosts(
	settings: MattermostSettings,
	session: MattermostSession,
	channelID: string,
	before?: string
) {
	const page = await ask<{ order: string[]; posts: Record<string, MattermostPost & { metadata?: { reactions?: MattermostReaction[] } }> }>(
		settings,
		session,
		'GET',
		`/channels/${channelID}/posts?per_page=50${before ? `&before=${encodeURIComponent(before)}` : ''}`
	);
	return page.order
		.map((id) => page.posts[id])
		.reverse()
		.map((post) => ({
			id: post.id,
			channelID: post.channel_id,
			parentID: post.root_id || undefined,
			author: { externalID: post.user_id },
			body: post.message,
			postedAt: new Date(post.create_at).toISOString(),
			editedAt: post.edit_at ? new Date(post.edit_at).toISOString() : undefined,
			reactions: reactionsOf(post.metadata?.reactions ?? [])
		}));
}

function reactionsOf(reactions: MattermostReaction[]) {
	const people = new Map<string, { externalID: string }[]>();
	for (const reaction of reactions) {
		const already = people.get(reaction.emoji_name) ?? [];
		already.push({ externalID: reaction.user_id });
		people.set(reaction.emoji_name, already);
	}
	return [...people].map(([emoji, who]) => ({ emoji, people: who }));
}

export async function writePost(
	settings: MattermostSettings,
	session: MattermostSession,
	channelID: string,
	body: string,
	parentID?: string
) {
	const post = await ask<MattermostPost>(settings, session, 'POST', '/posts', {
		channel_id: channelID,
		message: body,
		root_id: parentID ?? ''
	});
	return {
		id: post.id,
		channelID: post.channel_id,
		parentID: post.root_id || undefined,
		author: { externalID: post.user_id },
		body: post.message,
		postedAt: new Date(post.create_at).toISOString(),
		reactions: []
	};
}

export async function editPost(settings: MattermostSettings, session: MattermostSession, postID: string, body: string) {
	const post = await ask<MattermostPost>(settings, session, 'PUT', `/posts/${postID}/patch`, { message: body });
	return {
		id: post.id,
		channelID: post.channel_id,
		parentID: post.root_id || undefined,
		author: { externalID: post.user_id },
		body: post.message,
		postedAt: new Date(post.create_at).toISOString(),
		editedAt: post.edit_at ? new Date(post.edit_at).toISOString() : undefined,
		reactions: []
	};
}

export async function erasePost(settings: MattermostSettings, session: MattermostSession, postID: string) {
	await ask<void>(settings, session, 'DELETE', `/posts/${postID}`);
}

export async function addReaction(
	settings: MattermostSettings,
	session: MattermostSession,
	postID: string,
	emoji: string
) {
	await ask<void>(settings, session, 'POST', '/reactions', {
		user_id: session.userID,
		post_id: postID,
		emoji_name: emoji
	});
}

export async function removeReaction(
	settings: MattermostSettings,
	session: MattermostSession,
	postID: string,
	emoji: string
) {
	await ask<void>(settings, session, 'DELETE', `/users/${session.userID}/posts/${postID}/reactions/${emoji}`);
}

export async function readCustomEmoji(
	settings: MattermostSettings,
	session: MattermostSession
): Promise<{ name: string; url: string }[]> {
	const listed = await ask<{ id: string; name: string }[]>(
		settings,
		session,
		'GET',
		'/emoji?per_page=200'
	);
	const drawn = await Promise.all(
		listed.map(async (emoji) => {
			const response = await fetch(`${settings.baseURL}/api/v4/emoji/${encodeURIComponent(emoji.id)}/image`, {
				headers: { Authorization: `Bearer ${session.token}` }
			});
			if (!response.ok) return null;
			const type = response.headers.get('content-type') ?? 'image/png';
			const bytes = new Uint8Array(await response.arrayBuffer());
			if (bytes.length === 0 || bytes.length > 100_000) return null;
			return { name: emoji.name, url: `data:${type};base64,${Buffer.from(bytes).toString('base64')}` };
		})
	);
	return drawn.filter((emoji): emoji is { name: string; url: string } => emoji !== null);
}

export async function readProfilePicture(
	settings: MattermostSettings,
	session: MattermostSession,
	externalID: string
): Promise<{ dataURL: string } | null> {
	const response = await fetch(`${settings.baseURL}/api/v4/users/${encodeURIComponent(externalID)}/image`, {
		headers: { Authorization: `Bearer ${session.token}` }
	});
	if (!response.ok) return null;
	const type = response.headers.get('content-type') ?? 'image/png';
	const bytes = new Uint8Array(await response.arrayBuffer());
	if (bytes.length === 0 || bytes.length > 200_000) return null;
	return { dataURL: `data:${type};base64,${Buffer.from(bytes).toString('base64')}` };
}

export async function readPeople(settings: MattermostSettings, session: MattermostSession) {
	const users = await ask<MattermostUser[]>(settings, session, 'GET', '/users?per_page=200&active=true');
	return users.map((user) => ({
		externalID: user.id,
		name: [user.first_name, user.last_name].filter(Boolean).join(' ') || user.username,
		email: user.email
	}));
}
