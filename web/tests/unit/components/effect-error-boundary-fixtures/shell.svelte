<script>
	import EffectErrorBoundary from '../../../../src/lib/components/effect-error-boundary.svelte';
	import FailingEffect from './failing-effect.svelte';
	import Gate from './gate.svelte';

	let { isContained } = $props();
</script>

{#if isContained}
	<EffectErrorBoundary region="app rail">
		<FailingEffect />
	</EffectErrorBoundary>
{:else}
	<FailingEffect />
{/if}
<Gate />
