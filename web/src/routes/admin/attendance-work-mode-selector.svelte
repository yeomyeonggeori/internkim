<script lang="ts">
	import type { AdminPageText, AttendanceWorkMode } from './admin-types';

	type Props = {
		value: AttendanceWorkMode;
		text: AdminPageText;
		disabled: boolean;
		onChange: (mode: AttendanceWorkMode) => void;
	};

	let { value, text, disabled, onChange }: Props = $props();

	const modes: AttendanceWorkMode[] = ['autonomous', 'flexible', 'fixed'];

	function modeLabel(mode: AttendanceWorkMode): string {
		return text.workSettings[mode];
	}

	function modeDescription(mode: AttendanceWorkMode): string {
		return text.workSettings[`${mode}Description`];
	}
</script>

<div class="grid gap-3 md:grid-cols-3">
	{#each modes as mode (mode)}
		<button
			type="button"
			class={[
				'relative rounded-xl border p-4 text-left transition-colors',
				value === mode
					? 'border-foreground bg-background ring-1 ring-foreground'
					: 'border-border hover:border-foreground/40'
			]}
			aria-pressed={value === mode}
			{disabled}
			onclick={() => onChange(mode)}
		>
			<span class="block text-sm font-semibold">{modeLabel(mode)}</span>
			<span class="mt-2 block text-xs leading-5 text-muted-foreground">
				{modeDescription(mode)}
			</span>
		</button>
	{/each}
</div>
