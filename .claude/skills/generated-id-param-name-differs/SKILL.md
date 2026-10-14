---
name: generated-id-param-name-differs
description: "Use when tfgen reports FIELD_LIFECYCLE_UNSUPPORTED with Get=false, UPDATE_ID_NOT_IN_BODY, or an SDK signature mismatch (folder_id vs folderId) for a released resource whose path parameter is not named id. Maps the parameter to the id field."
---

# Generate a resource whose id path parameter has another name

**Trigger:** Get is `/things/{thing_id}`, but the resource field is `id`; Create returns only `thingId`; Replace returns `{}`.

**Fix:** in `behavior-overrides.yaml`:
```yaml
api:
  requestWrapper: thing        # if the body wraps the resource
  updateIDInBody: true
  idField: id                  # the resource field that {thing_id} names
  createResponseID: thingId    # Create response holds only the id; Create then calls Get
  serverSetID: true            # the shared schema cannot require the id
types:
  Thing: {fields: {id: {readOnly: true}}}
```
An empty Update response needs no line: the generated Update calls Get afterwards. If every field reports `Get=false`, the Get field is `allOf: [$ref]`; the generator reads it since beta.30.

**Why:** the generator matched the path parameter name to a resource field, so a different name looked like a missing id. Check each wire fact against a real recording before relying on it (a delete of a missing folder was 403, not 404).
