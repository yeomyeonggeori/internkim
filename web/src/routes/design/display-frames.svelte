<script lang="ts">
	import { getCoreRowModel, type ColumnDef } from '@tanstack/table-core';
	import { BarChart } from 'layerchart';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import { alertVariants } from '$lib/components/ui/alert/index.js';
	import * as Attachment from '$lib/components/ui/attachment/index.js';
	import { attachmentVariants } from '$lib/components/ui/attachment/index.js';
	import * as Avatar from '$lib/components/ui/avatar/index.js';
	import { Badge, badgeVariants } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Chart from '$lib/components/ui/chart/index.js';
	import * as Code from '$lib/components/ui/code/index.js';
	import { codeVariants } from '$lib/components/ui/code/index.js';
	import { createSvelteTable, FlexRender } from '$lib/components/ui/data-table/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import { emptyMediaVariants } from '$lib/components/ui/empty/empty-media.svelte';
	import * as Item from '$lib/components/ui/item/index.js';
	import { itemVariants } from '$lib/components/ui/item/item.svelte';
	import * as Marker from '$lib/components/ui/marker/index.js';
	import { markerVariants } from '$lib/components/ui/marker/index.js';
	import Meter from '$lib/components/ui/meter/meter.svelte';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton/index.js';
	import { Spinner } from '$lib/components/ui/spinner/index.js';
	import * as Table from '$lib/components/ui/table/index.js';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import CircleAlertIcon from '@lucide/svelte/icons/circle-alert';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import FileSpreadsheetIcon from '@lucide/svelte/icons/file-spreadsheet';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import FolderOpenIcon from '@lucide/svelte/icons/folder-open';
	import InfoIcon from '@lucide/svelte/icons/info';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import SparklesIcon from '@lucide/svelte/icons/sparkles';
	import XIcon from '@lucide/svelte/icons/x';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import DesignFrame from './design-frame.svelte';
	import DesignSection from './design-section.svelte';
	import { designText } from './text';
	import { variantAxes, variantValues } from './workspace.svelte';

	let { pageID }: { pageID: string } = $props();

	const text = createPageText(designText);

	type SampleRow = { name: string; team: string; status: string };

	const rows: SampleRow[] = $derived([
		{ name: '이샘플', team: text.sample.teamSales, status: text.sample.statusDone },
		{ name: '박예시', team: text.sample.teamProduct, status: text.sample.statusDoing },
		{ name: '최견본', team: text.sample.teamProduct, status: text.sample.statusWaiting }
	]);
	const columns: ColumnDef<SampleRow>[] = $derived([
		{ accessorKey: 'name', header: text.sample.name },
		{ accessorKey: 'team', header: text.sample.team },
		{ accessorKey: 'status', header: text.sample.status }
	]);
	const table = createSvelteTable({
		get data() {
			return rows;
		},
		get columns() {
			return columns;
		},
		getCoreRowModel: getCoreRowModel()
	});

	const chartData = $derived([
		{ week: text.sample.weekOne, done: 8, open: 5 },
		{ week: text.sample.weekTwo, done: 12, open: 4 },
		{ week: text.sample.weekThree, done: 9, open: 7 },
		{ week: text.sample.weekFour, done: 15, open: 3 }
	]);
	const chartConfig: Chart.ChartConfig = $derived({
		done: { label: text.sample.statusDone, color: 'var(--color-brand)' },
		open: { label: text.sample.statusDoing, color: 'var(--color-muted-foreground)' }
	});
	const chartSeries = $derived([
		{ key: 'done', label: text.sample.statusDone, color: 'var(--color-brand)' },
		{ key: 'open', label: text.sample.statusDoing, color: 'var(--color-muted-foreground)' }
	]);

	const sampleCode = `import { Button } from '$lib/components/ui/button';\n\nconst label = 'save';`;
</script>

<DesignSection {pageID} title={text.displaySection}>
	<DesignFrame id="badge" name="Badge" source="ui/badge" variants={variantAxes(badgeVariants.variants)}>
		{#snippet children()}
			<div class="flex flex-wrap items-center gap-2">
				{#each variantValues(badgeVariants.variants.variant) as variant (variant)}
					<Badge {variant}>{variant}</Badge>
				{/each}
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="avatar" name="Avatar" source="ui/avatar">
		{#snippet children()}
			<div class="flex items-center gap-3">
				<Avatar.Root><Avatar.Fallback>이</Avatar.Fallback></Avatar.Root>
				<Avatar.Root class="size-10"><Avatar.Fallback>박</Avatar.Fallback></Avatar.Root>
				<PersonAvatar name="최견본" seed="design-avatar" class="size-8" />
			</div>
			<Avatar.Group>
				<Avatar.Root><Avatar.Fallback>이</Avatar.Fallback></Avatar.Root>
				<Avatar.Root><Avatar.Fallback>박</Avatar.Fallback></Avatar.Root>
				<Avatar.Root><Avatar.Fallback>최</Avatar.Fallback></Avatar.Root>
				<Avatar.GroupCount>+4</Avatar.GroupCount>
			</Avatar.Group>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="card" name="Card" source="ui/card" class="w-96">
		{#snippet children()}
			<Card.Root>
				<Card.Header>
					<Card.Title>{text.sample.cardTitle}</Card.Title>
					<Card.Description>{text.sample.cardDescription}</Card.Description>
					<Card.Action><Button variant="outline" size="sm">{text.sample.open}</Button></Card.Action>
				</Card.Header>
				<Card.Content class="text-sm">{text.sample.cardBody}</Card.Content>
				<Card.Footer class="text-muted-foreground text-xs">{text.sample.cardFooter}</Card.Footer>
			</Card.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="item" name="Item" source="ui/item" variants={variantAxes(itemVariants.variants)} class="w-96">
		{#snippet children()}
			<Item.Group>
				{#each variantValues(itemVariants.variants.variant) as variant (variant)}
					<Item.Root {variant}>
						<Item.Media variant="icon"><FileTextIcon /></Item.Media>
						<Item.Content>
							<Item.Title>{text.sample.fileName}</Item.Title>
							<Item.Description>{variant}</Item.Description>
						</Item.Content>
						<Item.Actions><Button variant="ghost" size="icon-sm" aria-label={text.sample.download}><DownloadIcon /></Button></Item.Actions>
					</Item.Root>
				{/each}
			</Item.Group>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="table" name="Table" source="ui/table" class="w-[28rem]">
		{#snippet children()}
			<Table.Root>
				<Table.Caption>{text.sample.tableCaption}</Table.Caption>
				<Table.Header>
					<Table.Row>
						<Table.Head>{text.sample.name}</Table.Head>
						<Table.Head>{text.sample.team}</Table.Head>
						<Table.Head class="text-right">{text.sample.status}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each rows as row (row.name)}
						<Table.Row>
							<Table.Cell class="font-medium">{row.name}</Table.Cell>
							<Table.Cell>{row.team}</Table.Cell>
							<Table.Cell class="text-right">{row.status}</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="data-table" name="DataTable" source="ui/data-table" class="w-[28rem]">
		{#snippet children()}
			<div class="rounded-lg border">
				<Table.Root>
					<Table.Header>
						{#each table.getHeaderGroups() as headerGroup (headerGroup.id)}
							<Table.Row>
								{#each headerGroup.headers as header (header.id)}
									<Table.Head><FlexRender content={header.column.columnDef.header} context={header.getContext()} /></Table.Head>
								{/each}
							</Table.Row>
						{/each}
					</Table.Header>
					<Table.Body>
						{#each table.getRowModel().rows as row (row.id)}
							<Table.Row>
								{#each row.getVisibleCells() as cell (cell.id)}
									<Table.Cell><FlexRender content={cell.column.columnDef.cell} context={cell.getContext()} /></Table.Cell>
								{/each}
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="chart" name="Chart" source="ui/chart" class="w-[28rem]">
		{#snippet children()}
			<Chart.Container config={chartConfig} class="h-48 w-full">
				<BarChart data={chartData} x="week" axis="x" grid rule={false} series={chartSeries} seriesLayout="group" props={{ bars: { strokeWidth: 0, radius: 2 } }} />
			</Chart.Container>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="meter" name="Meter" source="ui/meter">
		{#snippet children()}
			<Meter value={25} aria-label="25%" />
			<Meter value={60} aria-label="60%" />
			<Meter value={95} class="h-1" aria-label="95%" />
		{/snippet}
	</DesignFrame>

	<DesignFrame id="code" name="Code" source="ui/code" variants={variantAxes(codeVariants.variants)} class="w-96">
		{#snippet children()}
			<Code.Root lang="typescript" code={sampleCode} class="text-xs"><Code.CopyButton /></Code.Root>
			<Code.Root lang="bash" hideLines code="./internkim dev plane" variant="secondary" class="text-xs" />
		{/snippet}
	</DesignFrame>

	<DesignFrame id="separator" name="Separator" source="ui/separator">
		{#snippet children()}
			<div class="grid gap-3 text-sm">
				<span>{text.sample.cardTitle}</span>
				<Separator />
				<div class="flex h-5 items-center gap-3">
					<span>{text.sample.day}</span>
					<Separator orientation="vertical" />
					<span>{text.sample.week}</span>
					<Separator orientation="vertical" />
					<span>{text.sample.month}</span>
				</div>
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="skeleton" name="Skeleton" source="ui/skeleton">
		{#snippet children()}
			<div class="flex items-center gap-3">
				<Skeleton class="size-10 rounded-full" />
				<div class="grid flex-1 gap-2">
					<Skeleton class="h-4 w-3/4" />
					<Skeleton class="h-4 w-1/2" />
				</div>
			</div>
			<Skeleton class="h-24 w-full rounded-xl" />
		{/snippet}
	</DesignFrame>

	<DesignFrame id="spinner" name="Spinner" source="ui/spinner">
		{#snippet children()}
			<div class="flex items-center gap-4">
				<Spinner class="size-3" />
				<Spinner />
				<Spinner class="size-6" />
				<Button disabled size="sm"><Spinner data-icon="inline-start" />{text.sample.saving}</Button>
			</div>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="marker" name="Marker" source="ui/marker" variants={variantAxes(markerVariants.variants)}>
		{#snippet children()}
			{#each variantValues(markerVariants.variants.variant) as variant (variant)}
				<Marker.Root {variant}>
					{#if variant === 'default'}<Marker.Icon><InfoIcon /></Marker.Icon>{/if}
					<Marker.Content>{variant}</Marker.Content>
				</Marker.Root>
			{/each}
		{/snippet}
	</DesignFrame>

	<DesignFrame id="empty" name="Empty" source="ui/empty" variants={variantAxes(emptyMediaVariants.variants)}>
		{#snippet children()}
			<Empty.Root class="border border-dashed">
				<Empty.Header>
					<Empty.Media variant="icon"><FolderOpenIcon /></Empty.Media>
					<Empty.Title>{text.sample.emptyTitle}</Empty.Title>
					<Empty.Description>{text.sample.emptyDescription}</Empty.Description>
				</Empty.Header>
				<Empty.Content><Button size="sm">{text.sample.add}</Button></Empty.Content>
			</Empty.Root>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="alert" name="Alert" source="ui/alert" variants={variantAxes(alertVariants.variants)} class="w-96">
		{#snippet children()}
			{#each variantValues(alertVariants.variants.variant) as variant (variant)}
				<Alert.Root {variant}>
					{#if variant === 'destructive'}<CircleAlertIcon />{:else}<SparklesIcon />{/if}
					<Alert.Title>{variant === 'destructive' ? text.sample.alertErrorTitle : text.sample.alertTitle}</Alert.Title>
					<Alert.Description>{variant === 'destructive' ? text.sample.alertErrorDescription : text.sample.alertDescription}</Alert.Description>
					<Alert.Action><Button variant="outline" size="xs">{text.sample.open}</Button></Alert.Action>
				</Alert.Root>
			{/each}
		{/snippet}
	</DesignFrame>

	<DesignFrame id="attachment" name="Attachment" source="ui/attachment" variants={variantAxes(attachmentVariants.variants)} class="w-[30rem]">
		{#snippet children()}
			<div class="flex flex-wrap gap-2">
				<Attachment.Root>
					<Attachment.Media><FileSpreadsheetIcon /></Attachment.Media>
					<Attachment.Content>
						<Attachment.Title>{text.sample.sheetName}</Attachment.Title>
						<Attachment.Description>XLSX · 48 KB</Attachment.Description>
					</Attachment.Content>
					<Attachment.Actions><Attachment.Action aria-label={text.sample.download}><DownloadIcon /></Attachment.Action></Attachment.Actions>
				</Attachment.Root>
				<Attachment.Root state="uploading">
					<Attachment.Media><Spinner /></Attachment.Media>
					<Attachment.Content>
						<Attachment.Title>{text.sample.fileName}</Attachment.Title>
						<Attachment.Description>{text.sample.uploading}</Attachment.Description>
					</Attachment.Content>
				</Attachment.Root>
				<Attachment.Root state="error">
					<Attachment.Media><FileTextIcon /></Attachment.Media>
					<Attachment.Content>
						<Attachment.Title>{text.sample.fileName}</Attachment.Title>
						<Attachment.Description>{text.sample.loadFailed}</Attachment.Description>
					</Attachment.Content>
					<Attachment.Actions><Attachment.Action aria-label={text.sample.retry}><RotateCwIcon /></Attachment.Action></Attachment.Actions>
				</Attachment.Root>
				<Attachment.Root state="idle" size="sm">
					<Attachment.Media><FileTextIcon /></Attachment.Media>
					<Attachment.Content><Attachment.Title>{text.sample.fileName}</Attachment.Title></Attachment.Content>
					<Attachment.Actions><Attachment.Action aria-label={text.sample.remove}><XIcon /></Attachment.Action></Attachment.Actions>
				</Attachment.Root>
			</div>
			<div class="flex flex-wrap gap-2">
				{#each variantValues(attachmentVariants.variants.size) as size (size)}
					<Attachment.Root orientation="vertical" {size}>
						<Attachment.Media><FileTextIcon /></Attachment.Media>
						<Attachment.Content><Attachment.Title>{size}</Attachment.Title></Attachment.Content>
					</Attachment.Root>
				{/each}
			</div>
		{/snippet}
	</DesignFrame>
</DesignSection>
