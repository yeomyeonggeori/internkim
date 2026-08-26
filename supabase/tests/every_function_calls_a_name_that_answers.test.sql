begin;
create extension if not exists pgtap with schema extensions;
select plan(2);

-- A plpgsql body resolves the names it calls when it runs, so renaming a
-- function leaves every body that still spells the old name compiling fine and
-- failing in production. These are the names nothing answers to any more.
select is_empty(
  $$
  select p.proname || ' still calls ' || retired.name
  from pg_proc p
  join pg_namespace n on n.oid = p.pronamespace
  cross join (values
    ('save_flow_task'), ('save_calendar_event'), ('save_crm_task'), ('save_crm_vocabulary'),
    ('close_crm_opportunity'), ('save_task_vocabulary'), ('set_task_parent'), ('link_task_children'),
    ('save_teams'), ('save_member_profiles'), ('save_own_member_profile'), ('set_member_leave_days'),
    ('set_my_notification_settings'), ('save_attendance_calendar'),
    ('save_attendance_reconciliation_settings'), ('save_attendance_work_policy'),
    ('correct_attendance_events'), ('claim_push_device'), ('release_push_device')
  ) as retired(name)
  where n.nspname = 'public'
    and p.prosrc like '%public.' || retired.name || '(%'
  $$,
  'no function body calls a name that was retired'
);

-- And the names themselves are gone, so a caller that learned one fails loudly
-- rather than finding a leftover.
select is_empty(
  $$
  select proname
  from pg_proc p
  join pg_namespace n on n.oid = p.pronamespace
  where n.nspname = 'public'
    and proname in (
      'save_flow_task', 'save_calendar_event', 'save_crm_task', 'save_crm_vocabulary',
      'close_crm_opportunity', 'save_task_vocabulary', 'set_task_parent', 'link_task_children',
      'save_teams', 'save_member_profiles', 'save_own_member_profile', 'set_member_leave_days',
      'set_my_notification_settings', 'save_attendance_calendar',
      'save_attendance_reconciliation_settings', 'save_attendance_work_policy',
      'correct_attendance_events', 'claim_push_device', 'release_push_device'
    )
  $$,
  'the retired names are gone'
);

select * from finish();
rollback;
