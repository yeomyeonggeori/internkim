<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Textarea } from '$lib/components/ui/textarea';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { isDefaultNickname, nicknamesWithDefaults } from '$lib/persona/agent-names';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import { fetchAgentIdentity, updateAgentIdentity, type AgentIdentity } from '../settings/persona-api';

	const text = createPageText({
		ko: { title: '이름과 소개', description: '김인턴이 어떤 이름에 대답하고, 무슨 일을 맡고, 자기를 어떻게 소개하는지 정합니다.', name: '이름', mention: '메신저에서는 @{handle}', nicknames: '별명', nicknamesDescription: '기본 별명은 지울 수 없습니다.', addNickname: '별명 추가', nicknamePlaceholder: '새 별명', removeNickname: '{name} 지우기', role: '맡은 일', introduction: '자기소개', save: '저장', saved: '이름과 소개를 저장했습니다.', loadFailed: '이름과 소개를 불러오지 못했습니다.', saveFailed: '이름과 소개를 저장하지 못했습니다.' },
		en: { title: 'Name and introduction', description: 'The names the agent answers to, the work it takes on, and how it introduces itself.', name: 'Name', mention: '@{handle} on the messenger', nicknames: 'Nicknames', nicknamesDescription: 'The default nicknames cannot be removed.', addNickname: 'Add nickname', nicknamePlaceholder: 'New nickname', removeNickname: 'Remove {name}', role: 'Work it takes on', introduction: 'Introduction', save: 'Save', saved: 'Saved the name and introduction.', loadFailed: 'Could not load the name and introduction.', saveFailed: 'Could not save the name and introduction.' }
	});
	const roleMaximumLength = 120;
	const introductionMaximumLength = 400;
	let saved = $state<AgentIdentity>();
	let nicknames = $state<string[]>([]);
	let role = $state('');
	let introduction = $state('');
	let isSaving = $state(false);
	let loadError = $state('');
	let saveError = $state('');
	let isAddingNickname = $state(false);
	let newNickname = $state('');
	let nicknameInput = $state<HTMLInputElement | null>(null);
	const hasChanges = $derived(saved !== undefined && (nicknames.join('\n') !== nicknamesWithDefaults(saved.names.slice(1)).join('\n') || role !== (saved.role ?? '') || introduction !== (saved.introduction ?? '')));

	onMount(async () => {
		try {
			show(await fetchAgentIdentity());
		} catch {
			loadError = text.loadFailed;
		}
	});

	function show(identity: AgentIdentity): void {
		saved = identity;
		nicknames = nicknamesWithDefaults(identity.names.slice(1));
		role = identity.role ?? '';
		introduction = identity.introduction ?? '';
	}

	async function openNicknameInput(): Promise<void> {
		isAddingNickname = true;
		await tick();
		nicknameInput?.focus();
	}

	function closeNicknameInput(): void {
		isAddingNickname = false;
		newNickname = '';
	}

	function addNickname(): void {
		nicknames = nicknamesWithDefaults([...nicknames, newNickname]);
		newNickname = '';
	}

	function handleNicknameKey(event: KeyboardEvent): void {
		if (event.isComposing) return;
		if (event.key === 'Enter') {
			event.preventDefault();
			addNickname();
		}
		if (event.key === 'Escape') closeNicknameInput();
	}

	function handleNicknameBlur(): void {
		if (newNickname.trim() === '') closeNicknameInput();
	}

	function removeNickname(nickname: string): void {
		nicknames = nicknames.filter((candidate) => candidate !== nickname);
	}

	async function save(): Promise<void> {
		if (!saved) return;
		isSaving = true;
		saveError = '';
		try {
			show(await updateAgentIdentity({ ...saved, names: [saved.names[0], ...nicknamesWithDefaults(nicknames)], role: role.trim(), introduction: introduction.trim() }));
			toast.success(text.saved);
		} catch {
			saveError = text.saveFailed;
		} finally {
			isSaving = false;
		}
	}
</script>

<Card.Root>
	<Card.Header><Card.Title>{text.title}</Card.Title><Card.Description>{text.description}</Card.Description></Card.Header>
	<Card.Content>
		{#if loadError}<p class="text-sm text-destructive">{loadError}</p>
		{:else if !saved}<Skeleton class="h-56 w-full" />
		{:else}
			<Field.Group>
				<Field.Field>
					<Field.Label>{text.name}</Field.Label>
					<p class="flex flex-wrap gap-x-2 text-sm"><span class="font-medium">{saved.names[0]}</span>{#if saved.handle}<span class="text-muted-foreground">{text.mention.replace('{handle}', saved.handle)}</span>{/if}</p>
				</Field.Field>
				<Field.Field>
					<Field.Label>{text.nicknames}</Field.Label>
					<div class="flex flex-wrap items-center gap-2">
						{#each nicknames as nickname (nickname)}
							{#if isDefaultNickname(nickname)}<Badge variant="secondary">{nickname}</Badge>
							{:else}
								<Badge variant="outline" class="gap-1 pr-1">
									{nickname}
									<button type="button" class="rounded-sm text-muted-foreground hover:text-foreground" aria-label={text.removeNickname.replace('{name}', nickname)} disabled={isSaving} onclick={() => removeNickname(nickname)}><XIcon class="size-3" /></button>
								</Badge>
							{/if}
						{/each}
						{#if isAddingNickname}
							<Input bind:ref={nicknameInput} bind:value={newNickname} class="h-7 w-36" maxlength={64} placeholder={text.nicknamePlaceholder} aria-label={text.addNickname} onkeydown={handleNicknameKey} onblur={handleNicknameBlur} />
						{:else}
							<Button variant="ghost" size="sm" disabled={isSaving} onclick={openNicknameInput}><PlusIcon data-icon="inline-start" />{text.addNickname}</Button>
						{/if}
					</div>
					<Field.Description>{text.nicknamesDescription}</Field.Description>
				</Field.Field>
				<Field.Field>
					<Field.Label for="identity-role">{text.role}</Field.Label>
					<Textarea id="identity-role" bind:value={role} disabled={isSaving} maxlength={roleMaximumLength} class="min-h-16" />
				</Field.Field>
				<Field.Field>
					<Field.Label for="identity-introduction">{text.introduction}</Field.Label>
					<Textarea id="identity-introduction" bind:value={introduction} disabled={isSaving} maxlength={introductionMaximumLength} class="min-h-24" />
				</Field.Field>
				{#if saveError}<Field.Error>{saveError}</Field.Error>{/if}
			</Field.Group>
		{/if}
	</Card.Content>
	{#if saved}<Card.Footer class="justify-end"><Button disabled={!hasChanges || isSaving} onclick={save}>{text.save}</Button></Card.Footer>{/if}
</Card.Root>
