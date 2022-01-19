(
 (const_spec
  name: (identifier) @_id
  value: (expression_list (raw_string_literal) @injection.content))

 (#match? @_id ".*Query$")
 (#set! injection.language "sql")
)
