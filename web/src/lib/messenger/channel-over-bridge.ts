import { channelName } from './channel-name';
import { currentLocale } from '$lib/i18n/locale.svelte';
import { currentPersonNameLocale } from '$lib/person-name.svelte';
import { channelText } from '$lib/i18n/channel-text';
import { supabaseMember } from '$lib/supabase-session';
import { customEmoji } from '$lib/stores/custom-emoji.svelte';
import { personPicture } from '$lib/stores/person-picture.svelte';
import { attachmentSource } from '$lib/stores/attachment-source.svelte';
import { emojifyText, glyphOfEmojiName } from './emoji-glyph';
import { customEmojiNamesIn } from './custom-emoji-names';
import { messengerCacheScope, requireCurrentMessengerScope } from './cache-scope';
import {
	addReaction,
	deletePost,
	editPost,
	fetchChannels,
	fetchPeople,
	fetchPosts,
	keepAttachmentForSending,
	openDirectChannel,
	removeReaction,
	writePost,
	type OutgoingAttachment,
	type MessengerChannel,
	type MessengerPerson,
	type MessengerPost
} from './messenger-api';
import {
	externalIDsOfMember,
	fetchMessengerDirectory,
	personKey,
	personLabel,
	type MessengerDirectory
} from './messenger-directory';

type ChannelSummary = {
	id: string;
	name: string;
	kind: 'dm' | 'group';
	isPrivate: boolean;
	counterpart?: MessengerPerson;
	members?: ChannelMember[];
	myRole?: ChannelRole;
	isWithTheAgent: boolean;
	description?: string;
	platform?: string;
	webURL?: string;
};
type Person = { id: string; name: string; avatarURL?: string };
type ChannelRole = 'owner' | 'admin' | 'member';
type ChannelMember = MessengerPerson & { name: string; role: ChannelRole };
type Participant = { id: string; name: string; avatarURL?: string; memberID?: string; externalID?: string };
type Reaction = { emoji: string; value: string; count: number; reactedByMe: boolean; imageURL?: string; people: Participant[] };
// url is where the message says the file is, which names it and is what the
// body's own link is stripped against; source is where this browser can
// actually open it.
type Attachment = {
	kind: 'image' | 'file';
	url: string;
	source: string;
	filename?: string;
	mimeType?: string;
	sizeBytes?: number;
	widthPixels?: number;
	heightPixels?: number;
};
type Message = {
	id: string;
	threadRootId?: string;
	sender: Participant;
	text: string;
	sentAt: string;
	editedAt?: string;
	mentions?: Mentions;
	reactions?: Reaction[];
	attachments?: Attachment[];
};
type Mentions = { externalIDs: string[]; isEveryone: boolean };
type Conversation = {
	conversationID: string;
	currentUserID: string;
	messages: Message[];
	hasMoreBefore: boolean;
	historyCursor: string;
};

const pageSize = 50;

// key is the one name this app knows the reader by; names is every name they
// answer to, because a conversation carries whichever one the messenger it came
// from uses.
type Viewer = { key: string; names: Set<string> };

function externalIDOf(person: MessengerPerson, people: MessengerDirectory): string {
	if (person.externalID) return person.externalID;
	return person.memberID ? (externalIDsOfMember(people, person.memberID)[0] ?? '') : '';
}

async function whoIsReading(people: MessengerDirectory): Promise<Viewer> {
	const { memberID } = await supabaseMember();
	if (!memberID) return { key: '', names: new Set() };
	const key = personKey({ memberID });
	const messengerAccounts = externalIDsOfMember(people, memberID).map((externalID) => personKey({ externalID }));
	return { key, names: new Set([key, ...messengerAccounts]) };
}

function isViewer(person: MessengerPerson, people: MessengerDirectory, viewer: Viewer): boolean {
	return viewer.names.has(canonicalKey(person, people)) || viewer.names.has(personKey(person));
}

function canonicalKey(person: MessengerPerson, people: MessengerDirectory): string {
	const memberID = person.memberID ?? (person.externalID ? people.memberOfExternal.get(person.externalID) : undefined);
	return memberID ? personKey({ memberID }) : personKey(person);
}

// The screen tells the reader's own messages apart by comparing this id against
// currentUserID, so one of theirs carries the id this app knows them by whatever
// the messenger called its author.
function senderOf(person: MessengerPerson, people: MessengerDirectory, viewer: Viewer): Participant {
	const sender = participantOf(person, people);
	return isViewer(person, people, viewer) ? { ...sender, id: viewer.key } : sender;
}

function participantOf(person: MessengerPerson, people: MessengerDirectory): Participant {
	return {
		id: canonicalKey(person, people),
		name: personLabel(person, people, currentPersonNameLocale()),
		...placementOf(person, people)
	};
}

function placementOf(person: MessengerPerson, people: MessengerDirectory): MessengerPerson {
	const memberID = person.memberID ?? (person.externalID ? people.memberOfExternal.get(person.externalID) : undefined);
	const externalID = externalIDOf(person, people);
	return { ...(memberID ? { memberID } : {}), ...(externalID ? { externalID } : {}) };
}

export async function bridgeConversations(): Promise<ChannelSummary[]> {
	const scope = await messengerCacheScope();
	const [answer, people] = await Promise.all([fetchChannels(), fetchMessengerDirectory()]);
	const { channels, agentExternalID } = answer;
	const [viewer, messengerNames] = await Promise.all([whoIsReading(people), messengerNamesOf(channels, people)]);
	await requireCurrentMessengerScope(scope);
	void personPicture.rememberExternals(channels.flatMap((channel) => channel.participants.map((person) => externalIDOf(person, people))));
	return [...channels]
		.sort((left, right) => left.position - right.position)
		.map((channel) => ({
			id: channel.id,
			name: channelName(
				channel,
				people,
				(person) => isViewer(person, people, viewer),
				currentPersonNameLocale(),
				channelText[currentLocale.value].title
			),
			kind: channel.isDirect ? ('dm' as const) : ('group' as const),
			isPrivate: channel.isPrivate,
			counterpart: channel.isDirect ? counterpartOf(channel, people, viewer) : undefined,
			members: channel.isDirect
				? undefined
				: membersOf(channel, people, messengerNames, agentExternalID),
			myRole: channel.isDirect ? undefined : roleOfViewer(channel, people, viewer),
			isWithTheAgent: channel.isWithTheAgent === true,
			description: channel.description,
			platform: channel.platform,
			webURL: channel.webURL,
			unreadCount: channel.unreadCount ?? 0
		}));
}

function counterpartOf(channel: MessengerChannel, people: MessengerDirectory, viewer: Viewer): MessengerPerson | undefined {
	const other = channel.participants.find((person) => !isViewer(person, people, viewer));
	return other ? placementOf(other, people) : undefined;
}

function roleOfViewer(channel: MessengerChannel, people: MessengerDirectory, viewer: Viewer): ChannelRole {
	const me = channel.participants.find((person) => isViewer(person, people, viewer));
	return me ? roleOf(channel, externalIDOf(me, people)) : 'member';
}

function roleOf(channel: MessengerChannel, externalID: string): ChannelRole {
	const role = channel.roleOfExternalID?.[externalID];
	return role === 'owner' || role === 'admin' ? role : 'member';
}

function membersOf(
	channel: MessengerChannel,
	people: MessengerDirectory,
	messengerNames: Map<string, string>,
	agentExternalID?: string
): ChannelMember[] {
	return channel.participants.map((person) => {
		const externalID = externalIDOf(person, people);
		const name =
			personLabel(person, people, currentPersonNameLocale()) ||
			(externalID && externalID === agentExternalID ? channelText[currentLocale.value].title : '') ||
			messengerNames.get(externalID) ||
			'';
		return { ...placementOf(person, people), name, role: roleOf(channel, externalID) };
	});
}

async function messengerNamesOf(
	channels: MessengerChannel[],
	people: MessengerDirectory
): Promise<Map<string, string>> {
	const unknown = channels
		.filter((channel) => !channel.isDirect)
		.flatMap((channel) => channel.participants)
		.filter((person) => !personLabel(person, people, currentPersonNameLocale()))
		.map((person) => externalIDOf(person, people))
		.filter(Boolean);
	if (unknown.length === 0) return new Map();
	const known = await fetchPeople().catch((failure: unknown) => {
		console.warn('the messenger did not answer with its people', failure);
		return [];
	});
	return new Map(known.map((person) => [person.externalID, person.name]));
}

export async function bridgePeople(): Promise<Person[]> {
	const people = await fetchMessengerDirectory();
	void personPicture.rememberEveryone();
	return [...people.nameOfMember]
		.map(([memberID, name]) => ({ id: memberID, name }))

		.filter((person) => person.name)
		.sort((left, right) => left.name.localeCompare(right.name));
}

export async function bridgeDirectMessage(personID: string): Promise<string> {
	const people = await fetchMessengerDirectory();
	const externalID = externalIDsOfMember(people, personID)[0] ?? personID;
	const channel = await openDirectChannel([externalID]);
	return channel.id;
}

export async function bridgeAgentConversation(): Promise<string> {
	return (await openDirectChannel([])).id;
}

export async function bridgeConversation(channelID?: string, before?: string): Promise<Conversation> {
	const scope = await messengerCacheScope();
	const [people, posts] = await Promise.all([
		fetchMessengerDirectory(),
		channelID ? fetchPosts(channelID, before) : Promise.resolve([])
	]);
	const viewer = await whoIsReading(people);
	await requireCurrentMessengerScope(scope);
	if (!channelID) {
		return { conversationID: '', currentUserID: viewer.key, messages: [], hasMoreBefore: false, historyCursor: '' };
	}
	void customEmoji.load().then(() => customEmoji.draw(customEmojiNamesIn(posts))).catch((failure: unknown) =>
		console.warn('custom emoji did not load', failure)
	);
	void personPicture.rememberExternals(posts.map((post) => externalIDOf(post.author, people)));
	void attachmentSource.wants(posts.flatMap((post) => post.attachments));
	return {
		conversationID: channelID,
		currentUserID: viewer.key,
		messages: posts.map((post) => messageOf(post, people, viewer)),
		hasMoreBefore: posts.length >= pageSize,
		historyCursor: posts[0]?.id ?? ''
	};
}

function messageOf(post: MessengerPost, people: MessengerDirectory, viewer: Viewer): Message {
	return {
		id: post.id,
		threadRootId: post.parentID,
		sender: senderOf(post.author, people, viewer),
		text: emojifyText(post.body),
		sentAt: post.postedAt,
		editedAt: post.editedAt,
		mentions: post.mentions,
		reactions: post.reactions.map((reaction) => ({
			emoji: glyphOfEmojiName(reaction.emoji) ?? reaction.emoji,
			value: reaction.emoji,
			count: reaction.people.length,
			imageURL: reaction.imageURL ?? customEmoji.nameToURL.get(reaction.emoji),
			reactedByMe: reaction.people.some((person) => isViewer(person, people, viewer)),
			people: reaction.people.map((person) => participantOf(person, people))
		})),
		attachments: post.attachments.map((attachment) => ({
			kind: attachment.contentType.startsWith('image/') ? ('image' as const) : ('file' as const),
			url: attachment.url,
			source: attachmentSource.openable(attachment.url),
			filename: attachment.filename,
			mimeType: attachment.contentType,
			sizeBytes: attachment.sizeBytes,
			widthPixels: attachment.widthPixels,
			heightPixels: attachment.heightPixels
		}))
	};
}

export async function bridgeSendMessage(
	text: string,
	channelID?: string,
	replyToRootID?: string,
	attachments: OutgoingAttachment[] = [],
	mentions?: Mentions
): Promise<void> {
	if (!channelID) throw new Error('choose a conversation first');
	const kept = [];
	for (const attachment of attachments) kept.push(await keepAttachmentForSending(attachment));
	await writePost(channelID, text, replyToRootID, kept, mentions);
}

export async function bridgeEditMessage(channelID: string | undefined, messageID: string, text: string): Promise<void> {
	if (!channelID) throw new Error('choose a conversation first');
	await editPost(channelID, messageID, text);
}

export async function bridgeDeleteMessage(channelID: string | undefined, messageID: string): Promise<void> {
	if (!channelID) throw new Error('choose a conversation first');
	await deletePost(channelID, messageID);
}

export async function bridgeAddReaction(channelID: string | undefined, messageID: string, value: string): Promise<void> {
	if (!channelID) throw new Error('choose a conversation first');
	await addReaction(channelID, messageID, value);
}

export async function bridgeRemoveReaction(channelID: string | undefined, messageID: string, value: string): Promise<void> {
	if (!channelID) throw new Error('choose a conversation first');
	await removeReaction(channelID, messageID, value);
}
