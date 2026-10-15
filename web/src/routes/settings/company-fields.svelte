<script lang="ts">
	import { onMount } from 'svelte';
	import { z } from 'zod';
	import * as Card from '$lib/components/ui/card';
	import * as Select from '$lib/components/ui/select';
	import * as Table from '$lib/components/ui/table';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { invokeTool } from '$lib/public-api-call';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { dataRoomRequest } from '$lib/data-room/guest';
	import { categoryName, circleName } from '$lib/data-room/model';
	import { circleListResultSchema, dataRoomGetResultSchema } from '$lib/data-room/schemas';
	import { dataRoomCompanySchema, humanizedKey } from '$lib/data-room/company';
	import { dataRoomCompanyText } from '$lib/data-room/company-text';
	import {
		circlesReading,
		companyFieldRows,
		unlistedCategory,
		type CompanyFieldKind,
		type CompanyFieldRow
	} from '$lib/data-room/company-fields';

	type Room = z.infer<typeof dataRoomGetResultSchema>;
	type Circle = z.infer<typeof circleListResultSchema>['circles'][number];

	const kinds: CompanyFieldKind[] = ['profile', 'metric', 'record', 'empty'];
	const text = $derived(dataRoomCompanyText[currentLocale.value]);
	let room = $state<Room | null>(null);
	let circles = $state<Circle[]>([]);
	let rows = $state<CompanyFieldRow[]>([]);
	let savingField = $state('');
	let errorMessage = $state('');
	const categories = $derived((room?.categories ?? []).filter((category) => category.code !== unlistedCategory));

	async function load() {
		errorMessage = '';
		try {
			const [roomAnswer, circleAnswer, companyAnswer] = await Promise.all([
				invokeTool('dataroom_get', {}),
				invokeTool('circle_list', {}),
				dataRoomRequest('/api/v1/data-room/company')
			]);
			room = dataRoomGetResultSchema.parse(roomAnswer);
			circles = circleListResultSchema.parse(circleAnswer).circles;
			rows = companyFieldRows(room.categories, dataRoomCompanySchema.parse(companyAnswer));
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.loadFailed;
		}
	}

	async function move(row: CompanyFieldRow, categoryCode: string) {
		const previous = row.categoryCode;
		row.categoryCode = categoryCode;
		savingField = row.field;
		errorMessage = '';
		try {
			await invokeTool('dataroom_field_update', {
				field: row.field,
				...(categoryCode === unlistedCategory ? {} : { categoryCode })
			});
		} catch (error) {
			row.categoryCode = previous;
			errorMessage = error instanceof Error ? error.message : text.loadFailed;
		} finally {
			savingField = '';
		}
	}

	function fieldLabel(field: string): string {
		return text.fields[field] ?? text.metricNames[field] ?? text.recordCategories[field] ?? humanizedKey(field);
	}

	function categoryLabel(code: string): string {
		if (code === unlistedCategory) return text.settings.unlisted;
		const category = categories.find((candidate) => candidate.code === code);
		return category ? `${category.code} · ${categoryName(category, currentLocale.value)}` : code;
	}

	function readers(code: string): string {
		const names = circlesReading(code, circles, room?.categories ?? [])
			.map((circle) => circleName(circle, currentLocale.value));
		return names.join(', ') || text.settings.nobody;
	}

	onMount(load);
</script>

{#snippet categorySelect(row: CompanyFieldRow, canManage: boolean)}
	<Select.Root
		type="single"
		value={row.categoryCode}
		onValueChange={(value) => move(row, value)}
		disabled={savingField === row.field || !canManage}
	>
		<Select.Trigger class="w-full md:w-64" aria-label={fieldLabel(row.field)}>
			<span class="truncate">{categoryLabel(row.categoryCode)}</span>
		</Select.Trigger>
		<Select.Content class="max-h-80">
			<Select.Item value={unlistedCategory} label={text.settings.unlisted}>{text.settings.unlisted}</Select.Item>
			<Select.Separator />
			{#each categories as category (category.code)}
				<Select.Item value={category.code} label={categoryLabel(category.code)} class={category.parent ? 'pl-6' : 'font-medium'}>
					{categoryLabel(category.code)}
				</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
{/snippet}

<Card.Root>
	<Card.Header>
		<Card.Title>{text.settings.title}</Card.Title>
		<Card.Description>{text.settings.description}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-3">
		{#if !room && !errorMessage}
			<div role="status" aria-label={text.settings.title} aria-busy="true" class="grid gap-3">
				{#each [0, 1, 2] as row (row)}<Skeleton class="h-9 w-full" />{/each}
			</div>
		{:else if room}
			<Table.Root class="table-fixed">
				<Table.Header>
					<Table.Row>
						<Table.Head class="md:w-48">{text.settings.field}</Table.Head>
						<Table.Head class="w-68 max-md:hidden">{text.settings.category}</Table.Head>
						<Table.Head class="max-md:hidden">{text.settings.readers}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each kinds as kind (kind)}
						{@const kindRows = rows.filter((row) => row.kind === kind)}
						{#if kindRows.length > 0}
							<Table.Row class="hover:bg-transparent">
								<Table.Cell colspan={3} class="text-muted-foreground pt-5 pb-1 text-xs font-medium">{text.settings.kinds[kind]}</Table.Cell>
							</Table.Row>
							{#each kindRows as row (row.field)}
								<Table.Row>
									<Table.Cell>
										<p class="truncate font-medium">{fieldLabel(row.field)}</p>
										<p class="text-muted-foreground truncate font-mono text-xs">{row.field}</p>
										<div class="mt-2 grid gap-1 md:hidden">
											{@render categorySelect(row, room.canManage)}
											<p class="text-muted-foreground truncate text-xs">{readers(row.categoryCode)}</p>
										</div>
									</Table.Cell>
									<Table.Cell class="max-md:hidden">{@render categorySelect(row, room.canManage)}</Table.Cell>
									<Table.Cell class="text-muted-foreground text-sm whitespace-normal max-md:hidden">{readers(row.categoryCode)}</Table.Cell>
								</Table.Row>
							{/each}
						{/if}
					{/each}
				</Table.Body>
			</Table.Root>
		{/if}
		{#if errorMessage}<p role="alert" class="text-destructive text-sm">{errorMessage}</p>{/if}
	</Card.Content>
</Card.Root>
