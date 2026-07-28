<script lang="ts">
	import * as InputGroup from '$lib/components/ui/input-group';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { calendarText } from '../text';
	import type { CalendarSearchResult } from './calendar-search';

	type CalendarSearchBoxProps = {
		searchText: string;
		searchResults: CalendarSearchResult[];
		navigateToSearchResult: (result: CalendarSearchResult) => void;
	};

	let {
		searchText = $bindable(''),
		searchResults,
		navigateToSearchResult
	}: CalendarSearchBoxProps = $props();

	const text = createPageText(calendarText);

	function selectSearchResult(result: CalendarSearchResult): void {
		navigateToSearchResult(result);
		searchText = '';
	}
</script>

<div class="calendar-search-shell">
	<InputGroup.Root>
		<InputGroup.Addon>
			<SearchIcon />
		</InputGroup.Addon>
		<InputGroup.Input bind:value={searchText} placeholder={text.search} aria-label={text.searchCalendar} autocomplete="off" />
	</InputGroup.Root>
	{#if searchResults.length > 0}
		<div
			class="bg-popover text-popover-foreground absolute top-[calc(100%+6px)] right-0 z-40 w-[min(360px,72vw)] overflow-hidden rounded-md border shadow-md"
			role="listbox"
			aria-label={text.searchResults}
		>
			{#each searchResults as result (result.id)}
				<button
					type="button"
					role="option"
					aria-selected="false"
					class="hover:bg-accent focus-visible:bg-accent grid w-full grid-cols-[88px_minmax(0,1fr)] items-center gap-2.5 px-3 py-2.5 text-left outline-none"
					onclick={() => selectSearchResult(result)}
				>
					<span class="text-muted-foreground text-xs font-medium whitespace-nowrap">{result.dateLabel}</span>
					<span class="min-w-0 truncate text-sm">
						{#each result.highlightParts as part}
							{#if part.isMatch}
								<mark class="bg-primary/20 rounded-xs px-px text-inherit">{part.text}</mark>
							{:else}
								{part.text}
							{/if}
						{/each}
					</span>
				</button>
			{/each}
		</div>
	{:else if searchText.trim()}
		<div
			class="bg-popover text-muted-foreground absolute top-[calc(100%+6px)] right-0 z-40 w-[min(360px,72vw)] overflow-hidden rounded-md border px-3 py-2.5 text-sm shadow-md"
			role="status"
		>
			{text.noResults}
		</div>
	{/if}
</div>

<style>
	.calendar-search-shell {
		position: relative;
		width: min(220px, 20vw);
		flex-shrink: 1;
	}

	@media (max-width: 767px) {
		.calendar-search-shell {
			order: 4;
			width: 100%;
			margin-left: 0;
		}
	}
</style>
