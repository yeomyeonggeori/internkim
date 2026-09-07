<script lang="ts">
	import PersonChipList from '$lib/components/person-chip-list.svelte';
	import type { Task } from './task-types';

	type Props = {
		task: Task;
		memberEmail: (memberID: string) => string;
	};

	let { task, memberEmail }: Props = $props();
</script>

<PersonChipList names={task.participantNames} memberIDs={task.participantIDs} {memberEmail} class="max-w-56" />
