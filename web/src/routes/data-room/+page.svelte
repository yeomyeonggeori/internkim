<script lang="ts">
	import { onMount } from 'svelte';
	import { z } from 'zod';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { invokeTool } from '$lib/public-api-call';
	import { categoryName, type DataRoomCategory } from '$lib/data-room/model';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { dataRoomGetResultSchema, companyDocumentListResultSchema } from '$lib/data-room/schemas';
	import { dataRoomRequest } from '$lib/data-room/guest';

	let room = $state<z.infer<typeof dataRoomGetResultSchema> | null>(null);
	let documents = $state<z.infer<typeof companyDocumentListResultSchema>['documents']>([]);
	let selectedCode = $state('');
	let recipientEmail = $state('');
	let roleCode = $state('investor');
	let canDownload = $state(false);
	let invitationURL = $state('');
	let invitationRecipient = $state('');
	let isInvitationSent = $state(false);
	let isSharing = $state(false);
	let isCreatingRole = $state(false);
	let newRoleCode = $state('');
	let newRoleName = $state('');
	let readableCategories = $state<string[]>([]);
	let isBusy = $state(false);
	let failure = $state('');
	const isKorean = $derived(currentLocale.value === 'ko');
	const role = $derived(room?.roles.find((candidate) => candidate.code === roleCode));
	const visibleDocuments = $derived(documents.filter((document) => !selectedCode
		|| document.categoryCode === selectedCode
		|| room?.categories.find((category) => category.code === document.categoryCode)?.parent === selectedCode));
	const parents = $derived(room?.categories.filter((category) => category.parent === null) ?? []);
	const roleItems = $derived(room?.roles.map((role) => ({ value: role.code,
		label: isKorean ? role.nameKO || role.name : role.name })) ?? []);

	async function load() {
		await perform(async () => {
			const [roomAnswer, documentAnswer] = await Promise.all([
				invokeTool('company_dataroom_get', {}), invokeTool('company_document_list', {})
			]);
			room = dataRoomGetResultSchema.parse(roomAnswer);
			documents = companyDocumentListResultSchema.parse(documentAnswer).documents;
		});
	}

	async function perform(operation: () => Promise<void>) {
		isBusy = true;
		failure = '';
		try { await operation(); }
		catch (error) { failure = error instanceof Error ? error.message : String(error); }
		finally { isBusy = false; }
	}

	async function invite() {
		await perform(async () => {
			invitationURL = '';
			isInvitationSent = false;
			const answer = z.object({ shareID: z.string() }).parse(await invokeTool('company_dataroom_share_add', {
				email: recipientEmail, audience: 'email', roleCode, canDownload
			}));
			invitationURL = `${window.location.origin}/share/invitations/${answer.shareID}`;
			invitationRecipient = recipientEmail;
			room = dataRoomGetResultSchema.parse(await invokeTool('company_dataroom_get', {}));
			await dataRoomRequest(`/api/v1/data-room/invitations/${answer.shareID}/send`, 'POST');
			isInvitationSent = true;
		});
	}

	async function revoke(shareID: string) {
		await perform(async () => {
			await invokeTool('company_dataroom_share_delete', { shareID });
			room = dataRoomGetResultSchema.parse(await invokeTool('company_dataroom_get', {}));
		});
	}

	async function createRole() {
		await perform(async () => {
			await invokeTool('company_dataroom_role_update', {
				code: newRoleCode, name: newRoleName, nameKO: newRoleName, readableCategories
			});
			room = dataRoomGetResultSchema.parse(await invokeTool('company_dataroom_get', {}));
			roleCode = newRoleCode;
			isCreatingRole = false;
		});
	}

	function toggleCategory(code: string) {
		readableCategories = readableCategories.includes(code)
			? readableCategories.filter((category) => category !== code) : [...readableCategories, code];
	}

	function label(category: DataRoomCategory) {
		return categoryName(category, currentLocale.value);
	}

	onMount(load);
</script>

<svelte:head><title>{isKorean ? '데이터룸' : 'Data room'}</title></svelte:head>

<div class="mx-auto w-full max-w-6xl p-4 md:p-6">
	<header class="mb-5 flex items-center justify-between gap-4">
		<div><h1 class="text-xl font-semibold">{isKorean ? '데이터룸' : 'Data room'}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{isKorean ? '회사의 문서 보관소' : 'The company document archive'}</p></div>
		{#if room?.canManage}<Button onclick={() => isSharing = !isSharing}>{isKorean ? '공유' : 'Share'}</Button>{/if}
	</header>
	{#if failure}<p role="alert" class="mb-4 text-sm text-destructive">{failure}</p>{/if}
	{#if isSharing && room?.canManage}
		<section class="mb-5 space-y-4 rounded-lg border p-4" aria-label={isKorean ? '공유 설정' : 'Sharing'}>
			<form onsubmit={(event) => { event.preventDefault(); void invite(); }} class="flex flex-col items-start gap-3 sm:flex-row sm:items-end">
				<div class="w-full flex-1"><label for="recipient-email" class="mb-1 block text-sm">{isKorean ? '받는 사람' : 'Recipient'}</label>
					<Input id="recipient-email" type="email" required bind:value={recipientEmail} placeholder="name@example.com" /></div>
				<div class="w-full sm:w-48"><label for="reader-role" class="mb-1 block text-sm">{isKorean ? '역할' : 'Role'}</label>
					<Select.Root type="single" items={roleItems} bind:value={roleCode}>
						<Select.Trigger id="reader-role" class="w-full">{roleItems.find((item) => item.value === roleCode)?.label}</Select.Trigger>
						<Select.Content>{#each roleItems as item (item.value)}<Select.Item value={item.value} label={item.label}>{item.label}</Select.Item>{/each}</Select.Content>
					</Select.Root></div>
				<Button type="submit" disabled={isBusy}>{isKorean ? '공유하기' : 'Share'}</Button>
			</form>
			<p class="text-sm text-muted-foreground">{isKorean ? '현재 자료와 이후 추가되는 자료를 읽을 수 있습니다.' : 'Includes current documents and future additions.'}</p>
			<label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={canDownload} />{isKorean ? '원본 다운로드 허용' : 'Allow original downloads'}</label>
			<div class="flex flex-wrap gap-2">{#each role?.readableCategories ?? [] as code (code)}
				<span class="rounded border px-2 py-1 text-xs">{code} · {isKorean ? room.categories.find((category) => category.code === code)?.nameKO : room.categories.find((category) => category.code === code)?.name}</span>
			{/each}</div>
			{#if invitationURL}
				{#if isInvitationSent}<p role="status" class="text-sm">{isKorean ? `${invitationRecipient}에게 초대를 보냈습니다.` : `Invitation sent to ${invitationRecipient}.`}</p>{/if}
				<div class="flex items-center gap-2"><Input aria-label="Invitation URL" value={invitationURL} readonly /><Button variant="outline" onclick={() => navigator.clipboard.writeText(invitationURL)}>{isKorean ? '링크 복사' : 'Copy link'}</Button></div>
			{/if}
			<Button variant="ghost" onclick={() => isCreatingRole = !isCreatingRole}>{isKorean ? '커스텀 역할 만들기' : 'Create custom role'}</Button>
			{#if isCreatingRole}
				<form onsubmit={(event) => { event.preventDefault(); void createRole(); }} class="space-y-3 border-t pt-4">
					<div class="grid gap-3 sm:grid-cols-2"><Input aria-label="Role code" placeholder="custom-role" required bind:value={newRoleCode} /><Input aria-label={isKorean ? '역할 이름' : 'Role name'} placeholder={isKorean ? '역할 이름' : 'Role name'} required bind:value={newRoleName} /></div>
					<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">{#each parents as parent (parent.code)}<fieldset class="space-y-1">
						<label class="flex items-center gap-2 text-sm font-medium"><input type="checkbox" checked={readableCategories.includes(parent.code)} onchange={() => toggleCategory(parent.code)} />{parent.code} · {label(parent)}</label>
						{#each room.categories.filter((category) => category.parent === parent.code) as child (child.code)}<label class="ml-5 flex items-center gap-2 text-sm"><input type="checkbox" disabled={readableCategories.includes(parent.code)} checked={readableCategories.includes(parent.code) || readableCategories.includes(child.code)} onchange={() => toggleCategory(child.code)} />{child.code} · {label(child)}</label>{/each}
					</fieldset>{/each}</div>
					<Button type="submit" disabled={isBusy}>{isKorean ? '역할 저장' : 'Save role'}</Button>
				</form>
			{/if}
			{#each room.shares.filter((share) => share.audience === 'email' && !share.revokedAt) as share (share.id)}
				<div class="flex flex-wrap items-center justify-between gap-2 border-t pt-3 text-sm"><span>{share.email} · {share.roleCode} · {share.acceptedAt ? (isKorean ? '수락됨' : 'Accepted') : (isKorean ? '대기' : 'Pending')}</span><Button variant="ghost" size="sm" disabled={isBusy} onclick={() => revoke(share.id)}>{isKorean ? '접근 회수' : 'Revoke'}</Button></div>
			{/each}
		</section>
	{/if}
	<div class="grid gap-5 md:grid-cols-[220px_1fr]">
		<nav aria-label={isKorean ? '문서 분류' : 'Document categories'} class="space-y-1 rounded-lg border p-2">
			<Button variant={selectedCode === '' ? 'secondary' : 'ghost'} class="w-full justify-start" onclick={() => selectedCode = ''}>{isKorean ? '전체 문서' : 'All documents'}</Button>
			{#each parents as parent (parent.code)}
				<Button variant={selectedCode === parent.code ? 'secondary' : 'ghost'} class="w-full justify-start" onclick={() => selectedCode = parent.code}><span class="w-6 font-mono text-xs text-muted-foreground">{parent.code}</span>{label(parent)}</Button>
				{#if selectedCode === parent.code || room?.categories.find((category) => category.code === selectedCode)?.parent === parent.code}
					{#each room?.categories.filter((category) => category.parent === parent.code) ?? [] as child (child.code)}<Button variant={selectedCode === child.code ? 'secondary' : 'ghost'} size="sm" class="w-full justify-start pl-5" onclick={() => selectedCode = child.code}><span class="w-7 font-mono text-xs text-muted-foreground">{child.code}</span>{label(child)}</Button>{/each}
				{/if}
			{/each}
		</nav>
		<section aria-label={isKorean ? '문서' : 'Documents'} class="min-w-0 rounded-lg border">
			{#each visibleDocuments as document (document.documentID)}<article class="border-b p-4 last:border-b-0">
				<div class="flex items-start justify-between gap-3"><h2 class="text-sm font-medium">{document.title}</h2><span class="font-mono text-xs text-muted-foreground">{document.categoryCode ?? 'legacy'}</span></div>
				<p class="mt-1 text-sm text-muted-foreground">{document.summary ?? ''}</p>
				{#if document.date}<p class="mt-2 text-xs text-muted-foreground">{document.date}</p>{/if}
			</article>{:else}<p class="p-8 text-center text-sm text-muted-foreground">{isBusy ? (isKorean ? '불러오는 중…' : 'Loading…') : (isKorean ? '보관된 문서가 없습니다.' : 'No documents yet.')}</p>{/each}
		</section>
	</div>
</div>
