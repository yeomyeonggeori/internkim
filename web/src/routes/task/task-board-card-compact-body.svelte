<script lang="ts">
	import PersonAvatarStack from '$lib/components/person-avatar-stack.svelte';
	import TaskCompactMetadata from './task-compact-metadata.svelte';
	import type { Task } from './task-types';

	type Participant = { name: string; seed: string; email?: string };

	type Props = {
		task: Task;
		participants: Participant[];
		participantNameList: string;
		isOverduePlan: boolean;
		businessColor: string;
	};

	let { task, participants, participantNameList, isOverduePlan, businessColor }: Props = $props();
</script>

<div class="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-x-3 gap-y-1 px-3 py-2.5" data-task-board-card-compact>
	<div class="line-clamp-2 break-words break-keep text-sm font-medium leading-5 text-card-foreground">{task.content}</div>
	<span class="justify-self-end pt-0.5 text-xs font-medium leading-4 text-muted-foreground tabular-nums">{task.size}</span>
	<TaskCompactMetadata {task} {businessColor} {isOverduePlan} />
	{#if participants.length > 0}
		<PersonAvatarStack
			people={participants}
			max={3}
			class="col-start-2 -space-x-1 justify-self-end"
			avatarClass="size-5 ring-1 ring-card"
			label={participantNameList}
		/>
	{/if}
</div>
