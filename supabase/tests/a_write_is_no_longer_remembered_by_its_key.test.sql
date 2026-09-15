begin;
create extension if not exists pgtap with schema extensions;
select plan(1);

select hasnt_table('public', 'idempotency_key', 'forgotten: no table keeps the answers of keyed writes');

select * from finish();
rollback;
