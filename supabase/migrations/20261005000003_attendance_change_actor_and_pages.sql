-- Legacy actors remain unobserved. Presentation may explicitly default those
-- records to their subject; future changes capture the authenticated actor.
alter table public.attendance
 add column changed_by uuid references public.member(id) on delete set null,
 add column changed_at timestamptz,
 add column change_actor_recorded boolean not null default false,
 add column change_previous_at timestamptz,
 add column change_previous_location text,
 add column change_previous_recorded boolean not null default false;

create function public.record_attendance_change_actor() returns trigger
language plpgsql security invoker set search_path = public as $$
begin
 if tg_op = 'INSERT' then
  new.changed_by := null;
  new.changed_at := null;
  new.change_actor_recorded := false;
  new.change_previous_at := null;
  new.change_previous_location := null;
  new.change_previous_recorded := false;
  if new.edit_reason is not null or new.original_occurred_at is not null then
   new.changed_by := public.my_member();
   new.changed_at := now();
   new.change_actor_recorded := true;
  end if;
 elsif new.occurred_at is distinct from old.occurred_at
    or new.location is distinct from old.location
    or new.edit_reason is distinct from old.edit_reason then
  new.changed_by := public.my_member();
  new.changed_at := now();
  new.change_actor_recorded := true;
  new.change_previous_at := old.occurred_at;
  new.change_previous_location := old.location;
  new.change_previous_recorded := true;
 else
  new.changed_by := old.changed_by;
  new.changed_at := old.changed_at;
  new.change_actor_recorded := old.change_actor_recorded;
  new.change_previous_at := old.change_previous_at;
  new.change_previous_location := old.change_previous_location;
  new.change_previous_recorded := old.change_previous_recorded;
 end if;
 return new;
end $$;
create trigger zz_record_attendance_change_actor before insert or update
 on public.attendance for each row execute function public.record_attendance_change_actor();
create index attendance_changes_recent on public.attendance
 (coalesce(changed_at, occurred_at) desc, id desc)
 where deleted_at is null and (edit_reason is not null or original_occurred_at is not null);

create function public.attendance_changes_page(
 from_timestamp timestamptz, to_timestamp timestamptz,
 page_offset integer default 0, page_limit integer default 24,
 team_search text default '', actor_search text default '',
 selected_members uuid[] default null, selected_team_key text default null, selected_actor uuid default null
) returns jsonb language plpgsql security invoker set search_path = public as $$
declare own_company uuid; answer jsonb;
begin
 select company_id into own_company from public.member where id = public.my_member();
 if own_company is null or not public.is_company_admin() then
  raise exception 'Only a company administrator can read attendance changes' using errcode = 'insufficient_privilege';
 end if;
 if page_offset is null or page_limit is null or page_offset < 0 or page_limit < 1 or page_limit > 100 or from_timestamp is null or to_timestamp is null or from_timestamp > to_timestamp then
  raise exception 'Invalid attendance changes page' using errcode = 'invalid_parameter_value';
 end if;
 with filtered as materialized (
  select event.*, subject.name as subject_name, subject.email as subject_email,
    coalesce(team.name, company.name) as team_name,
    coalesce(actor.name, actor.email) as actor_name, actor.email as actor_email,
    case when event.change_actor_recorded then 'observed' else 'legacy_subject' end as actor_source,
    company.timezone as zone
  from public.attendance event
  join public.member subject on subject.id = event.member_id
  join public.company company on company.id = subject.company_id
  left join public.team team on team.id = subject.team_id and team.company_id = own_company
  left join public.member actor on actor.id = case when event.change_actor_recorded then event.changed_by else event.member_id end and actor.company_id = own_company
  where subject.company_id = own_company and event.deleted_at is null
   and (event.edit_reason is not null or event.original_occurred_at is not null)
   and event.occurred_at between from_timestamp and to_timestamp
   and (selected_members is null or event.member_id = any(selected_members))
   and (selected_team_key is null or (selected_team_key='unassigned' and subject.team_id is null) or subject.team_id::text=selected_team_key)
   and (selected_actor is null or (case when event.change_actor_recorded then event.changed_by else event.member_id end)=selected_actor)
   and (team_search = '' or position(lower(team_search) in lower(coalesce(team.name, company.name))) > 0)
   and (actor_search = '' or position(lower(actor_search) in lower(coalesce(actor.name,'') || ' ' || coalesce(actor.email,''))) > 0)
 ), page as (
  select * from filtered order by coalesce(changed_at, occurred_at) desc, id desc
  offset page_offset limit page_limit
 )
 select jsonb_build_object('totalCount',(select count(*) from filtered),
  'pageOffset',page_offset,'pageLimit',page_limit,
  'attendance',coalesce((select jsonb_agg(jsonb_build_object(
   'eventID',id,'personID',member_id,'person',coalesce(subject_name,subject_email,''),'personEmail',subject_email,
   'kind',kind,'date',to_char(occurred_at at time zone zone,'YYYY-MM-DD'),
   'time',to_char(occurred_at at time zone zone,'HH24:MI'),'occurredAt',occurred_at,
   'location',location,'wasCorrected',change_previous_recorded or original_occurred_at is not null,
   'originalDate',to_char(coalesce(change_previous_at,original_occurred_at) at time zone zone,'YYYY-MM-DD'),
   'originalTime',to_char(coalesce(change_previous_at,original_occurred_at) at time zone zone,'HH24:MI'),
   'originalOccurredAt',coalesce(change_previous_at,original_occurred_at),'originalLocation',change_previous_location,'previousRecorded',change_previous_recorded,'reason',edit_reason,
   'teamName',team_name,'changedByID',case when change_actor_recorded then changed_by else member_id end,'changedByName',actor_name,'changedByEmail',actor_email,
   'changedBySource',actor_source,'changedAt',changed_at
  ) order by coalesce(changed_at, occurred_at) desc,id desc) from page),'[]'::jsonb)) into answer;
 return answer;
end $$;
revoke all on function public.attendance_changes_page(timestamptz,timestamptz,integer,integer,text,text,uuid[],text,uuid) from public;
grant execute on function public.attendance_changes_page(timestamptz,timestamptz,integer,integer,text,text,uuid[],text,uuid) to authenticated;

-- Revert the last observed revision atomically. An old record without a captured
-- previous time cannot distinguish a manual addition from a location-only edit.
-- This function cannot turn that ambiguity into a deletion.
create function public.attendance_change_undo(event_id uuid)
returns jsonb language plpgsql security definer set search_path = public as $$
declare held public.attendance; zone text; previous_at timestamptz;
begin
 if public.my_member() is null or not public.is_company_admin() then
  raise exception 'Only a company administrator can undo attendance changes' using errcode = 'insufficient_privilege';
 end if;
 perform event.id from public.attendance event
 where event.member_id in (
  select target.member_id from public.attendance target
  join public.member subject on subject.id = target.member_id
  where target.id = event_id and target.deleted_at is null
   and subject.company_id = internal.company_of_member(public.my_member())
 )
 order by event.member_id, event.id
 for update;
 select event.* into held from public.attendance event
 join public.member subject on subject.id=event.member_id
 where event.id=event_id and event.deleted_at is null
 and subject.company_id=internal.company_of_member(public.my_member())
 for update of event;
 if not found or (held.edit_reason is null and held.original_occurred_at is null) then
  raise exception 'Attendance change not found' using errcode = 'insufficient_privilege';
 end if;
 if not held.change_previous_recorded and held.original_occurred_at is null and not held.change_actor_recorded then
  raise exception 'Previous attendance record is unknown; cannot infer a deletion' using errcode = 'check_violation';
 end if;
 previous_at := coalesce(held.change_previous_at,held.original_occurred_at);
 if previous_at is null then
  return public.attendance_remove(held.id,'추가 기록 되돌리기');
 end if;
 select company.timezone into zone from public.company company
 join public.member subject on subject.company_id=company.id where subject.id=held.member_id;
 return public.attendance_correct(jsonb_build_array(jsonb_build_object(
  'event_id',held.id,
  'local_date',to_char(previous_at at time zone zone,'YYYY-MM-DD'),
  'local_time',to_char(previous_at at time zone zone,'HH24:MI:SS.US'),
  'location',case when held.change_previous_recorded then held.change_previous_location else held.location end
 )),'이전 기록 되돌리기');
end $$;
revoke all on function public.attendance_change_undo(uuid) from public, anon, service_role;
grant execute on function public.attendance_change_undo(uuid) to authenticated;
