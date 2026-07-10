<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { personProfileImagePath } from '$lib/person-profile-image';
	import type { FlowTask } from './flow-types';

	type Props = {
		task: FlowTask;
	};

	let { task }: Props = $props();
</script>

<div class="flex items-center gap-2 font-medium">
	<PersonAvatar name={task.ownerName} seed={task.ownerID || task.ownerName} image={personProfileImagePath(task.ownerID)} class="size-6" />
	{task.ownerName}
</div>
