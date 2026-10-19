create or replace function internal.masked_person_name(full_name text)
returns text language sql immutable set search_path = ''
as $$
  with parts as (
    select word, position, count(*) over () as words
    from regexp_split_to_table(btrim(coalesce(full_name, '')), '\s+') with ordinality as part(word, position)
    where word <> ''
  )
  select case
    when (select max(words) from parts) = 1 and full_name ~ '[가-힣]'
      then (select left(word, 1) || repeat('•', char_length(word) - 1) from parts)
    else (select coalesce(string_agg(case when position = words and words > 1 then word else repeat('•', char_length(word)) end,
      ' ' order by position), '') from parts)
  end;
$$;

comment on function internal.masked_person_name(text) is
  'a person as a data room reader may see them: the family name kept and the given name redacted, in the order the name is recorded (given name first); a one-word Korean name keeps its first letter';
