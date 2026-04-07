<script lang="ts">
	import type { ChatBubbleProps } from './types';
	import { cn } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		variant,
		children,
		class: className,
		...rest
	}: ChatBubbleProps = $props();
</script>

<div
	{...rest}
	bind:this={ref}
	class={cn(
		"group/chat-bubble flex max-w-[75%] flex-row place-items-end gap-1.5 data-[variant='received']:ml-2 data-[variant='sent']:mr-2 data-[variant='sent']:place-self-end",
		className
	)}
	data-variant={variant}
>
	{@render children?.()}
</div>
