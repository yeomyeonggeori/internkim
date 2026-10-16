<script lang="ts">
	import AwardIcon from '@lucide/svelte/icons/award';
	import BadgeCheckIcon from '@lucide/svelte/icons/badge-check';
	import BuildingIcon from '@lucide/svelte/icons/building-2';
	import ScrollTextIcon from '@lucide/svelte/icons/scroll-text';
	import CircleDotIcon from '@lucide/svelte/icons/circle-dot';
	import type { Component } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { dataRoomCompanyText } from './company-text';
	import { humanizedKey, type RecordGroup } from './company';

	let { groups, locale }: { groups: RecordGroup[]; locale: 'ko' | 'en' } = $props();

	const visibleCount = 4;
	const text = $derived(dataRoomCompanyText[locale]);
	const columns = $derived(groups.length % 3 === 0 ? 3 : Math.min(groups.length, 2));
	const icons: Record<string, Component> = {
		certification: BadgeCheckIcon,
		ip: ScrollTextIcon,
		award: AwardIcon,
		reference: BuildingIcon
	};
	let expanded = $state<string[]>([]);

	function toggle(category: string) {
		expanded = expanded.includes(category) ? expanded.filter((open) => open !== category) : [...expanded, category];
	}

	function yearOf(date: string | undefined): string {
		return date?.slice(0, 4) ?? '';
	}
</script>

<section class="grid gap-3" aria-labelledby="company-proof">
	<h3 id="company-proof" class="text-lg font-semibold">{text.proof}</h3>
	<div class="bg-border grid gap-px overflow-hidden rounded-lg border sm:grid-cols-(--proof-columns)" style:--proof-columns={`repeat(${columns}, minmax(0, 1fr))`}>
		{#each groups as group, index (group.category)}
			{@const Icon = icons[group.category] ?? CircleDotIcon}
			{@const isExpanded = expanded.includes(group.category)}
			{@const shown = isExpanded ? group.records : group.records.slice(0, visibleCount)}
			<article class="bg-background grid content-start gap-3 p-5" style:grid-column={index === groups.length - 1 && groups.length % columns !== 0 ? `span ${columns - (groups.length % columns) + 1}` : undefined}>
				<header class="flex items-center gap-2">
					<Icon class="text-muted-foreground size-4" />
					<h4 class="text-sm font-semibold">{text.recordCategories[group.category] ?? humanizedKey(group.category)}</h4>
					<span class="text-muted-foreground text-sm tabular-nums">{group.records.length}</span>
				</header>
				<ul class="grid gap-2">
					{#each shown as record, index (index)}
						<li class="grid grid-cols-[minmax(0,1fr)_auto] gap-3 text-sm">
							<div class="min-w-0">
								<p class="text-pretty">{record.title}</p>
								{#if record.detail}<p class="text-muted-foreground text-xs text-pretty">{record.detail}</p>{/if}
							</div>
							<time class="text-muted-foreground text-xs tabular-nums">{yearOf(record.date)}</time>
						</li>
					{/each}
				</ul>
				{#if group.records.length > visibleCount}
					<Button variant="link" size="sm" class="h-auto justify-self-start p-0" onclick={() => toggle(group.category)}>
						{isExpanded ? text.showFewer : text.showAll(group.records.length)}
					</Button>
				{/if}
			</article>
		{/each}
	</div>
</section>
