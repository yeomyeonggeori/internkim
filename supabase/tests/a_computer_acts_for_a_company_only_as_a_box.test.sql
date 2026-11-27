begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7a000000-0000-0000-0000-0000000000c1', 'Only Boxes', 'only-boxes', 'KR', 'ko', 'Asia/Seoul');

insert into public.agent (company_id, name, api_key_hash) values
  ('7a000000-0000-0000-0000-0000000000c1', 'company-computer', repeat('a', 64));

select public.keep_claimed_box('7a000000-0000-0000-0000-0000000000c1', 'the-box', repeat('b', 43), null, false);

create function pg_temp.what_the_session_holds(claims jsonb)
returns table (company uuid, box_role text)
language plpgsql
as $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', claims::text, true);
  return query select public.my_app_company(), public.my_app_box_role();
  reset role;
end;
$$;

select is(
  (select company from pg_temp.what_the_session_holds('{"sub":"7a000000-0000-0000-0000-0000000000e1","app_metadata":{"company_id":"7a000000-0000-0000-0000-0000000000c1","agent_key_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}')),
  null,
  'a session carrying only the hash of an agent key that stands acts for no company'
);

select is(
  (select box_role from pg_temp.what_the_session_holds('{"sub":"7a000000-0000-0000-0000-0000000000e1","app_metadata":{"company_id":"7a000000-0000-0000-0000-0000000000c1","agent_key_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}')),
  null,
  'a session carrying only the hash of an agent key that stands is no service computer'
);

select is(
  (select company from pg_temp.what_the_session_holds('{"sub":"7a000000-0000-0000-0000-0000000000e1","iat":0,"app_metadata":{"company_id":"7a000000-0000-0000-0000-0000000000c1"}}')),
  null,
  'a session naming no box acts for no company, however early it was issued'
);

select is(
  (select box_role from pg_temp.what_the_session_holds('{"sub":"7a000000-0000-0000-0000-0000000000e1","iat":0,"app_metadata":{"company_id":"7a000000-0000-0000-0000-0000000000c1"}}')),
  null,
  'a session naming no box is no service computer, however early it was issued'
);

select is(
  (select company from pg_temp.what_the_session_holds('{"sub":"7a000000-0000-0000-0000-0000000000e1","app_metadata":{"company_id":"7a000000-0000-0000-0000-0000000000c1","fleet_id":"the-box"}}')),
  '7a000000-0000-0000-0000-0000000000c1'::uuid,
  'a session naming one of the company''s boxes acts for the company'
);

select is(
  (select box_role from pg_temp.what_the_session_holds('{"sub":"7a000000-0000-0000-0000-0000000000e1","app_metadata":{"company_id":"7a000000-0000-0000-0000-0000000000c1","fleet_id":"the-box"}}')),
  'service',
  'a session naming the company''s service box is its service computer'
);

select * from finish();
rollback;
