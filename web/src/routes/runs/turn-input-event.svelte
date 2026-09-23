<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import { fetchTurnInput } from './llm-calls';
	import { formatTaskTimestamp } from './runs-view';
	import type { TasksText } from './text';

	let { taskEventID, createdAt, text }: { taskEventID: string; createdAt?: string; text: TasksText } = $props();

	let isOpen = $state(false);
	let turnInputDocument = $state('');
	let loadError = $state('');
	let isLoading = $state(false);

	async function loadTurnInput() {
		if (turnInputDocument || isLoading) return;
		isLoading = true;
		loadError = '';
		try {
			turnInputDocument = JSON.stringify(await fetchTurnInput(taskEventID), undefined, 2);
		} catch (error) {
			loadError = error instanceof Error && error.message ? error.message : text.turnInputLoadError;
		} finally {
			isLoading = false;
		}
	}

	function handleOpenChange(isNowOpen: boolean) {
		isOpen = isNowOpen;
		if (isNowOpen) void loadTurnInput();
	}
</script>

<article class="overflow-hidden rounded-lg border">
	<Collapsible.Root open={isOpen} onOpenChange={handleOpenChange}>
		<Collapsible.Trigger class="group flex w-full items-center gap-2 px-3 py-3 text-left hover:bg-muted/50">
			<ChevronRightIcon class="size-3.5 shrink-0 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
			<Badge variant="outline">{text.turnInput}</Badge>
			<span class="min-w-0 flex-1 truncate text-xs text-muted-foreground">{text.turnInputDescription}</span>
			<span class="shrink-0 text-xs text-muted-foreground">{formatTaskTimestamp(createdAt)}</span>
		</Collapsible.Trigger>
		<Collapsible.Content class="flex flex-col gap-2 border-t px-3 py-3">
			{#if isLoading}
				<Skeleton class="h-24 w-full" />
			{:else if loadError}
				<p class="text-xs text-destructive">{loadError}</p>
			{:else if turnInputDocument}
				<CopyButton text={turnInputDocument} variant="ghost" size="xs" class="self-start">
					<span>{text.copyBytes}</span>
				</CopyButton>
				<pre class="max-h-[32rem] overflow-auto rounded-lg border bg-muted/30 px-3 py-2 text-xs leading-relaxed whitespace-pre-wrap">{turnInputDocument}</pre>
			{/if}
		</Collapsible.Content>
	</Collapsible.Root>
</article>
