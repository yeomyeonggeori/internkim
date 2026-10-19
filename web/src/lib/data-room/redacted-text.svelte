<script lang="ts">
	import { redactedSegments } from './activity';

	let { text, seed, label }: { text: string; seed: string; label: string } = $props();

	function pathOf(outline: [number, number][]): string {
		return `M${outline.map(([x, y]) => `${x} ${y}`).join(' L')} Z`;
	}
</script>

{#each redactedSegments(seed, text) as segment, index (index)}
	{#if 'text' in segment}{segment.text}{:else}<svg
			class="text-foreground mx-[0.16em] inline-block h-[1.15em] overflow-visible align-[-0.2em]"
			style:width={`${segment.redactedLength * 0.78 * segment.widthScale}em`}
			style:transform={`rotate(${segment.tiltDegrees}deg)`}
			viewBox="0 0 100 100"
			preserveAspectRatio="none"
			role="img"
			aria-label={label}
			><path
				d={pathOf(segment.outline)}
				fill="currentColor"
				stroke="currentColor"
				stroke-width="3"
				stroke-linejoin="round"
				vector-effect="non-scaling-stroke"
			/></svg
		>{/if}
{/each}
