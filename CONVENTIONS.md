# Code conventions

How the Go code is formatted and written. `STYLE.md` covers architecture (packages, endpoints, tests); this covers the lines
inside them. Where they disagree, `STYLE.md` wins.

Base: the [Google Go Style Guide](https://google.github.io/styleguide/go/) and `gofmt`. Nothing here enforces it yet, so
check by eye and by `gofmt -l .`.

## Spacing
- Separate the steps of a function with a blank line: set up, act, handle the result. A function that does three things
  should show three blocks.
- Put a blank line before a `return` unless the block is two or three lines.
- Put a blank line after an `if err != nil { ... }` block.
- No blank line between a call and the `if err` that checks it.

## Structs and declarations
- One field per line. Share a type on one line only for tight pairs (`x, y int`, `Path string` / `Timeout time.Duration`
  stay on their own lines).
- Group fields that belong together and separate groups with a blank line and a one-line comment saying what the group is.
- No one-line struct bodies except `struct{}`.

## Names
- Short names for short scopes (`e`, `v`, `i`); descriptive names once a variable lives longer than about ten lines.
- Do not shadow a parameter (`ctx, cancel := context.WithTimeout(ctx, ...)`) when the new value means something different.
  Name it (`ctx` stays the caller's, `runCtx` is the bounded one).

## Comments
- Say why, not what. One line is usually enough.
- Mark the lines that carry a safety rule (size caps, timeouts, untrusted input) so a later edit does not remove them.
- Exported names get a doc comment starting with the name.

## Errors
- Follow `STYLE.md` (`st.StackTrace`, `http_outcome`). Where that is not converted yet, wrap with `fmt.Errorf("what failed: %w", err)`.
- Say what was being done, not "error".
