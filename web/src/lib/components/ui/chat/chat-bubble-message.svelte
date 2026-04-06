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

<div
	{...rest}
	bind:this={ref}
	class={cn(
		"chat-bubble-msg order-2 text-sm group-data-[variant='sent']/chat-bubble:order-1",
		className
	)}
>
	{#if typing}
		<div class="flex items-center" style="height: calc(1lh);">
			<LoadingDots />
		</div>
	{:else}
		{@render children?.()}
	{/if}
</div>
