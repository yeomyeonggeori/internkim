<script lang="ts">
	import { personProfileImagePath } from '$lib/person-profile-image';
	import TaskEditorSummary from './task-editor-summary.svelte';
	import TaskPersonChip from './task-person-chip.svelte';
	import { hasTaskRequestProvenance } from './task-options';
	import type { TaskEditorText } from './task-editor-types';
	import type { Task } from './task-types';

	type Props = {
		task: Task;
		text: TaskEditorText;
		statusLabel: (status: string) => string;
		businessColor: (business: string | null) => string;
		taskTypeColor: (type: string | null) => string;
		memberEmail: (memberID: string) => string;
	};

	let { task, text, statusLabel, businessColor, taskTypeColor, memberEmail }: Props = $props();

	const dateRange = $derived([task.startDate, task.endDate].filter(Boolean).join(' — '));
</script>

<div class="space-y-4">
	<TaskEditorSummary taskDraft={task} {text} {statusLabel} {businessColor} {taskTypeColor} />

	<div>
		<p class="text-xs text-muted-foreground">{text.content}</p>
		<p class="mt-1 text-base font-medium leading-6">{task.content}</p>
	</div>

	<div class="grid gap-4 sm:grid-cols-2">
		{#if hasTaskRequestProvenance(task)}
			<div>
				<p class="text-xs text-muted-foreground">{text.requester}</p>
				<div class="mt-1 text-sm">
					<TaskPersonChip
						name={task.requesterName || memberEmail(task.requesterID || '') || task.requesterID || text.requesterUnavailable}
						email={memberEmail(task.requesterID || '')}
						seed={task.requesterID || task.requesterName || text.requesterUnavailable}
						image={personProfileImagePath(task.requesterID || '')}
					/>
				</div>
			</div>
		{/if}
		{#if task.participantNames.length}
			<div>
				<p class="text-xs text-muted-foreground">{text.participants}</p>
				<div class="mt-1 flex flex-wrap gap-1">
					{#each task.participantIDs as participantID, index (participantID)}
						<TaskPersonChip
								name={task.participantNames[index] ?? participantID}
								email={memberEmail(participantID)}
								seed={participantID}
								image={personProfileImagePath(participantID)}
							/>
					{/each}
				</div>
			</div>
		{/if}
		{#if dateRange}
			<div>
				<p class="text-xs text-muted-foreground">{text.startDate} — {text.endDate}</p>
				<p class="mt-1 text-sm tabular-nums">{dateRange}</p>
			</div>
		{/if}
	</div>

</div>
