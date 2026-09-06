<script lang="ts">
	import { onMount } from 'svelte';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { fetchLearningSoul, type SoulHistory } from './learning-soul-api';

	const text = createPageText({ ko: { title: '현재 원칙', description: '에이전트가 업무에서 지키는 원칙입니다. 현재 내용과 이전 버전을 읽기 전용으로 확인합니다.', current: '현재', values: '지키는 것', boundaries: '하지 않는 것', workingStyle: '일하는 방식', tone: '말투', language: '기본 언어', history: '변경 이력', version: '버전', failed: '원칙을 불러오지 못했습니다.', empty: '아직 적힌 원칙이 없습니다.' }, en: { title: 'Working principles', description: 'The principles the agent follows at work. Review the current and previous versions read-only.', current: 'Current', values: 'Values', boundaries: 'Boundaries', workingStyle: 'Working style', tone: 'Tone', language: 'Default language', history: 'Change history', version: 'Version', failed: 'Could not load working principles.', empty: 'No principles have been written yet.' } });
	let soul = $state<SoulHistory>();
	let errorMessage = $state('');
	const currentDocument = $derived(soul ? readSoul(soul.current.document) : undefined);
	onMount(load);
	async function load(): Promise<void> { try { soul = await fetchLearningSoul(); errorMessage = ''; } catch { errorMessage = text.failed; } }
	function readSoul(value: unknown): Record<string, unknown> { return isRecord(value) ? value : {}; }
	function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value); }
	function readStrings(value: unknown): string[] { return Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === 'string' && entry.trim() !== '') : []; }
	function readString(value: unknown): string { return typeof value === 'string' ? value : ''; }
	function toneLabel(value: string): string { return value === 'polite' ? '정중한 말투' : value; }
	function languageLabel(value: string): string { return value === 'ko' ? '한국어' : value; }
</script>

<Card.Root>
	<Card.Header><Card.Title>{text.title}</Card.Title><Card.Description>{text.description}</Card.Description></Card.Header>
	<Card.Content>
		{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>
		{:else if !soul}<Skeleton class="h-36 w-full" />
		{:else}<div class="grid gap-5"><div class="flex items-center gap-2"><Badge variant="secondary">{text.current}</Badge><span class="text-sm text-muted-foreground">{text.version} {soul.current.version}</span></div><div class="grid gap-4 sm:grid-cols-3">{#each [[text.values, currentDocument?.values], [text.boundaries, currentDocument?.boundaries], [text.workingStyle, currentDocument?.workingStyle]] as section}<section class="grid gap-2"><h3 class="text-sm font-medium">{section[0]}</h3>{#if readStrings(section[1]).length}<ul class="grid gap-1 text-sm">{#each readStrings(section[1]) as line}<li>{line}</li>{/each}</ul>{:else}<p class="text-sm text-muted-foreground">{text.empty}</p>{/if}</section>{/each}</div><div class="flex flex-wrap gap-2">{#if isRecord(currentDocument?.tone)}<Badge variant="outline">{text.tone}: {toneLabel(readString(currentDocument.tone.register))}</Badge>{#each readStrings(currentDocument.tone.traits) as trait}<Badge variant="outline">{trait}</Badge>{/each}{/if}{#if isRecord(currentDocument?.language) && readString(currentDocument.language.default)}<Badge variant="outline">{text.language}: {languageLabel(readString(currentDocument.language.default))}</Badge>{/if}</div><section class="grid gap-3"><h3 class="text-sm font-medium">{text.history}</h3>{#each [...soul.history].reverse() as revision (revision.version)}<div class="border-t py-3"><span class="font-medium">{text.version} {revision.version}</span>{#if revision.reason}<p class="text-sm text-muted-foreground">{revision.reason}</p>{/if}</div>{/each}</section></div>{/if}
	</Card.Content>
</Card.Root>
