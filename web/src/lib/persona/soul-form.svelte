<script lang="ts">
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { toneRegisters, type SoulDraft } from './soul-draft';

	export type SoulFormText = {
		valuesLabel: string;
		boundariesLabel: string;
		workingStyleLabel: string;
		linesPlaceholder: string;
		toneRegisterLabel: string;
		toneRegisterUnset: string;
		toneRegisters: Record<'formal' | 'polite' | 'casual', string>;
		traitsLabel: string;
		traitsPlaceholder: string;
		languageLabel: string;
		matchRequesterLabel: string;
	};

	type SoulFormProps = {
		draft: SoulDraft;
		text: SoulFormText;
		disabled?: boolean;
	};

	let { draft = $bindable(), text, disabled = false }: SoulFormProps = $props();
	const fieldID = $props.id();
</script>

<div class="grid gap-5">
	<div class="grid gap-5 md:grid-cols-3">
		<Field.Field>
			<Field.Label for="{fieldID}-values">{text.valuesLabel}</Field.Label>
			<Textarea id="{fieldID}-values" bind:value={draft.valuesText} placeholder={text.linesPlaceholder} {disabled} class="min-h-32" />
		</Field.Field>
		<Field.Field>
			<Field.Label for="{fieldID}-boundaries">{text.boundariesLabel}</Field.Label>
			<Textarea id="{fieldID}-boundaries" bind:value={draft.boundariesText} placeholder={text.linesPlaceholder} {disabled} class="min-h-32" />
		</Field.Field>
		<Field.Field>
			<Field.Label for="{fieldID}-working-style">{text.workingStyleLabel}</Field.Label>
			<Textarea id="{fieldID}-working-style" bind:value={draft.workingStyleText} placeholder={text.linesPlaceholder} {disabled} class="min-h-32" />
		</Field.Field>
	</div>
	<div class="grid gap-5 md:grid-cols-3">
		<Field.Field>
			<Field.Label for="{fieldID}-register">{text.toneRegisterLabel}</Field.Label>
			<select id="{fieldID}-register" bind:value={draft.register} {disabled} class="border-input bg-background h-9 rounded-md border px-3 text-sm">
				<option value="">{text.toneRegisterUnset}</option>
				{#each toneRegisters as register (register)}
					<option value={register}>{text.toneRegisters[register]}</option>
				{/each}
			</select>
		</Field.Field>
		<Field.Field>
			<Field.Label for="{fieldID}-traits">{text.traitsLabel}</Field.Label>
			<Textarea id="{fieldID}-traits" bind:value={draft.traitsText} placeholder={text.traitsPlaceholder} {disabled} class="min-h-20" />
		</Field.Field>
		<Field.Field>
			<Field.Label for="{fieldID}-language">{text.languageLabel}</Field.Label>
			<Input id="{fieldID}-language" bind:value={draft.languageDefault} placeholder="ko" {disabled} />
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={draft.matchRequester} {disabled} />
				{text.matchRequesterLabel}
			</label>
		</Field.Field>
	</div>
</div>
