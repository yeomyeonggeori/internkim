<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import { MediaQuery } from 'svelte/reactivity';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import type { AttendanceKind } from '../attendance-context.svelte';
	import { attendanceText } from '../text';
	import { getHandWrittenState } from './hand-written-state.svelte';
	import type { HandWrittenRecord } from './hand-written-records';

	const text = createPageText(attendanceText);
	const handWritten = getHandWrittenState();
	const isMobile = new MediaQuery('(max-width: 639px)');

	function kindLabel(kind: AttendanceKind): string {
		return kind === 'clock_in' ? text.clockIn : text.clockOut;
	}

	function wasMoved(record: HandWrittenRecord): boolean {
		return record.originalDate !== null && record.originalTime !== null;
	}

	function undoTitle(record: HandWrittenRecord): string {
		return wasMoved(record) ? text.handWritten.undoMovedTitle : text.handWritten.undoAddedTitle;
	}

	function undoDescription(record: HandWrittenRecord): string {
		const moved = wasMoved(record);
		const template = moved
			? text.handWritten.undoMovedDescriptionTemplate
			: text.handWritten.undoAddedDescriptionTemplate;
		return template
			.replace('{person}', record.person)
			.replace('{kind}', kindLabel(record.kind))
			.replace('{date}', moved ? (record.originalDate ?? '') : record.date)
			.replace('{time}', moved ? (record.originalTime ?? '') : record.time);
	}

	function undoReason(record: HandWrittenRecord): string {
		return wasMoved(record) ? text.handWritten.undoReasonMoved : text.handWritten.undoReasonAdded;
	}
</script>

{#snippet undoControl(record: HandWrittenRecord)}
	<AlertDialog.Root>
		<AlertDialog.Trigger
			class={buttonVariants({ variant: 'outline', size: 'sm' })}
			disabled={handWritten.undoingEventID !== ''}
			data-testid="hand-written-undo"
		>
			{text.handWritten.undo}
		</AlertDialog.Trigger>
		<AlertDialog.Content data-testid="hand-written-undo-dialog">
			<AlertDialog.Header>
				<AlertDialog.Title>{undoTitle(record)}</AlertDialog.Title>
				<AlertDialog.Description>
					{undoDescription(record)}
				</AlertDialog.Description>
			</AlertDialog.Header>
			<AlertDialog.Footer>
				<AlertDialog.Cancel>{text.cancel}</AlertDialog.Cancel>
				<AlertDialog.Action
					data-testid="hand-written-undo-confirm"
					onclick={() => void handWritten.undo(record, undoReason(record))}
				>
					{text.handWritten.undo}
				</AlertDialog.Action>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>
{/snippet}

<section class="mx-auto min-h-0 w-full max-w-7xl space-y-5" data-testid="hand-written-view">
	<header class="flex flex-wrap items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{text.handWritten.title}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{text.handWritten.description}</p>
			<p class="mt-1 text-xs text-muted-foreground" data-testid="hand-written-day-range">
				{handWritten.dayRange.from} — {handWritten.dayRange.to}
			</p>
		</div>
		<Button
			variant={handWritten.isLoading ? 'secondary' : 'ghost'}
			size="icon"
			aria-label={text.handWritten.refresh}
			aria-busy={handWritten.isLoading}
			onclick={() => void handWritten.load()}
			disabled={handWritten.isLoading}
			data-testid="hand-written-refresh"
		>
			<RefreshCwIcon class={handWritten.isLoading ? 'animate-spin text-primary' : ''} />
		</Button>
	</header>

	{#if handWritten.errorMessage}
		<p class="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
			{handWritten.errorMessage}
		</p>
	{/if}

	<Card.Root>
		<Card.Content class="overflow-x-auto pt-6">
			{#if handWritten.records.length === 0}
				<p class="py-6 text-sm text-muted-foreground" data-testid="hand-written-empty">
					{text.handWritten.empty}
				</p>
			{:else if isMobile.current}
				<ul class="divide-y">
					{#each handWritten.records as record (record.eventID)}
						<li class="grid min-w-0 gap-3 py-4" data-testid="hand-written-row" data-event-id={record.eventID}>
							<div class="flex items-start justify-between gap-3"><div class="min-w-0"><p class="break-words font-medium">{record.person}</p><p class="text-sm text-muted-foreground">{kindLabel(record.kind)}</p></div>{@render undoControl(record)}</div>
							<dl class="grid gap-2 text-sm">
								<div><dt class="text-xs text-muted-foreground">{text.handWritten.now}</dt><dd data-testid="hand-written-now">{record.date} {record.time}</dd></div>
								<div><dt class="text-xs text-muted-foreground">{text.handWritten.before}</dt><dd data-testid="hand-written-before">{wasMoved(record) ? `${record.originalDate} ${record.originalTime}` : text.handWritten.neverMoved}</dd></div>
								<div><dt class="text-xs text-muted-foreground">{text.handWritten.reason}</dt><dd class="break-words">{record.reason || text.handWritten.noReason}</dd></div>
							</dl>
						</li>
					{/each}
				</ul>
			{:else}
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>{text.handWritten.person}</Table.Head>
							<Table.Head>{text.handWritten.kind}</Table.Head>
							<Table.Head>{text.handWritten.now}</Table.Head>
							<Table.Head>{text.handWritten.before}</Table.Head>
							<Table.Head>{text.handWritten.reason}</Table.Head>
							<Table.Head class="text-right">{text.handWritten.undo}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each handWritten.records as record (record.eventID)}
							<Table.Row data-testid="hand-written-row" data-event-id={record.eventID}>
								<Table.Cell class="font-medium">{record.person}</Table.Cell>
								<Table.Cell>{kindLabel(record.kind)}</Table.Cell>
								<Table.Cell data-testid="hand-written-now">
									{record.date}
									{record.time}
								</Table.Cell>
								<Table.Cell class="text-muted-foreground" data-testid="hand-written-before">
									{wasMoved(record)
										? `${record.originalDate} ${record.originalTime}`
										: text.handWritten.neverMoved}
								</Table.Cell>
								<Table.Cell class="text-muted-foreground">
									{record.reason || text.handWritten.noReason}
								</Table.Cell>
								<Table.Cell class="text-right">
									{@render undoControl(record)}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			{/if}
		</Card.Content>
	</Card.Root>
</section>
