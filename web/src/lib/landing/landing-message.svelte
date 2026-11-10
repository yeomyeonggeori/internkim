<script lang="ts">
	import ChannelTextBubble from '$lib/components/channel/channel-text-bubble.svelte';
	import SenderAvatar from '$lib/components/channel/sender-avatar.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Message from '$lib/components/ui/message';
	import { agentPicture } from './links';

	let { from, text, agentName }: { from: string; text: string; agentName: string } = $props();

	const isPerson = $derived(from === 'person');
</script>

<Message.Root align={isPerson ? 'end' : 'start'}>
	{#if !isPerson}
		<SenderAvatar><PersonAvatar name={agentName} image={agentPicture} /></SenderAvatar>
	{/if}
	<Message.Content>
		<ChannelTextBubble mine={isPerson} source={text} />
	</Message.Content>
</Message.Root>
