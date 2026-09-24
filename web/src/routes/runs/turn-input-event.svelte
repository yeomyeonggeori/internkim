<script lang="ts">
	import type { TasksText } from './text';
	import TimelineEvent from './timeline-event.svelte';
	import TurnInputDetail from './turn-input-detail.svelte';

	let { value, isOpen, taskEventID, createdAt, text }: { value: string; isOpen: boolean; taskEventID: string; createdAt?: string; text: TasksText } = $props();
</script>

<TimelineEvent {value} title="task.turn_input" lane="other" {createdAt} {isOpen}>
	<TurnInputDetail {taskEventID} {text} />
</TimelineEvent>
