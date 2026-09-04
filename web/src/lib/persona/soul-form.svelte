<script lang="ts">
	import * as Field from '$lib/components/ui/field';
	import * as Select from '$lib/components/ui/select';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import { Textarea } from '$lib/components/ui/textarea';
	import { toneRegisters, toneTraitLimit, toneTraits, type SoulDraft } from './soul-draft';
	import { replyLanguageOptions } from './languages';

	export type SoulFormText = {
		valuesLabel: string;
		boundariesLabel: string;
		workingStyleLabel: string;
		linesPlaceholder: string;
		toneRegisterLabel: string;
		toneRegisters: Record<'formal' | 'polite' | 'casual', string>;
		traitsLabel: string;
		toneTraits: Record<string, string>;
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

	const selectableTraits = $derived([...toneTraits, ...draft.traits.filter((trait) => !toneTraits.includes(trait))]);
	const isTraitLimitReached = $derived(draft.traits.length >= toneTraitLimit);
	const languageOptions = $derived(replyLanguageOptions(draft.languageDefault));
	const selectedLanguageLabel = $derived(languageOptions.find((option) => option.value === draft.languageDefault)?.label ?? '');
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
	<div class="grid gap-5 md:grid-cols-2">
		<Field.Field>
			<Field.Label for="{fieldID}-register">{text.toneRegisterLabel}</Field.Label>
			<Select.Root type="single" bind:value={draft.register} {disabled}>
				<Select.Trigger id="{fieldID}-register" class="w-full">
					{text.toneRegisters[draft.register]}
				</Select.Trigger>
				<Select.Content>
					{#each toneRegisters as register (register)}
						<Select.Item value={register} label={text.toneRegisters[register]}>{text.toneRegisters[register]}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</Field.Field>
		<Field.Field>
			<Field.Label for="{fieldID}-language">{text.languageLabel}</Field.Label>
			<Select.Root type="single" bind:value={draft.languageDefault} {disabled}>
				<Select.Trigger id="{fieldID}-language" class="w-full">
					{selectedLanguageLabel}
				</Select.Trigger>
				<Select.Content>
					{#each languageOptions as option (option.value)}
						<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={draft.matchRequester} {disabled} />
				{text.matchRequesterLabel}
			</label>
		</Field.Field>
	</div>
	<Field.Field>
		<Field.Label>{text.traitsLabel}</Field.Label>
		<ToggleGroup.Root type="multiple" bind:value={draft.traits} variant="outline" size="sm" {disabled} class="flex-wrap justify-start">
			{#each selectableTraits as trait (trait)}
				<ToggleGroup.Item value={trait} disabled={disabled || (isTraitLimitReached && !draft.traits.includes(trait))}>
					{text.toneTraits[trait] || trait}
				</ToggleGroup.Item>
			{/each}
		</ToggleGroup.Root>
	</Field.Field>
</div>
