export const typingAnnounceIntervalMilliseconds = 3_000;
export const typingLifetimeMilliseconds = 8_000;
export const typingEventKind = 'typing.started';
const sentJustBeforeMilliseconds = 2_000;

export type Typer = { externalID: string; heardAt: number };

export type TypingText = {
	typingDirect: string;
	typingOne: string;
	typingTwo: string;
	typingMany: string;
};

export type ActivityText = TypingText & { working: string };

export type ConversationActivity = {
	typerExternalIDs: string[];
	nameOf: (externalID: string) => string | undefined;
	isGroup: boolean;
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

export function typingLabel(names: string[], isDirect: boolean, text: TypingText): string {
	if (names.length === 0) return '';
	if (isDirect) return text.typingDirect;
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
	const working = activity.isAgentWorking ? text.working : '';
	if (!activity.isGroup) return working || typingLabel(activity.typerExternalIDs, true, text);
	const names = activity.typerExternalIDs.flatMap((externalID) => activity.nameOf(externalID) ?? []);
	return typingLabel(names, false, text) || working;
}
