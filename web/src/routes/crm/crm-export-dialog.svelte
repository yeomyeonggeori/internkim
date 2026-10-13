<script lang="ts">
	import { untrack } from 'svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Select from '$lib/components/ui/select';
	import * as Table from '$lib/components/ui/table';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Input } from '$lib/components/ui/input';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { csvDocument } from '$lib/csv';
	import { saveFile } from '$lib/save-file';
	import { currentCRMDate } from './crm-date';
	import {
		crmExportFilename,
		crmExportRowMatches,
		crmExportTable,
		crmExportTabs,
		type CRMExportSource,
		type CRMExportTab
	} from './crm-export-table';

	type Props = {
		open: boolean;
		initialTab: CRMExportTab;
		source: CRMExportSource;
	};

	let { open = $bindable(), initialTab, source }: Props = $props();

	const everyOrganization = 'all';

	let tab = $state<CRMExportTab>('relationships');
	let organizationID = $state(everyOrganization);
	let query = $state('');
	let checked = $state<Record<string, boolean>>({});

	const text = $derived(source.text);
	const table = $derived(crmExportTable(tab, source));
	const visibleRows = $derived(table.rows.filter((row) => crmExportRowMatches(row, query)));
	const selectedRows = $derived(table.rows.filter((row) => checked[row.id]));
	const tabLabels = $derived<Record<CRMExportTab, string>>({
		relationships: text.relationships,
		contacts: text.contactDirectory,
		pipeline: text.pipeline,
		activities: text.activities
	});
	const organizationOptions = $derived(
		[...source.organizations].sort((left, right) => left.name.localeCompare(right.name))
	);
	const organizationLabel = $derived(
		organizationOptions.find((organization) => organization.id === organizationID)?.name ?? text.export.allOrganizations
	);

	$effect(() => {
		if (!open) return;
		untrack(() => restart(initialTab, everyOrganization));
	});

	function restart(nextTab: CRMExportTab, nextOrganizationID: string): void {
		tab = nextTab;
		organizationID = nextOrganizationID;
		query = '';
		checked = Object.fromEntries(
			crmExportTable(nextTab, source).rows.map((row) => [
				row.id,
				nextOrganizationID === everyOrganization || row.organizationID === nextOrganizationID
			])
		);
	}

	function chooseTab(value: string): void {
		const chosen = crmExportTabs.find((candidate) => candidate === value);
		if (chosen) restart(chosen, organizationID);
	}

	function chooseOrganization(value: string): void {
		restart(tab, value || everyOrganization);
	}

	function markVisibleRows(value: boolean): void {
		checked = { ...checked, ...Object.fromEntries(visibleRows.map((row) => [row.id, value])) };
	}

	function download(): void {
		const content = csvDocument([table.columns, ...selectedRows.map((row) => row.cells)]);
		saveFile(crmExportFilename(tabLabels[tab], currentCRMDate(new Date(), source.timeZone)), content, 'text/csv;charset=utf-8');
		open = false;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content
		class="flex max-h-[calc(100dvh-2rem)] flex-col sm:max-w-5xl max-sm:h-dvh max-sm:max-h-dvh max-sm:max-w-full max-sm:rounded-none"
		closeLabel={text.cancel}
	>
		<Dialog.Header>
			<Dialog.Title>{text.export.title}</Dialog.Title>
			<Dialog.Description>{text.export.description}</Dialog.Description>
		</Dialog.Header>
		<Tabs.Root value={tab} onValueChange={chooseTab}>
			<Tabs.List aria-label={text.export.tabs} class="max-sm:w-full">
				{#each crmExportTabs as option (option)}
					<Tabs.Trigger value={option}>{tabLabels[option]}</Tabs.Trigger>
				{/each}
			</Tabs.List>
		</Tabs.Root>
		<div class="flex flex-wrap items-center gap-2">
			<Select.Root type="single" value={organizationID} onValueChange={chooseOrganization}>
				<Select.Trigger class="min-w-44 max-sm:w-full" aria-label={text.export.organizationFilter}>
					{text.export.organizationFilter}: {organizationLabel}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value={everyOrganization}>{text.export.allOrganizations}</Select.Item>
					{#each organizationOptions as organization (organization.id)}
						<Select.Item value={organization.id}>{organization.name}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
			<label class="relative min-w-0 flex-1 basis-60">
				<span class="sr-only">{text.export.search}</span>
				<SearchIcon class="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
				<Input type="search" class="pl-8" placeholder={text.export.search} bind:value={query} />
			</label>
		</div>
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex gap-2">
				<Button type="button" variant="outline" size="sm" onclick={() => markVisibleRows(true)}>{text.export.selectAll}</Button>
				<Button type="button" variant="outline" size="sm" onclick={() => markVisibleRows(false)}>{text.export.clearAll}</Button>
			</div>
			<span class="text-muted-foreground text-sm tabular-nums" aria-live="polite">
				{text.export.selectedCount
					.replace('{selected}', String(selectedRows.length))
					.replace('{total}', String(table.rows.length))}
			</span>
		</div>
		<div class="min-h-48 flex-1 overflow-y-auto rounded-lg border" data-crm-export-rows>
			<Table.Root class="whitespace-nowrap">
				<Table.Header class="bg-muted/50">
					<Table.Row class="hover:bg-transparent">
						<Table.Head class="w-10"><span class="sr-only">{text.export.selectAll}</span></Table.Head>
						{#each table.columns as column (column)}
							<Table.Head>{column}</Table.Head>
						{/each}
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each visibleRows as row (row.id)}
						<Table.Row class={checked[row.id] ? '' : 'text-muted-foreground'}>
							<Table.Cell>
								<Checkbox
									checked={checked[row.id] ?? false}
									onCheckedChange={(value) => (checked = { ...checked, [row.id]: value })}
									aria-label={text.export.selectRow.replace('{name}', row.cells[0] ?? '')}
								/>
							</Table.Cell>
							{#each row.cells as cell, index (index)}
								<Table.Cell class="max-w-64 truncate">{cell}</Table.Cell>
							{/each}
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
			{#if visibleRows.length === 0}
				<p class="text-muted-foreground p-8 text-center text-sm">{text.export.noMatches}</p>
			{/if}
		</div>
		<Dialog.Footer>
			<Button type="button" variant="outline" onclick={() => (open = false)}>{text.cancel}</Button>
			<Button type="button" disabled={selectedRows.length === 0} onclick={download}>
				<DownloadIcon data-icon="inline-start" />
				{text.export.download.replace('{count}', String(selectedRows.length))}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
