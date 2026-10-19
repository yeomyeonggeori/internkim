create or replace function internal.masked_task_title(title text)
returns text language sql immutable set search_path = ''
as $$
  select coalesce(string_agg(repeat('•', char_length(word)), ' ' order by position), '')
  from regexp_split_to_table(btrim(coalesce(title, '')), '\s+') with ordinality as part(word, position)
  where word <> '';
$$;

comment on function internal.masked_task_title(text) is
  'a task title as a data room reader may see it: every word redacted, only the length of each left';
