<script lang="ts">
	import { onMount } from 'svelte';
	import { z } from 'zod';
	import { REGEXP_ONLY_DIGITS } from 'bits-ui';
	import { page } from '$app/state';
	import * as InputOTP from '$lib/components/ui/input-otp';
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as Field from '$lib/components/ui/field';
	import LockKeyholeIcon from '@lucide/svelte/icons/lock-keyhole';
	import GuestRoom from '$lib/data-room/guest-room.svelte';
	import { sharedDataRoomLinkSchema } from '$lib/data-room/schemas';
	import { DataRoomRefused } from '$lib/data-room/viewer';
	import { dataRoomNoticeVersion } from '$lib/data-room/links';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { dataRoomSharingText } from '$lib/data-room/sharing-text';
	import { dataRoomText } from '$lib/data-room/text';
	type SharedLinkRoom = z.infer<typeof sharedDataRoomLinkSchema>;
	const text = createPageText(dataRoomSharingText);
	const browserText = createPageText(dataRoomText);
	const fieldID = $props.id();
	let accessCode = $state('');
	let hasAccepted = $state(false);
	let isBusy = $state(false);
	let errorMessage = $state('');
	let room = $state<SharedLinkRoom | null>(null);
	const endpoint = $derived(`/api/v1/data-room/links/${page.params.linkID}`);
	async function readResponse(response: Response): Promise<unknown> {
		const answer: unknown = await response.json();
		if (response.ok) return answer;
		if (response.status === 401 || response.status === 403) room = null;
		const refusal = z.object({ message: z.string() }).safeParse(answer);
		throw new DataRoomRefused(refusal.success ? refusal.data.message : text.failure, response.status);
	}

	async function load() {
		room = sharedDataRoomLinkSchema.parse(await readResponse(await fetch(endpoint, { cache: 'no-store' })));
	}

	async function unlock(event: SubmitEvent) {
		event.preventDefault();
		if (!hasAccepted) return;
		isBusy = true;
		errorMessage = '';
		try {
			await readResponse(
				await fetch(endpoint, {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ accessCode, noticeVersion: dataRoomNoticeVersion })
				})
			);
			accessCode = '';
			await load();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failure;
		} finally {
			isBusy = false;
		}
	}

	async function signFile(documentID: string, derivedFileName?: string): Promise<string> {
		const parameters = new URLSearchParams({ documentID });
		if (derivedFileName) parameters.set('fileName', derivedFileName);
		const answer = await readResponse(await fetch(`${endpoint}?${parameters}`, { cache: 'no-store' }));
		return z.object({ downloadURL: z.string().url() }).parse(answer).downloadURL;
	}

	async function download(documentID: string) {
		isBusy = true;
		errorMessage = '';
		try {
			window.location.assign(await signFile(documentID));
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failure;
		} finally {
			isBusy = false;
		}
	}

	async function refresh() {
		try {
			await load();
			errorMessage = '';
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failure;
		}
	}

	async function resumeSession() {
		await load().catch((error: unknown) => {
			if (error instanceof DataRoomRefused && (error.status === 401 || error.status === 403)) return;
			errorMessage = error instanceof Error ? error.message : text.failure;
		});
	}

	onMount(() => {
		void resumeSession();
		const timer = window.setInterval(() => {
			if (room && !isBusy) void refresh();
		}, 30000);
		return () => window.clearInterval(timer);
	});
</script>

<svelte:head
	><title>{browserText.title}</title><meta name="robots" content="noindex,nofollow" /><meta
		name="referrer"
		content="no-referrer"
	/></svelte:head
>

<main class="bg-muted/40 min-h-dvh">
	{#if !room}
		<div class="flex min-h-dvh items-center justify-center p-4">
		<section class="bg-background grid w-full max-w-md gap-6 rounded-2xl border p-6 shadow-sm sm:p-8">
			<header class="grid gap-2">
				<div class="bg-primary text-primary-foreground mb-2 flex size-10 items-center justify-center rounded-lg">
					<LockKeyholeIcon class="size-5" />
				</div>
				<h1 class="text-xl font-semibold">{text.enterCode}</h1>
				<p class="text-sm text-muted-foreground">{text.unlockDescription}</p>
			</header>
			<form onsubmit={unlock} class="grid gap-5">
				<Field.Field>
					<Field.Label for="{fieldID}-code">{text.code}</Field.Label>
					<InputOTP.Root
						inputId="{fieldID}-code"
						pushPasswordManagerStrategy="none"
						type="password"
						inputmode="numeric"
						autocomplete="one-time-code"
						pattern={REGEXP_ONLY_DIGITS}
						minlength={6}
						maxlength={6}
						required
						bind:value={accessCode}
						disabled={isBusy}
					>
						{#snippet children({ cells })}
							<InputOTP.Group>
								{#each cells.slice(0, 3) as cell (cell)}
									<InputOTP.Slot cell={{ ...cell, char: cell.char ? '•' : cell.char }} />
								{/each}
							</InputOTP.Group>
							<InputOTP.Separator />
							<InputOTP.Group>
								{#each cells.slice(3, 6) as cell (cell)}
									<InputOTP.Slot cell={{ ...cell, char: cell.char ? '•' : cell.char }} />
								{/each}
							</InputOTP.Group>
						{/snippet}
					</InputOTP.Root>
				</Field.Field>
				<section class="grid gap-2 rounded-lg border p-4" aria-label={text.noticeTitle}>
					<h2 class="text-sm font-semibold">{text.noticeTitle}</h2>
					<p class="text-sm leading-relaxed text-muted-foreground">{text.notice}</p>
				</section>
				<div class="flex items-start gap-3">
					<Checkbox id="{fieldID}-consent" bind:checked={hasAccepted} disabled={isBusy} /><label
						for="{fieldID}-consent"
						class="text-sm leading-relaxed">{text.consent}</label
					>
				</div>
				<Button type="submit" disabled={!hasAccepted || accessCode.length !== 6 || isBusy}
					>{#if isBusy}<Spinner />{/if}{text.unlock}</Button
				>
			</form>
			{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
		</section>
		</div>
	{:else}
		<GuestRoom
			{room}
			canDownload={room.canDownload}
			expiresAt={room.expiresAt}
			{signFile}
			onDownload={download}
			isDownloading={isBusy}
			onRefresh={refresh}
		/>
		{#if errorMessage}<p
				role="alert"
				class="bg-background text-destructive fixed inset-x-4 bottom-4 mx-auto max-w-md rounded-lg border p-3 text-sm shadow-lg"
			>
				{errorMessage}
			</p>{/if}
	{/if}
</main>
