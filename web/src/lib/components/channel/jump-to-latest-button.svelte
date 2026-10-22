<script lang="ts">
	import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
	import { Button } from '$lib/components/ui/button/index.js';

	let { isVisible, label, onclick }: { isVisible: boolean; label: string; onclick: () => void } = $props();
</script>

<Button
	variant="outline"
	size="icon"
	{onclick}
	aria-label={label}
	data-visible={isVisible}
	class="jump-to-latest frosted-surface pointer-events-auto size-9 rounded-full"
>
	<ArrowDownIcon />
</Button>

<style>
	:global(.jump-to-latest) {
		transition:
			opacity 180ms var(--ease-out-strong),
			transform 180ms var(--ease-out-strong),
			visibility 0s;
	}
	:global(.jump-to-latest[data-visible='false']) {
		visibility: hidden;
		opacity: 0;
		transform: translateY(8px) scale(0.96);
		transition:
			opacity 180ms var(--ease-out-strong),
			transform 180ms var(--ease-out-strong),
			visibility 0s linear 180ms;
	}
	@media (prefers-reduced-motion: reduce) {
		:global(.jump-to-latest[data-visible='false']) {
			transform: none;
		}
	}
</style>
