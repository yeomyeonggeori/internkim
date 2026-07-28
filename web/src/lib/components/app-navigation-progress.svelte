<script lang="ts">
	let { isNavigating }: { isNavigating: boolean } = $props();
</script>

{#if isNavigating}
	<div class="internkim-navigation-progress" role="progressbar" aria-label="loading" aria-busy="true"></div>
{/if}
