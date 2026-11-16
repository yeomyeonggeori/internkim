<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Switch } from '$lib/components/ui/switch';
	import { Textarea } from '$lib/components/ui/textarea';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { languageAutonym, replyLanguageOptions } from '$lib/persona/languages';
	import { speechRegisterName, speechRegisterOf, speechRegisters } from '$lib/persona/registers';
	import { toneTraits } from '$lib/persona/user-draft';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import soulSchema from '../../../../internal/admind/persona-schema/soul.schema.json';
	import { companySettingsText } from '../settings/text';
	import { fetchLearningSoul, reviseLearningSoul, SoulRevisionError, type SoulHistory, type SoulRevision } from './learning-soul-api';
	import { soulChangesBetween, soulDocumentOf, soulOf, type Soul, type SoulChange } from './soul-document';

	const text = createPageText({
		ko: { title: '일하는 원칙', description: '모든 대화에서 지키는 원칙입니다. 관리자가 고칠 수 있고, 스스로 배우기가 켜져 있으면 김인턴도 끝난 업무를 근거로 고칩니다. 바뀐 내용은 모두 아래 이력에 남습니다.', edit: '편집', cancel: '취소', save: '저장', saved: '원칙을 저장했습니다.', addLine: '줄 추가', removeLine: '이 줄 지우기', reason: '고친 이유', reasonPlaceholder: '비워 둬도 됩니다', values: '지키는 것', boundaries: '하지 않는 것', workingStyle: '일하는 방식', none: '없음', register: '말투', traits: '어조', language: '답하는 언어', languageDefault: '기본 언어', matchRequesterSwitch: '상대가 다른 언어로 쓰면 그 언어로 답하기', matchRequester: '상대가 쓴 언어', matchRequesterAfter: '{language} (상대가 다른 언어로 쓰면 그 언어)', history: '변경 이력', versionCount: '{count}개 버전', version: '버전', current: '지금 쓰는 버전', initial: '처음 설치', reflection: '김인턴이 고침', administrator: '관리자가 고침', default: '새 기본값', revert: '이 변경 되돌리기', revertConfirm: '버전 {version} 내용으로 되돌리기', revertReason: '버전 {version} 내용으로 되돌림', reverted: '버전 {version} 내용으로 되돌렸습니다.', noChange: '바뀐 내용이 없습니다.', failed: '원칙을 불러오지 못했습니다.', saveFailed: '원칙을 저장하지 못했습니다.', stale: '그사이 원칙이 바뀌었습니다. 다시 불러온 내용을 보고 고쳐 주세요.', forbidden: '관리자만 원칙을 고칠 수 있습니다.' },
		en: { title: 'Working principles', description: 'What the agent holds to in every conversation. An administrator can change them, and with learning on the agent also rewrites them from finished work. Every change is kept in the history below.', edit: 'Edit', cancel: 'Cancel', save: 'Save', saved: 'Saved the principles.', addLine: 'Add line', removeLine: 'Remove this line', reason: 'Why', reasonPlaceholder: 'Optional', values: 'Holds to', boundaries: 'Never does', workingStyle: 'How it works', none: 'None', register: 'Register', traits: 'Tone', language: 'Answers in', languageDefault: 'Default language', matchRequesterSwitch: 'Answer in the language the person wrote in', matchRequester: 'the language the person wrote in', matchRequesterAfter: '{language}, or the language the person wrote in', history: 'History', versionCount: '{count} versions', version: 'Version', current: 'In use', initial: 'Installed', reflection: 'Changed by the agent', administrator: 'Changed by an administrator', default: 'New default', revert: 'Revert this change', revertConfirm: 'Go back to version {version}', revertReason: 'Back to version {version}', reverted: 'Went back to version {version}.', noChange: 'Nothing changed.', failed: 'Could not load working principles.', saveFailed: 'Could not save the principles.', stale: 'The principles changed meanwhile. Check the reloaded version and edit again.', forbidden: 'Only an administrator can change the principles.' }
	});
	const settingsText = createPageText(companySettingsText);
	const columns = ['values', 'boundaries', 'workingStyle'] as const;
	const lineLimit = soulSchema.properties.values.maxItems;
	const traitLimit = soulSchema.properties.tone.properties.traits.maxItems;
	let soul = $state<SoulHistory>();
	let errorMessage = $state('');
	let saveError = $state('');
	let openVersion = $state<number>();
	let draft = $state<Soul>();
	let reason = $state('');
	let isSaving = $state(false);
	let isConfirmingRevert = $state(false);
	const current = $derived(soul ? soulOf(soul.current.document) : undefined);
	const revisions = $derived(soul ? [...soul.history].sort((first, second) => second.version - first.version) : []);
	const previousRevision = $derived(revisions[1]);
	const selectedRegister = $derived(draft ? speechRegisterOf(draft.register) : undefined);
	const languageOptions = $derived(replyLanguageOptions(draft?.languageDefault ?? ''));
	const selectableTraits = $derived(draft ? [...toneTraits, ...draft.traits.filter((trait) => !toneTraits.includes(trait))] : toneTraits);

	onMount(load);
	async function load(): Promise<void> {
		try {
			soul = await fetchLearningSoul();
			openVersion = soul.current.version;
			errorMessage = '';
		} catch {
			errorMessage = text.failed;
		}
	}
	function fill(template: string, name: string, value: number | string): string { return template.replace(`{${name}}`, String(value)); }
	function traitName(trait: string): string { const labels: Record<string, string> = settingsText.persona.toneTraits; return labels[trait] || trait; }
	function languageName(document: Soul): string {
		if (!document.languageDefault) return document.matchRequester ? text.matchRequester : '';
		const language = languageAutonym(document.languageDefault);
		return document.matchRequester ? fill(text.matchRequesterAfter, 'language', language) : language;
	}
	function authorOf(revision: SoulRevision): string {
		const authors: Record<string, string> = { initial: text.initial, reflection: text.reflection, administrator: text.administrator, default: text.default };
		return authors[revision.origin] ?? text.reflection;
	}
	function dateOf(revision: SoulRevision): string {
		const date = new Date(revision.createdAt);
		if (Number.isNaN(date.getTime())) return '';
		return new Intl.DateTimeFormat(currentLocale.value, { month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date);
	}
	function changesOf(revision: SoulRevision): SoulChange[] {
		const previous = soul?.history.filter((candidate) => candidate.version < revision.version).sort((first, second) => second.version - first.version)[0];
		return previous ? soulChangesBetween(soulOf(previous.document), soulOf(revision.document)) : [];
	}
	function changeLabel(change: SoulChange): string {
		if (change.field === 'register') return text.register;
		if (change.field === 'traits') return text.traits;
		if (change.field === 'language') return text.language;
		return text[change.field];
	}
	function changeValue(change: SoulChange, value: string | string[] | Soul): string {
		if (typeof value === 'string') return change.field === 'register' ? speechRegisterName(value, currentLocale.value) : value;
		if (Array.isArray(value)) return value.map(traitName).join(' · ') || text.none;
		return languageName(value) || text.none;
	}
	function startEditing(): void {
		if (!current) return;
		draft = { ...current, values: [...current.values], boundaries: [...current.boundaries], workingStyle: [...current.workingStyle], traits: [...current.traits] };
		reason = '';
		saveError = '';
	}
	function stopEditing(): void {
		draft = undefined;
		saveError = '';
	}
	function addLine(column: (typeof columns)[number]): void {
		if (draft) draft[column] = [...draft[column], ''];
	}
	function removeLine(column: (typeof columns)[number], index: number): void {
		if (draft) draft[column] = draft[column].filter((_, candidate) => candidate !== index);
	}
	function failureMessage(error: unknown): string {
		if (error instanceof SoulRevisionError && error.status === 409) return text.stale;
		if (error instanceof SoulRevisionError && error.status === 403) return text.forbidden;
		return text.saveFailed;
	}
	async function revise(document: unknown, revisionReason: string, successMessage: string): Promise<boolean> {
		if (!soul) return false;
		isSaving = true;
		saveError = '';
		try {
			await reviseLearningSoul(soul.current.version, document, revisionReason);
			await load();
			toast.success(successMessage);
			return true;
		} catch (error) {
			saveError = failureMessage(error);
			if (error instanceof SoulRevisionError && error.status === 409) await load();
			return false;
		} finally {
			isSaving = false;
		}
	}
	async function saveDraft(): Promise<void> {
		if (!draft) return;
		if (await revise(soulDocumentOf(draft), reason.trim(), text.saved)) stopEditing();
	}
	async function revertLatest(): Promise<void> {
		if (!previousRevision) return;
		const version = previousRevision.version;
		if (await revise(previousRevision.document, fill(text.revertReason, 'version', version), fill(text.reverted, 'version', version))) isConfirmingRevert = false;
	}
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.title}</Card.Title>
		<Card.Description>{text.description}</Card.Description>
		{#if current && !draft}<Card.Action><Button variant="outline" size="sm" onclick={startEditing}>{text.edit}</Button></Card.Action>{/if}
	</Card.Header>
	<Card.Content>
		{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>
		{:else if !soul || !current}<Skeleton class="h-36 w-full" />
		{:else if draft}
			<Field.Group>
				<div class="grid gap-5">
					{#each columns as column (column)}
						<Field.Set>
							<Field.Legend variant="label">{text[column]}</Field.Legend>
							<div class="grid gap-2">
								{#each draft[column] as _, index (index)}
									<div class="flex items-start gap-1">
										<Textarea bind:value={draft[column][index]} maxlength={200} rows={1} disabled={isSaving} aria-label={text[column]} class="min-h-9" />
										<Button variant="ghost" size="icon-sm" aria-label={text.removeLine} disabled={isSaving} onclick={() => removeLine(column, index)}><XIcon /></Button>
									</div>
								{/each}
								{#if draft[column].length < lineLimit}<Button variant="ghost" size="sm" class="justify-self-start" disabled={isSaving} onclick={() => addLine(column)}><PlusIcon data-icon="inline-start" />{text.addLine}</Button>{/if}
							</div>
						</Field.Set>
					{/each}
				</div>
				<div class="grid gap-5 md:grid-cols-2">
					<Field.Field>
						<Field.Label for="soul-register">{text.register}</Field.Label>
						<Select.Root type="single" bind:value={draft.register} disabled={isSaving}>
							<Select.Trigger id="soul-register" class="w-full">{speechRegisterName(draft.register, currentLocale.value)}</Select.Trigger>
							<Select.Content><Select.Group>{#each speechRegisters as register (register.value)}<Select.Item value={register.value} label={speechRegisterName(register.value, currentLocale.value)}>{speechRegisterName(register.value, currentLocale.value)}</Select.Item>{/each}</Select.Group></Select.Content>
						</Select.Root>
						{#if selectedRegister}<Field.Description class="grid gap-0.5"><span>{selectedRegister.korean.example}</span><span>{selectedRegister.english.example}</span></Field.Description>{/if}
					</Field.Field>
					<Field.Field>
						<Field.Label>{text.traits}</Field.Label>
						<ToggleGroup.Root type="multiple" bind:value={draft.traits} variant="outline" size="sm" spacing={2} disabled={isSaving} aria-label={text.traits} class="flex-wrap justify-start">
							{#each selectableTraits as trait (trait)}
								<ToggleGroup.Item value={trait} disabled={isSaving || (draft.traits.length >= traitLimit && !draft.traits.includes(trait))}>{traitName(trait)}</ToggleGroup.Item>
							{/each}
						</ToggleGroup.Root>
					</Field.Field>
					<Field.Field>
						<Field.Label for="soul-language">{text.languageDefault}</Field.Label>
						<Select.Root type="single" bind:value={draft.languageDefault} disabled={isSaving}>
							<Select.Trigger id="soul-language" class="w-full">{draft.languageDefault ? languageAutonym(draft.languageDefault) : text.none}</Select.Trigger>
							<Select.Content><Select.Group>{#each languageOptions as option (option.value)}<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>{/each}</Select.Group></Select.Content>
						</Select.Root>
					</Field.Field>
					<Field.Field orientation="horizontal" class="self-end">
						<Switch id="soul-match-requester" bind:checked={draft.matchRequester} disabled={isSaving} />
						<Field.Label for="soul-match-requester">{text.matchRequesterSwitch}</Field.Label>
					</Field.Field>
				</div>
				<Field.Field>
					<Field.Label for="soul-reason">{text.reason}</Field.Label>
					<Input id="soul-reason" bind:value={reason} maxlength={200} placeholder={text.reasonPlaceholder} disabled={isSaving} />
				</Field.Field>
				{#if saveError}<Field.Error>{saveError}</Field.Error>{/if}
				<div class="flex justify-end gap-2">
					<Button variant="outline" disabled={isSaving} onclick={stopEditing}>{text.cancel}</Button>
					<Button disabled={isSaving} onclick={saveDraft}>{text.save}</Button>
				</div>
			</Field.Group>
		{:else}
			<div class="grid gap-5">
				<div class="grid gap-5 sm:grid-cols-3">
					{#each columns as column (column)}
						<section class="grid content-start gap-2">
							<h3 class="text-sm font-medium">{text[column]}</h3>
							{#if current[column].length}<ul class="grid list-disc gap-1.5 pl-4 text-sm">{#each current[column] as line (line)}<li>{line}</li>{/each}</ul>
							{:else}<p class="text-sm text-muted-foreground">{text.none}</p>{/if}
						</section>
					{/each}
				</div>
				<div class="flex flex-wrap gap-x-5 gap-y-1 border-t pt-4 text-sm text-muted-foreground">
					{#if current.register}<span>{text.register} <span class="font-medium text-foreground">{speechRegisterName(current.register, currentLocale.value)}</span></span>{/if}
					{#if current.traits.length}<span>{text.traits} <span class="font-medium text-foreground">{current.traits.map(traitName).join(' · ')}</span></span>{/if}
					{#if languageName(current)}<span>{text.language} <span class="font-medium text-foreground">{languageName(current)}</span></span>{/if}
				</div>
				{#if saveError}<p class="text-sm text-destructive">{saveError}</p>{/if}
				<section class="grid gap-2" aria-label={text.history}>
					<div class="flex items-center justify-between gap-2 text-sm"><h3 class="font-medium">{text.history}</h3><span class="text-muted-foreground">{fill(text.versionCount, 'count', revisions.length)}</span></div>
					<div class="rounded-lg border">
						{#each revisions as revision (revision.version)}
							{@const isOpen = openVersion === revision.version}
							{@const isLatest = revision.version === soul.current.version}
							<div class="border-t first:border-t-0">
								<button type="button" class="grid w-full grid-cols-[minmax(0,1fr)_auto] gap-x-3 gap-y-0.5 px-3.5 py-3 text-left text-sm hover:bg-muted/50" aria-expanded={isOpen} onclick={() => (openVersion = isOpen ? undefined : revision.version)}>
									<span class="font-medium">{text.version} {revision.version} · {authorOf(revision)}{#if isLatest}<span class="ml-1.5 text-xs font-normal text-muted-foreground">{text.current}</span>{/if}</span>
									<span class="text-xs text-muted-foreground tabular-nums">{dateOf(revision)}</span>
									{#if revision.reason}<span class="col-span-2 text-muted-foreground">{revision.reason}</span>{/if}
								</button>
								{#if isOpen && revision.origin !== 'initial'}
									{@const changes = changesOf(revision)}
									<div class="grid gap-2.5 px-3.5 pb-3.5">
										{#if changes.length}
											<div class="grid gap-0.5 rounded-md bg-muted/50 px-3 py-2.5 text-sm">
												{#each changes as change, index (index)}
													{#if index === 0 || changes[index - 1].field !== change.field}<span class="mt-1.5 text-xs text-muted-foreground first:mt-0">{changeLabel(change)}</span>{/if}
													{#if change.removed !== undefined}<span class="text-destructive line-through">− {changeValue(change, change.removed)}</span>{/if}
													{#if change.added !== undefined}<span class="text-success">+ {changeValue(change, change.added)}</span>{/if}
												{/each}
											</div>
										{:else}<p class="text-sm text-muted-foreground">{text.noChange}</p>{/if}
										{#if isLatest && previousRevision}
											<div class="flex flex-wrap justify-end gap-2">
												{#if isConfirmingRevert}
													<Button variant="outline" size="sm" disabled={isSaving} onclick={() => (isConfirmingRevert = false)}>{text.cancel}</Button>
													<Button variant="destructive" size="sm" disabled={isSaving} onclick={revertLatest}>{fill(text.revertConfirm, 'version', previousRevision.version)}</Button>
												{:else}
													<Button variant="outline" size="sm" onclick={() => (isConfirmingRevert = true)}>{text.revert}</Button>
												{/if}
											</div>
										{/if}
									</div>
								{/if}
							</div>
						{/each}
					</div>
				</section>
			</div>
		{/if}
	</Card.Content>
</Card.Root>
