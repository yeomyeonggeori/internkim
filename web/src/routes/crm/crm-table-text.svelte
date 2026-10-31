<script lang="ts">
	import { cn } from '$lib/utils';

	let { text, href, class: className }: { text: string; href?: string; class?: string } = $props();

	function keepRowClosed(event: Event): void {
		event.stopPropagation();
	}
</script>

{#if href}
	<a class={cn('block max-w-48 truncate hover:underline', className)} {href} title={text} onclick={keepRowClosed} onkeydown={keepRowClosed}>{text}</a>
{:else}
	<p class={cn('max-w-48 truncate', className)} title={text}>{text}</p>
{/if}
