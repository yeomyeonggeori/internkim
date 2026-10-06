<script lang="ts">
	import { onMount } from 'svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Empty from '$lib/components/ui/empty';
	import { Separator } from '$lib/components/ui/separator';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Input } from '$lib/components/ui/input';
	import * as Field from '$lib/components/ui/field';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import RefreshIcon from '@lucide/svelte/icons/refresh-cw';
	import { fetchLearnedSkills, fetchLearningSettings, fetchLearnedSkillHistory, updateLearningSettings, updateLearnedSkill, type LearnedSkill, type LearnedSkillInventory } from './learned-skills-api';
	import { isSkillReadAccessDenied } from './skill-read-error';

	const text = createPageText({
		ko: { title: '배운 절차', description: '실제 업무 근거로 남은 재사용 절차입니다.', refresh: '새로고침', empty: '아직 배운 절차가 없습니다.', failed: '배운 절차를 불러오지 못했습니다.', malformed: '응답에 표시할 수 없는 절차가 포함되어 있습니다.', purpose: '언제 쓰는지', evidence: '근거', version: '버전', active: '사용 중', retired: '보관됨', protect: '보호', unprotect: '보호 해제', retire: '사용 중지', restore: '복원', history: '변경 이력', reason: '배운 이유', verification: '검증', evidenceReviewed: '근거 검토 완료', companyAudience: '회사 전체', personalAudience: '개인', scopedAudience: '지정 범위', learning: '학습', enabled: '학습 켜기', disabled: '학습 꺼짐', limit: '활성 절차 한도', save: '저장' },
		en: { title: 'Learned procedures', description: 'Reusable procedures retained from grounded work evidence.', refresh: 'Refresh', empty: 'No learned procedures yet.', failed: 'Could not load learned procedures.', malformed: 'Some returned procedures could not be displayed.', purpose: 'When it applies', evidence: 'Evidence', version: 'Version', active: 'Active', retired: 'Retired', protect: 'Protect', unprotect: 'Unprotect', retire: 'Retire', restore: 'Restore', history: 'History', reason: 'Why it was learned', verification: 'Verification', evidenceReviewed: 'Evidence reviewed', companyAudience: 'Company-wide', personalAudience: 'Personal', scopedAudience: 'Specific scope', learning: 'Learning enabled', enabled: 'Learning enabled', disabled: 'Learning disabled', limit: 'Active procedure limit', save: 'Save' }
	});
	let inventory = $state<LearnedSkillInventory | undefined>();
	let errorMessage = $state('');
	let isLoading = $state(true);
	let pendingID = $state('');
	let selectedID = $state('');
	let learningSettings = $state<{ enabled: boolean; activeLimit: number }>();
	let history = $state<LearnedSkill[]>([]);
	let authorityGeneration = 0;
	let loadSequence = 0;
	let historySequence = 0;
	let mutationSequence = 0;
	const selectedSkill = $derived(inventory?.skills.find((skill) => skill.id === selectedID));
	function verificationLabel(value: string): string { return value === 'evidence-reviewed' ? text.evidenceReviewed : value; }
	function audienceLabel(value: string): string { return value === 'company' ? text.companyAudience : value.startsWith('person:') ? text.personalAudience : text.scopedAudience; }
	function reportReadFailure(error: unknown): void {
		if (isSkillReadAccessDenied(error)) {
			authorityGeneration += 1;
			loadSequence += 1;
			historySequence += 1;
			mutationSequence += 1;
			isLoading = false;
			pendingID = '';
			inventory = undefined;
			learningSettings = undefined;
			history = [];
			selectedID = '';
		}
		errorMessage = text.failed;
	}
	async function load(): Promise<void> {
		const sequence = ++loadSequence;
		const generation = authorityGeneration;
		const isCurrent = () => sequence === loadSequence && generation === authorityGeneration;
		function guardReadFailure(error: unknown): never {
			// A sibling may reject after Promise.all has already reported a transient failure.
			if (isCurrent() && isSkillReadAccessDenied(error)) reportReadFailure(error);
			throw error;
		}
		isLoading = true;
		errorMessage = '';
		try {
			const [nextInventory, nextSettings] = await Promise.all([
				fetchLearnedSkills().catch(guardReadFailure),
				fetchLearningSettings().catch(guardReadFailure)
			]);
			if (!isCurrent()) return;
			inventory = nextInventory;
			learningSettings = nextSettings;
		} catch (error) {
			if (isCurrent()) reportReadFailure(error);
		} finally {
			if (isCurrent()) isLoading = false;
		}
	}
	async function showHistory(skillID: string): Promise<void> {
		const sequence = ++historySequence;
		const generation = authorityGeneration;
		try {
			const nextHistory = await fetchLearnedSkillHistory(skillID);
			if (sequence !== historySequence || generation !== authorityGeneration) return;
			history = nextHistory;
			selectedID = skillID;
		} catch (error) {
			if (sequence === historySequence && generation === authorityGeneration) reportReadFailure(error);
		}
	}
	async function saveSettings(): Promise<void> { if (!learningSettings) return; try { await updateLearningSettings(learningSettings); } catch { errorMessage = text.failed; } }
	async function change(skillID: string, action: 'protect' | 'retire' | 'restore', protectedValue = action === 'protect'): Promise<void> {
		if (pendingID) return;
		const sequence = ++mutationSequence;
		const generation = authorityGeneration;
		pendingID = skillID;
		try {
			await updateLearnedSkill(skillID, action, protectedValue);
			if (sequence === mutationSequence && generation === authorityGeneration) await load();
		} catch {
			if (sequence === mutationSequence && generation === authorityGeneration) errorMessage = text.failed;
		} finally {
			if (sequence === mutationSequence && generation === authorityGeneration) pendingID = '';
		}
	}
	onMount(load);
</script>

<Card.Root>
	<Card.Header class="gap-3">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<Card.Title>{text.title}</Card.Title>
			<div class="flex items-center gap-2">
				{#if inventory?.activeCount !== undefined && inventory.activeLimit !== undefined}<Badge variant="secondary">{inventory.activeCount} / {inventory.activeLimit}</Badge>{/if}
				<Button variant="outline" size="sm" onclick={load} disabled={isLoading}><RefreshIcon data-icon="inline-start" />{text.refresh}</Button>
			</div>
		</div>
		<Card.Description>{text.description}</Card.Description>
	</Card.Header>
	<Card.Content>
		{#if learningSettings}<div class="mb-5 grid gap-3 rounded-lg border p-4 sm:grid-cols-[1fr_auto_auto] sm:items-end"><Field.Field><Field.Label>{text.learning}</Field.Label><div class="flex items-center gap-2"><Checkbox bind:checked={learningSettings.enabled} /><span class="text-sm">{learningSettings.enabled ? text.enabled : text.disabled}</span></div></Field.Field><Field.Field><Field.Label for="learned-limit">{text.limit}</Field.Label><Input id="learned-limit" type="number" min="1" max="100" bind:value={learningSettings.activeLimit} /></Field.Field><Button variant="outline" onclick={saveSettings}>{text.save}</Button></div>{/if}
		{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
		{#if !inventory && !errorMessage}<Skeleton class="h-28 w-full" />
		{:else if inventory && inventory.skills.length === 0}
			{#if isLoading}<Skeleton class="h-28 w-full" />
			{:else if inventory.invalidSkillCount > 0}<p role="alert" class="text-sm text-destructive">{text.malformed}</p>
			{:else if !errorMessage}
				<Empty.Root><Empty.Header><Empty.Title>{text.empty}</Empty.Title></Empty.Header></Empty.Root>
			{/if}
		{:else if inventory}<div class="grid gap-4">{#if inventory.invalidSkillCount > 0}<p role="alert" class="text-sm text-destructive">{text.malformed}</p>{/if}{#each inventory.skills as skill (skill.id + skill.version)}
			<article class="grid gap-3 rounded-lg border p-4">
				<div class="flex flex-wrap items-start justify-between gap-3"><div><button type="button" class="text-left font-medium underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" onclick={() => selectedID = selectedID === skill.id ? '' : skill.id}>{skill.id}</button><p class="text-sm text-muted-foreground">{skill.description || text.purpose}</p></div><Badge variant={skill.status === 'active' ? 'secondary' : 'outline'}>{skill.status === 'active' ? text.active : text.retired}</Badge></div>
				<div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground"><span>v{skill.version}</span><span>{text.evidence} {skill.evidenceIDs.length}</span><span>{audienceLabel(skill.audience)}</span>{#if skill.reason}<span>{text.reason}: {skill.reason}</span>{/if}{#if skill.verification}<span>{text.verification}: {verificationLabel(skill.verification)}</span>{/if}</div>
				<Separator />
				<div class="flex flex-wrap gap-2"><Button size="sm" variant="outline" disabled={pendingID === skill.id} onclick={() => change(skill.id, 'protect', !skill.protected)}>{skill.protected ? text.unprotect : text.protect}</Button><Button size="sm" variant="outline" onclick={() => showHistory(skill.id)}>{text.history}</Button>{#if skill.status === 'active'}<Button size="sm" variant="outline" disabled={pendingID === skill.id} onclick={() => change(skill.id, 'retire')}>{text.retire}</Button>{:else}<Button size="sm" variant="outline" disabled={pendingID === skill.id} onclick={() => change(skill.id, 'restore')}>{text.restore}</Button>{/if}</div>
			</article>
			{#if selectedSkill?.id === skill.id}<div class="grid gap-4 rounded-lg bg-muted/40 p-4 text-sm"><p class="whitespace-pre-wrap">{selectedSkill.instruction}</p>{#if history.length > 0}<section class="grid gap-2" aria-label={text.history}><h3 class="font-medium">{text.history}</h3>{#each [...history].sort((first, second) => second.version - first.version) as revision (revision.id + revision.version)}<article class="grid gap-1 border-t pt-3"><div class="flex flex-wrap items-center gap-2"><span class="font-medium">{text.version} {revision.version}</span><Badge variant="outline">{revision.status === 'active' ? text.active : text.retired}</Badge></div>{#if revision.reason}<p class="text-muted-foreground">{text.reason}: {revision.reason}</p>{/if}<details><summary class="cursor-pointer font-medium">{text.purpose}</summary><p class="mt-2 whitespace-pre-wrap text-muted-foreground">{revision.instruction}</p></details></article>{/each}</section>{/if}</div>{/if}
		{/each}</div>{/if}
	</Card.Content>
</Card.Root>
