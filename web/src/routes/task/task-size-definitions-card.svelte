<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as Accordion from '$lib/components/ui/accordion';
	import { MediaQuery } from 'svelte/reactivity';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { taskSizes } from '$lib/task/task-sizes';
	import { sizeBadgeClass } from './task-style';

	type SizeDefinitionsText = {
		size: string;
		sizeDescription: string;
		sizeName: string;
		distance: string;
		maxHours: string;
		developmentExample: string;
		otherExample: string;
		note: string;
	};

	type Props = {
		text: SizeDefinitionsText;
	};

	let { text }: Props = $props();
	const sizes = $derived(taskSizes(currentLocale.value));
	const isMobile = new MediaQuery('(max-width: 639px)');
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.size}</Card.Title>
		<Card.Description>{text.sizeDescription}</Card.Description>
	</Card.Header>
	<Card.Content>
		{#if isMobile.current}
			<Accordion.Root type="multiple">
				{#each sizes as size (size.name)}
					<Accordion.Item value={size.name}>
						<Accordion.Trigger class="min-h-11"><Badge class={sizeBadgeClass(size.name)}>{size.name}</Badge><span class="ml-auto text-sm tabular-nums">{size.distanceKm} km · {size.maxHours} h</span></Accordion.Trigger>
						<Accordion.Content><dl class="grid gap-3 text-sm">
							<div><dt class="text-xs text-muted-foreground">{text.developmentExample}</dt><dd>{size.developmentExample}</dd></div>
							<div><dt class="text-xs text-muted-foreground">{text.otherExample}</dt><dd>{size.otherExample}</dd></div>
							<div><dt class="text-xs text-muted-foreground">{text.note}</dt><dd>{size.note}</dd></div>
						</dl></Accordion.Content>
					</Accordion.Item>
				{/each}
			</Accordion.Root>
		{:else}
		<div class="overflow-hidden rounded-lg border">
			<Table.Root class="min-w-[980px]">
				<Table.Header class="bg-muted/40">
					<Table.Row class="hover:bg-transparent">
						<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.sizeName}</Table.Head>
						<Table.Head class="h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.distance}</Table.Head>
						<Table.Head class="h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.maxHours}</Table.Head>
						<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.developmentExample}</Table.Head>
						<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.otherExample}</Table.Head>
						<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.note}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each sizes as size (size.name)}
						<Table.Row>
							<Table.Cell><Badge class={sizeBadgeClass(size.name)}>{size.name}</Badge></Table.Cell>
							<Table.Cell class="text-right tabular-nums">{size.distanceKm}</Table.Cell>
							<Table.Cell class="text-right tabular-nums">{size.maxHours}</Table.Cell>
							<Table.Cell>{size.developmentExample}</Table.Cell>
							<Table.Cell>{size.otherExample}</Table.Cell>
							<Table.Cell class="text-muted-foreground">{size.note}</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
		{/if}
	</Card.Content>
</Card.Root>
