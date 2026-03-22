---
title: "Web Server"
myvar: "example value"
---
{@header}

## Hello, World

{myvar}

{@widget}

<div class="{=class}" {arg="arg1"}></div>

{arg2}

{$arg3}

{?arg4 {
  test4
}}

{!arg5 {
  test5

  {myvar}
}}

{#html}

{:plug1 arg="arg1" arg2 {
  content
}}

{:plug2 arg="arg1" arg2}
