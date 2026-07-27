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
		unlockBuzzIdentity
	} from '$lib/buzz-identity-session';
	import { deriveBuzzPasskeyOutput, isPasskeySupported, registerBuzzPasskey } from '$lib/buzz-passkey';
	import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';

	const transport = createBuzzIdentityTransport();
	const passkeyAvailable = isPasskeySupported();

	type Mode = 'loading' | 'enroll' | 'unlock' | 'error';
	let mode = $state<Mode>('loading');
	let email = $state('');
	let displayName = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let busy = $state(false);
	let errorMessage = $state('');

	const open = $derived(buzzIdentity.secretHex === null && mode !== 'loading');

	onMount(async () => {
		try {
			const session: { email?: string } = await fetch('/auth/session', {
				credentials: 'include'
			}).then((response) => response.json());
			email = session.email ?? '';
			displayName = email;
			mode = (await hasEnrolledBuzzIdentity(transport)) ? 'unlock' : 'enroll';
		} catch (error) {
			mode = 'error';
			errorMessage = describe(error);
		}
	});

	function describe(error: unknown): string {
		return error instanceof Error ? error.message : 'Something went wrong';
	}

	async function run(work: () => Promise<string>) {
		busy = true;
		errorMessage = '';
		try {
			buzzIdentity.secretHex = await work();
			password = '';
			confirmPassword = '';
		} catch (error) {
			errorMessage = describe(error);
		} finally {
			busy = false;
		}
	}

	function enrollWithPasskey() {
		return run(async () => {
			await registerBuzzPasskey(email, displayName);
			const output = await deriveBuzzPasskeyOutput();
			return enrollBuzzIdentity(transport, { kind: 'passkey', output });
		});
	}

	function unlockWithPasskey() {
		return run(async () => {
			const output = await deriveBuzzPasskeyOutput();
			return unlockBuzzIdentity(transport, { kind: 'passkey', output });
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
		return run(() => enrollBuzzIdentity(transport, { kind: 'password', password }));
	}

	function unlockWithPassword() {
		return run(() => unlockBuzzIdentity(transport, { kind: 'password', password }));
	}

	const text = {
		enrollTitle: '보안 신원 설정',
		enrollDescription: '메시지를 이 브라우저에서 직접 서명합니다. 키는 서버가 아니라 당신만 열 수 있게 보호됩니다.',
		unlockTitle: '신원 잠금 해제',
		unlockDescription: '이 계정의 서명 키를 잠금 해제하세요.',
		usePasskey: '패스키로',
		orPassword: '비밀번호로',
		password: '비밀번호',
		confirmPassword: '비밀번호 확인',
		enroll: '설정하기',
		unlock: '잠금 해제',
		passwordTooShort: '비밀번호는 8자 이상이어야 합니다',
		passwordMismatch: '비밀번호가 일치하지 않습니다'
	};
</script>

<Dialog.Root {open}>
	<Dialog.Content
		class="sm:max-w-md"
		onEscapeKeydown={(event) => event.preventDefault()}
		onInteractOutside={(event) => event.preventDefault()}
	>
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
		</div>

		<Dialog.Footer>
			<Button
				onclick={mode === 'unlock' ? unlockWithPassword : enrollWithPassword}
				disabled={busy || password.length === 0}
			>
				{mode === 'unlock' ? text.unlock : text.enroll}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
