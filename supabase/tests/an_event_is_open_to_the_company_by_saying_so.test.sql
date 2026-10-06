begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

insert into auth.users (id, email) values
  ('49000000-0000-0000-0000-000000000001', 'host@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('49000000-0000-0000-0000-0000000000a0', 'Open Scope', 'open-scope', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  (
    '49000000-0000-0000-0000-0000000000a1',
    '49000000-0000-0000-0000-0000000000a0',
    'host@example.test',
    '49000000-0000-0000-0000-000000000001',
    'active',
    true
  ),
  (
    '49000000-0000-0000-0000-0000000000a2',
    '49000000-0000-0000-0000-0000000000a0',
    'leaver@example.test',
    null,
    'active',
    false
  );

create function pg_temp.save_event(
  event_title text,
  participants uuid[],
  open_to_company boolean default null,
  event_id uuid default null
) returns uuid
language plpgsql
as $$
declare
  saved uuid;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"49000000-0000-0000-0000-000000000001"}', true);
  saved := public.task_save(
    target_task_id => event_id,
    target_title => event_title,
    target_starts_at => '2026-10-20T02:00:00Z',
    target_ends_at => '2026-10-20T03:00:00Z',
    target_size => 'M',
    target_participant_ids => participants,
    target_is_event => true,
    target_is_open_to_company => open_to_company
  );
  reset role;
  return saved;
end $$;

select pg_temp.save_event('Town hall', '{}', true);

select is(
  (select is_open_to_company from public.task where title = 'Town hall'),
  true,
  'an event saved as open to the company is open'
);

select pg_temp.save_event('Town hall', '{}', null, (select id from public.task where title = 'Town hall'));

select is(
  (select is_open_to_company from public.task where title = 'Town hall'),
  true,
  'a write that says nothing about scope leaves an open event open'
);

select pg_temp.save_event('Dentist', array['49000000-0000-0000-0000-0000000000a2'::uuid]);

select is(
  (select is_open_to_company from public.task where title = 'Dentist'),
  false,
  'an event with named people is personal'
);

delete from public.member where id = '49000000-0000-0000-0000-0000000000a2';

select results_eq(
  $$select is_open_to_company, (select count(*) from public.task_participant where task_id = task.id)
    from public.task where title = 'Dentist'$$,
  $$values (false, 0::bigint)$$,
  'deleting the only participant leaves the event personal, not open to the company'
);

select throws_ok(
  $$select pg_temp.save_event('Mixed', array['49000000-0000-0000-0000-0000000000a1'::uuid], true)$$,
  '22023',
  'an event open to the whole company names no participants',
  'an event cannot be open to everyone and name people'
);

select throws_ok(
  $$insert into public.task (company_id, title, is_open_to_company)
    values ('49000000-0000-0000-0000-0000000000a0', 'Not an event', true)$$,
  '23514',
  null,
  'only an event can be open to the company'
);

select lives_ok(
  $$select pg_temp.save_event('Dentist', '{}', true)$$,
  'an open event is not a second copy of a personal one nobody attends'
);

select results_eq(
  $$select is_open_to_company from public.task where title = 'Dentist' order by is_open_to_company$$,
  $$values (false), (true)$$,
  'both stay, one personal and one open'
);

select * from finish();
rollback;
