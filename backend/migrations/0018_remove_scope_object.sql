-- Page scope is the only scope dimension. No compatibility reads or writes remain.
ALTER TABLE runs DROP COLUMN scope_object;
