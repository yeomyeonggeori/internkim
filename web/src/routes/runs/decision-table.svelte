<script lang="ts">
	import { Meter } from '$lib/components/ui/meter';
	import * as Table from '$lib/components/ui/table';
	import type { DecisionRow } from './llm-calls';
	import type { TasksText } from './text';

	let { rows, text }: { rows: DecisionRow[]; text: TasksText } = $props();

	function formatProbability(probability: number | undefined): string {
		return probability === undefined ? '—' : probability.toFixed(2);
	}

	function isDrawnBelow(row: DecisionRow): boolean {
		return row.draw !== undefined && row.probability !== undefined && row.draw < row.probability;
	}
</script>

<Table.Root>
	<Table.Header>
		<Table.Row>
			<Table.Head>{text.questionColumn}</Table.Head>
			<Table.Head>{text.answerColumn}</Table.Head>
			<Table.Head class="w-48">{text.probabilityColumn}</Table.Head>
			<Table.Head class="w-20 text-right">{text.drawColumn}</Table.Head>
		</Table.Row>
	</Table.Header>
	<Table.Body>
		{#each rows as row (row.question)}
			<Table.Row>
				<Table.Cell class="font-mono text-xs">{row.question}</Table.Cell>
				<Table.Cell class="text-sm">{row.answer}</Table.Cell>
				<Table.Cell>
					<div class="flex items-center gap-2">
						<Meter value={(row.probability ?? 0) * 100} class="h-1.5" aria-label={row.question} />
						<span class="w-10 shrink-0 text-right text-xs tabular-nums">{formatProbability(row.probability)}</span>
					</div>
				</Table.Cell>
				<Table.Cell class="text-right text-xs tabular-nums {isDrawnBelow(row) ? 'font-medium text-foreground' : 'text-muted-foreground'}">
					{formatProbability(row.draw)}
				</Table.Cell>
			</Table.Row>
		{/each}
	</Table.Body>
</Table.Root>
