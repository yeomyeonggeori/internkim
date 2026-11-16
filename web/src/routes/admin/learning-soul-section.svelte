<script lang="ts">
	import { onMount } from 'svelte';
	import * as Card from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { languageAutonym } from '$lib/persona/languages';
	import { speechRegisterName } from '$lib/persona/registers';
	import { companySettingsText } from '../settings/text';
	import { fetchLearningSoul, type SoulHistory, type SoulRevision } from './learning-soul-api';
	import { soulChangesBetween, soulOf, type Soul, type SoulChange } from './soul-document';

	const text = createPageText({
		ko: { title: '일하는 원칙', description: '모든 대화에서 지키는 원칙입니다. 스스로 배우기가 켜져 있으면 김인턴이 끝난 업무를 근거로 고치고, 바뀐 내용은 아래 이력에 남습니다.', values: '지키는 것', boundaries: '하지 않는 것', workingStyle: '일하는 방식', none: '없음', register: '말투', traits: '어조', language: '답하는 언어', matchRequester: '상대가 쓴 언어', matchRequesterAfter: '{language} (상대가 다른 언어로 쓰면 그 언어)', history: '변경 이력', versionCount: '{count}개 버전', version: '버전', current: '지금 쓰는 버전', initial: '처음 설치', reflection: '김인턴이 고침', noChange: '바뀐 내용이 없습니다.', failed: '원칙을 불러오지 못했습니다.' },
		en: { title: 'Working principles', description: 'What the agent holds to in every conversation. With learning on, it rewrites them from finished work, and every change is kept in the history below.', values: 'Holds to', boundaries: 'Never does', workingStyle: 'How it works', none: 'None', register: 'Register', traits: 'Tone', language: 'Answers in', matchRequester: 'the language the person wrote in', matchRequesterAfter: '{language}, or the language the person wrote in', history: 'History', versionCount: '{count} versions', version: 'Version', current: 'In use', initial: 'Installed', reflection: 'Changed by the agent', noChange: 'Nothing changed.', failed: 'Could not load working principles.' }
	});
	const settingsText = createPageText(companySettingsText);
	const columns = ['values', 'boundaries', 'workingStyle'] as const;
	let soul = $state<SoulHistory>();
	let errorMessage = $state('');
	let openVersion = $state<number>();
	const current = $derived(soul ? soulOf(soul.current.document) : undefined);
	const revisions = $derived(soul ? [...soul.history].sort((first, second) => second.version - first.version) : []);

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
	function countText(template: string, count: number): string { return template.replace('{count}', String(count)); }
	function traitName(trait: string): string { const labels: Record<string, string> = settingsText.persona.toneTraits; return labels[trait] || trait; }
	function languageName(document: Soul): string {
		if (!document.languageDefault) return document.matchRequester ? text.matchRequester : '';
		const language = languageAutonym(document.languageDefault);
		return document.matchRequester ? text.matchRequesterAfter.replace('{language}', language) : language;
	}
	function authorOf(revision: SoulRevision): string { return revision.origin === 'initial' ? text.initial : text.reflection; }
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
</script>

<Card.Root>
	<Card.Header><Card.Title>{text.title}</Card.Title><Card.Description>{text.description}</Card.Description></Card.Header>
	<Card.Content>
		{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>
		{:else if !soul || !current}<Skeleton class="h-36 w-full" />
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
				<section class="grid gap-2" aria-label={text.history}>
					<div class="flex items-center justify-between gap-2 text-sm"><h3 class="font-medium">{text.history}</h3><span class="text-muted-foreground">{countText(text.versionCount, revisions.length)}</span></div>
					<div class="rounded-lg border">
						{#each revisions as revision (revision.version)}
							{@const isOpen = openVersion === revision.version}
							<div class="border-t first:border-t-0">
								<button type="button" class="grid w-full grid-cols-[minmax(0,1fr)_auto] gap-x-3 gap-y-0.5 px-3.5 py-3 text-left text-sm hover:bg-muted/50" aria-expanded={isOpen} onclick={() => (openVersion = isOpen ? undefined : revision.version)}>
									<span class="font-medium">{text.version} {revision.version} · {authorOf(revision)}{#if revision.version === soul.current.version}<span class="ml-1.5 text-xs font-normal text-muted-foreground">{text.current}</span>{/if}</span>
									<span class="text-xs text-muted-foreground tabular-nums">{dateOf(revision)}</span>
									{#if revision.reason}<span class="col-span-2 text-muted-foreground">{revision.reason}</span>{/if}
								</button>
								{#if isOpen && revision.origin !== 'initial'}
									{@const changes = changesOf(revision)}
									<div class="px-3.5 pb-3.5">
										{#if changes.length}
											<div class="grid gap-0.5 rounded-md bg-muted/50 px-3 py-2.5 text-sm">
												{#each changes as change, index (index)}
													{#if index === 0 || changes[index - 1].field !== change.field}<span class="mt-1.5 text-xs text-muted-foreground first:mt-0">{changeLabel(change)}</span>{/if}
													{#if change.removed !== undefined}<span class="text-destructive line-through">− {changeValue(change, change.removed)}</span>{/if}
													{#if change.added !== undefined}<span class="text-success">+ {changeValue(change, change.added)}</span>{/if}
												{/each}
											</div>
										{:else}<p class="text-sm text-muted-foreground">{text.noChange}</p>{/if}
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
