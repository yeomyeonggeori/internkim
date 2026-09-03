begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

select is(
  (
    select array_agg(pg_enum.enumlabel::text order by pg_enum.enumlabel)
    from pg_enum
    join pg_type on pg_type.oid = pg_enum.enumtypid
    where pg_type.typname = 'member_status'
      and pg_type.typnamespace = 'public'::regnamespace
  ),
  array['active', 'departed', 'invited', 'pending', 'withdrawn'],
  'status: the member status enum names exactly these five, which a web test holds to the shared source'
);

select col_type_is(
  'public', 'member', 'is_admin', 'boolean',
  'role: a member is an administrator or is not, so the role vocabulary is two words and a web test holds them to the shared source'
);

select is_empty(
  $$
    select column_name
    from information_schema.columns
    where table_schema = 'public' and table_name = 'member' and column_name = 'role'
  $$,
  'role: nothing but is_admin says what a member may do, so a third role cannot arrive as a column nobody declared'
);

select * from finish();
rollback;
