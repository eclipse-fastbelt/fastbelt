# Step 8 – Types artifact

Read `TASK.md` (section "Summary") first.

## Goal

A descriptive (not Go) view of the interfaces/types declared in the grammar, resolved and flattened.

## Work

1. Package `pluginapi/types` (imports `ast`, never the other way around):
   ```go
   type Type struct {
       Name          string
       Declaration   *ast.Interface
       Extends       []*Type  // direct supertypes
       AllSupertypes []*Type  // transitive, deterministic order
       Subtypes      []*Type  // direct subtypes
       OwnFields     []*Field
       AllFields     []*Field // incl. inherited, same semantics/order as generator's collectAllFields
       ReturnedBy    []*ast.ParserRule // rules whose (implicit or explicit) return type is this type
   }

   type Field struct {
       Name        string
       Owner       *Type
       Type        *FieldType
       Declaration *ast.Field
   }

   type FieldTypeKind int // Primitive, Node, Reference, Array

   type FieldType struct {
       Kind      FieldTypeKind
       Primitive string     // "string" | "bool" | "composite" when Kind == Primitive
       Target    *Type      // Kind == Node or Reference
       Element   *FieldType // Kind == Array
   }
   ```
2. `BuildArtifacts.Types []*types.Type` in grammar declaration order (same order as `types_gen.go`).
3. Reuse generator helpers (`getAllExtends`, `collectAllFields`, `grammar.FindReturnType`) via small exports
   instead of duplicating logic.

## Tests

- Simple, reference, array (incl. `[]*X`), primitive and composite field types.
- Single and multiple inheritance, diamond: `AllSupertypes` without duplicates, `AllFields` matches the
  generated Go interfaces for a fixture.
- `Subtypes` is the inverse of `Extends`.
- `ReturnedBy` for explicit `returns` and implicit (rule name = interface name), incl. a multi-file grammar.
- `Declaration` links back to the right AST nodes.
