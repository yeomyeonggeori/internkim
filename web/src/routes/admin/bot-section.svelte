<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import { apiErrorMessage, fetchIdentity, fetchSoul, updateIdentity, updateSoul } from './admin-api';
	import type { AdminPageText, AgentIdentity, AgentSoul, AgentToneRegister } from './admin-types';

	type BotSectionProps = {
		adminBaseURL: string;
		isDeviceReachable: boolean;
		text: AdminPageText;
	};

	let { adminBaseURL, isDeviceReachable, text }: BotSectionProps = $props();

	const toneRegisters: AgentToneRegister[] = ['formal', 'polite', 'casual'];

	let loadedAdminBaseURL = $state('');
	let identity = $state<AgentIdentity>(defaultIdentity());
	let aliasesText = $state('');
	let valuesText = $state('');
	let boundariesText = $state('');
	let workingStyleText = $state('');
	let traitsText = $state('');
	let toneRegister = $state<AgentToneRegister | ''>('');
	let languageDefault = $state('ko');
	let matchRequester = $state(true);
	let errorMessage = $state('');
	let isLoading = $state(false);
	let isSaving = $state(false);

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		loadPersona();
	});

	function defaultIdentity(): AgentIdentity {
		return { schemaVersion: 1, name: '김인턴', englishName: 'Intern Kim', handle: 'internkim', aliases: [] };
	}

	function linesOf(values: string[] | undefined): string {
		return (values ?? []).join('\n');
	}

	function listFrom(textValue: string): string[] {
		return textValue
			.split('\n')
			.map((line) => line.trim())
			.filter(Boolean);
	}

	function applyIdentity(loaded: AgentIdentity) {
		identity = { ...defaultIdentity(), ...loaded, name: '김인턴', handle: 'internkim' };
		aliasesText = linesOf(loaded.aliases);
	}

	function applySoul(loaded: AgentSoul) {
		valuesText = linesOf(loaded.values);
		boundariesText = linesOf(loaded.boundaries);
		workingStyleText = linesOf(loaded.workingStyle);
		traitsText = linesOf(loaded.tone?.traits);
		toneRegister = loaded.tone?.register ?? '';
		languageDefault = loaded.language?.default ?? '';
		matchRequester = loaded.language?.matchRequester ?? false;
	}

	function soulFromForm(): AgentSoul {
		const soul: AgentSoul = { schemaVersion: 1 };
		const values = listFrom(valuesText);
		const boundaries = listFrom(boundariesText);
		const workingStyle = listFrom(workingStyleText);
		const traits = listFrom(traitsText);
		if (values.length) soul.values = values;
		if (boundaries.length) soul.boundaries = boundaries;
		if (workingStyle.length) soul.workingStyle = workingStyle;
		if (toneRegister || traits.length) soul.tone = { ...(toneRegister ? { register: toneRegister } : {}), ...(traits.length ? { traits } : {}) };
		if (languageDefault.trim() || matchRequester) {
			soul.language = { ...(languageDefault.trim() ? { default: languageDefault.trim() } : {}), matchRequester };
		}
		return soul;
	}

	function identityFromForm(): AgentIdentity {
		const aliases = listFrom(aliasesText);
		return {
			schemaVersion: 1,
			name: '김인턴',
			handle: 'internkim',
			...(identity.englishName?.trim() ? { englishName: identity.englishName.trim() } : {}),
			...(aliases.length ? { aliases } : {}),
			...(identity.role?.trim() ? { role: identity.role.trim() } : {}),
			...(identity.creature?.trim() ? { creature: identity.creature.trim() } : {}),
			...(identity.emoji?.trim() ? { emoji: identity.emoji.trim() } : {}),
			...(identity.introduction?.trim() ? { introduction: identity.introduction.trim() } : {})
		};
	}

	async function loadPersona() {
		if (!adminBaseURL) return;
		isLoading = true;
		errorMessage = '';
		try {
			applyIdentity(await fetchIdentity(adminBaseURL, text.messages.botProfileLoadError));
			applySoul(await fetchSoul(adminBaseURL, text.messages.botProfileLoadError));
		} catch {
			errorMessage = text.messages.botProfileLoadError;
		} finally {
			isLoading = false;
		}
	}

	async function savePersona() {
		if (!adminBaseURL) return;
		isSaving = true;
		errorMessage = '';
		try {
			applyIdentity(await updateIdentity(adminBaseURL, identityFromForm(), text.messages.botProfileSaveError));
			applySoul(await updateSoul(adminBaseURL, soulFromForm(), text.messages.botProfileSaveError));
		} catch (error) {
			errorMessage = apiErrorMessage(error, text.messages.botProfileSaveError);
		} finally {
			isSaving = false;
		}
	}
</script>

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.bot.title}</Card.Title>
		<Card.Description>{text.bot.description}</Card.Description>
		<Card.Action>
			<Badge variant="outline" class="font-mono">@{identity.handle}</Badge>
		</Card.Action>
	</Card.Header>
	<Card.Content class="grid gap-8">
		<Field.Group>
			<Field.Legend>{text.bot.identityTitle}</Field.Legend>
			<div class="grid gap-5 md:grid-cols-2">
				<Field.Field>
					<Field.Label for="bot-name">{text.bot.nameLabel}</Field.Label>
					<Input id="bot-name" value={identity.name} readonly />
					<Field.Description>{text.bot.nameFixedNotice}</Field.Description>
				</Field.Field>
				<Field.Field>
					<Field.Label for="bot-english-name">{text.bot.englishNameLabel}</Field.Label>
					<Input id="bot-english-name" bind:value={identity.englishName} disabled={isLoading} />
				</Field.Field>
			</div>
			<div class="grid gap-5 md:grid-cols-2">
				<Field.Field>
					<Field.Label for="bot-role">{text.bot.roleLabel}</Field.Label>
					<Input id="bot-role" bind:value={identity.role} placeholder={text.bot.rolePlaceholder} disabled={isLoading} />
				</Field.Field>
				<Field.Field>
					<Field.Label for="bot-emoji">{text.bot.emojiLabel}</Field.Label>
					<Input id="bot-emoji" bind:value={identity.emoji} placeholder="🐱" disabled={isLoading} />
				</Field.Field>
			</div>
			<Field.Field>
				<Field.Label for="bot-creature">{text.bot.creatureLabel}</Field.Label>
				<Input id="bot-creature" bind:value={identity.creature} placeholder={text.bot.creaturePlaceholder} disabled={isLoading} />
			</Field.Field>
			<div class="grid gap-5 md:grid-cols-2">
				<Field.Field>
					<Field.Label for="bot-aliases">{text.bot.aliasesLabel}</Field.Label>
					<Textarea id="bot-aliases" bind:value={aliasesText} placeholder={text.bot.aliasesPlaceholder} disabled={isLoading} class="min-h-24" />
				</Field.Field>
				<Field.Field>
					<Field.Label for="bot-introduction">{text.bot.introductionLabel}</Field.Label>
					<Textarea id="bot-introduction" bind:value={identity.introduction} placeholder={text.bot.introductionPlaceholder} disabled={isLoading} class="min-h-24" />
				</Field.Field>
			</div>
		</Field.Group>
		<Field.Group>
			<Field.Legend>{text.bot.soulTitle}</Field.Legend>
			<Field.Description>{text.bot.soulDescription}</Field.Description>
			<div class="grid gap-5 md:grid-cols-3">
				<Field.Field>
					<Field.Label for="bot-values">{text.bot.valuesLabel}</Field.Label>
					<Textarea id="bot-values" bind:value={valuesText} placeholder={text.bot.linesPlaceholder} disabled={isLoading} class="min-h-32" />
				</Field.Field>
				<Field.Field>
					<Field.Label for="bot-boundaries">{text.bot.boundariesLabel}</Field.Label>
					<Textarea id="bot-boundaries" bind:value={boundariesText} placeholder={text.bot.linesPlaceholder} disabled={isLoading} class="min-h-32" />
				</Field.Field>
				<Field.Field>
					<Field.Label for="bot-working-style">{text.bot.workingStyleLabel}</Field.Label>
					<Textarea id="bot-working-style" bind:value={workingStyleText} placeholder={text.bot.linesPlaceholder} disabled={isLoading} class="min-h-32" />
				</Field.Field>
			</div>
			<div class="grid gap-5 md:grid-cols-3">
				<Field.Field>
					<Field.Label for="bot-tone-register">{text.bot.toneRegisterLabel}</Field.Label>
					<select id="bot-tone-register" bind:value={toneRegister} disabled={isLoading} class="border-input bg-background h-9 rounded-md border px-3 text-sm">
						<option value="">{text.bot.toneRegisterUnset}</option>
						{#each toneRegisters as register (register)}
							<option value={register}>{text.bot.toneRegisters[register]}</option>
						{/each}
					</select>
				</Field.Field>
				<Field.Field>
					<Field.Label for="bot-traits">{text.bot.traitsLabel}</Field.Label>
					<Textarea id="bot-traits" bind:value={traitsText} placeholder={text.bot.traitsPlaceholder} disabled={isLoading} class="min-h-20" />
				</Field.Field>
				<Field.Field>
					<Field.Label for="bot-language">{text.bot.languageLabel}</Field.Label>
					<Input id="bot-language" bind:value={languageDefault} placeholder="ko" disabled={isLoading} />
					<label class="flex items-center gap-2 text-sm">
						<input type="checkbox" bind:checked={matchRequester} disabled={isLoading} />
						{text.bot.matchRequesterLabel}
					</label>
				</Field.Field>
			</div>
		</Field.Group>
		{#if errorMessage}
			<Field.Error>{errorMessage}</Field.Error>
		{/if}
	</Card.Content>
	<Card.Footer class="justify-end">
		<Button disabled={!isDeviceReachable || isSaving || isLoading} onclick={savePersona}>
			{#if isSaving}
				<LoaderIcon class="animate-spin" />
			{/if}
			{text.bot.save}
		</Button>
	</Card.Footer>
</Card.Root>
