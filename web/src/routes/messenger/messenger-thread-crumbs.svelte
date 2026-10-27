<script lang="ts">
	import * as Breadcrumb from '$lib/components/ui/breadcrumb/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import HashIcon from '@lucide/svelte/icons/hash';

	let { conversationName, isGroup, back }: { conversationName: string; isGroup: boolean; back: () => void } = $props();

	const text = createPageText(channelText);
</script>

<Button variant="ghost" size="icon-sm" class="shrink-0" aria-label={text.closeThread} onclick={back}>
	<ChevronLeftIcon />
</Button>
<Breadcrumb.Root aria-label={text.threadTitle} class="min-w-0">
	<Breadcrumb.List class="flex-nowrap">
		<Breadcrumb.Item class="min-w-0">
			<button type="button" class="hover:text-foreground flex min-w-0 cursor-pointer items-center gap-1 transition-colors" onclick={back}>
				{#if isGroup}<HashIcon class="size-4 shrink-0" />{/if}
				<span class="truncate">{conversationName}</span>
			</button>
		</Breadcrumb.Item>
		<Breadcrumb.Separator />
		<Breadcrumb.Item class="shrink-0">
			<Breadcrumb.Page class="font-semibold">{text.threadTitle}</Breadcrumb.Page>
		</Breadcrumb.Item>
	</Breadcrumb.List>
</Breadcrumb.Root>
