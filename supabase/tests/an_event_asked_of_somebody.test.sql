begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

insert into auth.users (id, email) values
  ('48000000-0000-0000-0000-000000000001', 'asker@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('48000000-0000-0000-0000-0000000000a0', 'Asked Of', 'asked-of', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  (
    '48000000-0000-0000-0000-0000000000a1',
    '48000000-0000-0000-0000-0000000000a0',
    'asker@example.test',
    '48000000-0000-0000-0000-000000000001',
    'active',
    true
  ),
  (
    '48000000-0000-0000-0000-0000000000a2',
    '48000000-0000-0000-0000-0000000000a0',
    'asked@example.test',
    null,
    'active',
    false
  );

-- An event one person files for another stands at requested, and the company
-- reads who asked from that rather than from anything the caller sends.
select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"48000000-0000-0000-0000-000000000001"}', true);
  perform public.task_save(
    target_task_id => null,
    target_title => 'Mona developer meeting',
    target_note => null,
    target_location => null,
    target_starts_at => '2026-08-26T02:00:00Z',
    target_ends_at => '2026-08-26T03:00:00Z',
    target_is_whole_day => false,
    target_size => 'M',
    target_participant_ids => array['48000000-0000-0000-0000-0000000000a2'::uuid],
    target_expected_updated_at => null,
    target_notify_minutes_before => null,
    target_status => 'requested',
    target_is_event => true
  );
  reset role;
end $$;$block$, 'an event asked of a colleague is saved');

select is(
  (select requester_id from public.task where title = 'Mona developer meeting'),
  '48000000-0000-0000-0000-0000000000a1'::uuid,
  'the company stamps who asked from the status'
);

-- An event somebody files for themselves is nobody's request of anybody.
select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"48000000-0000-0000-0000-000000000001"}', true);
  perform public.task_save(
    target_task_id => null,
    target_title => 'Dentist',
    target_note => null,
    target_location => null,
    target_starts_at => '2026-08-27T02:00:00Z',
    target_ends_at => '2026-08-27T03:00:00Z',
    target_is_whole_day => false,
    target_size => 'M',
    target_participant_ids => array['48000000-0000-0000-0000-0000000000a1'::uuid],
    target_expected_updated_at => null,
    target_notify_minutes_before => 45,
    target_status => null,
    target_is_event => true
  );
  reset role;
end $$;$block$, 'an event somebody files for themselves is saved');

select results_eq(
  $$select requester_id, notify_minutes_before from public.task where title = 'Dentist'$$,
  $$values (null::uuid, 45)$$,
  'nobody asked, and the reminder the caller set is the reminder the company keeps'
);

select * from finish();
rollback;
