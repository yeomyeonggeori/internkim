<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import type { CalendarGoogleAccountText } from './text';

	let {
		text,
		isUploadingGoogleOAuthClient,
		googleOAuthJavascriptOrigin,
		googleOAuthRedirectURI,
		uploadGoogleOAuthClient
	}: {
		text: CalendarGoogleAccountText;
		isUploadingGoogleOAuthClient: boolean;
		googleOAuthJavascriptOrigin: string;
		googleOAuthRedirectURI: string;
		uploadGoogleOAuthClient: (file: File) => Promise<boolean>;
	} = $props();

	let selectedGoogleOAuthClientFile = $state<File | null>(null);
	let isGoogleOAuthClientDropActive = $state(false);

	function selectGoogleOAuthClientFile(fileList: FileList | null): void {
		selectedGoogleOAuthClientFile = fileList?.[0] ?? null;
	}

	function handleGoogleOAuthClientDrop(event: DragEvent): void {
		event.preventDefault();
		isGoogleOAuthClientDropActive = false;
		selectGoogleOAuthClientFile(event.dataTransfer?.files ?? null);
	}

	function handleGoogleOAuthClientDragOver(event: DragEvent): void {
		event.preventDefault();
		isGoogleOAuthClientDropActive = true;
	}

	function handleGoogleOAuthClientDragLeave(): void {
		isGoogleOAuthClientDropActive = false;
	}

	async function submitGoogleOAuthClientFile(): Promise<void> {
		if (!selectedGoogleOAuthClientFile || isUploadingGoogleOAuthClient) return;
		const didUploadGoogleOAuthClient = await uploadGoogleOAuthClient(selectedGoogleOAuthClientFile);
		if (didUploadGoogleOAuthClient) {
			selectedGoogleOAuthClientFile = null;
		}
	}
</script>

<div
	role="group"
	aria-label={text.googleOAuthClientUploadTitle}
	class={`grid min-w-0 gap-3 rounded-md border border-dashed px-3 py-3 ${isGoogleOAuthClientDropActive ? 'border-primary bg-primary/5' : 'border-border bg-muted/30'}`}
	ondrop={handleGoogleOAuthClientDrop}
	ondragover={handleGoogleOAuthClientDragOver}
	ondragleave={handleGoogleOAuthClientDragLeave}
>
	<div class="flex min-w-0 items-start gap-2">
		<UploadIcon class="mt-0.5 size-4 shrink-0 text-muted-foreground" />
		<div class="min-w-0 space-y-1">
			<p class="text-sm font-medium">{text.googleOAuthClientUploadTitle}</p>
			<p class="text-xs leading-relaxed text-muted-foreground">{text.googleOAuthClientUploadHint}</p>
			{#if selectedGoogleOAuthClientFile}
				<p class="truncate text-xs text-foreground">{selectedGoogleOAuthClientFile.name}</p>
			{/if}
		</div>
	</div>
	<div class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-2">
		<label
			for="google-oauth-client-file"
			class="flex h-9 min-w-0 cursor-pointer items-center justify-center rounded-md border bg-background px-3 text-sm font-medium hover:bg-muted"
		>
			<span class="block min-w-0 truncate">
				{selectedGoogleOAuthClientFile?.name ?? text.googleOAuthClientChooseFile}
			</span>
		</label>
		<input
			id="google-oauth-client-file"
			aria-label={text.googleOAuthClientFileLabel}
			accept="application/json,.json"
			type="file"
			class="sr-only"
			onchange={(event) => selectGoogleOAuthClientFile(event.currentTarget.files)}
		/>
		<Button
			variant="outline"
			class="justify-center gap-2"
			disabled={!selectedGoogleOAuthClientFile || isUploadingGoogleOAuthClient}
			onclick={submitGoogleOAuthClientFile}
		>
			<UploadIcon class={isUploadingGoogleOAuthClient ? 'size-4 animate-pulse' : 'size-4'} />
			<span>{isUploadingGoogleOAuthClient ? text.googleOAuthClientUploading : text.googleOAuthClientUploadAction}</span>
		</Button>
	</div>
	<details class="rounded-md border bg-background px-3 py-2 text-xs text-muted-foreground">
		<summary class="cursor-pointer text-sm font-medium text-foreground">{text.googleOAuthClientGuide.title}</summary>
		<div class="mt-3 space-y-3">
			<p class="leading-relaxed">{text.googleOAuthClientGuide.intro}</p>
			<div class="space-y-1">
				<p class="font-medium text-foreground">{text.googleOAuthClientGuide.checklistTitle}</p>
				<ul class="list-disc space-y-1 pl-4">
					{#each text.googleOAuthClientGuide.checks as check}
						<li>{check}</li>
					{/each}
				</ul>
			</div>
			<div class="space-y-2">
				<p class="font-medium text-foreground">{text.googleOAuthClientGuide.stepsTitle}</p>
				<ol class="list-decimal space-y-2 pl-4">
					{#each text.googleOAuthClientGuide.steps as step}
						<li>
							<p class="font-medium text-foreground">{step.title}</p>
							<p class="mt-0.5 leading-relaxed">{step.body}</p>
							{#if step.action}
								<a
									href={step.action.url}
									target="_blank"
									rel="noreferrer"
									class="mt-1 inline-flex text-xs font-medium text-primary underline-offset-4 hover:underline"
								>
									{step.action.label}
								</a>
							{/if}
						</li>
					{/each}
				</ol>
			</div>
			<div class="space-y-1">
				<p class="font-medium text-foreground">{text.googleOAuthClientGuide.redirectURI}</p>
				<code class="block rounded-md bg-muted px-2 py-1.5 font-mono text-[11px] break-all text-foreground">{googleOAuthRedirectURI}</code>
			</div>
			<div class="space-y-1">
				<p class="font-medium text-foreground">{text.googleOAuthClientGuide.javascriptOrigin}</p>
				<code class="block rounded-md bg-muted px-2 py-1.5 font-mono text-[11px] break-all text-foreground">{googleOAuthJavascriptOrigin}</code>
			</div>
		</div>
	</details>
</div>
