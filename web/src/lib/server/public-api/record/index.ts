import type { SupabaseClient } from '@supabase/supabase-js';
import { attendanceAdd, attendanceDelete, attendanceList, attendanceUpdate } from './attendance-tools';
import { NoSuchAttendanceRecord } from './attendance';
import { recordContextOf, type RecordContext } from './company';
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
import {
	companyDocumentList,
	companyDocumentRegister,
	companyDocumentSearch,
	companyDocumentUpdate,
	companyMetricList,
	companyMetricRecord,
	companyRecordAdd,
	companyRecordDelete,
	companyRecordList,
	companyRecordUpdate
} from './company-ledger-tools';
import {
	CalendarEventDuplicate,
	CalendarEventVersionConflict,
	eventAdd,
	eventDelete,
	eventList,
	eventUpdate
} from './event-tools';
import { HintRefused } from './hint-resolution';
import {
	conversationMute,
	conversationUnmute,
	notificationSettingsGet,
	notificationSettingsSet,
	pushDeviceClaim,
	pushDeviceRelease,
	pushReachabilityGet
} from './notification-tools';
import { LabelUnresolved } from './labels';
import { RecordRefusedTheWrite, WriteNotReadBack } from './tasks';
import { NoSuchLeave, NoSuchLeaveKind } from './leave';
import { leaveBalance, leaveDecide, leaveDelete, leaveList, leaveRequest, leaveUpdate } from './leave-tools';
import { personInvite, personList, personUpdate } from './people-tools';
import { taskAdd, taskDelete, taskList, taskUpdate } from './task-tools';
import { teamAdd, teamDelete, teamList, teamUpdate } from './team-tools';
import { previewOfTool } from './preview';
import { answererOfTool, toolNamesAnsweredBy } from '../catalog';

type ToolInput = Record<string, unknown>;
type ToolRun = (context: RecordContext, input: ToolInput) => Promise<unknown> | unknown;

const toolsOverTheRecord: Record<string, ToolRun> = {
	task_add: (context, input) => taskAdd(context, input),
	task_update: (context, input) => taskUpdate(context, input),
	task_list: (context, input) => taskList(context, input),
	task_delete: (context, input) => taskDelete(context, input),
	event_add: (context, input) => eventAdd(context, input),
	event_update: (context, input) => eventUpdate(context, input),
	event_list: (context, input) => eventList(context, input),
	event_delete: (context, input) => eventDelete(context, input),
	person_list: (context) => personList(context),
	person_update: (context, input) => personUpdate(context, input),
	person_invite: (context, input) => personInvite(context, input),
	team_list: (context) => teamList(context),
	team_add: (context, input) => teamAdd(context, input),
	team_update: (context, input) => teamUpdate(context, input),
	team_delete: (context, input) => teamDelete(context, input),
	leave_list: (context, input) => leaveList(context, input),
	leave_balance: (context, input) => leaveBalance(context, input),
	leave_request: (context, input) => leaveRequest(context, input),
	leave_update: (context, input) => leaveUpdate(context, input),
	leave_delete: (context, input) => leaveDelete(context, input),
	leave_decide: (context, input) => leaveDecide(context, input),
	attendance_list: (context, input) => attendanceList(context, input),
	attendance_add: (context, input) => attendanceAdd(context, input),
	attendance_update: (context, input) => attendanceUpdate(context, input),
	attendance_delete: (context, input) => attendanceDelete(context, input),
	attendance_work_policy_get: (context) => attendanceWorkPolicyGet(context),
	attendance_work_policy_set: (context, input) => attendanceWorkPolicySet(context, input),
	attendance_leave_policy_get: (context) => attendanceLeavePolicyGet(context),
	attendance_leave_policy_set: (context, input) => attendanceLeavePolicySet(context, input),
	company_settings_get: (context) => companySettingsGet(context),
	company_settings_update: (context, input) => companySettingsUpdate(context, input),
	company_info_get: (context, input) => companyInfoGet(context, input),
	company_info_set: (context, input) => companyInfoSet(context, input),
	company_holiday_list: (context, input) => companyHolidayList(context, input),
	company_holiday_add: (context, input) => companyHolidayAdd(context, input),
	company_holiday_update: (context, input) => companyHolidayUpdate(context, input),
	company_holiday_delete: (context, input) => companyHolidayDelete(context, input),
	company_metric_record: (context, input) => companyMetricRecord(context, input),
	company_metric_list: (context, input) => companyMetricList(context, input),
	company_record_add: (context, input) => companyRecordAdd(context, input),
	company_record_list: (context, input) => companyRecordList(context, input),
	company_record_update: (context, input) => companyRecordUpdate(context, input),
	company_record_delete: (context, input) => companyRecordDelete(context, input),
	company_document_register: (context, input) => companyDocumentRegister(context, input),
	company_document_list: (context, input) => companyDocumentList(context, input),
	company_document_search: (context, input) => companyDocumentSearch(context, input),
	company_document_update: (context, input) => companyDocumentUpdate(context, input),
	notification_settings_get: (context) => notificationSettingsGet(context),
	notification_settings_set: (context, input) => notificationSettingsSet(context, input),
	conversation_mute: (context, input) => conversationMute(context, input),
	conversation_unmute: (context, input) => conversationUnmute(context, input),
	push_reachability_get: (context) => pushReachabilityGet(context),
	push_device_claim: (context, input) => pushDeviceClaim(context, input),
	push_device_release: (context, input) => pushDeviceRelease(context, input),
	crm_organization_list: (context, input) => crmOrganizationList(context, input),
	crm_organization_add: (context, input) => crmOrganizationAdd(context, input),
	crm_organization_update: (context, input) => crmOrganizationUpdate(context, input),
	crm_organization_archive: (context, input) => crmOrganizationArchive(context, input),
	crm_contact_list: (context, input) => crmContactList(context, input),
	crm_contact_add: (context, input) => crmContactAdd(context, input),
	crm_contact_update: (context, input) => crmContactUpdate(context, input),
	crm_contact_archive: (context, input) => crmContactArchive(context, input),
	crm_opportunity_list: (context, input) => crmOpportunityList(context, input),
	crm_opportunity_add: (context, input) => crmOpportunityAdd(context, input),
	crm_opportunity_update: (context, input) => crmOpportunityUpdate(context, input),
	crm_opportunity_move: (context, input) => crmOpportunityMove(context, input),
	crm_opportunity_archive: (context, input) => crmOpportunityArchive(context, input),
	crm_vocabulary_get: (context) => crmVocabularyGet(context),
	crm_vocabulary_set: (context, input) => crmVocabularySet(context, input),
	crm_activity_list: (context, input) => crmActivityList(context, input),
	crm_activity_save: (context, input) => crmActivitySave(context, input)
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

export function toolsTheRecordAnswers(): string[] {
	return toolNamesAnsweredBy('record');
}

export type ToolAnswer = { status: number; body: unknown };

export async function previewToolOverTheRecord(
	caller: SupabaseClient,
	accountDirectory: SupabaseClient,
	requesterID: string,
	name: string,
	input: ToolInput,
	now: Date
): Promise<ToolAnswer> {
	const preview = previewOfTool(name);
	if (!preview) return { status: 200, body: { tool: name, target: null } };

	try {
		const context = await recordContextOf(caller, accountDirectory, requesterID, now);
		return { status: 200, body: { tool: name, target: await preview(context, input) } };
	} catch (refusal) {
		return refusalAnswer(name, refusal);
	}
}

export async function runToolOverTheRecord(
	caller: SupabaseClient,
	accountDirectory: SupabaseClient,
	requesterID: string,
	name: string,
	input: ToolInput,
	now: Date
): Promise<ToolAnswer> {
	const run = toolsOverTheRecord[name];
	if (!run) return { status: 404, body: { error: `no tool here goes by ${name}` } };

	try {
		const context = await recordContextOf(caller, accountDirectory, requesterID, now);
		return { status: 200, body: { tool: name, result: await run(context, input) } };
	} catch (refusal) {
		return refusalAnswer(name, refusal);
	}
}

// A caller who named something the record could not place gets the candidates
// back, so the next call can name one exactly rather than guess again.
function refusalAnswer(name: string, refusal: unknown): ToolAnswer {
	if (refusal instanceof HintRefused) {
		return {
			status: 409,
			body: {
				error: refusal.message,
				errorCode: refusal.errorCode,
				failureStage: refusal.failureStage,
				retryable: refusal.retryable,
				safeRetry: refusal.safeRetry,
				hint: refusal.hint,
				candidates: refusal.candidates
			}
		};
	}
	if (refusal instanceof WriteNotReadBack) {
		return {
			status: 502,
			body: {
				error: refusal.message,
				errorCode: refusal.errorCode,
				failureStage: refusal.failureStage,
				retryable: refusal.retryable,
				safeRetry: refusal.safeRetry
			}
		};
	}
	if (refusal instanceof CalendarEventVersionConflict) {
		return {
			status: 409,
			body: {
				error: refusal.message,
				errorCode: refusal.errorCode,
				failureStage: refusal.failureStage,
				retryable: refusal.retryable,
				safeRetry: refusal.safeRetry,
				updatedAt: refusal.updatedAt
			}
		};
	}
	if (refusal instanceof CalendarEventDuplicate) {
		return {
			status: 409,
			body: {
				error: refusal.message,
				errorCode: refusal.errorCode,
				failureStage: refusal.failureStage,
				retryable: refusal.retryable,
				safeRetry: refusal.safeRetry,
				...(refusal.eventID ? { eventID: refusal.eventID } : {})
			}
		};
	}
	if (refusal instanceof LabelUnresolved) {
		return {
			status: 409,
			body: {
				error: refusal.message,
				errorCode: refusal.errorCode,
				failureStage: refusal.failureStage,
				retryable: refusal.retryable,
				safeRetry: refusal.safeRetry,
				registered: refusal.registered
			}
		};
	}
	if (refusal instanceof NoSuchLeave) {
		return { status: 409, body: { error: refusal.message, hint: refusal.hint, candidates: refusal.candidates } };
	}
	if (refusal instanceof NoSuchLeaveKind) {
		return { status: 409, body: { error: refusal.message, registered: refusal.registered } };
	}
	if (refusal instanceof NoSuchAttendanceRecord) {
		return { status: 409, body: { error: refusal.message, hint: refusal.hint, candidates: refusal.candidates } };
	}
	if (refusal instanceof RecordRefusedTheWrite) {
		return {
			status: refusal.status,
			body: {
				error: refusal.message,
				errorCode: refusal.errorCode,
				retryable: refusal.status === 404 || refusal.status === 409,
				safeRetry: refusal.status !== 502
			}
		};
	}
	if (refusal instanceof Error) return { status: 400, body: { error: refusal.message } };
	return { status: 500, body: { error: `${name} failed for a reason it did not name` } };
}
