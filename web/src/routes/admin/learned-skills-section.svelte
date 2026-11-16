<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import * as Empty from '$lib/components/ui/empty';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Switch } from '$lib/components/ui/switch';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import RefreshIcon from '@lucide/svelte/icons/refresh-cw';
	import { fetchLearnedSkills, fetchLearningSettings, fetchLearnedSkillHistory, updateLearningSettings, updateLearnedSkill, type LearnedSkill, type LearnedSkillInventory } from './learned-skills-api';
	import { isSkillReadAccessDenied } from './skill-read-error';

	const text = createPageText({
		ko: { title: '스스로 배우기', description: '켜 두면 업무가 끝날 때마다 돌아보고 두 가지를 합니다.', effectProcedures: '반복되는 일의 순서를 아래 배운 절차로 남겨 다음부터 그대로 씁니다.', effectPrinciples: '필요하면 일하는 원칙을 고칩니다. 바뀐 내용은 원칙의 변경 이력에 남습니다.', offNote: '꺼져 있어도 배운 절차는 그대로 쓰고, 새로 배우거나 원칙을 고치지는 않습니다.', enabledSaved: '스스로 배우기를 켰습니다.', disabledSaved: '스스로 배우기를 껐습니다.', procedures: '배운 절차', activeCount: '쓰는 중 {count}개', limit: '최대 {count}개까지 씁니다', changeLimit: '바꾸기', limitLabel: '최대 개수', limitSaved: '배운 절차를 최대 {count}개까지 씁니다.', save: '저장', cancel: '취소', refresh: '새로고침', empty: '아직 배운 절차가 없습니다.', failed: '배운 절차를 불러오지 못했습니다.', saveFailed: '설정을 저장하지 못했습니다.', actionFailed: '절차를 바꾸지 못했습니다.', malformed: '응답에 표시할 수 없는 절차가 포함되어 있습니다.', details: '내용과 이력', hideDetails: '접기', lock: '더 고치지 않게', unlock: '다시 고칠 수 있게', locked: '김인턴이 더 고치지 않음', retire: '그만 쓰기', restore: '다시 쓰기', retiredGroup: '그만 쓴 절차 {count}개', evidence: '근거 {count}건', reason: '배운 이유', history: '변경 이력', version: '버전', active: '쓰는 중', retired: '그만 씀', companyAudience: '회사 전체', personalAudience: '개인', scopedAudience: '지정 범위' },
		en: { title: 'Learning on its own', description: 'When on, the agent looks back at every finished task and does two things.', effectProcedures: 'It keeps the steps of repeated work as a learned procedure below and follows them next time.', effectPrinciples: 'When needed, it rewrites its working principles. Each change is kept in their history.', offNote: 'While off, it keeps using learned procedures but learns nothing new and leaves its principles alone.', enabledSaved: 'Learning is on.', disabledSaved: 'Learning is off.', procedures: 'Learned procedures', activeCount: '{count} in use', limit: 'Up to {count} in use', changeLimit: 'Change', limitLabel: 'Most in use', limitSaved: 'Up to {count} learned procedures are used.', save: 'Save', cancel: 'Cancel', refresh: 'Refresh', empty: 'No learned procedures yet.', failed: 'Could not load learned procedures.', saveFailed: 'Could not save the setting.', actionFailed: 'Could not change the procedure.', malformed: 'Some returned procedures could not be displayed.', details: 'Steps and history', hideDetails: 'Hide', lock: 'Stop changing it', unlock: 'Allow changes', locked: 'The agent leaves it as is', retire: 'Stop using', restore: 'Use again', retiredGroup: '{count} no longer in use', evidence: '{count} pieces of evidence', reason: 'Why it was learned', history: 'History', version: 'Version', active: 'In use', retired: 'Not in use', companyAudience: 'Company-wide', personalAudience: 'Personal', scopedAudience: 'Specific scope' }
	});
	let inventory = $state<LearnedSkillInventory | undefined>();
	let errorMessage = $state('');
	let isLoading = $state(true);
	let pendingID = $state('');
	let selectedID = $state('');
	let learningSettings = $state<{ enabled: boolean; activeLimit: number }>();
	let isSavingSettings = $state(false);
	let isEditingLimit = $state(false);
	let limitDraft = $state(20);
	let history = $state<LearnedSkill[]>([]);
	let authorityGeneration = 0;
	let loadSequence = 0;
	let historySequence = 0;
	let mutationSequence = 0;
	const activeSkills = $derived(inventory?.skills.filter((skill) => skill.status === 'active') ?? []);
	const retiredSkills = $derived(inventory?.skills.filter((skill) => skill.status !== 'active') ?? []);
	function countText(template: string, count: number): string { return template.replace('{count}', String(count)); }
	function audienceLabel(value: string): string { return value === 'company' ? text.companyAudience : value.startsWith('person:') ? text.personalAudience : text.scopedAudience; }
	function skillTitle(skill: LearnedSkill): string { return skill.description.trim() || skill.id; }
	function revisionSummary(revision: LearnedSkill): string { return [`${text.version} ${revision.version}`, revision.status === 'active' ? text.active : text.retired, revision.reason].filter(Boolean).join(' · '); }
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
	async function toggleDetails(skillID: string): Promise<void> {
		if (selectedID === skillID) { selectedID = ''; return; }
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
	async function saveSettings(next: { enabled: boolean; activeLimit: number }, savedMessage: string): Promise<boolean> {
		if (!learningSettings || isSavingSettings) return false;
		const previous = learningSettings;
		learningSettings = next;
		isSavingSettings = true;
		errorMessage = '';
		try {
			await updateLearningSettings(next);
			toast.success(savedMessage);
			return true;
		} catch {
			learningSettings = previous;
			errorMessage = text.saveFailed;
			return false;
		} finally {
			isSavingSettings = false;
		}
	}
	function setEnabled(enabled: boolean): void {
		if (!learningSettings) return;
		void saveSettings({ ...learningSettings, enabled }, enabled ? text.enabledSaved : text.disabledSaved);
	}
	function startEditingLimit(): void {
		if (!learningSettings) return;
		limitDraft = learningSettings.activeLimit;
		isEditingLimit = true;
	}
	async function saveLimit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (!learningSettings) return;
		const activeLimit = Math.min(100, Math.max(1, Math.round(Number(limitDraft)) || learningSettings.activeLimit));
		if (await saveSettings({ ...learningSettings, activeLimit }, countText(text.limitSaved, activeLimit))) isEditingLimit = false;
	}
	async function change(skillID: string, action: 'protect' | 'retire' | 'restore', protectedValue = action === 'protect'): Promise<void> {
		if (pendingID) return;
		const sequence = ++mutationSequence;
		const generation = authorityGeneration;
		pendingID = skillID;
		try {
			await updateLearnedSkill(skillID, action, protectedValue);
			if (sequence === mutationSequence && generation === authorityGeneration) await load();
		} catch {
			if (sequence === mutationSequence && generation === authorityGeneration) errorMessage = text.actionFailed;
		} finally {
			if (sequence === mutationSequence && generation === authorityGeneration) pendingID = '';
		}
	}
	onMount(load);
</script>

{#snippet procedure(skill: LearnedSkill)}
	<article class="grid gap-1.5 border-t p-4 first:border-t-0">
		<div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
			<span class="font-medium" class:text-muted-foreground={skill.status !== 'active'}>{skillTitle(skill)}</span>
			{#if skill.protected && skill.status === 'active'}<span class="text-xs text-muted-foreground">{text.locked}</span>{/if}
		</div>
		{#if skill.reason}<p class="text-sm text-muted-foreground">{text.reason}: {skill.reason}</p>{/if}
		<div class="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground"><span>{audienceLabel(skill.audience)}</span><span>{countText(text.evidence, skill.evidenceIDs.length)}</span><span>{text.version} {skill.version}</span></div>
		{#if selectedID === skill.id}
			<div class="mt-1 grid gap-3 rounded-md bg-muted/50 p-3 text-sm">
				<p class="whitespace-pre-wrap">{skill.instruction}</p>
				{#if history.length > 0}
					<section class="grid gap-1 border-t pt-2 text-xs text-muted-foreground" aria-label={text.history}>
						{#each [...history].sort((first, second) => second.version - first.version) as revision (revision.id + revision.version)}
							<details>
								<summary class="cursor-pointer">{revisionSummary(revision)}</summary>
								<p class="mt-1 whitespace-pre-wrap">{revision.instruction}</p>
							</details>
						{/each}
					</section>
				{/if}
			</div>
		{/if}
		<div class="mt-1 flex flex-wrap gap-2">
			<Button size="sm" variant="outline" aria-expanded={selectedID === skill.id} onclick={() => toggleDetails(skill.id)}>{selectedID === skill.id ? text.hideDetails : text.details}</Button>
			{#if skill.status === 'active'}
				<Button size="sm" variant="outline" disabled={pendingID === skill.id} onclick={() => change(skill.id, 'protect', !skill.protected)}>{skill.protected ? text.unlock : text.lock}</Button>
				<Button size="sm" variant="outline" class="text-destructive" disabled={pendingID === skill.id} onclick={() => change(skill.id, 'retire')}>{text.retire}</Button>
			{:else}
				<Button size="sm" variant="outline" disabled={pendingID === skill.id} onclick={() => change(skill.id, 'restore')}>{text.restore}</Button>
			{/if}
		</div>
	</article>
{/snippet}

<Card.Root>
	<Card.Header>
		<div class="flex items-center justify-between gap-3">
			<Card.Title id="learning-title">{text.title}</Card.Title>
			<div class="flex items-center gap-1">
				<Button variant="ghost" size="icon-sm" aria-label={text.refresh} title={text.refresh} onclick={load} disabled={isLoading}><RefreshIcon /></Button>
				{#if learningSettings}<Switch aria-labelledby="learning-title" checked={learningSettings.enabled} disabled={isSavingSettings} onCheckedChange={setEnabled} />{/if}
			</div>
		</div>
		<Card.Description>{text.description}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-5">
		<ul class="grid list-disc gap-1 pl-5 text-sm text-muted-foreground">
			<li>{text.effectProcedures}</li>
			<li>{text.effectPrinciples}</li>
		</ul>
		{#if learningSettings && !learningSettings.enabled}<p class="text-sm text-muted-foreground">{text.offNote}</p>{/if}
		{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
		<section class="grid gap-2" aria-label={text.procedures}>
			<div class="flex flex-wrap items-center justify-between gap-2">
				<h3 class="text-sm font-medium">{text.procedures} {#if inventory}<span class="font-normal text-muted-foreground">· {countText(text.activeCount, activeSkills.length)}</span>{/if}</h3>
				{#if learningSettings}
					{#if isEditingLimit}
						<form class="flex items-center gap-2" onsubmit={saveLimit}>
							<label for="learned-limit" class="text-sm text-muted-foreground">{text.limitLabel}</label>
							<Input id="learned-limit" type="number" min="1" max="100" class="h-8 w-20" bind:value={limitDraft} />
							<Button size="sm" type="submit" disabled={isSavingSettings}>{text.save}</Button>
							<Button size="sm" variant="ghost" type="button" onclick={() => (isEditingLimit = false)}>{text.cancel}</Button>
						</form>
					{:else}
						<div class="flex items-center gap-1 text-sm text-muted-foreground"><span>{countText(text.limit, learningSettings.activeLimit)}</span><Button size="sm" variant="ghost" onclick={startEditingLimit}>{text.changeLimit}</Button></div>
					{/if}
				{/if}
			</div>
			{#if !inventory && !errorMessage}<Skeleton class="h-28 w-full" />
			{:else if inventory && inventory.skills.length === 0}
				{#if isLoading}<Skeleton class="h-28 w-full" />
				{:else if inventory.invalidSkillCount > 0}<p role="alert" class="text-sm text-destructive">{text.malformed}</p>
				{:else if !errorMessage}<Empty.Root class="border"><Empty.Header><Empty.Title>{text.empty}</Empty.Title></Empty.Header></Empty.Root>{/if}
			{:else if inventory}
				{#if inventory.invalidSkillCount > 0}<p role="alert" class="text-sm text-destructive">{text.malformed}</p>{/if}
				{#if activeSkills.length > 0}
					<div class="rounded-lg border">{#each activeSkills as skill (skill.id + skill.version)}{@render procedure(skill)}{/each}</div>
				{/if}
				{#if retiredSkills.length > 0}
					<Collapsible.Root>
						<Collapsible.Trigger class="group flex items-center gap-1.5 py-1 text-sm text-muted-foreground hover:text-foreground">
							<ChevronRightIcon class="size-4 transition-transform group-data-[state=open]:rotate-90" />{countText(text.retiredGroup, retiredSkills.length)}
						</Collapsible.Trigger>
						<Collapsible.Content>
							<div class="mt-2 rounded-lg border">{#each retiredSkills as skill (skill.id + skill.version)}{@render procedure(skill)}{/each}</div>
						</Collapsible.Content>
					</Collapsible.Root>
				{/if}
			{/if}
		</section>
	</Card.Content>
</Card.Root>
