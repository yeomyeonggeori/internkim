<script lang="ts">
	import type { Task } from '../../routes/task/task-types';
	import ActivityAvatar from './activity-avatar.svelte';
	import { activityListContext } from './activity-list-context';
	import RedactedText from './redacted-text.svelte';
	import { redactedPersonName } from '$lib/person-name';
	import { currentPersonNameLocale } from '$lib/person-name.svelte';

	let { task }: { task: Task } = $props();
	const context = $derived(activityListContext());
</script>

<span class="inline-flex max-w-56 min-w-0 items-center gap-1.5 text-sm">
	<span class="flex -space-x-1">
		{#each task.participantIDs.slice(0, 3) as person (person)}
			<ActivityAvatar person={context.people[person]} seed={person} class="ring-background size-5 ring-1" />
		{/each}
	</span>
	<span class="truncate whitespace-nowrap"><RedactedText text={redactedPersonName(task.participantNames[0] ?? '', currentPersonNameLocale())} seed={task.participantIDs[0] ?? task.id} label={context.hiddenLabel} /></span>
</span>
