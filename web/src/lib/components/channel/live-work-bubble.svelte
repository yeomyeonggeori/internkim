<script lang="ts">
	import * as Bubble from '$lib/components/ui/bubble/index.js';
	import CircleDashedIcon from '@lucide/svelte/icons/circle-dashed';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
	import CheckIcon from '@lucide/svelte/icons/check';
	import XIcon from '@lucide/svelte/icons/x';
	import type { ToolStep } from '$lib/messenger/agent-work';

	let { steps, label }: { steps: ToolStep[]; label: string } = $props();
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
						{#if step.status === 'pending'}
							<CircleDashedIcon class="size-3.5 shrink-0" />
						{:else if step.status === 'in_progress'}
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
		<div class="flex h-4 items-center gap-1" aria-hidden="true">
			{#each [0, 160, 320] as delayMilliseconds (delayMilliseconds)}
				<span class="typing-dot bg-muted-foreground size-1.5 rounded-full" style:animation-delay="{delayMilliseconds}ms"></span>
			{/each}
		</div>
	</Bubble.Content>
</Bubble.Root>

<style>
	@keyframes typing-dot {
		0%,
		60%,
		100% {
			opacity: 0.35;
			transform: translateY(0);
		}
		30% {
			opacity: 1;
			transform: translateY(-2px);
		}
	}

	.typing-dot {
		animation: typing-dot 1.2s ease-in-out infinite;
	}

	@media (prefers-reduced-motion: reduce) {
		.typing-dot {
			animation: none;
			opacity: 0.6;
		}
	}
</style>
