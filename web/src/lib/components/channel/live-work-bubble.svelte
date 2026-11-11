<script lang="ts">
	import * as Bubble from '$lib/components/ui/bubble/index.js';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
	import CheckIcon from '@lucide/svelte/icons/check';
	import XIcon from '@lucide/svelte/icons/x';
	import type { ShownStep } from '$lib/messenger/agent-work';
	import TypingDots from './typing-dots.svelte';

	let { steps, label }: { steps: ShownStep[]; label: string } = $props();
</script>

<Bubble.Root variant="muted" class="max-w-[min(80%,32rem)]">
	<Bubble.Content role="status" aria-label={label} class="grid gap-1.5 py-2.5">
		{#if steps.length > 0}
			<div class="grid gap-1">
				{#each steps as step, index (index)}
					<div
						data-status={step.status}
						class="text-muted-foreground data-[status=in_progress]:text-foreground flex min-w-0 items-center gap-2 font-mono text-xs"
					>
						{#if step.status === 'in_progress'}
							<LoaderCircleIcon class="size-3.5 shrink-0 animate-spin motion-reduce:animate-none" />
						{:else if step.status === 'completed'}
							<CheckIcon class="text-brand size-3.5 shrink-0" />
						{:else}
							<XIcon class="text-destructive size-3.5 shrink-0" />
						{/if}
						<span class="truncate">{step.title}</span>
					</div>
				{/each}
			</div>
		{/if}
		<TypingDots />
	</Bubble.Content>
</Bubble.Root>
