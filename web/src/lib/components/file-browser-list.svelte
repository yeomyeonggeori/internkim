<script lang="ts" module>
	export type FileBrowserEntry = {
		id: string;
		name: string;
		fileName?: string;
		isDirectory?: boolean;
		entryCount?: number;
		tag?: string;
		cells: string[];
	};

	export type FileBrowserColumn = {
		label: string;
		width: 'narrow' | 'wide';
		isBadge?: boolean;
	};
</script>

<script lang="ts">
	import FolderIcon from '@lucide/svelte/icons/folder';
	import FolderOpenIcon from '@lucide/svelte/icons/folder-open';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Badge, type BadgeVariant } from '$lib/components/ui/badge';
	import CountBadge from '$lib/components/count-badge.svelte';
	import * as Empty from '$lib/components/ui/empty';
	import { cn } from '$lib/utils';
	import { fileVisual } from '$lib/files/view';

	let {
		entries,
		selectedID,
		isLoading = false,
		hasLoadError = false,
		title,
		nameLabel,
		columns,
		countBadgeVariant,
		emptyLabel,
		onSelect
	}: {
		entries: FileBrowserEntry[];
		selectedID?: string;
		isLoading?: boolean;
		hasLoadError?: boolean;
		title: string;
		nameLabel: string;
		columns: FileBrowserColumn[];
		countBadgeVariant?: BadgeVariant;
		emptyLabel: string;
		onSelect: (entry: FileBrowserEntry) => void;
	} = $props();

	const columnWidths = { narrow: '5rem', wide: '7rem' };
	const gridTemplate = $derived(['minmax(0,1fr)', ...columns.map((column) => columnWidths[column.width])].join(' '));
	const rowClass = 'grid grid-cols-(--file-columns) gap-3 max-sm:grid-cols-[minmax(0,1fr)_6rem]';

	function isHiddenOnPhone(index: number): boolean {
		return index < columns.length - 1;
	}
</script>

<section
	aria-label={title}
	aria-busy={isLoading}
	class="bg-card min-w-0 flex-1 overflow-hidden rounded-xl border shadow-sm"
	style:--file-columns={gridTemplate}
>
	<div class={cn('text-muted-foreground bg-muted/40 border-b px-4 py-2.5 text-xs font-medium', rowClass)}>
		<span>{nameLabel}</span>
		{#each columns as column, index (column.label)}
			<span class={cn('text-right', isHiddenOnPhone(index) && 'max-sm:hidden')}>{column.label}</span>
		{/each}
	</div>
	{#if isLoading && entries.length === 0}
		<div role="status" aria-label={title} data-testid="file-list-loading-skeleton">
			{#each [0, 1, 2, 3, 4, 5] as row (row)}
				<div aria-hidden="true" class={cn('items-center border-b px-4 py-2.5 last:border-b-0 max-sm:min-h-16', rowClass)}>
					<div class="flex min-h-5 min-w-0 items-center gap-2.5"><Skeleton class="size-4 shrink-0" /><Skeleton class="h-4 w-3/4 max-w-64" /></div>
					{#each columns as column, index (column.label)}
						<Skeleton class={cn('ml-auto h-3', column.width === 'narrow' ? 'w-12' : 'w-20', isHiddenOnPhone(index) && 'max-sm:hidden')} />
					{/each}
				</div>
			{/each}
		</div>
	{:else if entries.length === 0 && !hasLoadError}
		<Empty.Root>
			<Empty.Header>
				<Empty.Media variant="icon"><FolderOpenIcon /></Empty.Media>
				<Empty.Title>{emptyLabel}</Empty.Title>
			</Empty.Header>
		</Empty.Root>
	{:else}
		{#each entries as entry (entry.id)}
			{@const visual = fileVisual(entry.fileName ?? entry.name)}
			{@const FileTypeIcon = visual.icon}
			<button
				type="button"
				aria-pressed={selectedID === entry.id}
				class={cn(
					'hover:bg-accent/60 w-full items-center border-b px-4 py-2.5 text-left transition-colors last:border-b-0 max-sm:min-h-16',
					rowClass,
					selectedID === entry.id && 'bg-accent'
				)}
				onclick={() => onSelect(entry)}
			>
				<span class={cn('flex min-w-0 items-center gap-2.5', entry.entryCount === 0 && 'opacity-50')}>
					{#if entry.isDirectory}<FolderIcon
							class="size-4 shrink-0 fill-amber-200 text-amber-500"
						/>
					{:else}<FileTypeIcon class={cn('size-4 shrink-0', visual.colorClass)} />{/if}
					<span class="min-w-0 text-sm max-sm:line-clamp-2 max-sm:break-all sm:truncate">{entry.name}</span>
					{#if entry.tag}<Badge variant="outline" class="shrink-0 font-mono">{entry.tag}</Badge>{/if}
					{#if entry.entryCount}<CountBadge count={entry.entryCount} variant={countBadgeVariant} />{/if}
				</span>
				{#each columns as column, index (column.label)}
					{@const cell = entry.cells[index] ?? ''}
					<span class={cn('text-muted-foreground flex justify-end truncate text-xs tabular-nums', isHiddenOnPhone(index) && 'max-sm:hidden')}>
						{#if column.isBadge && cell}<Badge variant="outline">{cell}</Badge>{:else}{cell}{/if}
					</span>
				{/each}
			</button>
		{/each}
	{/if}
</section>
