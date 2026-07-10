<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { personProfileImagePath } from '$lib/person-profile-image';
	import type { FlowTask } from './flow-types';

	type Props = {
		task: FlowTask;
	};

	let { task }: Props = $props();
</script>

<div class="flex max-w-56 flex-wrap gap-1">
	{#each task.participantNames as name, index}
		<Badge variant="outline" class="gap-1.5 pl-1">
			<PersonAvatar name={name} seed={task.participantIDs[index] ?? name} image={personProfileImagePath(task.participantIDs[index])} class="size-4" />
			{name}
		</Badge>
	{/each}
</div>
