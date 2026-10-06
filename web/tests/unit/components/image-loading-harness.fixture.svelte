<script lang="ts">
	import LoadingImage from '$lib/components/loading-image.svelte';
	let source = $state('/cached.svg');
	let visible = $state(true);

	export function replace(nextSource: string) { source = nextSource; }
	export function remove() { visible = false; }
</script>

{#if visible}
	<LoadingImage src={source} alt="Sample picture" width={240} height={320} loading="eager" />
{/if}
