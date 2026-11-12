import type { SupabaseClient } from '@supabase/supabase-js';
import { attendanceAdd, attendanceDelete, attendanceList, attendanceUpdate } from './attendance-tools';
import { currentAttendance } from './current-attendance';
import { teamAttendance, legacyTeamAttendance } from './team-attendance';
import { exchangeRatesOf } from '$lib/server/configured-exchange-rates';
import { recordContextOf, type RecordContext, type ToolPlace } from './company';
import type { TaskChangeAnnouncer } from './task-change';
import type { AttendanceAnnouncer } from './attendance-announcement';
import {
	attendanceLeavePolicyGet,
	attendanceLeavePolicySet,
	attendanceWorkPolicyGet,
	attendanceWorkPolicySet,
	companyHolidayAdd,
	companyHolidayDelete,
	companyHolidayList,
	companyHolidayUpdate,
	companyInfoGet,
	companyInfoSet,
	companySettingsGet,
	companySettingsUpdate
} from './company-tools';
import { companyImageUpload } from './service-files';
import { companyCreditsGet } from './company-credits';
import { crmActivityList, crmActivitySave } from './crm-activity-tools';
import {
	crmContactAdd,
	crmContactArchive,
	crmContactList,
	crmContactUpdate,
	crmOpportunityAdd,
	crmOpportunityArchive,
	crmOpportunityList,
	crmOpportunityMove,
	crmOpportunityUpdate,
	crmOrganizationAdd,
	crmOrganizationArchive,
	crmOrganizationList,
	crmOrganizationUpdate,
	crmVocabularyGet,
	crmVocabularySet
} from './crm-tools';
import { crmRecordImport } from './crm-import-tools';
import { crmImportRecordsOf, crmRecordImportPreview } from './crm-import-file';
import type { CarriedWorkspaceFile } from '../carried-files';
import {
	companyDocumentDownload,
	companyDocumentList,
	companyDocumentRegister,
	companyDocumentRecall,
	companyDocumentSearch,
	companyDocumentUpdate,
	companyDocumentUpload,
	companyMetricList,
	companyMetricRecord,
	companyRecordAdd,
	companyRecordDelete,
	companyRecordList,
	companyRecordUpdate
} from './company-ledger-tools';
import { companyEquityAdd, companyEquityDelete, companyEquityList } from './company-equity-tools';
import {
	eventAdd,
	eventDelete,
	eventList,
	eventUpdate
} from './event-tools';
import { calendarSubscriptionAdd, calendarSubscriptionDelete, calendarSubscriptionGet } from './calendar-subscription';
import {
	conversationMute,
	conversationUnmute,
	notificationSettingsGet,
	notificationSettingsSet
} from './notification-tools';
import { channelOrderGet, channelOrderSet, channelPin, channelUnpin } from './channel-order-tools';
import {
	leaveBalance,
	leaveDecide,
	leaveDelete,
	leaveGrantSet,
	leaveList,
	leaveRequest,
	leaveReturnEarly,
	leaveUpdate
} from './leave-tools';
import { personInvite, personList, personPasswordReset, personUpdate, personWithdraw } from './people-tools';
import { companyDataRoomGet, companyDataRoomCategorySet, companyDataRoomFieldSet, companyDataRoomFieldNameSet,
	companyDataRoomShareCreate, companyDataRoomShareRevoke,
	companyDataRoomLinksGet, companyDataRoomLinkCreate, companyDataRoomLinkRevoke,
	companyDataRoomInvitationAccept, companyDataRoomInvitationSend } from './data-room';
import { taskInvite, taskInvitationDelete, taskInvitationList } from './task-invitation';
import {
	companyCircleChannelAdd, companyCircleChannelDelete, companyCircleList, companyCircleMemberSet, companyCircleSet
} from './circle';
import { taskAdd, taskDelete, taskList, taskBoard, taskUpdate, taskVocabularySet } from './task-tools';
import { teamAdd, teamDelete, teamList, teamUpdate } from './team-tools';
import {
	companyHolidayDeleteTarget,
	companyRecordDeleteTarget,
	crmContactArchiveTarget,
	crmOpportunityArchiveTarget,
	crmOrganizationArchiveTarget,
	eventDeleteTarget,
	personTarget,
	taskDeleteTarget,
	type PreviewedTarget
} from './preview';
import { leavesTaskLabelsUndecided, type TaskLabelDecider } from './task-labels';
import { answererOfTool, toolNamesAnsweredBy } from '../catalog';
import { capabilityToolResultSchema, legacyAttendanceListResultSchema } from '../catalog/tools';
import { readWithNullAsAbsent } from '../null-as-absent';
import { sentencesOfSchemaRefusal } from '../schema-sentences';
import { Refusal, answerOfRefusal } from '../refusal';
import { attendanceAddInputSchema, attendanceCurrentGetInputSchema, attendanceDeleteInputSchema, attendanceListInputSchema, attendanceUpdateInputSchema, calendarAddInputSchema, calendarDeleteInputSchema, calendarListInputSchema, calendarUpdateInputSchema, leaveBalanceInputSchema, leaveDecideInputSchema, leaveDeleteInputSchema, leaveGrantSetInputSchema, leaveListInputSchema, leaveRequestInputSchema, leaveReturnEarlyInputSchema, leaveUpdateInputSchema, taskAddInputSchema, taskBoardInputSchema, taskDeleteInputSchema, taskListInputSchema, taskUpdateInputSchema } from '../catalog/tools';
import { taskVocabularySetInputSchema } from '../catalog/labels';
import { taskInvitationDeleteInputSchema, taskInvitationListInputSchema, taskInviteInputSchema } from '../catalog/task-invitation';
import { calendarSubscriptionInputSchema } from '../catalog/calendar-subscription';
import { personInviteInputSchema, personListInputSchema, personPasswordResetInputSchema, personUpdateInputSchema, personWithdrawInputSchema, teamAddInputSchema, teamDeleteInputSchema, teamListInputSchema, teamUpdateInputSchema } from '../catalog/people';
import { attendanceTeamPageInputSchema } from '$lib/attendance/team-page';
import { attendanceLeavePolicyGetInputSchema, attendanceLeavePolicySetInputSchema, attendanceWorkPolicyGetInputSchema, attendanceWorkPolicySetInputSchema, companyHolidayAddInputSchema, companyHolidayDeleteInputSchema, companyHolidayListInputSchema, companyHolidayUpdateInputSchema, companySettingsGetInputSchema, companySettingsUpdateInputSchema } from '../catalog/settings';
import { companyCreditsGetInputSchema } from '../catalog/credits';
import { companyDocumentDownloadInputSchema, companyDocumentListInputSchema, companyDocumentRegisterInputSchema, companyDocumentSearchInputSchema, companyDocumentUpdateInputSchema, companyDocumentUploadInputSchema, companyEquityAddInputSchema, companyEquityDeleteInputSchema, companyEquityListInputSchema, companyImageUploadInputSchema, companyInfoGetInputSchema, companyInfoSetInputSchema, companyMetricListInputSchema, companyMetricRecordInputSchema, companyRecordAddInputSchema, companyRecordDeleteInputSchema, companyRecordListInputSchema, companyRecordUpdateInputSchema } from '../catalog/company';
import { recallSourceInputSchema } from '../catalog/protocol';
import { circleChannelAddInputSchema, circleChannelDeleteInputSchema, circleListInputSchema, circleMemberSetInputSchema, circleSetInputSchema } from '../catalog/circle';
import { dataRoomCategorySetInputSchema, dataRoomFieldNameSetInputSchema, dataRoomFieldSetInputSchema, dataRoomGetInputSchema, dataRoomInvitationAcceptInputSchema, dataRoomInvitationSendInputSchema, dataRoomLinksGetInputSchema, dataRoomShareCreateInputSchema, dataRoomShareRevokeInputSchema } from '../catalog/data-room';
import { dataRoomLinkInputSchema, dataRoomLinkRevokeSchema } from '$lib/data-room/links';
import { conversationMuteInputSchema, notificationSettingsGetInputSchema, notificationSettingsSetInputIntentSchema } from '../catalog/notifications';
import { channelOrderGetInputSchema, channelOrderSetInputSchema, channelPinInputSchema } from '../catalog/channel-order';
import { crmActivityListInputSchema, crmActivitySaveInputSchema, crmContactAddInputSchema, crmContactArchiveInputSchema, crmContactListInputSchema, crmContactUpdateInputSchema, crmOpportunityAddInputSchema, crmOpportunityArchiveInputSchema, crmOpportunityListInputSchema, crmOpportunityMoveInputSchema, crmOpportunityUpdateInputSchema, crmOrganizationAddInputSchema, crmOrganizationArchiveInputSchema, crmOrganizationListInputSchema, crmOrganizationUpdateInputSchema, crmRecordImportInputSchema, crmVocabularyGetInputSchema, crmVocabularySetInputSchema } from '../catalog/crm';
import type { z } from 'zod';
import { inputOfTheCall, parsedToolInput } from '../tool-input';

type ToolInput = Record<string, unknown>;
type CallCarried = {
	files: CarriedWorkspaceFile[];
	place?: ToolPlace;
	announceTaskChange?: TaskChangeAnnouncer;
	announceAttendance?: AttendanceAnnouncer;
};
type RecordToolRun<Input> = (context: RecordContext, input: Input, carried: CallCarried) => unknown;
type RecordToolPreview<Input> = (
	context: RecordContext,
	input: Input,
	files: CarriedWorkspaceFile[]
) => Promise<PreviewedTarget | null>;
type RecordReach = { caller: SupabaseClient; context: () => Promise<RecordContext> };
type BoundRecordTool = {
	run: (reach: RecordReach, carried: CallCarried) => unknown;
	preview?: (context: RecordContext, files: CarriedWorkspaceFile[]) => Promise<PreviewedTarget | null>;
};
type RecordTool = { inputSchema: z.ZodType; boundTo: (asked: unknown) => BoundRecordTool };

function recordTool<Input>(
	inputSchema: z.ZodType<Input>,
	run: RecordToolRun<Input>,
	preview?: RecordToolPreview<Input>
): RecordTool {
	return {
		inputSchema,
		boundTo: (asked) => {
			const input = parsedToolInput(inputSchema, asked);
			const runWith: BoundRecordTool['run'] = async (reach, carried) => run(await reach.context(), input, carried);
			if (!preview) return { run: runWith };
			return { run: runWith, preview: (context, files) => preview(context, input, files) };
		}
	};
}

function callerTool<Input>(inputSchema: z.ZodType<Input>, run: (caller: SupabaseClient, input: Input) => unknown): RecordTool {
	return {
		inputSchema,
		boundTo: (asked) => {
			const input = parsedToolInput(inputSchema, asked);
			return { run: (reach) => run(reach.caller, input) };
		}
	};
}

function exactAttendancePersonIDs(name: string, input: ToolInput): string[] | undefined {
	if (name !== 'attendance_list' && name !== 'leave_list') return;
	const hints = input.personHints;
	if (!Array.isArray(hints) || hints.length !== 1 || typeof hints[0] !== 'string') return;
	return /^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/i.test(hints[0]) ? [hints[0]] : undefined;
}

const readsWithoutPeople = new Set([
	'company_settings_get', 'company_holiday_list', 'crm_organization_list',
	'crm_contact_list', 'crm_opportunity_list', 'crm_vocabulary_get',
	'calendar_subscription_get', 'calendar_subscription_add', 'calendar_subscription_delete',
	'channel_order_get', 'channel_order_set', 'channel_pin', 'channel_unpin',
	'task_invite', 'task_invitation_list', 'task_invitation_delete'
]);

function placeTheAppAnswersAt(name: string, place: ToolPlace | undefined): ToolPlace {
	if (!place) throw new Error(`${name} names an address in the app, and this call did not say where the app answers`);
	return place;
}

const toolsOverTheRecord: Record<string, RecordTool> = {
	task_add: recordTool(taskAddInputSchema, (context, input, { announceTaskChange }) => taskAdd(context, input, announceTaskChange)),
	task_update: recordTool(taskUpdateInputSchema, (context, input, { announceTaskChange }) => taskUpdate(context, input, announceTaskChange)),
	task_list: recordTool(taskListInputSchema, (context, input) => taskList(context, input)),
	task_board_get: recordTool(taskBoardInputSchema, (context, input) => taskBoard(context, input)),
	task_delete: recordTool(taskDeleteInputSchema, (context, input) => taskDelete(context, input), taskDeleteTarget),
	task_vocabulary_set: recordTool(taskVocabularySetInputSchema, (context, input) => taskVocabularySet(context, input)),
	task_invite: recordTool(taskInviteInputSchema, (context, input, { place }) => taskInvite(context, input, placeTheAppAnswersAt('task_invite', place))),
	task_invitation_list: recordTool(taskInvitationListInputSchema, (context) => taskInvitationList(context)),
	task_invitation_delete: recordTool(taskInvitationDeleteInputSchema, (context, input) => taskInvitationDelete(context, input)),
	event_add: recordTool(calendarAddInputSchema, (context, input) => eventAdd(context, input)),
	event_update: recordTool(calendarUpdateInputSchema, (context, input) => eventUpdate(context, input)),
	event_list: recordTool(calendarListInputSchema, (context, input) => eventList(context, input)),
	event_delete: recordTool(calendarDeleteInputSchema, (context, input) => eventDelete(context, input), eventDeleteTarget),
	calendar_subscription_get: recordTool(calendarSubscriptionInputSchema, (context) => calendarSubscriptionGet(context)),
	calendar_subscription_add: recordTool(calendarSubscriptionInputSchema, (context, _input, { place }) =>
		calendarSubscriptionAdd(context, placeTheAppAnswersAt('calendar_subscription_add', place))),
	calendar_subscription_delete: recordTool(calendarSubscriptionInputSchema, (context) => calendarSubscriptionDelete(context)),
	person_list: recordTool(personListInputSchema, (context, input) => personList(context, input)),
	person_update: recordTool(personUpdateInputSchema, (context, input) => personUpdate(context, input)),
	person_invite: recordTool(personInviteInputSchema, (context, input) => personInvite(context, input)),
	person_withdraw: recordTool(personWithdrawInputSchema, (context, input) => personWithdraw(context, input), personTarget),
	person_password_reset: recordTool(personPasswordResetInputSchema, (context, input) => personPasswordReset(context, input), personTarget),
	team_list: recordTool(teamListInputSchema, (context) => teamList(context)),
	team_add: recordTool(teamAddInputSchema, (context, input) => teamAdd(context, input)),
	team_update: recordTool(teamUpdateInputSchema, (context, input) => teamUpdate(context, input)),
	team_delete: recordTool(teamDeleteInputSchema, (context, input) => teamDelete(context, input)),
	leave_list: recordTool(leaveListInputSchema, (context, input) => leaveList(context, input)),
	leave_balance: recordTool(leaveBalanceInputSchema, (context, input) => leaveBalance(context, input)),
	leave_request: recordTool(leaveRequestInputSchema, (context, input, { announceAttendance }) => leaveRequest(context, input, announceAttendance)),
	leave_update: recordTool(leaveUpdateInputSchema, (context, input) => leaveUpdate(context, input)),
	leave_delete: recordTool(leaveDeleteInputSchema, (context, input) => leaveDelete(context, input)),
	leave_decide: recordTool(leaveDecideInputSchema, (context, input) => leaveDecide(context, input)),
	leave_grant_set: recordTool(leaveGrantSetInputSchema, (context, input) => leaveGrantSet(context, input)),
	leave_return_early: recordTool(leaveReturnEarlyInputSchema, (context, input, { announceAttendance }) => leaveReturnEarly(context, input, announceAttendance)),
	attendance_list: recordTool(attendanceListInputSchema, async (context, input) => legacyAttendanceListResultSchema.parse(await attendanceList(context, input))),
	attendance_changes_page_get: recordTool(attendanceListInputSchema, (context, input) => attendanceList(context, input)),
	attendance_current_get: callerTool(attendanceCurrentGetInputSchema, (caller) => currentAttendance(caller)),
	attendance_team_page_get: callerTool(attendanceTeamPageInputSchema, legacyTeamAttendance),
	attendance_team_dashboard_get: callerTool(attendanceTeamPageInputSchema, teamAttendance),
	attendance_add: recordTool(attendanceAddInputSchema, (context, input, { announceAttendance }) => attendanceAdd(context, input, announceAttendance)),
	attendance_update: recordTool(attendanceUpdateInputSchema, (context, input) => attendanceUpdate(context, input)),
	attendance_delete: recordTool(attendanceDeleteInputSchema, (context, input) => attendanceDelete(context, input)),
	attendance_work_policy_get: recordTool(attendanceWorkPolicyGetInputSchema, (context) => attendanceWorkPolicyGet(context)),
	attendance_work_policy_set: recordTool(attendanceWorkPolicySetInputSchema, (context, input) => attendanceWorkPolicySet(context, input)),
	attendance_leave_policy_get: recordTool(attendanceLeavePolicyGetInputSchema, (context) => attendanceLeavePolicyGet(context)),
	attendance_leave_policy_set: recordTool(attendanceLeavePolicySetInputSchema, (context, input) => attendanceLeavePolicySet(context, input)),
	company_settings_get: recordTool(companySettingsGetInputSchema, (context, input) => companySettingsGet(context, input)),
	company_settings_update: recordTool(companySettingsUpdateInputSchema, (context, input) => companySettingsUpdate(context, input)),
	company_credits_get: recordTool(companyCreditsGetInputSchema, (context) => companyCreditsGet(context)),
	company_info_get: recordTool(companyInfoGetInputSchema, (context, input) => companyInfoGet(context, input)),
	company_info_set: recordTool(companyInfoSetInputSchema, (context, input) => companyInfoSet(context, input)),
	company_image_upload: recordTool(companyImageUploadInputSchema, (context, input) => companyImageUpload(context, input)),
	company_holiday_list: recordTool(companyHolidayListInputSchema, (context, input) => companyHolidayList(context, input)),
	company_holiday_add: recordTool(companyHolidayAddInputSchema, (context, input) => companyHolidayAdd(context, input)),
	company_holiday_update: recordTool(companyHolidayUpdateInputSchema, (context, input) => companyHolidayUpdate(context, input)),
	company_holiday_delete: recordTool(companyHolidayDeleteInputSchema, (context, input) => companyHolidayDelete(context, input), companyHolidayDeleteTarget),
	company_metric_record: recordTool(companyMetricRecordInputSchema, (context, input) => companyMetricRecord(context, input)),
	company_metric_list: recordTool(companyMetricListInputSchema, (context, input) => companyMetricList(context, input)),
	company_record_add: recordTool(companyRecordAddInputSchema, (context, input) => companyRecordAdd(context, input)),
	company_record_list: recordTool(companyRecordListInputSchema, (context, input) => companyRecordList(context, input)),
	company_record_update: recordTool(companyRecordUpdateInputSchema, (context, input) => companyRecordUpdate(context, input)),
	company_record_delete: recordTool(companyRecordDeleteInputSchema, (context, input) => companyRecordDelete(context, input), companyRecordDeleteTarget),
	company_equity_add: recordTool(companyEquityAddInputSchema, (context, input) => companyEquityAdd(context, input)),
	company_equity_list: recordTool(companyEquityListInputSchema, (context, input) => companyEquityList(context, input)),
	company_equity_delete: recordTool(companyEquityDeleteInputSchema, (context, input) => companyEquityDelete(context, input)),
	company_document_register: recordTool(companyDocumentRegisterInputSchema, (context, input) => companyDocumentRegister(context, input)),
	company_document_list: recordTool(companyDocumentListInputSchema, (context, input) => companyDocumentList(context, input)),
	company_document_search: recordTool(companyDocumentSearchInputSchema, (context, input) => companyDocumentSearch(context, input)),
	company_document_recall: recordTool(recallSourceInputSchema, (context, input) => companyDocumentRecall(context, input)),
	company_document_update: recordTool(companyDocumentUpdateInputSchema, (context, input) => companyDocumentUpdate(context, input)),
	company_document_upload: recordTool(companyDocumentUploadInputSchema, (context, input) => companyDocumentUpload(context, input)),
	company_document_download: recordTool(companyDocumentDownloadInputSchema, (context, input) => companyDocumentDownload(context, input)),
	circle_list: recordTool(circleListInputSchema, (context) => companyCircleList(context)),
	circle_update: recordTool(circleSetInputSchema, (context, input) => companyCircleSet(context, input)),
	circle_member_update: recordTool(circleMemberSetInputSchema, (context, input) => companyCircleMemberSet(context, input)),
	circle_channel_add: recordTool(circleChannelAddInputSchema, (context, input) => companyCircleChannelAdd(context, input)),
	circle_channel_delete: recordTool(circleChannelDeleteInputSchema, (context, input) => companyCircleChannelDelete(context, input)),
	dataroom_get: recordTool(dataRoomGetInputSchema, (context) => companyDataRoomGet(context)),
	dataroom_links_get: recordTool(dataRoomLinksGetInputSchema, (context) => companyDataRoomLinksGet(context)),
	dataroom_link_add: recordTool(dataRoomLinkInputSchema, (context, input) => companyDataRoomLinkCreate(context, input)),
	dataroom_link_delete: recordTool(dataRoomLinkRevokeSchema, (context, input) => companyDataRoomLinkRevoke(context, input)),
	dataroom_category_update: recordTool(dataRoomCategorySetInputSchema, (context, input) => companyDataRoomCategorySet(context, input)),
	dataroom_field_update: recordTool(dataRoomFieldSetInputSchema, (context, input) => companyDataRoomFieldSet(context, input)),
	dataroom_field_name_update: recordTool(dataRoomFieldNameSetInputSchema, (context, input) => companyDataRoomFieldNameSet(context, input)),
	dataroom_share_add: recordTool(dataRoomShareCreateInputSchema, (context, input) => companyDataRoomShareCreate(context, input)),
	dataroom_share_delete: recordTool(dataRoomShareRevokeInputSchema, (context, input) => companyDataRoomShareRevoke(context, input)),
	dataroom_invitation_send: recordTool(dataRoomInvitationSendInputSchema, (context, input, { place }) =>
		companyDataRoomInvitationSend(context, input, placeTheAppAnswersAt('dataroom_invitation_send', place))),
	dataroom_invitation_accept: recordTool(dataRoomInvitationAcceptInputSchema, (context, input) => companyDataRoomInvitationAccept(context, input)),
	notification_settings_get: recordTool(notificationSettingsGetInputSchema, (context) => notificationSettingsGet(context)),
	notification_settings_set: recordTool(notificationSettingsSetInputIntentSchema, (context, input) => notificationSettingsSet(context, input)),
	conversation_mute: recordTool(conversationMuteInputSchema, (context, input) => conversationMute(context, input)),
	conversation_unmute: recordTool(conversationMuteInputSchema, (context, input) => conversationUnmute(context, input)),
	channel_order_get: recordTool(channelOrderGetInputSchema, (context) => channelOrderGet(context)),
	channel_order_set: recordTool(channelOrderSetInputSchema, (context, input) => channelOrderSet(context, input)),
	channel_pin: recordTool(channelPinInputSchema, (context, input) => channelPin(context, input)),
	channel_unpin: recordTool(channelPinInputSchema, (context, input) => channelUnpin(context, input)),
	crm_organization_list: recordTool(crmOrganizationListInputSchema, (context, input) => crmOrganizationList(context, input)),
	crm_organization_add: recordTool(crmOrganizationAddInputSchema, (context, input) => crmOrganizationAdd(context, input)),
	crm_organization_update: recordTool(crmOrganizationUpdateInputSchema, (context, input) => crmOrganizationUpdate(context, input)),
	crm_organization_archive: recordTool(crmOrganizationArchiveInputSchema, (context, input) => crmOrganizationArchive(context, input), crmOrganizationArchiveTarget),
	crm_contact_list: recordTool(crmContactListInputSchema, (context, input) => crmContactList(context, input)),
	crm_contact_add: recordTool(crmContactAddInputSchema, (context, input) => crmContactAdd(context, input)),
	crm_contact_update: recordTool(crmContactUpdateInputSchema, (context, input) => crmContactUpdate(context, input)),
	crm_contact_archive: recordTool(crmContactArchiveInputSchema, (context, input) => crmContactArchive(context, input), crmContactArchiveTarget),
	crm_opportunity_list: recordTool(crmOpportunityListInputSchema, (context, input) => crmOpportunityList(context, input)),
	crm_opportunity_add: recordTool(crmOpportunityAddInputSchema, (context, input) => crmOpportunityAdd(context, input)),
	crm_opportunity_update: recordTool(crmOpportunityUpdateInputSchema, (context, input) => crmOpportunityUpdate(context, input)),
	crm_opportunity_move: recordTool(crmOpportunityMoveInputSchema, (context, input, { place }) =>
		crmOpportunityMove(context, input, exchangeRatesOf(place?.environment ?? {}))),
	crm_opportunity_archive: recordTool(crmOpportunityArchiveInputSchema, (context, input) => crmOpportunityArchive(context, input), crmOpportunityArchiveTarget),
	crm_vocabulary_get: recordTool(crmVocabularyGetInputSchema, (context) => crmVocabularyGet(context)),
	crm_vocabulary_set: recordTool(crmVocabularySetInputSchema, (context, input) => crmVocabularySet(context, input)),
	crm_record_import: recordTool(
		crmRecordImportInputSchema,
		async (context, input, { files }) => crmRecordImport(context, await crmImportRecordsOf(input, files)),
		(context, input, files) => crmRecordImportPreview(context, input, files)
	),
	crm_activity_list: recordTool(crmActivityListInputSchema, (context, input) => crmActivityList(context, input)),
	crm_activity_save: recordTool(crmActivitySaveInputSchema, (context, input) => crmActivitySave(context, input))
};

export function recordRunsTheTool(name: string): boolean {
	return answererOfTool(name) === 'record';
}

export function recordToolsWithoutAnImplementation(): string[] {
	return toolsTheRecordAnswers().filter((name) => !Object.hasOwn(toolsOverTheRecord, name));
}

export function toolsTheRecordRuns(): string[] {
	return Object.keys(toolsOverTheRecord);
}

export function inputSchemaTheRecordReads(name: string): z.ZodType | undefined {
	return toolsOverTheRecord[name]?.inputSchema;
}

export function toolsTheRecordAnswers(): string[] {
	return toolNamesAnsweredBy('record');
}

export type ToolAnswer = { status: number; body: unknown };

export async function previewToolOverTheRecord(
	caller: SupabaseClient,
	accountDirectory: SupabaseClient,
	requesterID: string,
	name: string,
	input: unknown,
	now: Date,
	files: CarriedWorkspaceFile[] = []
): Promise<ToolAnswer> {
	const tool = toolsOverTheRecord[name];
	if (!tool) return { status: 200, body: { tool: name, target: null } };

	try {
		const { preview } = tool.boundTo(inputOfTheCall(name, input));
		if (!preview) return { status: 200, body: { tool: name, target: null } };
		const context = await recordContextOf(caller, accountDirectory, requesterID, now, leavesTaskLabelsUndecided);
		return { status: 200, body: { tool: name, target: await preview(context, files) } };
	} catch (refusal) {
		return refusalAnswer(name, refusal);
	}
}

export async function runToolOverTheRecord(
	caller: SupabaseClient,
	accountDirectory: SupabaseClient,
	requesterID: string,
	name: string,
	input: unknown,
	now: Date,
	decideTaskLabels: TaskLabelDecider,
	files: CarriedWorkspaceFile[] = [],
	place?: ToolPlace,
	announceTaskChange?: TaskChangeAnnouncer,
	announceAttendance?: AttendanceAnnouncer
): Promise<ToolAnswer> {
	const tool = toolsOverTheRecord[name];
	if (!tool) return { status: 404, body: { error: `no tool here goes by ${name}` } };

	try {
		const asked = inputOfTheCall(name, input);
		const { run } = tool.boundTo(asked);
		const context = () => recordContextOf(caller, accountDirectory, requesterID, now, decideTaskLabels,
					!readsWithoutPeople.has(name) && !((name === 'attendance_list' || name === 'attendance_changes_page_get') && asked.handWrittenOnly === true && asked.pageOffset !== undefined && asked.scope === 'all' && !asked.personHints) && !((name === 'attendance_update' || name === 'attendance_delete') && asked.undoOnly === true), exactAttendancePersonIDs(name, asked));
		const result = await run({ caller, context }, { files, place, announceTaskChange, announceAttendance });
		noteWhereTheAnswerLeftItsContract(name, result);
		return { status: 200, body: { tool: name, result } };
	} catch (refusal) {
		return refusalAnswer(name, refusal);
	}
}

// The contract these tools answer under is written here, and a device reads it
// through a copy that arrives only with an OTA release, so this is the only
// side that can be fixed when an answer and the contract disagree. A device
// that refused the disagreement would report it to a machine that can do
// nothing but refuse, and would take every record tool on the fleet down until
// the next release; capabilityd therefore reads the answer by name and leaves
// the judgement here (#1486).
//
// Naming it is the whole job. The answer still goes out: the record has
// already been written by the time this runs, and losing a completed write to
// a fault no caller can fix trades one defect for a worse one. The suites hold
// the same answers to the same schema through heldToTheContract, so a shape a
// test covers fails before it ships and a shape only production reaches is
// named in the log with the field that broke.
function noteWhereTheAnswerLeftItsContract(name: string, result: unknown): void {
	const schema = capabilityToolResultSchema(name);
	if (!schema) return;
	const parsed = schema.safeParse(readWithNullAsAbsent(schema, result));
	if (parsed.success) return;
	console.error(
		`tool.answer_left_its_contract: tool=${name} ${sentencesOfSchemaRefusal(parsed.error.issues, result, 'result')}`
	);
}

function refusalAnswer(name: string, refusal: unknown): ToolAnswer {
	if (refusal instanceof Refusal) return answerOfRefusal(refusal);
	console.error('tool.failed', { tool: name, failure: refusal });
	const reason = refusal instanceof Error ? refusal.message : String(refusal);
	return { status: 500, body: { error: `${name} failed: ${reason}` } };
}
