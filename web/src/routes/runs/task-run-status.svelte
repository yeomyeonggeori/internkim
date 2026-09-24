<script lang="ts">
	import { cn } from '$lib/utils.js';
	import { taskStatusIcon, taskStatusToneClass } from './runs-view';

	let { status, label, class: className }: { status: string; label: string; class?: string } = $props();

	const StatusIcon = $derived(taskStatusIcon(status));
</script>

<span class={cn('inline-flex items-center gap-1.5 whitespace-nowrap text-sm', taskStatusToneClass(status), className)}>
	<StatusIcon class={cn('size-3.5 shrink-0', status === 'running' && 'animate-spin [animation-duration:2s]')} aria-hidden="true" />
	{label}
</span>
