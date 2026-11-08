<script lang="ts">
	import * as Tabs from '$lib/components/ui/tabs';
	import { Spinner } from '$lib/components/ui/spinner';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { maxPreviewRows, type SheetGrid } from './kind';
	import SheetGridTable from './sheet-grid-table.svelte';
	import { readSheets, type Sheet } from './spreadsheet';
	import { previewText } from './text';

	let {
		bytes,
		fileName,
		onFailure
	}: { bytes: ArrayBuffer; fileName: string; onFailure: (error: unknown) => void } = $props();

	const text = createPageText(previewText);
	let sheets = $state.raw<Sheet[] | null>(null);
	let selectedSheet = $state('');

	$effect(() => {
		readSheets(bytes, fileName, maxPreviewRows).then((read) => {
			sheets = read;
			selectedSheet = read[0]?.name ?? '';
		}, onFailure);
	});
</script>

{#snippet sheetTable(grid: SheetGrid)}
	{#if grid.rows.length === 0}
		<p class="text-muted-foreground p-6 text-sm">{text.emptySheet}</p>
	{:else}
		<div class="min-h-0 flex-1 overflow-auto">
			<SheetGridTable {grid} />
		</div>
		{#if grid.isTruncated}<p class="text-muted-foreground border-t px-3 py-1.5 text-xs">{text.truncated}</p>{/if}
	{/if}
{/snippet}

{#if !sheets}
	<div class="flex h-full items-center justify-center"><Spinner /></div>
{:else if sheets.length === 1}
	<div class="flex h-full min-h-0 flex-col">{@render sheetTable(sheets[0].grid)}</div>
{:else}
	<Tabs.Root bind:value={selectedSheet} class="flex h-full min-h-0 flex-col gap-0">
		{#each sheets as sheet (sheet.name)}
			<Tabs.Content value={sheet.name} class="flex min-h-0 flex-1 flex-col">{@render sheetTable(sheet.grid)}</Tabs.Content>
		{/each}
		<div class="overflow-x-auto border-t p-1.5">
			<Tabs.List>
				{#each sheets as sheet (sheet.name)}<Tabs.Trigger value={sheet.name}>{sheet.name}</Tabs.Trigger>{/each}
			</Tabs.List>
		</div>
	</Tabs.Root>
{/if}
