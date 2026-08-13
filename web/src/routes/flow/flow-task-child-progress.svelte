<script lang="ts">
	import type { FlowTaskChildProgress as ChildProgress } from './flow-task-relationships';

	type Props = {
		progress: ChildProgress;
		label: string;
	};

	let { progress, label }: Props = $props();
</script>

<div class="space-y-1.5 border-t border-border/60 pt-1.5" data-flow-task-child-progress>
	<div class="flex items-center justify-between gap-2 text-[11px] leading-4 text-muted-foreground">
		<span>{label}</span>
		<span class="shrink-0 tabular-nums">{progress.percent}%</span>
	</div>
	<div
		class="grid h-1.5 gap-0.5"
		style={`grid-template-columns: repeat(${progress.total}, minmax(0, 1fr))`}
		role="progressbar"
		aria-label={label}
		aria-valuemin="0"
		aria-valuemax={progress.total}
		aria-valuenow={progress.completed}
	>
		{#each Array.from({ length: progress.total }) as _, index (index)}
			<span
				class={index < progress.completed
					? 'min-w-0 rounded-full bg-violet-500/80'
					: 'min-w-0 rounded-full bg-violet-500/15'}
				data-flow-task-child-progress-segment
			></span>
		{/each}
	</div>
</div>
