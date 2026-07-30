<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import ColorPicker from '$lib/components/color-picker.svelte';
	import { flowDefinitionBadgeStyle, flowSizeColor } from './flow-definition-colors';
	import { sizeBadgeClass } from './flow-style';
	import type { FlowDefinitions } from './flow-types';

	type SizeDefinitionsText = {
		size: string;
		sizeDescription: string;
		sizeName: string;
		distance: string;
		maxHours: string;
		developmentExample: string;
		otherExample: string;
		note: string;
		color: string;
	};

	type Props = {
		definitions: FlowDefinitions;
		text: SizeDefinitionsText;
		isAdmin?: boolean;
		setSizeColor?: (name: string, color: string) => void;
	};

	let { definitions, text, isAdmin = false, setSizeColor }: Props = $props();
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.size}</Card.Title>
		<Card.Description>{text.sizeDescription}</Card.Description>
	</Card.Header>
	<Card.Content>
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
					{#each definitions.sizes as size (size.name)}
						<Table.Row>
							<Table.Cell>
								<div class="flex items-center gap-2">
									{#if isAdmin && setSizeColor}
										<ColorPicker
											value={flowSizeColor(size.name, definitions) || '#64748b'}
											label={text.color}
											class="size-7"
											onChange={(color) => setSizeColor(size.name, color)}
										/>
									{/if}
									<Badge
										class={flowSizeColor(size.name, definitions) ? 'rounded-md border-transparent font-mono tabular-nums shadow-none' : sizeBadgeClass(size.name)}
										style={flowDefinitionBadgeStyle(flowSizeColor(size.name, definitions))}
									>{size.name}</Badge>
								</div>
							</Table.Cell>
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
	</Card.Content>
</Card.Root>
