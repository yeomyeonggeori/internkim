<script lang="ts">
	import { appNavigation } from '$lib/components/app-navigation.svelte';
	import { attendanceClock } from '$lib/components/attendance-clock.svelte';
	import * as Command from '$lib/components/ui/command/index.js';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../../routes/attendance/text';
	import CircleIcon from '@lucide/svelte/icons/circle';
	import LogInIcon from '@lucide/svelte/icons/log-in';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import PowerOffIcon from '@lucide/svelte/icons/power-off';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	const text = createPageText(appShellText);
	const attendanceLabels = createPageText(attendanceText);
	const clockOutShortcut = '0';
	const locationShortcuts = ['1', '2', '3', '4', '5', '6', '7', '8', '9'];
	let searchValue = $state('');

	$effect(() => {
		if (!open) return;
		attendanceClock.load();
	});

	function runClockIn(locationID: string) {
		open = false;
		attendanceClock.clock('clock_in', locationID);
	}

	function runClockOut() {
		open = false;
		attendanceClock.clock('clock_out', '');
	}

	function runLogOut() {
		open = false;
		confirmDelete({
			title: text.logOutConfirmTitle,
			description: text.logOutConfirmDescription,
			confirm: { text: text.logOut },
			cancel: { text: text.cancel },
			onConfirm: appNavigation.logOut
		});
	}

	function locationShortcut(locationID: string) {
		const index = attendanceClock.locations.findIndex((location) => location.id === locationID);
		return locationShortcuts[index] ?? '';
	}

	function handleShortcut(event: KeyboardEvent) {
		if (searchValue !== '' || event.altKey || event.metaKey || event.ctrlKey) return;
		if (event.key === clockOutShortcut) {
			if (!attendanceClock.isClockedIn) return;
			event.preventDefault();
			runClockOut();
			return;
		}
		const location = attendanceClock.locations[locationShortcuts.indexOf(event.key)];
		if (!location) return;
		event.preventDefault();
		runClockIn(location.id);
	}
</script>

<Command.Dialog bind:open title={text.search} description={text.search} onkeydown={handleShortcut}>
	<Command.Input placeholder={text.search} bind:value={searchValue} />
	<Command.List>
		<Command.Empty>{text.searchNoResults}</Command.Empty>

		<Command.Group heading={text.suggestions}>
			{#if attendanceClock.isClockedIn}
				<Command.Item keywords={[attendanceLabels.clockOut]} onSelect={runClockOut}>
					<LogOutIcon />
					{attendanceLabels.clockOut}
					<Command.Shortcut>{clockOutShortcut}</Command.Shortcut>
				</Command.Item>
			{:else if attendanceClock.defaultLocation}
				{@const defaultLocation = attendanceClock.defaultLocation}
				<Command.Item
					keywords={[attendanceLabels.clockIn, defaultLocation.name]}
					onSelect={() => runClockIn(defaultLocation.id)}
				>
					<CircleIcon style="color: {defaultLocation.color}" />
					{attendanceLabels.clockIn} · {defaultLocation.name}
					<Command.Shortcut>{locationShortcut(defaultLocation.id)}</Command.Shortcut>
				</Command.Item>
			{/if}
		</Command.Group>

		<Command.Separator />

		<Command.Group heading={text.navigate}>
			{#each [...appNavigation.apps, ...appNavigation.workspace] as item (item.href)}
				{@const Icon = item.icon}
				<Command.LinkItem href={item.href} keywords={[item.label]} onSelect={() => (open = false)}>
					<Icon />
					{item.label}
				</Command.LinkItem>
			{/each}
		</Command.Group>

		<Command.Separator />

		<Command.Group heading={text.account}>
			{#each [appNavigation.contactItem] as item (item.href)}
				{@const Icon = item.icon}
				<Command.LinkItem href={item.href} target="_blank" rel="noopener noreferrer" onSelect={() => (open = false)}>
					<Icon />
					{item.label}
				</Command.LinkItem>
			{/each}
			<Command.Item keywords={[text.logOut]} onSelect={runLogOut}>
				<PowerOffIcon />
				{text.logOut}
			</Command.Item>
		</Command.Group>

		<Command.Separator />

		<Command.Group heading={text.attendance}>
			{#each attendanceClock.locations as location (location.id)}
				<Command.Item
					keywords={[attendanceLabels.clockIn, location.name]}
					onSelect={() => runClockIn(location.id)}
				>
					<CircleIcon style="color: {location.color}" />
					{attendanceLabels.clockIn} · {location.name}
					<Command.Shortcut>{locationShortcut(location.id)}</Command.Shortcut>
				</Command.Item>
			{/each}
			<Command.Item
				keywords={[attendanceLabels.clockOut]}
				disabled={!attendanceClock.isClockedIn}
				onSelect={runClockOut}
			>
				<LogOutIcon />
				{attendanceLabels.clockOut}
				<Command.Shortcut>{clockOutShortcut}</Command.Shortcut>
			</Command.Item>
		</Command.Group>
	</Command.List>
</Command.Dialog>
