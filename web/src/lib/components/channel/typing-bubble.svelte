<script lang="ts">
	import * as Bubble from '$lib/components/ui/bubble/index.js';
	import TypingDots from './typing-dots.svelte';

	let { label }: { label: string } = $props();
</script>

<Bubble.Root variant="muted">
	<Bubble.Content role="status" aria-label={label} class="py-2.5">
		<TypingDots />
	</Bubble.Content>
</Bubble.Root>
