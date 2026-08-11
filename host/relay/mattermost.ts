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
	token: string,
	method: string,
	path: string,
	body?: unknown
): Promise<Value> {
	const response = await fetch(`${settings.baseURL}/api/v4${path}`, {
		method,
		headers: {
			Authorization: `Bearer ${token}`,
			...(body === undefined ? {} : { 'Content-Type': 'application/json' })
		},
		body: body === undefined ? undefined : JSON.stringify(body)
	});
	if (!response.ok) throw new Error(`mattermost ${method} ${path} returned ${response.status}`);
	if (response.status === 204) return undefined as Value;
	return (await response.json()) as Value;
}

type MattermostUser = { id: string; username: string; first_name: string; last_name: string; email: string };

export async function readCustomEmoji(
	settings: MattermostSettings,
	token: string
): Promise<{ name: string; url: string }[]> {
	const listed = await ask<{ id: string; name: string }[]>(
		settings,
		token,
		'GET',
		'/emoji?per_page=200'
	);
	const drawn = await Promise.all(
		listed.map(async (emoji) => {
			const response = await fetch(`${settings.baseURL}/api/v4/emoji/${encodeURIComponent(emoji.id)}/image`, {
				headers: { Authorization: `Bearer ${token}` }
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
	token: string,
	externalID: string,
	largestBytes: number
): Promise<{ dataURL: string } | null> {
	const response = await fetch(`${settings.baseURL}/api/v4/users/${encodeURIComponent(externalID)}/image`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	if (!response.ok) return null;
	const type = response.headers.get('content-type') ?? 'image/png';
	const bytes = new Uint8Array(await response.arrayBuffer());
	if (bytes.length === 0 || bytes.length > largestBytes) return null;
	return { dataURL: `data:${type};base64,${Buffer.from(bytes).toString('base64')}` };
}

export async function readPeople(settings: MattermostSettings, token: string) {
	const users = await ask<MattermostUser[]>(settings, token, 'GET', '/users?per_page=200&active=true');
	return users.map((user) => ({
		externalID: user.id,
		name: [user.first_name, user.last_name].filter(Boolean).join(' ') || user.username,
		email: user.email
	}));
}

export async function mintUserAccessToken(
	settings: MattermostSettings,
	token: string,
	externalID: string
): Promise<string> {
	const minted = await ask<{ token?: string }>(
		settings,
		token,
		'POST',
		`/users/${encodeURIComponent(externalID)}/tokens`,
		{ description: 'internkim-messenger' }
	);
	if (!minted.token) throw new Error('mattermost returned no personal access token');
	return minted.token;
}
