<script lang="ts">
	import type { Snippet } from 'svelte';
	import * as Bubble from '$lib/components/ui/bubble/index.js';
	import * as Collapsible from '$lib/components/ui/collapsible/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { FailureCause } from '$lib/messenger/failure-cause';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';

	let {
		text,
		isRawReason = false,
		cause,
		children
	}: { text: string; isRawReason?: boolean; cause?: FailureCause; children?: Snippet } = $props();

	const channel = createPageText(channelText);
	let isShowingReason = $state(false);

	const causeExplanation = $derived(
		cause === 'transient' ? channel.failureCauseTransient : cause === 'administrator' ? channel.failureCauseAdministrator : undefined
	);
</script>

<Bubble.Root variant="muted" class="max-w-[min(80%,32rem)]">
	<Bubble.Content class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-2">
		<CircleAlertIcon class="text-destructive mt-[0.1875rem] size-4" aria-hidden="true" />
		{#if isRawReason}
			<Collapsible.Root bind:open={isShowingReason} class="grid min-w-0 gap-1">
				<p class="font-medium">{channel.requestNotFinished}</p>
				{#if causeExplanation}
					<p class="text-muted-foreground">{causeExplanation}</p>
				{/if}
				<Collapsible.Trigger
					class="text-muted-foreground hover:text-foreground focus-visible:ring-ring/50 inline-flex w-fit items-center gap-0.5 rounded-sm text-xs outline-none focus-visible:ring-3"
				>
					{channel.technicalDetails}
					<ChevronDownIcon class={['size-3.5 transition-transform motion-reduce:transition-none', isShowingReason && 'rotate-180']} aria-hidden="true" />
				</Collapsible.Trigger>
				<Collapsible.Content>
					<code class="bg-background text-muted-foreground block rounded-sm px-2 py-1.5 font-mono text-xs wrap-anywhere whitespace-pre-wrap">{text}</code>
				</Collapsible.Content>
			</Collapsible.Root>
		{:else}
			<p class="break-words whitespace-pre-wrap">{text}</p>
		{/if}
	</Bubble.Content>
	{@render children?.()}
</Bubble.Root>
