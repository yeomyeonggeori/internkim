<script lang="ts">
	import type { AttendancePresence } from '../attendance-context.svelte';
	import type { PresenceFilter, PresencePerson } from './status-board-model';
	import { presenceDotClass } from './status-board-model';

	type PresenceFilterOption = {
		value: PresenceFilter;
		label: string;
	};

	type Props = {
		options: PresenceFilterOption[];
		selected: PresenceFilter;
		people: PresencePerson[];
		filteredPeople: PresencePerson[];
		presenceCounts: Record<AttendancePresence, number>;
		label: string;
		onSelect: (value: PresenceFilter) => void;
	};

	let {
		options,
		selected,
		people,
		filteredPeople,
		presenceCounts,
		label,
		onSelect
	}: Props = $props();

	function optionCount(value: PresenceFilter): number {
		return value === 'all' ? people.length : presenceCounts[value];
	}

	function buttonClass(value: PresenceFilter): string {
		const base = 'inline-flex h-8 items-center gap-1.5 rounded-full border px-3 text-xs font-medium transition';
		if (value === selected) {
			return `${base} border-foreground bg-foreground text-background`;
		}
		return `${base} border-border bg-background text-muted-foreground hover:border-foreground/40 hover:text-foreground`;
	}
</script>

{#if people.length > 0}
	<div class="border-t px-6 py-3">
		<div class="flex flex-wrap items-center gap-2">
			<span class="text-xs font-medium text-muted-foreground">{label}</span>
			{#each options as option (option.value)}
				<button
					type="button"
					class={buttonClass(option.value)}
					aria-pressed={selected === option.value}
					onclick={() => onSelect(option.value)}
				>
					{#if option.value !== 'all'}
						<span class={`size-2 rounded-full ${presenceDotClass(option.value)}`}></span>
					{/if}
					{option.label} {optionCount(option.value)}
				</button>
			{/each}
		</div>
		<div class="mt-2 flex flex-wrap gap-1.5">
			{#each filteredPeople as person (person.email)}
				<span class="inline-flex items-center gap-1.5 rounded-full bg-muted px-2.5 py-1 text-xs text-muted-foreground">
					<span class={`size-2 rounded-full ${presenceDotClass(person.presence)}`}></span>
					{person.displayName}
				</span>
			{/each}
		</div>
	</div>
{/if}
