<script lang="ts" module>
	export type FileBrowserEntry = {
		id: string;
		name: string;
		fileName?: string;
		isDirectory?: boolean;
		secondary: string;
		date: string;
	};
</script>

<script lang="ts">
	import FolderIcon from '@lucide/svelte/icons/folder';
	import FolderOpenIcon from '@lucide/svelte/icons/folder-open';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Badge } from '$lib/components/ui/badge';
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
		secondaryLabel,
		isSecondaryBadge = false,
		dateLabel,
		emptyLabel,
		onSelect
	}: {
		entries: FileBrowserEntry[];
		selectedID?: string;
		isLoading?: boolean;
		hasLoadError?: boolean;
		title: string;
		nameLabel: string;
		secondaryLabel: string;
		isSecondaryBadge?: boolean;
		dateLabel: string;
		emptyLabel: string;
		onSelect: (entry: FileBrowserEntry) => void;
	} = $props();
</script>

<section
	aria-label={title}
	aria-busy={isLoading}
	class="bg-card min-w-0 flex-1 overflow-hidden rounded-xl border shadow-sm"
>
	<div
		class="text-muted-foreground bg-muted/40 grid grid-cols-[minmax(0,1fr)_5rem_7rem] gap-3 border-b px-4 py-2.5 text-xs font-medium max-sm:grid-cols-[minmax(0,1fr)_6rem]"
	>
		<span>{nameLabel}</span>
		<span class="text-right max-sm:hidden">{secondaryLabel}</span>
		<span class="text-right">{dateLabel}</span>
	</div>
	{#if isLoading && entries.length === 0}
		<div role="status" aria-label={title} data-testid="file-list-loading-skeleton">
			{#each [0, 1, 2, 3, 4, 5] as row (row)}
				<div aria-hidden="true" class="grid grid-cols-[minmax(0,1fr)_5rem_7rem] items-center gap-3 border-b px-4 py-2.5 last:border-b-0 max-sm:min-h-16 max-sm:grid-cols-[minmax(0,1fr)_6rem]">
					<div class="flex min-h-5 min-w-0 items-center gap-2.5"><Skeleton class="size-4 shrink-0" /><Skeleton class="h-4 w-3/4 max-w-64" /></div>
					<Skeleton class="ml-auto h-3 w-12 max-sm:hidden" /><Skeleton class="ml-auto h-3 w-20" />
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
					'hover:bg-accent/60 grid w-full grid-cols-[minmax(0,1fr)_5rem_7rem] items-center gap-3 border-b px-4 py-2.5 text-left transition-colors last:border-b-0 max-sm:min-h-16 max-sm:grid-cols-[minmax(0,1fr)_6rem]',
					selectedID === entry.id && 'bg-accent'
				)}
				onclick={() => onSelect(entry)}
			>
				<span class="flex min-w-0 items-center gap-2.5">
					{#if entry.isDirectory}<FolderIcon
							class="size-4 shrink-0 fill-amber-200 text-amber-500"
						/>
					{:else}<FileTypeIcon class={cn('size-4 shrink-0', visual.colorClass)} />{/if}
					<span class="min-w-0 text-sm max-sm:line-clamp-2 max-sm:break-all sm:truncate">{entry.name}</span>
				</span>
				<span class="text-muted-foreground flex justify-end text-xs tabular-nums max-sm:hidden">
					{#if isSecondaryBadge && entry.secondary}<Badge variant="outline">{entry.secondary}</Badge>
					{:else}{entry.secondary}{/if}
				</span>
				<span class="text-muted-foreground flex justify-end truncate text-xs">
					{entry.date}
				</span>
			</button>
		{/each}
		{#if isLoading}<div class="flex justify-center py-4" role="status" aria-label={title}><Spinner /></div>{/if}
	{/if}
</section>
