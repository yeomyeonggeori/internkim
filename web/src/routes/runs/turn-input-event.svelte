<script lang="ts">
	import type { TasksText } from './text';
	import TimelineEvent from './timeline-event.svelte';
	import TurnInputDetail from './turn-input-detail.svelte';

	let { value, isOpen, taskEventID, elapsed, text }: { value: string; isOpen: boolean; taskEventID: string; elapsed: string; text: TasksText } = $props();
</script>

<TimelineEvent {value} lane="other" laneLabel={text.laneOther} title="task.turn_input" {elapsed} {isOpen}>
	<TurnInputDetail {taskEventID} {text} />
</TimelineEvent>
