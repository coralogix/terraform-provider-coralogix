---
name: generated-create-returns-only-id
description: "Use when tfgen fails for a released resource whose Create returns only an id or whose Update returns {}. Names the overrides keys."
---

# Generate a resource whose Create returns only the id

**Trigger:** Create returns `{thingId}`, Replace returns `{}`, and Get is `/things/{thing_id}` with the id in property `id`.

**Fix:** in `behavior-overrides.yaml`:
```yaml
api:
  requestWrapper: thing        # the body wraps the resource
  updateIDInBody: true
  idProperty: id               # the id property next to the path parameter thing_id
  createResponseOnlyIDProperty: thingId    # Create then reads the resource with Get
  serverSetID: true            # the shared schema cannot require the id
types:
  Thing: {fields: {id: {readOnly: true}}}
```
An empty Update response needs no line: Update calls Get afterwards.

**Why:** the generated Create and Update read the resource with Get when the response has no resource. Record the wire facts on a real tenant first.
