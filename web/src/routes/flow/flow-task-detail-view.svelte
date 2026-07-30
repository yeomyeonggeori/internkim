<script lang="ts">
	import { personProfileImagePath } from '$lib/person-profile-image';
	import FlowTaskEditorSummary from './flow-task-editor-summary.svelte';
	import FlowTaskPersonChip from './flow-task-person-chip.svelte';
	import type { FlowTaskEditorText } from './flow-task-editor-types';
	import type { FlowTask } from './flow-types';

	type Props = {
		task: FlowTask;
		text: FlowTaskEditorText;
		statusLabel: (status: string) => string;
		businessColor: (business: string) => string;
		taskTypeColor: (type: string) => string;
		memberEmail: (memberID: string) => string;
	};

	let { task, text, statusLabel, businessColor, taskTypeColor, memberEmail }: Props = $props();

	const dateRange = $derived([task.startDate, task.endDate].filter(Boolean).join(' — '));
</script>

<div class="space-y-4">
	<FlowTaskEditorSummary taskDraft={task} {text} {statusLabel} {businessColor} {taskTypeColor} />

	<div>
		<p class="text-xs text-muted-foreground">{text.content}</p>
		<p class="mt-1 text-base font-medium leading-6">{task.content}</p>
	</div>

	{#if task.goal}
		<div>
			<p class="text-xs text-muted-foreground">{text.goal}</p>
			<p class="mt-1 text-sm leading-6">{task.goal}</p>
		</div>
	{/if}

	<div class="grid gap-4 sm:grid-cols-2">
		<div>
			<p class="text-xs text-muted-foreground">{text.owner}</p>
			<div class="mt-1 flex flex-wrap gap-1">
				<FlowTaskPersonChip name={task.ownerName} email={memberEmail(task.ownerID)} seed={task.ownerID} image={personProfileImagePath(task.ownerID)} />
			</div>
		</div>
		{#if task.participantNames.length}
			<div>
				<p class="text-xs text-muted-foreground">{text.participants}</p>
				<div class="mt-1 flex flex-wrap gap-1">
					{#each task.participantNames as participantName, index (participantName)}
						<FlowTaskPersonChip
								name={participantName}
								email={memberEmail(task.participantIDs[index] ?? '')}
								seed={task.participantIDs[index] ?? participantName}
								image={personProfileImagePath(task.participantIDs[index])}
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

	{#if task.requestReason}
		<div>
			<p class="text-xs text-muted-foreground">{text.requestReason}</p>
			<p class="mt-1 text-sm leading-6">{task.requestReason}</p>
		</div>
	{/if}
</div>
