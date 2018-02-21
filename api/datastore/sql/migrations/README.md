# Migrations How-To

All migration files should be of the format:

`[0-9]+_[add|remove]_model[_field]*.sql`

The number at the beginning of the file name should be monotonically
increasing, from the last highest file number in this directory. E.g. if there
is `11_add_foo_bar.sql`, your new file should be `12_add_bar_baz.sql`.
