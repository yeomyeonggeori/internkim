<script>
	import EffectErrorBoundary from '../../../../src/lib/components/effect-error-boundary.svelte';
	import FailingEffect from './failing-effect.svelte';
	import Gate from './gate.svelte';

	let path = $state('/failing');
</script>

<EffectErrorBoundary region="page" resetKey={path}>
	{#if path === '/failing'}
		<FailingEffect />
	{:else}
		<p data-page>next-page</p>
	{/if}
	{#snippet failed()}
		<p data-failure>page-failed</p>
	{/snippet}
</EffectErrorBoundary>
<button data-leave onclick={() => (path = '/next')}>leave</button>
<Gate />
