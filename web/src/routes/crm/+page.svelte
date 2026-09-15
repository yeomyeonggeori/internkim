<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import * as Select from '$lib/components/ui/select';
	import * as Tabs from '$lib/components/ui/tabs';
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { isSupabaseConfigured, supabaseMemberRole } from '$lib/supabase-session';
	import AlertCircleIcon from '@lucide/svelte/icons/alert-circle';
	import CheckCircle2Icon from '@lucide/svelte/icons/check-circle-2';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import { onMount } from 'svelte';
	import CRMActivityDetailSheet from './crm-activity-detail-sheet.svelte';
	import CRMActivityTable from './crm-activity-table.svelte';
	import CRMContactEditSheet from './crm-contact-edit-sheet.svelte';
	import CRMContactTable from './crm-contact-table.svelte';
	import CRMDefinitionsEditor from './crm-definitions-editor.svelte';
	import { crmFixtureMode } from './dev-crm-fixture';
	import { currentCRMDate } from './crm-date';
	import { crmDefinitionLabel, crmLabel } from './crm-labels';
	import CRMKPICell from './crm-kpi-card.svelte';
	import CRMOpportunityEditSheet from './crm-opportunity-edit-sheet.svelte';
	import { CRMPageController } from './crm-page-controller.svelte';
	import CRMPipelineBoard from './crm-pipeline-board.svelte';
	import type { CRMPipelineBoardMoveRequest } from './crm-pipeline-board-drag';
	import CRMProgressTable from './crm-progress-table.svelte';
	import CRMRecordSheet from './crm-record-sheet.svelte';
	import CRMRelationshipDetailSheet from './crm-relationship-detail-sheet.svelte';
	import CRMRelationshipEditSheet from './crm-relationship-edit-sheet.svelte';
	import CRMRelationshipTable from './crm-relationship-table.svelte';
	import CRMReportDashboard from './crm-report-dashboard.svelte';
	import * as Card from '$lib/components/ui/card';
	import FilterCombobox from '$lib/components/filter-combobox.svelte';
	import { buildCRMKPICards } from './crm-kpi';
	import { crmViewCurrency, isViewCurrencyAvailable } from './crm-view-currency.svelte';
	import { dealStageIcon } from './crm-status-icons';
	import CRMViewCurrencySelect from './crm-view-currency-select.svelte';
	import type {
		CRMOrganization,
		CRMOrganizationStatus,
		CRMActivity,
		CRMActivityEditDraft,
		CRMActivityKind,
		CRMContact,
		CRMCreateDraft,
		CRMOpportunity,
	CRMOpportunityStage,
		CRMProgressKind,
		CRMRecordKind
	} from './crm-types';
	import {
		crmImportanceOptions,
		crmLastContactWindowOptions,
		crmOrganizationStatusOptions,
		findOrganizationByID,
		opportunityStageLabel,
		organizationMatchesFacets,
		type CRMImportanceFilter,
		type CRMLastContactWindow,
		type CRMOrganizationStatusFilter,
		type CRMOrganizationTypeFilter,
		type CRMRelationshipFacets,
		type CRMTab
	} from './crm-view-model';
	import { crmText } from './text';
	import type { TaskDefinitions } from '../task/task-types';

	type RelationshipView = 'all' | 'mine';
	type PipelineView = 'table' | 'board';
	type PipelineFilter = CRMProgressKind | 'all';
	type ActivityView = CRMActivityKind | 'all';

	const text = createPageText(crmText);
	const controller = new CRMPageController(text);
	const currencyCatalogue = $derived(controller.currencyCatalogue);
	const companyBaseCurrency = $derived(controller.companyBaseCurrency);
	const organizationTypeDefinitions = $derived(controller.vocabulary.organization_types);
	let isAdmin = $state(crmFixtureMode);
	const tabItems: Array<{ value: CRMTab; label: string }> = $derived([
		{ value: 'relationships', label: text.relationships },
		{ value: 'contacts', label: text.contactDirectory },
		{ value: 'pipeline', label: text.pipeline },
		{ value: 'activities', label: text.activities },
		{ value: 'reports', label: text.reports },
		...(isAdmin ? [{ value: 'definitions' as CRMTab, label: text.definitions.title }] : [])
	]);

	let selectedTab = $state<CRMTab>('relationships');
	let selectedStatus = $state<CRMOrganizationStatusFilter>('all');
	let selectedType = $state<CRMOrganizationTypeFilter>('all');
	let selectedImportance = $state<CRMImportanceFilter>('all');
	let selectedLastContact = $state<CRMLastContactWindow>('all');
	let selectedStage = $state<CRMOpportunityStage | 'all'>('all');
	let relationshipView = $state<RelationshipView>('all');
	let activityView = $state<ActivityView>('all');
	let pipelineView = $state<PipelineView>('table');
	let selectedPipeline = $state<PipelineFilter>('all');
	let selectedOrganizationID = $state<string | null>(null);
	let selectedContactID = $state<string | null>(null);
	let selectedOpportunityID = $state<string | null>(null);
	let requestedOpportunityStage = $state<CRMOpportunityStage | undefined>();
	let selectedActivityID = $state<string | null>(null);
	let isOrganizationSheetOpen = $state(false);
	let isRelationshipEditOpen = $state(false);
	let isContactEditOpen = $state(false);
	let isOpportunityEditOpen = $state(false);
	let isActivityEditOpen = $state(false);
	let isRecordSheetOpen = $state(false);
	let createKind = $state<CRMRecordKind>('relationship');
	let createOrganizationID = $state('');
	let feedbackMessage = $state('');
	let activityKinds = $derived(controller.activityKindOptions);
	let organizationTypeFilters = $derived<CRMOrganizationTypeFilter[]>(['all', ...controller.organizationTypeOptions]);
	let statusFilterOptions = $derived(
		crmOrganizationStatusOptions
			.filter((status) => status !== 'all')
			.map((status) => ({ value: status, label: text.organizationStatuses[status as CRMOrganizationStatus] }))
	);
	let typeFilterOptions = $derived(
		organizationTypeFilters
			.filter((organizationType) => organizationType !== 'all')
			.map((organizationType) => ({
				value: organizationType,
				label: crmDefinitionLabel(organizationTypeDefinitions, text.organizationTypes, organizationType)
			}))
	);
	let pipelineFilterOptions = $derived(
		controller.pipelines.map((pipeline) => ({ value: pipeline.pipeline, label: pipeline.label }))
	);
	let activityKindFilterOptions = $derived(
		activityKinds.map((activityKind) => ({ value: activityKind, label: crmLabel(text.activityKinds, activityKind) }))
	);
	let importanceFilterOptions = $derived(
		crmImportanceOptions.map((importance) => ({ value: importance, label: text.importanceLabels[importance] }))
	);
	let lastContactFilterOptions = $derived(
		crmLastContactWindowOptions.map((window) => ({ value: window, label: text.lastContactWindows[window] }))
	);
	let stageFilterOptions = $derived(
		controller.stages.map((stage) => ({ value: stage.stage, label: opportunityStageLabel(controller.stages, stage.stage, text) }))
	);
	let relationshipFacets = $derived<CRMRelationshipFacets>({
		status: selectedStatus,
		type: selectedType,
		importance: selectedImportance,
		lastContact: selectedLastContact
	});
	let hasRelationshipFacets = $derived(
		selectedStatus !== 'all' || selectedType !== 'all' || selectedImportance !== 'all' || selectedLastContact !== 'all'
	);
	let hasPipelineFacets = $derived(selectedPipeline !== 'all' || selectedStage !== 'all');

	function resetRelationshipFacets(): void {
		selectedStatus = 'all';
		selectedType = 'all';
		selectedImportance = 'all';
		selectedLastContact = 'all';
	}

	function resetPipelineFacets(): void {
		selectedPipeline = 'all';
		selectedStage = 'all';
	}

	$effect(() => {
		const requestedOrganizationID = page.url.searchParams.get('organization');
		const requestedContactID = page.url.searchParams.get('contact');
		if (!requestedOrganizationID && !requestedContactID) return;
		if (requestedOrganizationID) openOrganization(requestedOrganizationID);
		if (requestedContactID) openContactEdit(requestedContactID);
		replaceState('/crm', page.state);
	});

	onMount(() => {
		void controller.load(page.data.session?.email ?? '');
		if (isSupabaseConfigured()) void supabaseMemberRole().then((role) => (isAdmin = role === 'admin'));
	});

	$effect(() => {
		if (!isAdmin && selectedTab === 'definitions') selectedTab = 'relationships';
	});

	$effect(() => {
		if (!isViewCurrencyAvailable()) return;
		void crmViewCurrency.follow(companyBaseCurrency, opportunityCurrencies);
	});

	$effect(() => {
		if (selectedPipeline !== 'all' && !controller.pipelines.some((pipeline) => pipeline.pipeline === selectedPipeline)) {
			selectedPipeline = 'all';
		}
		selectedOrganizationID ??= controller.organizations[0]?.id ?? null;
		selectedContactID ??= controller.contacts[0]?.id ?? null;
		selectedOpportunityID ??= controller.opportunities[0]?.id ?? null;
		selectedActivityID ??= controller.activities[0]?.id ?? null;
	});

	let filteredOrganizations = $derived(controller.organizations.filter((organization) => {
		if (!organizationMatchesFacets(organization, relationshipFacets, currentCRMDate())) return false;
		return relationshipView === 'all' || organization.ownerName === controller.currentOwnerName;
	}));
	let filteredActivities = $derived(
		activityView === 'all' ? controller.activities : controller.activities.filter((activity) => activity.kind === activityView)
	);
	let pipelineOpportunities = $derived(controller.opportunities.filter((opportunity) => {
		if (selectedPipeline !== 'all' && (opportunity.pipeline ?? opportunity.kind) !== selectedPipeline) return false;
		return selectedStage === 'all' || opportunity.stage === selectedStage;
	}));
	let selectedOrganization = $derived(selectedOrganizationID ? findOrganizationByID(controller.organizations, selectedOrganizationID) : undefined);
	let selectedContact = $derived(selectedContactID ? controller.contacts.find((contact) => contact.id === selectedContactID) : undefined);
	let selectedOpportunity = $derived(selectedOpportunityID ? controller.opportunities.find((opportunity) => opportunity.id === selectedOpportunityID) : undefined);
	let selectedActivity = $derived(selectedActivityID ? controller.activities.find((activity) => activity.id === selectedActivityID) : undefined);
	let opportunityCurrencies = $derived([...new Set(controller.opportunities.map((opportunity) => opportunity.currency))]);
	let activityTaskDefinitions = $derived<TaskDefinitions>({
		categories: controller.taskVocabulary.businesses?.map((entry) => entry.name) ?? [],
		categoryColors: Object.fromEntries(
			(controller.taskVocabulary.businesses ?? []).flatMap((entry) =>
				entry.color ? [[entry.name, entry.color]] : []
			)
		),
		types: controller.taskVocabulary.types?.map((entry) => entry.name) ?? [],
		typeColors: Object.fromEntries(
			(controller.taskVocabulary.types ?? []).flatMap((entry) => (entry.color ? [[entry.name, entry.color]] : []))
		),
		sizes: []
	});

	let kpiCards = $derived(buildCRMKPICards(currencyCatalogue, controller.organizations, controller.opportunities, controller.nextActions, controller.pipelines, controller.stages, text, undefined, currentLocale.value, crmViewCurrency));

	function openOrganization(organizationID: string): void {
		selectedOrganizationID = organizationID;
		isOrganizationSheetOpen = true;
	}

	function openOrganizationEdit(organizationID: string): void {
		selectedOrganizationID = organizationID;
		isOrganizationSheetOpen = false;
		isRelationshipEditOpen = true;
	}

	function openOpportunityEdit(opportunityID: string): void {
		selectedOpportunityID = opportunityID;
		requestedOpportunityStage = undefined;
		isOpportunityEditOpen = true;
	}

	function openContactEdit(contactID: string): void {
		selectedContactID = contactID;
		isContactEditOpen = true;
	}

	function openRelationshipContactEdit(contactID: string): void {
		isRelationshipEditOpen = false;
		openContactEdit(contactID);
	}

	function openRelationshipContactCreate(organizationID: string): void {
		isRelationshipEditOpen = false;
		createKind = 'contact';
		createOrganizationID = organizationID;
		isRecordSheetOpen = true;
	}

	function openActivityEdit(activityID: string): void {
		selectedActivityID = activityID;
		isActivityEditOpen = true;
	}

	function openCreateSheet(kind: CRMRecordKind): void {
		createKind = kind;
		createOrganizationID = '';
		isRecordSheetOpen = true;
	}

	function selectPipeline(value: string): void {
		selectedPipeline = value as PipelineFilter;
	}

	async function handleCreate(draft: CRMCreateDraft): Promise<void> {
		await controller.create(draft);
		selectedTab = draft.kind === 'relationship' ? 'relationships' : draft.kind === 'contact' ? 'contacts' : draft.kind === 'progress' ? 'pipeline' : 'activities';
		feedbackMessage = text.savedToService;
	}

	async function saveOrganization(organization: CRMOrganization): Promise<void> {
		await controller.saveOrganization(organization);
		feedbackMessage = text.savedToService;
	}

	async function archiveOrganization(organizationID: string): Promise<void> {
		await controller.archiveOrganization(organizationID);
		isRelationshipEditOpen = false;
		feedbackMessage = text.archivedFromService;
	}

	async function saveContact(contact: CRMContact): Promise<void> {
		await controller.saveContact(contact);
		feedbackMessage = text.savedToService;
	}

	async function saveOpportunity(opportunity: CRMOpportunity): Promise<void> {
		await controller.saveOpportunity(opportunity);
		feedbackMessage = text.savedToService;
	}

	async function archiveOpportunity(opportunityID: string): Promise<void> {
		await controller.archiveOpportunity(opportunityID);
		isOpportunityEditOpen = false;
		feedbackMessage = text.archivedFromService;
	}

	async function saveActivity(activity: CRMActivity, draft: CRMActivityEditDraft): Promise<void> {
		await controller.saveActivity(activity, draft);
		feedbackMessage = text.savedToService;
	}

	function moveOpportunity(request: CRMPipelineBoardMoveRequest): void {
		const outcome = controller.stages.find((stage) => stage.stage === request.targetStage)?.outcome;
		if (outcome === 'won' || outcome === 'lost') {
			selectedOpportunityID = request.opportunityID;
			requestedOpportunityStage = request.targetStage;
			isOpportunityEditOpen = true;
			return;
		}
		void controller.moveOpportunity(request)
			.then(() => {
				feedbackMessage = text.savedToService;
			})
			.catch(() => {
				feedbackMessage = '';
			});
	}
</script>

<svelte:head><title>{text.pageTitle}</title></svelte:head>

<main data-crm-ready={!controller.isLoading && !controller.errorMessage} class="grid min-h-full w-full content-start gap-4 px-4 py-6 md:px-8">
	{#if feedbackMessage}
		<div role="status" class="flex items-center gap-2 rounded-md border bg-muted/40 px-3 py-2 text-sm"><CheckCircle2Icon class="size-4 text-primary" /><span>{feedbackMessage}</span><Button type="button" variant="ghost" size="icon-sm" class="ml-auto" aria-label={text.cancel} onclick={() => (feedbackMessage = '')}><XIcon /></Button></div>
	{/if}
	{#if controller.errorMessage}
		<div role="alert" class="flex flex-wrap items-center gap-3 rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
			<AlertCircleIcon class="size-4" /><span class="flex-1">{controller.errorMessage}</span>
			<Button type="button" variant="outline" size="sm" onclick={() => controller.load(page.data.session?.email ?? '')}>{text.retry}</Button>
		</div>
	{/if}

	{#if controller.isLoading}
		<div role="status" class="flex min-h-64 items-center justify-center gap-2 rounded-md border text-sm text-muted-foreground"><LoaderCircleIcon class="size-4 animate-spin" />{text.loading}</div>
	{:else if !controller.errorMessage}
		<section class="min-w-0" data-crm-metrics>
			<Card.Root class="grid min-w-0 grid-cols-2 gap-px bg-border py-0 lg:grid-cols-4">
				{#each kpiCards as card (card.id)}<CRMKPICell {card} />{/each}
			</Card.Root>
		</section>

		<UnderlineTabs.Root bind:value={selectedTab} class="min-w-0 gap-3">
			<UnderlineTabs.List class="overflow-x-clip">{#each tabItems as tab (tab.value)}<UnderlineTabs.Trigger value={tab.value}>{tab.label}</UnderlineTabs.Trigger>{/each}</UnderlineTabs.List>

			<UnderlineTabs.Content value="relationships" class="grid min-w-0 gap-3 pb-24">
				<div class="flex min-w-0 flex-wrap items-center gap-2">
					<Tabs.Root value={relationshipView} onValueChange={(value) => (relationshipView = value as RelationshipView)} aria-label={text.relationships}><Tabs.List><Tabs.Trigger value="all">{text.allRelationships}</Tabs.Trigger><Tabs.Trigger value="mine">{text.myRelationships}</Tabs.Trigger></Tabs.List></Tabs.Root>
					<FilterCombobox bind:value={selectedStatus} options={statusFilterOptions} label={text.status} clearValue="all" searchable={false} class="w-auto" />
					<FilterCombobox bind:value={selectedType} options={typeFilterOptions} label={text.type} clearValue="all" searchable={false} class="w-auto" />
					<FilterCombobox bind:value={selectedImportance} options={importanceFilterOptions} label={text.importance} clearValue="all" searchable={false} class="w-auto" />
					<FilterCombobox bind:value={selectedLastContact} options={lastContactFilterOptions} label={text.lastContact} clearValue="all" searchable={false} class="w-auto" />
					<CRMViewCurrencySelect {text} {currencyCatalogue} {companyBaseCurrency} sourceCurrencies={opportunityCurrencies} />
					{#if hasRelationshipFacets}
						<Button type="button" variant="ghost" onclick={resetRelationshipFacets}>{text.resetFilters}</Button>
					{/if}
					<Button type="button" class="ml-auto" onclick={() => openCreateSheet('relationship')}><PlusIcon data-icon="inline-start" />{text.newRelationship}</Button>
				</div>
				<CRMRelationshipTable organizations={filteredOrganizations} contacts={controller.contacts} {organizationTypeDefinitions} {currencyCatalogue} {text} {openOrganization} />
			</UnderlineTabs.Content>

			<UnderlineTabs.Content value="contacts" class="grid min-w-0 gap-3 pb-24">
				<div class="flex min-w-0 flex-wrap items-center gap-2">
					<Button type="button" class="ml-auto" onclick={() => openCreateSheet('contact')}><PlusIcon data-icon="inline-start" />{text.newContact}</Button>
				</div>
				<CRMContactTable contacts={controller.contacts} organizations={controller.organizations} people={controller.people} {text} onEdit={openContactEdit} />
			</UnderlineTabs.Content>

			<UnderlineTabs.Content value="pipeline" class="grid min-w-0 gap-3 pb-24">
				<div class="flex min-w-0 flex-wrap items-center gap-2">
					<Tabs.Root bind:value={pipelineView} aria-label={text.pipeline}><Tabs.List><Tabs.Trigger value="table">{text.tableView}</Tabs.Trigger><Tabs.Trigger value="board">{text.boardView}</Tabs.Trigger></Tabs.List></Tabs.Root>
					<FilterCombobox value={selectedPipeline} options={pipelineFilterOptions} label={text.progressKind} clearValue="all" onSelect={selectPipeline} searchable={false} class="w-auto" />
					<FilterCombobox bind:value={selectedStage} options={stageFilterOptions} label={text.stage} clearValue="all" searchable={false} class="w-auto">
						{#snippet optionContent(option)}
							{@const StageIcon = dealStageIcon(option.value)}
							<StageIcon class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
							<span class="min-w-0 truncate">{option.label}</span>
						{/snippet}
						{#snippet selectedContent(option)}
							{@const StageIcon = dealStageIcon(option.value)}
							<StageIcon class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
							<span class="truncate">{text.stage}: {option.label}</span>
						{/snippet}
					</FilterCombobox>
					<CRMViewCurrencySelect {text} {currencyCatalogue} {companyBaseCurrency} sourceCurrencies={opportunityCurrencies} />
					{#if hasPipelineFacets}
						<Button type="button" variant="ghost" onclick={resetPipelineFacets}>{text.resetFilters}</Button>
					{/if}
					<Button type="button" class="ml-auto" onclick={() => openCreateSheet('progress')}><PlusIcon data-icon="inline-start" />{text.newOpportunity}</Button>
				</div>
				{#if pipelineView === 'table'}
					<CRMProgressTable opportunities={pipelineOpportunities} organizations={controller.organizations} pipelines={controller.pipelines} nextActions={controller.nextActions} stages={controller.stages} {text} onEdit={openOpportunityEdit} />
				{:else}
					<CRMPipelineBoard opportunities={pipelineOpportunities} organizations={controller.organizations} pipelines={controller.pipelines} nextActions={controller.nextActions} stages={controller.stages} {text} onMove={moveOpportunity} />
				{/if}
			</UnderlineTabs.Content>

			<UnderlineTabs.Content value="activities" class="grid min-w-0 gap-3 pb-24">
				<div class="flex min-w-0 flex-wrap items-center gap-2">
					<FilterCombobox bind:value={activityView} options={activityKindFilterOptions} label={text.activityKind} clearValue="all" searchable={false} class="w-auto" />
					{#if activityView !== 'all'}
						<Button type="button" variant="ghost" onclick={() => (activityView = 'all')}>{text.resetFilters}</Button>
					{/if}
					<Button type="button" class="ml-auto" onclick={() => openCreateSheet('activity')}><PlusIcon data-icon="inline-start" />{text.logActivity}</Button>
				</div>
				<CRMActivityTable activities={filteredActivities} organizations={controller.organizations} opportunities={controller.opportunities} people={controller.people} taskDefinitions={activityTaskDefinitions} {text} onEdit={openActivityEdit} />
			</UnderlineTabs.Content>

			<UnderlineTabs.Content value="reports" class="min-w-0 pb-24"><CRMReportDashboard organizations={controller.organizations} opportunities={controller.opportunities} pipelines={controller.pipelines} nextActions={controller.nextActions} stages={controller.stages} {currencyCatalogue} {companyBaseCurrency} sourceCurrencies={opportunityCurrencies} {text} onOpenOrganization={openOrganization} /></UnderlineTabs.Content>
			<UnderlineTabs.Content value="definitions" class="min-w-0 pb-24"><CRMDefinitionsEditor vocabulary={controller.vocabulary} isSaving={controller.isSaving} errorMessage={controller.errorMessage} text={text.definitions} onSave={(vocabulary) => controller.saveVocabulary(vocabulary)} /></UnderlineTabs.Content>
		</UnderlineTabs.Root>
	{/if}
</main>

<CRMRelationshipDetailSheet bind:open={isOrganizationSheetOpen} organization={selectedOrganization} contacts={controller.contacts} opportunities={controller.opportunities} activities={controller.activities} stages={controller.stages} {organizationTypeDefinitions} {currencyCatalogue} {text} onEdit={openOrganizationEdit} />
<CRMRelationshipEditSheet bind:open={isRelationshipEditOpen} organization={selectedOrganization} contacts={controller.contacts} organizationTypeOptions={controller.organizationTypeOptions} {organizationTypeDefinitions} people={controller.people} groups={controller.groups} {text} onSave={saveOrganization} onArchive={archiveOrganization} onEditContact={openRelationshipContactEdit} onCreateContact={openRelationshipContactCreate} />
<CRMContactEditSheet bind:open={isContactEditOpen} contact={selectedContact} organizations={controller.organizations} opportunities={controller.opportunities} {text} onSave={saveContact} />
<CRMOpportunityEditSheet {currencyCatalogue} {companyBaseCurrency} bind:open={isOpportunityEditOpen} opportunity={selectedOpportunity} organizations={controller.organizations} contacts={controller.contacts} pipelines={controller.pipelines} stages={controller.stages} businessOptions={controller.businessOptions} people={controller.people} groups={controller.groups} requestedStage={requestedOpportunityStage} {text} onSave={saveOpportunity} onArchive={archiveOpportunity} />
<CRMActivityDetailSheet bind:open={isActivityEditOpen} activity={selectedActivity} organizations={controller.organizations} opportunities={controller.opportunities} contacts={controller.contacts} businessOptions={controller.businessOptions} activityKindOptions={controller.activityKindOptions} people={controller.people} groups={controller.groups} {text} onSave={saveActivity} />
<CRMRecordSheet {currencyCatalogue} {companyBaseCurrency} bind:open={isRecordSheetOpen} initialKind={createKind} initialOrganizationID={createOrganizationID} organizations={controller.organizations} contacts={controller.contacts} opportunities={controller.opportunities} pipelines={controller.pipelines} stages={controller.stages} businessOptions={controller.businessOptions} organizationTypeOptions={controller.organizationTypeOptions} {organizationTypeDefinitions} activityKindOptions={controller.activityKindOptions} defaultOwnerPersonID={controller.currentOwnerPersonID} people={controller.people} groups={controller.groups} {text} onCreate={handleCreate} />
