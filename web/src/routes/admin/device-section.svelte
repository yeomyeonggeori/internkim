<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { fetchDeviceHealth } from './admin-api';
	import type { AdminPageText, AdminSession } from './admin-types';

	type DeviceSectionProps = {
		adminBaseURL: string;
		adminSession: AdminSession | null;
		isDeviceReachable: boolean;
		fleetIdInput: string;
		text: AdminPageText;
		onFleetIDSaved: (fleetID: string) => Promise<void> | void;
	};

	let {
		adminBaseURL,
		adminSession,
		isDeviceReachable = $bindable(false),
		fleetIdInput = $bindable(''),
		text,
		onFleetIDSaved
	}: DeviceSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let isCheckingDevice = $state(false);
	let adminErrorMessage = $state('');

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		checkDevice();
	});

	function adminSessionStatusText() {
		if (!adminSession) return '';
		if (adminSession.isAdmin) return text.device.admin;
		if (adminSession.bootstrapStatus === 'identity_missing') return text.device.accessEmailMissing;
		if (adminSession.bootstrapStatus === 'failed') return text.device.claimFailed;
		if (adminSession.bootstrapStatus === 'rejected') return text.device.notAdmin;
		if (!adminSession.isClaimed) return text.device.firstAdminClaimPending;
		return text.device.notAdmin;
	}

	async function saveFleetId() {
		fleetIdInput = fleetIdInput.trim().toLowerCase();
		await onFleetIDSaved(fleetIdInput);
		await checkDevice();
	}

	async function checkDevice() {
		if (!adminBaseURL) return;

		isCheckingDevice = true;
		adminErrorMessage = '';
		try {
			await fetchDeviceHealth(adminBaseURL, text.messages.deviceUnreachable);
			isDeviceReachable = true;
		} catch {
			isDeviceReachable = false;
			adminErrorMessage = text.messages.deviceUnreachable;
		} finally {
			isCheckingDevice = false;
		}
	}

</script>

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.device.title}</Card.Title>
		<Card.Description>{text.device.description}</Card.Description>
		<Card.Action>
			<Badge variant={isDeviceReachable ? 'secondary' : 'outline'}>
				{isDeviceReachable ? text.device.online : text.device.unreachable}
			</Badge>
		</Card.Action>
	</Card.Header>
	<Card.Content>
		<Field.Group>
			<form
				onsubmit={(event) => {
					event.preventDefault();
					saveFleetId();
				}}
			>
				<Field.Field>
					<Field.Label for="device-fleet-id">{text.device.fleetIDPlaceholder}</Field.Label>
					<div class="flex flex-wrap items-center gap-2">
						<Input id="device-fleet-id" class="min-w-64 flex-1" bind:value={fleetIdInput} autocomplete="off" />
						<Button type="submit" variant="outline">
							{#if isCheckingDevice}
								<LoaderIcon class="size-4 animate-spin" />
							{:else}
								<RefreshCwIcon />
							{/if}
							{text.device.check}
						</Button>
					</div>
					{#if adminSession?.email}
						<Field.Description>
							Cloudflare Access: <span class="text-foreground font-medium">{adminSession.email}</span>
							{#if adminSession.isAdmin}
								· {text.device.admin}
							{:else}
								· {adminSession.bootstrapStatus === 'failed' ? text.device.claimFailed : adminSessionStatusText()}
							{/if}
						</Field.Description>
					{:else if adminSession}
						<Field.Description>Cloudflare Access: {adminSessionStatusText()}</Field.Description>
					{/if}
				</Field.Field>
			</form>
			{#if adminErrorMessage || adminSession?.bootstrapError}
				<Field.Error>{adminErrorMessage || adminSession?.bootstrapError}</Field.Error>
			{/if}
		</Field.Group>
	</Card.Content>
</Card.Root>
