import { supabase } from '$lib/supabase';
import {
	fetchChannels,
	fetchPosts,
	openDirectChannel,
	writePost,
	type MessengerChannel,
	type MessengerPerson,
	type MessengerPost
} from './messenger-api';
import { fetchMessengerDirectory, personKey, personLabel, type MessengerDirectory } from './messenger-directory';

type ChannelSummary = { id: string; name: string; kind: 'dm' | 'group'; avatarURL?: string };
type Person = { id: string; name: string; avatarURL?: string };
type Participant = { id: string; name: string };
type Reaction = { emoji: string; count: number; reactedByMe: boolean; people?: Participant[] };
type Message = {
	id: string;
	threadRootId?: string;
	sender: Participant;
	text: string;
	sentAt: string;
	reactions?: Reaction[];
};
type Conversation = {
	conversationID: string;
	currentUserID: string;
	messages: Message[];
	hasMoreBefore: boolean;
	historyCursor: string;
};

const platform = 'mattermost';

let directory: MessengerDirectory | null = null;

async function knownPeople(): Promise<MessengerDirectory> {
	directory ??= await fetchMessengerDirectory();
	return directory;
}

async function myPersonKey(): Promise<string> {
	const { data } = await supabase().auth.getSession();
	const accountID = data.session?.user.id;
	if (!accountID) return '';
	const member = await supabase().from('member').select('id').eq('user_id', accountID).maybeSingle<{ id: string }>();
	return member.data ? personKey({ memberID: member.data.id }) : '';
}

function participantOf(person: MessengerPerson, people: MessengerDirectory): Participant {
	return { id: personKey(person), name: personLabel(person, people) };
}

function channelName(channel: MessengerChannel, people: MessengerDirectory, mine: string): string {
	if (!channel.isDirect) return channel.name;
	const others = channel.participants.filter((person) => personKey(person) !== mine);
	const named = (others.length > 0 ? others : channel.participants).map((person) => personLabel(person, people));
	return named.filter(Boolean).join(', ') || channel.name;
}

export async function bridgeConversations(): Promise<ChannelSummary[]> {
	const [channels, people, mine] = await Promise.all([fetchChannels(platform), knownPeople(), myPersonKey()]);
	return [...channels]
		.sort((left, right) => left.position - right.position)
		.map((channel) => ({
			id: channel.id,
			name: channelName(channel, people, mine),
			kind: channel.isDirect ? ('dm' as const) : ('group' as const)
		}));
}

export async function bridgePeople(): Promise<Person[]> {
	const people = await knownPeople();
	return [...people.nameOfMember]
		.map(([memberID, name]) => ({ id: memberID, name }))
		.filter((person) => person.name)
		.sort((left, right) => left.name.localeCompare(right.name));
}

export async function bridgeDirectMessage(personID: string): Promise<string> {
	const channel = await openDirectChannel([personID], platform);
	return channel.id;
}

export async function bridgeConversation(channelID?: string, before?: string): Promise<Conversation> {
	const [people, mine] = await Promise.all([knownPeople(), myPersonKey()]);
	if (!channelID) {
		return { conversationID: '', currentUserID: mine, messages: [], hasMoreBefore: false, historyCursor: '' };
	}
	const posts = await fetchPosts(channelID, before);
	return {
		conversationID: channelID,
		currentUserID: mine,
		messages: posts.map((post) => messageOf(post, people, mine)),
		hasMoreBefore: posts.length > 0,
		historyCursor: posts[0]?.id ?? ''
	};
}

function messageOf(post: MessengerPost, people: MessengerDirectory, mine: string): Message {
	return {
		id: post.id,
		threadRootId: post.parentID,
		sender: participantOf(post.author, people),
		text: post.body,
		sentAt: post.postedAt,
		reactions: post.reactions.map((reaction) => ({
			emoji: reaction.emoji,
			count: reaction.people.length,
			reactedByMe: reaction.people.some((person) => personKey(person) === mine),
			people: reaction.people.map((person) => participantOf(person, people))
		}))
	};
}

export async function bridgeSendMessage(text: string, channelID?: string, replyToRootID?: string): Promise<void> {
	if (!channelID) throw new Error('choose a conversation first');
	await writePost(channelID, text, replyToRootID);
}
