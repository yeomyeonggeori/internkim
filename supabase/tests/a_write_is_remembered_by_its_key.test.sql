begin;
create extension if not exists pgtap with schema extensions;
select plan(10);

delete from public.company;

insert into auth.users (id, email) values
  ('1d000000-0000-0000-0000-000000000011', 'first@example.test'),
  ('1d000000-0000-0000-0000-000000000012', 'second@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('1d000000-0000-0000-0000-0000000000c1', 'Company', 'company', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  ('1d000000-0000-0000-0000-0000000000a1', '1d000000-0000-0000-0000-0000000000c1',
   'first@example.test', '1d000000-0000-0000-0000-000000000011', 'active'),
  ('1d000000-0000-0000-0000-0000000000a2', '1d000000-0000-0000-0000-0000000000c1',
   'second@example.test', '1d000000-0000-0000-0000-000000000012', 'active');

select ok(has_table_privilege('authenticated', 'public.idempotency_key', 'select'),
  'a member reads the writes remembered for them');
select ok(has_table_privilege('authenticated', 'public.idempotency_key', 'insert'),
  'a member remembers a write');
select ok(not has_table_privilege('authenticated', 'public.idempotency_key', 'update'),
  'a remembered answer is written once');
select ok(not has_table_privilege('authenticated', 'public.idempotency_key', 'delete'),
  'a remembered answer is not forgotten by its member');
select ok(has_table_privilege('service_role', 'public.idempotency_key', 'select'),
  'the service role reads every remembered write');

do $$
declare
  planting_blocked boolean := false;
  repeating_blocked boolean := false;
  remembered integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"1d000000-0000-0000-0000-000000000011"}', true);

  insert into public.idempotency_key (member_id, key, tool_name, response)
  values ('1d000000-0000-0000-0000-0000000000a1', 'first-key', 'task_add', '{"status":200,"body":{"tool":"task_add"}}');

  begin
    insert into public.idempotency_key (member_id, key, tool_name, response)
    values ('1d000000-0000-0000-0000-0000000000a2', 'planted-key', 'task_add', '{}');
  exception when insufficient_privilege then
    planting_blocked := true;
  end;
  assert planting_blocked, 'a member does not remember a write as somebody else';

  begin
    insert into public.idempotency_key (member_id, key, tool_name, response)
    values ('1d000000-0000-0000-0000-0000000000a1', 'first-key', 'task_add', '{}');
  exception when unique_violation then
    repeating_blocked := true;
  end;
  assert repeating_blocked, 'the same key from the same member is one write';

  insert into public.idempotency_key (member_id, key, tool_name, response)
  values ('1d000000-0000-0000-0000-0000000000a1', 'second-key', 'task_add', '{"status":200,"body":{}}');

  select count(*) into remembered from public.idempotency_key;
  assert remembered = 2, 'a member reads back the writes they remembered';

  reset role;
end;
$$;

select pass('a member remembers a write under their own id and no other');
select pass('the same key from the same member is one write');
select pass('a different key is another write');

do $$
declare
  remembered integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"1d000000-0000-0000-0000-000000000012"}', true);

  select count(*) into remembered from public.idempotency_key;
  assert remembered = 0, 'a colleague does not see the writes remembered for somebody else';

  reset role;
end;
$$;

select pass('a member reads only the writes remembered for them');

do $$
declare
  remembered integer;
begin
  set local role service_role;

  select count(*) into remembered from public.idempotency_key;
  assert remembered = 2, 'the service role reads every remembered write';

  reset role;
end;
$$;

select pass('the service role reads every remembered write');

select finish();
rollback;
