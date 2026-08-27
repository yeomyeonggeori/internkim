import { channelName } from './channel-name';
import { currentLocale } from '$lib/i18n/locale.svelte';
import { channelText } from '$lib/i18n/channel-text';
import { supabaseMember } from '$lib/supabase-session';
import { customEmoji } from '$lib/stores/custom-emoji.svelte';
import { personPicture } from '$lib/stores/person-picture.svelte';
import { attachmentSource } from '$lib/stores/attachment-source.svelte';
import { emojifyText, glyphOfEmojiName } from './emoji-glyph';
import { customEmojiNamesIn } from './custom-emoji-names';
import {
	fetchChannels,
	fetchPosts,
	openDirectChannel,
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
	avatarURL?: string;
	platform?: string;
	webURL?: string;
};
type Person = { id: string; name: string; avatarURL?: string };
type Participant = { id: string; name: string; avatarURL?: string };
type Reaction = { emoji: string; count: number; reactedByMe: boolean; imageURL?: string; people?: Participant[] };
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
	reactions?: Reaction[];
	attachments?: Attachment[];
};
type Conversation = {
	conversationID: string;
	currentUserID: string;
	messages: Message[];
	hasMoreBefore: boolean;
	historyCursor: string;
};

const platform = 'mattermost';
const pageSize = 50;

// key is the one name this app knows the reader by; names is every name they
// answer to, because a conversation carries whichever one the messenger it came
// from uses.
type Viewer = { key: string; names: Set<string> };

let directory: MessengerDirectory | null = null;

function externalIDOf(person: MessengerPerson, people: MessengerDirectory): string {
	if (person.externalID) return person.externalID;
	return person.memberID ? (externalIDsOfMember(people, person.memberID)[0] ?? '') : '';
}

async function knownPeople(): Promise<MessengerDirectory> {
	directory ??= await fetchMessengerDirectory();
	return directory;
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
		name: personLabel(person, people, currentLocale.value),
		avatarURL: personPicture.pictureOfExternal(externalIDOf(person, people)) || undefined
	};
}


export async function bridgeConversations(): Promise<ChannelSummary[]> {
	const [channels, people] = await Promise.all([fetchChannels(), knownPeople()]);
	const viewer = await whoIsReading(people);
	await personPicture.rememberExternals(channels.flatMap((channel) => channel.participants.map((person) => externalIDOf(person, people))));
	return [...channels]
		.sort((left, right) => left.position - right.position)
		.map((channel) => ({
			id: channel.id,
			name: channelName(
				channel,
				people,
				(person) => isViewer(person, people, viewer),
				currentLocale.value,
				channelText[currentLocale.value].title
			),
			kind: channel.isDirect ? ('dm' as const) : ('group' as const),
			isPrivate: channel.isPrivate,
			avatarURL: channel.isDirect ? avatarOfDirect(channel, people, viewer) : undefined,
			platform: channel.platform,
			webURL: channel.webURL
		}));
}

// The other side of a direct conversation is drawn by their picture wherever
// the record can place them, and by the account the conversation named them by
// where it cannot.
function avatarOfDirect(channel: MessengerChannel, people: MessengerDirectory, viewer: Viewer): string | undefined {
	const other = channel.participants.find((person) => !isViewer(person, people, viewer));
	if (!other) return undefined;
	const memberID = other.memberID ?? (other.externalID ? people.memberOfExternal.get(other.externalID) : undefined);
	const drawn = memberID ? personPicture.pictureOf({ memberID }) : '';
	return drawn || personPicture.pictureOfExternal(externalIDOf(other, people)) || undefined;
}

export async function bridgePeople(): Promise<Person[]> {
	const people = await knownPeople();
	await personPicture.rememberExternals([...people.externalsOfMember.values()].flat());
	return [...people.nameOfMember]
		.map(([memberID, name]) => ({
			id: memberID,
			name,
			avatarURL: personPicture.pictureOf({ memberID }) || undefined
		}))
		.filter((person) => person.name)
		.sort((left, right) => left.name.localeCompare(right.name));
}

export async function bridgeDirectMessage(personID: string): Promise<string> {
	const people = await knownPeople();
	const externalID = externalIDsOfMember(people, personID)[0] ?? personID;
	const channel = await openDirectChannel([externalID]);
	return channel.id;
}

export async function bridgeConversation(channelID?: string, before?: string): Promise<Conversation> {
	const people = await knownPeople();
	const viewer = await whoIsReading(people);
	if (!channelID) {
		return { conversationID: '', currentUserID: viewer.key, messages: [], hasMoreBefore: false, historyCursor: '' };
	}
	const posts = await fetchPosts(channelID, before);
	await customEmoji.load();
	await customEmoji.draw(customEmojiNamesIn(posts));
	await personPicture.rememberExternals(posts.map((post) => externalIDOf(post.author, people)));
	await attachmentSource.wants(posts.flatMap((post) => post.attachments));
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
		reactions: post.reactions.map((reaction) => ({
			emoji: glyphOfEmojiName(reaction.emoji) ?? reaction.emoji,
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
	attachments: OutgoingAttachment[] = []
): Promise<void> {
	if (!channelID) throw new Error('choose a conversation first');
	await writePost(channelID, text, replyToRootID, attachments);
}
