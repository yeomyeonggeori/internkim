begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

insert into auth.users (id, email) values
  ('47000000-0000-0000-0000-000000000001', 'version-a@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('47000000-0000-0000-0000-0000000000a0', 'Calendar Version', 'calendar-version', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  (
    '47000000-0000-0000-0000-0000000000a1',
    '47000000-0000-0000-0000-0000000000a0',
    'version-a@example.test',
    '47000000-0000-0000-0000-000000000001',
    'active',
    true
  );

insert into public.task (id, company_id, title, is_event, starts_at, ends_at, size) values
  (
    '47000000-0000-0000-0000-000000000101',
    '47000000-0000-0000-0000-0000000000a0',
    'Portland trip',
    true,
    '2026-08-24T00:00:00Z',
    '2026-08-28T00:00:00Z',
    'M'
  );

select lives_ok($block$do $$
declare
  version timestamptz;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"47000000-0000-0000-0000-000000000001"}', true);
  select updated_at into version from public.task where id = '47000000-0000-0000-0000-000000000101';
  perform public.task_save(
    target_task_id => '47000000-0000-0000-0000-000000000101',
    target_title => 'Portland trip, confirmed',
    target_note => null,
    target_location => null,
    target_starts_at => '2026-08-24T00:00:00Z',
    target_ends_at => '2026-08-28T00:00:00Z',
    target_is_whole_day => true,
    target_size => 'M',
    target_participant_ids => array['47000000-0000-0000-0000-0000000000a1'::uuid],
    target_expected_updated_at => version,
    target_is_event => true
  );
  reset role;
end $$;$block$, 'calendar save: the version that was read is the version that is written');

select is(
  (select title from public.task where id = '47000000-0000-0000-0000-000000000101'),
  'Portland trip, confirmed',
  'calendar save: the write that named the current version landed'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"47000000-0000-0000-0000-000000000001"}', true);
  perform public.task_save(
    target_task_id => '47000000-0000-0000-0000-000000000101',
    target_title => 'Portland trip, overwritten',
    target_note => null,
    target_location => null,
    target_starts_at => '2026-08-24T00:00:00Z',
    target_ends_at => '2026-08-28T00:00:00Z',
    target_is_whole_day => true,
    target_size => 'M',
    target_participant_ids => array['47000000-0000-0000-0000-0000000000a1'::uuid],
    target_expected_updated_at => '2020-01-01T00:00:00Z',
    target_is_event => true
  );
  reset role;
end $$;$block$, '40001', 'task changed since it was read',
  'calendar save: a write that names a version that is gone is refused');

select * from finish();
rollback;
