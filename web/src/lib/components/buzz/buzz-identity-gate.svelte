<script lang="ts">
	import { onMount } from 'svelte';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import {
		createBuzzIdentityTransport,
		enrollBuzzIdentity,
		hasEnrolledBuzzIdentity,
		recoverBuzzIdentity,
		unlockBuzzIdentity,
		type BuzzUnlockFactor
	} from '$lib/buzz-identity-session';
	import { generateRecoveryCode } from '$lib/buzz-key-vault';
	import { deriveBuzzPasskeyOutput, isPasskeySupported, registerBuzzPasskey } from '$lib/buzz-passkey';
	import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';

	const transport = createBuzzIdentityTransport();
	const passkeyAvailable = isPasskeySupported();

	type Mode = 'loading' | 'enroll' | 'unlock' | 'recover' | 'showRecovery' | 'error';
	let mode = $state<Mode>('loading');
	let authenticated = $state(false);
	let email = $state('');
	let displayName = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let recoveryInput = $state('');
	let recoveryCodeToShow = $state('');
	let pendingSecret = $state('');
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
			mode = (await hasEnrolledBuzzIdentity(transport)) ? 'unlock' : 'enroll';
		} catch (error) {
			mode = 'error';
			errorMessage = describe(error);
		}
	});

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

	function describe(error: unknown): string {
		return error instanceof Error ? error.message : text.genericError;
	}

	async function run(work: () => Promise<{ secretHex: string; recoveryCode?: string }>) {
		busy = true;
		errorMessage = '';
		try {
			const result = await work();
			if (result.recoveryCode) {
				pendingSecret = result.secretHex;
				recoveryCodeToShow = result.recoveryCode;
				mode = 'showRecovery';
			} else {
				buzzIdentity.secretHex = result.secretHex;
			}
			password = '';
			confirmPassword = '';
			recoveryInput = '';
		} catch (error) {
			errorMessage = describe(error);
		} finally {
			busy = false;
		}
	}

	function enrollWithPasskey() {
		const recoveryCode = generateRecoveryCode();
		return run(async () => {
			await registerBuzzPasskey(email, displayName);
			const output = await deriveBuzzPasskeyOutput();
			return { secretHex: await enrollBuzzIdentity(transport, { kind: 'passkey', output }, recoveryCode), recoveryCode };
		});
	}

	function unlockWithPasskey() {
		return run(async () => {
			const output = await deriveBuzzPasskeyOutput();
			return { secretHex: await unlockBuzzIdentity(transport, { kind: 'passkey', output }) };
		});
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
		const recoveryCode = generateRecoveryCode();
		const factor: BuzzUnlockFactor = { kind: 'password', password };
		return run(async () => ({
			secretHex: await enrollBuzzIdentity(transport, factor, recoveryCode),
			recoveryCode
		}));
	}

	function unlockWithPassword() {
		const factor: BuzzUnlockFactor = { kind: 'password', password };
		return run(async () => ({ secretHex: await unlockBuzzIdentity(transport, factor) }));
	}

	function recoverWithCode() {
		if (password.length < 8) {
			errorMessage = text.passwordTooShort;
			return;
		}
		const recoveryCode = generateRecoveryCode();
		const factor: BuzzUnlockFactor = { kind: 'password', password };
		return run(async () => ({
			secretHex: await recoverBuzzIdentity(transport, recoveryInput, factor, recoveryCode),
			recoveryCode
		}));
	}

	function acknowledgeRecovery() {
		buzzIdentity.secretHex = pendingSecret;
	}

	const text = {
		enrollTitle: '보안 신원 설정',
		enrollDescription: '메시지를 이 브라우저에서 직접 서명합니다. 키는 서버가 아니라 당신만 열 수 있게 보호됩니다.',
		unlockTitle: '신원 잠금 해제',
		unlockDescription: '이 계정의 서명 키를 잠금 해제하세요.',
		recoverTitle: '복구코드로 복구',
		recoverDescription: '저장해 둔 복구코드와 새 비밀번호를 입력하세요.',
		showRecoveryTitle: '복구코드를 저장하세요',
		showRecoveryDescription: '기기·비밀번호를 모두 잃었을 때 이 코드로만 복구할 수 있습니다. 안전한 곳에 보관하세요. 다시 볼 수 없습니다.',
		usePasskey: '패스키로',
		orPassword: '비밀번호로',
		password: '비밀번호',
		newPassword: '새 비밀번호',
		confirmPassword: '비밀번호 확인',
		recoveryCode: '복구코드',
		enroll: '설정하기',
		unlock: '잠금 해제',
		recover: '복구하기',
		forgot: '잠금 해제를 못 하시나요? 복구코드로 복구',
		saved: '저장했습니다',
		passwordTooShort: '비밀번호는 8자 이상이어야 합니다',
		passwordMismatch: '비밀번호가 일치하지 않습니다',
		genericError: '문제가 발생했습니다',
		logOut: '로그아웃'
	};
</script>

<Dialog.Root {open}>
	<Dialog.Content
		class="sm:max-w-md"
		showCloseButton={false}
		onEscapeKeydown={(event) => event.preventDefault()}
		onInteractOutside={(event) => event.preventDefault()}
	>
		{#if mode === 'showRecovery'}
			<Dialog.Header>
				<Dialog.Title>{text.showRecoveryTitle}</Dialog.Title>
				<Dialog.Description>{text.showRecoveryDescription}</Dialog.Description>
			</Dialog.Header>
			<div class="my-4 rounded-md border bg-muted p-4 text-center font-mono text-lg tracking-widest select-all">
				{recoveryCodeToShow}
			</div>
			<Dialog.Footer>
				<Button onclick={acknowledgeRecovery}>{text.saved}</Button>
			</Dialog.Footer>
		{:else if mode === 'recover'}
			<Dialog.Header>
				<Dialog.Title>{text.recoverTitle}</Dialog.Title>
				<Dialog.Description>{text.recoverDescription}</Dialog.Description>
			</Dialog.Header>
			<div class="flex flex-col gap-4 py-2">
				<div class="flex flex-col gap-2">
					<Label for="buzz-recovery">{text.recoveryCode}</Label>
					<Input id="buzz-recovery" bind:value={recoveryInput} disabled={busy} />
				</div>
				<div class="flex flex-col gap-2">
					<Label for="buzz-new-password">{text.newPassword}</Label>
					<Input id="buzz-new-password" type="password" bind:value={password} disabled={busy} />
				</div>
				{#if errorMessage}
					<p class="text-sm text-destructive">{errorMessage}</p>
				{/if}
			</div>
			<Dialog.Footer>
				<Button onclick={recoverWithCode} disabled={busy || recoveryInput.length === 0 || password.length === 0}>
					{text.recover}
				</Button>
			</Dialog.Footer>
		{:else}
			<Dialog.Header>
				<Dialog.Title>{mode === 'unlock' ? text.unlockTitle : text.enrollTitle}</Dialog.Title>
				<Dialog.Description>
					{mode === 'unlock' ? text.unlockDescription : text.enrollDescription}
				</Dialog.Description>
			</Dialog.Header>

			<div class="flex flex-col gap-4 py-2">
				{#if passkeyAvailable}
					<Button onclick={mode === 'unlock' ? unlockWithPasskey : enrollWithPasskey} disabled={busy}>
						{text.usePasskey}
					</Button>
					<div class="text-center text-xs text-muted-foreground">{text.orPassword}</div>
				{/if}

				<div class="flex flex-col gap-2">
					<Label for="buzz-password">{text.password}</Label>
					<Input id="buzz-password" type="password" bind:value={password} disabled={busy} />
				</div>

				{#if mode === 'enroll'}
					<div class="flex flex-col gap-2">
						<Label for="buzz-confirm">{text.confirmPassword}</Label>
						<Input id="buzz-confirm" type="password" bind:value={confirmPassword} disabled={busy} />
					</div>
				{/if}

				{#if errorMessage}
					<p class="text-sm text-destructive">{errorMessage}</p>
				{/if}

				{#if mode === 'unlock'}
					<button type="button" class="text-left text-xs text-muted-foreground underline" onclick={() => (mode = 'recover')}>
						{text.forgot}
					</button>
				{/if}

				<button type="button" class="text-left text-xs text-muted-foreground underline" onclick={logOut} disabled={busy}>
					{text.logOut}
				</button>
			</div>

			<Dialog.Footer>
				<Button
					onclick={mode === 'unlock' ? unlockWithPassword : enrollWithPassword}
					disabled={busy || password.length === 0}
				>
					{mode === 'unlock' ? text.unlock : text.enroll}
				</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
