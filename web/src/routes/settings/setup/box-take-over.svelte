<script lang="ts">
	import * as Field from '$lib/components/ui/field';
	import * as Select from '$lib/components/ui/select';
	import { Switch } from '$lib/components/ui/switch';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { CompanyBoxes, TakeOver } from '$lib/company/box';
	import { keepTakeOver } from './host-setup-client';
	import { hostSetupText } from './text';

	let { takeOver, onKept }: { takeOver: TakeOver; onKept: (boxes: CompanyBoxes) => void } = $props();

	const text = createPageText(hostSetupText);
	const lapseChoices = [300, 900, 3600, 14400];
	let isSaving = $state(false);
	let errorMessage = $state('');

	function lapseLabel(seconds: number): string {
		return seconds < 3600
			? text.takeOverMinutes.replace('{minutes}', String(seconds / 60))
			: text.takeOverHours.replace('{hours}', String(seconds / 3600));
	}

	async function keep(next: TakeOver) {
		isSaving = true;
		errorMessage = '';
		try {
			onKept(await keepTakeOver(next));
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		} finally {
			isSaving = false;
		}
	}
</script>

<Field.Field orientation="horizontal">
	<Field.Content>
		<Field.Label for="box-take-over">{text.takeOver}</Field.Label>
		<Field.Description>{text.takeOverDescription}</Field.Description>
	</Field.Content>
	<Switch
		id="box-take-over"
		checked={takeOver.isAutomatic}
		disabled={isSaving}
		onCheckedChange={(isAutomatic) => keep({ ...takeOver, isAutomatic })}
	/>
</Field.Field>
{#if takeOver.isAutomatic}
	<Field.Field>
		<Field.Label>{text.takeOverAfter}</Field.Label>
		<Select.Root
			type="single"
			value={String(takeOver.lapseSeconds)}
			disabled={isSaving}
			onValueChange={(value) => keep({ ...takeOver, lapseSeconds: Number(value) })}
		>
			<Select.Trigger class="w-40">{lapseLabel(takeOver.lapseSeconds)}</Select.Trigger>
			<Select.Content>
				{#each lapseChoices as seconds (seconds)}
					<Select.Item value={String(seconds)} label={lapseLabel(seconds)} />
				{/each}
			</Select.Content>
		</Select.Root>
	</Field.Field>
{/if}
{#if errorMessage}<Field.Error>{errorMessage}</Field.Error>{/if}
