<script lang="ts">
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<div
	data-slot="sender-avatar"
	class="bg-muted mt-[5px] flex w-fit min-w-8 shrink-0 items-center justify-center self-start overflow-hidden rounded-full group-has-data-[slot=message-header]/message:mt-[23px]"
>
	{@render children()}
</div>
