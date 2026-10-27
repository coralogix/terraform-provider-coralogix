---
name: generated-create-returns-only-id
description: "Use when tfgen fails for a released resource whose Create returns only an id, whose Update returns {}, or whose delete of a missing item is not a 404. Names the overrides keys and hooks."
---

# Generate a resource whose Create returns only the id

**Trigger:** Create returns `{thingId}`, Replace returns `{}`, Get is `/things/{thing_id}` with the id in property `id`, or a delete of a missing item is answered 403.

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
An empty Update response needs no line: Update calls Get afterwards. For another status than 404, pass `Hooks.DeleteGone` or `Hooks.UpdateGone` to `NewResource`: read the resource, and answer true only for a 404.

**Why:** a 403 can also mean a missing permission, so the hook reads the resource instead of trusting the status. Record the wire facts on a real tenant first.
