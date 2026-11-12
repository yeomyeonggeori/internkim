<script lang="ts">
	import type { Snippet } from 'svelte';
	import * as Bubble from '$lib/components/ui/bubble/index.js';

	let { text, children }: { text: string; children?: Snippet } = $props();
</script>

<Bubble.Root variant="destructive" class="max-w-[min(80%,32rem)]">
	<Bubble.Content class="break-words whitespace-pre-wrap">{text}</Bubble.Content>
	{@render children?.()}
</Bubble.Root>
