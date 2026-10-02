<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Label } from '$lib/components/ui/label';
	import {
		belongsToACompany,
		checkCompanyAddress,
		foundCompany,
		type FoundedCompany
	} from '$lib/company/found-company';
	import { homePath } from '$lib/home-path';
	import { returnPathOf } from '$lib/return-path';
	import { leavesRightAfterFounding } from './leaving-after-founding';
	import { isSupabaseConfigured } from '$lib/supabase';
	import { signOutOfSupabase } from '$lib/supabase-session';
	import { onMount } from 'svelte';

	const fieldID = $props.id();
	const addressZone = $derived(page.data.addressZone);

	let name = $state('');
	let slug = $state('');
	let invited = $state('');
	let addressNotice = $state('');
	let isAddressUsable = $state(false);
	let isFounding = $state(false);
	let errorMessage = $state('');
	let founded = $state<FoundedCompany | null>(null);
	let isSigningOut = $state(false);

	const signedInAs = $derived(page.data.session?.email ?? '');
	const whereTheyWereGoing = $derived(returnPathOf(page.url));

	async function signOut() {
		isSigningOut = true;
		await signOutOfSupabase();
		location.replace(homePath);
	}

	const invitedAddresses = $derived(
		invited
			.split(/[\s,]+/)
			.map((entry) => entry.trim().toLowerCase())
			.filter((entry) => entry.includes('@'))
	);

	function suggestAddress() {
		if (slug.trim()) return;
		slug = name
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-+|-+$/g, '')
			.slice(0, 40);
		void checkAddress();
	}

	async function checkAddress() {
		const asked = slug.trim().toLowerCase();
		if (!asked) {
			addressNotice = '';
			isAddressUsable = false;
			return;
		}
		try {
			const answer = await checkCompanyAddress(asked);
			if (asked !== slug.trim().toLowerCase()) return;
			isAddressUsable = answer.usable;
			addressNotice = answer.usable
				? `${asked}.${addressZone} 를 쓸 수 있습니다.`
				: answer.taken
					? '이미 쓰이는 주소입니다.'
					: '영문 소문자, 숫자, 하이픈으로 3자 이상이어야 합니다.';
		} catch (error) {
			if (asked !== slug.trim().toLowerCase()) return;
			addressNotice = error instanceof Error ? error.message : '주소를 확인하지 못했습니다.';
			isAddressUsable = false;
		}
	}

	function clearAddressCheck() {
		isAddressUsable = false;
		addressNotice = '';
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		isFounding = true;
		errorMessage = '';
		try {
			founded = await foundCompany({ name: name.trim(), slug: slug.trim().toLowerCase(), invited: invitedAddresses });
			if (leavesRightAfterFounding(whereTheyWereGoing, founded.invitations.length)) {
				await goto(whereTheyWereGoing);
			}
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : '회사를 만들지 못했습니다.';
		} finally {
			isFounding = false;
		}
	}

	onMount(async () => {
		if (!isSupabaseConfigured()) return;
		if (await belongsToACompany()) await goto(whereTheyWereGoing || homePath);
	});
</script>

<svelte:head><title>회사 만들기</title></svelte:head>

<main class="flex min-h-svh items-center justify-center overflow-y-auto p-4 sm:p-6">
	{#if founded}
		<Card.Root class="w-full max-w-lg">
			<Card.Header>
				<Card.Title>{name} 준비됐습니다</Card.Title>
				<Card.Description>
					회사 컴퓨터를 연결하면 김인턴과 메신저를 사용할 수 있습니다.
				</Card.Description>
			</Card.Header>
			<Card.Content class="grid gap-3">
				{#if founded.invitations.length > 0}
					<p class="text-sm text-muted-foreground">
						아래 임시 비밀번호를 각자에게 전달하세요. 로그인 후 본인이 바꿉니다.
					</p>
					{#each founded.invitations as invitation (invitation.memberID)}
						<div class="grid gap-1 rounded-lg border p-3">
							<span class="break-all text-sm">{invitation.email}</span>
							<span class="break-all font-mono text-lg select-all">{invitation.temporaryPassword}</span>
						</div>
					{/each}
				{/if}
			</Card.Content>
			<Card.Footer class="flex-col items-stretch gap-2 sm:flex-row sm:items-center">
				{#if whereTheyWereGoing}
					<Button onclick={() => goto(whereTheyWereGoing)}>적어뒀습니다, 계속하기</Button>
					<Button variant="outline" onclick={() => goto('/settings/setup')}>회사 컴퓨터 연결하기</Button>
				{:else}
					<Button onclick={() => goto('/settings/setup')}>회사 컴퓨터 연결하기</Button>
				{/if}
			</Card.Footer>
		</Card.Root>
	{:else}
		<Card.Root class="w-full max-w-lg">
			<Card.Header>
				<Card.Title class="text-2xl">회사 만들기</Card.Title>
				<Card.Description>
					{signedInAs ? `${signedInAs} 계정은 아직 어느 회사에도 속해 있지 않습니다.` : '초대받지 않은 계정이라 새 회사를 시작합니다.'}
				</Card.Description>
			</Card.Header>
			<Card.Content>
				<form class="grid gap-4" onsubmit={create}>
					<div class="grid gap-1.5">
						<Label for="company-name-{fieldID}">회사 이름</Label>
						<Input id="company-name-{fieldID}" bind:value={name} onblur={suggestAddress} disabled={isFounding} />
					</div>
					<div class="grid gap-1.5">
						<Label for="company-slug-{fieldID}">주소</Label>
						<div class="flex min-w-0 items-center gap-2">
							<Input class="min-w-0" id="company-slug-{fieldID}" bind:value={slug} oninput={clearAddressCheck} onblur={checkAddress} disabled={isFounding} />
							<span class="text-sm whitespace-nowrap text-muted-foreground">.{addressZone}</span>
						</div>
						{#if addressNotice}
							<p class="text-xs {isAddressUsable ? 'text-muted-foreground' : 'text-destructive'}">{addressNotice}</p>
						{/if}
					</div>
					<div class="grid gap-1.5">
						<Label for="company-invited-{fieldID}">함께할 사람들</Label>
						<Textarea rows={2}
							id="company-invited-{fieldID}"
							bind:value={invited}
							placeholder="이메일을 쉼표나 줄바꿈으로"
							disabled={isFounding}
						/>
						<p class="text-xs text-muted-foreground">
							{invitedAddresses.length > 0 ? `${invitedAddresses.length}명을 초대합니다.` : '나중에 초대해도 됩니다.'}
						</p>
					</div>
					{#if errorMessage}
						<p class="text-sm text-destructive">{errorMessage}</p>
					{/if}
					<Button type="submit" disabled={isFounding || !name.trim() || !isAddressUsable}>만들기</Button>
				</form>
			</Card.Content>
			{#if isSupabaseConfigured()}
				<Card.Footer class="justify-center">
					<Button variant="link" class="text-muted-foreground" onclick={signOut} disabled={isSigningOut}>
						다른 계정으로 로그인
					</Button>
				</Card.Footer>
			{/if}
		</Card.Root>
	{/if}
</main>
