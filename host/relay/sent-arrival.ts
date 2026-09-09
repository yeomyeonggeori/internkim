import { readArrivedMessage, type ArrivedMessage } from './arrived';

type Answer = { status: number; body: unknown };

export type SentMessageDispatch = {
	askChatd: (capability: string, body: Record<string, unknown>) => Promise<Answer>;
	tellThoseAddressed: (arrived: ArrivedMessage) => Promise<number>;
};

const identityCapability = 'person.identity';
const conversationsCapability = 'person.conversations.list';

export async function tellAboutSentMessage(
	dispatch: SentMessageDispatch,
	body: Record<string, unknown>,
	actor: Record<string, unknown>,
	messageID: string
): Promise<number> {
	const arrived = await arrivalOfSentMessage(dispatch, body, actor, messageID);
	if (!arrived) return 0;
	return dispatch.tellThoseAddressed(arrived);
}

export async function arrivalOfSentMessage(
	dispatch: SentMessageDispatch,
	body: Record<string, unknown>,
	actor: Record<string, unknown>,
	messageID: string
): Promise<ArrivedMessage | null> {
	const conversationID = text(body.conversationID);
	if (!conversationID || !messageID) return null;
	const [identity, conversations] = await Promise.all([
		dispatch.askChatd(identityCapability, { actor }),
		dispatch.askChatd(conversationsCapability, { actor })
	]);
	return readArrivedMessage({
		conversationID,
		messageID,
		authorExternalID: externalIDOf(identity),
		recipientExternalIDs: participantsOf(conversations, conversationID),
		preview: text(body.body)
	});
}

function externalIDOf(answer: Answer): string {
	if (answer.status !== 200) return '';
	return text((answer.body as { externalID?: unknown } | null)?.externalID);
}

function participantsOf(answer: Answer, conversationID: string): string[] {
	if (answer.status !== 200) return [];
	const conversations = (answer.body as { conversations?: unknown } | null)?.conversations;
	if (!Array.isArray(conversations)) return [];
	const conversation = conversations.find((one) => (one as { id?: unknown }).id === conversationID) as
		| { participantExternalIDs?: unknown }
		| undefined;
	const participants = conversation?.participantExternalIDs;
	return Array.isArray(participants) ? participants.filter((one): one is string => typeof one === 'string') : [];
}

function text(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim() : '';
}
