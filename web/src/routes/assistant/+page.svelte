<script lang="ts">
	import AgentDmConversation from '$lib/components/agent-dm/agent-dm-conversation.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import { agentDmText } from '$lib/i18n/agent-dm-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const text = createPageText(agentDmText);
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
		<AgentDmConversation />
	</Card.Root>
</div>
