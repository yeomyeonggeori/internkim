<script lang="ts">
	import UploadIcon from '@lucide/svelte/icons/upload';
	import { toast } from 'svelte-sonner';
	import { Badge } from '$lib/components/ui/badge';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as FileDropZone from '$lib/components/ui/file-drop-zone';
	import * as Select from '$lib/components/ui/select';
	import * as Table from '$lib/components/ui/table';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import type { CRMRecordImportKind } from '$lib/crm/crm-record-import';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { crmErrorMessage } from './crm-error-text';
	import { crmImportMaxFileBytes } from '$lib/crm/crm-import-csv';
	import { crmImportFieldNames, crmImportValueLabels } from './crm-import-headers';
	import { CRMImportFlow, crmImportFilters, type CRMImportPreviewRow } from './crm-import-flow.svelte';
	import { crmImportRequiredFields, type CRMImportField } from '$lib/crm/crm-import-mapping';
	import { crmImportText } from './crm-import-text';
	import { crmText } from './text';

	let {
		open = $bindable(false),
		kind,
		onImported
	}: {
		open?: boolean;
		kind: CRMRecordImportKind;
		onImported: () => Promise<void>;
	} = $props();

	const text = createPageText(crmImportText);
	const pageText = createPageText(crmText);
	const skipValue = '';
	const stepNumbers = { upload: 1, mapping: 2, preview: 3 } as const;

	let flow = $state(newFlow());
	let errorMessage = $state('');

	$effect(() => {
		if (!open) return;
		flow = newFlow();
		errorMessage = '';
	});

	function newFlow(): CRMImportFlow {
		return new CRMImportFlow({
			kind,
			fieldNames: crmImportFieldNames(kind),
			valueLabels: crmImportValueLabels
		});
	}

	function fieldLabel(field: CRMImportField): string {
		const label = text.fields[field];
		return crmImportRequiredFields[kind].includes(field) ? text.requiredMark.replace('{field}', label) : label;
	}

	async function run(action: () => Promise<void>): Promise<void> {
		errorMessage = '';
		try {
			await action();
		} catch (error) {
			errorMessage = crmErrorMessage(error, pageText);
		}
	}

	async function readFile(files: File[]): Promise<void> {
		const [file] = files;
		if (!file) return;
		await run(async () => {
			try {
				await flow.readFile(file);
			} catch (error) {
				if (!(error instanceof Error)) throw error;
				errorMessage = text.fileUnreadable;
				return;
			}
			if (flow.sheet?.lines.length === 0) errorMessage = text.fileEmpty;
		});
	}

	function rejectFile({ reason }: { reason: FileDropZone.FileRejectedReason }): void {
		errorMessage = reason === 'Maximum file size exceeded' ? text.fileTooLarge : text.fileNotCSV;
	}

	async function save(): Promise<void> {
		await run(async () => {
			const saved = await flow.save();
			if (!saved.written) return;
			await onImported();
			toast.success(text.saved.replace('{count}', String(saved.created + saved.updated)));
			open = false;
		});
	}

	function firstValue(column: number): string {
		return flow.sheet?.lines.find((line) => (line.cells[column] ?? '').trim() !== '')?.cells[column] ?? '';
	}

	function checkLabel(row: CRMImportPreviewRow): string {
		if (row.problems.length > 0) return row.problems.map((problem) => text.problems[problem]).join(', ');
		return row.isDuplicate ? text.duplicate : text.newRecord;
	}

	function checkVariant(row: CRMImportPreviewRow): 'destructive' | 'secondary' | 'outline' {
		if (row.problems.length > 0) return 'destructive';
		return row.isDuplicate ? 'secondary' : 'outline';
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-3xl">
		<Dialog.Header>
			<Dialog.Title>{text.titles[kind]}</Dialog.Title>
			<Dialog.Description>
				{text.stepCount.replace('{current}', String(stepNumbers[flow.step])).replace('{total}', '3')} · {text.steps[flow.step]}
			</Dialog.Description>
		</Dialog.Header>

		{#if flow.step === 'upload'}
			<FileDropZone.Root
				accept=".csv,text/csv"
				maxFileSize={crmImportMaxFileBytes}
				onUpload={readFile}
				onFileRejected={rejectFile}
				disabled={flow.isBusy}
			>
				<FileDropZone.Trigger>
					<div class="flex h-48 flex-col items-center justify-center gap-3 rounded-lg border border-dashed p-6 hover:cursor-pointer hover:bg-accent/25">
						<UploadIcon class="size-7 text-muted-foreground" aria-hidden="true" />
						<span class="font-medium">{text.dropHint}</span>
						<span class={buttonVariants({ variant: 'outline', size: 'sm' })}>{text.chooseFile}</span>
						<span class="text-sm text-muted-foreground">{text.fileLimit}</span>
					</div>
				</FileDropZone.Trigger>
			</FileDropZone.Root>
			<p class="text-sm text-muted-foreground">{text.exportedFileHint}</p>
		{:else if flow.step === 'mapping' && flow.sheet}
			<p class="text-sm text-muted-foreground">
				{text.fileSummary
					.replace('{fileName}', flow.sheet.fileName)
					.replace('{lines}', String(flow.sheet.lines.length))
					.replace('{columns}', String(flow.sheet.headers.length))}
			</p>
			<div class="max-h-[50vh] overflow-auto rounded-md border">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>{text.csvColumn}</Table.Head>
							<Table.Head>{text.firstValue}</Table.Head>
							<Table.Head>{text.targetField}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each flow.sheet.headers as header, column (column)}
							{@const field = flow.mapping[column] ?? null}
							<Table.Row>
								<Table.Cell class="font-medium">{header}</Table.Cell>
								<Table.Cell class="max-w-48 truncate text-muted-foreground">{firstValue(column)}</Table.Cell>
								<Table.Cell>
									<Select.Root
										type="single"
										value={field ?? skipValue}
										onValueChange={(value) =>
											flow.chooseField(column, flow.fields.find((candidate) => candidate === value) ?? null)}
									>
										<Select.Trigger class="w-full" aria-label={header}>
											{field ? fieldLabel(field) : text.skipColumn}
										</Select.Trigger>
										<Select.Content>
											<Select.Item value={skipValue} label={text.skipColumn}>{text.skipColumn}</Select.Item>
											{#each flow.fields as option (option)}
												<Select.Item value={option} label={fieldLabel(option)}>{fieldLabel(option)}</Select.Item>
											{/each}
										</Select.Content>
									</Select.Root>
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
			<p class="text-sm text-muted-foreground">{text.mappingHint}</p>
			{#if flow.missingFields.length > 0}
				<p class="text-sm text-destructive">
					{text.requiredMissing.replace('{fields}', flow.missingFields.map((field) => text.fields[field]).join(', '))}
				</p>
			{/if}
		{:else if flow.step === 'preview'}
			<div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
				{#each ['create', 'update', 'skip', 'refused'] as const as outcome (outcome)}
					<div class="rounded-md border p-3">
						<div class="text-sm text-muted-foreground">{text.counts[outcome]}</div>
						<div class="text-xl font-semibold">{flow.count(outcome)}</div>
					</div>
				{/each}
			</div>
			<ToggleGroup.Root
				type="single"
				value={flow.filter}
				onValueChange={(value) => {
					flow.filter = crmImportFilters.find((filter) => filter === value) ?? flow.filter;
				}}
				variant="outline"
				size="sm"
			>
				{#each crmImportFilters as filter (filter)}
					<ToggleGroup.Item value={filter}>{text.filters[filter]}</ToggleGroup.Item>
				{/each}
			</ToggleGroup.Root>
			<div class="max-h-[45vh] overflow-auto rounded-md border">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head class="w-16">{text.line}</Table.Head>
							<Table.Head>{text.fields.name}</Table.Head>
							<Table.Head>{text.check}</Table.Head>
							<Table.Head class="w-40">{text.action}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each flow.visibleRows as row (row.line)}
							<Table.Row>
								<Table.Cell class="text-muted-foreground">{row.line}</Table.Cell>
								<Table.Cell>{row.name}</Table.Cell>
								<Table.Cell><Badge variant={checkVariant(row)}>{checkLabel(row)}</Badge></Table.Cell>
								<Table.Cell>
									{#if row.outcome === 'refused'}
										<span class="text-muted-foreground">{text.notSaved}</span>
									{:else if row.isDuplicate}
										<Select.Root
											type="single"
											value={flow.choiceOf(row.line)}
											onValueChange={(value) => flow.choose(row.line, value === 'update' ? 'update' : 'skip')}
										>
											<Select.Trigger class="w-full" aria-label={`${text.action} ${row.line}`}>
												{text.duplicateChoices[flow.choiceOf(row.line)]}
											</Select.Trigger>
											<Select.Content>
												<Select.Item value="skip" label={text.duplicateChoices.skip}>{text.duplicateChoices.skip}</Select.Item>
												<Select.Item value="update" label={text.duplicateChoices.update}>{text.duplicateChoices.update}</Select.Item>
											</Select.Content>
										</Select.Root>
									{:else}
										<span>{text.counts.create}</span>
									{/if}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
			{#if flow.count('refused') > 0}
				<p class="text-sm text-destructive">{text.refusedHint.replace('{count}', String(flow.count('refused')))}</p>
			{/if}
		{/if}

		{#if errorMessage}
			<p class="text-sm text-destructive" role="alert">{errorMessage}</p>
		{/if}

		<Dialog.Footer>
			{#if flow.step === 'upload'}
				<Button variant="outline" onclick={() => (open = false)}>{text.cancel}</Button>
			{:else}
				<Button variant="outline" onclick={() => flow.back()} disabled={flow.isBusy}>{text.back}</Button>
			{/if}
			{#if flow.step === 'mapping'}
				<Button
					onclick={() => run(() => flow.check())}
					disabled={flow.isBusy || flow.missingFields.length > 0 || flow.sheet?.lines.length === 0}
				>
					{flow.isBusy ? text.checking : text.next}
				</Button>
			{:else if flow.step === 'preview'}
				<Button onclick={save} disabled={flow.isBusy || flow.savableCount === 0}>
					{flow.isBusy ? text.saving : text.save.replace('{count}', String(flow.savableCount))}
				</Button>
			{/if}
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
