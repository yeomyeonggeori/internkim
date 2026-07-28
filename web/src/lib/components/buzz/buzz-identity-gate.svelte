<script lang="ts">
	import { onMount } from 'svelte';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import FingerprintIcon from '@lucide/svelte/icons/fingerprint';
	import {
		addBuzzPasskeyCopy,
		createBuzzIdentityTransport,
		enrollBuzzIdentity,
		hasBuzzPasskeyCopy,
		hasEnrolledBuzzIdentity,
		unlockBuzzIdentity,
		type BuzzUnlockFactor
	} from '$lib/buzz-identity-session';
	import { deriveBuzzPasskeyOutput, isPasskeySupported, registerBuzzPasskey } from '$lib/buzz-passkey';
	import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';

	const transport = createBuzzIdentityTransport();
	const passkeyAvailable = isPasskeySupported();

	type Mode = 'loading' | 'enroll' | 'addPasskey' | 'unlock' | 'error';
	let mode = $state<Mode>('loading');
	let authenticated = $state(false);
	let email = $state('');
	let displayName = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let pendingSecret = $state('');
	let hasPasskey = $state(false);
	let busy = $state(false);
	let errorMessage = $state('');

	const open = $derived(authenticated && buzzIdentity.secretHex === null && mode !== 'loading');

	onMount(async () => {
		try {
			const session: { authenticated?: boolean; email?: string } = await fetch('/auth/session', {
				credentials: 'include'
			}).then((response) => response.json());
			if (!session.authenticated || !session.email) return;
			authenticated = true;
			email = session.email;
			displayName = email;
			if (await hasEnrolledBuzzIdentity(transport)) {
				hasPasskey = await hasBuzzPasskeyCopy(transport);
				mode = 'unlock';
			} else {
				mode = 'enroll';
			}
		} catch (error) {
			mode = 'error';
			errorMessage = describe(error);
		}
	});

	function describe(error: unknown): string {
		return error instanceof Error ? error.message : text.genericError;
	}

	async function run(work: () => Promise<void>) {
		busy = true;
		errorMessage = '';
		try {
			await work();
		} catch (error) {
			errorMessage = describe(error);
		} finally {
			busy = false;
		}
	}

	function finish(secretHex: string) {
		buzzIdentity.secretHex = secretHex;
		password = '';
		confirmPassword = '';
	}

	function enrollWithPassword() {
		if (password.length < 8) {
			errorMessage = text.passwordTooShort;
			return;
		}
		if (password !== confirmPassword) {
			errorMessage = text.passwordMismatch;
			return;
		}
		const factor: BuzzUnlockFactor = { kind: 'password', password };
		return run(async () => {
			pendingSecret = await enrollBuzzIdentity(transport, factor);
			mode = passkeyAvailable ? 'addPasskey' : 'unlock';
			if (mode === 'unlock') finish(pendingSecret);
		});
	}

	function addPasskeyNow() {
		return run(async () => {
			await registerBuzzPasskey(email, displayName);
			const output = await deriveBuzzPasskeyOutput();
			await addBuzzPasskeyCopy(transport, pendingSecret, output);
			finish(pendingSecret);
		});
	}

	function skipPasskey() {
		finish(pendingSecret);
	}

	function unlockWithPasskey() {
		return run(async () => {
			const output = await deriveBuzzPasskeyOutput();
			finish(await unlockBuzzIdentity(transport, { kind: 'passkey', output }));
		});
	}

	function unlockWithPassword() {
		const factor: BuzzUnlockFactor = { kind: 'password', password };
		return run(async () => finish(await unlockBuzzIdentity(transport, factor)));
	}

	async function logOut() {
		try {
			const response = await fetch(`/auth/logout?return=${encodeURIComponent(location.pathname)}`, {
				method: 'POST',
				credentials: 'include'
			});
			const body = (await response.json()) as { redirectURL?: string };
			location.replace(body.redirectURL ?? '/');
		} catch {
			location.replace('/');
		}
	}

	const text = {
		enrollTitle: '보안 신원 설정',
		enrollDescription:
			'메시지를 이 브라우저에서 직접 서명합니다. 이메일과 비밀번호로 어디서나 로그인하고, 키는 서버가 아니라 당신만 열 수 있게 보호됩니다.',
		addPasskeyTitle: '이 기기에 패스키 추가',
		addPasskeyDescription: '다음부턴 비밀번호 없이 지문·얼굴로 원터치 로그인합니다. 선택 사항이에요.',
		unlockTitle: '신원 잠금 해제',
		unlockDescription: '이 계정의 서명 키를 잠금 해제하세요.',
		usePasskey: '패스키로 로그인',
		addPasskey: '패스키 추가',
		later: '나중에',
		password: '비밀번호',
		newPassword: '비밀번호 (이메일 + 비밀번호로 로그인)',
		confirmPassword: '비밀번호 확인',
		setPassword: '설정하기',
		unlock: '잠금 해제',
		logOut: '로그아웃',
		passwordTooShort: '비밀번호는 8자 이상이어야 합니다',
		passwordMismatch: '비밀번호가 일치하지 않습니다',
		genericError: '문제가 발생했습니다'
	};
</script>

<Dialog.Root {open}>
	<Dialog.Content
		class="sm:max-w-md"
		showCloseButton={false}
		onEscapeKeydown={(event) => event.preventDefault()}
		onInteractOutside={(event) => event.preventDefault()}
	>
		{#if mode === 'addPasskey'}
			<Dialog.Header>
				<Dialog.Title>{text.addPasskeyTitle}</Dialog.Title>
				<Dialog.Description>{text.addPasskeyDescription}</Dialog.Description>
			</Dialog.Header>
			<div class="flex flex-col gap-3 py-2">
				<Button onclick={addPasskeyNow} disabled={busy} class="gap-2">
					<FingerprintIcon class="size-4" />
					{text.addPasskey}
				</Button>
				<Button variant="ghost" onclick={skipPasskey} disabled={busy}>{text.later}</Button>
				{#if errorMessage}
					<p class="text-sm text-destructive">{errorMessage}</p>
				{/if}
			</div>
		{:else if mode === 'unlock'}
			<Dialog.Header>
				<Dialog.Title>{text.unlockTitle}</Dialog.Title>
				<Dialog.Description>{text.unlockDescription}</Dialog.Description>
			</Dialog.Header>
			<div class="flex flex-col gap-4 py-2">
				{#if hasPasskey && passkeyAvailable}
					<Button onclick={unlockWithPasskey} disabled={busy} class="gap-2">
						<FingerprintIcon class="size-4" />
						{text.usePasskey}
					</Button>
				{/if}
				<div class="flex flex-col gap-2">
					<Label for="buzz-password">{text.password}</Label>
					<Input id="buzz-password" type="password" bind:value={password} disabled={busy} />
				</div>
				{#if errorMessage}
					<p class="text-sm text-destructive">{errorMessage}</p>
				{/if}
				<div class="flex items-center justify-between gap-2">
					<button type="button" class="text-xs text-muted-foreground underline" onclick={logOut} disabled={busy}>
						{text.logOut}
					</button>
					<Button onclick={unlockWithPassword} disabled={busy || password.length === 0}>{text.unlock}</Button>
				</div>
			</div>
		{:else}
			<Dialog.Header>
				<Dialog.Title>{text.enrollTitle}</Dialog.Title>
				<Dialog.Description>{text.enrollDescription}</Dialog.Description>
			</Dialog.Header>
			<div class="flex flex-col gap-4 py-2">
				<div class="flex flex-col gap-2">
					<Label for="buzz-new-password">{text.newPassword}</Label>
					<Input id="buzz-new-password" type="password" bind:value={password} disabled={busy} />
				</div>
				<div class="flex flex-col gap-2">
					<Label for="buzz-confirm">{text.confirmPassword}</Label>
					<Input id="buzz-confirm" type="password" bind:value={confirmPassword} disabled={busy} />
				</div>
				{#if errorMessage}
					<p class="text-sm text-destructive">{errorMessage}</p>
				{/if}
				<div class="flex items-center justify-between gap-2">
					<button type="button" class="text-xs text-muted-foreground underline" onclick={logOut} disabled={busy}>
						{text.logOut}
					</button>
					<Button onclick={enrollWithPassword} disabled={busy || password.length === 0}>{text.setPassword}</Button>
				</div>
			</div>
		{/if}
	</Dialog.Content>
</Dialog.Root>
