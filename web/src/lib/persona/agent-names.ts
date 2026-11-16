export const agentDefaultNicknames = ['Intern Kim', '인턴킴'];

export function isDefaultNickname(name: string): boolean {
	return agentDefaultNicknames.includes(name);
}

export function nicknamesWithDefaults(nicknames: string[]): string[] {
	const added = nicknames.map((nickname) => nickname.trim()).filter((nickname) => nickname !== '' && !isDefaultNickname(nickname));
	return [...agentDefaultNicknames, ...new Set(added)];
}
