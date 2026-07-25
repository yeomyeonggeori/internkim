export type AgentDirectMessageAuthor = 'me' | 'agent';

export type AgentChoiceOption = {
	key: string;
	label: string;
	value?: string;
};

export type AgentInteraction = {
	kind: string;
	question: string;
	options: AgentChoiceOption[];
	recommendedOptionKey?: string;
};

export type AgentDirectMessage = {
	id: string;
	author: AgentDirectMessageAuthor;
	text: string;
	sentAt: string;
	isProgress?: boolean;
	interaction?: AgentInteraction;
};

export type AgentConversation = {
	conversationID: string;
	messages: AgentDirectMessage[];
};

export async function fetchAgentConversation(): Promise<AgentConversation> {
	const response = await fetch('/agent/api/dm', { credentials: 'include', cache: 'no-store' });
	if (!response.ok) throw new Error(await response.text());
	return (await response.json()) as AgentConversation;
}

export async function sendAgentDirectMessage(message: string): Promise<void> {
	const response = await fetch('/agent/api/dm', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ message })
	});
	if (!response.ok) throw new Error(await response.text());
}
