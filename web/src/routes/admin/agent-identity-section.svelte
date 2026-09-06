<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { fetchAgentIdentity, updateAgentIdentity, type AgentIdentity } from '../settings/persona-api';

	const emptyIdentity: AgentIdentity = { schemaVersion: 1, names: [''] };
	let identity = $state<AgentIdentity>(emptyIdentity);
	let isLoading = $state(true);
	let isSaving = $state(false);
	let errorMessage = $state('');
	let aliasText = $state('');

	onMount(async () => {
		try {
			identity = await fetchAgentIdentity();
			aliasText = identity.names.slice(1).join(', ');
		} catch { errorMessage = '아이덴티티를 불러오지 못했습니다.'; }
		finally { isLoading = false; }
	});

	async function save(): Promise<void> {
		isSaving = true;
		errorMessage = '';
		try {
			identity = await updateAgentIdentity({ ...identity, names: [identity.names[0], ...aliasText.split(',').map((name) => name.trim()).filter(Boolean)] });
			aliasText = identity.names.slice(1).join(', ');
		} catch { errorMessage = '아이덴티티를 저장하지 못했습니다.'; }
		finally { isSaving = false; }
	}
</script>

<Card.Root>
	<Card.Header><Card.Title>에이전트 아이덴티티</Card.Title><Card.Description>이름, 역할, 이모지와 소개를 관리합니다.</Card.Description></Card.Header>
	<Card.Content class="grid gap-5">
		<Field.Field><Field.Label for="identity-name">대표 이름</Field.Label><Input id="identity-name" value={identity.names[0]} disabled /></Field.Field>
		<Field.Field><Field.Label for="identity-aliases">추가 이름</Field.Label><Input id="identity-aliases" bind:value={aliasText} disabled={isLoading} placeholder="쉼표로 구분" /></Field.Field>
		<div class="grid gap-5 sm:grid-cols-2"><Field.Field><Field.Label for="identity-role">역할</Field.Label><Input id="identity-role" bind:value={identity.role} disabled={isLoading} /></Field.Field><Field.Field><Field.Label for="identity-creature">형태</Field.Label><Input id="identity-creature" bind:value={identity.creature} disabled={isLoading} /></Field.Field></div>
		<Field.Field><Field.Label for="identity-emoji">이모지</Field.Label><Input id="identity-emoji" bind:value={identity.emoji} disabled={isLoading} maxlength={8} /></Field.Field>
		<Field.Field><Field.Label for="identity-introduction">소개</Field.Label><Textarea id="identity-introduction" bind:value={identity.introduction} disabled={isLoading} class="min-h-24" /></Field.Field>
		{#if errorMessage}<Field.Error>{errorMessage}</Field.Error>{/if}
	</Card.Content>
	<Card.Footer class="justify-end"><Button disabled={isLoading || isSaving} onclick={save}>저장</Button></Card.Footer>
</Card.Root>
