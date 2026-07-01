<script lang="ts">
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
	<label class="calendar-search">
		<SearchIcon class="size-4" />
		<input bind:value={searchText} placeholder={text.search} aria-label={text.searchCalendar} autocomplete="off" />
	</label>
	{#if searchResults.length > 0}
		<div class="calendar-search-results" role="listbox" aria-label={text.searchResults}>
			{#each searchResults as result (result.id)}
				<button type="button" role="option" aria-selected="false" class="calendar-search-result" onclick={() => selectSearchResult(result)}>
					<span class="calendar-search-result-date">{result.dateLabel}</span>
					<span class="calendar-search-result-title">
						{#each result.highlightParts as part}
							{#if part.isMatch}
								<mark>{part.text}</mark>
							{:else}
								{part.text}
							{/if}
						{/each}
					</span>
				</button>
			{/each}
		</div>
	{:else if searchText.trim()}
		<div class="calendar-search-results" role="status">
			<p class="calendar-search-empty">{text.noResults}</p>
		</div>
	{/if}
</div>

<style>
	.calendar-search-shell {
		position: relative;
		width: min(220px, 20vw);
		flex-shrink: 1;
	}

	.calendar-search {
		display: flex;
		height: 36px;
		width: 100%;
		align-items: center;
		gap: 10px;
		border: 1px solid #e5e7eb;
		border-radius: 8px;
		background: #ffffff;
		padding: 0 12px;
		color: #71717a;
	}

	.calendar-search input {
		min-width: 0;
		flex: 1;
		border: 0;
		background: transparent;
		color: #111827;
		font-size: 14px;
		outline: none;
	}

	.calendar-search input::placeholder {
		color: #71717a;
	}

	.calendar-search-results {
		position: absolute;
		z-index: 40;
		top: calc(100% + 6px);
		right: 0;
		width: min(360px, 72vw);
		overflow: hidden;
		border: 1px solid #e5e7eb;
		border-radius: 10px;
		background: #ffffff;
		box-shadow: 0 14px 30px rgb(15 23 42 / 0.16);
	}

	.calendar-search-result {
		display: grid;
		width: 100%;
		grid-template-columns: 88px minmax(0, 1fr);
		align-items: center;
		gap: 10px;
		border: 0;
		background: transparent;
		padding: 10px 12px;
		text-align: left;
		color: #18181b;
		cursor: pointer;
	}

	.calendar-search-result:hover,
	.calendar-search-result:focus-visible {
		background: #f4f4f5;
		outline: none;
	}

	.calendar-search-result-date {
		color: #71717a;
		font-size: 12px;
		font-weight: 700;
		white-space: nowrap;
	}

	.calendar-search-result-title {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 13px;
		font-weight: 650;
	}

	.calendar-search-result-title mark {
		border-radius: 4px;
		background: color-mix(in oklab, oklch(0.55 0.19 255) 20%, transparent);
		color: inherit;
		padding: 0 1px;
	}

	.calendar-search-empty {
		margin: 0;
		padding: 10px 12px;
		color: #71717a;
		font-size: 13px;
	}

	:global(html.dark) .calendar-search {
		border-color: #27272a;
		background: #09090b;
		color: #a1a1aa;
	}

	:global(html.dark) .calendar-search input {
		color: #f4f4f5;
	}

	:global(html.dark) .calendar-search-results {
		border-color: #27272a;
		background: #09090b;
		box-shadow: 0 14px 30px rgb(0 0 0 / 0.38);
	}

	:global(html.dark) .calendar-search-result {
		color: #f4f4f5;
	}

	:global(html.dark) .calendar-search-result:hover,
	:global(html.dark) .calendar-search-result:focus-visible {
		background: #18181b;
	}

	:global(html.dark) .calendar-search-result-title mark {
		background: color-mix(in oklab, oklch(0.6 0.2 255) 36%, transparent);
	}

	@media (max-width: 767px) {
		.calendar-search-shell {
			order: 4;
			width: 100%;
			margin-left: 0;
		}
	}
</style>
