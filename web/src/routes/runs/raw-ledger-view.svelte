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

	const hasSeveralRuns = $derived(Math.max(...sections.map((section) => section.turnNumber)) > 1);

	function sectionTitle(section: LedgerSection): string {
		if (section.turnNumber === 0) return text.intakeDecisionTitle;
		if (!hasSeveralRuns) return text.runSection;
		return text.numberedRunSection.replace('{number}', String(section.turnNumber));
	}

	function stepSubject(step: LedgerStep): { label: string; isIdentifier: boolean } {
		if (step.toolName) return { label: step.toolName, isIdentifier: true };
		if (step.action === 'finish') return { label: text.replyStep, isIdentifier: false };
		if (step.action) return { label: step.action, isIdentifier: true };
		return { label: text.stepModelCall, isIdentifier: false };
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
			<h3 class="border-b pb-2 text-base font-semibold">{sectionTitle(section)}</h3>
			{#each section.steps as step (step.number)}
				{@const prominent = step.entries.filter((entry) => isProminentEvent(entry.event))}
				{@const background = step.entries.filter((entry) => !isProminentEvent(entry.event))}
				<div class="flex flex-col gap-1">
					{#if step.number > 0}
						{@const subject = stepSubject(step)}
						<h4 class="flex items-center gap-2 text-sm font-medium">
							<span
								class="flex size-5 shrink-0 items-center justify-center rounded-full border text-xs font-normal text-muted-foreground tabular-nums"
								aria-label={text.stepNumber.replace('{number}', String(step.number))}
							>
								{step.number}
							</span>
							{#if subject.isIdentifier}<code>{subject.label}</code>{:else}{subject.label}{/if}
						</h4>
					{/if}
					<div class={step.number > 0 ? 'ml-2.5 border-l pl-5' : ''}>
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
