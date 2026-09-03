begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

select is(
  (
    select array_agg(pg_enum.enumlabel::text order by pg_enum.enumlabel)
    from pg_enum
    join pg_type on pg_type.oid = pg_enum.enumtypid
    where pg_type.typname = 'crm_stage'
      and pg_type.typnamespace = 'public'::regnamespace
  ),
  array['done', 'in_progress', 'lost', 'on_hold', 'review', 'waiting'],
  'stage: the crm stage enum names exactly these six, which a web test holds to the shared source'
);

select is(
  (
    select pg_type.typname::text
    from pg_attribute
    join pg_type on pg_type.oid = pg_attribute.atttypid
    where pg_attribute.attrelid = 'public.opportunity'::regclass
      and pg_attribute.attname = 'stage_id'
  ),
  'crm_stage',
  'stage: an opportunity holds its stage as the enum rather than as free text'
);

select is(
  (
    select count(*)
    from pg_constraint
    where conrelid = 'public.opportunity'::regclass
      and conname = 'opportunity_stage_id_is_valid'
  ),
  0::bigint,
  'stage: the fixed-set check constraint is gone, the type says it now'
);

insert into auth.users (id, email) values
  ('59700000-0000-0000-0000-000000000001', 'stage-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone, currency_code, crm_vocabulary) values
  (
    '59700000-0000-0000-0000-0000000000a0',
    'Stage Company',
    'stage-company',
    'KR',
    'ko',
    'Asia/Seoul',
    'KRW',
    '{"organization_types":[],"pipelines":[{"id":"partnership","name":"파트너십"}]}'
  );

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('59700000-0000-0000-0000-0000000000a1', '59700000-0000-0000-0000-0000000000a0', 'stage-admin@example.test', '59700000-0000-0000-0000-000000000001', 'active', true);

insert into public.organization (id, company_id, name) values
  ('59700000-0000-0000-0000-000000000101', '59700000-0000-0000-0000-0000000000a0', 'Stage Organization');

insert into public.opportunity (id, company_id, organization_id, name, pipeline_id, stage_id, amount_minor, currency_code) values
  ('59700000-0000-0000-0000-000000000201', '59700000-0000-0000-0000-0000000000a0', '59700000-0000-0000-0000-000000000101', 'Stage deal', 'partnership', 'review', 18000000, 'KRW');

set constraints all immediate;

select throws_ok(
  $$
    insert into public.opportunity (company_id, organization_id, name, pipeline_id, stage_id) values (
      '59700000-0000-0000-0000-0000000000a0',
      '59700000-0000-0000-0000-000000000101',
      'Invented stage deal',
      'partnership',
      'negotiating'
    )
  $$,
  '22P02',
  null,
  'stage: a word the enum does not name is refused by the type'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"59700000-0000-0000-0000-000000000001","role":"authenticated"}', true);
  perform public.crm_opportunity_close('59700000-0000-0000-0000-000000000201', 'done', 1024, now(), null, null, null);
  assert (
    select stage_id = 'done' and base_amount_minor = 18000000
    from public.opportunity
    where id = '59700000-0000-0000-0000-000000000201'
  ), 'the close settles the amount it won';
  reset role;
end $$;$block$, 'stage: closing still moves the opportunity now that the stage is a type');

select * from finish();
rollback;
