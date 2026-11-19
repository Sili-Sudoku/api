# Contract

Files here come from the web repo. Do not edit by hand.

Expected, not yet exported by the web repo:
- `openapi.yaml` - the public API.
- `puzzle-definition.schema.json` - JSON Schema for PuzzleDefinition.

## validate-puzzle tool
Built from the web repo's engine. Reads puzzle JSON on stdin, writes one JSON object on stdout:

    {"valid": true, "solutions": "one", "errors": []}

`solutions` is `none`, `one` or `many`. A non-zero exit means the tool failed, not that the puzzle is bad;
the job retries. The Go type is `validate.Verdict`.
