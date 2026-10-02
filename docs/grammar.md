---
title: Grammar
description: The formal grammar of Vascula templates.
section: Reference
order: 50
---

# Grammar

This is the complete grammar, in EBNF. `{` `}` mean "zero or more", `[` `]`
"optional", `|` "or". Quoted strings are literal. Whitespace between tokens
inside `{{ }}` and `{% %}` is ignored.

## Template

```text
template     = { text | output | tag | comment } ;
text         = any characters not starting "{{", "{%" or "{#" ;
output       = "{{" [ "-" ] expression [ "-" ] "}}" ;
comment      = "{#" [ "-" ] any characters [ "-" ] "#}"
             | "{%" "comment" "%}" any characters "{%" "endcomment" "%}" ;
tag          = "{%" [ "-" ] tag-body [ "-" ] "%}" ;
```

A `-` next to a delimiter removes all whitespace on that side of the markup.

## Tags

```text
if           = "if" expression "%}" template
               { "{%" "elsif" expression "%}" template }
               [ "{%" "else" "%}" template ]
               "{%" "endif" ;
unless       = "unless" expression "%}" template
               [ "{%" "else" "%}" template ]
               "{%" "endunless" ;
case         = "case" expression "%}" { text }
               when { when }
               [ "{%" "else" "%}" template ]
               "{%" "endcase" ;
when         = "{%" "when" expression { ( "," | "or" ) expression } "%}" template ;
for          = "for" name "in" ( path | range ) { modifier } "%}" template
               [ "{%" "else" "%}" template ]
               "{%" "endfor" ;
modifier     = "limit" ":" expression | "offset" ":" expression | "reversed" ;
break        = "break" ;
continue     = "continue" ;
cycle        = "cycle" expression { "," expression } ;
assign       = "assign" name "=" expression ;
capture      = "capture" name "%}" template "{%" "endcapture" ;
render       = "render" string { "," name ":" expression } ;
form         = "form" string { "," attribute ":" expression } "%}" template "{%" "endform" ;
host-tag     = host-name [ expression ] ;
```

`for`, `assign`, `capture` and `render` names may not be `settings`,
`forloop` or a name the application reserves. `form` kinds and host tag names
are defined by the application.

## Expressions

```text
expression   = and-expr { "or" and-expr } ;
and-expr     = comparison { "and" comparison } ;
comparison   = operand [ comparator operand ] ;
comparator   = "==" | "!=" | "<" | ">" | "<=" | ">=" | "contains" ;
operand      = primary { "|" filter } ;
filter       = name [ ":" arguments ] ;
arguments    = positional { "," positional } { "," named }
             | named { "," named } ;
positional   = primary ;
named        = name ":" primary ;
primary      = string | number | "true" | "false" | "nil" | "null"
             | "empty" | "blank" | range | path ;
range        = "(" ( number | path ) ".." ( number | path ) ")" ;
path         = name { "." name | "[" expression "]" } ;
```

## Tokens

```text
name         = ( letter | "_" ) { letter | digit | "_" } ;
string       = '"' { character | "\" character } '"'
             | "'" { character | "\" character } "'" ;
number       = [ "-" ] digit { digit } [ "." digit { digit } ] ;
```

A backslash in a string takes the next character literally. A number without
a decimal point is an integer.
