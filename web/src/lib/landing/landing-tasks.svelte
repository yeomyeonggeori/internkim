<script lang="ts">
	import { paletteColorAt } from '$lib/color-picker-palette';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import TaskBoard from '../../routes/task/task-board.svelte';
	import { taskText } from '../../routes/task/text';
	import { sampleTasks } from './landing-samples';

	const text = createPageText(taskText);
	const tasks = $derived(sampleTasks(currentLocale.value, new Date()));
	const definitionValues = $derived([...new Set(tasks.flatMap((task) => [task.business ?? '', task.type ?? '']))]);
	const statusLabels: Record<string, string> = $derived(text.status);

	function definitionColor(value: string | null): string {
		return paletteColorAt(definitionValues.indexOf(value ?? ''));
	}
</script>

<div class="overflow-hidden px-4 py-4 md:px-8 [--task-board-height:22rem]">
	<TaskBoard
		{tasks}
		allTasks={tasks}
		boardText={text.task.board}
		etcLabel={text.task.etcLabel}
		statusLabel={(status) => statusLabels[status] ?? status}
		openTask={() => {}}
		createTask={() => {}}
		moveTask={() => {}}
		pendingTaskIDs={[]}
		canUpdateTask={() => false}
		businessColor={definitionColor}
		taskTypeColor={definitionColor}
		childProgressLabel={text.task.relationships.progressLabel}
		memberEmail={() => ''}
		participantScope="other"
		fitsViewport={false}
	/>
</div>
