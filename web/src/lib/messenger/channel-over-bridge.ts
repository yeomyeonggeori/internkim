import { channelName } from './channel-name';
import { currentLocale } from '$lib/i18n/locale.svelte';
import { channelText } from '$lib/i18n/channel-text';
import { supabaseMember } from '$lib/supabase-session';
import { customEmoji } from '$lib/stores/custom-emoji.svelte';
import { personPicture } from '$lib/stores/person-picture.svelte';
import { attachmentSource } from '$lib/stores/attachment-source.svelte';
import { emojify, get as glyphOf } from 'node-emoji';
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
import { fetchMessengerDirectory, personKey, personLabel, type MessengerDirectory } from './messenger-directory';

type ChannelSummary = {
	id: string;
	name: string;
	kind: 'dm' | 'group';
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

let directory: MessengerDirectory | null = null;

function externalIDOf(person: MessengerPerson, people: MessengerDirectory): string {
	if (person.externalID) return person.externalID;
	return person.memberID ? (people.externalOfMember.get(person.memberID) ?? '') : '';
}

async function knownPeople(): Promise<MessengerDirectory> {
	directory ??= await fetchMessengerDirectory();
	return directory;
}

async function myPersonKey(): Promise<string> {
	const { memberID } = await supabaseMember();
	return memberID ? personKey({ memberID }) : '';
}

function canonicalKey(person: MessengerPerson, people: MessengerDirectory): string {
	const memberID = person.memberID ?? (person.externalID ? people.memberOfExternal.get(person.externalID) : undefined);
	return memberID ? personKey({ memberID }) : personKey(person);
}

function participantOf(person: MessengerPerson, people: MessengerDirectory): Participant {
	return {
		id: canonicalKey(person, people),
		name: personLabel(person, people, currentLocale.value),
		avatarURL: personPicture.pictureOfExternal(externalIDOf(person, people)) || undefined
	};
}


export async function bridgeConversations(): Promise<ChannelSummary[]> {
	const [channels, people, mine] = await Promise.all([fetchChannels(), knownPeople(), myPersonKey()]);
	await personPicture.rememberExternals(channels.flatMap((channel) => channel.participants.map((person) => externalIDOf(person, people))));
	return [...channels]
		.sort((left, right) => left.position - right.position)
		.map((channel) => ({
			id: channel.id,
			name: channelName(channel, people, mine, canonicalKey, currentLocale.value, channelText[currentLocale.value].title),
			kind: channel.isDirect ? ('dm' as const) : ('group' as const),
			avatarURL: channel.isDirect ? avatarOfDirect(channel, people, mine) : undefined,
			platform: channel.platform,
			webURL: channel.webURL
		}));
}

function avatarOfDirect(channel: MessengerChannel, people: MessengerDirectory, mine: string): string | undefined {
	const other = channel.participants.find((person) => canonicalKey(person, people) !== mine);
	return other ? personPicture.pictureOfExternal(externalIDOf(other, people)) || undefined : undefined;
}

export async function bridgePeople(): Promise<Person[]> {
	const people = await knownPeople();
	await personPicture.rememberExternals([...people.externalOfMember.values()]);
	return [...people.nameOfMember]
		.map(([memberID, name]) => ({
			id: memberID,
			name,
			avatarURL: personPicture.pictureOfExternal(people.externalOfMember.get(memberID) ?? '') || undefined
		}))
		.filter((person) => person.name)
		.sort((left, right) => left.name.localeCompare(right.name));
}

export async function bridgeDirectMessage(personID: string): Promise<string> {
	const people = await knownPeople();
	const externalID = people.externalOfMember.get(personID) ?? personID;
	const channel = await openDirectChannel([externalID]);
	return channel.id;
}

export async function bridgeConversation(channelID?: string, before?: string): Promise<Conversation> {
	const [people, mine] = await Promise.all([knownPeople(), myPersonKey()]);
	if (!channelID) {
		return { conversationID: '', currentUserID: mine, messages: [], hasMoreBefore: false, historyCursor: '' };
	}
	const posts = await fetchPosts(channelID, before);
	await customEmoji.load();
	await customEmoji.draw(customEmojiNamesIn(posts));
	await personPicture.rememberExternals(posts.map((post) => externalIDOf(post.author, people)));
	await attachmentSource.wants(posts.flatMap((post) => post.attachments));
	return {
		conversationID: channelID,
		currentUserID: mine,
		messages: posts.map((post) => messageOf(post, people, mine)),
		hasMoreBefore: posts.length >= pageSize,
		historyCursor: posts[0]?.id ?? ''
	};
}

function messageOf(post: MessengerPost, people: MessengerDirectory, mine: string): Message {
	return {
		id: post.id,
		threadRootId: post.parentID,
		sender: participantOf(post.author, people),
		text: emojify(post.body),
		sentAt: post.postedAt,
		reactions: post.reactions.map((reaction) => ({
			emoji: glyphOf(reaction.emoji) ?? reaction.emoji,
			count: reaction.people.length,
			imageURL: reaction.imageURL ?? customEmoji.nameToURL.get(reaction.emoji),
			reactedByMe: reaction.people.some((person) => canonicalKey(person, people) === mine),
			people: reaction.people.map((person) => participantOf(person, people))
		})),
		attachments: post.attachments.map((attachment) => ({
			kind: attachment.contentType.startsWith('image/') ? ('image' as const) : ('file' as const),
			url: attachment.url,
			source: attachmentSource.openable(attachment.url),
			filename: attachment.filename,
			mimeType: attachment.contentType,
			sizeBytes: attachment.sizeBytes
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
