It's possible to open the sql event store in two modes.

## Open(db *sql.DB) *SQL 

Creates a new sql event store with no restrictions.

## OpenWithSingelWriter(db *sql.DB) *SQL

Prevents multiple writers to save events concurrently to the event store. This can prevent 
problems in sqlite there multiple go routines writing concurrently could lock the datase.

https://www.sqlite.org/cgi/src/doc/begin-concurrent/doc/begin_concurrent.md
 
> If there is significant contention for the writer lock, this mechanism can
> be inefficient. In this case it is better for the application to use a mutex
> or some other mechanism that supports blocking to ensure that at most one
> writer is attempting to COMMIT a BEGIN CONCURRENT transaction at a time.
> This is usually easier if all writers are part of the same operating system process.
