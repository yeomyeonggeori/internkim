<script lang="ts">
	import Channel from './channel.svelte';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import MessageCircleIcon from '@lucide/svelte/icons/message-circle';

	const text = createPageText(channelText);
	let isOpen = $state(false);
</script>

<Sheet.Root bind:open={isOpen}>
	<Tooltip.Root>
		<Tooltip.Trigger>
			{#snippet child({ props })}
				<Sheet.Trigger>
					{#snippet child({ props: triggerProps })}
						<Button
							{...props}
							{...triggerProps}
							variant="ghost"
							size="icon"
							aria-label={text.openLabel}
						>
							<MessageCircleIcon />
						</Button>
					{/snippet}
				</Sheet.Trigger>
			{/snippet}
		</Tooltip.Trigger>
		<Tooltip.Content>{text.openLabel}</Tooltip.Content>
	</Tooltip.Root>
	<Sheet.Content side="right" class="flex w-full flex-col gap-0 p-0 sm:max-w-md">
		<Sheet.Header class="gap-1 border-b">
			<Sheet.Title>{text.title}</Sheet.Title>
			<Sheet.Description>{text.description}</Sheet.Description>
		</Sheet.Header>
		<Channel isActive={isOpen} />
		<div class="border-t px-4 py-2 text-center">
			<a href="/assistant/" class="text-muted-foreground hover:text-foreground text-xs">
				{text.openFullConversation}
			</a>
		</div>
	</Sheet.Content>
</Sheet.Root>
