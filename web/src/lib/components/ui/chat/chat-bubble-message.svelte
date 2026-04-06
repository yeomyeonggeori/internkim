<script lang="ts">
	import LoadingDots from './loading-dots.svelte';
	import { cn } from '$lib/utils.js';
	import type { ChatBubbleMessageProps } from './types';

	let {
		ref = $bindable(null),
		typing = false,
		class: className,
		children,
		...rest
	}: ChatBubbleMessageProps = $props();
</script>

{#if typing}
	<div class="typing-wrapper w-fit">
		<div
			class={cn(
				"chat-bubble-msg !p-3 order-2 text-sm group-data-[variant='sent']/chat-bubble:order-1",
				className
			)}
		>
			<div class="flex items-center justify-center -translate-y-[1px]">
				<LoadingDots size={6} />
			</div>
		</div>
		<span class="typing-dot typing-dot-lg"></span>
		<span class="typing-dot typing-dot-sm"></span>
	</div>
{:else}
	<div
		{...rest}
		bind:this={ref}
		class={cn(
			"chat-bubble-msg order-2 text-sm group-data-[variant='sent']/chat-bubble:order-1",
			className
		)}
	>
		{@render children?.()}
	</div>
{/if}
