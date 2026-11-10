<script lang="ts">
	import BadgeCheckIcon from '@lucide/svelte/icons/badge-check';
	import CodeXmlIcon from '@lucide/svelte/icons/code-xml';
	import HandIcon from '@lucide/svelte/icons/hand';
	import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { repositoryURL } from './links';
	import { landingText } from './text';

	const text = createPageText(landingText);
	const itemIcons = [CodeXmlIcon, BadgeCheckIcon, ShieldCheckIcon, HandIcon];
</script>

<div class="grid gap-12 lg:grid-cols-[minmax(0,6fr)_minmax(0,5fr)] lg:gap-12">
	<div class="grid content-start gap-3">
		<Card.Root class="gap-0 py-0">
			<Table.Root>
				<Table.Caption class="sr-only">{text.ownership.table.caption}</Table.Caption>
				<Table.Header>
					<Table.Row>
						<Table.Head class="w-[44%] px-5">{text.ownership.table.what}</Table.Head>
						<Table.Head class="px-5">{text.ownership.table.hosted}</Table.Head>
						<Table.Head class="px-5">{text.ownership.table.selfHosted}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each text.ownership.table.rows as row (row.what)}
						<Table.Row class="hover:bg-transparent">
							<Table.Cell class="px-5 py-4 font-medium whitespace-normal break-keep">{row.what}</Table.Cell>
							<Table.Cell class="text-muted-foreground px-5 py-4 whitespace-normal break-keep">{row.hosted}</Table.Cell>
							<Table.Cell class="px-5 py-4 whitespace-normal break-keep">{row.selfHosted}</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</Card.Root>
		<p class="text-muted-foreground text-sm">{text.ownership.table.note}</p>
	</div>
	<div class="grid content-start gap-8">
		<ul class="grid gap-8 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
			{#each text.ownership.items as item, index (item.title)}
				{@const ItemIcon = itemIcons[index]}
				<li class="grid content-start gap-2">
					<ItemIcon class="text-muted-foreground size-5" />
					<p class="font-semibold break-keep">{item.title}</p>
					<p class="text-muted-foreground leading-7 break-keep">{item.body}</p>
				</li>
			{/each}
		</ul>
		<a href={repositoryURL} class="w-fit text-sm font-medium underline-offset-4 hover:underline">{text.ownership.source} →</a>
	</div>
</div>
