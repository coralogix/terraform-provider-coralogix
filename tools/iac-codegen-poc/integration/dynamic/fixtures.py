#!/usr/bin/env python3
"""Writes the API fixtures of the dynamic widget equivalence test
(equivalence_fixtures.json) from the spec: widgets.Dynamic with a value for
every field. Fixture N of each mode picks arm N of every oneOf (modulo its
arm count) and value N of every enum, so the fixtures of a mode together set
every arm:
  - full: every field, lists with two items;
  - zero: every field with its zero value, lists with one item (the API
    leaves some zero values in, as protobuf JSON does inside a set arm);
  - minimal: only the fields that the spec or the handwritten schema
    requires, the chosen arms, and the objects and lists of objects that hold
    them, so every other scalar is missing. A dashboard that Terraform
    manages always has the fields that the handwritten schema requires, so
    the API sends them;
  - empty: every list and map empty;
  - hollow: a list of objects has one item, in which every list and map is
    empty, so each list inside an item is empty once too;
  - noarm: every oneOf object sets no arm ({}), but the visualization and
    the query, without which the widget is refused.
The skipped fields of overrides.yaml are left out. The widgets of the
provider's dynamic widget unit tests (unit_test_widgets.json, the API values
that their Terraform models send) are fixtures too.

Run from the repository root: python3 integration/dynamic/fixtures.py
"""
import json
import yaml

SPEC = 'spec/openapi.patched.yaml'
OVERRIDES = 'integration/dynamic/overrides.yaml'
ROOT = 'widgets.Dynamic'
OUT = 'integration/dynamic/equivalence_fixtures.json'

# The fields that the handwritten schema of definition.dynamic requires
# (from its schema dump), as API component and field. The ones inside the
# typeStrings and the custom fields are covered by these.
TERRAFORM_REQUIRED = {
    ('Dynamic.QueryDefinition', 'query'), ('Filter.LogsFilter', 'operator'), ('HorizontalBarsMulti.QueryFieldSettings', 'queryId'),
    ('Metrics', 'promqlQuery'), ('ObservationField', 'keypath'), ('ObservationField', 'scope'), ('QueryDisplaySettings', 'queryId'),
    ('SpanObservationField', 'keypath'), ('SpanObservationField', 'scope'), ('SpansFilter', 'operator'),
    ('VerticalBarsMulti.QueryFieldSettings', 'queryId'), ('widgets.Dynamic', 'queryDefinitions'),
}

# The fields that are never empty in a Terraform dashboard: the handwritten
# validator requires a not_equals filter to have a value. Every mode sends
# them with values.
NOT_EMPTY = {('NotEquals.Selection.ListSelection', 'values')}

# The enum values that the handwritten code has no name for: new API values,
# which the second step adds (the compare lists them as new values).
NEW_VALUES = {'LEGEND_COLUMN_SIMPLE_VALUE', 'ORDER_DIRECTION_NONE'}

schemas = yaml.safe_load(open(SPEC))['components']['schemas']
overrides = yaml.safe_load(open(OVERRIDES))
skipped = {(c, f) for c, fs in (overrides.get('types') or {}).items() for f, o in fs.items() if isinstance(o, dict) and o.get('skip')}


def resolve(p):
    """Returns the schema of the property p, following $ref and allOf."""
    if '$ref' in p:
        return p['$ref'].split('/')[-1], schemas[p['$ref'].split('/')[-1]]
    for a in p.get('allOf', []):
        if '$ref' in a:
            return a['$ref'].split('/')[-1], schemas[a['$ref'].split('/')[-1]]
    return None, p


def groups(s):
    """The oneOf groups of the object s, as lists of arm names."""
    out = []
    for entry in [s] + s.get('allOf', []):
        arms = [r for o in entry.get('oneOf', []) for r in o.get('required', [])]
        if arms:
            out.append(arms)
    return out


def value(name, p, mode, n, depth, stack):
    comp, s = resolve(p)
    if comp in stack or depth > 12:
        return None
    t = s.get('type')
    if 'enum' in s:
        vals = s['enum']
        if mode == 'zero':
            return vals[0]
        real = [v for v in vals if (not v.endswith('_UNSPECIFIED') or v.endswith('_OR_UNSPECIFIED')) and v not in NEW_VALUES] or vals
        return real[n % len(real)]
    if t == 'array':
        if mode == 'empty' or mode == 'hollow' and not holds_objects(s):
            return []
        if mode == 'hollow':
            item = value(name, s['items'], 'empty', n, depth + 1, stack)
            return [item] if item is not None else []
        item = value(name, s['items'], mode, n, depth + 1, stack)
        second = value(name, s['items'], mode, n + 1, depth + 1, stack)
        return [x for x in (item, second) if x is not None][: 2 if mode == 'full' else 1]
    if t == 'object' and 'additionalProperties' in s and isinstance(s['additionalProperties'], dict):
        v = value(name, s['additionalProperties'], mode, n, depth + 1, stack)
        return {} if mode == 'empty' else {'k': v}
    if t == 'object' or 'properties' in s:
        return obj(comp, s, mode, n, depth, stack + [comp])
    if t == 'string':
        if s.get('format') == 'date-time':
            return '2026-09-27T08:00:00Z'
        if s.get('pattern') in ('^-?[0-9]+$', '^[0-9]+$'):
            return '0' if mode == 'zero' else str(5 + n)
        duration = name.lower().endswith('timeframe') or name in ('duration', 'relativeTimeFrame', 'interval')
        if mode == 'zero':
            return '0s' if duration else ''  # protobuf JSON writes a zero duration as 0s
        if duration:
            return '%ds' % (900 + 60 * n)
        if name.lower() == 'id' or name.lower().endswith('id'):
            return '00000000-0000-4000-8000-%012d' % (n + 1)
        return '%s-%d' % (name, n)
    if t == 'integer':
        return 0 if mode == 'zero' else 3 + n
    if t == 'number':
        return 0 if mode == 'zero' else 1.5 + n
    if t == 'boolean':
        return mode != 'zero'
    return None


# The oneOf objects that the noarm fixtures keep an arm in: the handwritten
# flatten refuses a widget, a query, a time frame, or a logs aggregation
# without one.
KEEP_ARM = {'Visualization', 'Dynamic.Query', 'TimeFrameSelect', 'LogsAggregation'}


def obj(comp, s, mode, n, depth, stack):
    props = s.get('properties', {})
    arms = groups(s)
    if mode == 'noarm' and arms and comp not in KEEP_ARM:
        arms_only = all(any(k in g for g in arms) for k in props)
        if arms_only:
            return {}
    chosen = set()
    for g in arms:
        live = [a for a in g if (comp, a) not in skipped]
        if live:
            chosen.add(live[n % len(live)])
    required = set(s.get('required', [])) | {f for c, f in TERRAFORM_REQUIRED if c == comp}
    out = {}
    for k, p in props.items():
        if (comp, k) in skipped:
            continue
        if any(k in g for g in arms) and k not in chosen:
            continue
        if mode == 'minimal' and k not in required and k not in chosen and not holds_objects(p) and (comp, k) not in NOT_EMPTY:
            continue
        v = value(k, p, 'full' if (comp, k) in NOT_EMPTY else mode, n, depth + 1, stack)
        if v is not None:
            out[k] = v
    return out


def holds_objects(p):
    """Reports whether the property p is an object or a list of objects."""
    _, s = resolve(p)
    if s.get('type') == 'array':
        _, s = resolve(s['items'])
    return 'enum' not in s and (s.get('type') == 'object' or 'properties' in s)


def max_arms(comp, seen):
    if comp in seen:
        return 1
    seen.add(comp)
    s = schemas[comp]
    m = max([len(g) for g in groups(s)] + [1])
    for p in s.get('properties', {}).values():
        c, ps = resolve(p)
        if ps.get('type') == 'array':
            c, ps = resolve(ps['items'])
        if c:
            m = max(m, max_arms(c, seen))
    return m


fixtures = {}
for n in range(max_arms(ROOT, set())):
    for mode in ('full', 'zero', 'minimal', 'empty', 'hollow', 'noarm'):
        fixtures['%s %02d' % (mode, n)] = obj(ROOT, schemas[ROOT], mode, n, 0, [ROOT])
for name, widget in json.load(open('integration/dynamic/unit_test_widgets.json')).items():
    fixtures['unit test ' + name] = widget
json.dump(fixtures, open(OUT, 'w'), indent=1, sort_keys=True)
print(len(fixtures), 'fixtures,', sum(len(json.dumps(f)) for f in fixtures.values()), 'bytes')
