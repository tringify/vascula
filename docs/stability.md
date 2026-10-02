---
title: Versions and stability
description: How Vascula is versioned, what will not change, and how to upgrade.
section: Reference
order: 51
---

# Versions and stability

## Versions

Vascula uses semantic versioning. Before 1.0, a release can change the
language or the Go API; every change is in the [changelog](/changelog), with
what to check when you upgrade.

## What does not change

These will not change:

- Output is always escaped, and templates cannot switch escaping off.
- A template reads only what the application gives it; unknown names are
  errors.
- Templates are bounded: compilation and rendering have limits.
- Template output is never evaluated as a template.
- Mistakes are errors with positions, not silently empty output.

## Upgrading

1. Pin an exact version and read the changelog entries between your version
   and the new one.
2. Compile every stored template with the new version. Compilation is
   strict, so syntax changes show up here.
3. If you can, render your templates with representative data on both
   versions and compare the output.
4. Recompile cached templates: a compiled template belongs to the version
   that compiled it.
