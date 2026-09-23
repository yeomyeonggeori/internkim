<script lang="ts">
	import * as Accordion from '$lib/components/ui/accordion';
	import LLMCallEvent from './llm-call-event.svelte';
	import { parseJSON, readLLMCallRecord, readRecord } from './llm-calls';
	import RawDocument from './raw-document.svelte';
	import { eventTitle, isProminentEvent, type LedgerEntry, type LedgerSection, type LedgerStep } from './raw-ledger';
	import { eventLane, formatEventBody } from './runs-api';
	import { formatEventClock } from './runs-view';
	import type { TasksText } from './text';
	import TimelineEvent from './timeline-event.svelte';
	import TurnInputEvent from './turn-input-event.svelte';

	let { sections, text }: { sections: LedgerSection[]; text: TasksText } = $props();

	let openValues = $state<string[]>([]);

	function valueOf(entry: LedgerEntry): string {
		return entry.event.id ?? `${entry.event.name}-${entry.index}`;
	}

	function sectionTitle(section: LedgerSection): string {
		return section.turnNumber === 0 ? text.intakeSection : text.turnSection.replace('{number}', String(section.turnNumber));
	}

	function stepTitle(step: LedgerStep): string {
		const subject = step.toolName ?? (step.action === 'finish' ? text.replyStep : step.action || text.stepModelCall);
		return `${step.number}. ${subject}`;
	}

	function isFailedEntry(entry: LedgerEntry): boolean {
		return eventLane(entry.event.name) === 'failure' || readRecord(parseJSON(entry.event.body))?.failure !== undefined;
	}

	function backgroundDocument(entries: LedgerEntry[]): string {
		return entries.map((entry) => `${formatEventClock(entry.event.createdAt)}  ${entry.event.name}\n${formatEventBody(entry.event.body)}`).join('\n\n');
	}
</script>

{#snippet prominentEntry(entry: LedgerEntry)}
	{@const value = valueOf(entry)}
	{@const isOpen = openValues.includes(value)}
	{@const llmCallRecord = entry.event.name === 'llm.call' ? readLLMCallRecord(entry.event.body) : undefined}
	{#if llmCallRecord}
		<LLMCallEvent {value} {isOpen} createdAt={entry.event.createdAt} llmCallID={entry.event.id} record={llmCallRecord} {text} />
	{:else if entry.event.name === 'task.turn_input' && entry.event.id}
		<TurnInputEvent {value} {isOpen} createdAt={entry.event.createdAt} taskEventID={entry.event.id} {text} />
	{:else}
		<TimelineEvent {value} {isOpen} title={eventTitle(entry.event)} createdAt={entry.event.createdAt} isFailed={isFailedEntry(entry)}>
			<RawDocument document={formatEventBody(entry.event.body)} />
		</TimelineEvent>
	{/if}
{/snippet}

{#snippet backgroundEntries(entries: LedgerEntry[])}
	{@const value = `background-${entries[0].index}`}
	<Accordion.Item {value}>
		<Accordion.Trigger class="py-1.5">
			<span class="flex-1 text-xs font-normal text-muted-foreground">{text.backgroundEvents.replace('{count}', String(entries.length))}</span>
		</Accordion.Trigger>
		<Accordion.Content>
			{#if openValues.includes(value)}
				<RawDocument document={backgroundDocument(entries)} />
			{/if}
		</Accordion.Content>
	</Accordion.Item>
{/snippet}

<Accordion.Root type="multiple" bind:value={openValues} class="flex flex-col gap-8">
	{#each sections as section (section.turnNumber)}
		<section class="flex flex-col gap-4">
			<h3 class="flex items-baseline gap-2 border-b pb-2 text-base font-semibold">
				{sectionTitle(section)}
				<span class="text-xs font-normal text-muted-foreground tabular-nums">{formatEventClock(section.steps[0]?.entries[0]?.event.createdAt)}</span>
			</h3>
			{#each section.steps as step (step.number)}
				{@const prominent = step.entries.filter((entry) => isProminentEvent(entry.event))}
				{@const background = step.entries.filter((entry) => !isProminentEvent(entry.event))}
				<div class="flex flex-col gap-1">
					{#if step.number > 0}
						<h4 class="text-sm font-medium">{stepTitle(step)}</h4>
					{/if}
					<div class={step.number > 0 ? 'border-l pl-4' : ''}>
						{#each prominent as entry (entry.index)}
							{@render prominentEntry(entry)}
						{/each}
						{#if background.length > 0}
							{@render backgroundEntries(background)}
						{/if}
					</div>
				</div>
			{/each}
		</section>
	{/each}
</Accordion.Root>
