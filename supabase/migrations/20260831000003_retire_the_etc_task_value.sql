-- '기타' was the device runtime's sentinel for "no type"; the web now renders
-- a null business/type as the localized etc label, so the sentinel and blank
-- strings retire into null.

update public.task set business = nullif(btrim(business), '') where business is not null;
update public.task set type = nullif(btrim(type), '') where type is not null;
update public.task set business = null where business = '기타';
update public.task set type = null where type = '기타';

update public.company
set task_vocabulary = jsonb_set(
  task_vocabulary,
  '{businesses}',
  coalesce(
    (
      select jsonb_agg(entry)
      from jsonb_array_elements(task_vocabulary -> 'businesses') entry
      where btrim(entry ->> 'name') not in ('', '기타')
    ),
    '[]'::jsonb
  )
)
where task_vocabulary ? 'businesses';

update public.company
set task_vocabulary = jsonb_set(
  task_vocabulary,
  '{types}',
  coalesce(
    (
      select jsonb_agg(entry)
      from jsonb_array_elements(task_vocabulary -> 'types') entry
      where btrim(entry ->> 'name') not in ('', '기타')
    ),
    '[]'::jsonb
  )
)
where task_vocabulary ? 'types';
