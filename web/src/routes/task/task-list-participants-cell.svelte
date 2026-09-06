<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import type { Task } from './task-types';

	type Props = {
		task: Task;
		memberEmail: (memberID: string) => string;
	};

	let { task, memberEmail }: Props = $props();
</script>

<div class="flex max-w-56 flex-wrap gap-1">
	{#each task.participantNames as name, index}
		<Badge variant="outline" class="gap-1.5 pl-1">
			<PersonAvatar name={displayPersonName(name)} email={memberEmail(task.participantIDs[index] ?? '')} seed={task.participantIDs[index] ?? name} class="size-4" />
			{displayPersonName(name)}
		</Badge>
	{/each}
</div>
