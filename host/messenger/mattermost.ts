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

export async function readChannels(settings: MattermostSettings, session: MattermostSession) {
	const teams = await ask<{ id: string }[]>(settings, session, 'GET', `/users/me/teams`);
	const channels: MattermostChannel[] = [];
	for (const team of teams) {
		channels.push(
			...(await ask<MattermostChannel[]>(settings, session, 'GET', `/users/me/teams/${team.id}/channels`))
		);
	}
	return channels
		.filter((channel) => channel.type !== 'D' || channel.display_name)
		.map((channel, index) => ({
			id: channel.id,
			platform: 'mattermost',
			name: channel.display_name || channel.name,
			isDirect: channel.type === 'D' || channel.type === 'G',
			position: index,
			participants: []
		}));
}

export async function readPosts(settings: MattermostSettings, session: MattermostSession, channelID: string) {
	const page = await ask<{ order: string[]; posts: Record<string, MattermostPost> }>(
		settings,
		session,
		'GET',
		`/channels/${channelID}/posts?per_page=50`
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
			reactions: []
		}));
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

export async function readPeople(settings: MattermostSettings, session: MattermostSession) {
	const users = await ask<MattermostUser[]>(settings, session, 'GET', '/users?per_page=200&active=true');
	return users.map((user) => ({
		externalID: user.id,
		name: [user.first_name, user.last_name].filter(Boolean).join(' ') || user.username,
		email: user.email
	}));
}
