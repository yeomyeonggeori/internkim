<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { fetchTurnInput } from './llm-calls';
	import RawDocument from './raw-document.svelte';
	import type { TasksText } from './text';
	import TimelineEvent from './timeline-event.svelte';

	let { value, isOpen, taskEventID, elapsed, text }: { value: string; isOpen: boolean; taskEventID: string; elapsed: string; text: TasksText } = $props();

	let turnInputRequest: Promise<string> | undefined;

	function turnInputOnce(): Promise<string> {
		turnInputRequest ??= fetchTurnInput(taskEventID).then((turnInput) => JSON.stringify(turnInput, undefined, 2));
		return turnInputRequest;
	}

	function loadErrorOf(error: unknown): string {
		return error instanceof Error && error.message ? error.message : text.turnInputLoadError;
	}
</script>

<TimelineEvent {value} lane="other" laneLabel={text.laneOther} title="task.turn_input" {elapsed} {isOpen}>
	{#await turnInputOnce()}
		<Skeleton class="h-24 w-full" />
	{:then turnInputDocument}
		<RawDocument document={turnInputDocument} />
	{:catch error}
		<p class="text-xs text-destructive">{loadErrorOf(error)}</p>
	{/await}
</TimelineEvent>
