-- Fourteen names for one kind of thing. A caller reached for save_, set_,
-- close_, link_, correct_ or record_ depending on which day the function was
-- written, and the same call read differently on each side of the same row:
-- task_add from a tool, save_flow_task from the web.
--
-- The noun comes first and the verb follows it, which is what the tools already
-- do. A rename keeps the body, the grants and the oid, so nothing else moves.
alter function public.save_crm_task(uuid, text, public.task_status, text, text, text, timestamptz, timestamptz, timestamptz, boolean, boolean, integer, jsonb, uuid, uuid[], uuid, uuid, uuid) rename to crm_task_save;
alter function public.save_crm_vocabulary(jsonb) rename to crm_vocabulary_save;
alter function public.close_crm_opportunity(uuid, text, integer, timestamptz, text, bigint, text) rename to crm_opportunity_close;

alter function public.save_task_vocabulary(jsonb) rename to task_vocabulary_save;
alter function public.set_task_parent(uuid, uuid) rename to task_parent_set;
alter function public.link_task_children(uuid, uuid[]) rename to task_children_link;

alter function public.save_teams(jsonb) rename to team_save;
alter function public.save_member_profiles(jsonb) rename to member_profiles_save;
alter function public.save_own_member_profile(text, date) rename to member_profile_save_own;
alter function public.set_member_leave_days(uuid, numeric) rename to member_leave_days_set;
alter function public.set_my_notification_settings(jsonb) rename to notification_settings_set;

alter function public.save_attendance_calendar(uuid, jsonb) rename to attendance_calendar_save;
alter function public.save_attendance_reconciliation_settings(uuid, jsonb, jsonb) rename to attendance_reconciliation_save;
alter function public.save_attendance_work_policy(uuid, jsonb) rename to attendance_policy_save;
alter function public.correct_attendance_events(jsonb, text) rename to attendance_correct;

alter function public.claim_push_device(text, text, jsonb) rename to push_device_claim;
alter function public.release_push_device(text, text) rename to push_device_release;

-- A function body resolves the names it calls when it runs, so the two that
-- reach for their neighbours are rewritten to the names those neighbours now
-- answer to. task_save is rewritten where it is defined.
create or replace function public.attendance_reconciliation_save(target_company uuid, attendance_work_policy jsonb, attendance_calendar jsonb)
 RETURNS void
 LANGUAGE plpgsql
 SET search_path TO 'public'
AS $function$
begin
  if attendance_work_policy is not null then
    perform public.attendance_policy_save(target_company, attendance_work_policy);
  end if;
  if attendance_calendar is not null then
    perform public.attendance_calendar_save(target_company, attendance_calendar);
  end if;
end;
$function$
