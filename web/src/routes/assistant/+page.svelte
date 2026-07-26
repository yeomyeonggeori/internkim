<script lang="ts">
	import Channel from '$lib/components/channel/channel.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const text = createPageText(channelText);
</script>

<svelte:head>
	<title>{text.title}</title>
</svelte:head>

<div class="flex min-h-0 w-full flex-1 justify-center p-4">
	<Card.Root class="flex min-h-0 w-full max-w-3xl flex-col gap-0 overflow-hidden">
		<Card.Header class="gap-1 border-b">
			<Card.Title>{text.title}</Card.Title>
			<Card.Description>{text.description}</Card.Description>
		</Card.Header>
		<Channel />
	</Card.Root>
</div>
