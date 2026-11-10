<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import { categoryName, dataRoomTemplate } from '$lib/data-room/model';
	import { currentLocale } from '$lib/i18n/locale.svelte';

	let { title }: { title: string } = $props();

	const outsideCircleIDs = ['investor', 'accountant', 'legal', 'lender'];

	const circles = $derived(
		dataRoomTemplate.circles
			.filter((circle) => outsideCircleIDs.includes(circle.id))
			.map((circle) => ({
				id: circle.id,
				name: currentLocale.value === 'ko' ? circle.nameKO : circle.name,
				categories: circle.readableCategories
					.map((code) => dataRoomTemplate.categories.find((category) => category.code === code))
					.filter((category) => category !== undefined)
					.map((category) => categoryName(category, currentLocale.value))
			}))
	);
</script>

<Card.Root class="gap-0 py-0">
	<Card.Header class="border-b py-4">
		<Card.Title class="text-muted-foreground text-sm font-medium">{title}</Card.Title>
	</Card.Header>
	<Card.Content class="px-0">
		<dl class="divide-y">
			{#each circles as circle (circle.id)}
				<div class="grid gap-2.5 px-6 py-4 sm:grid-cols-[7.5rem_minmax(0,1fr)] sm:gap-4">
					<dt class="font-medium break-keep">{circle.name}</dt>
					<dd class="flex flex-wrap gap-1.5">
						{#each circle.categories as category (category)}
							<Badge variant="secondary" class="font-normal">{category}</Badge>
						{/each}
					</dd>
				</div>
			{/each}
		</dl>
	</Card.Content>
</Card.Root>
