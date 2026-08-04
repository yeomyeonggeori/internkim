<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { HostUnreachableError } from '$lib/host-bridge';
	import {
		fetchChannels,
		fetchPosts,
		writePost,
		type MessengerChannel,
		type MessengerPost
	} from '$lib/messenger/messenger-api';
	import {
		fetchMessengerDirectory,
		personKey,
		personLabel,
		type MessengerDirectory
	} from '$lib/messenger/messenger-directory';
	import { channelLabel, threadsOf } from './messenger-thread';
	import { onMount } from 'svelte';

	let channels = $state<MessengerChannel[]>([]);
	let posts = $state<MessengerPost[]>([]);
	let directory = $state<MessengerDirectory | null>(null);
	let selectedChannelID = $state('');
	let replyingTo = $state<MessengerPost | null>(null);
	let draft = $state('');
	let isSending = $state(false);
	let notice = $state('');

	const selectedChannel = $derived(channels.find((channel) => channel.id === selectedChannelID));
	const threads = $derived(threadsOf(posts));

	function nameOf(post: MessengerPost): string {
		return directory ? personLabel(post.author, directory) : '';
	}

	function reactionLabel(people: { memberID?: string; externalID?: string }[]): string {
		if (!directory) return '';
		return people.map((person) => personLabel(person, directory!)).filter(Boolean).join(', ');
	}

	async function load() {
		try {
			directory = await fetchMessengerDirectory();
			channels = await fetchChannels();
			notice = channels.length === 0 ? '연결된 대화가 없습니다.' : '';
			if (channels.length > 0) await openChannel(channels[0].id);
		} catch (error) {
			notice = noticeOf(error);
		}
	}

	async function openChannel(channelID: string) {
		selectedChannelID = channelID;
		replyingTo = null;
		posts = [];
		try {
			posts = await fetchPosts(channelID);
			notice = '';
		} catch (error) {
			notice = noticeOf(error);
		}
	}

	async function send(event: SubmitEvent) {
		event.preventDefault();
		const body = draft.trim();
		if (!body || !selectedChannelID) return;
		isSending = true;
		try {
			const written = await writePost(selectedChannelID, body, replyingTo?.id);
			posts = [...posts, written];
			draft = '';
			replyingTo = null;
			notice = '';
		} catch (error) {
			notice = noticeOf(error);
		} finally {
			isSending = false;
		}
	}

	function noticeOf(error: unknown): string {
		if (error instanceof HostUnreachableError) return '앱이 실행 중이 아닙니다.';
		return error instanceof Error ? error.message : '알 수 없는 문제가 생겼습니다.';
	}

	onMount(load);
</script>

<main class="grid h-[calc(100vh-3rem)] min-h-0 grid-cols-[16rem_minmax(0,1fr)] bg-background text-foreground">
	<aside class="min-h-0 overflow-y-auto border-r p-2" data-testid="messenger-channels">
		{#each channels as channel (channel.id)}
			<button
				type="button"
				class="w-full truncate rounded-md px-3 py-2 text-left text-sm hover:bg-muted {channel.id === selectedChannelID ? 'bg-muted font-medium' : ''}"
				onclick={() => openChannel(channel.id)}
			>
				{channel.isDirect ? '@' : '#'}
				{channelLabel(channel, directory)}
			</button>
		{/each}
	</aside>

	<section class="grid min-h-0 grid-rows-[auto_minmax(0,1fr)_auto]">
		<header class="border-b px-6 py-3 text-sm font-medium">
			{selectedChannel ? channelLabel(selectedChannel, directory) : '대화'}
		</header>

		<div class="min-h-0 space-y-4 overflow-y-auto px-6 py-4" data-testid="messenger-posts">
			{#if notice}
				<p class="rounded-md border bg-muted/30 px-4 py-3 text-sm text-muted-foreground">{notice}</p>
			{/if}
			{#each threads as thread (thread.post.id)}
				<article class="space-y-2">
					{#snippet written(post: MessengerPost)}
						<div class="flex gap-3">
							<PersonAvatar name={nameOf(post)} seed={personKey(post.author)} class="size-8 shrink-0" />
							<div class="min-w-0 flex-1">
								<p class="text-sm font-medium">{nameOf(post)}</p>
								<p class="text-sm whitespace-pre-wrap">{post.body}</p>
								{#if post.reactions.length > 0}
									<div class="mt-1 flex flex-wrap gap-1">
										{#each post.reactions as reaction (reaction.emoji)}
											<span class="rounded-full border px-2 py-0.5 text-xs" title={reactionLabel(reaction.people)}>
												:{reaction.emoji}: {reaction.people.length}
											</span>
										{/each}
									</div>
								{/if}
								<button
									type="button"
									class="mt-1 text-xs text-muted-foreground underline"
									onclick={() => (replyingTo = thread.post)}
								>
									답글
								</button>
							</div>
						</div>
					{/snippet}

					{@render written(thread.post)}
					{#if thread.replies.length > 0}
						<div class="ml-11 space-y-2 border-l pl-4">
							{#each thread.replies as reply (reply.id)}
								{@render written(reply)}
							{/each}
						</div>
					{/if}
				</article>
			{/each}
		</div>

		<form class="grid gap-2 border-t px-6 py-3" onsubmit={send}>
			{#if replyingTo}
				<p class="flex items-center gap-2 text-xs text-muted-foreground">
					<span class="truncate">답글: {replyingTo.body}</span>
					<button type="button" class="underline" onclick={() => (replyingTo = null)}>취소</button>
				</p>
			{/if}
			<div class="flex gap-2">
				<Input bind:value={draft} placeholder="메시지" disabled={isSending || !selectedChannelID} />
				<Button type="submit" disabled={isSending || !draft.trim() || !selectedChannelID}>보내기</Button>
			</div>
		</form>
	</section>
</main>
