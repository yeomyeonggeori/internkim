<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { cn } from '$lib/utils.js';
	import { isTaskStatusNeedingAttention, taskStatusIcon, taskStatusIconClass } from './runs-view';

	let { status, label, class: className }: { status: string; label: string; class?: string } = $props();

	const StatusIcon = $derived(taskStatusIcon(status));
</script>

{#if isTaskStatusNeedingAttention(status)}
	<Badge variant="destructive" class={className}>
		<StatusIcon />
		{label}
	</Badge>
{:else}
	<span class={cn('inline-flex items-center gap-1.5 text-sm whitespace-nowrap', className)}>
		<StatusIcon class={cn('size-3.5 shrink-0', taskStatusIconClass(status))} aria-hidden="true" />
		{label}
	</span>
{/if}
