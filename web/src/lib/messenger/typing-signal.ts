export const typingAnnounceIntervalMilliseconds = 3_000;
export const typingLifetimeMilliseconds = 8_000;
export const typingEventKind = 'typing.started';
const sentJustBeforeMilliseconds = 2_000;

export type Typer = { externalID: string; heardAt: number };

export type TypingText = {
	typingAgent: string;
	typingUnnamed: string;
	typingOne: string;
	typingTwo: string;
	typingMany: string;
};

export type ActivityText = TypingText & { working: string };

export type Typist = { name: string; isAgent: boolean };

export type ConversationActivity = {
	typists: Typist[];
	isAgentWorking: boolean;
};

export type SentMessage = { senderExternalID?: string; sentAt: string };

export function typersAfterSignal(typers: Typer[], externalID: string, now: number): Typer[] {
	return [...typers.filter((typer) => typer.externalID !== externalID), { externalID, heardAt: now }];
}

export function typersStillTyping(typers: Typer[], messages: SentMessage[], now: number): Typer[] {
	return typers.filter(
		(typer) => now - typer.heardAt < typingLifetimeMilliseconds && !hasSentSince(typer, messages)
	);
}

function hasSentSince(typer: Typer, messages: SentMessage[]): boolean {
	return messages.some(
		(message) => message.senderExternalID === typer.externalID && Date.parse(message.sentAt) + sentJustBeforeMilliseconds >= typer.heardAt
	);
}

export function isTimeToAnnounceTyping(lastAnnouncedAt: number | null, now: number): boolean {
	return lastAnnouncedAt === null || now - lastAnnouncedAt >= typingAnnounceIntervalMilliseconds;
}

export function typingLabel(typists: Typist[], text: TypingText): string {
	if (typists.length === 0) return '';
	if (typists.length === 1 && typists[0].isAgent) return text.typingAgent;
	const names = typists.flatMap((typist) => typist.name || []);
	if (names.length === 0) return text.typingUnnamed;
	const [first, second] = names;
	if (names.length === 1) return text.typingOne.replace('{first}', first);
	const named = text.typingTwo.replace('{first}', first).replace('{second}', second);
	if (names.length === 2) return named;
	return text.typingMany
		.replace('{first}', first)
		.replace('{second}', second)
		.replace('{count}', String(names.length - 2));
}

export function activityLabel(activity: ConversationActivity, text: ActivityText): string {
	return typingLabel(activity.typists, text) || (activity.isAgentWorking ? text.working : '');
}
