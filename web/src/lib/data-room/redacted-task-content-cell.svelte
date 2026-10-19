<script lang="ts">
	import type { Task } from '../../routes/task/task-types';
	import { activityListContext } from './activity-list-context';
	import RedactedText from './redacted-text.svelte';

	let { task }: { task: Task } = $props();
	const context = $derived(activityListContext());
</script>

<span class="block max-w-[28rem] overflow-hidden whitespace-nowrap"><RedactedText text={task.content} seed={task.id} label={context.hiddenLabel} /></span>
