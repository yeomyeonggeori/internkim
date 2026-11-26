<script lang="ts">
	import { untrack } from 'svelte';

	let { resetKey, reset }: { resetKey: string | undefined; reset: () => void } = $props();

	const failedAt = untrack(() => resetKey);

	$effect(() => {
		if (resetKey !== failedAt) reset();
	});
</script>
