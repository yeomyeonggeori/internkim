<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { columnLabel, type SheetGrid } from './kind';
	import { previewText } from './text';

	let { grid }: { grid: SheetGrid } = $props();

	const text = createPageText(previewText);
</script>

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
