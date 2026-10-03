create table employees (
    id uuid primary key,
    fio text not null,
    birth_date timestamp not null,
    is_fired boolean not null
);

---- create above / drop below ----

drop table employees;