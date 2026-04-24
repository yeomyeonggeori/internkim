<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Separator } from '$lib/components/ui/separator';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import QrCode from 'svelte-qrcode';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';

	type UsersResponse = {
		users?: string[];
	};

	const logoSrc = '/logo.svg';

	let userEmails = $state<string[]>([]);
	let newEmail = $state('');
	let isLoadingUsers = $state(false);
	let isSavingUser = $state(false);
	let errorMessage = $state('');

	const deviceId = () => {
		if (!browser) return '';
		const host = location.hostname;
		if (host === 'localhost' || host === '127.0.0.1' || /^\d+\.\d+\.\d+\.\d+$/.test(host)) return '';
		const suffix = '.intern.kim';
		if (!host.endsWith(suffix)) return '';
		const id = host.slice(0, -suffix.length);
		if (!id || id === 'api' || id.includes('.')) return '';
		return id;
	};

	const pagesApi = () => {
		const id = deviceId();
		return id ? `https://api.intern.kim/api` : '/api';
	};

	const mattermostURL = () => {
		const id = deviceId();
		return id ? `https://${id}.intern.kim` : '';
	};

	const isDeviceContext = () => deviceId() !== '';

	onMount(() => {
		loadUsers();
	});

	async function loadUsers() {
		const id = deviceId();
		if (!id) return;

		isLoadingUsers = true;
		errorMessage = '';
		try {
			const response = await fetch(`${pagesApi()}/users?device_id=${id}`);
			if (!response.ok) {
				errorMessage = response.status === 403 ? '관리자만 초대 목록을 볼 수 있습니다.' : '초대 목록을 불러오지 못했습니다.';
				return;
			}
			const data = (await response.json()) as UsersResponse;
			userEmails = data.users ?? [];
		} catch {
			errorMessage = '초대 목록을 불러오지 못했습니다.';
		} finally {
			isLoadingUsers = false;
		}
	}

	async function addEmail() {
		const email = newEmail.trim().toLowerCase();
		if (!email || !deviceId()) return;

		isSavingUser = true;
		errorMessage = '';
		try {
			const response = await fetch(`${pagesApi()}/users`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ device_id: deviceId(), email })
			});
			if (!response.ok) {
				errorMessage = response.status === 403 ? '관리자만 사용자를 초대할 수 있습니다.' : '사용자 초대에 실패했습니다.';
				return;
			}
			const data = (await response.json()) as UsersResponse;
			userEmails = data.users ?? [];
			newEmail = '';
		} catch {
			errorMessage = '사용자 초대에 실패했습니다.';
		} finally {
			isSavingUser = false;
		}
	}

	async function removeEmail(email: string) {
		if (!deviceId()) return;

		isSavingUser = true;
		errorMessage = '';
		try {
			const response = await fetch(`${pagesApi()}/users/${encodeURIComponent(email)}?device_id=${deviceId()}`, {
				method: 'DELETE'
			});
			if (!response.ok) {
				errorMessage = response.status === 403 ? '관리자만 사용자를 제거할 수 있습니다.' : '사용자 제거에 실패했습니다.';
				return;
			}
			const data = (await response.json()) as UsersResponse;
			userEmails = data.users ?? [];
		} catch {
			errorMessage = '사용자 제거에 실패했습니다.';
		} finally {
			isSavingUser = false;
		}
	}
</script>

<svelte:head>
	<title>intern kim</title>
</svelte:head>

<main class="bg-background text-foreground min-h-svh">
	<div class="mx-auto flex min-h-svh w-full max-w-3xl flex-col px-5 py-6">
		<header class="flex items-center justify-between gap-4">
			<div class="flex items-center gap-3">
				<img src={logoSrc} alt="intern kim" class="size-9" />
				<div>
					<h1 class="text-lg font-semibold leading-tight">intern kim</h1>
					<p class="text-muted-foreground text-sm">Mattermost와 Slack에서 사용하는 사내 AI 하드웨어</p>
				</div>
			</div>
			<Badge variant="secondary" class="gap-1.5">
				<ShieldCheckIcon class="size-3.5" />
				Access protected
			</Badge>
		</header>

		<section class="grid gap-4 py-8 sm:grid-cols-[190px_1fr]">
			<div class="flex items-center justify-center rounded-lg border bg-muted/30 p-4">
				{#if mattermostURL()}
					<QrCode value={mattermostURL()} size="150" />
				{:else}
					<MessageSquareIcon class="text-muted-foreground size-16" strokeWidth={1.5} />
				{/if}
			</div>
			<div class="flex min-w-0 flex-col justify-center gap-4">
				<div>
					<h2 class="text-2xl font-semibold">대화는 Mattermost에서 시작하세요.</h2>
					<p class="text-muted-foreground mt-2 text-sm leading-6">
						초대받은 팀원은 Mattermost와 Slack에서 Intern Kim에게 바로 일을 맡길 수 있습니다.
					</p>
				</div>
				{#if mattermostURL()}
					<div class="flex flex-wrap items-center gap-2">
						<Button href={mattermostURL()} class="gap-2">
							<ExternalLinkIcon class="size-4" />
							Open Mattermost
						</Button>
						<CopyButton text={mattermostURL()} variant="outline" />
					</div>
				{:else}
					<p class="text-muted-foreground rounded-md border bg-muted/30 px-3 py-2 text-sm">
						기기 등록이 끝나면 전용 Mattermost 주소와 초대 관리가 표시됩니다.
					</p>
				{/if}
			</div>
		</section>

		<Separator />

		<section class="grid gap-5 py-6">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div>
					<h2 class="text-base font-semibold">Allowed Users</h2>
					<p class="text-muted-foreground text-sm">초대 목록은 Cloudflare Access와 Blueclaw policy의 기준 이메일입니다.</p>
				</div>
				<Badge variant="outline">{userEmails.length} users</Badge>
			</div>

			{#if !isDeviceContext()}
				<p class="text-muted-foreground rounded-md border bg-muted/30 px-3 py-2 text-sm">
					초대 목록은 등록된 기기 주소에서 관리할 수 있습니다.
				</p>
			{:else}
				<form
					class="flex gap-2"
					onsubmit={(event) => {
						event.preventDefault();
						addEmail();
					}}
				>
					<div class="relative flex-1">
						<MailIcon class="text-muted-foreground pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2" />
						<Input bind:value={newEmail} type="email" placeholder="name@company.com" class="pl-9" />
					</div>
					<Button type="submit" disabled={isSavingUser || !newEmail.trim()} class="gap-2">
						{#if isSavingUser}
							<LoaderIcon class="size-4 animate-spin" />
						{:else}
							<PlusIcon class="size-4" />
						{/if}
						Invite
					</Button>
				</form>

				{#if errorMessage}
					<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{errorMessage}</p>
				{/if}

				{#if isLoadingUsers}
					<p class="text-muted-foreground text-sm">초대 목록을 불러오는 중...</p>
				{:else if userEmails.length === 0}
					<p class="text-muted-foreground text-sm">등록된 사용자가 없습니다.</p>
				{:else}
					<div class="overflow-hidden rounded-lg border">
						{#each userEmails as email, index}
							<div class="flex items-center justify-between gap-3 border-b px-3 py-2 last:border-b-0">
								<div class="min-w-0">
									<p class="truncate text-sm font-medium">{email}</p>
									{#if index === 0}
										<p class="text-muted-foreground text-xs">initial admin</p>
									{/if}
								</div>
								{#if index === 0}
									<Badge variant="secondary">admin</Badge>
								{:else}
									<Button variant="ghost" size="icon" disabled={isSavingUser} onclick={() => removeEmail(email)}>
										<XIcon class="size-4" />
									</Button>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			{/if}
		</section>
	</div>
</main>
