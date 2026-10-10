<script lang="ts">
	import * as Tabs from '$lib/components/ui/tabs';
	import { Spinner } from '$lib/components/ui/spinner';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { boundedGrid, columnLabel, extensionOf, maxPreviewRows, type SheetGrid } from './kind';
	import { previewText } from './text';

	type Sheet = { name: string; grid: SheetGrid };

	let {
		bytes,
		fileName,
		onFailure
	}: { bytes: ArrayBuffer; fileName: string; onFailure: (error: unknown) => void } = $props();
	const delimitedExtensions = ['csv', 'tsv'];

	const text = createPageText(previewText);
	let sheets = $state.raw<Sheet[] | null>(null);
	let selectedSheet = $state('');

	$effect(() => {
		void readWorkbook(bytes, delimitedExtensions.includes(extensionOf(fileName)));
	});

	async function readWorkbook(source: ArrayBuffer, isDelimitedText: boolean) {
		try {
			const spreadsheet = await import('xlsx');
			const options = { dense: true, cellDates: true, sheetRows: maxPreviewRows + 1 } as const;
			const workbook = isDelimitedText
				? spreadsheet.read(new TextDecoder().decode(source), { ...options, type: 'string' })
				: spreadsheet.read(source, { ...options, type: 'array' });
			sheets = workbook.SheetNames.map((name) => ({
				name,
				grid: boundedGrid(
					spreadsheet.utils.sheet_to_json<unknown[]>(workbook.Sheets[name], {
						header: 1,
						raw: false,
						defval: '',
						blankrows: true
					})
				)
			}));
			selectedSheet = sheets[0]?.name ?? '';
		} catch (error) {
			onFailure(error);
		}
	}
</script>

{#snippet sheetTable(grid: SheetGrid)}
	{#if grid.rows.length === 0}
		<p class="text-muted-foreground p-6 text-sm">{text.emptySheet}</p>
	{:else}
		<div class="min-h-0 flex-1 overflow-auto">
			<table class="border-separate border-spacing-0 font-mono text-xs">
				<thead>
					<tr>
						<th class="bg-muted text-muted-foreground sticky top-0 left-0 z-20 min-w-10 border-r border-b px-2 py-1 font-normal"
							><span class="sr-only">{text.rowNumber}</span></th
						>
						{#each { length: grid.columnCount } as _, columnIndex (columnIndex)}
							<th class="bg-muted text-muted-foreground sticky top-0 z-10 min-w-20 border-r border-b px-2 py-1 font-normal"
								>{columnLabel(columnIndex)}</th
							>
						{/each}
					</tr>
				</thead>
				<tbody>
					{#each grid.rows as row, rowIndex (rowIndex)}
						<tr>
							<th class="bg-muted text-muted-foreground sticky left-0 z-10 border-r border-b px-2 py-1 text-right font-normal tabular-nums"
								>{rowIndex + 1}</th
							>
							{#each { length: grid.columnCount } as _, columnIndex (columnIndex)}
								<td class="bg-background max-w-72 truncate border-r border-b px-2 py-1 whitespace-nowrap"
									title={row[columnIndex]}>{row[columnIndex] ?? ''}</td
								>
							{/each}
						</tr>
					{/each}
				</tbody>
			</table>
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
