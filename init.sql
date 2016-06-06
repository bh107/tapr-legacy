drop table if exists volume;
create table volume (
	id integer primary key,
	serial text not null unique,
	slot integer,
	status text not null,
	library text
);
