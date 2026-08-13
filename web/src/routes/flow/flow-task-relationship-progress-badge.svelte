<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';

	type Props = {
		completed: number;
		total: number;
		percent: number;
		label: string;
	};

	let { completed, total, percent, label }: Props = $props();
	const circumference = 2 * Math.PI * 6;
	let progressOffset = $derived(circumference * (1 - percent / 100));
</script>

<Badge
	variant="outline"
	class="h-6 gap-1.5 rounded-full px-1.5 font-semibold tabular-nums text-muted-foreground shadow-none"
	aria-label={`${label}, ${percent}%`}
>
	<svg
		viewBox="0 0 16 16"
		class="size-4 -rotate-90 text-violet-600 dark:text-violet-400"
		aria-hidden="true"
		data-flow-relationship-progress-ring
	>
		<circle cx="8" cy="8" r="6" fill="none" stroke="var(--muted)" stroke-width="3" />
		<circle
			cx="8"
			cy="8"
			r="6"
			fill="none"
			stroke="currentColor"
			stroke-width="3"
			stroke-linecap="round"
			stroke-dasharray={circumference}
			stroke-dashoffset={progressOffset}
		/>
	</svg>
	<span>{completed} / {total}</span>
</Badge>
