<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import FilterCombobox from '$lib/components/filter-combobox.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Empty from '$lib/components/ui/empty';
	import * as Item from '$lib/components/ui/item';
	import BrainIcon from '@lucide/svelte/icons/brain';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import RotateCcwIcon from '@lucide/svelte/icons/rotate-ccw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import TrashIcon from '@lucide/svelte/icons/trash';
	import { onMount } from 'svelte';
	import { fetchMemoryFacts, forgetMemoryFact, type MemoryFact, type MemoryFactsResponse } from './memory-facts-api';
	import {
		allFilterValue,
		emptyMemoryFactFilters,
		filterMemoryFacts,
		isExpiringFact,
		isOwnOnlyFact,
		memoryFactCirclesOf,
		memoryFactKindsOf,
		ownOnlyFilterValue,
		sortMemoryFactsByRecency
	} from './memory-fact-list-model';
	import type { MemoryText } from './text';

	let { text }: { text: MemoryText } = $props();

	let memoryFacts = $state<MemoryFactsResponse | null>(null);
	let filters = $state(emptyMemoryFactFilters());
	let errorMessage = $state('');
	let actionErrorMessage = $state('');
	let isLoading = $state(false);

	const facts = $derived(memoryFacts?.facts ?? []);
	const profileLines = $derived([...(memoryFacts?.profile.identityLines ?? []), ...(memoryFacts?.profile.currentLines ?? [])]);
	const visibleFacts = $derived(sortMemoryFactsByRecency(filterMemoryFacts(facts, filters)));
	const hasActiveFilters = $derived(
		filters.searchText.trim() !== '' || filters.kind !== allFilterValue || filters.circle !== allFilterValue
	);

	const kindOptions = $derived([
		{ value: allFilterValue, label: text.factFilterKindAll },
		...memoryFactKindsOf(facts).map((kind) => ({ value: kind, label: kindLabel(kind) }))
	]);
	const circleOptions = $derived([
		{ value: allFilterValue, label: text.factFilterCircleAll },
		{ value: ownOnlyFilterValue, label: text.myMemory },
		...memoryFactCirclesOf(facts).map((circleID) => ({ value: circleID, label: circleID }))
	]);

	onMount(loadMemoryFacts);

	async function loadMemoryFacts(): Promise<void> {
		isLoading = true;
		errorMessage = '';
		actionErrorMessage = '';
		try {
			memoryFacts = await fetchMemoryFacts();
		} catch {
			errorMessage = text.loadFailed;
		} finally {
			isLoading = false;
		}
	}

	function resetFilters(): void {
		filters = emptyMemoryFactFilters();
	}

	function kindLabel(kind: string): string {
		if (kind === 'identity') return text.factKindIdentity;
		if (kind === 'preference') return text.factKindPreference;
		if (kind === 'fact') return text.factKindFact;
		if (kind === 'episode') return text.factKindEpisode;
		if (kind === 'temporary') return text.factKindTemporary;
		return kind;
	}

	function sharingText(fact: MemoryFact): string {
		if (isOwnOnlyFact(fact)) return text.myMemory;
		return fact.circleIDs.join(', ');
	}

	function validityText(fact: MemoryFact): string {
		const from = fact.validFrom.slice(0, 10);
		if (isExpiringFact(fact) && fact.validUntil) return `${from} → ${fact.validUntil.slice(0, 10)}`;
		return from;
	}

	function reinforcementText(fact: MemoryFact): string {
		if (fact.reinforcementCount <= 1) return '';
		return text.factReinforcedTemplate.replace('{count}', String(fact.reinforcementCount));
	}

	function countSummaryText(): string {
		return text.factListCountTemplate
			.replace('{visible}', String(visibleFacts.length))
			.replace('{total}', String(facts.length));
	}

	function confirmForgetFact(fact: MemoryFact): void {
		confirmDelete({
			title: text.memoryDeleteTitle,
			description: text.factDeleteDescription,
			confirm: { text: text.memoryDeleteAction },
			cancel: { text: text.cancel },
			onConfirm: async () => {
				await forgetFact(fact);
			}
		});
	}

	async function forgetFact(fact: MemoryFact): Promise<void> {
		actionErrorMessage = '';
		try {
			await forgetMemoryFact(fact.factID, text.factForgetReason);
			await loadMemoryFacts();
		} catch {
			actionErrorMessage = text.memoryDeleteFailed;
		}
	}
</script>

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.factListTab}</Card.Title>
		<Card.Description>{countSummaryText()}</Card.Description>
		<Card.Action>
			<Button type="button" variant="ghost" size="icon-sm" disabled={isLoading} onclick={loadMemoryFacts} aria-label={text.refresh} title={text.refresh}>
				<RefreshCwIcon class={isLoading ? 'animate-spin' : ''} />
			</Button>
		</Card.Action>
	</Card.Header>
	<Card.Content class="grid min-w-0 gap-4">
		{#if profileLines.length > 0}
			<section class="bg-muted/40 grid gap-1 rounded-md border px-3 py-2" aria-label={text.profileTitle}>
				<h3 class="text-muted-foreground text-xs font-medium tracking-wide uppercase">{text.profileTitle}</h3>
				<ul class="grid gap-0.5 text-sm leading-5">
					{#each profileLines as line (line)}
						<li>{line}</li>
					{/each}
				</ul>
			</section>
		{/if}

		<div class="flex min-w-0 flex-wrap items-center gap-2">
			<div class="relative min-w-48 flex-1">
				<SearchIcon class="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
				<Input bind:value={filters.searchText} placeholder={text.factFilterPlaceholder} autocomplete="off" class="pl-8" />
			</div>
			<FilterCombobox bind:value={filters.kind} options={kindOptions} label={text.factFilterKindAll} clearValue="all" class="w-36" />
			<FilterCombobox bind:value={filters.circle} options={circleOptions} label={text.factFilterCircleAll} clearValue="all" class="w-36" />
			{#if hasActiveFilters}
				<Button type="button" variant="ghost" size="sm" onclick={resetFilters}>
					<RotateCcwIcon class="size-4" />
					{text.factFilterReset}
				</Button>
			{/if}
		</div>

		{#if errorMessage || actionErrorMessage}
			<Field.Error>{errorMessage || actionErrorMessage}</Field.Error>
		{/if}

		{#if visibleFacts.length === 0 && !isLoading}
			<Empty.Root class="border border-dashed">
				<Empty.Header>
					<Empty.Media variant="icon">
						<BrainIcon />
					</Empty.Media>
					<Empty.Title>{facts.length === 0 ? text.noVisibleMemory : text.factListEmpty}</Empty.Title>
				</Empty.Header>
			</Empty.Root>
		{:else}
			<Item.Group class="gap-2">
				{#each visibleFacts as fact (fact.factID)}
					<Item.Root variant="outline" class="items-start">
						<Item.Content>
							<Item.Title class="text-muted-foreground gap-2 font-normal">
								<Badge variant="outline">{kindLabel(fact.kind)}</Badge>
								<span class="truncate text-xs">{sharingText(fact)}</span>
								<span class="text-xs tabular-nums">{validityText(fact)}</span>
								{#if reinforcementText(fact)}
									<span class="text-xs tabular-nums">{reinforcementText(fact)}</span>
								{/if}
							</Item.Title>
							<p class="text-sm leading-5">{fact.content}</p>
						</Item.Content>
						<Item.Actions class="self-start">
							<Button
								type="button"
								variant="ghost"
								size="icon-sm"
								onclick={() => confirmForgetFact(fact)}
								aria-label={text.memoryDelete}
								title={text.memoryDelete}
							>
								<TrashIcon class="size-4" />
							</Button>
						</Item.Actions>
					</Item.Root>
				{/each}
			</Item.Group>
		{/if}
	</Card.Content>
</Card.Root>
