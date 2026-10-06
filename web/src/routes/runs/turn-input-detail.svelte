<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { fetchTurnInput } from './llm-calls';
	import RawDocument from './raw-document.svelte';
	import type { TasksText } from './text';

	let { taskEventID, text }: { taskEventID: string; text: TasksText } = $props();

	let turnInputRequest: Promise<string> | undefined;

	function turnInputOnce(): Promise<string> {
		turnInputRequest ??= fetchTurnInput(taskEventID).then((turnInput) => JSON.stringify(turnInput, undefined, 2));
		return turnInputRequest;
	}

	function loadErrorOf(error: unknown): string {
		return error instanceof Error && error.message ? error.message : text.turnInputLoadError;
	}
</script>

{#await turnInputOnce()}
	<div role="status" aria-label="task.turn_input" aria-busy="true" class="rounded-lg border bg-muted/30 py-3 pr-10 pl-3" data-testid="turn-input-loading-skeleton"><div aria-hidden="true" class="grid gap-2">{#each [0, 1, 2, 3, 4, 5] as line (line)}<Skeleton class="h-3 w-4/5" />{/each}</div></div>
{:then turnInputDocument}
	<RawDocument document={turnInputDocument} />
{:catch error}
	<p class="text-xs text-destructive">{loadErrorOf(error)}</p>
{/await}
