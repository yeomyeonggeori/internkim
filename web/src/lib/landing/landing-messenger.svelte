<script lang="ts">
	import HashIcon from '@lucide/svelte/icons/hash';
	import ChannelTextBubble from '$lib/components/channel/channel-text-bubble.svelte';
	import SenderAvatar from '$lib/components/channel/sender-avatar.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Message from '$lib/components/ui/message';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { agentPicture } from './links';
	import { landingText } from './text';

	const text = createPageText(landingText);
	const agentName = $derived(text.demo.conversation);
	const mentionLabels = $derived([{ label: agentName }]);
</script>

<div class="grid">
	<div class="flex h-12 items-center gap-1.5 border-b px-4 text-sm font-medium">
		<HashIcon class="text-muted-foreground size-4" />
		{text.channel.name}
	</div>
	<div class="grid gap-4 p-4 sm:p-6">
		{#each text.channel.messages as message, index (index)}
			{@const isAgent = message.sender === agentName}
			{@const isMine = message.sender === text.channel.viewer}
			<Message.Root align={isMine ? 'end' : 'start'}>
				{#if !isMine}
					<SenderAvatar><PersonAvatar name={message.sender} image={isAgent ? agentPicture : ''} /></SenderAvatar>
				{/if}
				<Message.Content>
					{#if !isMine}<Message.Header class="-mb-2 px-0">{message.sender}</Message.Header>{/if}
					<ChannelTextBubble mine={isMine} source={message.text} {mentionLabels} />
				</Message.Content>
			</Message.Root>
		{/each}
	</div>
</div>
