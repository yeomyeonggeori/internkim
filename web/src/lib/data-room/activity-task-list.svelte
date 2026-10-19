<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import TaskListView from '../../routes/task/task-list-view.svelte';
	import { taskText } from '../../routes/task/text';
	import { activityTasks, type CompanyActivity } from './activity';
	import { provideActivityListContext } from './activity-list-context';
	import { dataRoomCompanyText } from './company-text';
	import RedactedTaskContentCell from './redacted-task-content-cell.svelte';
	import RedactedTaskParticipantsCell from './redacted-task-participants-cell.svelte';

	let { activity, locale }: { activity: CompanyActivity; locale: 'ko' | 'en' } = $props();

	const neutralColor = '#a1a1aa';
	const pageText = createPageText(taskText);
	const text = $derived(dataRoomCompanyText[locale].activity);
	const statusText = $derived(taskText[locale].status);
	const tasks = $derived(activityTasks(activity));
	const total = $derived(Object.values(activity.board.counts).reduce((sum, count) => sum + count, 0));

	provideActivityListContext(() => ({ people: activity.people, hiddenLabel: text.hidden }));

	function neutral(): string {
		return neutralColor;
	}

	function statusLabel(status: string): string {
		return statusText[status as keyof typeof statusText] ?? status;
	}

	function cannotUpdate(): boolean {
		return false;
	}

	function noStatusOptions(): [] {
		return [];
	}

	function noEmail(): string {
		return '';
	}

	async function ignoreStatusChange(): Promise<void> {}
</script>

<div class="grid gap-2">
	<TaskListView
		{tasks}
		emptyLabel="—"
		showEmpty={false}
		text={pageText}
		statusOptionsForTask={noStatusOptions}
		pendingStatusTaskID=""
		{statusLabel}
		updateTaskStatus={ignoreStatusChange}
		canUpdateTask={cannotUpdate}
		focusedTaskID=""
		memberEmail={noEmail}
		businessColor={neutral}
		taskTypeColor={neutral}
		cells={{ content: RedactedTaskContentCell, participants: RedactedTaskParticipantsCell }}
	/>
	{#if total > tasks.length}
		<p class="text-muted-foreground text-xs tabular-nums">{text.moreTasks(total - tasks.length)}</p>
	{/if}
</div>
